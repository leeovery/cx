package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// Task spectrum-tui-design-10-1 consolidation gate. headerPadRight (header.go) and
// noticeBandPadRight (notice_band.go) were byte-for-byte the same right-pad
// geometry — the `segWidth >= w` return-unchanged guard plus a JoinHorizontal of a
// styled strings.Repeat(" ", w-segWidth) pad — differing ONLY in the fill style
// (the owned canvas vs the band's tint). They now both route through the single
// padRightWithStyle core. These tests pin the shared geometry at the new owner and
// prove each wrapper delegates with its distinct fill, producing byte-identical
// output to a verbatim reproduction of the pre-refactor body.
//
// No t.Parallel() — the shared style helpers make parallelism unsafe.

// preHeaderPadRight reproduces the ORIGINAL headerPadRight inline body verbatim —
// the golden the consolidation must preserve.
func preHeaderPadRight(seg string, segWidth, w int, th theme.Theme, colourless bool) string {
	if segWidth >= w {
		return seg
	}
	pad := headerCanvasBg(th, colourless).Render(strings.Repeat(" ", w-segWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, seg, pad)
}

// preNoticeBandPadRight reproduces the ORIGINAL noticeBandPadRight inline body
// verbatim — the golden the consolidation must preserve.
func preNoticeBandPadRight(seg string, segWidth, w int, tint theme.Token, th theme.Theme, colourless bool) string {
	if segWidth >= w {
		return seg
	}
	pad := noticeBandTintStyle(tint, th, colourless).Render(strings.Repeat(" ", w-segWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, seg, pad)
}

// TestPadRightWithStyle_ReturnsSegUnchangedWhenFull pins the guard clause: a
// segment already at/over w is returned verbatim (no pad joined), for both a
// canvas fill and a tint fill, across modes × colourless.
func TestPadRightWithStyle_ReturnsSegUnchangedWhenFull(t *testing.T) {
	const seg = "PORTAL"
	segWidth := lipgloss.Width(seg)
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		for _, colourless := range []bool{false, true} {
			fills := map[string]lipgloss.Style{
				"canvas": headerCanvasBg(th, colourless),
				"tint":   noticeBandTintStyle(th.BgSelection, th, colourless),
			}
			for name, fill := range fills {
				// segWidth == w
				if got := padRightWithStyle(seg, segWidth, segWidth, fill); got != seg {
					t.Errorf("padRightWithStyle(%s, w==segWidth, th=%v col=%v) = %q, want unchanged %q",
						name, themeLabel(th), colourless, got, seg)
				}
				// segWidth > w
				if got := padRightWithStyle(seg, segWidth, segWidth-2, fill); got != seg {
					t.Errorf("padRightWithStyle(%s, w<segWidth, th=%v col=%v) = %q, want unchanged %q",
						name, themeLabel(th), colourless, got, seg)
				}
			}
		}
	}
}

// TestPadRightWithStyle_JoinsStyledPadOfCorrectWidth pins the fill path: a segment
// narrower than w is joined with a fill-styled pad of exactly w-segWidth cells,
// byte-identically to a verbatim canvas-fill / tint-fill JoinHorizontal.
func TestPadRightWithStyle_JoinsStyledPadOfCorrectWidth(t *testing.T) {
	const seg = "hi"
	segWidth := lipgloss.Width(seg)
	const w = 10
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		for _, colourless := range []bool{false, true} {
			cases := map[string]lipgloss.Style{
				"canvas": headerCanvasBg(th, colourless),
				"tint":   noticeBandTintStyle(th.BgSelection, th, colourless),
			}
			for name, fill := range cases {
				want := lipgloss.JoinHorizontal(lipgloss.Top, seg, fill.Render(strings.Repeat(" ", w-segWidth)))
				got := padRightWithStyle(seg, segWidth, w, fill)
				if got != want {
					t.Errorf("padRightWithStyle(%s, th=%v col=%v) = %q, want %q",
						name, themeLabel(th), colourless, got, want)
				}
				if width := lipgloss.Width(got); width != w {
					t.Errorf("padRightWithStyle(%s, th=%v col=%v) width = %d, want %d",
						name, themeLabel(th), colourless, width, w)
				}
			}
		}
	}
}

// TestHeaderPadRight_DelegatesToPadRightWithStyle proves headerPadRight is a thin
// wrapper binding the canvas fill: its render is byte-identical to the pre-refactor
// inline body across width/mode/colourless.
func TestHeaderPadRight_DelegatesToPadRightWithStyle(t *testing.T) {
	const seg = "PORTAL ▌"
	segWidth := lipgloss.Width(seg)
	for _, w := range []int{0, segWidth - 1, segWidth, segWidth + 1, 40, 80} {
		for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
			for _, colourless := range []bool{false, true} {
				want := preHeaderPadRight(seg, segWidth, w, th, colourless)
				got := headerPadRight(seg, segWidth, w, th, colourless)
				if got != want {
					t.Errorf("headerPadRight(w=%d th=%v col=%v) = %q, want pre-refactor %q",
						w, themeLabel(th), colourless, got, want)
				}
			}
		}
	}
}

// TestNoticeBandPadRight_DelegatesToPadRightWithStyle proves noticeBandPadRight is
// a thin wrapper binding the tint fill: its render is byte-identical to the
// pre-refactor inline body across width/tint/mode/colourless.
func TestNoticeBandPadRight_DelegatesToPadRightWithStyle(t *testing.T) {
	const seg = "▌ no tags yet"
	segWidth := lipgloss.Width(seg)
	tintRoles := []string{"bg.selection", "bg.attention"}
	for _, tintRole := range tintRoles {
		for _, w := range []int{0, segWidth - 1, segWidth, segWidth + 1, 40, 80} {
			for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
				tint := tokenNamed(t, th, tintRole)
				for _, colourless := range []bool{false, true} {
					want := preNoticeBandPadRight(seg, segWidth, w, tint, th, colourless)
					got := noticeBandPadRight(seg, segWidth, w, tint, th, colourless)
					if got != want {
						t.Errorf("noticeBandPadRight(tint=%s w=%d th=%v col=%v) = %q, want pre-refactor %q",
							tint.Name, w, themeLabel(th), colourless, got, want)
					}
				}
			}
		}
	}
}
