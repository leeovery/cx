package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// §9.2's SLOT-FROM-CONSTANT CONFIRM: the panel's one gate, and the only place a
// keypress the user was told is inert can silently cost them a setting they chose.
//
// `d`/`l` are described as inert — they assign a slot, and §9.2 states plainly that
// committing to a non-active slot changes nothing on screen. Over a CONSTANT that
// description is false in a way the user cannot see coming: on `"theme": "nord"`,
// pressing `l` clears the constant, the untouched dark slot falls back to the
// shipped default, and `Esc` in a dark terminal lands on `tokyo-night` rather than
// `nord`. The confirm exists for exactly that case, and for no other: `Enter` over
// a pair raises none, because it visibly does what it says.
//
// THREE PROPERTIES MAKE IT MORE THAN A PROMPT, and each is load-bearing:
//
//   - IT IS KEY-EXCLUSIVE WITHIN THE PANEL. Its arm sits at the TOP of
//     updateThemePanel, ahead of the arrows, the commit keys and the `Esc` close, so
//     while it is live it owns the input. An arrow that moved the cursor mid-question
//     would re-theme the screen while the user is answering about a row that has just
//     stopped being the previewed one.
//   - IT RESOLVES ON EXACTLY THREE INPUTS — `y`/`Y`, `n`/`N`/`Esc`, and `Ctrl-C`,
//     which quits rather than cancels (§9.7 keeps it live everywhere; swallowing it
//     would take away the exit key inside a settings surface). Everything else is
//     SWALLOWED, and the question persists: nothing has been written, so there is no
//     partial state to leave behind.
//   - `Esc` CANCELS RATHER THAN CLOSES, because the innermost thing resolves first —
//     the same nesting rule the panel already applies over multi-select (§9.7).
//
// NOTHING IS WRITTEN UNTIL `y`. The raise records what it WOULD write and nothing
// more, so a cancel is inert by construction rather than by an undo.

// themeSlotConfirm is the assignment a live confirm will apply when the user
// answers `y`: the slug under the cursor at the moment the question was asked, and
// the typed slot the keypress named.
//
// THE SLUG IS CAPTURED AT THE RAISE, not re-read at the answer. The two cannot
// differ today — the confirm swallows every key that could move the cursor — and
// capturing it is what keeps that true independently of the routing: the question
// on screen names one row, and the write is that row.
type themeSlotConfirm struct {
	// slug is the row the cursor was on when the confirm was raised.
	slug string
	// slot is the half of the adaptive pair `d`/`l` named. It is the TYPED prefs
	// value threaded from the keypress, so no path here can mint a third slot.
	slot prefs.ThemeSlot
}

// confirming reports whether §9.2's confirm is live — the ONE liveness predicate,
// read off the message slot rather than off a second flag beside it.
//
// The slot is the confirm's whole visible existence (the question, and the footer
// scope themePanelFooterScope substitutes from it), so deriving liveness from
// anything else would admit a state where the panel is answering a question it is
// not asking.
func (p themePanel) confirming() bool {
	return p.message.Kind == themeMessageConfirm
}

// raiseSlotConfirm asks §9.2's question: it records the pending assignment and
// raises the confirm naming the constant that will be cleared.
//
// NOTHING IS WRITTEN HERE. That is the whole point of the gate — the write happens
// on `y` and nowhere else — so this leaves the raw keys, the badges, the cursor and
// the previewed theme exactly as it found them.
//
// The message's own datum is the PERSISTED constant, read by the writer from
// m.themeKeys (see Model.raiseThemePanelConfirm for why the raw key rather than the
// resolution): the confirm names what is being CLEARED, while the pending value
// below names what is being SET. Under §8.5's fallback those two rows are not even
// the same theme.
func (m *Model) raiseSlotConfirm(slug string, slot prefs.ThemeSlot) {
	m.themePanel.pending = themeSlotConfirm{slug: slug, slot: slot}
	m.raiseThemePanelConfirm()
}

