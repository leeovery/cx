package theme_test

import (
	"go/ast"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// nominationLoader returns the production-shaped loader every resolution fixture
// runs through: EVERY BUILT-IN SLUG RESERVED, with the §12.3 event seam silent.
//
// The reserved set is load-bearing rather than incidental here. The theme a slot
// falls back to is a built-in, and §5.4 exists so that built-in can never be the
// user's own broken file — a fixture staging `tokyo-night.theme` in the themes
// directory would resolve to it under a hand-built Loader and prove the opposite
// of what the assertion claims.
//
// The seam is NIL — the silent one — because these fixtures assert the RESOLUTION
// RECORD, and a resolution now emits `theme: loaded` and `theme: fallback applied`
// per slot. Those events are asserted where they belong, against a capturing seam
// in events_test.go; a loader here that emitted into the process handler would
// only add noise to assertions about the record's shape. It doubles as the
// standing proof that the nil seam is safe on the path every TUI construction
// runs.
func nominationLoader() theme.Loader {
	return theme.NewLoader(nil)
}

// builtinTheme returns the palette the named built-in parses to — the value a
// mode-matched fallback must land on, taken from the embedded set rather than
// restated, so a corrected built-in cannot leave the expectation behind.
func builtinTheme(t *testing.T, slug string) theme.Theme {
	t.Helper()

	result, rejection, found := nominationLoader().LoadBuiltin(slug)
	if !found {
		t.Fatalf("LoadBuiltin(%q) reports not found — the fallback it names cannot resolve", slug)
	}
	if rejection != nil {
		t.Fatalf("LoadBuiltin(%q) = %v, want the embedded palette", slug, rejection)
	}
	return result.Theme
}

// dropInTheme returns the palette the theme file at path parses to, loaded
// through the ladder's own entry point.
//
// It is LoadFile's answer rather than a restatement, which is what makes an
// assertion over it a claim about WHICH theme a slot resolved to rather than
// about how the file parses.
func dropInTheme(t *testing.T, path string) theme.Theme {
	t.Helper()

	result, rejection := nominationLoader().LoadFile(path)
	if rejection != nil {
		t.Fatalf("LoadFile(%s) = %v, want the drop-in's palette", path, rejection)
	}
	return result.Theme
}

// requireDistinctDefaults fails unless the two shipped defaults are different
// slugs naming different palettes.
//
// Every mode-matched assertion in this file would pass a SWAPPED fallback map if
// they were not, so the distinctness is checked rather than assumed.
func requireDistinctDefaults(t *testing.T) {
	t.Helper()

	if theme.DefaultLightSlug == theme.DefaultDarkSlug {
		t.Fatalf("DefaultLightSlug and DefaultDarkSlug are both %q — a swapped fallback map would be undetectable", theme.DefaultLightSlug)
	}
	if builtinTheme(t, theme.DefaultLightSlug) == builtinTheme(t, theme.DefaultDarkSlug) {
		t.Fatal("the two shipped defaults parse to the same palette — a swapped fallback map would be undetectable")
	}
}

// TestResolveNomination_FallbackIsModeMatched pins §8.5's table: an unloadable
// `theme_light` falls to the LIGHT default, an unloadable `theme_dark` and an
// unloadable constant `theme` to the DARK one.
//
// Mode-matching is the whole decision. A single fixed fallback regardless of mode
// was rejected because it throws a light-terminal user with a typo in their light
// slot onto a dark palette — a bigger surprise than falling to the light default,
// and the one outcome the fallback exists to avoid.
//
// The expectations are the SHARED CONSTANTS, never the literals they hold. §8.3's
// "the adaptive pair degrades to a constant dark default" argument is true only
// because an unresolvable slot lands on the theme the shipped default already
// nominates, so a literal here would leave that argument silently untrue the day
// either constant moves.
func TestResolveNomination_FallbackIsModeMatched(t *testing.T) {
	requireDistinctDefaults(t)

	tests := []struct {
		name         string
		setting      theme.Setting
		wantSlots    int
		index        int
		wantSlot     theme.Slot
		wantSlug     string
		wantFallback string
	}{
		{
			name:         "a broken light slot",
			setting:      theme.Setting{Light: "gone-light", Dark: "nord"},
			wantSlots:    2,
			index:        0,
			wantSlot:     theme.SlotLight,
			wantSlug:     "gone-light",
			wantFallback: theme.DefaultLightSlug,
		},
		{
			name:         "a broken dark slot",
			setting:      theme.Setting{Light: "nord", Dark: "gone-dark"},
			wantSlots:    2,
			index:        1,
			wantSlot:     theme.SlotDark,
			wantSlug:     "gone-dark",
			wantFallback: theme.DefaultDarkSlug,
		},
		{
			name:         "a broken constant",
			setting:      theme.Setting{IsConstant: true, Constant: "gone"},
			wantSlots:    1,
			index:        0,
			wantSlot:     theme.SlotConstant,
			wantSlug:     "gone",
			wantFallback: theme.DefaultDarkSlug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := nominationLoader().ResolveNomination(tt.setting, t.TempDir())

			if err != nil {
				t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", tt.setting, err)
			}
			if len(got.Slots) != tt.wantSlots {
				t.Fatalf("Slots = %+v, want %d", got.Slots, tt.wantSlots)
			}
			want := theme.SlotResolution{
				Slot:      tt.wantSlot,
				Requested: tt.wantSlug,
				Resolved:  tt.wantFallback,
				FellBack:  true,
				Reason:    theme.ReasonNotFound,
				Theme:     builtinTheme(t, tt.wantFallback),
			}
			if got.Slots[tt.index] != want {
				t.Errorf("slot resolution = %+v, want %+v", got.Slots[tt.index], want)
			}
		})
	}
}

