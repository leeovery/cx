package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
)

func newProjectsPageTestModel(t *testing.T, w, h int, th theme.Theme, projects []project.Project) Model {
	t.Helper()
	m := New(fakeLister{}, WithThemeNomination(theme.ConstantNomination(th)))
	m.termWidth = w
	m.termHeight = h
	m.activePage = PageProjects
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	return m
}

func sampleProjects() []project.Project {
	return []project.Project{
		{Name: "flow-v1-api", Path: "/Users/leeovery/Code/fabric/flowv1/flow-v1-api"},
		{Name: "portal", Path: "/Users/leeovery/Code/portal"},
		{Name: "mint", Path: "/Users/leeovery/Code/mint"},
		{Name: "agntc", Path: "/Users/leeovery/Code/agntc"},
	}
}

func TestViewProjectList_ComposesHeaderSectionAndFooter(t *testing.T) {
	m := newProjectsPageTestModel(t, 90, 24, testDarkTheme(t), sampleProjects())
	view := m.viewProjectList()
	visible := ansi.Strip(view)

	if !strings.Contains(visible, "P O R T A L") {
		t.Errorf("composed Projects view missing the PORTAL wordmark:\n%s", visible)
	}
	if !strings.Contains(visible, "Projects") {
		t.Errorf("composed Projects view missing the Projects section label:\n%s", visible)
	}
	if seq := tokenFgSeq(t, testDarkTheme(t).StatePositive); !strings.Contains(view, seq) {
		t.Errorf("composed Projects view missing the state.green label role sequence %q", seq)
	}
	countRun := headerStyle(testDarkTheme(t).TextMuted, testDarkTheme(t), false).Render("4")
	if !strings.Contains(view, countRun) {
		t.Errorf("composed Projects view missing the text.detail count run for 4 projects:\n%s", view)
	}
	if !strings.Contains(view, sectionFilterHint) {
		t.Errorf("composed Projects view missing the %q hint:\n%s", sectionFilterHint, view)
	}

	for _, want := range []string{"⏎ new session", "x sessions", "e edit", "/ filter", "? help"} {
		if !strings.Contains(visible, want) {
			t.Errorf("composed Projects view missing the condensed footer entry %q:\n%s", want, visible)
		}
	}
	for _, banned := range []string{"new in cwd", "delete"} {
		if strings.Contains(visible, banned) {
			t.Errorf("composed Projects view leaked legacy three-column footer copy %q:\n%s", banned, visible)
		}
	}
}

func TestViewProjectList_HeaderSectionRowsShareLeftEdge(t *testing.T) {
	m := newProjectsPageTestModel(t, 90, 24, testDarkTheme(t), sampleProjects())
	view := m.viewProjectList()

	var wordmarkCol, sectionCol, barCol = -1, -1, -1
	for line := range strings.SplitSeq(view, "\n") {
		stripped := strings.TrimLeft(ansi.Strip(line), " ")
		switch {
		case strings.HasPrefix(stripped, "P O R T A L"):
			wordmarkCol = leadingPrintableCol(line)
		case strings.HasPrefix(stripped, "Projects"):
			sectionCol = leadingPrintableCol(line)
		case strings.HasPrefix(stripped, "▌") && barCol < 0:
			barCol = leadingPrintableCol(line)
		}
	}
	if wordmarkCol < 0 || sectionCol < 0 || barCol < 0 {
		t.Fatalf("composed view missing a measured row: wordmarkCol=%d sectionCol=%d barCol=%d\n%s", wordmarkCol, sectionCol, barCol, view)
	}
	if wordmarkCol != sectionCol || sectionCol != barCol {
		t.Errorf("left edges differ: PORTAL=%d Projects=%d bar=%d; all three must share the content's left edge", wordmarkCol, sectionCol, barCol)
	}
}

func TestViewProjectList_ModalClearsToCanvas(t *testing.T) {
	m := newProjectsPageTestModel(t, 90, 24, testDarkTheme(t), sampleProjects())
	m.modal = modalDeleteProject
	m.pendingDeleteName = "portal"
	view := m.viewProjectList()
	visible := ansi.Strip(view)

	if !strings.Contains(visible, "▲ Delete project?") {
		t.Errorf("delete modal header not rendered on the cleared canvas:\n%s", visible)
	}
	if !strings.Contains(visible, "portal") {
		t.Errorf("delete modal target name 'portal' not rendered on the cleared canvas:\n%s", visible)
	}
	if strings.Contains(visible, "⏎ new session") || strings.Contains(visible, "P O R T A L") {
		t.Errorf("modal frame leaked the list/header/footer chrome:\n%s", visible)
	}
}