// resolveSlotConfirm takes the question down — the message slot emptied and the
// footer restored with it — and hands the caller what was pending.
//
// IT IS THE ONE RESOLUTION SITE, reached by all three of §9.2's answers, so a
// confirm cannot be cleared in one direction and left standing in the other. The
// pending value is RETURNED rather than read afterwards, which is what lets the
// clear happen first: `y`'s commit runs against a panel that has already stopped
// asking, so no failure path can leave a resolved question on screen.
func (m *Model) resolveSlotConfirm() themeSlotConfirm {
	pending := m.themePanel.pending
	m.themePanel.pending = themeSlotConfirm{}
	m.clearThemePanelMessage()
	return pending
}

// updateSlotConfirm is §9.2's key-exclusive arm: the three inputs that resolve the
// confirm, and the swallow that everything else meets.
//
// `Ctrl-C` IS DELIBERATELY ABSENT FROM THIS SWITCH. It is answered one level up, at
// updateThemePanel's own first arm, because it is not a confirm answer — it is the
// global quit §9.7 keeps live EVERYWHERE, and restating it here would be a second
// place for it to be conditionally lost.
//
// THE DEFAULT ARM IS THE FEATURE. Arrows, paging, `Enter`, the other slot key, a
// second `d`/`l`, and every page key the panel already swallows all land here and
// change nothing at all — the question stands until it is answered.
func (m Model) updateSlotConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case themeConfirmAnswer(msg, themeConfirmYesKey):
		(&m).confirmSlotAssignment()
		return m, nil
	case themeConfirmAnswer(msg, themeConfirmNoKey), keyIsCode(msg, tea.KeyEscape):
		// The pending assignment is DISCARDED: a cancel writes nothing, so the value
		// resolveSlotConfirm hands back has no reader on this path.
		(&m).resolveSlotConfirm()
		return m, nil
	default:
		return m, nil
	}
}

// confirmSlotAssignment is §9.2's `y`: the constant cleared and the slot written in
// ONE atomic prefs write (task 6-2's prefs.SaveThemeSlot), mirrored in memory, and
// the panel recomputed against it.
//
// THE CONFIRM COMES DOWN FIRST AND UNCONDITIONALLY. The question has been answered
// whichever way the write goes, so the footer is restored on both paths and §9.13's
// failed-commit line — task 9-7's — is raised into a slot the confirm has already
// vacated rather than one it is competing for. §9.1's single-slot arbiter has
// exactly these two contenders and they can never be live at once precisely because
// this ordering holds.
//
// THE WRITE IS commitSlot's, NOT A SECOND COPY OF IT. Every rule that path carries
// applies here unchanged — the nil persister is INERT rather than failed, the mirror
// is applied to the construction-time snapshot rather than to the merged bytes the
// read-modify-write just held, on error NOTHING moves, and the recompute is its last
// step and is reached only past both early returns.
//
// ON FAILURE THE ERROR GOES TO TASK 9-7'S SEAM and the constant is NOT cleared in
// memory, so the badges still show it — §9.13's "a failed commit does not move the
// `●`" falls out of commitSlot leaving the keys untouched rather than out of a
// second rule stated here.
//
// A NIL PERSISTER IS INERT, NOT COMMITTED, and that is why the persister is read
// AGAIN below rather than inferred from the nil error. commitSlot returns nil for
// TWO outcomes — a write that landed, and the absence of a writer (a fixture or
// `capturetool` model, task 6-7) — and the second returns before the mirror and the
// recompute. loadNewlyLiveSlot runs after a write that LANDED, on mirrored keys;
// handing it the un-mirrored ones would have it resolve off keys that still hold the
// constant — whose slot slugs are both empty — and report a commit-time
// `theme: loaded` for a write that never happened. The distinction is re-derived at
// this ONE call site deliberately: teaching commitSlot to report whether it wrote
// would widen the shared path `Enter` and the pair-`d`/`l` route also take, which
// nothing needs yet.
func (m *Model) confirmSlotAssignment() {
	pending := m.resolveSlotConfirm()
	if err := m.commitSlot(pending.slug, pending.slot); err != nil {
		m.reportThemeCommitFailure(err)
		return
	}
	if m.themePersister == nil {
		return
	}
	m.loadNewlyLiveSlot(pending.slot)
}

