package tui

import "github.com/leeovery/portal/internal/theme"

// themeState is the model's theme machinery: the loaded setting, the seams the
// panel reaches it through, the light/dark resolution that selects a palette out
// of it, and the palette every renderer paints from. It sits beside themePanel,
// which holds the slide-over's own per-open state — this is what OUTLIVES a panel.
//
// TWO INVARIANTS BIND THESE FIELDS, AND BOTH LOOK LIKE DRIFT:
//
//   - canvasMode DELIBERATELY DIVERGES FROM gate.appearance after a mid-session
//     constant → adaptive conversion. The gate resolves at most once; the
//     conversion writes the retained terminal answer straight into canvasMode
//     while the pinned gate keeps the fallback it was constructed with. Routing
//     the conversion back through syncResolvedMode to "fix the drift" re-captures
//     startupCanvasHex mid-session, which is how Portal comes to set a colour the
//     user never chose back into their terminal on exit.
//   - startupCanvasHex DELIBERATELY DOES NOT MOVE WITH active. It is the canvas in
//     force during the STARTUP window, frozen at the moment the gate selected a
//     member; a commit or an uncommitted preview moves the active palette and must
//     leave this alone, because it is what the exit-time canvas-echo guard
//     compares against.
type themeState struct {
	// nomination is the LOADED theme setting injected at construction
	// (WithThemeNomination) — one Theme under a constant, both under an adaptive
	// pair. It is the model's whole theme INPUT: it replaces the appearance pref
	// that used to be injected, because a theme IS the mode and there is no mode
	// left to pin.
	//
	// Its zero value is neither state, and that is the honest "nothing was
	// injected" sentinel: a model constructed without Build keeps New's
	// dark-built-in seed rather than selecting a zero Theme out of an empty
	// nomination.
	nomination theme.Nomination

	// keys are prefs.json's three theme keys AS READ — control-stripped
	// and post-translation, the value the panel LISTS a persisted-but-unresolvable
	// slug from and MARKS its `●` by.
	//
	// It is a CONSTRUCTION-TIME SNAPSHOT and is never refreshed. That asymmetry
	// with the fresh per-open directory read is deliberate, because the two files
	// are: the themes directory is what the drop-in loop edits by hand between
	// panel opens, whereas prefs.json is what Portal itself writes — so re-reading
	// it would let another instance's commit silently change what this panel shows
	// and marks, a cross-instance sync Portal deliberately declines. A
	// user who hand-edits prefs mid-session sees it next launch, consistent with
	// every other prefs consumer.
	keys theme.RawKeys

	// enumerator is the panel's theme seam. It is consulted ONLY on the `t`
	// keypress — never at construction, discovery is lazy — and a nil seam makes
	// `t` a silent no-op, following the persister nil-guard precedent.
	enumerator ThemeEnumerator

	// persister is the theme-commit seam, HELD and nothing more: the model
	// neither logs nor retries, and the panel owns the keypresses, the
	// outstanding-failure state and its message. Nil is the ordinary
	// unwired state (a fixture / capturetool model), so every call site must
	// nil-guard exactly as the mode persister's does.
	persister ThemePersister

	// gate is the detect-or-timeout first-paint mechanism and the SINGLE
	// source of truth for whether the real canvas may paint (Model.modeResolved
	// reads it). Under an ADAPTIVE nomination Build opens its detect-or-timeout
	// window via arm() and it resolves on whichever of the OSC 11
	// BackgroundColorMsg or the appearanceTimeoutMsg fires first; a constant
	// nomination, a nomination-less model and the NO_COLOR carve-out are already
	// resolved and unarmable, so detection and the wait are skipped. canvasMode
	// mirrors gate.appearance for the render path (see Model.syncResolvedMode).
	gate appearanceGate

	// canvasMode is the light/dark answer in force. appearanceDarkCanvas is the
	// zero value (the standing no-answer fallback), so an unconfigured model paints
	// the dark canvas. Until a conversion it is the painted mirror of
	// gate.appearance: every gate resolution (OSC 11 reply, timeout) syncs it via
	// Model.syncResolvedMode, and it is what selects the active member out of an
	// ADAPTIVE nomination.
	//
	// Under a constant (or no) nomination it starts as that standing fallback and
	// nothing more — no question was asked, so it must not be read as a fact about
	// the terminal — UNTIL the mid-session constant → adaptive conversion, the
	// THIRD writer. loadNewlyLiveSlot records Model.retainedCanvasAnswer straight
	// into this field: the terminal's own verdict where a reply arrived, the dark
	// no-answer fallback where none did. So from that keypress on it is the
	// answer IN FORCE rather than a value nothing ever asked for, while
	// gate.appearance stays pinned on the constant's fallback and never moves again
	// (a pinned gate's resolve always returns false, so nothing re-syncs). See the
	// type's first invariant for why that divergence must not be "fixed".
	canvasMode canvasAppearance

	// active is the palette EVERY renderer paints from. The model holds it
	// and passes it where a light/dark mode used to be passed, so anything taking
	// the theme as a parameter re-derives per frame — which is what makes a live
	// theme swap complete. There is deliberately NO package-level mutable theme
	// state to replace the retired built-in var: it
	// would put order-dependent state on the render path in a suite that already
	// forbids t.Parallel().
	//
	// New seeds it with the dark built-in before the options apply, so a model
	// constructed without Build is still themed (an empty Theme would resolve
	// through lipgloss.Color("")'s no-colour sentinel: a SILENT colourless render,
	// not a compile error). Applying a nomination overwrites the seed.
	active theme.Theme

	// startupCanvasHex is the canvas hex of the theme the GATE SELECTED, captured
	// at the single moment the gate resolves — which is also the moment the first
	// frame is composed, so it is defined for every frame that exists.
	//
	// It is what RestoreTerminalBackground's canvas-echo guard compares against,
	// and holding it here is the whole point: the comparison must
	// be against the canvas in force during the STARTUP window, never against
	// whatever theme is active at exit (see the type's second invariant).
	//
	// It is taken from active.Canvas.Value — the parsed, canonical value —
	// rather than from a re-read of the nomination, because under an adaptive pair
	// the two differ until the gate resolves.
	//
	// EMPTY while the gate is still open: the pre-resolution frame paints no canvas
	// and sets no OSC 11 background, so a Portal that dies in that window painted
	// nothing and has nothing to restore. sameHexColour reports false for an empty
	// value, so the set-back is emitted to the terminal's own original — a harmless
	// no-op write.
	startupCanvasHex string

	// bgReplyArrived records that an OSC 11 reply reached Update AT ALL, which is
	// a different fact from Model.originalBg being non-empty: a no-answer-shaped
	// reply (nil Color) leaves the hex empty while still being an answer that
	// arrived.
	//
	// bgReplyDark is what that reply SAID, classified by the reply's own IsDark at
	// the moment it landed. The pair is what makes the answer readable later by a
	// consumer that did not observe the arrival — the mid-session constant →
	// adaptive conversion, which must distinguish "the terminal said light" from
	// "nothing ever came back" (Model.retainedCanvasAnswer is its one reader). Both
	// are retained under EVERY setting shape because the query is issued under every
	// shape: a constant asks no light/dark question, so nothing here is ever turned
	// into an answer at construction.
	//
	// The classification is taken from tea.BackgroundColorMsg.IsDark rather than
	// re-derived from the retained hex, so the conversion's answer is the SAME
	// verdict the gate would have reached on the same reply (resolveFromDark reads
	// the identical call). Re-deriving it would be a second luminance rule in a
	// second package, free to drift from the one the gate uses — and it would have
	// to special-case the empty hex a nil-colour reply leaves behind, which IsDark
	// already answers (nil is dark).
	bgReplyArrived bool
	bgReplyDark    bool

	// commitFailed is the OUTSTANDING-FAILURE STATE: a theme write failed
	// and the user has not been told about it on a surface they are left looking at.
	//
	// IT IS A STATE, NOT A MESSAGE, and the two have different lifetimes on purpose.
	// The panel's message slot reports the failure until the NEXT KEYPRESS; this runs
	// from the failed write until a SUBSEQUENT COMMIT SUCCEEDS, and nothing else
	// clears it. Arrowing away therefore dismisses the message while leaving this
	// set, which is what stops the very next `Esc` — a close re-resolves PERSISTED
	// state, silently dropping the theme the user chose — from reinstating the
	// silent revert this state exists to close.
	//
	// IT LIVES HERE RATHER THAN ON themePanel because it must OUTLIVE the
	// panel: closing discards that struct whole, and the close is exactly when the
	// report is due.
	//
	// A NIL PERSISTER NEVER SETS IT. That is the absence of a writer rather than a
	// failed write (see commitConstant), so no model can enter the reported-failure
	// state by COMMITTING without one — a capture included. A capture fixture
	// declares it directly instead, because the report is a designed surface and a
	// frame of it has to be capturable.
	commitFailed bool

	// flashOrigin is the precedence tier of the active inline flash: a
	// theme-origin flash claims the notice slot even while the filter line is
	// live, while every other flash keeps today's order. It is reset to
	// flashOriginDefault by setFlash / setSuccessFlash and stamped only by
	// setThemeFlash, so the tier can never be inherited by an unrelated message.
	flashOrigin flashOrigin

	// initialCursor is the capture-only PANEL cursor anchor: the row IDENTITY
	// the slide-over's cursor lands on once the panel has opened.
	//
	// It exists because a fixture is a ONE-SHOT RENDER and the open puts the cursor
	// on the theme actually rendering. The constant-while-previewing frame needs the
	// cursor on a row OTHER than the marked one, which is otherwise reachable only
	// by arrowing — so without this seed that frame cannot be captured at all.
	//
	// IT IS PLACEMENT ONLY AND APPLIES NO THEME (armThemePanel anchors with it and
	// nothing else). Applying the seeded row's palette would make
	// `capturetool --theme <slug|path>` inert on precisely the frames a drop-in
	// author most wants to check.
	//
	// Empty is a no-op — production never sets it (WithInitialThemeCursor is wired
	// only by the offline capture harness).
	initialCursor string

	// initialConfirm is the capture-only seed for the slot-from-constant
	// confirm: it raises the question against the persisted constant once the panel
	// has opened, exactly as an `l` over that setting would.
	//
	// It exists for the same reason the cursor anchor does — a fixture is a ONE-SHOT
	// RENDER — and it declares STATE rather than text: the copy is composed by the
	// message slot from its own pinned constants, so a fixture can never ship a
	// paraphrase of it.
	//
	// False is a no-op; production never sets it.
	initialConfirm bool

	// initialCommitFailed is the capture-only seed for the failed-commit
	// report: it raises the message slot's line AND sets the outstanding-failure
	// state once the panel has opened, exactly as a write that did not land does.
	//
	// It declares STATE rather than text for the same reason its sibling does, and
	// both halves are seeded because the line and the state have different
	// lifetimes — a frame
	// showing the line while the state was unset would be a shape production cannot
	// reach.
	//
	// False is a no-op; production never sets it.
	initialCommitFailed bool
}
