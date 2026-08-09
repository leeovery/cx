package capture_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"image/color"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// This file is the swap-and-diff completeness guard: render EVERY fixture
// under theme A, swap live to theme B through the production entry point, render
// again, and scan the second frame for any colour belonging to A. A survivor means
// some element never got the new theme — the "assert no stale data survived the
// invalidation" trick applied to rendered output rather than to a cache.
//
// It exists because the completeness risk's class of once-assigned cached styles
// (bubbles/list's help styles, pagination dots, TitleBar, both filter inputs) "cannot reliably
// be found by reading code". A one-off sweep is a point-in-time act; this is the
// standing behavioural net that catches whatever the sweep missed and whatever is
// added later.
//
// It lives in package capture_test because it needs internal/capture (the fixture
// registry and the render-swap-render seam), internal/tui (the model the seam
// returns) and internal/theme (the palettes) at once — a combination no single
// production package may take.
//
// LANE: unit. It renders only through the offline harness: no tmux server, no
// daemon, no built binary. No t.Parallel() anywhere (the cmd/tui packages inject
// through package-level state and the project bans it outright).
//
// THREE WAYS TO BUILD IT WRONG, ALL SILENT, all closed here:
//
//  1. Naming fixtures instead of enumerating them. The guard iterates
//     capture.FixtureNames(); the completeness guard: "the fixture list IS the coverage list,
//     and it grows automatically as screens are added". A guard naming four or five passes
//     today and keeps passing while the next screen goes uncovered.
//  2. Using two shipped themes. A hex both palettes set identically renders
//     identically before and after, so the guard cannot tell whether that site
//     updated. Hence two synthetic palettes with all 38 values unique.
//  3. Searching for the wrong representation. No scan here can IDENTIFY a wrong
//     form: 1 is a negative, so it never finds one and passes vacuously, and 2's
//     symmetry comparison empties both observed sets alike, which still agree. 2's
//     emptiness floor does fire for a form nothing renders — but it reports
//     "nothing observed on this screen", the symptom, not the malformation — and it
//     stays silent for a wrong form that is still a SUBSTRING of the real run,
//     which carriesRun goes on finding. So the derivation is pinned from outside
//     every scan, by TestThemeSwapGuard_TokenFormsAreWellFormed: every derived form
//     must round-trip back to its own token's channels — see tokenForms below.
//
// All three of the completeness guard's assertions live here: 1 (no theme-A value survives the
// swap), 2 (every token rendering under A renders again under B, as a union
// across fixtures) and 3 (every token in the vocabulary is rendered by SOME
// fixture, which is what makes 2's union complete rather than self-balancing).

// The guard renders at the shared harness dimensions declared in
// swap_harness_test.go (harnessWidth × harnessHeight): wide and tall enough that a
// fixture renders its full chrome — section header, list rows, pagination row,
// footer — rather than a degraded ladder step, so every colour-bearing element is
// genuinely on the frame instead of clipped off it. The size is pinned by that one
// pair of named constants rather than re-declared here, so the two guard files
// cannot render at silently different sizes.

// truecolorForegroundIntroducer / truecolorBackgroundIntroducer are the SGR
// parameter prefixes a 24-bit foreground and background open with. The
// well-formedness assertion below pins every derived form against them, so a
// derivation that produced some other shape is caught rather than scanned with.
const (
	truecolorForegroundIntroducer = "38;2;"
	truecolorBackgroundIntroducer = "48;2;"
)

// syntheticRedA / syntheticRedB are the fixed RED channel each synthetic palette
// is generated from — the single byte that makes the two palettes disjoint.
//
// Every generated component sits in the 100–255 decimal range, so every rendered
// channel is exactly THREE digits. That is load-bearing twice over: decimal SGR
// parameters are prefix-ambiguous (`38;2;1;0;5` is a prefix of `38;2;1;0;55`), and
// a fixed-width triple can never appear from anything but a token — today's canned
// scrollback carries only 4-bit SGRs, but the generator must not depend on that
// staying true.
const (
	syntheticRedA = 0x6E // 110
	syntheticRedB = 0xD2 // 210
)

// syntheticGreenBase / syntheticBlueBase are the per-token ramps: token i takes
// green base+i and blue base+i, so the 19 values within one palette are unique and
// every component stays three digits (green 129–147, blue 201–219).
const (
	syntheticGreenBase = 0x80 // 128
	syntheticBlueBase  = 0xC8 // 200
)

// syntheticTheme builds one whole 19-token palette from a fixed red channel.
//
// theme.Theme is an ordinary struct, so the guard's palettes need no
// loader, no file and no embedded set — which is what keeps them independent of
// anything done to the shipped ones.
//
// SHIPPED PALETTES ARE DELIBERATELY NOT USED. Two shipped themes fail two
// ways, both a matter of time: a hex both palettes happen to set identically
// survives the swap LEGITIMATELY, so the guard fails permanently for a non-bug;
// and — worse because it is silent — a token with the same value either side
// renders identically before and after, so the guard cannot tell whether that site
// updated. It passes either way and the site is uncovered with no signal.
func syntheticTheme(red int) theme.Theme {
	v := func(i int) theme.Token {
		return theme.Token{Value: fmt.Sprintf("#%02X%02X%02X", red, syntheticGreenBase+i, syntheticBlueBase+i)}
	}
	return theme.Theme{
		TextPrimary:      v(1),
		TextSecondary:    v(2),
		TextTertiary:     v(3),
		TextMuted:        v(4),
		TextSubtle:       v(5),
		TextFaint:        v(6),
		TextOnSelection:  v(7),
		AccentPrimary:    v(8),
		AccentKey:        v(9),
		AccentMode:       v(10),
		AccentAttention:  v(11),
		StatePositive:    v(12),
		StateDestructive: v(13),
		Canvas:           v(14),
		BgSelection:      v(15),
		BgAttention:      v(16),
		BgSubtle:         v(17),
		Border:           v(18),
		TextOnAttention:  v(19),
	}
}

// syntheticPalettes returns the two palettes every assertion here swaps between.
func syntheticPalettes() (a, b theme.Theme) {
	return syntheticTheme(syntheticRedA), syntheticTheme(syntheticRedB)
}

// tokenForm is one token's name paired with BOTH of its rendered truecolor
// parameter runs.
type tokenForm struct {
	name string
	fg   string // `38;2;R;G;B` — the foreground run
	bg   string // `48;2;R;G;B` — the background run
}

