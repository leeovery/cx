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
	// Lower one-eighth block: sits at the bottom edge of its cell (unlike `─`)
	// and draws as one continuous bar with no inter-cell dashing (unlike `_`).
	headerRuleGlyph = "▁"
)

const headerFallbackWidth = 80

const minTerminalWidth = 40

// Sized to the rendered wordmark+caret and subtitle widths plus a gap; below
// headerWordmarkMinWidth the wordmark collapses to the compact form.
const (
	headerSubtitleMinWidth = 30
	headerWordmarkMinWidth = 13
)

// The budget computation and the render must resolve width through here, or
// they disagree on the header's height.
func headerWidthOrFallback(width int) int {
	if width <= 0 {
		return headerFallbackWidth
	}
	return width
}

func headerStyle(fg theme.Token, th theme.Theme, colourless bool) lipgloss.Style {
	if colourless {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().
		Foreground(fg.Color()).
		Background(th.Canvas.Color())
}

// Structural spacers must be painted so gaps are canvas, not terminal-bg islands.
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

// Do not insert a blank row between band and rule: it reads taller than the
// one-row gutter above the band and unbalances the wordmark. The trailing blank
// belongs here — applySectionHeader's surgery assumes the title sits on line 0.
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

func headerPadLeft(seg string, segWidth, w int, th theme.Theme, colourless bool) string {
	if segWidth >= w {
		return seg
	}
	pad := headerCanvasBg(th, colourless).Render(strings.Repeat(" ", w-segWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, pad, seg)
}
