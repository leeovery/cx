package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// destructive_confirm.go owns the SINGLE destructive-confirm panel grammar
// shared by the kill modal and the delete-project modal — two modals that
// render the identical "destructive confirm" element: a state.destructive ▲
// <Title> header, a state.destructive + bold target name row, an optional set
// of extra body rows (the delete modal's project path), ONE canvas-painted
// blank separator, a text.muted consequence word-wrapped at the shared body
// width, and a `y <verb>   esc cancel` footer. Before this file the
// per-compartment render logic was duplicated wholesale across kill_modal.go
// and the near-verbatim delete_modal.go clone; the destructive treatment (▲
// glyph, state.destructive role, text.muted consequence colour, body-width 52,
// footer shape) now lives here in exactly one place, so a future change is a
// single edit and the two modals can never silently diverge.
//
// Destructive emphasis is carried by glyph + colour + bold, never colour
// alone: the ▲ triangle and the title + target name render in state.destructive
// AND bold, so under the NO_COLOR carve-out the ▲ glyph + bold still mark the
// action as destructive on the terminal's native fg.

const (
	// destructiveTitleGlyph is the destructive ▲ triangle that opens the header —
	// state.destructive (red is destructive-only). Glyph + colour + bold. The
	// single definition shared by the kill and delete modals.
	destructiveTitleGlyph = "▲"
	// destructiveBodyWidth is the word-wrap target for the consequence line (in cells)
	// and the panel's minimum content width, so the panel stays a consistent size
	// regardless of target-name/path length and the consequence wraps to the ~two lines
	// the reference frames show. The single definition shared by both modals.
	destructiveBodyWidth = 52
	// destructiveKeyCancel / destructiveLabelCancel are the always-present cancel hint —
	// the dismiss key lives in the footer as `esc cancel` for every destructive
	// confirm.
	destructiveKeyCancel   = "esc"
	destructiveLabelCancel = "cancel"
)

// destructiveConfirmSpec captures the per-modal DATA a destructive-confirm
// panel needs; everything else (the ▲ glyph, the state.destructive/text.muted
// roles, the separator, the body width, the cancel hint) is owned by
// renderDestructiveConfirm. The kill modal supplies a nameTrailer (the `· N
// window(s)` count rides the name row); the delete modal supplies extraBodyRows
// (the project-path row below the name).
type destructiveConfirmSpec struct {
	// title is the header title text (rendered state.destructive + bold beside the ▲).
	title string
	// targetName is the destructive target (session / project name), rendered state.destructive
	// + bold as the body's first row.
	targetName string
	// nameTrailer, when non-empty, is appended to the name row in text.muted after a
	// two-cell canvas gap (the kill modal's `· N window(s)` count) — rendered by
	// destructiveNameRow. Empty for delete.
	nameTrailer string
	// extraBodyRows are already-styled rows inserted below the name row, before the
	// blank separator (the delete modal's project-path row). Empty for kill.
	extraBodyRows []string
	// consequence is the irreversibility / record-only warning, word-wrapped at
	// destructiveBodyWidth and rendered in text.muted.
	consequence string
	// confirmKey / confirmLabel are the footer's confirm hint (e.g. `y` / `kill`); the
	// cancel hint is always `esc cancel`.
	confirmKey   string
	confirmLabel string
}

// renderDestructiveConfirm composes the destructive-confirm panel for the given spec,
// drawing the three compartments through the shared single-tone joined panel:
//
//	header:  ▲ <Title>                 (▲ + title, state.destructive + bold)
//	body:    <name> [· trailer]         (name state.destructive+bold, trailer text.muted)
//	         [extra rows]                (e.g. the delete modal's project path)
//	         <blank>                     (the single "what" → "warning" separator)
//	         <consequence …>             (text.muted, word-wrapped at body width)
//	footer:  <key> <verb>   esc cancel   (glyphs accent.key, labels text.muted)
//
// Vertical spacing is terminal-native FLUSH (the help/kill/delete convention): every
// compartment's content is flush to its dividers; the ONE blank row inside the body is
// the deliberate semantic separator between the target and the warning.
func renderDestructiveConfirm(spec destructiveConfirmSpec, th theme.Theme, colourless bool) string {
	header := []string{destructiveHeaderRow(spec.title, th, colourless)}
	body := destructiveBodyRows(spec, th, colourless)
	footer := []string{destructiveFooterRow(spec.confirmKey, spec.confirmLabel, th, colourless)}
	return renderJoinedPanel([][]string{header, body, footer}, th.Border, th, colourless)
}

// destructiveHeaderRow renders `▲ <Title>` — the ▲ glyph and the title text
// both in state.destructive and bold (glyph + colour + bold). Under NO_COLOR
// the state.destructive hue drops (native fg) but the glyph + bold remain so
// the destructive signal survives.
func destructiveHeaderRow(title string, th theme.Theme, colourless bool) string {
	style := headerStyle(th.StateDestructive, th, colourless).Bold(true)
	glyph := style.Render(destructiveTitleGlyph)
	gap := headerCanvasBg(th, colourless).Render(" ")
	titleSeg := style.Render(title)
	return lipgloss.JoinHorizontal(lipgloss.Top, glyph, gap, titleSeg)
}

// destructiveBodyRows builds the body compartment: the target name row (with an optional
// same-line trailer), any extra body rows, ONE blank separator row, then the
// word-wrapped consequence line(s).
func destructiveBodyRows(spec destructiveConfirmSpec, th theme.Theme, colourless bool) []string {
	rows := []string{destructiveNameRow(spec.targetName, spec.nameTrailer, th, colourless)}
	rows = append(rows, spec.extraBodyRows...)
	// The single blank row separating the "what" (target) from the "warning"
	// (consequence) — the body's only blank, canvas-painted so it carries no
	// terminal-bg island.
	rows = append(rows, headerCanvasBg(th, colourless).Render(""))
	rows = append(rows, destructiveConsequenceRows(spec.consequence, th, colourless)...)
	return rows
}

// destructiveNameRow renders the target name in state.destructive + bold (the
// destructive target emphasis). When trailer is non-empty it appends
// `  <trailer>` in text.muted on the same line (the kill modal's `· N
// window(s)` count); an empty trailer renders the name alone (the delete
// modal's name row).
func destructiveNameRow(name, trailer string, th theme.Theme, colourless bool) string {
	nameSeg := headerStyle(th.StateDestructive, th, colourless).Bold(true).Render(name)
	if trailer == "" {
		return nameSeg
	}
	gap := headerCanvasBg(th, colourless).Render("  ")
	trailerSeg := headerStyle(th.TextMuted, th, colourless).Render(trailer)
	return lipgloss.JoinHorizontal(lipgloss.Top, nameSeg, gap, trailerSeg)
}

// destructiveConsequenceRows word-wraps the consequence sentence to destructiveBodyWidth
// and renders each wrapped line in text.muted — so the panel grows to the body width
// and the consequence reads across the wrapped lines of the reference frames. The
// single word-wrap loop shared by both modals.
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

// destructiveFooterRow renders `<key> <verb>   esc cancel` — the key glyphs in
// accent.key, the labels in text.muted, via the shared renderConfirmCancelFooter so
// the confirm/cancel footer shape lives in one place. The cancel hint is always
// `esc cancel`.
func destructiveFooterRow(confirmKey, confirmLabel string, th theme.Theme, colourless bool) string {
	return renderConfirmCancelFooter(confirmKey, confirmLabel, destructiveKeyCancel, destructiveLabelCancel, th, colourless)
}