// tokenForms derives every token's rendered SGR forms, in the canonical table order.
//
// THE COMPARISON IS AGAINST THE RENDERED FORM, NOT THE HEX. Styled output
// carries no hex at all — a truecolor foreground is `ESC[38;2;R;G;Bm`, decimal —
// so a guard that searched for `#6E81C9` would find nothing, ever. Assertion 1 is
// a NEGATIVE ("no theme-A value survives"), so searching for the wrong
// representation passes vacuously and silently.
//
// NO SCANNING ASSERTION BACKSTOPS THIS DERIVATION FULLY — assertion 2 comes
// closest, and only through one of its two halves. Its SYMMETRY comparison cannot
// judge a form at all: a wrong one empties the A-observed and B-observed sets
// alike, and two empty sets agree, so it passes as silently as assertion 1 does.
// Its EMPTINESS FLOOR is not blind — a form nothing renders empties every
// fixture's A set and the floor errors — but it reports "no theme-A token observed
// at all", which is the symptom; a screen that genuinely rendered nothing reads
// identically. And the floor only sees a form nothing renders: one still contained
// INSIDE the real run is found by carriesRun regardless, and every scan here passes
// on it (see the two measured slips on
// TestThemeSwapGuard_TokenFormsAreWellFormed).
//
// So the derivation is pinned from outside all of them, by
// TestThemeSwapGuard_TokenFormsAreWellFormed: every form must round-trip back to
// its own token's channels. It is the only check that fires for EVERY wrong form,
// and the only one that names the derivation as the cause. Sharing one derivation
// across the two assertions keeps them from drifting apart, but it is that test,
// not the sharing, that keeps it honest.
//
// BOTH forms are derived for every token because which one a token renders as is
// not knowable from its name: `border` renders as a FOREGROUND on rules and modal
// frames, while `canvas`, `bg.selection` and `bg.attention` only ever render as a
// BACKGROUND. `bg.subtle` is the measured exception to the tint intuition — the
// loading bar's track paints it as foreground and background alike — as is
// `accent.primary`, which backs the bar's filled run. See
// TestTokenCoverage_MatchesBackgroundForm, which asserts that split rather than
// leaving it to this comment.
//
// Each form is taken from a real lipgloss render rather than string-formatted, so
// it cannot drift from what the renderer actually emits.
func tokenForms(t *testing.T, th theme.Theme) []tokenForm {
	t.Helper()
	tokens := th.All()
	forms := make([]tokenForm, 0, len(tokens))
	for _, tok := range tokens {
		forms = append(forms, tokenForm{
			name: tok.Name,
			fg:   sgrParameterRun(t, lipgloss.NewStyle().Foreground(tok.Color())),
			bg:   sgrParameterRun(t, lipgloss.NewStyle().Background(tok.Color())),
		})
	}
	return forms
}

// sgrParameterRun renders a one-cell probe through the style and returns the SGR
// parameter run it opens with — everything between the `[` and the terminating
// `m`.
func sgrParameterRun(t *testing.T, style lipgloss.Style) string {
	t.Helper()
	probe := style.Render("x")
	start := strings.IndexByte(probe, '[')
	end := strings.IndexByte(probe, 'm')
	if start < 0 || end <= start {
		t.Fatalf("could not derive an SGR parameter run from %q", probe)
	}
	return probe[start+1 : end]
}

// carriesRun reports whether the frame contains the SGR parameter run TERMINATED
// — by `m` (the run closes the escape) or `;` (a further parameter follows).
//
// The terminator check is what stops a longer parameter swallowing a shorter one:
// decimal SGR parameters are prefix-ambiguous, so an untermined match would report
// `38;2;1;0;5` present inside `38;2;1;0;55`. The generator's fixed-width
// components already remove that case, and this removes it again — assertion 1 is
// a negative, and belt-and-braces on a negative is cheap.
func carriesRun(frame, run string) bool {
	for offset := 0; offset < len(frame); {
		at := strings.Index(frame[offset:], run)
		if at < 0 {
			return false
		}
		end := offset + at + len(run)
		if end < len(frame) && (frame[end] == 'm' || frame[end] == ';') {
			return true
		}
		offset += at + 1
	}
	return false
}

// observedTokens returns the names of every token whose foreground OR background
// form appears on the frame.
func observedTokens(frame string, forms []tokenForm) map[string]bool {
	seen := make(map[string]bool, len(forms))
	for _, form := range forms {
		if carriesRun(frame, form.fg) || carriesRun(frame, form.bg) {
			seen[form.name] = true
		}
	}
	return seen
}

// colourReporter is the structural discriminator the completeness guard excludes colourless
// fixtures on. It is an interface so the exclusion can be driven from a test with
// a locally-constructed colourless fixture — no registry fixture is colourless
// today, so asserting only over the registered set would exercise nothing but the
// false branch.
type colourReporter interface{ Colourless() bool }

// excludeColourless drops every colourless fixture from the enumerated set.
//
// A colourless render carries no theme colours at all, so there is nothing to diff
// and inclusion would be meaningless rather than merely redundant. The exclusion
// reads each fixture's own flag rather than a name list, so a colourless fixture
// added later is excluded automatically. No fixture sets it today, which makes the
// exclusion live but empty — deliberately NOT pinned by an excluded-count
// assertion, which would fail the day one is added.
func excludeColourless[F colourReporter](all []F) []F {
	return slices.DeleteFunc(all, func(fx F) bool { return fx.Colourless() })
}

// registryFixtures resolves every ENUMERATED fixture — capture.FixtureNames() is
// iterated, no fixture is ever named.
//
// The single sanctioned skip is the contrast-validation swatch, compared against
// the exported constant rather than a string literal: it is a standalone tea.Model
// resolved by the capture tool and deliberately not routed through tui.Build,
// so it has no tui.Model to swap. Every other enumerated name must
// resolve or the guard fails LOUDLY — a silent skip reads as coverage, which is
// the whole failure mode this construction exists to prevent.
func registryFixtures(t *testing.T) []*capture.Fixture {
	t.Helper()
	names := capture.FixtureNames()
	fixtures := make([]*capture.Fixture, 0, len(names))
	for _, name := range names {
		if name == capture.ContrastValidationFixture {
			continue
		}
		fx, err := capture.FixtureByName(name)
		if err != nil {
			t.Fatalf("FixtureByName(%s): %v — every enumerated name but %s must resolve; skipping one silently would read as coverage", name, err, capture.ContrastValidationFixture)
		}
		fixtures = append(fixtures, fx)
	}
	return fixtures
}