// loadNewlyLiveSlot is §9.3's transition, both halves: the OPPOSITE slot's palette
// (the file) and the in-force light/dark classification (the answer).
//
// Assigning a slot over a constant makes the slot the user did NOT assign live in
// the same keypress, and §8.4 never loaded it: construction loads every NOMINATED
// theme, and under a constant the slots are not read at all (§8.2). This is
// therefore THE ONE THEME LOAD OUTSIDE CONSTRUCTION, which is why §12.3's cadence
// column gives `theme: loaded` a commit-time entry at all.
//
// IT HANGS OFF THE CONFIRM, NOT OFF commitSlot, because the confirm is the ONLY
// route that creates the state: a `d`/`l` over an adaptive pair changes which slug a
// live slot names, and nothing becomes newly live. Putting the seam on the shared
// path would give it a caller with nothing to do and a cleared constant it could no
// longer see. That route is ALSO where the converting condition comes from — §8.4's
// "pre-commit keys carried a non-empty `theme`" — since handleSlotCommitKey raises
// the confirm on a CONSTANT setting and on nothing else, so reaching here is the
// condition rather than something to re-test against keys the mirror has since
// cleared.
//
// IT RUNS AFTER A WRITE THAT LANDED — past commitSlot's own mirror and recompute
// rather than between them, and never on the nil-persister path, which returns
// before both (see confirmSlotAssignment). That ordering is deliberate: the load
// needs the MIRRORED keys to know which slug the opposite slot now names, while the
// recompute needs nothing the load produces — it derives rows and badges from the
// raw keys and the retained enumeration (§9.2), which already resolve both slots.
// Nothing renders between the two: both land inside one keypress.
//
// THE ASSIGNED SLOT IS DELIBERATELY NOT RE-READ, and this is the half a reader is
// most likely to add "for symmetry". Its parse is already in hand — §5.8's retained
// enumeration is what makes arrowing an O(1) restyle, and the previewed palette IS
// that parse for the row the confirm captured — so resolving it again would re-parse
// a file the panel already holds and mint a second answer for one slug.
//
// THE OPPOSITE SLUG COMES OFF THE MIRRORED KEYS, THROUGH themeSetting. The mirror
// carries the other slot's raw value across untouched (mirrorThemeSlot), so that is
// exactly §8.4's "raw persisted value from the pre-commit keys" — empty for an
// untouched slot, the stale hand-edited slug otherwise — and routing it through the
// setting is what substitutes §8.3's shipped default for the empty case at the ONE
// site that owns that rule. An untouched slot therefore resolves the shipped default
// from the embedded set with FellBack false, rather than arriving empty and being
// reported as a fallback of a slug nobody set.
//
// ON THE §7.6 FATAL IT DEGRADES AND NOTHING MOVES, the policy applyInForceTheme
// states for every panel call site of the seam: a settings surface must not become
// the route by which a broken binary quits Portal mid-session. Returning BEFORE the
// two assignments is what makes that a genuine no-op rather than a half-converted
// model holding a light/dark answer with no pair to select from.
//
// IT NEVER CALLS ApplyTheme. A commit is a WRITE, NOT A NAVIGATION (§9.2), so the
// pair is completed while the screen keeps previewing whatever the cursor is on. The
// join is model consistency rather than a dependency of the close: task 8-10's `Esc`
// re-resolves from persisted state regardless.
func (m *Model) loadNewlyLiveSlot(assigned prefs.ThemeSlot) {
	newlyLive := oppositeThemeSlot(assigned)
	resolved, err := m.themeEnumerator.ResolveSlot(m.themePanel.enumeration, newlyLive, m.persistedSlotSlug(newlyLive))
	if err != nil {
		return
	}

	m.canvasMode = m.retainedCanvasAnswer()
	m.nomination = joinNomination(newlyLive, resolved.Theme, m.activeTheme)
}

// oppositeThemeSlot is the half of the adaptive pair the keypress did NOT assign —
// the one §8.4 loads.
//
// It maps the prefs slot the keypress threaded onto the resolution's own, which are
// separate types because they name different things: one is what prefs.json writes,
// the other is what a §8.5 fallback is matched against. There is no third value on
// either side to fall through to — prefs.SaveThemeSlot rejects an out-of-range slot
// and commitSlot has already returned on that error — so the light case and its
// complement are the whole of it.
func oppositeThemeSlot(assigned prefs.ThemeSlot) theme.Slot {
	if assigned == prefs.SlotLight {
		return theme.SlotDark
	}
	return theme.SlotLight
}

