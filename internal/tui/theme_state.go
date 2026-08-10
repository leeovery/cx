package tui

import "github.com/leeovery/portal/internal/theme"

// themeState is the model's theme machinery: the loaded setting, the seams the
// panel reaches it through, the light/dark resolution that selects a palette out
// of it, and the palette every renderer paints from. It outlives any one panel
// open, unlike themePanel.
//
// Two of its fields look like drift and are not:
//
//   - canvasMode deliberately diverges from gate.appearance after a mid-session
//     constant → adaptive conversion. Routing the conversion back through
//     syncResolvedMode to "fix the drift" re-captures startupCanvasHex
//     mid-session, which sets a colour the user never chose back into their
//     terminal on exit.
//   - startupCanvasHex deliberately does not move with active. It is frozen at
//     the moment the gate selected a member; a commit or an uncommitted preview
//     moves the active palette and must leave this alone.
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

	// canvasMode is the light/dark answer in force and selects the active member
	// out of an adaptive nomination. Its zero value is the standing no-answer
	// fallback, because that is theme.Member's.
	//
	// It mirrors gate.appearance (synced by Model.syncResolvedMode) until the
	// mid-session constant → adaptive conversion, which writes
	// Model.retainedCanvasAnswer straight into it while the pinned gate keeps its
	// fallback. See the type's first invariant for why that divergence must not be
	// "fixed".
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
	// active at exit (see the type's second invariant).
	//
	// It is taken from active.Canvas.Value, the parsed canonical value, because
	// under an adaptive pair a re-read of the nomination differs until the gate
	// resolves.
	//
	// Empty while the gate is still open: the pre-resolution frame paints no canvas
	// and sets no OSC 11 background, so there is nothing to restore. sameHexColour
	// reports false for an empty value, making the set-back a harmless no-op write.
	startupCanvasHex string

	// bgReplyArrived records that an OSC 11 reply reached Update at all, which is a
	// different fact from Model.originalBg being non-empty: a no-answer-shaped
	// reply (nil Color) leaves the hex empty while still being an answer.
	// bgReplyDark is what that reply said. The pair lets the constant → adaptive
	// conversion, which did not observe the arrival, distinguish "the terminal said
	// light" from "nothing ever came back".
	//
	// The classification comes from tea.BackgroundColorMsg.IsDark rather than being
	// re-derived from the retained hex, so the conversion reaches the same verdict
	// the gate would on the same reply.
	bgReplyArrived bool
	bgReplyDark    bool

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

	// flashOrigin is the precedence tier of the active inline flash: a
	// theme-origin flash claims the notice slot even while the filter line is
	// live, while every other flash keeps today's order. It is reset to
	// flashOriginDefault by setFlash / setSuccessFlash and stamped only by
	// setThemeFlash, so the tier can never be inherited by an unrelated message.
	flashOrigin flashOrigin

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