// guardedFixtures is the set every assertion below runs over: the enumerated
// registry minus the swatch, minus the colourless ones.
//
// The emptiness floor is the non-vacuity guarantee AFTER the colourless filter, one
// layer below the one registryFixtures' caller applies before it. Every assertion
// here is a range over this set, so an empty one is silently green: the four
// per-fixture tests would run zero subtests and the union assertion would compare
// two empty maps. Nothing else would report it. It stays a floor and not a count —
// see excludeColourless on why pinning the excluded number would fail the day a
// colourless fixture is legitimately added.
func guardedFixtures(t *testing.T) []*capture.Fixture {
	t.Helper()
	fixtures := excludeColourless(registryFixtures(t))
	if len(fixtures) == 0 {
		t.Fatal("every enumerated fixture reported colourless; the guard would range over nothing and pass vacuously")
	}
	return fixtures
}

// swapModel drives one fixture through the production swap and hands back the
// model AFTER it, so a caller can read the frame's declarative background — which
// the string pair RenderSwapRender returns deliberately does not carry.
//
// It mirrors RenderSwapRender exactly: ONE model, rendered under A first (that
// render is what populates the once-assigned caches, so it is not optional
// set-up), then swapped through Model.ApplyTheme — the production entry point the
// panel's arrow-preview drives.
func swapModel(fx *capture.Fixture, a, b theme.Theme) tui.Model {
	m := fx.ModelAt(a, harnessWidth, harnessHeight)
	_ = m.View()
	m.ApplyTheme(b)
	return m
}

// sameColour compares two rendered colours by their resolved RGBA components, so
// the assertion does not depend on which concrete color.Color implementation
// lipgloss happens to return for a hex.
func sameColour(got, want color.Color) bool {
	if got == nil || want == nil {
		return got == nil && want == nil
	}
	gr, gg, gb, ga := got.RGBA()
	wr, wg, wb, wa := want.RGBA()
	return gr == wr && gg == wg && gb == wb && ga == wa
}

// TestThemeSwapGuard_EnumeratesRegistry pins the mechanism every claim this file
// makes rests on: the guard enumerates the harness's fixture set and never names a
// fixture, so a screen added later is covered with no edit here.
func TestThemeSwapGuard_EnumeratesRegistry(t *testing.T) {
	t.Run("every enumerated name but the swatch resolves to a fixture", func(t *testing.T) {
		resolved := make([]string, 0, len(capture.FixtureNames()))
		for _, fx := range registryFixtures(t) {
			resolved = append(resolved, fx.Name())
		}
		if len(resolved) == 0 {
			t.Fatal("the registry enumerated no build-backed fixtures; the guard would assert over nothing")
		}
		want := slices.DeleteFunc(capture.FixtureNames(), func(n string) bool {
			return n == capture.ContrastValidationFixture
		})
		slices.Sort(resolved)
		if !slices.Equal(resolved, want) {
			t.Errorf("the guard covers %v, want every enumerated fixture %v", resolved, want)
		}
	})

	// The converse of the one sanctioned skip. If the swatch is ever promoted to a
	// real tui.Build-backed fixture it will resolve here, this fails, and the skip
	// has to be reconsidered rather than silently excluding a covered screen.
	t.Run("the swatch is the only skip, and it is not a build-backed fixture", func(t *testing.T) {
		if _, err := capture.FixtureByName(capture.ContrastValidationFixture); err == nil {
			t.Errorf("FixtureByName(%s) resolved a fixture; it is skipped as a standalone tea.Model, so a resolvable one must go under the guard instead", capture.ContrastValidationFixture)
		}
	})
}

// TestFixtureRegistry_ByNameCasesMatchFixtureNames closes the two-list drift the
// enumeration would otherwise be blind to.
//
// FixtureByName's switch and FixtureNames()'s slice are two hand-maintained lists.
// A fixture present in the switch but ABSENT from the slice is invisible to an
// enumerating guard — it exists, it renders, and nothing ever swaps it. Absence
// reads as coverage, which is exactly the shape the completeness guard warns about.
func TestFixtureRegistry_ByNameCasesMatchFixtureNames(t *testing.T) {
	cases := fixtureByNameCases(t)
	want := slices.DeleteFunc(capture.FixtureNames(), func(n string) bool {
		return n == capture.ContrastValidationFixture
	})
	slices.Sort(cases)
	if !slices.Equal(cases, want) {
		t.Errorf("FixtureByName switches on %v, FixtureNames() lists %v — a fixture in one list and not the other is invisible to the guard", cases, want)
	}
}

// fixtureByNameCases AST-scans the fixture catalogue and returns the string
// literals FixtureByName's switch cases match on, so the comparison above reads
// the real code rather than a restated list.
//
// IT READS LITERALS ONLY, AND FATALS ON ANYTHING ELSE. A case written as a named
// constant is not a literal, so skipping it silently would drop it from the scanned
// set — and a fixture the switch resolves but neither list carries is invisible to
// the enumerating guard, which is the very failure the comparison above exists to
// close. Refusing to read is therefore not pedantry: a silent skip would make this
// check claim more than it delivers. The catalogue is 100% literal today, and
// FixtureNames() already names one fixture by constant, so a constant-cased case is
// a natural next edit.
//
// Resolving identifiers instead — scanning the file's const declarations and
// substituting values — was considered and rejected: it adds a symbol table for a
// case that does not exist yet, and it fails OPEN (back to a silent skip) whenever
// the constant is declared in another file. Failing loudly asks for one line of
// thought at the moment the shape changes and cannot degrade quietly in the
// meantime.
func fixtureByNameCases(t *testing.T) []string {
	t.Helper()
	const catalogue = "fixtures.go"
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, catalogue, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", catalogue, err)
	}
	var cases []string
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "FixtureByName" || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			clause, ok := n.(*ast.CaseClause)
			if !ok {
				return true
			}
			for _, expr := range clause.List {
				lit, ok := expr.(*ast.BasicLit)
				if !ok || lit.Kind != token.STRING {
					t.Fatalf("%s: FixtureByName carries a non string-literal case expression (%T) — this scan reads literals only, so such a case is invisible to the drift check above", catalogue, expr)
				}
				unquoted, err := strconv.Unquote(lit.Value)
				if err != nil {
					t.Fatalf("unquote case literal %s: %v", lit.Value, err)
				}
				cases = append(cases, unquoted)
			}
			return true
		})
	}
	if len(cases) == 0 {
		t.Fatalf("%s declares no FixtureByName switch cases; the drift check would pass vacuously", catalogue)
	}
	return cases
}

