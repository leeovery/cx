package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/project"
)

type projectsParityCreator struct{}

func (projectsParityCreator) CreateFromDir(string, []string) (string, error) { return "new", nil }

type projectsParityStore struct{}

func (projectsParityStore) List() ([]project.Project, error)       { return nil, nil }
func (projectsParityStore) CleanStale() ([]project.Project, error) { return nil, nil }
func (projectsParityStore) Remove(string, string) error            { return nil }

type stubTagsAliasEditor struct{}

func (stubTagsAliasEditor) Load() (map[string]string, error)        { return map[string]string{}, nil }
func (stubTagsAliasEditor) SetAndSave(_, _, _ string) error         { return nil }
func (stubTagsAliasEditor) DeleteAndSave(_, _ string) (bool, error) { return false, nil }

type stubTagsProjectEditor struct{}

func (stubTagsProjectEditor) Rename(_, _, _ string) error { return nil }
func (stubTagsProjectEditor) AddTag(_, _ string) error    { return nil }
func (stubTagsProjectEditor) RemoveTag(_, _ string) error { return nil }

func projectsDispatchModel(t *testing.T) Model {
	t.Helper()
	projects := []project.Project{
		{Path: "/p/one", Name: "one"},
	}
	m := Model{
		themeState:     themeState{active: testDarkTheme(t)},
		projects:       projects,
		projectList:    newProjectList(),
		activePage:     PageProjects,
		projectEditor:  stubTagsProjectEditor{},
		aliasEditor:    stubTagsAliasEditor{},
		projectStore:   projectsParityStore{},
		sessionCreator: projectsParityCreator{},
	}
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.projectList.Select(0)
	return m
}

func pressProject(t *testing.T, m Model, msg tea.KeyPressMsg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.updateProjectsPage(msg)
	return updated.(Model), cmd
}

func projectsNavModel(t *testing.T) Model {
	t.Helper()
	projects := []project.Project{
		{Path: "/p/one", Name: "one"},
		{Path: "/p/two", Name: "two"},
		{Path: "/p/three", Name: "three"},
		{Path: "/p/four", Name: "four"},
	}
	m := Model{
		themeState:  themeState{active: testDarkTheme(t)},
		projects:    projects,
		projectList: newProjectList(),
		activePage:  PageProjects,
	}
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.projectList.Select(0)
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	return m
}

func TestProjectsKeymapRevision(t *testing.T) {
	t.Run("it no longer toggles to Sessions on s", func(t *testing.T) {
		m := projectsDispatchModel(t)
		if m.activePage != PageProjects {
			t.Fatalf("precondition: want PageProjects, got %d", m.activePage)
		}
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: 's', Text: "s"})
		if m.activePage != PageProjects {
			t.Errorf("s must NOT toggle to Sessions (the s alias is dropped); active page = %d", m.activePage)
		}
	})

	t.Run("it is a harmless no-op on s (no modal, no crash)", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: 's', Text: "s"})
		if m.modal != modalNone {
			t.Errorf("s must not open a modal; modal = %v", m.modal)
		}
		if m.activePage != PageProjects {
			t.Errorf("s must stay on Projects; active page = %d", m.activePage)
		}
	})

	t.Run("it still toggles Projects→Sessions on x", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m, cmd := pressProject(t, m, tea.KeyPressMsg{Code: 'x', Text: "x"})
		if m.activePage != PageSessions {
			t.Errorf("x must toggle to Sessions; active page = %d", m.activePage)
		}
		_ = cmd
	})

	t.Run("it introduces no uppercase page-toggle binding", func(t *testing.T) {
		for _, k := range []tea.KeyPressMsg{
			{Code: 'S', Text: "S"},
			{Code: 'X', Text: "X"},
		} {
			m := projectsDispatchModel(t)
			m, _ = pressProject(t, m, k)
			if m.activePage != PageProjects {
				t.Errorf("uppercase key %+v must not toggle the page (no uppercase); active page = %d", k, m.activePage)
			}
		}
	})

	t.Run("it does not navigate via vim/uppercase/page-jump aliases on Projects", func(t *testing.T) {
		bannedNav := []tea.KeyPressMsg{
			{Code: 'j', Text: "j"},
			{Code: 'k', Text: "k"},
			{Code: 'h', Text: "h"},
			{Code: 'l', Text: "l"},
			{Code: 'g', Text: "g"},
			{Code: 'G', Text: "G"},
			{Code: 'b', Text: "b"},
			{Code: 'u', Text: "u"},
			{Code: 'f', Text: "f"},
			{Code: tea.KeyPgUp},
			{Code: tea.KeyPgDown},
			{Code: tea.KeyHome},
			{Code: tea.KeyEnd},
		}
		for _, k := range bannedNav {
			m := projectsNavModel(t)
			start := m.projectList.Index()
			m, _ = pressProject(t, m, k)
			if m.projectList.Index() != start {
				t.Errorf("key %+v must not move the Projects cursor (arrows only); index %d → %d", k, start, m.projectList.Index())
			}
			if m.activePage != PageProjects {
				t.Errorf("key %+v must not change the page; got %d", k, m.activePage)
			}
			if m.modal != modalNone {
				t.Errorf("key %+v must not open a modal; modal = %v", k, m.modal)
			}
		}
	})

	t.Run("it moves the cursor with ↑/↓ only and pages with Ctrl+↑/↓", func(t *testing.T) {
		m := projectsNavModel(t)
		start := m.projectList.Index()
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: tea.KeyDown})
		if m.projectList.Index() != start+1 {
			t.Errorf("↓ must move the Projects cursor down one; index %d → %d", start, m.projectList.Index())
		}
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: tea.KeyUp})
		if m.projectList.Index() != start {
			t.Errorf("↑ must move the Projects cursor back up one; index %d", m.projectList.Index())
		}

		if len(m.projectList.KeyMap.NextPage.Keys()) == 0 {
			t.Errorf("Ctrl+↓ paging must stay bound on Projects; NextPage keys are empty")
		}
		if len(m.projectList.KeyMap.PrevPage.Keys()) == 0 {
			t.Errorf("Ctrl+↑ paging must stay bound on Projects; PrevPage keys are empty")
		}
		if got := m.projectList.KeyMap.NextPage.Keys(); len(got) != 1 || got[0] != "ctrl+down" {
			t.Errorf("NextPage must be Ctrl+↓ only; keys = %v", got)
		}
		if got := m.projectList.KeyMap.PrevPage.Keys(); len(got) != 1 || got[0] != "ctrl+up" {
			t.Errorf("PrevPage must be Ctrl+↑ only; keys = %v", got)
		}
	})
}

