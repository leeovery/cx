package tui

import (
	"strings"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

const emptySessionsGlyph = "▌ ▌ ▌"

const emptySessionsMessage = "No sessions yet"

const emptySessionsHint = "Press n to start one in the current directory · x for projects"

const emptyProjectsGlyph = "▌ ▌ ▌"

const emptyProjectsMessage = "No projects yet"

const emptyProjectsHint = "Press n to start one in the current directory · x for sessions"

func renderEmptyStateBody(glyph, message, hint string, width, height int, th theme.Theme, colourless bool) string {
	w := headerWidthOrFallback(width)
	g := headerStyle(th.TextFaint, th, colourless).Render(glyph)
	msg := headerStyle(th.TextPrimary, th, colourless).Bold(true).Render(message)
	h := headerStyle(th.TextMuted, th, colourless).Render(hint)
	stack := lipgloss.JoinVertical(lipgloss.Center, g, "", msg, h)
	return lipgloss.Place(
		w, height,
		lipgloss.Center, lipgloss.Center,
		stack,
		lipgloss.WithWhitespaceStyle(headerCanvasBg(th, colourless)),
	)
}

// The Unfiltered requirement keeps this distinct from the no-matches state — a
// model with an active filter never enters the empty state.
func (m Model) sessionListEmpty() bool {
	if m.sessionList.FilterState() != list.Unfiltered {
		return false
	}
	return len(m.sessionList.VisibleItems()) == 0
}

func (m Model) projectListEmpty() bool {
	if m.projectList.FilterState() != list.Unfiltered {
		return false
	}
	return len(m.projectList.VisibleItems()) == 0
}

// The body renders at the list body's own height, so the composed view height
// — and the one-row-per-delegate pagination invariant — is unchanged.
func (m Model) replaceListBodyWithEmptyState(listView string, listHeight int, glyph, message, hint string) string {
	bodyHeight := max(listHeight-1, 1)
	body := renderEmptyStateBody(glyph, message, hint, m.contentWidth(), bodyHeight, m.themeState.active, m.colourless)
	idx := strings.IndexByte(listView, '\n')
	if idx < 0 {
		return listView + "\n" + body
	}
	return listView[:idx+1] + body
}

var emptyFooterKeys = []string{"n", "x", "/", "?"}

func emptyFooterDescriptor(keymap []keymapEntry) []keymapEntry {
	byKey := make(map[string]keymapEntry, len(keymap))
	for _, e := range keymap {
		byKey[e.Key] = e
	}
	entries := make([]keymapEntry, 0, len(emptyFooterKeys))
	for _, k := range emptyFooterKeys {
		e, ok := byKey[k]
		if !ok {
			continue
		}
		// Force Core so the condensed footer includes n, which is help-only in
		// the standard footer; RightAligned survives so ? stays the anchor.
		e.Core = true
		entries = append(entries, e)
	}
	return entries
}

func renderEmptySessionsFooter(width int, th theme.Theme, colourless bool) string {
	return renderCondensedFooter(emptyFooterDescriptor(sessionsKeymap()), width, th, colourless)
}

func renderEmptyProjectsFooter(width int, th theme.Theme, colourless bool) string {
	return renderCondensedFooter(emptyFooterDescriptor(projectsKeymap()), width, th, colourless)
}
