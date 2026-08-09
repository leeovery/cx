package capture

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
	"github.com/leeovery/portal/internal/tui"
)

// §13.3's FOUR panel-fixture inputs, and the no-I/O fake that turns three of
// them into a §9.4 union the panel can list.
//
// These tests are IN-PACKAGE because the inputs are declared fields and the fake
// is unexported: what they pin is that a fixture DECLARES its panel and that the
// declaration reaches the model, which is a statement about the catalogue rather
// than about a frame. The frame-level assertions (badges, cursor, palette) live
// beside them in package capture_test.
//
// No t.Parallel() anywhere — the project bans it outright.

// panelLeftBorder is §9.1's left-border-only glyph, the one column marking where
// the slide-over starts on every row of a frame. It is a literal because
// internal/tui keeps its own copy unexported.
const panelLeftBorder = "│"

// panelFixtureNames is EVERY §13.3 panel fixture, DERIVED FROM THE REGISTRY so a
// fixture added to the catalogue is covered by the assertions below without being
// enrolled a second time by hand.
//
// It is the whole set rather than the two setting-state frames the four-input
// check was first written against: a reader reasonably assumes the structural
// check spans the catalogue, and a fixture added to the catalogue without its four
// inputs is precisely the failure that check exists to name.
func panelFixtureNames() []string {
	var names []string
	for _, name := range FixtureNames() {
		if strings.HasPrefix(name, panelFixturePrefix) {
			names = append(names, name)
		}
	}
	return names
}

// panelFixturePrefix is what every §13.3 panel fixture's registered name begins
// with, and the whole of what distinguishes one from the picker fixtures beside
// it in the registry.
const panelFixturePrefix = "theme-panel-"

// TestPanelFixture_FourInputs: it declares all four panel inputs.
//
// §13.3 names four and calls the fourth PREVIOUSLY UNSTATED: the `--theme`
// palette, the raw persisted keys, the faked ThemeEnumerator's row set, and the
// CURSOR POSITION. The palette arrives per render (Deps takes it), so what a
// fixture can declare for itself is the other three — plus the enumeration and
// the slot records that feed the fake.
//
// Without the cursor input the mandated constant-while-previewing frame is
// unreachable: its whole point is a cursor on a row other than the marked one,
// otherwise reachable only by arrowing, and fixtures are one-shot renders.
func TestPanelFixture_FourInputs(t *testing.T) {
	for _, name := range panelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			fx, err := FixtureByName(name)
			if err != nil {
				t.Fatalf("FixtureByName(%s): %v", name, err)
			}

			if (fx.themeKeys == theme.RawKeys{}) {
				t.Error("input 2: the fixture declares no raw persisted theme keys, so its badges have nothing to derive from")
			}
			if len(fx.themeUnion.Rows) == 0 {
				t.Error("input 3a: the fixture declares no union rows, so the panel would list nothing")
			}
			if len(fx.themeSlots) == 0 {
				t.Error("input 3b: the fixture declares no slot resolutions, so no row carries a ● at all")
			}
			if fx.initialThemeCursor == "" {
				t.Error("input 4: the fixture declares no cursor row; §13.3 makes the cursor position a declared input precisely because a one-shot render cannot arrow to it")
			}

			// The palette is input 1 and arrives per render, so the declaration is
			// only complete once it reaches the model's seam set.
			deps := fx.Deps(theme.Theme{})
			if deps.ThemeKeys != fx.themeKeys {
				t.Errorf("Deps().ThemeKeys = %+v, want the declared %+v", deps.ThemeKeys, fx.themeKeys)
			}
			if deps.Capture.ThemeCursor != fx.initialThemeCursor {
				t.Errorf("Deps().Capture.ThemeCursor = %q, want the declared %q", deps.Capture.ThemeCursor, fx.initialThemeCursor)
			}
			if deps.ThemeEnumerator == nil {
				t.Fatal("Deps() wires no ThemeEnumerator, so `t` is a silent no-op and the fixture renders no panel at all")
			}

			enumeration, union := deps.ThemeEnumerator.Open(fx.themeKeys)
			if got, want := rowSortKeys(union.Rows), rowSortKeys(fx.themeUnion.Rows); !slices.Equal(got, want) {
				t.Errorf("the seam's union lists %v, want the declared %v", got, want)
			}
			if got, want := enumeration.DirPath, fx.themeEnumeration.DirPath; got != want {
				t.Errorf("the seam's enumeration is of %q, want the declared %q", got, want)
			}
		})
	}
}

