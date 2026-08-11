package tui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

// The target is the selected row, never m.themeState.keys — committing the
// persisted slug would write back the theme being replaced.
func (m *Model) commitSelectedConstant() error {
	slug, ok := committableThemeSlug(m.themePanel.list)
	if !ok {
		return nil
	}
	return m.commitConstant(slug)
}

func committableThemeSlug(l list.Model) (string, bool) {
	row, ok := selectedThemeRow(l)
	if !ok || !row.Selectable() || row.Slug == "" {
		return "", false
	}
	return row.Slug, true
}

// A nil persister is inert, not failed. The mirror applies prefs' rule to the
// keys in hand rather than re-reading — the persister's read-modify-write holds
// other instances' writes. On error nothing moves, so the `●` cannot move.
func (m *Model) commitConstant(slug string) error {
	if m.themeState.persister == nil {
		return nil
	}
	err := m.themeState.persister.CommitTheme(slug)
	m.applyCommitResult(err)
	if err != nil {
		return err
	}
	m.themeState.keys = m.themeState.keys.WithConstant(slug)
	m.recomputeThemePanel()
	return nil
}

// The message lasts until the next keypress, the state until a later commit
// succeeds (see themeState.commitFailed). No logging: the persister owns the
// `theme: commit failed` emission.
func (m *Model) applyCommitResult(err error) {
	if err != nil {
		m.reportCommitFailure()
		return
	}
	m.clearThemePanelCommitFailed()
	m.themeState.commitFailed = false
}

// Raised whole: a line without the state dies on the next arrow and the close
// silently reverts; a state without the line reports nothing visible.
func (m *Model) reportCommitFailure() {
	m.raiseThemePanelCommitFailed()
	m.themeState.commitFailed = true
}

// Over a constant a slot write would clear the constant as a side effect — the
// case the confirm gates, so it asks instead of writing. The shape comes from
// themeSetting so it cannot disagree with what the panel resolves.
func (m Model) handleSlotCommitKey(member theme.Member) (tea.Model, tea.Cmd) {
	if !m.themeSetting().IsConstant {
		_ = (&m).commitSelectedSlot(member)
		return m, nil
	}
	if slug, ok := committableThemeSlug(m.themePanel.list); ok {
		(&m).raiseSlotConfirm(slug, member)
	}
	return m, nil
}

func (m *Model) commitSelectedSlot(member theme.Member) error {
	slug, ok := committableThemeSlug(m.themePanel.list)
	if !ok {
		return nil
	}
	return m.commitSlot(slug, member)
}

// Carries commitConstant's rules. The untouched other slot is what makes
// `● both` reachable via `d` then `l` on one row.
func (m *Model) commitSlot(slug string, member theme.Member) error {
	if m.themeState.persister == nil {
		return nil
	}
	err := m.themeState.persister.CommitThemeSlot(slug, member)
	m.applyCommitResult(err)
	if err != nil {
		return err
	}
	m.themeState.keys = m.themeState.keys.WithMember(member, slug)
	m.recomputeThemePanel()
	return nil
}

// Landed writes only: badges and nomination derive from the raw keys, so
// recomputing after a failed write would move the `●` off what is persisted.
// Inputs are the retained enumeration and this instance's keys — not the
// persister's merged bytes (other instances' writes) and not a fresh read (a
// third parse could disagree with the rows shown). The previewed identity is
// captured before the union moves; the cursor re-anchors last, by identity.
func (m *Model) recomputeThemePanel() {
	previewed := previewedThemeIdentity(m.themePanel.list)

	m.themePanel.union = m.themeState.source.Reassemble(m.themePanel.enumeration, m.themeState.keys)
	m.applyCommittedSetting()

	// Always nil: filtering is disabled, the only case SetItems schedules
	// anything.
	_ = m.themePanel.list.SetItems(m.themePanel.rowItems())

	m.anchorThemePanelCursor(previewed)
}

func previewedThemeIdentity(l list.Model) string {
	row, ok := selectedThemeRow(l)
	if !ok {
		return ""
	}
	return row.Identity()
}

// Never ApplyTheme — a commit is a write, not a navigation, and re-theming
// would swap the screen off the live preview. On error nothing moves: a zero
// Resolution would wipe every `●` at the moment the user committed one.
func (m *Model) applyCommittedSetting() {
	resolution, err := m.themeState.source.Resolve(m.themePanel.enumeration, m.themeState.keys)
	if err != nil {
		return
	}
	m.themePanel.badges = theme.Badges(resolution.Slots)
	m.themeState.nomination = resolution.Nomination
}
