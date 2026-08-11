package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

// A different fact from the answer in force: a constant launch retains a reply
// without ever turning it into an answer, which is what lets a mid-session
// conversion classify a terminal this launch asked no question of.
type terminalReply struct {
	// Tracked rather than inferred from a non-empty Model.originalBg: a
	// no-answer-shaped reply leaves the hex empty while still being an answer.
	arrived bool
	member  theme.Member
}

// IsDark is nil-safe, so a no-answer-shaped reply is an arrival classified dark.
func terminalReplyFrom(msg tea.BackgroundColorMsg) terminalReply {
	if msg.IsDark() {
		return terminalReply{arrived: true, member: theme.MemberDark}
	}
	return terminalReply{arrived: true, member: theme.MemberLight}
}

func (r terminalReply) answer() theme.Member {
	if !r.arrived {
		return theme.MemberDark
	}
	return r.member
}

// themeState outlives any one panel open, unlike themePanel.
type themeState struct {
	// Zero value is the "nothing was injected" sentinel that leaves New's
	// dark-built-in seed in place. Describes what is persisted, not what is
	// rendered — the palette in force is active.
	nomination theme.Nomination

	// A construction-time snapshot, never refreshed: re-reading prefs would let
	// another instance's commit change what this panel shows.
	keys theme.RawKeys

	// Nil makes `t` a silent no-op.
	source ThemeSource

	// Nil is the ordinary unwired state, so every call site must nil-guard.
	persister ThemePersister

	gate appearanceGate

	// Zero value is the standing no-answer fallback. Established through
	// adoptGateAnswer / adoptRetainedReply, so which fact is in force is decided
	// there rather than at each reader.
	canvasMode theme.Member

	// New seeds this with the dark built-in before the options apply — an empty
	// Theme renders silently colourless rather than failing.
	active theme.Theme

	// Frozen at the moment the gate resolves and never moved with active: the
	// canvas-echo guard must compare against the canvas in force during the
	// startup window, not whatever theme is active at exit. Empty until then,
	// which sameHexColour reports false for, making the set-back a no-op.
	startupCanvasHex string

	reply terminalReply

	// Runs from a failed write until a later commit succeeds — longer than the
	// message slot's line, which dies on the next keypress. That is what stops
	// the next `Esc` from silently reverting the theme the user chose. It lives
	// here because it must outlive the panel struct the close discards.
	commitFailed bool

	// Capture-only seeds: no-ops at their zero values, never set in production.

	// Placement only — applying the seeded row's palette would make
	// `capturetool --theme <slug|path>` inert on the frames a drop-in author
	// most wants to check.
	initialCursor string

	initialConfirm bool

	// Both halves of the report are seeded because their lifetimes differ.
	initialCommitFailed bool
}

// The gate's resolution and the terminal's reply are not interchangeable: on a
// never-armed gate the resolution is the standing dark fallback, not a fact
// about the terminal.
func (s themeState) inForceMode() theme.Member {
	return s.canvasMode
}

func (s *themeState) adoptGateAnswer() {
	s.canvasMode = s.gate.appearance
}

// For the mid-session constant → adaptive conversion: the answer comes from the
// reply already in hand — no new query, no new race, the gate untouched.
func (s *themeState) adoptRetainedReply() {
	s.canvasMode = s.reply.answer()
}