// TestSyntheticThemes_AllValuesUnique pins the property the completeness guard makes the
// guard's discriminating power rest on: all 38 values differ, none repeated within a
// palette or across the pair. A value shared across the pair renders identically
// before and after, so the guard could not tell whether that site updated.
func TestSyntheticThemes_AllValuesUnique(t *testing.T) {
	a, b := syntheticPalettes()
	tokens := append(a.All(), b.All()...)

	t.Run("all 38 values are unique", func(t *testing.T) {
		if len(tokens) != 38 {
			t.Fatalf("the pair carries %d values, want 38 (19 tokens × 2 palettes)", len(tokens))
		}
		seen := make(map[string]string, len(tokens))
		for i, tok := range tokens {
			palette := "A"
			if i >= len(tokens)/2 {
				palette = "B"
			}
			owner := palette + "." + tok.Name
			if prior, dup := seen[tok.Value]; dup {
				t.Errorf("%s repeats %s's value %s; a shared value renders identically either side of the swap and covers nothing", owner, prior, tok.Value)
			}
			seen[tok.Value] = owner
		}
	})

	// Fixed-width three-digit components are what make no rendered triple a prefix
	// of another, and what keeps a synthetic value from colliding with the 4-bit
	// SGRs a fixture's canned content carries.
	t.Run("every component renders as three decimal digits", func(t *testing.T) {
		for _, tok := range tokens {
			for _, component := range rgbComponents(t, tok.Value) {
				if component < 100 || component > 255 {
					t.Errorf("token %s value %s has component %d outside the 100–255 three-digit range", tok.Name, tok.Value, component)
				}
			}
		}
	})
}

// rgbComponents parses a #RRGGBB value into its three decimal channels.
func rgbComponents(t *testing.T, value string) [3]int {
	t.Helper()
	if len(value) != 7 || value[0] != '#' {
		t.Fatalf("value %q is not #RRGGBB", value)
	}
	var components [3]int
	for i := range components {
		channel, err := strconv.ParseUint(value[1+2*i:3+2*i], 16, 8)
		if err != nil {
			t.Fatalf("parse channel %d of %q: %v", i, value, err)
		}
		components[i] = int(channel)
	}
	return components
}

// TestThemeSwapGuard_TokenFormsAreWellFormed is what makes the shared derivation
// itself load-bearing, closing failure mode 3 (searching for the wrong
// representation) from the derivation end rather than trusting a scan to notice.
//
// NO SCANNING ASSERTION IDENTIFIES A WRONG DERIVATION, AND THIS IS THE ONLY CHECK
// THAT CATCHES EVERY ONE. Assertion 1 is a negative, so a form nothing renders is
// never found and it passes vacuously. Assertion 2's symmetry comparison is blind
// from the other side — a wrong derivation empties the A-observed and B-observed
// sets alike, and two empty sets agree. Its emptiness floor is NOT blind: a form
// nothing renders empties every fixture's A set and the floor errors. But it
// reports "no theme-A token observed at all" — the symptom — and cannot say the
// derivation is why, because a screen that genuinely rendered nothing reads
// identically.
//
// MEASURED, over sgrParameterRun's two one-character slips (each applied alone,
// against the whole file):
//
//   - one char off the FRONT of the run (`38;2;R;G;B` → `8;2;R;G;B`) fires THIS
//     TEST ALONE. The short form is still a substring of the run it was sliced
//     from, so carriesRun goes on finding it: the truecolor canary, the emptiness
//     floor and the symmetry check all pass on a form the derivation got wrong.
//   - one char past its END (`38;2;R;G;B` → `38;2;R;G;Bm`) fires this test, the
//     canary and the floor — three failures, of which only this one names the
//     derivation; the other two report the symptom.
//
// So the derivation is pinned directly and positively: each form must open with
// its 24-bit introducer and carry exactly the token's own three channels. That is
// a round trip — hex in, rendered run out, channels back — which catches both a
// mis-sliced run and a truncating one, including the front slip nothing else here
// reports at all.
func TestThemeSwapGuard_TokenFormsAreWellFormed(t *testing.T) {
	a, b := syntheticPalettes()
	for _, palette := range []struct {
		name  string
		theme theme.Theme
	}{{"A", a}, {"B", b}} {
		t.Run("palette "+palette.name, func(t *testing.T) {
			values := make(map[string]string, len(palette.theme.All()))
			for _, tok := range palette.theme.All() {
				values[tok.Name] = tok.Value
			}
			for _, form := range tokenForms(t, palette.theme) {
				want := rgbComponents(t, values[form.name])
				assertParameterRun(t, form.name, "foreground", form.fg, truecolorForegroundIntroducer, want)
				assertParameterRun(t, form.name, "background", form.bg, truecolorBackgroundIntroducer, want)
			}
		})
	}
}

// assertParameterRun checks one derived run is exactly `<introducer>R;G;B` for the
// token's own channels.
func assertParameterRun(t *testing.T, token, role, run, introducer string, want [3]int) {
	t.Helper()
	channels, opened := strings.CutPrefix(run, introducer)
	if !opened {
		t.Errorf("token %s: derived %s run %q does not open with %q, so the guard scans for a form no render can contain", token, role, run, introducer)
		return
	}
	fields := strings.Split(channels, ";")
	if len(fields) != len(want) {
		t.Errorf("token %s: derived %s run %q carries %d channel(s) after %q, want %d", token, role, run, len(fields), introducer, len(want))
		return
	}
	for i, field := range fields {
		got, err := strconv.Atoi(field)
		if err != nil {
			t.Errorf("token %s: derived %s run %q has non-decimal channel %d (%q): %v", token, role, run, i, field, err)
			continue
		}
		if got != want[i] {
			t.Errorf("token %s: derived %s run %q has channel %d = %d, want %d — the run does not round-trip to the token's own value", token, role, run, i, got, want[i])
		}
	}
}