// TestResolveNomination_StructuredOutcome pins the shape §8.5's fallback is
// reported in: WHICH slot fell back, FROM which slug, FOR which reason, and what
// actually loaded — one record per slot, assembled from the Setting alone.
//
// Every surface over it reads this record rather than re-deriving it: §12.3's
// events, Phase 7's doctor line, and Phase 8's rule that the `●` stays on the
// PERSISTED slug while the fallback's own row carries no badge. That last one is
// why Requested and Resolved are carried separately — under a fallback they
// differ by design, and a resolver returning only a Theme makes the distinction
// unrecoverable.
//
// The whole slice is asserted at once, so a record that is right about the reason
// and wrong about the slot cannot pass.
func TestResolveNomination_StructuredOutcome(t *testing.T) {
	loader := nominationLoader()
	dir := t.TempDir()
	writeTheme(t, dir, "broken-light.theme", withoutKey(themeLines(), "bg.subtle"))
	survivor := writeTheme(t, dir, "nord-lee.theme", themeLines())
	setting := theme.Setting{Light: "broken-light", Dark: "nord-lee"}

	got, err := loader.ResolveNomination(setting, dir)

	if err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", setting, err)
	}
	want := []theme.SlotResolution{
		{
			Slot:      theme.SlotLight,
			Requested: "broken-light",
			Resolved:  theme.DefaultLightSlug,
			FellBack:  true,
			Reason:    theme.ReasonMissingTokens,
			Theme:     builtinTheme(t, theme.DefaultLightSlug),
		},
		{
			Slot:      theme.SlotDark,
			Requested: "nord-lee",
			Resolved:  "nord-lee",
			Theme:     dropInTheme(t, survivor),
		},
	}
	if len(got.Slots) != len(want) {
		t.Fatalf("Slots = %+v, want %d records", got.Slots, len(want))
	}
	for i := range want {
		if got.Slots[i] != want[i] {
			t.Errorf("Slots[%d] = %+v, want %+v", i, got.Slots[i], want[i])
		}
	}
}

// TestResolveNomination_NominationShapeMatchesSetting pins that the assembled
// Nomination is §8.2's two-state setting with the files read: one palette under a
// constant, two under a pair, and NO ACTIVE MEMBER selected either way.
//
// Nothing here selects a member. The light/dark gate does that when the OSC 11
// reply or the timeout lands (task 5-7), which is before anything is painted —
// so handing out a provisional member would invite a second resolution to
// reconcile with §8.8's resolve-once rule.
//
// The shape holds whether or not a slot fell back, which is why each case runs
// twice: a fallback substitutes a PALETTE, never a setting state.
func TestResolveNomination_NominationShapeMatchesSetting(t *testing.T) {
	t.Run("a constant yields one slot and a constant nomination", func(t *testing.T) {
		dir := t.TempDir()
		path := writeTheme(t, dir, "nord-lee.theme", themeLines())
		setting := theme.Setting{IsConstant: true, Constant: "nord-lee"}

		got, err := nominationLoader().ResolveNomination(setting, dir)

		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v", setting, err)
		}
		if len(got.Slots) != 1 || got.Slots[0].Slot != theme.SlotConstant {
			t.Fatalf("Slots = %+v, want exactly one SlotConstant record", got.Slots)
		}
		if !got.Nomination.IsConstant() {
			t.Error("IsConstant() = false, want true — the nomination mirrors the setting's state")
		}
		want := dropInTheme(t, path)
		if got.Nomination.Constant() != want {
			t.Errorf("Constant() = %+v, want the resolved palette %+v", got.Nomination.Constant(), want)
		}
		if got.Nomination.Select(true) != want || got.Nomination.Select(false) != want {
			t.Error("Select() answers differ under a constant, want the constant for either answer — detection is never consulted")
		}
	})

	t.Run("a pair yields two slots in light-then-dark order", func(t *testing.T) {
		dir := t.TempDir()
		light := writeTheme(t, dir, "nord-lee.theme", themeLines())
		setting := theme.Setting{Light: "nord-lee", Dark: "nord"}

		got, err := nominationLoader().ResolveNomination(setting, dir)

		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v", setting, err)
		}
		if len(got.Slots) != 2 {
			t.Fatalf("Slots = %+v, want exactly two records", got.Slots)
		}
		if got.Slots[0].Slot != theme.SlotLight || got.Slots[1].Slot != theme.SlotDark {
			t.Errorf("slot order = %v then %v, want light then dark", got.Slots[0].Slot, got.Slots[1].Slot)
		}
		if got.Nomination.IsConstant() {
			t.Error("IsConstant() = true, want false — an adaptive pair names no constant")
		}
		if want := (theme.Theme{}); got.Nomination.Constant() != want {
			t.Error("Constant() returned a palette under a pair, want the zero Theme — there is no active member to hand out")
		}
		if want := dropInTheme(t, light); got.Nomination.Select(false) != want {
			t.Errorf("Select(false) = %+v, want the light slot's palette %+v", got.Nomination.Select(false), want)
		}
		if want := builtinTheme(t, "nord"); got.Nomination.Select(true) != want {
			t.Errorf("Select(true) = %+v, want the dark slot's palette %+v", got.Nomination.Select(true), want)
		}
	})

	t.Run("a fallback substitutes a palette, never a state", func(t *testing.T) {
		constant, err := nominationLoader().ResolveNomination(theme.Setting{IsConstant: true, Constant: "gone"}, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(constant) = %v", err)
		}
		pair, err := nominationLoader().ResolveNomination(theme.Setting{Light: "gone-light", Dark: "gone-dark"}, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(pair) = %v", err)
		}

		if !constant.Nomination.IsConstant() || len(constant.Slots) != 1 {
			t.Errorf("a fallen-back constant = %+v, want one slot and a constant nomination", constant)
		}
		if pair.Nomination.IsConstant() || len(pair.Slots) != 2 {
			t.Errorf("a fallen-back pair = %+v, want two slots and an adaptive nomination", pair)
		}
	})
}

// withoutBuiltin returns a built-in byte source with one slug removed — the
// binary §7.6's build-time guarantee says cannot ship: one whose embedded set
// cannot supply the theme a slot falls back to.
//
// The seam is the only way to reach that state at all, which is exactly why it
// exists: in a correctly built binary the path is unreachable, and an
// unreachable path with no test is a path nobody has ever run.
func withoutBuiltin(omit string) func(string) ([]byte, bool) {
	return func(slug string) ([]byte, bool) {
		if slug == omit {
			return nil, false
		}
		return theme.BuiltinBytes(slug)
	}
}

