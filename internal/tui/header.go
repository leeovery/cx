package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

const (
	fullWordmark          = "P O R T A L"
	headerCompactWordmark = "PORTAL"
	headerCaret           = "▌"
	headerSubtitle        = "session manager"
	// Lower one-eighth block: it sits at the bottom edge of its cell (unlike
	// `─`) and draws as one continuous bar with no inter-cell dashing (unlike
	// the underscore).
	headerRuleGlyph = "▁"
)

const headerFallbackWidth = 80

const minTerminalWidth = 40

// Wordmark+caret is 13 cells and the subtitle 15, so the band needs
// 13 + 2 + 15 = 30 before the subtitle crowds the wordmark; below 13 the
// wordmark collapses to the compact form.
const (
	headerSubtitleMinWidth = 30
	headerWordmarkMinWidth = 13
)

// Single width source so the budget computation and the render agree exactly
// on the header's height.
func headerWidthOrFallback(width int) int {
	if width <= 0 {
		return headerFallbackWidth
	}
	return width
}

// headerStyle is the leaf paint: the role token's foreground over the theme's
// canvas, or a bare style under NO_COLOR.
func headerStyle(fg theme.Token, th theme.Theme, colourless bool) lipgloss.Style {
	if colourless {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Foreground(fg.Color()).
		Background(th.Canvas.Color())
}

// headerCanvasBg paints structural spacers so gaps are canvas, not terminal-bg
// islands.
func headerCanvasBg(th theme.Theme, colourless bool) lipgloss.Style {
	if colourless {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Background(th.Canvas.Color())
}

func headerWordmarkFor(width int) string {
	if width < headerWordmarkMinWidth {
		return headerCompactWordmark
	}
	return fullWordmark
}

func headerShowsSubtitle(width int) bool {
	return width >= headerSubtitleMinWidth
}

func headerSeparatorRule(width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	rule := strings.Repeat(headerRuleGlyph, w)
	return headerStyle(th.Border, th, colourless).Render(rule)
}

func blankCanvasRow(w int, th theme.Theme, colourless bool) string {
	return headerCanvasBg(th, colourless).Render(strings.Repeat(" ", w))
}

// The wordmark→rule gap is deliberately flush — do not insert a blank row: a
// blank row between two glyph rows reads taller than the one-row gutter above
// the band, so it unbalances rather than balances the wordmark. The trailing
// blank belongs to this block, not to the section header: it is then covered
// by the single headerHeight measurement, and applySectionHeader's line-0
// string surgery assumes the title sits on line 0.
func renderHeaderBlock(width int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	band := headerBand(w, th, colourless)
	rule := headerSeparatorRule(w, th, colourless)
	blank := blankCanvasRow(w, th, colourless)
	return lipgloss.JoinVertical(lipgloss.Left, band, rule, blank)
}

func headerBand(w int, th theme.Theme, colourless bool) string {
	wordmark := headerStyle(th.TextPrimary, th, colourless).Bold(true).
		Render(headerWordmarkFor(w))
	caret := headerStyle(th.AccentPrimary, th, colourless).Render(headerCaret)
	gap := headerCanvasBg(th, colourless).Render(" ")
	left := lipgloss.JoinHorizontal(lipgloss.Top, wordmark, gap, caret)

	leftWidth := lipgloss.Width(left)

	if !headerShowsSubtitle(w) || leftWidth >= w {
		return headerPadRight(left, leftWidth, w, th, colourless)
	}

	subtitle := headerStyle(th.TextMuted, th, colourless).Render(headerSubtitle)
	subWidth := lipgloss.Width(subtitle)

	// Drop the subtitle rather than overflow.
	if leftWidth+1+subWidth > w {
		return headerPadRight(left, leftWidth, w, th, colourless)
	}

	spacerWidth := w - leftWidth - subWidth
	spacer := headerCanvasBg(th, colourless).Render(strings.Repeat(" ", spacerWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, subtitle)
}

func padRightWithStyle(seg string, segWidth, w int, fill lipgloss.Style) string {
	if segWidth >= w {
		return seg
	}
	pad := fill.Render(strings.Repeat(" ", w-segWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, seg, pad)
}

func headerPadRight(seg string, segWidth, w int, th theme.Theme, colourless bool) string {
	return padRightWithStyle(seg, segWidth, w, headerCanvasBg(th, colourless))
}

// Mirror of headerPadRight, for the footer's bottom degrade rung where the
// `? help` anchor survives alone with no cluster to flex a spacer against.
func headerPadLeft(seg string, segWidth, w int, th theme.Theme, colourless bool) string {
	if segWidth >= w {
		return seg
	}
	pad := headerCanvasBg(th, colourless).Render(strings.Repeat(" ", w-segWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, pad, seg)
}