// rowSortKeys is a union's rows by identity, for comparing a seam's answer
// against a fixture's declaration without comparing palettes (which the fake
// deliberately rewrites — see newFakeThemeEnumerator).
func rowSortKeys(rows []theme.Row) []string {
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.SortKey())
	}
	return keys
}

// TestFakeThemeEnumerator_ResolveReportsTheInjectedPalette: it reports the
// injected palette.
//
// THE WHOLE POINT IS THAT THE OPEN-TIME ApplyTheme IS A NO-OP. Task 8-8's open
// applies the theme its Resolve return names, BEFORE the cursor seed runs. So a
// fake reporting anything other than the palette the model's nomination carries
// REPAINTS the frame — and three distinct failures follow, none of them loud:
//
//   - A ZERO resolution paints the whole panel through lipgloss.Color("")'s
//     no-colour sentinel: silently colourless, no compile error, no failing
//     assertion.
//   - A HARD-CODED BUILT-IN makes `--theme` inert on precisely the frames a
//     drop-in author most wants to check, and contradicts the coherence rule the
//     fixtures' own doc comments state.
//   - Inside §13.4's swap-and-diff guard the same apply OVERWRITES the synthetic
//     theme ModelAt was handed, so a panel fixture contributes neither an A value
//     nor a B value: assertion 1 passes as a vacuous negative, assertion 2's union
//     balances, and the panel's bubbles/list instance reads as covered while being
//     covered by nothing.
func TestFakeThemeEnumerator_ResolveReportsTheInjectedPalette(t *testing.T) {
	injected := themetest.Builtin(t, theme.DefaultDarkSlug)
	// The declared slots carry a DIFFERENT palette, so an implementation passing
	// them through untouched fails rather than agreeing by coincidence.
	declared := []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: "tokyo-night-day", Resolved: "tokyo-night-day", Theme: themetest.Builtin(t, theme.DefaultLightSlug)},
		{Slot: theme.SlotDark, Requested: "nord", Resolved: "nord", Theme: themetest.Builtin(t, "nord")},
	}
	fake := newFakeThemeEnumerator(injected, theme.Enumeration{}, theme.Union{}, declared)

	resolution, err := fake.Resolve(theme.Enumeration{}, theme.Setting{})
	if err != nil {
		t.Fatalf("Resolve returned %v; the fake has nothing that can fail", err)
	}

	t.Run("every slot reports the injected palette", func(t *testing.T) {
		if len(resolution.Slots) != len(declared) {
			t.Fatalf("Resolve returned %d slot(s), want the declared %d", len(resolution.Slots), len(declared))
		}
		for i, slot := range resolution.Slots {
			if slot.Slot != declared[i].Slot || slot.Requested != declared[i].Requested || slot.Resolved != declared[i].Resolved {
				t.Errorf("slot %d = %+v, want the declared identity %+v", i, slot, declared[i])
			}
			if slot.Theme != injected {
				t.Errorf("slot %d reports canvas %q, want the injected %q — the open-time ApplyTheme would repaint the frame off `--theme`", i, slot.Theme.Canvas.Value, injected.Canvas.Value)
			}
		}
	})

	t.Run("the nomination reports the injected palette", func(t *testing.T) {
		if !resolution.Nomination.IsConstant() {
			t.Fatal("Resolve's nomination is not the constant shape capturetool pins (§13.3)")
		}
		if active := resolution.Nomination.Constant(); active != injected {
			t.Errorf("the nomination carries canvas %q, want the injected %q", active.Canvas.Value, injected.Canvas.Value)
		}
	})

	// The non-vacuity control. A fake reporting a DIFFERENT palette genuinely
	// repaints the frame, which is what makes "reports the injected palette"
	// load-bearing rather than an assertion about an inert value.
	t.Run("a fake reporting another palette repaints the frame", func(t *testing.T) {
		pinned := themetest.Builtin(t, "nord")
		other := themetest.Builtin(t, theme.DefaultLightSlug)

		frame := driveToPanel(t, panelDepsReporting(t, "theme-panel-adaptive-pair", pinned, other))
		if !strings.Contains(frame, backgroundSGR(t, other.Canvas)) {
			t.Error("a fake reporting another palette did NOT repaint the frame, so the injected-palette requirement asserts nothing")
		}
	})

	// The named hazard, driven rather than described: a fake reporting a ZERO
	// palette resolves every panel surface through lipgloss.Color("")'s no-colour
	// sentinel. Nothing errors, nothing fails to compile, and the frame simply
	// stops carrying colour — which is why the requirement is stated in the fake's
	// own doc comment rather than left to be inferred.
	t.Run("a fake reporting a zero palette renders colourless", func(t *testing.T) {
		pinned := themetest.Builtin(t, "nord")

		frame := driveToPanel(t, panelDepsReporting(t, "theme-panel-adaptive-pair", pinned, theme.Theme{}))
		if strings.Contains(frame, backgroundSGR(t, pinned.Canvas)) {
			t.Error("the frame still carries `--theme`'s canvas after a zero-palette apply; the hazard is not being driven")
		}
		if strings.Contains(frame, truecolorBackground) {
			t.Errorf("the frame carries a 24-bit background after a zero-palette apply, so the colourless outcome this warns about is not what happens; re-derive the warning:\n%s", ansi.Strip(frame))
		}
	})
}