// corruptBuiltin returns a built-in byte source whose named slug is PRESENT and
// invalid, rather than absent — the other half of §7.6's broken-binary state, and
// the one that proves the outcome does not vary by cause.
func corruptBuiltin(corrupt string) func(string) ([]byte, bool) {
	broken := []byte(strings.Join(withoutKey(themeLines(), "canvas"), "\n") + "\n")
	return func(slug string) ([]byte, bool) {
		if slug == corrupt {
			return broken, true
		}
		return theme.BuiltinBytes(slug)
	}
}

// requireZeroResolution fails unless the resolution is the zero value: no slot
// records, and no palette of any kind reachable through the nomination.
//
// It is the checkable form of "an error rather than a zero-valued Theme". A
// Resolution handed back alongside an error would be a colourless picker painted
// from a palette nobody chose — the limp-on §7.6 refuses in favour of failing
// loudly.
func requireZeroResolution(t *testing.T, got theme.Resolution) {
	t.Helper()

	if got.Slots != nil {
		t.Errorf("Slots = %+v alongside an error, want none", got.Slots)
	}
	if got.Nomination.IsConstant() {
		t.Error("IsConstant() = true alongside an error, want the zero Nomination")
	}
	zero := theme.Theme{}
	if got.Nomination.Constant() != zero || got.Nomination.Select(true) != zero || got.Nomination.Select(false) != zero {
		t.Error("the nomination carries a palette alongside an error, want the zero Theme — there is no runtime last-resort palette")
	}
}

// TestResolveNomination_UnresolvableFallbackErrors pins §7.6's one genuinely
// fatal state: the theme a slot falls back to cannot itself be loaded from the
// embedded set.
//
// There is NO SAFETY NET beneath this point by decision — a build-time guarantee
// deliberately replaced the runtime crutch, and "a compiled-in last-resort
// palette equal to Tokyo Night Dark" was rejected in those terms. So the only
// honest answers are an ordinary error or a picker painted from values nobody
// chose, and this is the first.
//
// The message is NOT asserted here: task 5-6 owns §14A's pinned sentence and the
// single source it comes from. What is asserted is the shape — an error, no
// second fallback, and nothing to render.
func TestResolveNomination_UnresolvableFallbackErrors(t *testing.T) {
	tests := []struct {
		name    string
		omit    string
		setting theme.Setting
	}{
		{
			name:    "a constant whose dark fallback is missing",
			omit:    theme.DefaultDarkSlug,
			setting: theme.Setting{IsConstant: true, Constant: "gone"},
		},
		{
			name:    "a light slot whose light fallback is missing",
			omit:    theme.DefaultLightSlug,
			setting: theme.Setting{Light: "gone-light", Dark: "nord"},
		},
		{
			name:    "a dark slot whose dark fallback is missing",
			omit:    theme.DefaultDarkSlug,
			setting: theme.Setting{Light: "nord", Dark: "gone-dark"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := nominationLoader()
			loader.BuiltinSource = withoutBuiltin(tt.omit)

			got, err := loader.ResolveNomination(tt.setting, t.TempDir())

			if err == nil {
				t.Fatalf("ResolveNomination(%+v) = %+v, want an error — the fallback itself did not resolve", tt.setting, got)
			}
			requireZeroResolution(t, got)
		})
	}

	t.Run("a corrupt fallback fails the same way as a missing one", func(t *testing.T) {
		loader := nominationLoader()
		loader.BuiltinSource = corruptBuiltin(theme.DefaultDarkSlug)
		setting := theme.Setting{IsConstant: true, Constant: "gone"}

		got, err := loader.ResolveNomination(setting, t.TempDir())

		if err == nil {
			t.Fatalf("ResolveNomination(%+v) = %+v, want an error — the fallback parsed as invalid", setting, got)
		}
		requireZeroResolution(t, got)
	})

	t.Run("it never falls back a second time", func(t *testing.T) {
		var requested []string
		loader := nominationLoader()
		loader.BuiltinSource = func(slug string) ([]byte, bool) {
			requested = append(requested, slug)
			return withoutBuiltin(theme.DefaultDarkSlug)(slug)
		}

		if _, err := loader.ResolveNomination(theme.Setting{IsConstant: true, Constant: "gone"}, t.TempDir()); err == nil {
			t.Fatal("ResolveNomination succeeded with its fallback removed — the assertion below would be vacuous")
		}

		want := []string{"gone", theme.DefaultDarkSlug}
		if !slices.Equal(requested, want) {
			t.Errorf("built-ins asked for = %v, want %v — a failed fallback is fatal, never a second fallback", requested, want)
		}
	})
}

// resolutionSource returns the parsed resolution.go, failing rather than passing
// vacuously when the file is not found.
func resolutionSource(t *testing.T) parsedThemeSource {
	t.Helper()

	for _, source := range parseThemeSources(t) {
		if source.Name == "resolution.go" {
			return source
		}
	}
	t.Fatal("resolution.go not found among internal/theme's production sources")
	return parsedThemeSource{}
}

