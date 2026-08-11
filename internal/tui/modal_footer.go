package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// footerHintGroup is one `<key/glyph> <label>` pair; an empty key renders the
// label alone.
type footerHintGroup struct {
	key   string
	label string
}

// renderKeyHint is the one place the footer key-hint shape is authored; every
// modal/footer hint routes through it (callers default keyTok to accent.key).
// An empty key takes the label-only path — the form the edit footer's
// consequence note collapses onto.
func renderKeyHint(key, label string, keyTok theme.Token, th theme.Theme, colourless bool) string {
	labelSeg := headerStyle(th.TextMuted, th, colourless).Render(label)
	if key == "" {
		return labelSeg
	}
	keySeg := headerStyle(keyTok, th, colourless).Render(key)
	gap := headerCanvasBg(th, colourless).Render(" ")
	return lipgloss.JoinHorizontal(lipgloss.Top, keySeg, gap, labelSeg)
}

func renderBlueKeyHint(key, label string, th theme.Theme, colourless bool) string {
	return renderKeyHint(key, label, th.AccentKey, th, colourless)
}

// keyColumnRow lays a key/label pair out in a fixed-width key column so
// stacked rows share a label left edge. Every structural cell is
// canvas-painted.
//
// A glyph at or past columnWidth takes no pad segment: a canvas style renders
// the empty string as a styled empty run, so padding unconditionally would
// leave a stray escape pair on every full-width row. It takes ready styles
// rather than tokens because a key column can diverge in more than hue.
func keyColumnRow(glyph, label string, keyStyle, labelStyle lipgloss.Style, columnWidth int, gap string, th theme.Theme, colourless bool) string {
	key := keyStyle.Render(glyph)
	keyWidth := lipgloss.Width(key)
	pad := ""
	if keyWidth < columnWidth {
		pad = headerCanvasBg(th, colourless).Render(spaces(columnWidth - keyWidth))
	}
	gapSeg := headerCanvasBg(th, colourless).Render(gap)
	return lipgloss.JoinHorizontal(lipgloss.Top, key, pad, gapSeg, labelStyle.Render(label))
}

func renderConfirmCancelFooter(confirmKey, confirmLabel, cancelKey, cancelLabel string, th theme.Theme, colourless bool) string {
	confirm := renderKeyHint(confirmKey, confirmLabel, th.AccentKey, th, colourless)
	gap := headerCanvasBg(th, colourless).Render(modalFooterGap)
	cancel := renderKeyHint(cancelKey, cancelLabel, th.AccentKey, th, colourless)
	return lipgloss.JoinHorizontal(lipgloss.Top, confirm, gap, cancel)
}

const modalFooterGap = "   "
