package capture

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// swatchTestPalette builds a synthetic 19-token palette carrying canvas, with
// every other value distinct and prefixed by hi.
//
// Distinctness is what makes an assertion mean something: a value that reached
// the render cannot have got there by coincidence, and two palettes built with
// different prefixes share no value at all, so "this render carries THAT
// theme's colours" is decidable.
func swatchTestPalette(canvas, hi string) theme.Theme {
	value := func(n int) theme.Token {
		return theme.Token{Value: fmt.Sprintf("#%s%04d", hi, n)}
	}
	return theme.Theme{
		TextPrimary:      value(1),
		TextSecondary:    value(2),
		TextTertiary:     value(3),
		TextMuted:        value(4),
		TextSubtle:       value(5),
		TextFaint:        value(6),
		TextOnSelection:  value(7),
		AccentPrimary:    value(8),
		AccentKey:        value(9),
		AccentMode:       value(10),
		AccentAttention:  value(11),
		StatePositive:    value(12),
		StateDestructive: value(13),
		Canvas:           theme.Token{Value: canvas},
		BgSelection:      value(15),
		BgAttention:      value(16),
		BgSubtle:         value(17),
		Border:           value(18),
		TextOnAttention:  value(19),
	}
}

// wantStyle builds the style an assertion EXPECTS a token pairing to be rendered
// in, from the tokens alone.
//
// It deliberately does NOT delegate to the production onCanvas / onTint helpers,
// and must never be refactored to: an expectation routed through the helper under
// test moves with it, so both sides of the comparison stay equal under any
// mutation of that helper and the assertion asserts nothing. Constructing the
// style independently is what makes a swapped foreground/background, or a
// hardcoded canvas in place of the theme's own, a test failure — and the swatch is
// the only route to seeing a theme's visual change (§13.1), so a silently-wrong
// one is expensive.
//
// This mirrors the internal/tui probe convention (header_test.go's tokenFgSeq),
// which likewise builds its reference render straight from lipgloss.
func wantStyle(fg, bg theme.Token) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(fg.Color()).Background(bg.Color())
}

// TestSwatch_RendersFromInjectedTheme asserts the swatch renders the palette it
// is HANDED rather than a compiled-in one — the whole point of task 2-4, since a
// swatch that could only show one palette cannot satisfy a visual gate on a
// theme that is not that palette.
//
// Two synthetic palettes share no value, so the injected one's colours appearing
// while the other's are absent is decisive rather than coincidental.
func TestSwatch_RendersFromInjectedTheme(t *testing.T) {
	injected := swatchTestPalette("#0B0C14", "AA")
	other := swatchTestPalette("#E1E2E7", "BB")

	out := renderSwatch(injected)

	for _, tok := range []theme.Token{injected.Canvas, injected.BgSelection, injected.BgAttention, injected.BgSubtle, injected.Border} {
		if !strings.Contains(out, tok.Value) {
			t.Errorf("swatch does not carry the injected theme's %s\n--- swatch ---\n%s", tok.Value, out)
		}
	}

	for _, tok := range other.All() {
		if strings.Contains(out, tok.Value) {
			t.Errorf("swatch carries %s, which belongs to a theme it was never handed", tok.Value)
		}
	}

	if renderSwatch(other) == out {
		t.Error("two disjoint palettes render byte-identical swatches — the theme is not driving the render")
	}
}

// TestSwatch_UsesThemeOwnCanvas asserts every band is painted on the THEME's own
// canvas, not on a mode-selected constant.
//
// This is what makes the gate honest for a light theme: the light-tint-on-light-
// canvas wash-out is exactly the risk the numeric floors cannot catch (§13.5), so
// a light palette judged against a dark reference would prove nothing at all.
func TestSwatch_UsesThemeOwnCanvas(t *testing.T) {
	cases := []struct {
		name  string
		theme theme.Theme
	}{
		{"dark theme", swatchTestPalette("#0B0C14", "AA")},
		{"light theme", swatchTestPalette("#E1E2E7", "BB")},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := newSwatchModel(c.theme)

			if got := s.canvasHex(); got != c.theme.Canvas.Value {
				t.Errorf("canvasHex() = %q, want %q", got, c.theme.Canvas.Value)
			}
			if got := s.View().BackgroundColor; got != c.theme.Canvas.Color() {
				t.Errorf("View().BackgroundColor = %v, want the theme's canvas %v", got, c.theme.Canvas.Color())
			}

			out := renderSwatch(c.theme)
			if title := fmt.Sprintf("CONTRAST VALIDATION — canvas %s", c.theme.Canvas.Value); !strings.Contains(out, title) {
				t.Errorf("swatch title does not read %q\n--- swatch ---\n%s", title, out)
			}
			wantLabel := wantStyle(c.theme.TextMuted, c.theme.Canvas).
				Render(fmt.Sprintf("border  %s", c.theme.Border.Value))
			if !strings.Contains(out, wantLabel) {
				t.Errorf("the border label is not painted on the theme's canvas\nwant %q\n--- swatch ---\n%s", wantLabel, out)
			}
			// The border rule itself is the pinned tint judged here, and it is
			// judged solely as a foreground on the canvas — so it, not just its
			// label, has to carry this theme's canvas.
			wantRule := wantStyle(c.theme.Border, c.theme.Canvas).Render(strings.Repeat("─", bandWidth))
			if got := borderRule(c.theme); got != wantRule {
				t.Errorf("the border rule is not painted on the theme's canvas\n got %q\nwant %q", got, wantRule)
			}
		})
	}
}

