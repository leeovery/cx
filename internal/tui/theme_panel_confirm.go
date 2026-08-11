package tui

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

// The slug is captured at the raise, not re-read at the answer, so the question
// on screen and the write name the same row.
type themeSlotConfirm struct {
	slug   string
	member theme.Member
}

// Read off the message slot rather than a second flag, so liveness cannot
// diverge from what is on screen.
func (p themePanel) confirming() bool {
	return p.message.Kind == themeMessageConfirm
}

func (m *Model) raiseSlotConfirm(slug string, member theme.Member) {
	m.themePanel.pending = themeSlotConfirm{slug: slug, member: member}
	m.raiseThemePanelConfirm()
}

// Returns pending so the clear happens first: `y`'s commit runs against a panel
// that has stopped asking, so no failure path leaves a resolved question up.
func (m *Model) resolveSlotConfirm() themeSlotConfirm {
	pending := m.themePanel.pending
	m.themePanel.pending = themeSlotConfirm{}
	m.clearThemePanelMessage()
	return pending
}

// Ctrl-C is answered one level up. The default swallow is deliberate: the
// question stands until answered.
func (m Model) updateSlotConfirm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case themeConfirmAnswer(msg, themeConfirmYesKey):
		(&m).confirmSlotAssignment()
		return m, nil
	case themeConfirmAnswer(msg, themeConfirmNoKey), keyIsCode(msg, tea.KeyEscape):
		(&m).resolveSlotConfirm()
		return m, nil
	default:
		return m, nil
	}
}

// The confirm comes down first, whichever way the write goes. The persister
// nil-check cannot be inferred from the nil error — commitSlot returns nil on the
// writer-less path too, where the keys still hold the constant.
func (m *Model) confirmSlotAssignment() {
	pending := m.resolveSlotConfirm()
	if err := m.commitSlot(pending.slug, pending.member); err != nil {
		return
	}
	if m.themeState.persister == nil {
		return
	}
	m.loadNewlyLiveSlot(pending.member)
}

// The classification is assigned first and unconditionally — gating it on the
// load would let a failed palette decide which half is in force. The palette is
// deliberately discarded: applyCommittedSetting owns the nomination. Never
// ApplyTheme — the screen keeps previewing the cursor's row.
func (m *Model) loadNewlyLiveSlot(assigned theme.Member) {
	m.themeState.adoptRetainedReply()

	newlyLive := assigned.Opposite()
	if _, err := m.themeState.source.ResolveSlot(m.themePanel.enumeration, newlyLive.Slot(), m.themeState.keys); err != nil {
		return
	}
}

// Must stay in step with the letters themePanelConfirmKeymap advertises.
const (
	themeConfirmYesKey = "y"
	themeConfirmNoKey  = "n"
)

// Deliberately not Mod==0: uppercase always arrives with ModShift (legacy and
// Kitty alike) and caps/num lock ride along, so a clear-field mask leaves `Y`
// dead. Ctrl/Alt carry no Text in this stack; the mask backstops if that changes.
func themeConfirmAnswer(msg tea.KeyPressMsg, letter string) bool {
	return msg.Mod&^(tea.ModShift|tea.ModCapsLock|tea.ModNumLock) == 0 && strings.EqualFold(msg.Text, letter)
}