// TestResolveNomination_FallbackUsesSharedConstants pins the coincidence §8.3's
// argument rests on: THE FALLBACK VALUES ARE THE SHIPPED DEFAULT'S VALUES.
//
// "The adaptive pair degrades to a constant dark default" is true only because an
// unresolvable slot lands on the theme the shipped default already nominates —
// before the fallback was pinned, that argument was resting on two different
// notions of "default". So changing these values, or adopting the rejected
// single-fixed-fallback alternative, silently invalidates §8.3: nothing stops
// compiling and no floor moves, the reasoning simply stops holding.
//
// The behavioural half asserts the two mechanisms MEET — a pair whose slots are
// both unloadable paints exactly what a virgin install paints. The source half
// asserts they cannot drift, since a literal and a constant holding the same
// string are indistinguishable at runtime on the day they agree, which is every
// day until someone edits one of them.
func TestResolveNomination_FallbackUsesSharedConstants(t *testing.T) {
	requireDistinctDefaults(t)

	t.Run("a fallen-back pair paints what a virgin install paints", func(t *testing.T) {
		shipped, _ := theme.ResolveSetting("", "", "")

		fallen, err := nominationLoader().ResolveNomination(theme.Setting{Light: "gone-light", Dark: "gone-dark"}, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(a broken pair) = %v", err)
		}
		virgin, err := nominationLoader().ResolveNomination(shipped, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(the shipped pair) = %v", err)
		}

		for _, dark := range []bool{false, true} {
			if fallen.Nomination.Select(dark) != virgin.Nomination.Select(dark) {
				t.Errorf("Select(%t) differs between a fallen-back pair and the shipped default, want identical palettes — §8.3's degrades-to-a-constant-dark-default argument rests on them being the same values", dark)
			}
		}
		if got := []string{fallen.Slots[0].Resolved, fallen.Slots[1].Resolved}; !slices.Equal(got, []string{theme.DefaultLightSlug, theme.DefaultDarkSlug}) {
			t.Errorf("fallback slugs = %v, want the shared constants %v", got, []string{theme.DefaultLightSlug, theme.DefaultDarkSlug})
		}
	})

	t.Run("the fallback map names the constants, never their values", func(t *testing.T) {
		source := resolutionSource(t)

		ast.Inspect(source.File, func(n ast.Node) bool {
			lit, isLit := n.(*ast.BasicLit)
			if !isLit || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if value == theme.DefaultDarkSlug || value == theme.DefaultLightSlug {
				t.Errorf("%s:%d declares the literal %q — the fallback map is expressed in DefaultLightSlug/DefaultDarkSlug so it cannot drift from the shipped default", source.Name, source.Fset.Position(lit.Pos()).Line, value)
			}
			return true
		})
	})
}

// fallbackCause is one story about why a nomination did not load: what the file
// state is, what prefs.json names, and which §6.2 reason it lands on.
type fallbackCause struct {
	name       string
	slug       string
	stage      func(t *testing.T) string
	wantReason theme.Reason
}

// fallbackCauses enumerates every cause §8.5's ONE NOT-LOADABLE PATH serves.
//
// Nine stories, six reasons: a deleted file, a renamed file and a typo in
// prefs.json are the same `not found`, an unreadable file and an unreadable
// directory the same `unreadable`. That several stories share a reason is the
// point — the fallback branches on none of them.
func fallbackCauses() []fallbackCause {
	dirWith := func(base string, lines []string) func(t *testing.T) string {
		return func(t *testing.T) string {
			t.Helper()
			dir := t.TempDir()
			writeTheme(t, dir, base, lines)
			return dir
		}
	}

	return []fallbackCause{
		{
			name:       "a deleted file",
			slug:       "nord-lee",
			stage:      func(t *testing.T) string { return t.TempDir() },
			wantReason: theme.ReasonNotFound,
		},
		{
			name:       "a renamed file",
			slug:       "nord-lee",
			stage:      dirWith("nord-lee-renamed.theme", themeLines()),
			wantReason: theme.ReasonNotFound,
		},
		{
			name:       "a typo in prefs.json",
			slug:       "nrod-lee",
			stage:      dirWith("nord-lee.theme", themeLines()),
			wantReason: theme.ReasonNotFound,
		},
		{
			name:       "an illegal persisted slug",
			slug:       "../evil",
			stage:      dirWith("nord-lee.theme", themeLines()),
			wantReason: theme.ReasonBadName,
		},
		{
			name:       "a missing token",
			slug:       "nord-lee",
			stage:      dirWith("nord-lee.theme", withoutKey(themeLines(), "bg.subtle")),
			wantReason: theme.ReasonMissingTokens,
		},
		{
			name:       "a bad colour",
			slug:       "nord-lee",
			stage:      dirWith("nord-lee.theme", withValue(themeLines(), "canvas", "blue")),
			wantReason: theme.ReasonBadColour,
		},
		{
			name:       "a duplicate key",
			slug:       "nord-lee",
			stage:      dirWith("nord-lee.theme", append(themeLines(), "text.primary = #010203")),
			wantReason: theme.ReasonBadSyntax,
		},
		{
			name: "an unreadable file",
			slug: "nord-lee",
			stage: func(t *testing.T) string {
				t.Helper()
				dir := t.TempDir()
				writeUnreadableTheme(t, dir, "nord-lee.theme")
				return dir
			},
			wantReason: theme.ReasonUnreadable,
		},
		{
			name:       "an unreadable directory",
			slug:       "nord-lee",
			stage:      unreadableDir,
			wantReason: theme.ReasonUnreadable,
		},
	}
}

// reachableFallbackReasons is every §6.2 reason a NOMINATION can fail with.
//
// It is six of the seven. `reserved name` is missing because it is unreachable
// from this path by construction: a slug colliding with a built-in resolves to
// the BUILT-IN first (§8.4's ordering), so the user's file is never opened and
// the rung never runs — which is §5.4's no-shadowing property stated as an
// outcome, and the very thing that keeps a fallback safe.
var reachableFallbackReasons = []theme.Reason{
	theme.ReasonBadName,
	theme.ReasonUnreadable,
	theme.ReasonBadSyntax,
	theme.ReasonBadColour,
	theme.ReasonMissingTokens,
	theme.ReasonNotFound,
}

// TestResolveNomination_EveryCauseFallsBack pins §8.5's "one not-loadable path
// serves every cause": every story ends in the same fallback, the same kept
// persisted name and the same shipped default, with ONLY the reason differing.
//
// The whole record is asserted per cause, not just the reason, because the claim
// is that nothing else varies with the cause. A resolver that special-cased an
// absent file — the obvious temptation, since `not found` is the only reason
// outside §6.2's ladder — would pass a reason-only assertion and fail this one.
func TestResolveNomination_EveryCauseFallsBack(t *testing.T) {
	for _, tc := range fallbackCauses() {
		t.Run(tc.name, func(t *testing.T) {
			dir := tc.stage(t)
			setting := theme.Setting{IsConstant: true, Constant: tc.slug}

			got, err := nominationLoader().ResolveNomination(setting, dir)

			if err != nil {
				t.Fatalf("ResolveNomination(%+v) = %v, want the fallback applied", setting, err)
			}
			want := []theme.SlotResolution{{
				Slot:      theme.SlotConstant,
				Requested: tc.slug,
				Resolved:  theme.DefaultDarkSlug,
				FellBack:  true,
				Reason:    tc.wantReason,
				Theme:     builtinTheme(t, theme.DefaultDarkSlug),
			}}
			if !slices.Equal(got.Slots, want) {
				t.Errorf("Slots = %+v, want %+v", got.Slots, want)
			}
		})
	}

	t.Run("the causes cover every reason a nomination can fail with", func(t *testing.T) {
		var covered []theme.Reason
		for _, tc := range fallbackCauses() {
			if !slices.Contains(covered, tc.wantReason) {
				covered = append(covered, tc.wantReason)
			}
		}
		slices.Sort(covered)

		want := slices.Sorted(slices.Values(reachableFallbackReasons))
		if !slices.Equal(covered, want) {
			t.Errorf("causes cover %v, want %v — a reason with no case is a cause nobody has run through the fallback", covered, want)
		}
	})
}

