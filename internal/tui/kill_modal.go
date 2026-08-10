package tui

import (
	"fmt"

	"github.com/leeovery/portal/internal/theme"
)

// The kill-confirm modal. A reskin (not a rewrite): the confirm/cancel LOGIC
// is unchanged (handled in updateKillConfirmModal); this file owns only the
// kill modal's DATA. The destructive-confirm panel grammar (the
// state.destructive ▲ <Title> header, the state.destructive+bold target name
// row, the canvas blank separator, the text.muted consequence word-wrapped at
// body-width 52, and the y <verb> · esc cancel footer) lives once in
// destructive_confirm.go; this file supplies only the kill title / consequence
// / window-count / footer verb and calls the shared renderer.

const (
	// killTitle is the header title text (state.destructive): `Kill session?`.
	killTitle = "Kill session?"
	// killConsequence is the consequence line — the irreversibility warning,
	// rendered in text.muted and word-wrapped within the panel body width.
	killConsequence = "Ends the tmux session and all its panes. Can't be undone."

	// Footer confirm copy. The y key glyph renders in accent.key, the kill label in
	// text.muted. The cancel hint (`esc cancel`) is owned by destructive_confirm.go.
	killKeyConfirm   = "y"
	killLabelConfirm = "kill"
)

// renderKillModalContent composes the kill-confirm modal body for the given
// session name + window count by supplying the kill DATA to the shared
// destructive-confirm renderer. The window count rides the name row via nameTrailer
// (`<name>  · N window(s)`); kill has no extra body rows.
//
//	header:  ▲ Kill session?            (▲ + title, state.destructive + bold)
//	body:    <name>  · N window(s)      (name state.destructive+bold, count text.muted)
//	         <blank>                     (the single "what" → "warning" separator)
//	         Ends the tmux session …     (consequence, text.muted, word-wrapped)
//	footer:  y kill   esc cancel         (glyphs accent.key, labels text.muted)
func renderKillModalContent(name string, windows int, th theme.Theme, colourless bool) string {
	spec := destructiveConfirmSpec{
		title:        killTitle,
		targetName:   name,
		nameTrailer:  killWindowCount(windows),
		consequence:  killConsequence,
		confirmKey:   killKeyConfirm,
		confirmLabel: killLabelConfirm,
	}
	return renderDestructiveConfirm(spec, th, colourless)
}

// killWindowCount returns the `· N window(s)` count fragment with correct
// pluralisation (singular only for exactly 1; 0 and N>1 plural).
func killWindowCount(windows int) string {
	unit := "windows"
	if windows == 1 {
		unit = "window"
	}
	return fmt.Sprintf("· %d %s", windows, unit)
}