// persistedSlotSlug is the slug one slot of the mirrored keys nominates, with
// §8.3's shipped default already substituted for an unset one.
//
// It reads through themeSetting rather than off RawKeys so the unset-slot rule has
// the single site it has everywhere else in the panel — what the panel LISTS, MARKS
// and RESOLVES all collapse §8.2's keys through that one function, and a second
// substitution here is how an untouched slot would come to resolve one default in
// the badges and another in the load.
func (m Model) persistedSlotSlug(slot theme.Slot) string {
	setting := m.themeSetting()
	if slot == theme.SlotLight {
		return setting.Light
	}
	return setting.Dark
}

// joinNomination completes §8.2's pair: the newly-loaded palette in the slot that
// just became live, and the one already in hand in the slot the user assigned.
//
// THE ASSIGNED MEMBER IS THE PREVIEWED PALETTE, and that is an identity rather than
// an approximation: §9.2's arrow-preview applies the cursor row's own palette, the
// confirm captures the slug the cursor was on and swallows every key that could move
// it, and a commit takes the CURSOR's slug — so the palette on screen is the parse
// of the slug just written. Taking it from here is what keeps the assigned slot
// unread.
func joinNomination(newlyLive theme.Slot, loaded, assigned theme.Theme) theme.Nomination {
	if newlyLive == theme.SlotLight {
		return theme.AdaptivePair(loaded, assigned)
	}
	return theme.AdaptivePair(assigned, loaded)
}

// retainedCanvasAnswer is §9.3's ANSWER HALF: the light/dark classification of the
// OSC 11 reply this launch already received, which a constant-theme launch retained
// without ever turning into an answer.
//
// §9.3 dissolves the transition precisely because the answer already arrived —
// Model.Init issues the OSC 11 query REGARDLESS of the setting's shape (§8.8), since
// restore-on-exit needs the original background independently of detection. So a
// conversion starts USING an answer already in hand: no new query, no new race, no
// new gate, and §8.8's resolve-once rule untouched — a reply landing after this point
// still never re-themes.
//
// IT IS NOT READ OFF THE GATE. A constant's gate is pinned and its appearance is the
// STANDING DARK FALLBACK rather than a classification of the terminal (see
// appearanceGate.appearance), so reading it would answer "dark" for every light
// terminal in the product. The retained reply is the only value that is a fact about
// the terminal.
//
// NO REPLY AT ALL FALLS TO DARK — §8.8's no-answer fallback, the same rule as
// everywhere else. Two states reach it: a terminal that never answers OSC 11, where
// nothing arrives however late the panel is opened, and §9.3's own case of a panel
// opened within milliseconds of launch, before the answer landed.
//
// A NO-ANSWER-SHAPED REPLY IS NEITHER OF THEM. A nil colour still ARRIVES, leaving an
// empty hex behind and no colour to classify — which is why the read is keyed on the
// arrival rather than on a non-empty originalBg, and why dark is the answer either
// way: IsDark reports a nil colour as dark.
func (m Model) retainedCanvasAnswer() canvasAppearance {
	if m.bgReplyArrived && !m.bgReplyDark {
		return appearanceLightCanvas
	}
	return appearanceDarkCanvas
}

// reportThemeCommitFailure is §9.13's FAILED-COMMIT REPORT, and it is a NO-OP until
// task 9-7 fills it in.
//
// The report is a state rather than a message — outstanding from the moment a write
// fails until a subsequent commit succeeds, surviving the panel's close as a
// main-screen flash — and none of that can be half-built here. What is settled is
// that a failed write is HANDED the error rather than discarding it, from the one
// arm that can produce one.
//
// Until it lands the failure is silent except for cmd's own `theme: commit failed`
// record, and it leaves nothing behind: commitSlot returns before mutating anything,
// so the `●` cannot move.
func (m *Model) reportThemeCommitFailure(err error) {}

// The two answer letters, as the dispatch matches them. They are the terse forms
// §9.2 pins and the ones themePanelConfirmKeymap advertises in the substituted
// footer; the descriptor carries them for DISPLAY and this pair for DISPATCH, which
// is the parity keymap_dispatch_guard_test's contract exists to hold.
const (
	themeConfirmYesKey = "y"
	themeConfirmNoKey  = "n"
)

