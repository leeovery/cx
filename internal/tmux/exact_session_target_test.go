package tmux_test

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

// The two exact forms tmux needs, and the reason each site takes the one it
// does: tmux only honours the "=" exact-match prefix when it parses the target
// as a session. A command whose -t is a window or pane target ("coordinate"
// below) parses a bare "=foo" as a window/pane name, falls through to its own
// fuzzy lookup and prefix-matches anyway — the trailing ":" is what forces the
// leading component to be read as a session name. set-option is the trap: it
// writes a session option, but resolves a pane target to get there, so it takes
// the coordinate form despite being a per-session write.
const (
	sessionTargetForm = "=exact"
	coordTargetForm   = "=exact:"
)

func targetArgOf(t *testing.T, call []string) string {
	t.Helper()
	i := slices.Index(call, "-t")
	if i < 0 || i+1 >= len(call) {
		t.Fatalf("call %q composes no -t argument", call)
	}
	return call[i+1]
}

func TestSessionTargetsAreComposedExactly(t *testing.T) {
	const session = "exact"

	cases := []struct {
		name    string
		command string
		want    string
		invoke  func(*tmux.Client)
	}{
		{
			name:    "ShowEnvironment",
			command: "show-environment",
			want:    sessionTargetForm,
			invoke:  func(c *tmux.Client) { _, _ = c.ShowEnvironment(session) },
		},
		{
			name:    "SetSessionEnvironment",
			command: "set-environment",
			want:    sessionTargetForm,
			invoke:  func(c *tmux.Client) { _ = c.SetSessionEnvironment(session, "K", "v") },
		},
		{
			name:    "SetSessionOption",
			command: "set-option",
			want:    coordTargetForm,
			invoke:  func(c *tmux.Client) { _ = c.SetSessionOption(session, "@k", "v") },
		},
		{
			name:    "ActivePaneCurrentPath",
			command: "display-message",
			want:    coordTargetForm,
			invoke:  func(c *tmux.Client) { _, _ = c.ActivePaneCurrentPath(session) },
		},
		{
			name:    "ListPanesInSession",
			command: "list-panes",
			want:    coordTargetForm,
			invoke:  func(c *tmux.Client) { _, _ = c.ListPanesInSession(session) },
		},
		{
			name:    "ListWindowsAndPanesInSession",
			command: "list-panes",
			want:    coordTargetForm,
			invoke:  func(c *tmux.Client) { _, _ = c.ListWindowsAndPanesInSession(session) },
		},
		{
			name:    "ListPanes",
			command: "list-panes",
			want:    coordTargetForm,
			invoke:  func(c *tmux.Client) { _, _ = c.ListPanes(session) },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := &MockCommander{}
			tc.invoke(tmux.NewClient(mock))

			if len(mock.Calls) != 1 {
				t.Fatalf("composed %d tmux calls, want exactly 1: %q", len(mock.Calls), mock.Calls)
			}
			call := mock.Calls[0]
			if call[0] != tc.command {
				t.Errorf("ran tmux %q, want %q", call[0], tc.command)
			}
			if got := targetArgOf(t, call); got != tc.want {
				t.Errorf("-t argument = %q, want %q", got, tc.want)
			}
		})
	}
}