// TestSwatchBandsCoverEveryPinnedTint asserts the swatch renders a labelled band
// for each of the four eyeball-pinned tints (§13.5), every band carrying its
// token name AND that theme's value, so the human can read exactly what they are
// judging.
//
// The count is four, not five: the §2.2 consolidation collapsed
// border.separator / border.footer into the single `border`, and §13.5 makes the
// count load-bearing.
func TestSwatchBandsCoverEveryPinnedTint(t *testing.T) {
	th := swatchTestPalette("#E1E2E7", "CC")
	out := renderSwatch(th)

	pinned := []struct {
		label string
		token theme.Token
	}{
		{"bg.selection", th.BgSelection},
		{"bg.attention", th.BgAttention},
		{"bg.subtle", th.BgSubtle},
		{"border", th.Border},
	}

	for _, tint := range pinned {
		want := fmt.Sprintf("%s  %s", tint.label, tint.token.Value)
		if !strings.Contains(out, want) {
			t.Errorf("swatch is missing the labelled band %q\n--- swatch ---\n%s", want, out)
		}
	}

	// Every band is a continuous surface of exactly bandWidth cells, so the four
	// tints are judged side by side rather than at four different widths.
	surfaces := map[string]string{
		"bg.selection": selectionBand(th),
		"bg.attention": attentionBand(th),
		"bg.subtle":    subtleBand(th),
		"border":       borderRule(th),
	}
	for label, surface := range surfaces {
		if got := lipgloss.Width(surface); got != bandWidth {
			t.Errorf("the %s band is %d cells wide, want %d", label, got, bandWidth)
		}
		// And every band actually reaches the composed swatch. The labels above
		// are captions, not surfaces: a band dropped from renderSwatch leaves its
		// caption behind, so the label assertions alone cannot see it go. §13.1
		// makes this swatch the only route to SEEING a theme, so a silently-
		// missing band means a pinned tint is never judged at all.
		//
		// The band helper is a legitimate subject here because its styling is
		// independently pinned above (wantStyle, the width and fill assertions) —
		// what is under test is composition, not what the band looks like.
		if !strings.Contains(out, surface) {
			t.Errorf("the %s band is not in the composed swatch\n--- swatch ---\n%s", label, out)
		}
	}

	// bg.subtle carries no foreground pairing (§13.5 — nothing renders on it),
	// so its fill is asserted here: the accent.primary bar over the tint's own
	// empty track.
	bar := lipgloss.NewStyle().Background(th.AccentPrimary.Color()).Render(strings.Repeat(" ", subtleBarWidth))
	track := lipgloss.NewStyle().Background(th.BgSubtle.Color()).Render(strings.Repeat(" ", bandWidth-subtleBarWidth))
	if got := surfaces["bg.subtle"]; got != bar+track {
		t.Errorf("the bg.subtle band is not the accent.primary bar over the bg.subtle track\n got %q\nwant %q", got, bar+track)
	}
}

// TestSwatchCoversForegroundOnTintPairings asserts the swatch renders every
// foreground-on-tint pairing §13.5 lists, each one ON its tint rather than on the
// canvas: text.on-selection, text.tertiary, text.secondary and state.positive on
// bg.selection, and text.on-attention on bg.attention.
//
// state.positive is the load-bearing one — a single token rendering both on the
// canvas and on the selected row, so it must clear both floors, and the tint leg
// is only judgeable by eye.
func TestSwatchCoversForegroundOnTintPairings(t *testing.T) {
	th := swatchTestPalette("#0B0C14", "DD")
	out := renderSwatch(th)

	pairings := []struct {
		label string
		band  string
		want  string
	}{
		{"text.on-selection", selectionBand(th),
			wantStyle(th.TextOnSelection, th.BgSelection).Bold(true).Render(swatchSessionName)},
		{"text.tertiary", selectionBand(th),
			wantStyle(th.TextTertiary, th.BgSelection).Render(swatchSessionPath)},
		{"text.secondary", selectionBand(th),
			wantStyle(th.TextSecondary, th.BgSelection).Render(swatchWindowCount)},
		{"state.positive", selectionBand(th),
			wantStyle(th.StatePositive, th.BgSelection).Render(swatchAttached)},
		{"text.on-attention", attentionBand(th),
			wantStyle(th.TextOnAttention, th.BgAttention).Render(swatchAttentionMsg)},
	}

	for _, pairing := range pairings {
		if !strings.Contains(pairing.band, pairing.want) {
			t.Errorf("%s is not rendered on its tint\n--- band ---\n%q", pairing.label, pairing.band)
		}
		// The caption names the pairing, so the human reads which token each
		// on-tint foreground is rather than guessing from the colour.
		if !strings.Contains(out, pairing.label) {
			t.Errorf("no caption names the %s pairing\n--- swatch ---\n%s", pairing.label, out)
		}
	}

	// The two on-tint glyphs carry meaning of their own and must survive.
	for _, glyph := range []string{"●", "⚠"} {
		if !strings.Contains(out, glyph) {
			t.Errorf("swatch is missing the %q glyph\n--- swatch ---\n%s", glyph, out)
		}
	}
}