// TestResolveNomination_BothSlotsCanFallBack pins that the two slots fall back
// INDEPENDENTLY in a single launch: two records, two fallbacks, and the pair
// carrying the two shipped defaults.
//
// Nothing short-circuits. A resolver that abandoned the second slot on the first
// failure would leave an adaptive pair half-loaded, which is a state §8.2 has no
// name for and the gate has no answer for.
func TestResolveNomination_BothSlotsCanFallBack(t *testing.T) {
	requireDistinctDefaults(t)

	t.Run("both slots missing", func(t *testing.T) {
		setting := theme.Setting{Light: "gone-light", Dark: "gone-dark"}

		got, err := nominationLoader().ResolveNomination(setting, t.TempDir())

		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v, want two fallbacks", setting, err)
		}
		want := []theme.SlotResolution{
			{Slot: theme.SlotLight, Requested: "gone-light", Resolved: theme.DefaultLightSlug, FellBack: true, Reason: theme.ReasonNotFound, Theme: builtinTheme(t, theme.DefaultLightSlug)},
			{Slot: theme.SlotDark, Requested: "gone-dark", Resolved: theme.DefaultDarkSlug, FellBack: true, Reason: theme.ReasonNotFound, Theme: builtinTheme(t, theme.DefaultDarkSlug)},
		}
		if !slices.Equal(got.Slots, want) {
			t.Fatalf("Slots = %+v, want %+v", got.Slots, want)
		}
		if got.Nomination.Select(false) != builtinTheme(t, theme.DefaultLightSlug) || got.Nomination.Select(true) != builtinTheme(t, theme.DefaultDarkSlug) {
			t.Error("the nomination does not carry the two shipped defaults — a doubly-fallen-back pair is exactly the shipped pair")
		}
	})

	t.Run("both slots broken for different reasons", func(t *testing.T) {
		dir := t.TempDir()
		writeTheme(t, dir, "broken-light.theme", withValue(themeLines(), "canvas", "blue"))
		setting := theme.Setting{Light: "broken-light", Dark: "gone-dark"}

		got, err := nominationLoader().ResolveNomination(setting, dir)

		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v, want two fallbacks", setting, err)
		}
		want := []theme.Reason{theme.ReasonBadColour, theme.ReasonNotFound}
		if gotReasons := []theme.Reason{got.Slots[0].Reason, got.Slots[1].Reason}; !slices.Equal(gotReasons, want) {
			t.Errorf("reasons = %v, want %v — each slot reports its own cause", gotReasons, want)
		}
	})
}

// TestResolveNomination_SurvivingSlotUnaffected pins the other half of
// independence: the slot that loaded is left exactly as it was.
//
// It is the case the fallback exists to protect — a light-terminal user with a
// typo in their light slot keeps the dark theme they chose — and it is what a
// resolver that fell the whole SETTING back to the shipped pair would quietly
// destroy while still producing a picker that renders.
func TestResolveNomination_SurvivingSlotUnaffected(t *testing.T) {
	tests := []struct {
		name         string
		setting      theme.Setting
		survivor     int
		wantSlot     theme.Slot
		fallenSlot   theme.Slot
		wantFallback string
	}{
		{
			name:         "a broken light slot leaves dark alone",
			setting:      theme.Setting{Light: "gone-light", Dark: "nord-lee"},
			survivor:     1,
			wantSlot:     theme.SlotDark,
			fallenSlot:   theme.SlotLight,
			wantFallback: theme.DefaultLightSlug,
		},
		{
			name:         "a broken dark slot leaves light alone",
			setting:      theme.Setting{Light: "nord-lee", Dark: "gone-dark"},
			survivor:     0,
			wantSlot:     theme.SlotLight,
			fallenSlot:   theme.SlotDark,
			wantFallback: theme.DefaultDarkSlug,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeTheme(t, dir, "nord-lee.theme", themeLines())
			survivorTheme := dropInTheme(t, path)
			if survivorTheme == builtinTheme(t, tt.wantFallback) {
				t.Fatal("the drop-in parses to the shipped default's palette — an untouched slot and a fallen-back one would be indistinguishable")
			}

			got, err := nominationLoader().ResolveNomination(tt.setting, dir)

			if err != nil {
				t.Fatalf("ResolveNomination(%+v) = %v", tt.setting, err)
			}
			want := theme.SlotResolution{
				Slot:      tt.wantSlot,
				Requested: "nord-lee",
				Resolved:  "nord-lee",
				Theme:     survivorTheme,
			}
			if got.Slots[tt.survivor] != want {
				t.Errorf("the surviving slot = %+v, want %+v", got.Slots[tt.survivor], want)
			}
			if !got.Slots[1-tt.survivor].FellBack {
				t.Errorf("the other slot = %+v, want FellBack — this fixture is meant to stage one of each", got.Slots[1-tt.survivor])
			}
			if got.Nomination.Select(tt.wantSlot == theme.SlotDark) != survivorTheme {
				t.Error("the nomination's surviving member is not the drop-in's palette — a fallback substitutes one slot, never the setting")
			}
		})
	}
}

