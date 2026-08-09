package theme_test

import (
	"maps"
	"math"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/lucasb-eyer/go-colorful"
)

// This file is §13.5's contrast gate: the canonical rule set, measured over
// EVERY embedded theme.
//
// It is what the bundled tier means (§6.4). A built-in — or an accepted PR —
// must be valid AND good, because it carries Portal's name; a drop-in in the
// user's themes directory must only be valid, because whether it looks good is
// the user's business. Relaxing a floor for a named port was the one option
// ruled out, so the rules below carry no per-theme exception and no test here
// names a theme.
//
// Two properties make that claim real rather than aspirational:
//
//  1. The suite ENUMERATES the embedded set (embeddedThemes) instead of naming
//     palettes, so a new .theme file is floor-checked the moment it is
//     committed, with no test edit — the same "adding a built-in is adding a
//     file" property BuiltinSlugs gives the loader.
//  2. Every reference background is THE THEME'S OWN `canvas` token, never a
//     constant. A theme is one palette carrying its own surface (§3.1), so
//     there is no mode axis left to measure against: a light built-in is
//     checked against its light canvas by the same code, by construction.
//
// Three floors carry the whole set — 4.50 normal text, 3.00 large/UI, 1.10
// fill-perceptible — and the comparisons are plain >= / < against the raw
// ratio. Several shipped legs clear by a few ten-thousandths, so rounding
// before comparing would flip them.
//
// The math is WCAG 2.x relative luminance + contrast ratio. go-colorful's
// LinearRgb() applies the exact sRGB linearization WCAG specifies (v/12.92
// below 0.04045, ((v+0.055)/1.055)^2.4 above), so relative luminance is the
// standard 0.2126·R + 0.7152·G + 0.0722·B over the linearized channels. We
// validate the math against the known reference black/white = 21.00 in
// TestContrastMath, without which every assertion below could pass vacuously.

const (
	floorNormal          = 4.5  // §13.5 normal text
	floorLargeUI         = 3.0  // §13.5 large / bold / UI
	floorFillPerceptible = 1.10 // §13.5 fill-perceptible (a tint, NOT 3:1)

	// ratioIdentity is the ratio of a colour against itself. text.faint must be
	// strictly above it to be visible at all, which is the lower half of its
	// decorative band.
	ratioIdentity = 1.0
)

// relativeLuminance is the WCAG relative luminance of an sRGB hex. go-colorful's
// LinearRgb() is the WCAG sRGB linearization; the weighted sum is the standard
// luminance.
func relativeLuminance(t *testing.T, hex string) float64 {
	t.Helper()
	c, err := colorful.Hex(hex)
	if err != nil {
		t.Fatalf("parse hex %q: %v", hex, err)
	}
	r, g, b := c.LinearRgb()
	return 0.2126*r + 0.7152*g + 0.0722*b
}

// contrastRatio is the WCAG contrast ratio (L_lighter+0.05)/(L_darker+0.05).
func contrastRatio(t *testing.T, fg, bg string) float64 {
	t.Helper()
	l1 := relativeLuminance(t, fg)
	l2 := relativeLuminance(t, bg)
	if l1 < l2 {
		l1, l2 = l2, l1
	}
	return (l1 + 0.05) / (l2 + 0.05)
}

// TestContrastMath validates the WCAG implementation against the canonical
// reference: pure black on pure white is exactly 21:1. Without this anchor a
// silent bug in the luminance math would let every floor assertion below pass
// vacuously.
func TestContrastMath(t *testing.T) {
	got := contrastRatio(t, "#000000", "#FFFFFF")
	if math.Abs(got-21.0) > 0.01 {
		t.Fatalf("black/white contrast = %.4f, want 21.00 (WCAG math is wrong)", got)
	}
	// Identity sanity: a colour against itself is 1:1.
	if got := contrastRatio(t, "#737AA2", "#737AA2"); math.Abs(got-1.0) > 0.001 {
		t.Fatalf("self contrast = %.4f, want 1.00", got)
	}
}

