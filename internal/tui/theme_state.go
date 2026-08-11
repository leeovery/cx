package tui

import (
	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

// terminalReply is what the terminal SAID about its background — an optional
// value, empty until an OSC 11 reply reaches Update.
//
// It is a different fact from the answer in force (themeState.inForceMode): a
// constant launch retains a reply while never turning it into an answer, which is
// what lets the mid-session constant → adaptive conversion classify a terminal
// this launch asked no question of.
type terminalReply struct {
	// arrived records that a reply reached Update at all. It is tracked rather
	// than inferred from a non-empty Model.originalBg because a no-answer-shaped
	// reply (nil colour) leaves the hex empty while still being an answer.
	arrived bool
	// member is what that reply said, classified off the message rather than
	// re-derived from the retained hex, so every reader reaches the verdict the
	// gate reached on the same reply.
	member theme.Member
}

// terminalReplyFrom classifies an OSC 11 reply. IsDark is nil-safe, so a
// no-answer-shaped reply is an arrival classified dark.
func terminalReplyFrom(msg tea.BackgroundColorMsg) terminalReply {
	if msg.IsDark() {
		return terminalReply{arrived: true, member: theme.MemberDark}
	}
	return terminalReply{arrived: true, member: theme.MemberLight}
}

// answer is the reply as a light/dark verdict, with no reply at all falling to
// dark — the standing no-answer fallback everywhere else in the theme machinery.
func (r terminalReply) answer() theme.Member {
	if !r.arrived {
		return theme.MemberDark
	}
	return r.member
}

// themeState is the model's theme machinery: the loaded setting, the seams the
// panel reaches it through, the light/dark resolution that selects a palette out
// of it, and the palette every renderer paints from. It outlives any one panel
// open, unlike themePanel.
//
// startupCanvasHex deliberately does not move with active. It is frozen at the
// moment the gate selected a member; a commit or an uncommitted preview moves the
// active palette and must leave this alone.
type themeState struct {
	// nomination is the loaded theme setting — one Theme under a constant, both
	// under an adaptive pair. Its zero value is neither state, the "nothing was
	// injected" sentinel that leaves New's dark-built-in seed in place.
	//
	// It is kept consistent with the PERSISTED setting at one site: every commit
	// that lands re-resolves it (applyCommittedSetting), so it describes what is
	// persisted rather than only what was injected at construction.
	//
	// It is NOT a live input to rendering. The palette in force is
	// themeState.active, which a preview moves and a commit deliberately does not.
	// The nomination's readers are the appearance gate and syncResolvedMode, both
	// of which run before or at the gate's single resolution.
	nomination theme.Nomination

	// keys are prefs.json's three theme keys as read (control-stripped,
	// post-translation): what the panel lists a persisted-but-unresolvable slug
	// from and marks its `●` by.
	//
	// It is a construction-time snapshot and is never refreshed, unlike the fresh
	// per-open directory read: prefs.json is what Portal itself writes, so
	// re-reading it would let another instance's commit change what this panel
	// shows.
	keys theme.RawKeys

	// source is the panel's theme seam, consulted only on the `t` keypress
	// (discovery is lazy). Nil makes `t` a silent no-op.
	source ThemeSource

	// persister is the theme-commit seam, held and nothing more: the panel owns
	// the keypresses, the outstanding-failure state and its message. Nil is the
	// ordinary unwired state, so every call site must nil-guard.
	persister ThemePersister

	// gate is the detect-or-timeout first-paint mechanism and the source of truth
	// for whether the real canvas may paint (Model.modeResolved reads it). An
	// adaptive nomination arms it; a constant nomination, a nomination-less model
	// and the NO_COLOR carve-out are already resolved and unarmable.
	gate appearanceGate

	// canvasMode STORES the light/dark answer in force. Its zero value is the
	// standing no-answer fallback, because that is theme.Member's.
	//
	// It is established through adoptGateAnswer / adoptRetainedReply and read
	// through inForceMode, so which fact is in force is decided at those sites
	// rather than at each reader.
	canvasMode theme.Member

	// active is the palette every renderer paints from, passed where a light/dark
	// mode used to be so everything re-derives per frame.
	//
	// New seeds it with the dark built-in before the options apply, so a model
	// constructed without Build is still themed: an empty Theme resolves through
	// lipgloss.Color("")'s no-colour sentinel — a silent colourless render rather
	// than a compile error.
	active theme.Theme

	// startupCanvasHex is the canvas hex of the theme the gate selected, captured
	// at the single moment the gate resolves — also the moment the first frame is
	// composed, so it is defined for every frame that exists. It is what
	// RestoreTerminalBackground's canvas-echo guard compares against, and it must
	// be the canvas in force during the startup window, never whatever theme is
	// active at exit (see the type's invariant).
	//
	// It is taken from active.Canvas.Value, the parsed canonical value, because
	// under an adaptive pair a re-read of the nomination differs until the gate
	// resolves.
	//
	// Empty while the gate is still open: the pre-resolution frame paints no canvas
	// and sets no OSC 11 background, so there is nothing to restore. sameHexColour
	// reports false for an empty value, making the set-back a harmless no-op write.
	startupCanvasHex string

	// reply is what the terminal said about its background, filled by the OSC 11
	// arm.
	reply terminalReply

	// commitFailed is the outstanding-failure state: a theme write failed and the
	// user has not been told about it on a surface they are left looking at.
	//
	// It is a state, not a message, and the two have different lifetimes. The
	// panel's message slot reports the failure until the next keypress; this runs
	// from the failed write until a subsequent commit succeeds. Arrowing away
	// therefore dismisses the message while leaving this set, which is what stops
	// the next `Esc` — a close re-resolves persisted state — from silently
	// reverting the theme the user chose.
	//
	// It lives here rather than on themePanel because it must outlive the panel:
	// closing discards that struct whole, and the close is when the report is due.
	//
	// A nil persister never sets it: that is the absence of a writer rather than a
	// failed write (see commitConstant). Capture fixtures declare it directly.
	commitFailed bool

	// The three capture-only seeds below let the offline harness render frames a
	// one-shot render cannot otherwise reach. Each is a no-op at its zero value and
	// production never sets any of them.

	// initialCursor is the row identity the panel's cursor lands on once it has
	// opened. It is placement only and applies no theme; applying the seeded row's
	// palette would make `capturetool --theme <slug|path>` inert on precisely the
	// frames a drop-in author most wants to check.
	initialCursor string

	// initialConfirm raises the slot-from-constant confirm against the persisted
	// constant, exactly as an `l` over that setting would. It declares state rather
	// than text, so the copy stays composed by the message slot.
	initialConfirm bool

	// initialCommitFailed raises the failed-commit line and sets the
	// outstanding-failure state, exactly as a write that did not land does. Both
	// halves are seeded because their lifetimes differ, and a frame showing the
	// line with the state unset is unreachable in production.
	initialCommitFailed bool
}

// inForceMode is THE light/dark answer in force: the value that selects the
// active member out of an adaptive nomination and names the slot painting the
// screen. Every site deciding light-or-dark reads it, so none of them has to pick
// between the gate's resolution and the terminal's reply.
//
// The two are not interchangeable. The gate's resolution is the answer for a gate
// that was ARMED; on a never-armed gate it is the standing dark fallback and not a
// fact about the terminal, while the reply is a fact about the terminal that a
// constant launch never turned into an answer. Which one is in force is decided
// once, by whichever adopt call establishes it.
func (s themeState) inForceMode() theme.Member {
	return s.canvasMode
}

// adoptGateAnswer establishes the gate's resolution as the answer in force. It
// runs after every gate transition, so the answer tracks the single resolution the
// gate owns.
func (s *themeState) adoptGateAnswer() {
	s.canvasMode = s.gate.appearance
}

// adoptRetainedReply establishes the terminal's retained reply as the answer in
// force, for the mid-session constant → adaptive conversion: light/dark starts
// mattering on a launch that asked no question, so the answer comes from the reply
// already in hand — no new query, no new race, and the gate untouched.
func (s *themeState) adoptRetainedReply() {
	s.canvasMode = s.reply.answer()
}