// TestResolveNomination_UnsetSlotIsNotAFallback pins §8.5's "no new mechanism":
// an unset slot is not a fallback at all, it is the SAME default rule one step
// earlier.
//
// ResolveSetting has already substituted the shipped default, so the slot
// arrives here as an ordinary slug and resolves ordinarily — FellBack false, and
// Requested the shipped default's own slug. That is what makes unset and
// unloadable converge with no second mechanism, and it is why a virgin install
// produces ZERO fallbacks rather than two that happen to land on the right
// values.
//
// The three directory states are the three a virgin install can be in, the empty
// string included: task 5-7 degrades an unresolvable themes-directory path to it,
// and a built-in must still resolve with no path at all.
func TestResolveNomination_UnsetSlotIsNotAFallback(t *testing.T) {
	dirs := []struct {
		name  string
		stage func(t *testing.T) string
	}{
		{name: "an empty themes directory", stage: func(t *testing.T) string { return t.TempDir() }},
		{name: "an absent themes directory", stage: func(t *testing.T) string { return filepath.Join(t.TempDir(), "themes") }},
		{name: "no themes directory at all", stage: func(t *testing.T) string { return "" }},
	}

	for _, dir := range dirs {
		t.Run(dir.name, func(t *testing.T) {
			setting, _ := theme.ResolveSetting("", "", "")

			got, err := nominationLoader().ResolveNomination(setting, dir.stage(t))

			if err != nil {
				t.Fatalf("ResolveNomination(the shipped pair) = %v", err)
			}
			want := []theme.SlotResolution{
				{Slot: theme.SlotLight, Requested: theme.DefaultLightSlug, Resolved: theme.DefaultLightSlug, Theme: builtinTheme(t, theme.DefaultLightSlug)},
				{Slot: theme.SlotDark, Requested: theme.DefaultDarkSlug, Resolved: theme.DefaultDarkSlug, Theme: builtinTheme(t, theme.DefaultDarkSlug)},
			}
			if !slices.Equal(got.Slots, want) {
				t.Errorf("Slots = %+v, want %+v — a virgin install produces zero fallbacks", got.Slots, want)
			}
		})
	}
}

// TestResolveNomination_SetAndUnsetDefaultsAreIndistinguishable pins why the
// record carries NO SET-NESS FLAG: the question cannot be answered here, and
// nothing asks it.
//
// ResolveSetting substitutes the shipped default into the Setting before this
// function sees it, so a never-set slot and one explicitly set to the same slug
// arrive as the identical value — the distinction survives only in RawKeys, which
// this function is deliberately not given. A flag would therefore have to be
// GUESSED, and a guess about which slug carries the `●` is a second, wrong source
// of truth for §9.5's badge.
//
// Nothing consumes one either: §9.5's badge table keys all three of its rows on
// Requested alone, doctor reads the raw keys directly, and the panel's commit
// takes the raw slug as a parameter. The structural half is what keeps it that
// way, since a flag added later would be silently wrong rather than loudly
// missing.
func TestResolveNomination_SetAndUnsetDefaultsAreIndistinguishable(t *testing.T) {
	t.Run("the two settings resolve to identical records", func(t *testing.T) {
		unsetSetting, _ := theme.ResolveSetting("", "", "")
		setSetting, _ := theme.ResolveSetting("", theme.DefaultLightSlug, theme.DefaultDarkSlug)
		if unsetSetting != setSetting {
			t.Fatalf("Setting = %+v when unset and %+v when set to the same slugs — the claim below is about resolution, not about the setting", unsetSetting, setSetting)
		}

		unset, err := nominationLoader().ResolveNomination(unsetSetting, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(unset) = %v", err)
		}
		set, err := nominationLoader().ResolveNomination(setSetting, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(set) = %v", err)
		}

		if !slices.Equal(unset.Slots, set.Slots) {
			t.Errorf("Slots = %+v when unset and %+v when set to the same slugs, want identical", unset.Slots, set.Slots)
		}
	})

	t.Run("the record declares no set-ness flag", func(t *testing.T) {
		assertFieldKinds(t, reflect.TypeFor[theme.SlotResolution](), []fieldKind{
			{name: "Slot", kind: reflect.Int},
			{name: "Requested", kind: reflect.String},
			{name: "Resolved", kind: reflect.String},
			{name: "FellBack", kind: reflect.Bool},
			{name: "Reason", kind: reflect.String},
			{name: "Theme", kind: reflect.Struct},
		})
	})
}

// TestResolveNomination_NeverOverwritesPrefs pins §6.3's destructive-versus-
// transient rule: FALLING BACK MUST NEVER OVERWRITE THE PERSISTED THEME NAME.
//
// Portal keeps the user's choice and renders the fallback, so fixing the theme
// file restores it on the next launch with no re-selection. Persisting the
// fallback instead would turn a transient failure into a permanent one, at the
// moment the user is least able to tell what happened — and it would do so
// silently, since the picker would look correct afterwards.
//
// The fixtures run the REAL read path (prefs.json → the setting → this
// resolution), because "it did not write" is only worth asserting about the file
// production actually reads. Both fixtures resolve on the FALLBACK path, which is
// the only path on which a write is even tempting.
func TestResolveNomination_NeverOverwritesPrefs(t *testing.T) {
	resolveThrough := func(t *testing.T, path string) theme.Resolution {
		t.Helper()

		keys, err := prefs.NewStore(path).LoadThemeKeys()
		if err != nil {
			t.Fatalf("LoadThemeKeys(%s): %v", path, err)
		}
		setting, _ := theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)
		got, err := nominationLoader().ResolveNomination(setting, t.TempDir())
		if err != nil {
			t.Fatalf("ResolveNomination(%+v) = %v", setting, err)
		}
		return got
	}

	t.Run("a persisted name survives the fallback byte for byte", func(t *testing.T) {
		const persisted = `{"session_list_mode":"by-project","appearance":"dark","theme_light":"gone-light","theme_dark":"gone-dark"}`
		path := writeFile(t, t.TempDir(), "prefs.json", persisted)

		got := resolveThrough(t, path)

		for _, slot := range got.Slots {
			if !slot.FellBack {
				t.Fatalf("slot %+v did not fall back — the fixture must exercise the path a write would be tempting on", slot)
			}
			if slot.Requested == slot.Resolved {
				t.Fatalf("slot %+v resolved what it requested — the persisted name is meant to differ from the rendered one here", slot)
			}
		}
		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read back %s: %v", path, err)
		}
		if string(after) != persisted {
			t.Errorf("prefs.json =\n%s\nwant it byte-identical:\n%s\n— fixing the theme file must restore the theme on the next launch, with no re-selection", after, persisted)
		}
	})

	t.Run("an absent prefs.json stays absent", func(t *testing.T) {
		configDir := t.TempDir()
		path := filepath.Join(configDir, "prefs.json")

		got := resolveThrough(t, path)

		if len(got.Slots) != 2 {
			t.Fatalf("Slots = %+v, want the shipped pair", got.Slots)
		}
		entries, err := os.ReadDir(configDir)
		if err != nil {
			t.Fatalf("read %s: %v", configDir, err)
		}
		if len(entries) != 0 {
			t.Errorf("resolution left %d entries in the config directory, want none — §8.1 leaves prefs.json absent on a virgin install and nothing here seeds it", len(entries))
		}
	})

	t.Run("it reaches no write of any kind", func(t *testing.T) {
		calls := osCallsReachableFrom(t, "ResolveNomination")

		for _, write := range []string{"os.WriteFile", "os.Create", "os.CreateTemp", "os.OpenFile", "os.Rename", "os.Remove", "os.RemoveAll", "os.Mkdir", "os.MkdirAll", "os.Chmod", "os.Symlink", "os.Truncate"} {
			if got := calls[write]; got != 0 {
				t.Errorf("ResolveNomination reaches %d %s call sites, want 0 — resolution reads, it never writes", got, write)
			}
		}
		if calls["os.ReadFile"] == 0 {
			t.Error("ResolveNomination reaches no os.ReadFile call site — the walk resolved nothing, so the assertions above would be vacuous")
		}
	})
}

