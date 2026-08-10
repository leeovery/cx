package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// modal_footer.go owns the SINGLE canonical implementation of the footer key-hint
// shape and the confirm/cancel footer row — the shared footer contract. Before
// this file the `<key/glyph> <label>` primitive (key glyph in accent.key, a one-cell
// canvas-painted gap, label in text.muted, joined horizontally) was independently
// re-authored across the kill/delete/rename modals, the Preview nav footer and the
// contextual edit footer; the three modal footer rows each hand-assembled confirm-hint +
// fixed gap + cancel-hint. They now all route through renderKeyHint /
// renderConfirmCancelFooter so the convention (key-glyph colour role, gap width) lives in
// exactly one place and the modals can never silently drift from the footer.
//
// renderBlueKeyHint is the named accent.key pin every contextual footer hint (the edit
// footer + the Preview nav footer) shares — the ONE canonical blue-key-hint path, after
// the five superseded per-modal wrappers were removed.
//
// keyColumnRow is the file's second shape: the same key-then-label pairing laid out in a
// FIXED-WIDTH key COLUMN, for the vertical surfaces that stack such pairs and need their
// labels on a common left edge. It sits here rather than inside one of those surfaces so
// that none of them owns another's geometry.

// footerHintGroup is one `<key/glyph> <label>` footer-hint pair — the single shape
// modelling the {Key/Glyph, Label} concept across the contextual edit footer and the
// Preview nav footer. (It replaces the former parallel footerGroup / previewFooterGroup
// value types.) An empty key renders the label alone via renderKeyHint.
type footerHintGroup struct {
	key   string
	label string
}

// renderKeyHint renders one `<key> <label>` footer hint: the key glyph in keyTok over
// the owned canvas, a single canvas-painted gap, then the label in text.muted — joined
// horizontally. It is the ONE place the footer key-hint shape is authored;
// every modal/footer hint routes through it (callers default keyTok to accent.key).
//
// An empty key takes the label-only fast path (no glyph, no gap) — the form the edit
// footer's `empty on save = delete` consequence note collapses onto.
func renderKeyHint(key, label string, keyTok theme.Token, th theme.Theme, colourless bool) string {
	labelSeg := headerStyle(th.TextMuted, th, colourless).Render(label)
	if key == "" {
		return labelSeg
	}
	keySeg := headerStyle(keyTok, th, colourless).Render(key)
	gap := headerCanvasBg(th, colourless).Render(" ")
	return lipgloss.JoinHorizontal(lipgloss.Top, keySeg, gap, labelSeg)
}

// renderBlueKeyHint renders one `<key> <label>` footer hint with the key glyph pinned
// to accent.key — the SINGLE canonical accent.key key-hint path. It is a thin pin over
// renderKeyHint that fixes keyTok to the theme's accent.key token so the contextual edit footer
// and the Preview nav footer (the two live accent.key call sites) share one seam rather
// than each re-authoring the token. An empty key takes renderKeyHint's label-only path.
func renderBlueKeyHint(key, label string, th theme.Theme, colourless bool) string {
	return renderKeyHint(key, label, th.AccentKey, th, colourless)
}

// keyColumnRow renders one `<glyph> <label>` row in the fixed-width key-column layout:
// the glyph in keyStyle, padded out to columnWidth with canvas-painted spaces so the
// labels of stacked rows share a left edge, then a canvas-painted gap, then the label in
// labelStyle. Every structural cell is canvas-painted, so the column opens no
// terminal-bg island inside the surface it sits in.
//
// A glyph already at or past columnWidth takes NO pad segment: a canvas style renders
// the empty string as a styled empty run rather than as nothing, so padding
// unconditionally would leave a stray escape pair on every full-width row.
//
// It takes ready STYLES rather than role tokens because a key column can diverge in more
// than hue — a bolded glyph, a token chosen per entry — and resolving that at the call
// site is what keeps this builder free of a branch per surface.
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

// renderConfirmCancelFooter renders the two-hint modal footer row: the confirm hint,
// the fixed canvas-painted gap (modalFooterGap, "   "), then the cancel hint — both
// hints via renderKeyHint with accent.key key glyphs. The three modal footer rows
// (kill y/kill·esc/cancel, delete y/delete·esc/cancel, rename ⏎/rename·esc/cancel)
// route through here, passing their per-modal key/label constants as arguments.
func renderConfirmCancelFooter(confirmKey, confirmLabel, cancelKey, cancelLabel string, th theme.Theme, colourless bool) string {
	confirm := renderKeyHint(confirmKey, confirmLabel, th.AccentKey, th, colourless)
	gap := headerCanvasBg(th, colourless).Render(modalFooterGap)
	cancel := renderKeyHint(cancelKey, cancelLabel, th.AccentKey, th, colourless)
	return lipgloss.JoinHorizontal(lipgloss.Top, confirm, gap, cancel)
}

// modalFooterGap is the fixed gap between the two key/label groups in a confirm/cancel
// modal footer row (the reference's `y kill   esc cancel` spacing). It replaces the
// per-modal killFooterGap / deleteFooterGap / renameFooterGap constants, which were all
// "   " (three canvas spaces).
const modalFooterGap = "   "