// TestFakeThemeEnumerator_RowsCarryTheInjectedPalette: its union rows carry the
// injected palette too.
//
// This is the fake's FOURTH reason for reporting the injected palette, and the one
// belonging to the live view rather than to a still: §9.2's arrow-preview applies
// THE CURSOR ROW'S OWN palette, so a row left with a zero Theme paints the whole
// frame through lipgloss.Color("")'s no-colour sentinel the moment anyone presses
// `↓` in `go run ./cmd/capturetool --fixture …` — which §13.1 makes the human's
// route at the visual gate, the one audience a PNG cannot serve.
//
// IT IS DRIVEN RATHER THAN DESCRIBED because nothing else in this suite would
// notice. Row.Theme has exactly one consumer, this preview, and no still frame
// reads it: with the row repaint removed, every capture, every badge assertion and
// §13.4's whole guard stay green while the live view goes colourless on the first
// arrow key. That is the same silent-hazard shape as the sibling Resolve case, and
// it is held to the same standard.
func TestFakeThemeEnumerator_RowsCarryTheInjectedPalette(t *testing.T) {
	t.Run("arrowing to the next row keeps the frame on the injected palette", func(t *testing.T) {
		pinned := themetest.Builtin(t, "nord")

		fx, err := FixtureByName("theme-panel-adaptive-pair")
		if err != nil {
			t.Fatalf("FixtureByName: %v", err)
		}

		model := panelModel(t, fx.Deps(pinned))
		opened := frameOf(t, model)

		// The panel is OPEN before the arrow is sent. Without this the diff below
		// proves only that the FRAME changed: with the open short-circuited, `↓` moves
		// the Sessions cursor instead and the frame changes anyway, so the leg would
		// pass over a panel that never rendered.
		if !strings.Contains(ansi.Strip(opened), panelLeftBorder) {
			t.Fatalf("the frame carries no %q, so the panel never opened and the `↓` below cannot reach it:\n%s", panelLeftBorder, ansi.Strip(opened))
		}

		model, _ = model.Update(tea.KeyPressMsg{Code: tea.KeyDown})
		previewed := frameOf(t, model)

		// The non-vacuity leg. An `↓` that never reached the panel — an unbound key,
		// a cursor already at the end, a panel that never opened — leaves the frame
		// untouched, and the palette leg below would then pass over a preview that
		// never happened.
		if opened == previewed {
			t.Fatal("the `↓` changed nothing on the frame; the panel cursor never moved, so this asserts nothing about what a preview applies")
		}
		// Which is also the not-colourless leg: a canvas SGR is a 24-bit background
		// run, so a frame painted from a zero palette carries none of it.
		if !strings.Contains(previewed, backgroundSGR(t, pinned.Canvas)) {
			t.Errorf("one `↓` left the frame without `--theme`'s canvas %s; the previewed row's palette is not the injected one, and the live view is colourless from the first arrow key:\n%s", pinned.Canvas.Value, ansi.Strip(previewed))
		}
	})

	// The carve-out repaintUnion states, which no fixture's union can exercise
	// today: theme.Row populates Theme IFF Rejection is nil, so a rejected row
	// coming back with a palette would be a shape the real assembly cannot produce
	// — and §13.3's mandated invalid-row and `⚠ dir unreadable` fixtures are built
	// from precisely that row.
	t.Run("a rejected row keeps its rejection and takes no palette", func(t *testing.T) {
		injected := themetest.Builtin(t, "nord")
		rejection := &theme.Rejection{Reason: theme.ReasonBadSyntax, Detail: "line 4: quoted value"}
		declared := theme.Union{Rows: []theme.Row{
			{Slug: "valid-drop-in", Filename: "valid-drop-in.theme", Source: theme.SourceFile},
			{Filename: "broken.theme", Source: theme.SourceFile, Rejection: rejection},
		}}

		_, union := newFakeThemeEnumerator(injected, theme.Enumeration{}, declared, nil).Open(theme.RawKeys{})
		if len(union.Rows) != len(declared.Rows) {
			t.Fatalf("the fake lists %d row(s), want the declared %d", len(union.Rows), len(declared.Rows))
		}

		if got := union.Rows[0].Theme; got != injected {
			t.Errorf("the selectable row carries canvas %q, want the injected %q", got.Canvas.Value, injected.Canvas.Value)
		}
		if got := union.Rows[1].Theme; got != (theme.Theme{}) {
			t.Errorf("the rejected row carries canvas %q, want none — Theme is populated IFF Rejection is nil, and a half-populated rejected row is a shape the real assembly never returns", got.Canvas.Value)
		}
		if union.Rows[1].Rejection != rejection {
			t.Error("the rejected row's rejection was rewritten; the fake repaints palettes and touches nothing else")
		}
	})
}