// TestFloorsEnumerateTheEmbeddedSet pins the enrolment property every other test
// in this file rests on: the set measured is the set shipped.
//
// It is the assertion that makes "a new built-in is checked by default" a fact
// rather than a convention. A palette added to builtins/ but somehow missed by
// the enumeration would otherwise ship Portal-endorsed and unchecked, which is
// exactly the failure the bundled tier exists to prevent (§6.4).
func TestFloorsEnumerateTheEmbeddedSet(t *testing.T) {
	enrolled := slices.Sorted(maps.Keys(embeddedThemes(t)))

	if len(enrolled) == 0 {
		t.Fatal("no embedded theme is enrolled — every floor assertion in this file would pass vacuously")
	}
	if want := theme.BuiltinSlugs(); !slices.Equal(enrolled, want) {
		t.Errorf("the floor tests enrol %v, want every embedded built-in %v", enrolled, want)
	}
}

// TestForegroundFloorAgainstOwnCanvas is §13.5's foreground-vs-canvas table:
// every token that renders directly on the canvas, measured against that
// theme's own canvas.
//
// accent.primary takes the 3.00 large/UI floor rather than 4.50 because it
// renders bars and glyphs — the cursor, the selector bar, the loading bar — and
// never body text. Every other accent and state carries the normal-text floor.
//
// Six of the vocabulary's foregrounds and surfaces are deliberately absent.
// text.subtle and text.faint are BANDED — a ceiling as well as a floor — and
// have their own carriers; border and canvas carry no numeric floor at all; and
// the two pairing tokens, text.on-selection and text.on-attention, only ever
// render on a tint, so they are measured against that tint by the pair rules
// below rather than against the canvas they never touch.
func TestForegroundFloorAgainstOwnCanvas(t *testing.T) {
	themes := embeddedThemes(t)
	for _, slug := range slices.Sorted(maps.Keys(themes)) {
		th := themes[slug]
		rules := []struct {
			token string
			value string
			floor float64
		}{
			{"text.primary", th.TextPrimary.Value, floorNormal},
			{"text.secondary", th.TextSecondary.Value, floorNormal},
			{"text.tertiary", th.TextTertiary.Value, floorNormal},
			{"text.muted", th.TextMuted.Value, floorNormal},
			{"accent.primary", th.AccentPrimary.Value, floorLargeUI},
			{"accent.key", th.AccentKey.Value, floorNormal},
			{"accent.mode", th.AccentMode.Value, floorNormal},
			{"accent.attention", th.AccentAttention.Value, floorNormal},
			{"state.positive", th.StatePositive.Value, floorNormal},
			{"state.destructive", th.StateDestructive.Value, floorNormal},
		}

		for _, rule := range rules {
			t.Run(slug+"/"+rule.token, func(t *testing.T) {
				assertAtLeast(t, slug+" "+rule.token+" vs canvas", rule.value, th.Canvas.Value, rule.floor)
			})
		}
	}
}

// TestTextSubtleBand holds text.subtle inside the 3.00–4.49 band.
//
// The ceiling is the half that is easy to lose and the reason this is its own
// carrier: text.subtle must clear the UI floor AND stay below normal text, or
// it is not the de-emphasised step the ramp needs between text.muted and
// text.faint — it is just a second text.muted. A theme that brightens it into
// normal-text contrast fails here rather than silently flattening the ramp.
func TestTextSubtleBand(t *testing.T) {
	themes := embeddedThemes(t)
	for _, slug := range slices.Sorted(maps.Keys(themes)) {
		th := themes[slug]
		t.Run(slug+"/text.subtle", func(t *testing.T) {
			leg := slug + " text.subtle vs canvas"
			assertAtLeast(t, leg, th.TextSubtle.Value, th.Canvas.Value, floorLargeUI)
			assertBelow(t, leg, th.TextSubtle.Value, th.Canvas.Value, floorNormal)
		})
	}
}