// TestThemeSwapGuard_RenderIsTruecolor is the canary under every other assertion
// here: each A-frame must carry at least one DERIVED theme-A run.
//
// Comparing View().Content rather than a writer-flushed frame is what satisfies
// the completeness guard's truecolor requirement — lipgloss v2 moved profile handling to the
// output-writer layer, so Render is unconditionally truecolor and the model's own
// view string carries full 24-bit SGRs even under `go test`, where stdout is not a
// TTY. Without this canary a future profile change could strip colour and leave
// the whole guard passing on colourless bytes with nothing to diff.
//
// It searches for a form tokenForms actually produced rather than for the bare
// 24-bit introducer, which buys one thing the literal cannot: it catches a
// colour-profile divergence between the one-cell probe the forms are derived from
// and the fixture render they are scanned against. The introducer is blind to that
// split because both sides would still carry `38;2;` while agreeing on nothing.
//
// It is NOT a pin on the derivation's shape, even though it does fire for some
// malformations. carriesRun matches on substring, so a form sliced SHORT from
// inside the real run is still found and this passes on it — the measured front
// slip on TestThemeSwapGuard_TokenFormsAreWellFormed, which is the assertion that
// closes the shape for every wrong form, and the only one that does.
func TestThemeSwapGuard_RenderIsTruecolor(t *testing.T) {
	a, b := syntheticPalettes()
	aForms := tokenForms(t, a)
	for _, fx := range guardedFixtures(t) {
		t.Run(fx.Name(), func(t *testing.T) {
			before, _ := fx.RenderSwapRender(a, b, harnessWidth, harnessHeight)
			carried := slices.ContainsFunc(aForms, func(form tokenForm) bool {
				return carriesRun(before, form.fg) || carriesRun(before, form.bg)
			})
			if !carried {
				t.Error("the A-frame carries none of the derived theme-A runs, so either it is not a truecolor render or the derived forms do not match what it renders — either way there is nothing for the guard to diff")
			}
		})
	}
}

// TestThemeSwapGuard_NoStaleValueSurvives is the completeness guard's assertion 1: after the
// live swap, no theme-A value appears anywhere on the frame.
//
// A survivor means some element never got the new theme — a once-assigned cached
// style that the restyle path does not re-point, which is the class that
// "cannot reliably be found by reading code".
//
// The swap is a live mutation of ONE already-rendered model through
// RenderSwapRender: the A-render is what populates those caches, so a fixture
// rendered only after the swap would pass trivially, and a test building one model
// per theme would assign every cached style correctly in each and pass green while
// live swap was broken.
func TestThemeSwapGuard_NoStaleValueSurvives(t *testing.T) {
	a, b := syntheticPalettes()
	aForms := tokenForms(t, a)

	for _, fx := range guardedFixtures(t) {
		t.Run(fx.Name(), func(t *testing.T) {
			_, after := fx.RenderSwapRender(a, b, harnessWidth, harnessHeight)
			for _, form := range aForms {
				if carriesRun(after, form.fg) {
					t.Errorf("token %s: theme A's foreground run %q survives the swap on fixture %s — that site was never re-pointed onto the new theme", form.name, form.fg, fx.Name())
				}
				if carriesRun(after, form.bg) {
					t.Errorf("token %s: theme A's background run %q survives the swap on fixture %s — that site was never re-pointed onto the new theme", form.name, form.bg, fx.Name())
				}
			}
		})
	}
}

// TestThemeSwapGuard_EveryBValuePresentInUnion is the completeness guard's assertion 2: every
// token that rendered under theme A renders again under theme B — catching a site that
// renders NOTHING after the swap rather than merely something stale, which
// assertion 1, being a negative, reports as clean. That is a live shape here and
// not a hypothetical: theme.Token.Color() resolves a zero-value token through
// lipgloss.Color("") to the NO-COLOUR SENTINEL rather than erroring, so a cached
// style re-pointed at a partly-populated theme paints nothing at all.
//
// It is checked at two granularities, and each carries a DIFFERENT claim:
//
//   - PER FIXTURE, as a symmetry check: one screen renders exactly the same set of
//     token ROLES either side of the swap. This is what catches one screen losing a
//     role, which the union structurally cannot see — every other fixture still
//     rendering that token keeps the union's set complete while this one has gone
//     colourless. Its granularity is screen × role and NOT per element: a role
//     painted at two sites on one screen stays in the set when only one of them
//     goes colourless, so that residual is uncovered here and by assertion 1 alike
//     (a colourless site carries no stale run to find). Both scan the frame as one
//     string, so no per-element claim is available to either.
//   - ACROSS FIXTURES, as the UNION the completeness guard names. Deliberately not a
//     per-fixture "all 19 roles": no single screen renders them all, so that form would be
//     false for every fixture in the set.
//
// THE PER-FIXTURE CHECK IS THE STRICTLY STRONGER OF THE TWO — they do not divide
// the work between them. A token in the union under A but under B nowhere must be
// absent from some fixture's B set, so symmetry fails there too, and the union
// cannot fail alone. The union is kept because it is the form the spec
// names, because its message names every fixture the A form was seen on rather than
// only the first divergent screen, and because it is what still stands if the
// per-fixture check is ever weakened.
//
// The emptiness floor under both is what pins observedTokens itself. A collection
// helper that returned nothing would make every set empty, symmetric, and equally
// uninteresting, and both checks above would pass on it — the same vacuity that
// tokenForms is pinned against from outside, one layer down. Nothing else in the
// file routes through observedTokens (the truecolor canary calls carriesRun
// directly), so this floor is its only pin.
//
// SYMMETRY IS A PROPERTY OF THE RENDER, NOT A COINCIDENCE OF TODAY'S FIXTURE SET.
// Which roles a screen paints follows from its content and chrome, and no render
// path branches on a colour's VALUE — the one place that reads darkness is the
// appearance gate, which the harness resolves at construction by injecting a
// CONSTANT nomination. Swapping the palette therefore cannot change which
// sites paint, only what they paint with.
func TestThemeSwapGuard_EveryBValuePresentInUnion(t *testing.T) {
	a, b := syntheticPalettes()
	aForms, bForms := tokenForms(t, a), tokenForms(t, b)

	seenUnderA := make(map[string][]string, len(aForms))
	seenUnderB := make(map[string][]string, len(bForms))
	for _, fx := range guardedFixtures(t) {
		before, after := fx.RenderSwapRender(a, b, harnessWidth, harnessHeight)
		underA, underB := observedTokens(before, aForms), observedTokens(after, bForms)

		if len(underA) == 0 {
			t.Errorf("fixture %s: no theme-A token observed at all, so the swap has nothing to be diffed against on this screen", fx.Name())
		}
		namesA, namesB := slices.Sorted(maps.Keys(underA)), slices.Sorted(maps.Keys(underB))
		if !slices.Equal(namesA, namesB) {
			t.Errorf("fixture %s: renders %v under A but %v under B — a site on this screen stopped painting a token it painted before the swap, or started painting one it did not", fx.Name(), namesA, namesB)
		}

		for name := range underA {
			seenUnderA[name] = append(seenUnderA[name], fx.Name())
		}
		for name := range underB {
			seenUnderB[name] = append(seenUnderB[name], fx.Name())
		}
	}

	for _, name := range theme.TokenNames() {
		underA, underB := seenUnderA[name], seenUnderB[name]
		if len(underA) > 0 && len(underB) == 0 {
			slices.Sort(underA)
			t.Errorf("token %s renders under theme A (on %v) but no theme-B value for it appears on any fixture after the swap — that site renders nothing rather than merely stale", name, underA)
		}
		if len(underA) == 0 && len(underB) > 0 {
			slices.Sort(underB)
			t.Errorf("token %s renders under theme B (on %v) but never under theme A — the two renders disagree on which sites exist, so the diff is not comparing like for like", name, underB)
		}
	}
}

