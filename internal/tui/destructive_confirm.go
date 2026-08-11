package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

const (
	destructiveTitleGlyph = "▲"
	// Wrap target and minimum content width, so the panel size is stable
	// regardless of target-name/path length.
	destructiveBodyWidth   = 52
	destructiveKeyCancel   = "esc"
	destructiveLabelCancel = "cancel"
)

type destructiveConfirmSpec struct {
	title         string
	targetName    string
	nameTrailer   string
	extraBodyRows []string
	consequence   string
	confirmKey    string
	confirmLabel  string
}

func renderDestructiveConfirm(spec destructiveConfirmSpec, th theme.Theme, colourless bool) string {
	header := []string{destructiveHeaderRow(spec.title, th, colourless)}
	body := destructiveBodyRows(spec, th, colourless)
	footer := []string{destructiveFooterRow(spec.confirmKey, spec.confirmLabel, th, colourless)}
	return renderJoinedPanel([][]string{header, body, footer}, th.Border, th, colourless)
}

// Glyph + bold carry the destructive signal under NO_COLOR, where the hue drops.
func destructiveHeaderRow(title string, th theme.Theme, colourless bool) string {
	style := headerStyle(th.StateDestructive, th, colourless).Bold(true)
	glyph := style.Render(destructiveTitleGlyph)
	gap := headerCanvasBg(th, colourless).Render(" ")
	titleSeg := style.Render(title)
	return lipgloss.JoinHorizontal(lipgloss.Top, glyph, gap, titleSeg)
}

func destructiveBodyRows(spec destructiveConfirmSpec, th theme.Theme, colourless bool) []string {
	rows := []string{destructiveNameRow(spec.targetName, spec.nameTrailer, th, colourless)}
	rows = append(rows, spec.extraBodyRows...)
	rows = append(rows, headerCanvasBg(th, colourless).Render(""))
	rows = append(rows, destructiveConsequenceRows(spec.consequence, th, colourless)...)
	return rows
}

func destructiveNameRow(name, trailer string, th theme.Theme, colourless bool) string {
	nameSeg := headerStyle(th.StateDestructive, th, colourless).Bold(true).Render(name)
	if trailer == "" {
		return nameSeg
	}
	gap := headerCanvasBg(th, colourless).Render("  ")
	trailerSeg := headerStyle(th.TextMuted, th, colourless).Render(trailer)
	return lipgloss.JoinHorizontal(lipgloss.Top, nameSeg, gap, trailerSeg)
}

func destructiveConsequenceRows(text string, th theme.Theme, colourless bool) []string {
	wrapped := ansi.Wordwrap(text, destructiveBodyWidth, "")
	style := headerStyle(th.TextMuted, th, colourless)
	lines := strings.Split(wrapped, "\n")
	rows := make([]string, 0, len(lines))
	for _, line := range lines {
		rows = append(rows, style.Render(line))
	}
	return rows
}

func destructiveFooterRow(confirmKey, confirmLabel string, th theme.Theme, colourless bool) string {
	return renderConfirmCancelFooter(confirmKey, confirmLabel, destructiveKeyCancel, destructiveLabelCancel, th, colourless)
}