// TestTextFaintDecorativeBand holds text.faint inside its decorative band —
// above the identity ratio and strictly below the UI floor.
//
// Reaching the UI floor is a FAILURE, not a pass. text.faint renders inactive
// dots, `+ add` and hints, and §2.5 forbids it carrying content a user must
// read; the band is what structurally stops a theme promoting it into a
// functional role by making it legible enough to look like one.
func TestTextFaintDecorativeBand(t *testing.T) {
	themes := embeddedThemes(t)
	for _, slug := range slices.Sorted(maps.Keys(themes)) {
		th := themes[slug]
		t.Run(slug+"/text.faint", func(t *testing.T) {
			leg := slug + " text.faint vs canvas"
			assertAbove(t, leg, th.TextFaint.Value, th.Canvas.Value, ratioIdentity)
			assertBelow(t, leg, th.TextFaint.Value, th.Canvas.Value, floorLargeUI)
		})
	}
}

// TestBgSelectionPairRule is §13.5's three-leg rule for the selection band. A
// tint is never judged alone: it is judged with the text that sits on it and
// the accent that marks it.
//
//  1. text.on-selection on the tint ≥ 4.50 — the selected row's name stays
//     legible on the band rather than on the canvas.
//  2. accent.primary vs canvas ≥ 3.00 — the selector bar carries the UI
//     distinction, which is what lets leg 3 be a whisper.
//  3. the fill vs canvas ≥ 1.10 — perceptible, deliberately NOT 3:1: a subtle
//     highlight tint cannot meet 3:1 against its own canvas and would stop
//     being subtle if it did.
//
// All three must clear together, and all three against the theme's own canvas.
func TestBgSelectionPairRule(t *testing.T) {
	themes := embeddedThemes(t)
	for _, slug := range slices.Sorted(maps.Keys(themes)) {
		th := themes[slug]
		t.Run(slug+"/bg.selection", func(t *testing.T) {
			assertAtLeast(t, slug+" text.on-selection on bg.selection", th.TextOnSelection.Value, th.BgSelection.Value, floorNormal)
			assertAtLeast(t, slug+" accent.primary bar vs canvas", th.AccentPrimary.Value, th.Canvas.Value, floorLargeUI)
			assertAtLeast(t, slug+" bg.selection fill vs canvas", th.BgSelection.Value, th.Canvas.Value, floorFillPerceptible)
		})
	}
}

// TestBgAttentionPairRule is the same three-leg rule for the warning band:
// text.on-attention on the tint, the accent.attention bar against the canvas,
// and a perceptible fill.
//
// It is a separate carrier from the selection band because the two bands are
// tuned independently — a theme can hold one and lose the other, and the port
// that has to fix it needs to be told which.
func TestBgAttentionPairRule(t *testing.T) {
	themes := embeddedThemes(t)
	for _, slug := range slices.Sorted(maps.Keys(themes)) {
		th := themes[slug]
		t.Run(slug+"/bg.attention", func(t *testing.T) {
			assertAtLeast(t, slug+" text.on-attention on bg.attention", th.TextOnAttention.Value, th.BgAttention.Value, floorNormal)
			assertAtLeast(t, slug+" accent.attention bar vs canvas", th.AccentAttention.Value, th.Canvas.Value, floorLargeUI)
			assertAtLeast(t, slug+" bg.attention fill vs canvas", th.BgAttention.Value, th.Canvas.Value, floorFillPerceptible)
		})
	}
}

// TestInlineFlashAttentionPairClearsFloor is the §11.2 inline-flash band's
// co-tuned-pair gate: the flash renders its message in text.on-attention ON the
// bg.attention tint, so that exact pairing must clear the normal-text floor.
//
// The flash reuses the single co-tuned pairing rather than inventing a
// flash-specific one, so this is the same numeric leg the pair rule checks. It
// is asserted distinctly so a regression that breaks the inline flash fails
// with a §11.2-scoped message rather than only as one leg of a band rule.
func TestInlineFlashAttentionPairClearsFloor(t *testing.T) {
	themes := embeddedThemes(t)
	for _, slug := range slices.Sorted(maps.Keys(themes)) {
		th := themes[slug]
		t.Run(slug+"/text.on-attention", func(t *testing.T) {
			assertAtLeast(t, slug+" §11.2 inline-flash message text.on-attention on bg.attention",
				th.TextOnAttention.Value, th.BgAttention.Value, floorNormal)
		})
	}
}