// swappableFixture is everything the coverage scan needs of a fixture: a name for
// the failure message, and the render-swap-render seam.
//
// *capture.Fixture satisfies it. The interface exists so
// TestTokenCoverage_IgnoresExcludedFixtures can drive the same
// excludeColourless-then-cover composition over a stand-in, which no registry
// fixture can supply — none is colourless.
type swappableFixture interface {
	Name() string
	RenderSwapRender(a, b theme.Theme, w, h int) (before, after string)
}

// swappedFrame pairs a fixture's name with its POST-SWAP frame.
type swappedFrame struct {
	fixture string
	frame   string
}

// swappedFrames renders each fixture under a, swaps it live to b, and returns the
// theme-B frames.
//
// The render pass is separated from the scan because rendering the set is the
// expensive half and the scans below differ only in WHICH forms they look for, so
// one pass feeds several scans.
//
// Coverage is computed over the B-frames, matching the B half of assertion 2's
// union: assertion 3's claim is about that union, so it must be measured on the
// same frames.
func swappedFrames[F swappableFixture](fixtures []F, a, b theme.Theme) []swappedFrame {
	frames := make([]swappedFrame, 0, len(fixtures))
	for _, fx := range fixtures {
		_, after := fx.RenderSwapRender(a, b, harnessWidth, harnessHeight)
		frames = append(frames, swappedFrame{fixture: fx.Name(), frame: after})
	}
	return frames
}

// coveredTokens returns, for every token observed on some frame, the fixtures
// whose frame carried it — the LOCI, so a failure can say where a token is
// rendered rather than only that it is.
//
// It routes through observedTokens, the same collection helper assertion 2 uses,
// so the two cannot disagree about what "observed" means — which matters
// precisely because assertion 3's claim is that assertion 2's union is complete.
// observedTokens matches a token on EITHER rendered form; the measured reason
// that OR is load-bearing rather than defensive is pinned by
// TestTokenCoverage_MatchesForegroundForm and
// TestTokenCoverage_MatchesBackgroundForm.
func coveredTokens(frames []swappedFrame, forms []tokenForm) map[string][]string {
	loci := make(map[string][]string, len(forms))
	for _, f := range frames {
		for name := range observedTokens(f.frame, forms) {
			loci[name] = append(loci[name], f.fixture)
		}
	}
	return loci
}

// uncoveredTokens returns every token of the closed vocabulary that no frame
// carried, in the canonical table order.
//
// It enumerates theme.TokenNames() rather than the coverage map's keys, and that
// is the whole mechanism: an uncovered token is by definition absent from the
// map, so nothing derived from the map alone could ever name it.
func uncoveredTokens(loci map[string][]string) []string {
	var gaps []string
	for _, name := range theme.TokenNames() {
		if len(loci[name]) == 0 {
			gaps = append(gaps, name)
		}
	}
	return gaps
}

// TestThemeSwapGuard_EveryTokenExercisedByAFixture is the completeness guard's assertion 3:
// over the INCLUDED fixtures' post-swap frames, every one of the 19 tokens is observed
// at least once.
//
// IT IS WHAT MAKES ASSERTION 2's UNION COMPLETE. Assertion 2 compares a union
// under A against a union under B, so a token rendering on NO fixture is absent
// from both, the two balance perfectly, and it reports nothing — the guard is
// silently blind at exactly the sites it exists to protect. The harness contract states
// the same shape from the fixture end: "a missing fixture is a blind spot the
// guard structurally cannot report … absence reads as coverage."
//
// A GAP IS CLOSED BY ADDING A FIXTURE, NEVER BY EXEMPTING A TOKEN. An exemption
// list is the permanent render-layer carve-out the re-theme-everything rule calls "precisely
// the shape the swap-and-diff guard exists to catch"; carving one into the guard itself is
// the one response that cannot be right.
//
// It enumerates theme.TokenNames() rather than naming tokens, exactly as the
// fixture set is enumerated rather than named, so it covers the whole vocabulary
// and the harness contract's panel fixtures enrol with no edit here.
func TestThemeSwapGuard_EveryTokenExercisedByAFixture(t *testing.T) {
	a, b := syntheticPalettes()
	bForms := tokenForms(t, b)
	frames := swappedFrames(guardedFixtures(t), a, b)

	t.Run("every token in the vocabulary is rendered by some fixture", func(t *testing.T) {
		for _, name := range uncoveredTokens(coveredTokens(frames, bForms)) {
			t.Errorf("token %s renders on no included fixture, so it is absent from BOTH of assertion 2's unions, they balance, and nothing reports it — ADD A FIXTURE that renders %s. Do NOT exempt the token: an exemption is the permanent render-layer carve-out this guard exists to catch (§9.11). Do NOT weaken this to the tokens observed under theme A: that is assertion 2, and its A/B balance is exactly what hides the gap", name, name)
		}
	})

	// The non-vacuity proof. Every check above is a range over a gap list, so a
	// coverage scan that credited every token — or a gap list that named none —
	// would pass green with nothing exercised. Narrowing the set to one fixture
	// must therefore leave gaps: it is what shows the assertion reports a real
	// absence rather than being incapable of reporting one.
	//
	// It narrows the ALREADY-RENDERED frames rather than re-rendering, so the
	// negative control costs nothing and cannot drift from the set above.
	t.Run("a narrowed fixture set reports the tokens it drops", func(t *testing.T) {
		narrowed := frames[:1]
		if len(uncoveredTokens(coveredTokens(narrowed, bForms))) == 0 {
			t.Errorf("fixture %s alone renders all %d tokens, so narrowing to it demonstrates nothing about whether this assertion can report a gap; re-point the narrowing at a set that genuinely drops one", narrowed[0].fixture, len(theme.TokenNames()))
		}
	})
}

