package tui

import (
	"fmt"

	"charm.land/bubbles/v2/list"
	"github.com/leeovery/portal/internal/theme"
)

const noMatchesGlyph = "∅"

const noMatchesHint = "⌫ to widen the search · esc to clear the filter"

// Literal quote bytes, never %q, so the query renders byte-exact regardless of
// its content.
func formatNoMatchesMessage(query string) string {
	return fmt.Sprintf(`No sessions match "%s"`, query)
}

// The active-non-empty-query requirement keeps this distinct from the
// empty-sessions state.
func (m Model) sessionListNoMatches() bool {
	st := m.sessionList.FilterState()
	if st != list.Filtering && st != list.FilterApplied {
		return false
	}
	if m.sessionList.FilterValue() == "" {
		return false
	}
	return len(m.sessionList.VisibleItems()) == 0
}

func renderNoMatchesBody(query string, width, height int, th theme.Theme, colourless bool) string {
	return renderEmptyStateBody(noMatchesGlyph, formatNoMatchesMessage(query), noMatchesHint, width, height, th, colourless)
}

func noMatchesFooterEntries(th theme.Theme) []filterFooterEntry {
	return dropBrowseResults(filteringFooterEntries(th))
}

func renderNoMatchesFooter(width int, th theme.Theme, colourless bool) string {
	return renderFilterFooter(noMatchesFooterEntries(th), width, th, colourless)
}
