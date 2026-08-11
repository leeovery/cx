package tui

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/prefs"
)

func sessionsFooterString(m Model) string {
	return renderSessionsFooter(m.sessionsHelpKeymap(), m.contentWidth(), m.themeState.active, m.colourless)
}

func TestSessionsFooter_ShowsSwitchViewHint(t *testing.T) {
	m := flashModelWithSessions("alpha-row", "beta-row")
	m.termWidth = 120

	footer := sessionsFooterString(m)
	if !strings.Contains(footer, "switch view") {
		t.Errorf("sessions footer must contain %q hint, got:\n%s", "switch view", footer)
	}
	if !strings.Contains(footer, "projects") {
		t.Errorf("sessions footer must contain %q hint, got:\n%s", "projects", footer)
	}

	view := m.View().Content
	if !strings.Contains(view, "switch view") {
		t.Errorf("rendered sessions View() must contain %q, got:\n%s", "switch view", view)
	}
}

func TestSessionsFooter_ShowsSwitchViewHintAtZeroSessions(t *testing.T) {
	m := NewModelWithSessions(nil)
	m.termWidth = 120
	m.termHeight = 24

	footer := sessionsFooterString(m)
	if !strings.Contains(footer, "switch view") {
		t.Errorf("standard sessions footer must contain %q, got:\n%s", "switch view", footer)
	}

	view := m.View().Content
	if strings.Contains(view, "switch view") {
		t.Errorf("rendered empty-sessions View() must REPLACE the footer (no %q), got:\n%s", "switch view", view)
	}
	if !strings.Contains(view, "new in cwd") {
		t.Errorf("rendered empty-sessions View() must show the replaced empty-state footer, got:\n%s", view)
	}
}

func TestProjectsFooter_NoSwitchViewHint(t *testing.T) {
	m := flashModelWithSessions("alpha-row")
	m.activePage = PageProjects
	m.termWidth = 120

	footer := footerVisible(renderProjectsFooter(m.projectsHelpKeymap(), m.contentWidth(), m.themeState.active, m.colourless))
	if strings.Contains(footer, "switch view") {
		t.Errorf("projects footer must NOT contain %q, got:\n%s", "switch view", footer)
	}
	if strings.Contains(footer, "s/x") {
		t.Errorf("projects footer must NOT show the dropped %q alias copy, got:\n%s", "s/x", footer)
	}
	if !strings.Contains(footer, "x sessions") {
		t.Errorf("projects footer must show %q, got:\n%s", "x sessions", footer)
	}
}

func TestCommandPendingFooter_NoSwitchViewHint(t *testing.T) {
	m := flashModelWithSessions("alpha-row")
	m.activePage = PageProjects
	m.commandPending = true

	footer := footerVisible(renderCommandPendingFooter(m.contentWidth(), m.themeState.active, m.colourless))
	if strings.Contains(footer, "switch view") {
		t.Errorf("command-pending footer must NOT contain %q, got:\n%s", "switch view", footer)
	}
}

func TestSessionsFooter_ShowsSwitchAndProjectsOnFlatView(t *testing.T) {
	m := flashModelWithSessions("alpha-row", "beta-row")
	m.termWidth = 120
	m.sessionListMode = prefs.ModeFlat
	if m.sessionListMode != prefs.ModeFlat {
		t.Fatalf("precondition: want Flat view, got %v", m.sessionListMode)
	}

	footer := sessionsFooterString(m)
	for _, want := range []string{"switch view", "projects"} {
		if !strings.Contains(footer, want) {
			t.Errorf("Flat-view footer must contain %q, got:\n%s", want, footer)
		}
	}
}