// TestThemeSwapGuard_ViewBackgroundColourFollowsSwap covers the one themed value
// the frame scan structurally cannot see.
//
// View().BackgroundColor is the DECLARATIVE per-frame background — Bubble
// Tea diffs it and emits OSC 11 only on change — and it is not part of Content, so
// no amount of scanning the frame string would notice it holding the old canvas.
func TestThemeSwapGuard_ViewBackgroundColourFollowsSwap(t *testing.T) {
	a, b := syntheticPalettes()
	for _, fx := range guardedFixtures(t) {
		t.Run(fx.Name(), func(t *testing.T) {
			got := swapModel(fx, a, b).View().BackgroundColor
			if !sameColour(got, b.Canvas.Color()) {
				t.Errorf("View().BackgroundColor = %v after the swap, want theme B's canvas %s — the declarative frame background did not follow the swap", got, b.Canvas.Value)
			}
		})
	}
}

// stubFixture is a locally-constructed fixture standing in for one the registry
// does not contain: no registered fixture is colourless today, so the exclusion's
// true branch has no other way to be driven.
type stubFixture struct {
	name       string
	colourless bool
	// after is the canned POST-SWAP frame the stub hands back, so the coverage
	// scan can be driven over it. Left empty by callers that only exercise the
	// exclusion itself.
	after string
}

func (f stubFixture) Colourless() bool { return f.colourless }
func (f stubFixture) Name() string     { return f.name }

// RenderSwapRender hands back the canned frame as the post-swap half, so
// stubFixture satisfies swappableFixture. It renders nothing, so the palettes and
// the size are ignored, and the before-frame is empty — coveredTokens reads only
// the after-frame.
func (f stubFixture) RenderSwapRender(_, _ theme.Theme, _, _ int) (before, after string) {
	return "", f.after
}

// TestThemeSwapGuard_ExcludesColourlessFixtures drives the exclusion the guard
// filters its enumerated set through, over locally-constructed fixtures rather
// than registry names.
func TestThemeSwapGuard_ExcludesColourlessFixtures(t *testing.T) {
	coloured := stubFixture{name: "coloured"}
	colourless := stubFixture{name: "colourless", colourless: true}

	kept := excludeColourless([]stubFixture{coloured, colourless})
	if !slices.Equal(kept, []stubFixture{coloured}) {
		t.Errorf("excludeColourless kept %v, want only the coloured fixture — a colourless render carries no theme colours, so there is nothing to diff", kept)
	}
}

// foregroundOnlyForms / backgroundOnlyForms narrow every form to ONE rendered
// side, by duplicating that side into both fields — so a scan through
// observedTokens can only match that side, while observedTokens itself stays the
// file's single definition of "observed".
//
// They exist to MEASURE which side each token is actually found by, which is the
// evidence that observedTokens' OR is load-bearing rather than defensive. No
// assertion of the guard uses them.
func foregroundOnlyForms(forms []tokenForm) []tokenForm {
	narrowed := make([]tokenForm, 0, len(forms))
	for _, form := range forms {
		narrowed = append(narrowed, tokenForm{name: form.name, fg: form.fg, bg: form.fg})
	}
	return narrowed
}

func backgroundOnlyForms(forms []tokenForm) []tokenForm {
	narrowed := make([]tokenForm, 0, len(forms))
	for _, form := range forms {
		narrowed = append(narrowed, tokenForm{name: form.name, fg: form.bg, bg: form.bg})
	}
	return narrowed
}

// TestTokenCoverage_MatchesBackgroundForm pins the half of observedTokens' OR a
// foreground-only scan would lose.
//
// MEASURED over the guarded set at harnessWidth × harnessHeight: canvas,
// bg.selection and bg.attention are found by NO foreground run on ANY fixture.
// Scanning foregrounds alone would report all three missing, and assertion 3
// would then demand fixtures for tokens already rendered on 17/17, 13/17 and
// 2/17 fixtures respectively.
//
// bg.subtle is the exception in the table below and is NOT evidence for the OR.
// Its locus is the loading bar's empty track, which sets it as the foreground AND
// the background of the same glyph run (internal/tui/loading_view.go), so it is
// found either way. It is listed because the role contract groups it with the surfaces and the
// reasonable expectation is that a surface tint renders only as a background —
// an expectation the measurement contradicts, which is worth an assertion rather
// than a comment.
func TestTokenCoverage_MatchesBackgroundForm(t *testing.T) {
	a, b := syntheticPalettes()
	bForms := tokenForms(t, b)
	frames := swappedFrames(guardedFixtures(t), a, b)

	byForeground := coveredTokens(frames, foregroundOnlyForms(bForms))
	byBackground := coveredTokens(frames, backgroundOnlyForms(bForms))

	for _, tc := range []struct {
		token string
		// exclusivelyBackground is whether NO fixture renders the token as a
		// foreground — true for the three that evidence the OR, false for
		// bg.subtle, which renders both ways.
		exclusivelyBackground bool
	}{
		{token: "canvas", exclusivelyBackground: true},
		{token: "bg.selection", exclusivelyBackground: true},
		{token: "bg.attention", exclusivelyBackground: true},
		{token: "bg.subtle", exclusivelyBackground: false},
	} {
		t.Run(tc.token, func(t *testing.T) {
			if len(byBackground[tc.token]) == 0 {
				t.Errorf("no fixture renders %s as a background, so the background half of observedTokens' match finds it nowhere", tc.token)
			}
			foundByForeground := len(byForeground[tc.token]) > 0
			if tc.exclusivelyBackground && foundByForeground {
				t.Errorf("%s is now found by a FOREGROUND run too (on %v); it was measured background-only, so this row no longer evidences that observedTokens' OR is load-bearing — re-measure and re-record", tc.token, byForeground[tc.token])
			}
			if !tc.exclusivelyBackground && !foundByForeground {
				t.Errorf("%s is no longer found by a foreground run anywhere; it was measured as rendering BOTH ways (the loading bar's track paints it as foreground and background alike), so this row's exception is stale", tc.token)
			}
		})
	}
}