// TestPreviewPeekChromeClearsFloorAgainstCanvas is the §9.1 peek-mode chrome's
// gate: the frame and `◉ preview` marker (accent.mode), the session name
// (text.primary) and the counters plus right-aligned nav hints (text.muted) all
// render directly on the owned canvas, so each must clear the normal-text floor
// against it. The captured CONTENT is out-of-theme real ANSI and exempt.
//
// Every token here is already covered by the per-token gate above; this carrier
// exists so a regression that specifically breaks the preview chrome's
// legibility fails with a §9.1-scoped message naming the surface, not just the
// token.
func TestPreviewPeekChromeClearsFloorAgainstCanvas(t *testing.T) {
	themes := embeddedThemes(t)
	for _, slug := range slices.Sorted(maps.Keys(themes)) {
		th := themes[slug]
		chrome := []struct {
			token string
			role  string
			value string
		}{
			{"accent.mode", "frame + marker", th.AccentMode.Value},
			{"text.primary", "session name", th.TextPrimary.Value},
			{"text.muted", "counters + hints", th.TextMuted.Value},
		}

		for _, part := range chrome {
			t.Run(slug+"/"+part.token, func(t *testing.T) {
				assertAtLeast(t, slug+" §9.1 preview chrome "+part.role+" ("+part.token+") vs canvas",
					part.value, th.Canvas.Value, floorNormal)
			})
		}
	}
}

// TestBgSubtlePairRule is the single-leg rule for bg.subtle: fill vs canvas
// only.
//
// It is one leg rather than three because NOTHING renders on it — it is the
// loading bar's empty track, and the filled portion is drawn in accent.primary
// rather than as text on the tint. There is no on-band foreground to keep
// legible and no accent bar marking it, so a perceptible fill is the whole
// rule.
func TestBgSubtlePairRule(t *testing.T) {
	themes := embeddedThemes(t)
	for _, slug := range slices.Sorted(maps.Keys(themes)) {
		th := themes[slug]
		t.Run(slug+"/bg.subtle", func(t *testing.T) {
			assertAtLeast(t, slug+" bg.subtle fill vs canvas", th.BgSubtle.Value, th.Canvas.Value, floorFillPerceptible)
		})
	}
}

// TestForegroundOnTintPairings is §13.5's foreground-on-tint table: every
// foreground that renders on a tint, measured against that tint rather than
// against the canvas.
//
// These are the legs that catch a palette whose tokens each clear the canvas
// individually but collide on the selected row — the Nord port's second
// correction was found exactly here. text.primary on bg.selection is
// deliberately NOT a leg: §13.5 omits it because the selected row's name
// renders in text.on-selection, and it was walked during the Nord port as free
// information rather than as a gate.
func TestForegroundOnTintPairings(t *testing.T) {
	themes := embeddedThemes(t)
	for _, slug := range slices.Sorted(maps.Keys(themes)) {
		th := themes[slug]
		pairings := []struct {
			foreground string
			tint       string
			fg, bg     string
		}{
			{"text.on-selection", "bg.selection", th.TextOnSelection.Value, th.BgSelection.Value},
			{"text.secondary", "bg.selection", th.TextSecondary.Value, th.BgSelection.Value},
			{"text.tertiary", "bg.selection", th.TextTertiary.Value, th.BgSelection.Value},
			{"state.positive", "bg.selection", th.StatePositive.Value, th.BgSelection.Value},
			{"text.on-attention", "bg.attention", th.TextOnAttention.Value, th.BgAttention.Value},
		}

		for _, pair := range pairings {
			t.Run(slug+"/"+pair.foreground+" on "+pair.tint, func(t *testing.T) {
				assertAtLeast(t, slug+" "+pair.foreground+" on "+pair.tint, pair.fg, pair.bg, floorNormal)
			})
		}
	}
}