// panelDepsReporting builds a panel fixture's seam set painted from pinned, with
// its fake enumerator deliberately REPORTING a different palette — the only way
// to drive what happens when the two disagree, since Deps assembles them from one
// value precisely so they cannot.
func panelDepsReporting(t *testing.T, fixture string, pinned, reported theme.Theme) tui.Deps {
	t.Helper()

	fx, err := FixtureByName(fixture)
	if err != nil {
		t.Fatalf("FixtureByName(%s): %v", fixture, err)
	}
	deps := fx.Deps(pinned)
	deps.ThemeEnumerator = newFakeThemeEnumerator(reported, fx.themeEnumeration, fx.themeUnion, fx.themeSlots)
	return deps
}

// truecolorBackground is the SGR parameter prefix a 24-bit background opens with.
// A frame carrying none has no theme background anywhere on it.
const truecolorBackground = "48;2;"

// TestFakeThemeEnumerator_ResolveIsTheOnlyBadgeSource pins the other half of the
// Resolve contract: the DECLARED slots are what the `●` comes from, and a fixture
// declaring none renders no badge on any row.
//
// It is worth an assertion of its own because the failure is entirely silent.
// Task 8-8 retired the injected slot record, so the seam's Resolve return is now
// the panel's only badge source — a fixture that declared its slots anywhere else
// would list every row correctly, mark nothing, and look like a panel on an
// install with no theme set. On the adaptive-pair frame that is the loss of the
// entire subject: §9.14 makes those two badges the reference for a vocabulary
// with no prior art anywhere.
func TestFakeThemeEnumerator_ResolveIsTheOnlyBadgeSource(t *testing.T) {
	pinned := themetest.Builtin(t, "nord")

	fx, err := FixtureByName("theme-panel-adaptive-pair")
	if err != nil {
		t.Fatalf("FixtureByName: %v", err)
	}

	t.Run("the declared slots put a badge on their rows", func(t *testing.T) {
		for _, badge := range panelBadges(t, driveToPanel(t, fx.Deps(pinned))) {
			if badge != "" {
				return
			}
		}
		t.Error("no panel row carries a badge although the fixture declares two slots; the badge source is not the seam's Resolve return")
	})

	t.Run("declaring no slots renders no badge on any row", func(t *testing.T) {
		deps := fx.Deps(pinned)
		deps.ThemeEnumerator = newFakeThemeEnumerator(pinned, fx.themeEnumeration, fx.themeUnion, nil)

		for i, badge := range panelBadges(t, driveToPanel(t, deps)) {
			if badge != "" {
				t.Errorf("panel row %d carries badge %q although no slot was declared; something other than Resolve is marking rows", i, badge)
			}
		}
	})
}

