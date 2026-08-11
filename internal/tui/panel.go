package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

const (
	panelRuleGlyph = "─"
	// The per-row L/R inset for content rows. The divider does not take it — it
	// runs the full inner width so its `├`/`┤` junctions meet both side borders.
	panelRowInset = 2

	panelFrameTopLeft     = "╭"
	panelFrameTopRight    = "╮"
	panelFrameBottomLeft  = "╰"
	panelFrameBottomRight = "╯"
	panelFrameSide        = "│"
	panelFrameTeeLeft     = "├"
	panelFrameTeeRight    = "┤"
)

// Each compartment is a slice of already-styled rows at their natural width;
// dividers are drawn between adjacent compartments. Spacing is flush — a caller
// wanting a blank row passes an empty content row, which is inset and bordered.
func renderJoinedPanel(compartments [][]string, borderToken theme.Token, th theme.Theme, colourless bool) string {
	contentWidth := 0
	totalRows := 0
	for _, comp := range compartments {
		totalRows += len(comp)
		for _, r := range comp {
			if w := lipgloss.Width(r); w > contentWidth {
				contentWidth = w
			}
		}
	}
	innerWidth := contentWidth + 2*panelRowInset

	rows := make([]string, 0, totalRows+len(compartments)+1)
	rows = append(rows, panelFrameTop(innerWidth, borderToken, th, colourless))
	for i, comp := range compartments {
		if i > 0 {
			rows = append(rows, panelFrameDivider(innerWidth, borderToken, th, colourless))
		}
		for _, r := range comp {
			row := panelInsetRow(r, contentWidth, th, colourless)
			rows = append(rows, panelFrameContentLine(row, borderToken, th, colourless))
		}
	}
	rows = append(rows, panelFrameBottom(innerWidth, borderToken, th, colourless))
	return lipgloss.JoinVertical(lipgloss.Left, rows...)
}

// No background — the frame glyphs sit on whatever the placed canvas supplies.
func panelFrameStyle(borderToken theme.Token, th theme.Theme, colourless bool) lipgloss.Style {
	if colourless {
		return lipgloss.NewStyle()
	}
	return lipgloss.NewStyle().Foreground(borderToken.Color())
}

func panelFrameTop(w int, borderToken theme.Token, th theme.Theme, colourless bool) string {
	line := panelFrameTopLeft + strings.Repeat(panelRuleGlyph, w) + panelFrameTopRight
	return panelFrameStyle(borderToken, th, colourless).Render(line)
}

func panelFrameBottom(w int, borderToken theme.Token, th theme.Theme, colourless bool) string {
	line := panelFrameBottomLeft + strings.Repeat(panelRuleGlyph, w) + panelFrameBottomRight
	return panelFrameStyle(borderToken, th, colourless).Render(line)
}

func panelFrameDivider(w int, borderToken theme.Token, th theme.Theme, colourless bool) string {
	line := panelFrameTeeLeft + strings.Repeat(panelRuleGlyph, w) + panelFrameTeeRight
	return panelFrameStyle(borderToken, th, colourless).Render(line)
}

func panelFrameContentLine(row string, borderToken theme.Token, th theme.Theme, colourless bool) string {
	side := panelFrameStyle(borderToken, th, colourless).Render(panelFrameSide)
	return lipgloss.JoinHorizontal(lipgloss.Top, side, row, side)
}

// Inset and pad are canvas-painted so the row leaves no terminal-bg island.
func panelInsetRow(row string, contentWidth int, th theme.Theme, colourless bool) string {
	inset := headerCanvasBg(th, colourless).Render(strings.Repeat(" ", panelRowInset))
	padded := headerPadRight(row, lipgloss.Width(row), contentWidth, th, colourless)
	return lipgloss.JoinHorizontal(lipgloss.Top, inset, padded, inset)
}