// TestTokenCoverage_MatchesForegroundForm pins the other half.
//
// MEASURED over the guarded set: border is found by a FOREGROUND run and by no
// background run on any fixture. Its measured loci here are the title rule and
// the footer rule; the role contract also gives it modal frames and edit-modal chips, which no
// guarded fixture renders today — cited, not measured. Scanning backgrounds alone
// would report it missing on every screen that draws a rule.
func TestTokenCoverage_MatchesForegroundForm(t *testing.T) {
	a, b := syntheticPalettes()
	bForms := tokenForms(t, b)
	frames := swappedFrames(guardedFixtures(t), a, b)

	const token = "border"
	byForeground := coveredTokens(frames, foregroundOnlyForms(bForms))
	byBackground := coveredTokens(frames, backgroundOnlyForms(bForms))

	if len(byForeground[token]) == 0 {
		t.Errorf("no fixture renders %s as a foreground, so the foreground half of observedTokens' match finds it nowhere", token)
	}
	if got := byBackground[token]; len(got) > 0 {
		t.Errorf("%s is now found by a BACKGROUND run too (on %v); it was measured foreground-only, so this test no longer evidences that observedTokens' OR is load-bearing — re-measure and re-record", token, got)
	}
}

// TestTokenCoverage_IgnoresExcludedFixtures pins the completeness guard's "colourless fixtures
// are excluded" as it bears on COVERAGE specifically: a token carried only by an
// excluded fixture must still count as uncovered.
//
// The exclusion has opposite consequences for the two kinds of assertion.
// Dropping a colourless fixture from assertions 1 and 2 merely removes a frame
// with nothing to diff. Dropping it from assertion 3 must not quietly credit it
// with coverage: a colourless render carries no theme hexes at all, so "covered
// there" means covered NOWHERE and the token still needs a real fixture.
//
// It drives the same two steps in the same order the guard does —
// excludeColourless, then coveredTokens — over stand-in fixtures with canned
// frames, because no registry fixture is colourless and the true branch has no
// other way to be reached. The excluded stub's frame deliberately DOES carry a
// token's run, which a real colourless render never would: that is what makes the
// resulting gap attributable to the filter rather than to an empty frame.
func TestTokenCoverage_IgnoresExcludedFixtures(t *testing.T) {
	a, b := syntheticPalettes()
	forms := tokenForms(t, b)
	// Taken by position: any two distinct tokens do, since what is under test is
	// the filter rather than which token happens to be carried.
	included, excluded := forms[0], forms[1]

	fixtures := []stubFixture{
		{name: "coloured", after: frameCarrying(included.fg)},
		{name: "colourless", colourless: true, after: frameCarrying(excluded.bg)},
	}
	loci := coveredTokens(swappedFrames(excludeColourless(fixtures), a, b), forms)

	if got := loci[included.name]; !slices.Equal(got, []string{"coloured"}) {
		t.Errorf("token %s has loci %v, want only the included fixture — the scan did not read the included frame", included.name, got)
	}
	if got := loci[excluded.name]; len(got) > 0 {
		t.Errorf("token %s is credited to %v, but its only carrier is COLOURLESS — a colourless render carries no theme colour, so covering it there covers it nowhere", excluded.name, got)
	}
	if gaps := uncoveredTokens(loci); !slices.Contains(gaps, excluded.name) {
		t.Errorf("uncoveredTokens reports %v, which omits %s — a token carried only by an excluded fixture must still be reported as needing one", gaps, excluded.name)
	}
}

// frameCarrying returns a canned frame carrying one SGR parameter run, terminated
// by `m` exactly as a real render terminates it, so carriesRun matches it.
func frameCarrying(run string) string {
	return "\x1b[" + run + "mx\x1b[0m"
}

// TestTokenCoverage_TransientStatesHaveALocus records the NAMED locus of each
// at-risk token: the completeness guard's transient states (bg.attention / text.on-attention,
// accent.mode, state.destructive, text.on-selection) plus text.subtle and
// bg.subtle, whose only loci are on screens no flat Sessions fixture reaches.
//
// Assertion 3 proves each is rendered somewhere; this records WHERE. Without it
// the fixture carrying a token could be deleted or re-seeded and assertion 3
// would report only "text.on-attention renders on no included fixture" — true,
// but silent about which edit caused it.
//
// EVERY ROW IS MEASURED rather than read off the role table: that table names
// where a token is MEANT to render, which is a different claim from where the
// fixture set actually renders it. Where a token has several loci, the row
// records the one the role table makes its canonical home. text.on-attention is the
// thinnest edge of the set — the role table gives it exactly one role, the warning-flash
// message, so only a fixture seeding that flash can carry it at all.
func TestTokenCoverage_TransientStatesHaveALocus(t *testing.T) {
	a, b := syntheticPalettes()
	frames := swappedFrames(guardedFixtures(t), a, b)
	loci := coveredTokens(frames, tokenForms(t, b))

	for _, tc := range []struct{ token, fixture string }{
		{token: "bg.attention", fixture: "sessions-inline-flash"},
		{token: "text.on-attention", fixture: "sessions-inline-flash"},
		{token: "accent.mode", fixture: "preview-screen"},
		{token: "state.destructive", fixture: "loading-error"},
		{token: "text.on-selection", fixture: "sessions-flat"},
		{token: "text.subtle", fixture: "sessions-by-project"},
		{token: "bg.subtle", fixture: "loading-screen"},
	} {
		t.Run(tc.token, func(t *testing.T) {
			if !slices.Contains(loci[tc.token], tc.fixture) {
				t.Errorf("%s is not rendered by %s — its loci are %v. That fixture was this token's recorded locus, so either restore what rendered it there or record the fixture that carries it now", tc.token, tc.fixture, loci[tc.token])
			}
		})
	}
}