// panelBadges is the badge text of every rendered panel row, in frame order —
// empty for a row carrying none.
//
// It reads the PANEL side of each line (everything past the slide-over's one
// left-border column) rather than the whole frame, because the Sessions list
// behind it renders `● attached` on its own rows: a frame-wide scan for the glyph
// would find one on every fixture built from the shared session set.
func panelBadges(t *testing.T, frame string) []string {
	t.Helper()

	var badges []string
	for line := range strings.SplitSeq(ansi.Strip(frame), "\n") {
		_, text, onPanel := strings.Cut(line, panelLeftBorder)
		if !onPanel || strings.TrimSpace(text) == "" {
			continue
		}
		badges = append(badges, badgeIn(text))
	}
	if len(badges) == 0 {
		t.Fatalf("no panel rows were found in the frame; the slide-over did not render:\n%s", ansi.Strip(frame))
	}
	return badges
}

// badgeIn is the §9.5 `●` badge a panel row's text carries, from the glyph to the
// end of the row — or "" where the row carries none.
func badgeIn(text string) string {
	at := strings.Index(text, "●")
	if at < 0 {
		return ""
	}
	return strings.TrimSpace(text[at:])
}

// TestFakeThemeEnumerator_NoIO: its fake enumerator does no I/O.
//
// §7.1's no-real-config import guard forbids internal/capture reaching config at
// all, and §13.3 rests the whole panel-fixture route on the seam being fakeable
// wholesale — so the fake must answer from declared values on every one of its
// four methods.
//
// It is checked STRUCTURALLY and behaviourally, because neither alone is enough.
// A behavioural check can only prove the fake did not read the directories the
// test thought to poison; the structural scans prove it cannot read any.
//
// THE STRUCTURAL HALF IS TWO SCANS, NOT ONE. internal/theme is the one package
// the fake MUST import — it answers in theme.Theme / theme.Union /
// theme.Resolution values — so the import ban structurally cannot name it, and
// theme.Loader, the only I/O-capable type the package declares, would walk past a
// green import scan in silence. Closing it takes a scan of the fake's own FIELDS.
func TestFakeThemeEnumerator_NoIO(t *testing.T) {
	const source = "theme_fake.go"
	file := parseFakeSource(t, source)

	// The non-vacuity guard on BOTH structural scans: a fake renamed out from under
	// them leaves each one passing green over nothing.
	fake, ok := fakeThemeEnumeratorStruct(file)
	if !ok {
		t.Fatalf("%s declares no fakeThemeEnumerator struct, so scanning it proves nothing", source)
	}

	t.Run("the fake's source imports nothing that can touch the filesystem", func(t *testing.T) {
		// The I/O-capable stdlib packages a fake could reach for.
		banned := []string{"os", "io", "io/fs", "io/ioutil", "path/filepath", "embed", "os/exec", "net"}
		for _, spec := range file.Imports {
			path := strings.Trim(spec.Path.Value, `"`)
			if slices.Contains(banned, path) {
				t.Errorf("%s imports %q; the fake must answer from declared values with no file or directory access", source, path)
			}
		}
	})

	t.Run("the fake holds no theme.Loader", func(t *testing.T) {
		if field, held := loaderFieldIn(fake, themePackageName(file)); held {
			t.Errorf("%s's fakeThemeEnumerator declares field %q, a theme.Loader — the one I/O-capable type in internal/theme, and the one the import scan above cannot ban. The fake must hold no loader, resolve no path and read no file", source, field)
		}
	})

	// The non-vacuity control on that scan, which is a NEGATIVE: a scan looking for
	// the wrong shape — or for nothing at all — passes it exactly as readily as a
	// fake that genuinely holds no loader. This is the same "absence reads as
	// coverage" failure the assertion itself exists to close, one level up.
	t.Run("the loader scan reports one that is there", func(t *testing.T) {
		control := parseControlSource(t, loaderHoldingFakeSource)
		fake, ok := fakeThemeEnumeratorStruct(control)
		if !ok {
			t.Fatal("the control source declares no fakeThemeEnumerator struct, so it controls nothing")
		}
		field, held := loaderFieldIn(fake, themePackageName(control))
		if !held {
			t.Fatal("the scan reports no loader on a fake that holds one behind a pointer and an import alias; it would pass over theme_fake.go whatever that file held")
		}
		if want := "loader"; field != want {
			t.Errorf("the scan names field %q, want %q — it is matching some other field", field, want)
		}
	})

	t.Run("every method answers from declared values with the config paths poisoned", func(t *testing.T) {
		// A directory that does not exist and an unreadable prefs path: any method
		// that consulted either would fail or answer differently.
		missing := filepath.Join(t.TempDir(), "no-such-themes-dir")
		t.Setenv("PORTAL_THEMES_DIR", missing)
		t.Setenv("XDG_CONFIG_HOME", missing)
		t.Setenv("PORTAL_PREFS_FILE", filepath.Join(missing, "prefs.json"))

		fx, err := FixtureByName("theme-panel-adaptive-pair")
		if err != nil {
			t.Fatalf("FixtureByName: %v", err)
		}
		palette := themetest.Builtin(t, "nord")
		seam := fx.Deps(palette).ThemeEnumerator

		enumeration, union := seam.Open(fx.themeKeys)
		if got, want := rowSortKeys(union.Rows), rowSortKeys(fx.themeUnion.Rows); !slices.Equal(got, want) {
			t.Errorf("Open listed %v, want the declared %v", got, want)
		}
		if got, want := rowSortKeys(seam.Reassemble(enumeration, fx.themeKeys).Rows), rowSortKeys(fx.themeUnion.Rows); !slices.Equal(got, want) {
			t.Errorf("Reassemble listed %v, want the declared %v", got, want)
		}
		resolution, err := seam.Resolve(enumeration, theme.Setting{})
		if err != nil {
			t.Fatalf("Resolve returned %v", err)
		}
		if len(resolution.Slots) != len(fx.themeSlots) {
			t.Errorf("Resolve returned %d slot(s), want the declared %d", len(resolution.Slots), len(fx.themeSlots))
		}
		// ResolveSlot is unreachable in a capture (a fixture wires no theme
		// persister), so this is the only pin on it answering like its three
		// siblings: the slot and slug it was asked for, carrying the injected
		// palette, off no directory read.
		res, err := seam.ResolveSlot(enumeration, theme.SlotLight, "nord")
		if err != nil {
			t.Fatalf("ResolveSlot returned %v", err)
		}
		if res.Slot != theme.SlotLight || res.Requested != "nord" || res.Resolved != "nord" {
			t.Errorf("ResolveSlot answered slot=%v requested=%q resolved=%q, want the light slot and %q on both slugs", res.Slot, res.Requested, res.Resolved, "nord")
		}
		if res.Theme != palette {
			t.Errorf("ResolveSlot answered canvas %q, want the injected palette's %q — the fourth method must report the same palette as the other three", res.Theme.Canvas.Value, palette.Canvas.Value)
		}
		if _, err := os.Stat(missing); !os.IsNotExist(err) {
			t.Errorf("the poisoned themes directory %s now exists (%v); something on the render path created it", missing, err)
		}
	})
}