// themeConfirmAnswer reports whether msg answers the confirm with the given letter,
// in EITHER case (§9.2: "`y` or `Y` confirms", "`n`, `N` or `Esc` cancels").
//
// IT IS NOT isRuneKey, AND THE DIFFERENCE IS THE UPPERCASE HALF. AN UPPERCASE ANSWER
// NEVER REACHES PORTAL WITH A CLEAR MODIFIER FIELD — the two encodings CONVERGE on
// the base rune `y` carrying ModShift with the shifted text "Y". A legacy terminal
// sends the byte and parseUtf8 sets Text to it, then lowercases the ASCII rune into
// Code, moves the original into ShiftedCode and ORs ModShift (decoder.go:1097-1102);
// under the Kitty keyboard protocol the codepoint parameter carries that same base
// rune into Code (decoder.go:1404-1412) with shift riding the modifier parameter
// (1442-1444), and Text — empty out of the code parse — is filled in as the shifted
// "Y" (decoder.go:1504-1517). isRuneKey requires `Mod == 0`, so pinning
// the dispatch to it would leave `Y` dead on EVERY terminal rather than on some
// richer-protocol subset — a key §9.2 names as an answer, silently swallowed
// everywhere. (Its `Text == ch` comparison fails the case anyway; the point is that
// case-folding ALONE would not save it, and that ModShift is not a Kitty-only
// concern the mask could narrow away.)
//
// BOTH LOCK KEYS ARE FORGIVEN ALONGSIDE SHIFT, because the Kitty decoder leaves either
// one riding on an event whose text is a perfectly ordinary answer letter. Caps lock is
// the obvious half: a caps-lock user's `y` arrives as the base rune carrying
// ModCapsLock with the SHIFTED text, since ultraviolet clears Text for any modifier
// past ModShift (decoder.go:1445-1449) and then repopulates it for `keyMod <= ModShift`,
// `ModCapsLock` and the two combined (decoder.go:1480, 1504-1518).
//
// NUM LOCK IS THE SAME TRAP, ONE STEP LESS OBVIOUS — and the reason it reaches Mod at
// all is that the strip is a LOCAL COPY. The decoder does `keyMod := key.Mod; keyMod
// &^= ModNumLock` ("Remove these lock modifiers from now on since they don't affect the
// text", decoder.go:1474-1478) and computes the repopulation gate from that copy, never
// from key.Mod. So a num-lock keypress clears its text at 1448, passes the gate with
// `keyMod == 0`, and is repopulated with the UNSHIFTED text at 1506 — while the event
// Portal receives still carries the modifier: {Code:'y', Text:"y", Mod:ModNumLock}.
// A mask forgiving only shift and caps lock refuses a plain lowercase `y` for anyone
// typing with num lock on, which is the silent dead key this matcher exists to prevent.
//
// WHAT EXCLUDES `Ctrl-Y` AND `Alt-N` TODAY IS THE TEXT, NOT THE MASK. A combo with a
// modifier past shift carries NO Text in this stack — bubbletea's Key documents it
// ("empty ... for keys that don't represent printable characters like key combos with
// modifier keys") and every ultraviolet path enforces it: the Esc-prefixed alt path
// clears Text outright, control bytes decode to a bare Code plus ModCtrl, and the
// Kitty path's repopulation gate skips them. So the EqualFold alone already refuses
// them — and the mask is what keeps refusing them when that stops being true.
//
// THE ENCODING THE MASK ACTUALLY GUARDS IS NAMED. Kitty's associated-text parameter
// APPENDS to Text (`key.Text += string(rune(code))`, decoder.go:1461-1464) from a parse
// position the loop reaches AFTER the clear at 1448, so `{Text:"y", Mod:ModCtrl}` is a
// shape the decoder emits whenever ReportAssociatedText is negotiated. Portal never
// sets KeyboardEnhancements — bubbletea's keyboardEnhancementsFlags ORs only the basic
// disambiguation flag unconditionally and adds that one on the opt-in — so the shape is
// unreachable here, one struct field away. That is why the mask stays; what it must NOT
// do is refuse the locks.
func themeConfirmAnswer(msg tea.KeyPressMsg, letter string) bool {
	return msg.Mod&^(tea.ModShift|tea.ModCapsLock|tea.ModNumLock) == 0 && strings.EqualFold(msg.Text, letter)
}
