package tui_test

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tui"
)

func TestView(t *testing.T) {
	tests := []struct {
		name     string
		sessions []tmux.Session
		cursor   int
		checks   func(t *testing.T, view string)
	}{
		{
			name: "renders all session names",
			sessions: []tmux.Session{
				{Name: "dev", Windows: 3, Attached: true},
				{Name: "work", Windows: 5, Attached: false},
				{Name: "misc", Windows: 1, Attached: false},
			},
			checks: func(t *testing.T, view string) {
				t.Helper()
				for _, name := range []string{"dev", "work", "misc"} {
					if !strings.Contains(view, name) {
						t.Errorf("view missing session name %q", name)
					}
				}
			},
		},
		{
			name: "shows window count for each session",
			sessions: []tmux.Session{
				{Name: "dev", Windows: 3, Attached: false},
				{Name: "work", Windows: 1, Attached: false},
				{Name: "misc", Windows: 5, Attached: false},
			},
			checks: func(t *testing.T, view string) {
				t.Helper()
				if !strings.Contains(view, "3 windows") {
					t.Error("view missing '3 windows'")
				}
				if !strings.Contains(view, "1 window") {
					t.Error("view missing '1 window'")
				}
				if !strings.Contains(view, "5 windows") {
					t.Error("view missing '5 windows'")
				}
			},
		},
		{
			name: "shows attached indicator for attached sessions",
			sessions: []tmux.Session{
				{Name: "dev", Windows: 3, Attached: true},
				{Name: "work", Windows: 5, Attached: false},
			},
			checks: func(t *testing.T, view string) {
				t.Helper()
				lines := strings.Split(view, "\n")
				var devLine, workLine string
				for _, line := range lines {
					if strings.Contains(line, "dev") {
						devLine = line
					}
					if strings.Contains(line, "work") {
						workLine = line
					}
				}
				if !strings.Contains(devLine, "attached") {
					t.Errorf("attached session 'dev' line missing 'attached': %q", devLine)
				}
				if strings.Contains(workLine, "attached") {
					t.Errorf("detached session 'work' line should not contain 'attached': %q", workLine)
				}
			},
		},
		{
			name: "cursor starts at first session",
			sessions: []tmux.Session{
				{Name: "first", Windows: 1, Attached: false},
				{Name: "second", Windows: 2, Attached: false},
				{Name: "third", Windows: 3, Attached: false},
			},
			checks: func(t *testing.T, view string) {
				t.Helper()
				lines := strings.Split(view, "\n")
				var firstLine, secondLine string
				for _, line := range lines {
					if strings.Contains(line, "first") {
						firstLine = line
					}
					if strings.Contains(line, "second") {
						secondLine = line
					}
				}
				if !strings.Contains(firstLine, ">") {
					t.Errorf("first session should have cursor indicator: %q", firstLine)
				}
				if strings.Contains(secondLine, ">") {
					t.Errorf("second session should not have cursor indicator: %q", secondLine)
				}
			},
		},
		{
			name: "single session renders correctly",
			sessions: []tmux.Session{
				{Name: "solo", Windows: 2, Attached: true},
			},
			checks: func(t *testing.T, view string) {
				t.Helper()
				if !strings.Contains(view, "solo") {
					t.Error("view missing session name 'solo'")
				}
				if !strings.Contains(view, "2 windows") {
					t.Error("view missing '2 windows'")
				}
				if !strings.Contains(view, "attached") {
					t.Error("view missing 'attached' indicator")
				}
				if !strings.Contains(view, ">") {
					t.Error("view missing cursor indicator")
				}
			},
		},
		{
			name: "long session name renders without truncation",
			sessions: []tmux.Session{
				{Name: "my-very-long-project-name-that-should-not-be-truncated-x7k2m9", Windows: 1, Attached: false},
			},
			checks: func(t *testing.T, view string) {
				t.Helper()
				if !strings.Contains(view, "my-very-long-project-name-that-should-not-be-truncated-x7k2m9") {
					t.Error("long session name was truncated")
				}
			},
		},
		{
			name: "sessions displayed in order returned by tmux",
			sessions: []tmux.Session{
				{Name: "zebra", Windows: 1, Attached: false},
				{Name: "alpha", Windows: 2, Attached: false},
				{Name: "middle", Windows: 3, Attached: false},
			},
			checks: func(t *testing.T, view string) {
				t.Helper()
				zebraIdx := strings.Index(view, "zebra")
				alphaIdx := strings.Index(view, "alpha")
				middleIdx := strings.Index(view, "middle")
				if zebraIdx == -1 || alphaIdx == -1 || middleIdx == -1 {
					t.Fatal("not all session names found in view")
				}
				if zebraIdx >= alphaIdx {
					t.Errorf("zebra (idx %d) should appear before alpha (idx %d)", zebraIdx, alphaIdx)
				}
				if alphaIdx >= middleIdx {
					t.Errorf("alpha (idx %d) should appear before middle (idx %d)", alphaIdx, middleIdx)
				}
			},
		},
		{
			name: "window count uses correct pluralisation",
			sessions: []tmux.Session{
				{Name: "one-win", Windows: 1, Attached: false},
				{Name: "two-win", Windows: 2, Attached: false},
			},
			checks: func(t *testing.T, view string) {
				t.Helper()
				lines := strings.Split(view, "\n")
				var oneWinLine, twoWinLine string
				for _, line := range lines {
					if strings.Contains(line, "one-win") {
						oneWinLine = line
					}
					if strings.Contains(line, "two-win") {
						twoWinLine = line
					}
				}
				if !strings.Contains(oneWinLine, "1 window") {
					t.Errorf("single window should show '1 window': %q", oneWinLine)
				}
				if strings.Contains(oneWinLine, "1 windows") {
					t.Errorf("single window should not show '1 windows': %q", oneWinLine)
				}
				if !strings.Contains(twoWinLine, "2 windows") {
					t.Errorf("multiple windows should show '2 windows': %q", twoWinLine)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tui.NewModelWithSessions(tt.sessions)
			view := m.View()
			tt.checks(t, view)
		})
	}
}

// mockSessionLister implements tui.SessionLister for testing.
type mockSessionLister struct {
	sessions []tmux.Session
	err      error
}

func (m *mockSessionLister) ListSessions() ([]tmux.Session, error) {
	return m.sessions, m.err
}

func TestInit(t *testing.T) {
	t.Run("returns command that fetches sessions", func(t *testing.T) {
		sessions := []tmux.Session{
			{Name: "dev", Windows: 3, Attached: true},
			{Name: "work", Windows: 1, Attached: false},
		}
		mock := &mockSessionLister{sessions: sessions}
		m := tui.New(mock)

		cmd := m.Init()
		if cmd == nil {
			t.Fatal("Init() returned nil command")
		}

		msg := cmd()
		sessionsMsg, ok := msg.(tui.SessionsMsg)
		if !ok {
			t.Fatalf("expected SessionsMsg, got %T", msg)
		}
		if sessionsMsg.Err != nil {
			t.Fatalf("unexpected error: %v", sessionsMsg.Err)
		}
		if len(sessionsMsg.Sessions) != 2 {
			t.Fatalf("expected 2 sessions, got %d", len(sessionsMsg.Sessions))
		}
		if sessionsMsg.Sessions[0].Name != "dev" {
			t.Errorf("expected first session name 'dev', got %q", sessionsMsg.Sessions[0].Name)
		}
		if sessionsMsg.Sessions[1].Name != "work" {
			t.Errorf("expected second session name 'work', got %q", sessionsMsg.Sessions[1].Name)
		}
	})
}

func TestUpdate(t *testing.T) {
	t.Run("sessionsMsg populates sessions and sets cursor to zero", func(t *testing.T) {
		m := tui.New(nil)
		sessions := []tmux.Session{
			{Name: "dev", Windows: 3, Attached: true},
			{Name: "work", Windows: 1, Attached: false},
		}

		updated, _ := m.Update(tui.SessionsMsg{Sessions: sessions})
		view := updated.View()

		if !strings.Contains(view, "dev") {
			t.Error("view missing session 'dev' after sessionsMsg")
		}
		if !strings.Contains(view, "work") {
			t.Error("view missing session 'work' after sessionsMsg")
		}

		// Verify cursor is at first session
		lines := strings.Split(view, "\n")
		var devLine string
		for _, line := range lines {
			if strings.Contains(line, "dev") {
				devLine = line
				break
			}
		}
		if !strings.Contains(devLine, ">") {
			t.Errorf("cursor should be on first session after sessionsMsg: %q", devLine)
		}
	})

	t.Run("sessionsMsg with error returns quit command", func(t *testing.T) {
		m := tui.New(nil)
		errMsg := tui.SessionsMsg{Err: fmt.Errorf("tmux not running")}

		_, cmd := m.Update(errMsg)
		if cmd == nil {
			t.Fatal("expected quit command, got nil")
		}

		msg := cmd()
		if _, ok := msg.(tea.QuitMsg); !ok {
			t.Errorf("expected tea.QuitMsg, got %T", msg)
		}
	})
}
