package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// The slot-from-constant confirm: the panel's one gate, guarding the case where a
// keypress the user was told is inert silently costs them a setting they chose.
//
// `d`/`l` assign a slot, and committing to a non-active slot normally changes
// nothing on screen. Over a constant that is false in a way the user cannot see
// coming: on `"theme": "nord"`, pressing `l` clears the constant, the untouched
// dark slot falls back to the shipped default, and `Esc` in a dark terminal lands
// on `tokyo-night` rather than `nord`. `Enter` over a pair raises no confirm,
// because it visibly does what it says.
//
// Three properties are load-bearing:
//
//   - It is key-exclusive within the panel. Its arm sits at the top of
//     updateThemePanel, ahead of the arrows, the commit keys and the `Esc` close.
//     An arrow that moved the cursor mid-question would re-theme the screen while
//     the user is answering about a row that has stopped being the previewed one.
//   - It resolves on exactly three inputs — `y`/`Y`, `n`/`N`/`Esc`, and `Ctrl-C`,
//     which quits rather than cancels (the global quit stays live everywhere).
//     Everything else is swallowed and the question persists; nothing has been
//     written, so there is no partial state to leave behind.
//   - `Esc` cancels rather than closes: the innermost thing resolves first, the
//     same nesting rule the panel applies over multi-select.
//
// Nothing is written until `y`. The raise records what it would write and nothing
// more, so a cancel is inert by construction rather than by an undo.

// themeSlotConfirm is the assignment a live confirm will apply when the user
// answers `y`: the slug under the cursor at the moment the question was asked, and
// the typed slot the keypress named.
//
// The slug is captured at the raise rather than re-read at the answer, so the
// question on screen and the write name the same row independently of what the
// key routing allows.
type themeSlotConfirm struct {
	// slug is the row the cursor was on when the confirm was raised.
	slug string
	// slot is the half of the adaptive pair `d`/`l` named — the typed prefs value
	// threaded from the keypress, so no path here can mint a third slot.
	slot prefs.ThemeSlot
}

// confirming reports whether the confirm is live, read off the message slot
// rather than off a second flag beside it.
//
// The slot is the confirm's whole visible existence (the question, and the footer
// scope themePanelFooterScope substitutes from it), so deriving liveness from
// anything else would admit a state where the panel is answering a question it is
// not asking.
func (p themePanel) confirming() bool {
	return p.message.Kind == themeMessageConfirm
}

// raiseSlotConfirm asks the question: it records the pending assignment and
// raises the confirm naming the constant that will be cleared.
//
// Nothing is written here — the write waits on `y` — so the raw keys, the badges,
// the cursor and the previewed theme are left exactly as found.
//
// The message's own datum is the persisted constant, read by the writer from
// m.themeState.keys (see Model.raiseThemePanelConfirm for why the raw key rather
// than the resolution): the confirm names what is being cleared, while the pending
// value names what is being set. Under a fallback those two rows are not even the
// same theme.
func (m *Model) raiseSlotConfirm(slug string, slot prefs.ThemeSlot) {
	m.themePanel.pending = themeSlotConfirm{slug: slug, slot: slot}
	m.raiseThemePanelConfirm()
}

// resolveSlotConfirm takes the question down — the message slot emptied and the
// footer restored with it — and hands the caller what was pending.
//
// All three answers resolve through here, so a confirm cannot be cleared in one
// direction and left standing in the other. The pending value is returned rather
// than read afterwards, which lets the clear happen first: `y`'s commit runs
// against a panel that has already stopped asking, so no failure path can leave a
// resolved question on screen.
func (m *Model) resolveSlotConfirm() themeSlotConfirm {
	pending := m.themePanel.pending
	m.themePanel.pending = themeSlotConfirm{}
	m.clearThemePanelMessage()
	return pending
}