func TestProjectsRetainedActionParity(t *testing.T) {
	t.Run("e routes to edit (opens the edit project modal)", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: 'e', Text: "e"})
		if m.modal != modalEditProject {
			t.Errorf("e must open the edit project modal; modal = %v", m.modal)
		}
		if m.editProject.Name != "one" {
			t.Errorf("edit target = %q, want one", m.editProject.Name)
		}
	})

	t.Run("d routes to delete (opens the delete project confirm modal)", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: 'd', Text: "d"})
		if m.modal != modalDeleteProject {
			t.Errorf("d must open the delete confirm modal; modal = %v", m.modal)
		}
		if m.pendingDeleteName != "one" {
			t.Errorf("delete target = %q, want one", m.pendingDeleteName)
		}
	})

	t.Run("n routes to new-in-cwd (createSession dispatch)", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m, cmd := pressProject(t, m, tea.KeyPressMsg{Code: 'n', Text: "n"})
		if m.modal != modalNone {
			t.Errorf("n must not open a modal; modal = %v", m.modal)
		}
		if m.activePage != PageProjects {
			t.Errorf("n must stay on Projects; active page = %d", m.activePage)
		}
		if cmd == nil {
			t.Errorf("n must produce a createSession cmd (sessionCreator wired), got nil")
		}
	})

	t.Run("Enter routes to new-session-from-project (createSession dispatch)", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m, cmd := pressProject(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
		if m.modal != modalNone {
			t.Errorf("Enter must not open a modal; modal = %v", m.modal)
		}
		if m.activePage != PageProjects {
			t.Errorf("Enter must stay on Projects; active page = %d", m.activePage)
		}
		if cmd == nil {
			t.Errorf("Enter must produce a createSession cmd (sessionCreator wired), got nil")
		}
	})

	t.Run("q and Ctrl+C quit", func(t *testing.T) {
		for _, k := range []tea.KeyPressMsg{
			{Code: 'q', Text: "q"},
			{Code: 'c', Mod: tea.ModCtrl},
		} {
			m := projectsDispatchModel(t)
			_, cmd := m.updateProjectsPage(k)
			if cmd == nil {
				t.Errorf("key %+v must produce a quit cmd, got nil", k)
				continue
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Errorf("key %+v must quit, got a non-quit cmd", k)
			}
		}
	})

	t.Run("Esc quits when no filter is applied", func(t *testing.T) {
		m := projectsDispatchModel(t)
		_, cmd := m.updateProjectsPage(tea.KeyPressMsg{Code: tea.KeyEscape})
		if cmd == nil {
			t.Fatalf("Esc with no filter must quit, got nil cmd")
		}
		if _, ok := cmd().(tea.QuitMsg); !ok {
			t.Errorf("Esc with no filter must quit")
		}
	})

	t.Run("? opens the per-page help modal (no list self-toggle)", func(t *testing.T) {
		m := projectsDispatchModel(t)
		_, cmd := m.updateProjectsPage(tea.KeyPressMsg{Code: '?', Text: "?"})
		if cmd != nil {
			t.Errorf("? must be consumed (return nil cmd), got a non-nil cmd")
		}
		after, _ := pressProject(t, m, tea.KeyPressMsg{Code: '?', Text: "?"})
		if after.activePage != PageProjects {
			t.Errorf("? must not change the active page; got %d", after.activePage)
		}
		if after.modal != modalHelp {
			t.Errorf("? must open the help modal; modal = %v, want modalHelp", after.modal)
		}
	})
}

func TestProjectsCommandPendingGatingUnchanged(t *testing.T) {
	t.Run("x is a no-op in command-pending mode", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m.commandPending = true
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: 'x', Text: "x"})
		if m.activePage != PageProjects {
			t.Errorf("x must be a no-op in command-pending mode; active page = %d", m.activePage)
		}
	})

	t.Run("e is a no-op in command-pending mode", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m.commandPending = true
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: 'e', Text: "e"})
		if m.modal != modalNone {
			t.Errorf("e must be a no-op in command-pending mode; modal = %v", m.modal)
		}
	})

	t.Run("d is a no-op in command-pending mode", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m.commandPending = true
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: 'd', Text: "d"})
		if m.modal != modalNone {
			t.Errorf("d must be a no-op in command-pending mode; modal = %v", m.modal)
		}
	})

	t.Run("s is a no-op in command-pending mode (still no page toggle)", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m.commandPending = true
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: 's', Text: "s"})
		if m.activePage != PageProjects {
			t.Errorf("s must stay on Projects in command-pending mode; active page = %d", m.activePage)
		}
	})
}