// TestStatePositiveClearsCanvasAndSelection is the dual-clearance leg: one token
// renders in two places, so it must clear both.
//
// state.positive is the `●` attached marker and the Sessions count on the
// canvas AND on the selected row, and there is no per-context override to hide
// behind — the vocabulary is closed. This is the leg that caught the Nord green
// (6.13 on canvas but 4.23 on the selection tint) and the one MV itself solved
// by darkening its light green, which is why it is its own named carrier rather
// than one row folded into the pairing table above.
func TestStatePositiveClearsCanvasAndSelection(t *testing.T) {
	themes := embeddedThemes(t)
	for _, slug := range slices.Sorted(maps.Keys(themes)) {
		th := themes[slug]
		t.Run(slug+"/state.positive", func(t *testing.T) {
			assertAtLeast(t, slug+" state.positive vs canvas", th.StatePositive.Value, th.Canvas.Value, floorNormal)
			assertAtLeast(t, slug+" state.positive on bg.selection", th.StatePositive.Value, th.BgSelection.Value, floorNormal)
		})
	}
}

// embeddedThemes loads every built-in through the ordinary loader, keyed by
// slug.
//
// The set is DERIVED from BuiltinSlugs rather than listed here, which is the
// whole enrolment property: no test in this file names a theme, so adding a
// .theme file to builtins/ floor-checks it with no test edit.
//
// It FATALS on an empty set, because a suite that measures nothing passes
// everything — the one failure mode a loop over an enumerated set can silently
// have. It fatals on a rejection too: §7.6 makes a broken built-in a build-time
// failure, so reaching one here means the floors are being asked about a
// palette that does not exist.
//
// The event logger is a discarding one: this is diagnosis, not use, so the
// §12.3 trail has nothing to record.
func embeddedThemes(t *testing.T) map[string]theme.Theme {
	t.Helper()

	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("the embedded set is empty — every floor assertion in this file would pass vacuously")
	}

	loader := theme.NewSilentLoader()
	themes := make(map[string]theme.Theme, len(slugs))
	for _, slug := range slugs {
		result, rejection, found := loader.LoadBuiltin(slug)
		if !found {
			t.Fatalf("LoadBuiltin(%q) reported not found, want the embedded built-in", slug)
		}
		if rejection != nil {
			t.Fatalf("LoadBuiltin(%q) rejected the embedded file: %v", slug, rejection)
		}
		themes[slug] = result.Theme
	}
	return themes
}

// assertAtLeast fails unless fg clears floor against bg.
//
// The three assertion helpers compare the RAW ratio — no epsilon, no rounding
// to two decimals first — because several shipped legs clear by a few
// ten-thousandths, and a value rounded before comparison flips them from pass
// to fail. The message reports four decimals for the same reason: whoever is
// porting a palette needs to see how close the margin is, alongside the theme,
// the leg and both hexes so they know which value to move.
func assertAtLeast(t *testing.T, leg, fg, bg string, floor float64) {
	t.Helper()
	if got := contrastRatio(t, fg, bg); got < floor {
		t.Errorf("%s: %s on %s = %.4f, want >= %.2f", leg, fg, bg, got, floor)
	}
}

// assertAbove fails unless fg is strictly above floor against bg — the strict
// lower bound of text.faint's decorative band.
func assertAbove(t *testing.T, leg, fg, bg string, floor float64) {
	t.Helper()
	if got := contrastRatio(t, fg, bg); got <= floor {
		t.Errorf("%s: %s on %s = %.4f, want > %.2f", leg, fg, bg, got, floor)
	}
}

// assertBelow fails unless fg stays under ceiling against bg — the banded
// tokens' upper bound, where clearing MORE contrast is the failure.
func assertBelow(t *testing.T, leg, fg, bg string, ceiling float64) {
	t.Helper()
	if got := contrastRatio(t, fg, bg); got >= ceiling {
		t.Errorf("%s: %s on %s = %.4f, want < %.2f", leg, fg, bg, got, ceiling)
	}
}