// updateSlotConfirm is the confirm's key-exclusive arm: the inputs that resolve
// the confirm, and the swallow that everything else meets.
//
// `Ctrl-C` is deliberately absent from this switch. It is answered one level up,
// at updateThemePanel's own first arm, because it is the global quit rather than
// a confirm answer; restating it here would be a second place for it to be
// conditionally lost.
//
// The default arm is the feature: arrows, paging, `Enter`, the other slot key and
// every page key land here and change nothing — the question stands until it is
// answered.
func (m Model) updateSlotConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case themeConfirmAnswer(msg, themeConfirmYesKey):
		(&m).confirmSlotAssignment()
		return m, nil
	case themeConfirmAnswer(msg, themeConfirmNoKey), keyIsCode(msg, tea.KeyEscape):
		// The pending assignment is discarded: a cancel writes nothing, so the
		// value resolveSlotConfirm hands back has no reader on this path.
		(&m).resolveSlotConfirm()
		return m, nil
	default:
		return m, nil
	}
}

// confirmSlotAssignment is `y`: the constant cleared and the slot written in one
// atomic prefs write (prefs.SaveThemeSlot), mirrored in memory, and the panel
// recomputed against it.
//
// Order matters: the confirm comes down first and unconditionally. The question
// has been answered whichever way the write goes, so the footer is restored on
// both paths and a failed-commit line is raised into a message slot the confirm
// has already vacated rather than one it competes for.
//
// The write is commitSlot's, so every rule that path carries applies unchanged —
// a nil persister is inert rather than failed, the mirror is applied to the
// construction-time snapshot, on error nothing moves, and the recompute is its
// last step past both early returns. On failure this arm only returns; the
// message line and the outstanding-failure state are raised inside commitSlot.
//
// The persister is re-read below rather than inferred from the nil error, because
// commitSlot returns nil both for a write that landed and for the absence of a
// writer. loadNewlyLiveSlot requires mirrored keys: on the writer-less path the
// keys still hold the constant, whose slot slugs are both empty, and it would
// report a commit-time `theme: loaded` for a write that never happened.
func (m *Model) confirmSlotAssignment() {
	pending := m.resolveSlotConfirm()
	if err := m.commitSlot(pending.slug, pending.slot); err != nil {
		return
	}
	if m.themeState.persister == nil {
		return
	}
	m.loadNewlyLiveSlot(pending.slot)
}

// loadNewlyLiveSlot completes the constant → adaptive transition in both halves:
// the opposite slot's palette (the file) and the in-force light/dark
// classification (the answer).
//
// Assigning a slot over a constant makes the slot the user did not assign live in
// the same keypress, and construction never loaded it — construction loads
// nominated themes only, and under a constant the slots are not read. This is the
// one theme load outside construction, and the reason `theme: loaded` has a
// commit-time cadence.
//
// It hangs off the confirm rather than off commitSlot because only the confirm
// creates the state: a `d`/`l` over an adaptive pair changes which slug a live
// slot names and makes nothing newly live. Reaching here is therefore itself the
// converting condition — handleSlotCommitKey raises the confirm on a constant
// setting only — rather than something to re-test against keys the mirror has
// since cleared.
//
// It must run after a write that landed, past commitSlot's own mirror and
// recompute and never on the nil-persister path: the load needs the mirrored keys
// to know which slug the opposite slot now names, while the recompute needs
// nothing the load produces. Nothing renders between the two.
//
// The assigned slot is deliberately not re-read "for symmetry": its parse is
// already in hand as the previewed palette, so resolving it again would re-parse
// a file the panel already holds and mint a second answer for one slug.
//
// The opposite slug comes off the mirrored keys through themeSetting, which is
// the single site that substitutes the shipped default for an unset slot. An
// untouched slot therefore resolves the shipped default with FellBack false,
// rather than arriving empty and being reported as a fallback of a slug nobody
// set.
//
// On the broken-builtin fatal it degrades and nothing moves — the policy
// applyInForceTheme states for every panel call site of the seam. Returning
// before the two assignments is what makes that a genuine no-op rather than a
// half-converted model holding a light/dark answer with no pair to select from.
//
// It never calls ApplyTheme: a commit is a write, not a navigation, so the pair is
// completed while the screen keeps previewing whatever the cursor is on.
func (m *Model) loadNewlyLiveSlot(assigned prefs.ThemeSlot) {
	newlyLive := oppositeThemeMember(assigned)
	slot := newlyLive.Slot()
	resolved, err := m.themeState.enumerator.ResolveSlot(m.themePanel.enumeration, slot, m.persistedSlotSlug(slot))
	if err != nil {
		return
	}

	m.themeState.canvasMode = m.retainedCanvasAnswer()
	m.themeState.nomination = theme.AdaptivePair(
		newlyLive.Palette(resolved.Theme),
		newlyLive.Opposite().Palette(m.themeState.active),
	)
}