// enumerationOf reads dir through the panel's own entry point and hands back the
// enumeration it retained — the same value the panel holds for its lifetime
// (§5.8), produced by the same call the panel makes.
//
// The union is discarded deliberately: these fixtures are about what the
// RESOLUTION does with a retained parse, and building the enumeration by hand
// would let a fixture assert against entries the real classification would never
// have produced.
func enumerationOf(t *testing.T, loader theme.Loader, dir string) theme.Enumeration {
	t.Helper()

	enumeration, _ := loader.Open(dir, theme.RawKeys{})
	return enumeration
}

// TestResolveNominationFrom_ReadsNothing pins the whole reason this entry point
// exists: it resolves against the RETAINED enumeration and never against the
// filesystem (§8.4).
//
// A directory read here would produce a THIRD parse of the same slug — neither
// construction's nor the panel's — that can disagree with the row the user is
// looking at, reintroducing exactly the staleness split §5.8 exists to close.
//
// Both halves are asserted because neither reaches the other. The runtime half
// removes the directory after the enumeration, so a read would fail to resolve a
// drop-in that is plainly still in hand. The source half walks the call graph, so
// a read that happened to SUCCEED — the directory still being there in
// production — is caught too; its non-vacuity control is the sibling entry point,
// which must still reach the read this one must not.
func TestResolveNominationFrom_ReadsNothing(t *testing.T) {
	t.Run("it resolves a drop-in whose directory is gone", func(t *testing.T) {
		dir := t.TempDir()
		path := writeTheme(t, dir, "sunset.theme", themeLines())
		want := dropInTheme(t, path)
		loader := nominationLoader()
		enumeration := enumerationOf(t, loader, dir)

		if err := os.RemoveAll(dir); err != nil {
			t.Fatalf("remove %s: %v", dir, err)
		}

		setting := theme.Setting{IsConstant: true, Constant: "sunset"}
		got, err := loader.ResolveNominationFrom(enumeration, setting)

		if err != nil {
			t.Fatalf("ResolveNominationFrom(%+v) = %v, want the retained parse", setting, err)
		}
		if len(got.Slots) != 1 || got.Slots[0].FellBack {
			t.Fatalf("Slots = %+v, want the drop-in resolved with no fallback — the enumeration still holds its parse", got.Slots)
		}
		if got.Nomination.Constant() != want {
			t.Errorf("the constant resolved to %s, want the drop-in's palette %s", got.Nomination.Constant().Canvas.Value, want.Canvas.Value)
		}
	})

	t.Run("it reaches no os call at all", func(t *testing.T) {
		for name, count := range osCallsReachableFrom(t, "ResolveNominationFrom") {
			if strings.HasPrefix(name, "os.") {
				t.Errorf("ResolveNominationFrom reaches %d %s call sites, want 0 — it resolves against the retained enumeration, never the filesystem (§8.4)", count, name)
			}
		}
		if osCallsReachableFrom(t, "ResolveNomination")["os.ReadFile"] == 0 {
			t.Error("the by-name entry point reaches no os.ReadFile call site either — the walk resolved nothing, so the assertion above would be vacuous")
		}
	})
}

