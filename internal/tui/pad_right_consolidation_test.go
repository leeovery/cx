package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

func preHeaderPadRight(seg string, segWidth, w int, th theme.Theme, colourless bool) string {
	if segWidth >= w {
		return seg
	}
	pad := headerCanvasBg(th, colourless).Render(strings.Repeat(" ", w-segWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, seg, pad)
}

func preNoticeBandPadRight(seg string, segWidth, w int, tint theme.Token, th theme.Theme, colourless bool) string {
	if segWidth >= w {
		return seg
	}
	pad := noticeBandTintStyle(tint, th, colourless).Render(strings.Repeat(" ", w-segWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, seg, pad)
}

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
				if got := padRightWithStyle(seg, segWidth, segWidth, fill); got != seg {
					t.Errorf("padRightWithStyle(%s, w==segWidth, th=%v col=%v) = %q, want unchanged %q",
						name, themeLabel(th), colourless, got, seg)
				}
				if got := padRightWithStyle(seg, segWidth, segWidth-2, fill); got != seg {
					t.Errorf("padRightWithStyle(%s, w<segWidth, th=%v col=%v) = %q, want unchanged %q",
						name, themeLabel(th), colourless, got, seg)
				}
			}
		}
	}
}

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