// loaderHoldingFakeSource is a fake that DOES hold a loader, in the two shapes
// the field scan must see through: behind a POINTER, and behind an import ALIAS
// — so the scan is pinned to internal/theme's import path rather than to the
// literal identifier `theme`, and a decoy field beside it cannot be what it
// matched.
const loaderHoldingFakeSource = `package capture

import palette "github.com/leeovery/portal/internal/theme"

type fakeThemeEnumerator struct {
	slots  []palette.SlotResolution
	loader *palette.Loader
}
`

// parseFakeSource parses one of this package's own source files.
func parseFakeSource(t *testing.T, source string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), source, nil, 0)
	if err != nil {
		t.Fatalf("parse %s: %v", source, err)
	}
	return file
}

// parseControlSource parses a source LITERAL — the control fake above, which
// exists only in this file and so has no path to be read from.
func parseControlSource(t *testing.T, src string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "control_fake.go", src, 0)
	if err != nil {
		t.Fatalf("parse the control source: %v", err)
	}
	return file
}

// fakeThemeEnumeratorStruct resolves the fake's declared struct type out of a
// parsed file.
//
// The false return is what makes the two structural scans non-vacuous: a file
// that had been renamed out from under them declares no such type, and both would
// otherwise pass green over nothing.
func fakeThemeEnumeratorStruct(file *ast.File) (*ast.StructType, bool) {
	var found *ast.StructType
	ast.Inspect(file, func(n ast.Node) bool {
		spec, ok := n.(*ast.TypeSpec)
		if !ok || spec.Name.Name != "fakeThemeEnumerator" {
			return true
		}
		if declared, ok := spec.Type.(*ast.StructType); ok {
			found = declared
		}
		return false
	})
	return found, found != nil
}