// oppositeThemeMember is the half of the adaptive pair the keypress did not
// assign.
//
// It maps the prefs slot the keypress threaded onto the theme package's
// light/dark answer; the two are separate types because one is what prefs.json
// writes and the other is which palette of a pair is in play. Neither side has a
// third value to fall through to — prefs.SaveThemeSlot rejects an out-of-range
// slot and commitSlot has already returned on that error.
func oppositeThemeMember(assigned prefs.ThemeSlot) theme.Member {
	if assigned == prefs.SlotLight {
		return theme.MemberDark
	}
	return theme.MemberLight
}

// persistedSlotSlug is the slug one slot of the mirrored keys nominates, with
// the shipped default already substituted for an unset one.
//
// It reads through themeSetting rather than off RawKeys so the unset-slot rule
// keeps the single site it has everywhere else in the panel; a second
// substitution here is how an untouched slot would come to resolve one default in
// the badges and another in the load.
func (m Model) persistedSlotSlug(slot theme.Slot) string {
	setting := m.themeSetting()
	if slot == theme.SlotLight {
		return setting.Light
	}
	return setting.Dark
}

// retainedCanvasAnswer is the transition's answer half: the light/dark
// classification of the OSC 11 reply this launch already received, which a
// constant-theme launch retained without ever turning into an answer.
//
// Model.Init issues the OSC 11 query regardless of the setting's shape, because
// restore-on-exit needs the original background independently of detection. A
// conversion therefore uses an answer already in hand: no new query, no new race,
// and the resolve-once rule untouched — a reply landing after this point still
// never re-themes.
//
// It is not read off the gate: a constant's gate is pinned to the standing dark
// fallback rather than a classification of the terminal, so reading it would
// answer "dark" for every light terminal. The retained reply is the only value
// that is a fact about the terminal.
//
// No reply at all falls to dark, the standing no-answer fallback. A
// no-answer-shaped reply is a different state — a nil colour arrives, leaving an
// empty hex and no colour to classify — which is why the read is keyed on the
// arrival rather than on a non-empty originalBg; IsDark reports a nil colour as
// dark either way.
func (m Model) retainedCanvasAnswer() canvasAppearance {
	if m.themeState.bgReplyArrived && !m.themeState.bgReplyDark {
		return appearanceLightCanvas
	}
	return appearanceDarkCanvas
}

// The two answer letters, as the dispatch matches them. They must stay in step
// with the letters themePanelConfirmKeymap advertises in the substituted footer:
// the descriptor carries them for display, this pair for dispatch.
const (
	themeConfirmYesKey = "y"
	themeConfirmNoKey  = "n"
)

// themeConfirmAnswer reports whether msg answers the confirm with the given
// letter, in either case: `y` or `Y` confirms, `n`, `N` or `Esc` cancels.
//
// The modifier mask is deliberately not isRuneKey's `Mod == 0`. An uppercase
// answer never reaches Portal with a clear modifier field: both the legacy and
// the Kitty encodings converge on the base rune carrying ModShift with the
// shifted text, so requiring a clear field would leave `Y` dead on every
// terminal.
//
// Caps lock and num lock must be forgiven alongside shift for the same reason:
// the decoder leaves either riding on an event whose text is an ordinary answer
// letter, and a num-lock `y` arrives as {Code:'y', Text:"y", Mod:ModNumLock}
// because the decoder strips the lock modifiers only on a local copy when
// deciding whether to repopulate the text.
//
// `Ctrl-Y` and `Alt-N` are excluded by the EqualFold on Text rather than by the
// mask: a combo with a modifier past shift carries no Text in this stack. The
// mask is what keeps refusing them if that stops being true — Kitty's
// associated-text parameter appends to Text after the clear, so {Text:"y",
// Mod:ModCtrl} is emitted whenever ReportAssociatedText is negotiated, which
// Portal is one opt-in struct field away from.
func themeConfirmAnswer(msg tea.KeyPressMsg, letter string) bool {
	return msg.Mod&^(tea.ModShift|tea.ModCapsLock|tea.ModNumLock) == 0 && strings.EqualFold(msg.Text, letter)
}