// TestResolveNominationFrom_ResolvesAgainstTheEnumerationsEntries pins that the
// enumeration's CLASSIFICATION is what answers, reason and all: a valid entry
// resolves to its palette, and every not-loadable state takes §8.5's fallback
// carrying the reason the row the user is looking at carries.
//
// The unresolved states are the §5.5 pair the union's own persisted rows draw
// (unresolvedRejection): a slug nothing answers to is `not found` where the
// directory could be listed, and `unreadable` where it could not — so the panel's
// row and the theme that actually rendered can never state different reasons for
// the same slug.
func TestResolveNominationFrom_ResolvesAgainstTheEnumerationsEntries(t *testing.T) {
	loader := nominationLoader()

	t.Run("a valid entry resolves to its own palette", func(t *testing.T) {
		dir := t.TempDir()
		path := writeTheme(t, dir, "sunset.theme", themeLines())
		want := dropInTheme(t, path)
		setting := theme.Setting{IsConstant: true, Constant: "sunset"}

		got, err := loader.ResolveNominationFrom(enumerationOf(t, loader, dir), setting)

		if err != nil {
			t.Fatalf("ResolveNominationFrom(%+v) = %v", setting, err)
		}
		if slot := got.Slots[0]; slot.FellBack || slot.Resolved != "sunset" || slot.Theme != want {
			t.Errorf("slot = %+v, want `sunset` resolved to its own palette with no fallback", slot)
		}
	})

	for _, tt := range []struct {
		name        string
		enumeration func(*testing.T) theme.Enumeration
		want        theme.Reason
	}{
		{
			name: "an invalid entry falls back carrying its own reason",
			enumeration: func(t *testing.T) theme.Enumeration {
				dir := t.TempDir()
				writeTheme(t, dir, "sunset.theme", withValue(themeLines(), "canvas", "not-a-colour"))
				return enumerationOf(t, loader, dir)
			},
			want: theme.ReasonBadColour,
		},
		{
			name: "a slug the directory never held falls back as not found",
			enumeration: func(t *testing.T) theme.Enumeration {
				return enumerationOf(t, loader, t.TempDir())
			},
			want: theme.ReasonNotFound,
		},
		{
			name: "an unusable directory falls back as unreadable",
			enumeration: func(*testing.T) theme.Enumeration {
				return theme.Enumeration{DirUnusable: true}
			},
			want: theme.ReasonUnreadable,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			setting := theme.Setting{IsConstant: true, Constant: "sunset"}

			got, err := loader.ResolveNominationFrom(tt.enumeration(t), setting)

			if err != nil {
				t.Fatalf("ResolveNominationFrom(%+v) = %v, want the fallback applied", setting, err)
			}
			slot := got.Slots[0]
			if !slot.FellBack {
				t.Fatalf("slot = %+v, want the fallback applied", slot)
			}
			if slot.Reason != tt.want {
				t.Errorf("reason = %q, want %q — the resolution and the panel row state one reason for one slug", slot.Reason, tt.want)
			}
			if slot.Requested != "sunset" {
				t.Errorf("Requested = %q, want the persisted slug %q — a fallback never moves the `●`", slot.Requested, "sunset")
			}
			if slot.Resolved != theme.DefaultDarkSlug {
				t.Errorf("Resolved = %q, want the constant's mode-matched default %q", slot.Resolved, theme.DefaultDarkSlug)
			}
			if slot.Theme != builtinTheme(t, theme.DefaultDarkSlug) {
				t.Error("the slot carries a palette other than the fallback's")
			}
		})
	}
}

// TestResolveNominationFrom_ConsultsTheEmbeddedSetFirst pins that §8.4's ordering
// survives the change of source: the embedded set answers before the enumeration,
// so a drop-in taking a built-in's slug can never become what a nomination — or a
// fallback — resolves to.
//
// The fixture is the collision itself. `nord.theme` in the themes directory is
// enumerated as a `reserved name` entry (§5.4), so an enumeration consulted first
// would reject the nomination and send it to the dark default. Landing on the
// EMBEDDED nord instead — a different palette from the fallback's — is what tells
// the two orders apart.
func TestResolveNominationFrom_ConsultsTheEmbeddedSetFirst(t *testing.T) {
	loader := nominationLoader()
	dir := t.TempDir()
	writeTheme(t, dir, "nord.theme", themeLines())
	setting := theme.Setting{IsConstant: true, Constant: "nord"}

	got, err := loader.ResolveNominationFrom(enumerationOf(t, loader, dir), setting)

	if err != nil {
		t.Fatalf("ResolveNominationFrom(%+v) = %v", setting, err)
	}
	slot := got.Slots[0]
	if slot.FellBack {
		t.Fatalf("slot = %+v, want the embedded built-in resolved — the shadowing file must never be consulted", slot)
	}
	if want := builtinTheme(t, "nord"); slot.Theme != want {
		t.Errorf("slot resolved to canvas %s, want the embedded nord's %s", slot.Theme.Canvas.Value, want.Canvas.Value)
	}
}

// TestResolveNominationFrom_UnresolvableFallbackErrors pins that §7.6's one
// genuinely fatal state travels this entry point unchanged: a FALLBACK that
// cannot resolve within the embedded set is an ordinary error and a zero
// Resolution, never a second fallback and never a palette nobody chose.
//
// The panel's own policy for that error is its business (it degrades rather than
// escalating, since a settings surface must not be the route by which a broken
// binary quits mid-session) — what this pins is that the error is REPORTED here
// rather than absorbed, so the panel has something to degrade on.
func TestResolveNominationFrom_UnresolvableFallbackErrors(t *testing.T) {
	loader := nominationLoader()
	loader.BuiltinSource = withoutBuiltin(theme.DefaultDarkSlug)
	setting := theme.Setting{IsConstant: true, Constant: "gone"}

	got, err := loader.ResolveNominationFrom(theme.Enumeration{}, setting)

	if err == nil {
		t.Fatalf("ResolveNominationFrom(%+v) = %+v, want an error — the fallback itself did not resolve", setting, got)
	}
	requireZeroResolution(t, got)
}

// TestResolveNominationFrom_EmitsNoLoadedRecord pins §12.3's cadence split across
// the two entry points.
//
// `theme: loaded` is a per-LOAD INFO whose catalogued cadence is TUI construction
// plus the one commit-time load outside it. A panel open re-resolves the same
// persisted setting against a fresher parse, and emitting there — on every open
// and again on every `Esc` — would turn a per-load INFO into the running
// commentary its neighbours dedup precisely to avoid.
//
// `theme: fallback applied` is the opposite case and is asserted alongside, or
// the first assertion would be indistinguishable from a silent seam: §12.3 names
// the panel open and the `Esc` explicitly as resolutions that apply a fallback,
// and deduplicates the WARN per process rather than suppressing it per site.
func TestResolveNominationFrom_EmitsNoLoadedRecord(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	loader := theme.NewLoader(theme.NewEventLogger(logger))
	setting := theme.Setting{IsConstant: true, Constant: "gone"}

	got, err := loader.ResolveNominationFrom(theme.Enumeration{}, setting)

	if err != nil {
		t.Fatalf("ResolveNominationFrom(%+v) = %v, want the fallback applied", setting, err)
	}
	if !got.Slots[0].FellBack {
		t.Fatalf("slot = %+v, want the fallback applied — the assertions below need one", got.Slots[0])
	}
	if n := len(recordsNamed(sink, "loaded")); n != 0 {
		t.Errorf("a panel-open resolution emitted %d `theme: loaded` records, want 0 — the event's cadence is construction plus the commit-time load (§12.3):\n%s", n, sink.Body())
	}
	if n := len(recordsNamed(sink, "fallback applied")); n != 1 {
		t.Errorf("emitted %d `theme: fallback applied` records, want exactly 1 — §12.3 names the panel open as a site that applies one:\n%s", n, sink.Body())
	}
}
