package tui

import (
	"charm.land/bubbles/v2/textinput"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

const (
	renameTitle      = "Rename session"
	renameFieldLabel = "NEW NAME"
	renameWasPrefix  = "was: "

	// Anchors the panel's body width so the panel size is stable regardless of
	// value/old-name length.
	renameInputInnerWidth = 44

	renameKeyConfirm   = "⏎"
	renameLabelConfirm = "rename"
	renameKeyCancel    = "esc"
	renameLabelCancel  = "cancel"
)

func renderRenameModalContent(input textinput.Model, oldName string, th theme.Theme, colourless bool) string {
	header := []string{renameModalHeaderRow(th, colourless)}
	body := renameModalBodyRows(input, oldName, th, colourless)
	footer := []string{renameModalFooterRow(th, colourless)}
	return renderJoinedPanel([][]string{header, body, footer}, th.Border, th, colourless)
}

// The rename input is always editing — there is no navigate state — so the
// badge is always shown.
func renameModalHeaderRow(th theme.Theme, colourless bool) string {
	title := headerStyle(th.TextPrimary, th, colourless).Bold(true).Render(renameTitle)
	return renderHeaderWithBadge(title, renamePanelContentWidth(), true, th, colourless)
}

// The input box is the widest body element and anchors the panel width, so the
// header pinned to this width matches the box edge.
func renamePanelContentWidth() int {
	return renameInputInnerWidth + 2
}

func renameModalBodyRows(input textinput.Model, oldName string, th theme.Theme, colourless bool) []string {
	rows := []string{renameModalLabelRow(th, colourless)}
	rows = append(rows, renameModalInputBoxRows(input, th, colourless)...)
	rows = append(rows, renameModalWasRow(oldName, th, colourless))
	return rows
}

func renameModalLabelRow(th theme.Theme, colourless bool) string {
	return headerStyle(th.AccentPrimary, th, colourless).Render(renameFieldLabel)
}

// Each rendered line is one body row, since renderJoinedPanel expects
// single-line rows.
func renameModalInputBoxRows(input textinput.Model, th theme.Theme, colourless bool) []string {
	value := renameInputView(input, th, colourless)
	return renderInputBox(value, inputBoxEditing, true, renameInputInnerWidth, th, colourless)
}

// The inline prompt is cleared so the value renders alone inside the box — the
// NEW NAME label already carries the field name. Blink is disabled so a
// captured frame is deterministic. Input semantics are untouched; only Styles
// and Prompt change.
func renameInputView(input textinput.Model, th theme.Theme, colourless bool) string {
	input.Prompt = ""
	styles := input.Styles()
	if colourless {
		styles.Focused.Text = lipgloss.NewStyle()
		styles.Cursor.Color = lipgloss.NoColor{}
		styles.Cursor.Blink = false
		input.SetStyles(styles)
		return input.View()
	}
	styles.Focused.Text = lipgloss.NewStyle().Foreground(th.TextPrimary.Color())
	styles.Cursor.Color = th.AccentAttention.Color()
	styles.Cursor.Blink = false
	input.SetStyles(styles)
	return input.View()
}

// The name truncates within the budget left by the fixed-width prefix so an
// over-long name never overflows the panel.
func renameModalWasRow(oldName string, th theme.Theme, colourless bool) string {
	nameBudget := max(renameInputInnerWidth-lipgloss.Width(renameWasPrefix), 1)
	name := ansi.Truncate(oldName, nameBudget, "…")
	return headerStyle(th.TextMuted, th, colourless).Render(renameWasPrefix + name)
}

func renameModalFooterRow(th theme.Theme, colourless bool) string {
	return renderConfirmCancelFooter(renameKeyConfirm, renameLabelConfirm, renameKeyCancel, renameLabelCancel, th, colourless)
}
