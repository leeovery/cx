package tmux_test

import (
	"errors"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// The gone session and the live prefix sibling every case in this file works
// against. "goneSession" is never created: a session renamed away leaves tmux
// in exactly this state.
const (
	goneSession   = "sib"
	prefixSibling = "sib-2"
)

// The shape seedPrefixSiblingServer builds: two windows of two panes each, with
// the second window current. A single-window sibling would let a form that
// resolves only the current window pass as if it had resolved the whole
// session.
var (
	siblingAllCoords = []tmux.PaneCoord{
		{Window: 0, Pane: 0}, {Window: 0, Pane: 1},
		{Window: 1, Pane: 0}, {Window: 1, Pane: 1},
	}
	// list-panes without -s addresses one window, so this is the whole session
	// minus its first window.
	siblingCurrentWindowPaneKeys = []string{prefixSibling + ":1.0", prefixSibling + ":1.1"}
)

// seedPrefixSiblingServer starts an isolated server holding prefixSibling and
// nothing else, shaped to siblingAllCoords, and returns the socket, its client,
// and the dir every pane sits in as tmux reports it (macOS resolves the temp
// dir through /private).
func seedPrefixSiblingServer(t *testing.T) (*tmuxtest.Socket, *tmux.Client, string) {
	t.Helper()
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-exacttgt-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	if err := client.NewSession(prefixSibling, dir, ""); err != nil {
		t.Fatalf("NewSession(%q): %v", prefixSibling, err)
	}
	ts.WaitForSession(t, prefixSibling, 2*time.Second)

	// -c pins every pane to dir, so the ActivePaneCurrentPath expectation holds
	// whichever pane the splits leave active. new-window runs last-but-one, so
	// window 1 is the session's current window.
	ts.Run(t, "split-window", "-t", "="+prefixSibling+":0.0", "-c", dir)
	ts.Run(t, "new-window", "-t", "="+prefixSibling+":", "-c", dir)
	ts.Run(t, "split-window", "-t", "="+prefixSibling+":1.0", "-c", dir)
	return ts, client, dir
}

func siblingEnvironment(t *testing.T, ts *tmuxtest.Socket) string {
	t.Helper()
	return ts.Run(t, "show-environment", "-t", prefixSibling)
}

func siblingOptions(t *testing.T, ts *tmuxtest.Socket) string {
	t.Helper()
	return ts.Run(t, "show-options", "-t", prefixSibling)
}

// siblingWindowLayout reads the layout string of the sibling's first window,
// the window every SelectLayout case here writes to.
func siblingWindowLayout(t *testing.T, ts *tmuxtest.Socket) string {
	t.Helper()
	out := ts.Run(t, "display-message", "-p", "-t", "="+prefixSibling+":0", "#{window_layout}")
	return strings.TrimSpace(out)
}

// TestSessionTargets_GoneSessionDoesNotReachPrefixSibling is the wrong-session
// half: every per-session read and write must miss rather than silently land on
// the live sibling.
func TestSessionTargets_GoneSessionDoesNotReachPrefixSibling(t *testing.T) {
	t.Run("ActivePaneCurrentPath", func(t *testing.T) {
		_, client, dir := seedPrefixSiblingServer(t)

		got, err := client.ActivePaneCurrentPath(goneSession)
		if got == dir {
			t.Fatalf("ActivePaneCurrentPath(%q) returned %q — the live %q session's dir", goneSession, got, prefixSibling)
		}
		// tmux answers an unmatched display-message target with an empty
		// expansion at exit 0, so this site reports a miss as an empty path
		// rather than an error.
		if err != nil || got != "" {
			t.Errorf("ActivePaneCurrentPath(%q) = %q, %v; want an empty path and no error", goneSession, got, err)
		}
	})

	t.Run("ListPanesInSession", func(t *testing.T) {
		_, client, _ := seedPrefixSiblingServer(t)

		coords, err := client.ListPanesInSession(goneSession)
		if err == nil {
			t.Fatalf("ListPanesInSession(%q) succeeded with %v, want a tmux failure", goneSession, coords)
		}
	})

	t.Run("ListWindowsAndPanesInSession", func(t *testing.T) {
		_, client, _ := seedPrefixSiblingServer(t)

		groups, err := client.ListWindowsAndPanesInSession(goneSession)
		if err == nil {
			t.Fatalf("ListWindowsAndPanesInSession(%q) succeeded with %v, want a tmux failure", goneSession, groups)
		}
	})

	t.Run("ListPanes", func(t *testing.T) {
		_, client, _ := seedPrefixSiblingServer(t)

		keys, err := client.ListPanes(goneSession)
		if err == nil {
			t.Fatalf("ListPanes(%q) succeeded with %v, want a tmux failure", goneSession, keys)
		}
	})

	t.Run("ShowEnvironment", func(t *testing.T) {
		ts, client, _ := seedPrefixSiblingServer(t)
		ts.Run(t, "set-environment", "-t", prefixSibling, "PORTAL_SIB", "sibling-only")

		out, err := client.ShowEnvironment(goneSession)
		// The sentinel against real tmux, not a hand-supplied stderr: capture
		// discriminates on it to tell natural session churn from anomaly, so
		// the wire form has to keep producing it.
		if !errors.Is(err, tmux.ErrNoSuchSession) {
			t.Fatalf("ShowEnvironment(%q) = %q, %v; want an error matching ErrNoSuchSession", goneSession, out, err)
		}
		if strings.Contains(out, "sibling-only") {
			t.Errorf("ShowEnvironment(%q) read the live %q session's environment", goneSession, prefixSibling)
		}
	})

	t.Run("SetSessionEnvironment", func(t *testing.T) {
		ts, client, _ := seedPrefixSiblingServer(t)

		if err := client.SetSessionEnvironment(goneSession, "PORTAL_LEAK", "leaked"); err == nil {
			t.Fatalf("SetSessionEnvironment(%q) succeeded, want a tmux failure", goneSession)
		}
		if got := siblingEnvironment(t, ts); strings.Contains(got, "PORTAL_LEAK") {
			t.Errorf("the write landed on the live %q session:\n%s", prefixSibling, got)
		}
	})

	t.Run("SetSessionOption", func(t *testing.T) {
		ts, client, _ := seedPrefixSiblingServer(t)

		if err := client.SetSessionOption(goneSession, "@portal-leak", "leaked"); err == nil {
			t.Fatalf("SetSessionOption(%q) succeeded, want a tmux failure", goneSession)
		}
		if got := siblingOptions(t, ts); strings.Contains(got, "@portal-leak") {
			t.Errorf("the write landed on the live %q session:\n%s", prefixSibling, got)
		}
	})

	t.Run("SelectLayout", func(t *testing.T) {
		ts, client, _ := seedPrefixSiblingServer(t)
		before := siblingWindowLayout(t, ts)

		// Errorf, not Fatalf: a call that wrongly succeeds must still reach the
		// layout comparison, which is where the wrong-session write shows up.
		if err := client.SelectLayout(goneSession, 0, "even-horizontal"); err == nil {
			t.Errorf("SelectLayout(%q) succeeded, want a tmux failure", goneSession)
		}
		if got := siblingWindowLayout(t, ts); got != before {
			t.Errorf("the live %q session's layout changed from %q to %q", prefixSibling, before, got)
		}
	})
}

// TestSessionTargets_LiveSessionStillResolves is the half that keeps the exact
// forms honest: pinning the session must not cost the ordinary case, and a form
// tmux cannot resolve at all would read as a permanent miss rather than a
// wrong-session hit.
func TestSessionTargets_LiveSessionStillResolves(t *testing.T) {
	t.Run("ActivePaneCurrentPath", func(t *testing.T) {
		_, client, dir := seedPrefixSiblingServer(t)

		got, err := client.ActivePaneCurrentPath(prefixSibling)
		if err != nil {
			t.Fatalf("ActivePaneCurrentPath(%q): %v", prefixSibling, err)
		}
		if got != dir {
			t.Errorf("ActivePaneCurrentPath(%q) = %q, want %q", prefixSibling, got, dir)
		}
	})

	t.Run("ListPanesInSession", func(t *testing.T) {
		_, client, _ := seedPrefixSiblingServer(t)

		coords, err := client.ListPanesInSession(prefixSibling)
		if err != nil {
			t.Fatalf("ListPanesInSession(%q): %v", prefixSibling, err)
		}
		if !slices.Equal(coords, siblingAllCoords) {
			t.Errorf("ListPanesInSession(%q) = %v, want %v — every pane of every window", prefixSibling, coords, siblingAllCoords)
		}
	})

	t.Run("ListWindowsAndPanesInSession", func(t *testing.T) {
		_, client, _ := seedPrefixSiblingServer(t)

		groups, err := client.ListWindowsAndPanesInSession(prefixSibling)
		if err != nil {
			t.Fatalf("ListWindowsAndPanesInSession(%q): %v", prefixSibling, err)
		}
		if len(groups) != 2 {
			t.Fatalf("ListWindowsAndPanesInSession(%q) returned %d windows, want 2: %+v", prefixSibling, len(groups), groups)
		}
		for i, group := range groups {
			if group.WindowIndex != i || !slices.Equal(group.PaneIndices, []int{0, 1}) {
				t.Errorf("window %d = %+v, want index %d holding panes [0 1]", i, group, i)
			}
		}
	})

	t.Run("ListPanes", func(t *testing.T) {
		_, client, _ := seedPrefixSiblingServer(t)

		keys, err := client.ListPanes(prefixSibling)
		if err != nil {
			t.Fatalf("ListPanes(%q): %v", prefixSibling, err)
		}
		if !slices.Equal(keys, siblingCurrentWindowPaneKeys) {
			t.Errorf("ListPanes(%q) = %v, want %v — the current window's panes, not the session's", prefixSibling, keys, siblingCurrentWindowPaneKeys)
		}
	})

	t.Run("ShowEnvironment", func(t *testing.T) {
		ts, client, _ := seedPrefixSiblingServer(t)
		ts.Run(t, "set-environment", "-t", prefixSibling, "PORTAL_SIB", "sibling-only")

		out, err := client.ShowEnvironment(prefixSibling)
		if err != nil {
			t.Fatalf("ShowEnvironment(%q): %v", prefixSibling, err)
		}
		if !strings.Contains(out, "PORTAL_SIB=sibling-only") {
			t.Errorf("ShowEnvironment(%q) = %q, want it to carry PORTAL_SIB=sibling-only", prefixSibling, out)
		}
	})

	t.Run("SetSessionEnvironment", func(t *testing.T) {
		ts, client, _ := seedPrefixSiblingServer(t)

		if err := client.SetSessionEnvironment(prefixSibling, "PORTAL_SET", "written"); err != nil {
			t.Fatalf("SetSessionEnvironment(%q): %v", prefixSibling, err)
		}
		if got := siblingEnvironment(t, ts); !strings.Contains(got, "PORTAL_SET=written") {
			t.Errorf("the write did not land on %q:\n%s", prefixSibling, got)
		}
	})

	t.Run("SetSessionOption", func(t *testing.T) {
		ts, client, _ := seedPrefixSiblingServer(t)

		if err := client.SetSessionOption(prefixSibling, "@portal-set", "written"); err != nil {
			t.Fatalf("SetSessionOption(%q): %v", prefixSibling, err)
		}
		if got := siblingOptions(t, ts); !strings.Contains(got, "@portal-set written") {
			t.Errorf("the write did not land on %q:\n%s", prefixSibling, got)
		}
	})

	t.Run("SelectLayout", func(t *testing.T) {
		ts, client, _ := seedPrefixSiblingServer(t)
		ts.Run(t, "select-layout", "-t", "="+prefixSibling+":0", "even-vertical")
		before := siblingWindowLayout(t, ts)

		if err := client.SelectLayout(prefixSibling, 0, "even-horizontal"); err != nil {
			t.Fatalf("SelectLayout(%q): %v", prefixSibling, err)
		}
		if got := siblingWindowLayout(t, ts); got == before {
			t.Errorf("the layout of %q:0 is still %q — the write did not land", prefixSibling, got)
		}
	})
}
