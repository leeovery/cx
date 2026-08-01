package theme_test

import (
	"maps"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// This file is §13.5's light-only leg: the four eyeball-pinned surface tints,
// and the enrolment table that decides which built-ins they apply to.
//
// It exists because a light tint on a light canvas is NUMERICALLY
// INSUFFICIENT. The fill floor in contrast_test.go bounds such a tint from
// below, but nothing in the math distinguishes a tasteful #D0C6F0 from a
// washed-out one that also clears 1.10 — the judgement was made by human eye at
// MV's validation gate and cannot be reconstructed from the numbers. An
// exact-value pin is therefore the only guard those four values can have.
//
// THE SCOPE RULE, because "light-only" is easy to misread as a discount:
// the carve-out is ENROLMENT, NOT RELAXATION. Every §13.5 rule asserted in
// contrast_test.go applies to a light built-in exactly as it does to a dark one
// — including all three ≥ 1.10 fill legs on bg.selection, bg.attention and
// bg.subtle, which contrast_test.go already measures against each theme's own
// canvas. The two tests here are ADDITIONAL. Nothing in this file lowers,
// widens or exempts anything asserted there.
//
// Pins for any FUTURE light theme are established the same way these were: by
// human eyeball at a visual gate, through
// `go run ./cmd/capturetool --fixture contrast-validation --theme <slug>`.
// There is no automatic path and no derivation to fall back on — which is
// precisely why §7.5 defers a second light theme to separate work and calls it
// a design task rather than a file drop. A dark theme, by contrast, is
// near-free: it is floor-checked by auto-enumeration with no eyeball step at
// all.

// themeIsLight is the light/dark ENROLMENT table: every embedded built-in's
// slug, mapped to whether its palette is light.
//
// It is TEST-SIDE KNOWLEDGE ONLY, and deliberately so. Portal itself has no
// notion of a theme being light or dark (§4.7): it is neither declared in a
// .theme file nor derived from canvas luminance, because the mechanic has no
// consumer — under the adaptive two-slot form THE SLOT classifies the theme
// ("use this one when the terminal is light"), so Portal never inspects a
// palette to find out. A test is allowed to know things the runtime does not,
// and this is the one exception §4.7 names.
//
// The table's sole job is to tell the two tests below WHICH themes to run
// against, so a light theme is never pinned against a dark reference and a dark
// theme is never asked for pins it has none of.
//
// It is the one list in this package that is hand-maintained rather than
// derived — light/dark cannot be computed from a palette Portal refuses to
// classify — so it is held to the embedded set by
// TestThemeAppearanceTableCoversEveryEmbeddedTheme in both directions.
var themeIsLight = map[string]bool{
	"tokyo-night":     false,
	"tokyo-night-day": true,
}

// pinnedTokenNames is the eyeball-pinned SET: four tokens, and the COUNT IS
// LOAD-BEARING (§13.5).
//
// The pre-consolidation test listed five entries — bg.selection, bg.warning,
// bg.track, border.separator and border.footer — but the last two shared one
// light hex and §2.2 consolidated them into a single `border` role, so the set
// is four DISTINCT tokens under the current vocabulary. The count determines
// how wide the light-only enrolment has to be and which pin notes live as `#`
// comments in the theme files (§7.1), which is why it is asserted rather than
// merely written down.
//
// `border` is the row that makes the set four rather than three, and it is
// pinned here while carrying NO numeric floor anywhere in §13.5's table — it
// participates in the pins and in nothing else.
var pinnedTokenNames = []string{
	"bg.selection",
	"bg.attention",
	"bg.subtle",
	"border",
}

// lightPins is the per-light-theme table of eyeball-pinned hexes: slug → token
// name → the exact value that theme's file must carry.
//
// It is keyed by slug so a future light theme is a ROW, not a rewrite. The
// values are §4.3's canonical upper case, which is the form the parser hands
// back regardless of how the file writes them.
//
// tokyo-night-day's four are §7.3's values. §7.7's re-derivation check ran over
// that file's contrast corrections and found nothing over the ΔE threshold, so
// no pin moved and these are the values the built-in always shipped.
var lightPins = map[string]map[string]string{
	"tokyo-night-day": {
		// Derivation: the dark violet selection anchor #28243a lifted onto the
		// light canvas #e1e2e7; eyeball-confirmed.
		"bg.selection": "#D0C6F0",
		// Derivation: the dark amber warning anchor #241B10 lifted onto
		// #e1e2e7; eyeball-confirmed, and it held as derived.
		"bg.attention": "#E8D6A8",
		// Derivation: the dark grey track anchor #26283A lifted onto #e1e2e7;
		// eyeball-confirmed as a distinct empty-track surface.
		"bg.subtle": "#D2D4DE",
		// Derivation: the dark rule anchor #292E42 lifted onto #e1e2e7;
		// eyeball-confirmed. One value serves the title rule, the footer rule,
		// the modal frames and the edit-modal chips.
		"border": "#C9CDDB",
	},
}

// TestThemeAppearanceTableCoversEveryEmbeddedTheme is the assertion that makes
// the hand-maintained table safe: it is held to the embedded set in BOTH
// directions.
//
// A built-in added to builtins/ without an entry fails here rather than
// shipping unexamined — a light theme with no row would be skipped by both
// tests below, so its four un-derivable tints would carry no guard at all, and
// "Portal-endorsed but nobody checked it" is exactly the failure the bundled
// tier exists to prevent (§6.4). A row naming a slug that no longer exists
// fails here too, so a deleted or renamed built-in leaves no stale entry
// pretending to enrol something.
func TestThemeAppearanceTableCoversEveryEmbeddedTheme(t *testing.T) {
	embedded := theme.BuiltinSlugs()
	if len(embedded) == 0 {
		t.Fatal("the embedded set is empty — the enrolment assertion would pass vacuously")
	}

	for _, slug := range embedded {
		if _, enrolled := themeIsLight[slug]; !enrolled {
			t.Errorf("built-in %q has no light/dark entry — enrol it in themeIsLight so §13.5's light-only pins know whether to run against it", slug)
		}
	}

	enrolled := slices.Sorted(maps.Keys(themeIsLight))
	if !slices.Equal(enrolled, embedded) {
		t.Errorf("the light/dark table enrols %v, want exactly the embedded set %v", enrolled, embedded)
	}
}

// TestThemeAppearanceTableHasNoStaleEntries names the stale-row direction on
// its own.
//
// The coverage test above catches it as a set difference; this reports it as
// what it is, so a renamed or deleted built-in produces a message naming the row
// to remove rather than two slug lists to diff by eye.
func TestThemeAppearanceTableHasNoStaleEntries(t *testing.T) {
	embedded := theme.BuiltinSlugs()

	for _, slug := range slices.Sorted(maps.Keys(themeIsLight)) {
		if !slices.Contains(embedded, slug) {
			t.Errorf("the light/dark table enrols %q, which is not an embedded built-in (embedded: %v) — drop the stale row", slug, embedded)
		}
	}
}

// TestLightSurfaceTintsPinned pins each light built-in's four eyeball-locked
// surface tints to their exact hexes.
//
// This is the only guard those four values can have: the fill floor bounds them
// from below but cannot choose between two values that both clear it, so the
// pin is what makes a later change a deliberate, visible edit rather than a
// silent drift. It runs against LIGHT themes only, because a light tint judged
// against a dark reference is a meaningless measurement — enrolment, not
// relaxation: contrast_test.go's rules already apply to these themes in full.
func TestLightSurfaceTintsPinned(t *testing.T) {
	themes := embeddedThemes(t)

	for _, slug := range lightThemeSlugs(t) {
		pins, ok := lightPins[slug]
		if !ok {
			t.Errorf("light built-in %q has no row in lightPins — its four un-derivable tints would carry no guard", slug)
			continue
		}

		th := themes[slug]
		for _, token := range pinnedTokenNames {
			t.Run(slug+"/"+token, func(t *testing.T) {
				want, pinned := pins[token]
				if !pinned {
					t.Fatalf("lightPins[%q] carries no pin for %q — all four eyeball-pinned tints are required", slug, token)
				}
				if got := tokenValue(t, th, token); got != want {
					t.Errorf("%s %s = %q, want the eyeball-pinned %q", slug, token, got, want)
				}
			})
		}
	}
}

// TestLightTintFillsArePerceptible measures each light built-in's four pinned
// tints against THAT THEME'S OWN canvas.
//
// The reference is th.Canvas.Value rather than a light-canvas constant: a theme
// is one palette carrying its own surface (§3.1), so there is no mode axis left
// to measure against and no constant that would be right for a second light
// theme with a different near-white.
//
// It is the same 1.10 fill-perceptible floor contrast_test.go applies, asserted
// here over the pinned set — additional coverage on the values whose numbers
// cannot settle them, not a substitute for the three fill legs that already run
// against every embedded theme, light and dark alike.
func TestLightTintFillsArePerceptible(t *testing.T) {
	themes := embeddedThemes(t)

	for _, slug := range lightThemeSlugs(t) {
		th := themes[slug]
		for _, token := range pinnedTokenNames {
			t.Run(slug+"/"+token, func(t *testing.T) {
				assertAtLeast(t, slug+" "+token+" fill vs own canvas",
					tokenValue(t, th, token), th.Canvas.Value, floorFillPerceptible)
			})
		}
	}
}

// TestLightPins_AreExactlyFourTokens pins the count §13.5 calls load-bearing.
//
// Four, not the three it is easy to assume from the tint roles alone, and not
// the five rows the pre-consolidation test carried: `border` participates, and
// `border.separator`/`border.footer` no longer exist as separate roles (§2.2).
// The names are checked against the live vocabulary too, so a set that drifted
// back to the old pair — or to any name a .theme file cannot key against —
// fails here rather than silently pinning nothing.
func TestLightPins_AreExactlyFourTokens(t *testing.T) {
	const want = 4

	wantSet := slices.Sorted(slices.Values(pinnedTokenNames))

	if got := len(pinnedTokenNames); got != want {
		t.Errorf("the eyeball-pinned set is %d tokens %v, want exactly %d (§13.5)", got, pinnedTokenNames, want)
	}
	if got := len(slices.Compact(slices.Clone(wantSet))); got != len(pinnedTokenNames) {
		t.Errorf("the eyeball-pinned set %v repeats a token — the count is of DISTINCT tokens", pinnedTokenNames)
	}

	vocabulary := theme.TokenNames()
	for _, token := range pinnedTokenNames {
		if !slices.Contains(vocabulary, token) {
			t.Errorf("pinned token %q is not in the closed vocabulary %v", token, vocabulary)
		}
	}

	for _, slug := range slices.Sorted(maps.Keys(lightPins)) {
		if pinned := slices.Sorted(maps.Keys(lightPins[slug])); !slices.Equal(pinned, wantSet) {
			t.Errorf("lightPins[%q] pins %v, want exactly the four eyeball-pinned tokens %v", slug, pinned, wantSet)
		}
	}
}

// TestLightPins_SkipDarkThemes proves the light-only scope is real in the one
// direction that matters: a dark built-in is neither pinned nor fill-checked
// here.
//
// It is not an exemption. A dark theme is covered in full by contrast_test.go,
// which auto-enumerates the embedded set and measures every §13.5 rule —
// including the three ≥ 1.10 fill legs — against that theme's own canvas. What
// it has no business carrying is a set of exact-value pins established by
// eyeball against a near-white surface it does not have.
func TestLightPins_SkipDarkThemes(t *testing.T) {
	dark := darkThemeSlugs(t)
	if len(dark) == 0 {
		t.Fatal("no embedded theme is enrolled dark — this test would pass vacuously")
	}

	light := lightThemeSlugs(t)
	for _, slug := range dark {
		if slices.Contains(light, slug) {
			t.Errorf("%q is enrolled dark yet appears in the light run set %v", slug, light)
		}
		if _, pinned := lightPins[slug]; pinned {
			t.Errorf("dark built-in %q carries a light pin row — the eyeball pins are light-only", slug)
		}
	}
}

// lightThemeSlugs returns the enrolled-light built-ins, sorted.
//
// It FATALS on an empty set for the reason every enumerating test in this
// package does: a loop over nothing passes everything, and a light-only suite
// with no light theme in it is the one shape that would report green while
// guarding none of the four un-derivable values.
func lightThemeSlugs(t *testing.T) []string {
	t.Helper()
	return enrolledSlugs(t, true)
}

// darkThemeSlugs returns the enrolled-dark built-ins, sorted.
func darkThemeSlugs(t *testing.T) []string {
	t.Helper()
	return enrolledSlugs(t, false)
}

// enrolledSlugs returns every slug the table enrols with the given appearance,
// sorted, having first checked the table is not empty.
func enrolledSlugs(t *testing.T, isLight bool) []string {
	t.Helper()

	if len(themeIsLight) == 0 {
		t.Fatal("the light/dark table is empty — every assertion driven by it would pass vacuously")
	}

	var slugs []string
	for slug, light := range themeIsLight {
		if light == isLight {
			slugs = append(slugs, slug)
		}
	}
	slices.Sort(slugs)
	return slugs
}

// tokenValue returns the theme's value for the named vocabulary token.
//
// The lookup goes through All() rather than a struct field so the pin tables
// key by the same token NAMES a .theme file writes — and so a pin naming
// something outside the vocabulary fatals instead of resolving to an empty
// string that would compare equal to nothing and pass.
func tokenValue(t *testing.T, th theme.Theme, name string) string {
	t.Helper()

	for _, token := range th.All() {
		if token.Name == name {
			return token.Value
		}
	}
	t.Fatalf("token %q is not in the theme's vocabulary %v", name, theme.TokenNames())
	return ""
}