// themePackageName is the local name internal/theme is bound to in the parsed
// file — "" when the file does not import it at all, in which case nothing in it
// can spell theme.Loader.
//
// It is resolved from the IMPORT PATH rather than assumed to be `theme`, so an
// aliased import cannot walk a loader past the scan below.
func themePackageName(file *ast.File) string {
	const path = `"github.com/leeovery/portal/internal/theme"`
	for _, spec := range file.Imports {
		if spec.Path.Value != path {
			continue
		}
		if spec.Name != nil {
			return spec.Name.Name
		}
		return "theme"
	}
	return ""
}

// loaderFieldIn is the name of the first field of the parsed struct whose TYPE
// mentions <pkg>.Loader, and whether there was one.
//
// It walks each field's whole type expression rather than matching two literal
// shapes, so a loader behind a pointer, a slice, a map or a func result is found
// as readily as a bare one. An embedded field's name IS its type's name, which is
// why that case reports "Loader".
func loaderFieldIn(fake *ast.StructType, pkg string) (string, bool) {
	if pkg == "" {
		return "", false
	}
	for _, field := range fake.Fields.List {
		if !mentionsLoader(field.Type, pkg) {
			continue
		}
		if len(field.Names) == 0 {
			return "Loader", true
		}
		return field.Names[0].Name, true
	}
	return "", false
}

// mentionsLoader reports whether the type expression names <pkg>.Loader anywhere
// within it.
func mentionsLoader(expr ast.Expr, pkg string) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		selector, ok := n.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "Loader" {
			return true
		}
		if ident, isIdent := selector.X.(*ast.Ident); isIdent && ident.Name == pkg {
			found = true
		}
		return !found
	})
	return found
}

// panelModel builds the fixture's model, sizes it, ingests its data and plays the
// `t` its captureKeys declare — the same drive ModelAt performs — returning the
// model with the panel open.
//
// The MODEL rather than its frame, because §9.2's arrow-preview is only reachable
// from a model that can take another key: a still is where a fixture stops, not
// where the live view does.
func panelModel(t *testing.T, deps tui.Deps) tea.Model {
	t.Helper()

	sessions, err := deps.Lister.ListSessions()
	if err != nil {
		t.Fatalf("the fixture's lister failed: %v", err)
	}
	projects, err := deps.ProjectStore.List()
	if err != nil {
		t.Fatalf("the fixture's project store failed: %v", err)
	}

	var model tea.Model = tui.Build(deps)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 120, Height: 40})
	model, _ = model.Update(tui.SessionsMsg{Sessions: sessions})
	model, _ = model.Update(tui.ProjectsLoadedMsg{Projects: projects})
	model, _ = model.Update(keyRune('t'))
	return model
}

// driveToPanel is panelModel's painted frame — what most assertions here want,
// since internal/tui exports no panel accessor and the frame is what a tape
// screenshots.
func driveToPanel(t *testing.T, deps tui.Deps) string {
	t.Helper()
	return frameOf(t, panelModel(t, deps))
}

// frameOf is the model's painted content.
func frameOf(t *testing.T, model tea.Model) string {
	t.Helper()
	painted, ok := model.(tui.Model)
	if !ok {
		t.Fatalf("the drive returned a %T, not a tui.Model", model)
	}
	return painted.View().Content
}

// backgroundSGR is a token's rendered BACKGROUND SGR parameter run. Styled
// output carries no hex, so a frame is searched for this rather than for the
// value a theme file declares.
func backgroundSGR(t *testing.T, tok theme.Token) string {
	t.Helper()
	probe := lipgloss.NewStyle().Background(tok.Color()).Render("x")
	start := strings.IndexByte(probe, '[')
	end := strings.IndexByte(probe, 'm')
	if start < 0 || end <= start {
		t.Fatalf("could not derive the background SGR from %q", probe)
	}
	return probe[start+1 : end]
}
