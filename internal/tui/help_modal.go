package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

const (
	helpTitleGlyph  = "?"
	helpTitle       = "Keybindings"
	helpDismissHint = "esc close"
	helpColumnGap   = "   "
	// Sized for the widest glyph ("Home/End") so labels share a left edge.
	helpKeyColumnWidth = 10
)

func renderHelpModalContent(entries []keymapEntry, th theme.Theme, colourless bool) string {
	bodyRows := helpModalBodyRows(entries, th, colourless)

	// The title must be laid out here rather than inside renderJoinedPanel: its
	// width depends on contentWidth, since `esc close` right-aligns to it.
	contentWidth := lipgloss.Width(helpModalHeader(0, th, colourless))
	for _, r := range bodyRows {
		if w := lipgloss.Width(r); w > contentWidth {
			contentWidth = w
		}
	}
	title := helpModalHeader(contentWidth, th, colourless)

	return renderJoinedPanel([][]string{{title}, bodyRows}, th.Border, th, colourless)
}

// At or below the natural width (including the width-0 probe) it renders at
// natural width rather than dropping the hint or wrapping.
func helpModalHeader(width int, th theme.Theme, colourless bool) string {
	glyph := headerStyle(th.AccentPrimary, th, colourless).Bold(true).Render(helpTitleGlyph)
	gap := headerCanvasBg(th, colourless).Render(" ")
	title := headerStyle(th.TextPrimary, th, colourless).Bold(true).Render(helpTitle)
	left := lipgloss.JoinHorizontal(lipgloss.Top, glyph, gap, title)
	leftWidth := lipgloss.Width(left)

	dismiss := headerStyle(th.TextMuted, th, colourless).Render(helpDismissHint)
	dismissWidth := lipgloss.Width(dismiss)

	naturalWidth := leftWidth + 1 + dismissWidth
	spacerWidth := 1
	if width > naturalWidth {
		spacerWidth = width - leftWidth - dismissWidth
	}
	spacer := headerCanvasBg(th, colourless).Render(strings.Repeat(" ", spacerWidth))
	return lipgloss.JoinHorizontal(lipgloss.Top, left, spacer, dismiss)
}

func helpModalBody(entries []keymapEntry, th theme.Theme, colourless bool) string {
	return lipgloss.JoinVertical(lipgloss.Left, helpModalBodyRows(entries, th, colourless)...)
}

func helpModalBodyRows(entries []keymapEntry, th theme.Theme, colourless bool) []string {
	rows := make([]string, 0, len(entries))
	for _, e := range entries {
		// Skip the `?` self-entry — a help modal does not list its own open key.
		if e.RightAligned {
			continue
		}
		rows = append(rows, helpModalRow(e, th, colourless))
	}
	return rows
}

func helpModalRow(e keymapEntry, th theme.Theme, colourless bool) string {
	keyTok := th.AccentKey
	if isDestructiveHelpKey(e) {
		keyTok = th.StateDestructive
	}
	return keyColumnRow(
		helpKeyGlyph(e), helpActionLabel(e),
		headerStyle(keyTok, th, colourless).Bold(true),
		headerStyle(th.TextSecondary, th, colourless),
		helpKeyColumnWidth, helpColumnGap, th, colourless,
	)
}

func helpActionLabel(e keymapEntry) string {
	if e.HelpAction != "" {
		return e.HelpAction
	}
	return e.Action
}

func helpKeyGlyph(e keymapEntry) string {
	if e.HelpKey != "" {
		return e.HelpKey
	}
	return e.Key
}

// Reads the structural flag rather than matching key glyphs, so a future
// non-destructive `d`/`k` binding cannot render red.
func isDestructiveHelpKey(e keymapEntry) bool {
	return e.Destructive
}
