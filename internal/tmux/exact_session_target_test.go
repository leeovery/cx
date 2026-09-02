package tmux_test

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/commandertest"
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

// exactTargetSession is the session name every route in perSessionRoutes is
// invoked against; the two target forms above are its exact renderings.
const exactTargetSession = "exact"

// perSessionRoute is one operation that composes an exact tmux target for a
// single session.
type perSessionRoute struct {
	name    string
	command string
	want    string
	invoke  func(*tmux.Client)
}

// perSessionRoutes is the route set shared by both guards over the composition
// rule: the mock-level target-form table below and the real-tmux prefix-sibling
// routes. A route added here is unguarded until both enumerate it, so the two
// cannot drift apart. It is a maintained set, not the client's whole
// exact-target surface.
var perSessionRoutes = []perSessionRoute{
	{
		name:    "ShowEnvironment",
		command: "show-environment",
		want:    sessionTargetForm,
		invoke:  func(c *tmux.Client) { _, _ = c.ShowEnvironment(exactTargetSession) },
	},
	{
		name:    "SetSessionEnvironment",
		command: "set-environment",
		want:    sessionTargetForm,
		invoke:  func(c *tmux.Client) { _ = c.SetSessionEnvironment(exactTargetSession, "K", "v") },
	},
	{
		name:    "SetSessionOption",
		command: "set-option",
		want:    coordTargetForm,
		invoke:  func(c *tmux.Client) { _ = c.SetSessionOption(exactTargetSession, "@k", "v") },
	},
	{
		name:    "ActivePaneCurrentPath",
		command: "display-message",
		want:    coordTargetForm,
		invoke:  func(c *tmux.Client) { _, _ = c.ActivePaneCurrentPath(exactTargetSession) },
	},
	{
		name:    "ListPanesInSession",
		command: "list-panes",
		want:    coordTargetForm,
		invoke:  func(c *tmux.Client) { _, _ = c.ListPanesInSession(exactTargetSession) },
	},
	{
		name:    "ListWindowsAndPanesInSession",
		command: "list-panes",
		want:    coordTargetForm,
		invoke:  func(c *tmux.Client) { _, _ = c.ListWindowsAndPanesInSession(exactTargetSession) },
	},
	{
		name:    "SaverPaneID",
		command: "list-panes",
		want:    coordTargetForm,
		invoke:  func(c *tmux.Client) { _, _ = c.SaverPaneID(exactTargetSession) },
	},
	{
		name:    "SaverPanePIDOrAbsent",
		command: "list-panes",
		want:    coordTargetForm,
		invoke:  func(c *tmux.Client) { _, _, _ = tmux.SaverPanePIDOrAbsent(c, exactTargetSession) },
	},
}

func targetArgOf(t *testing.T, call []string) string {
	t.Helper()
	i := slices.Index(call, "-t")
	if i < 0 || i+1 >= len(call) {
		t.Fatalf("call %q composes no -t argument", call)
	}
	return call[i+1]
}

func TestSessionTargetsAreComposedExactly(t *testing.T) {
	for _, route := range perSessionRoutes {
		t.Run(route.name, func(t *testing.T) {
			mock := commandertest.Quiet()
			route.invoke(tmux.NewClient(mock))

			if len(mock.Calls()) != 1 {
				t.Fatalf("composed %d tmux calls, want exactly 1: %q", len(mock.Calls()), mock.Calls())
			}
			call := mock.Calls()[0]
			if call[0] != route.command {
				t.Errorf("ran tmux %q, want %q", call[0], route.command)
			}
			if got := targetArgOf(t, call); got != route.want {
				t.Errorf("-t argument = %q, want %q", got, route.want)
			}
		})
	}
}
