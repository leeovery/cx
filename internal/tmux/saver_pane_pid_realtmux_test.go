package tmux_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// saverPrefixSibling is the shape a renamed-away or version-upgraded saver
// leaves behind: a live session whose name extends PortalSaverName, with
// PortalSaverName itself absent.
const saverPrefixSibling = tmux.PortalSaverName + "-old"

// seedSaverServer starts an isolated server holding the named sessions and
// returns its socket and client.
func seedSaverServer(t *testing.T, sessions ...string) (*tmuxtest.Socket, *tmux.Client) {
	t.Helper()
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-saverpid-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	for _, name := range sessions {
		if err := client.NewSession(name, dir, ""); err != nil {
			t.Fatalf("NewSession(%q): %v", name, err)
		}
		waitForExactSession(t, ts, name)
	}
	return ts, client
}

// waitForExactSession covers the settle window a fresh session has been
// observed to need before it answers. It must pin the name exactly: this file
// stages a live prefix sibling, and the fuzzy form tmux applies to a bare -t
// makes the sibling answer for a session that does not exist yet.
func waitForExactSession(t *testing.T, ts *tmuxtest.Socket, name string) {
	t.Helper()
	appeared := tmuxtest.PollUntil(t, 2*time.Second, 20*time.Millisecond, func() bool {
		_, err := ts.TryRun("has-session", "-t", "="+name)
		return err == nil
	})
	if !appeared {
		t.Fatalf("session %q did not appear within 2s", name)
	}
}

// livePanePID reads the session's own pane pid straight from tmux, so the
// assertions compare against the wire truth rather than a second Portal read.
func livePanePID(t *testing.T, ts *tmuxtest.Socket, session string) int {
	t.Helper()
	out := strings.TrimSpace(ts.Run(t, "list-panes", "-t", "="+session+":", "-F", "#{pane_pid}"))
	pid, err := strconv.Atoi(strings.SplitN(out, "\n", 2)[0])
	if err != nil {
		t.Fatalf("parse pane pid %q: %v", out, err)
	}
	return pid
}

func TestSaverPanePID_RealTmux(t *testing.T) {
	t.Run("it does not resolve a prefix-sibling saver session", func(t *testing.T) {
		ts, client := seedSaverServer(t, saverPrefixSibling)
		siblingPID := livePanePID(t, ts, saverPrefixSibling)

		pid, err := tmux.SaverPanePID(client, tmux.PortalSaverName)
		if pid == siblingPID {
			t.Fatalf("SaverPanePID(%q) returned %d — the live %q session's pane pid",
				tmux.PortalSaverName, pid, saverPrefixSibling)
		}
		if err == nil {
			t.Fatalf("SaverPanePID(%q) = %d, nil; want a tmux failure", tmux.PortalSaverName, pid)
		}
	})

	t.Run("it reports absence when only a prefix sibling is live", func(t *testing.T) {
		ts, client := seedSaverServer(t, saverPrefixSibling)
		siblingPID := livePanePID(t, ts, saverPrefixSibling)

		pid, present, err := tmux.SaverPanePIDOrAbsent(client, tmux.PortalSaverName)
		if pid == siblingPID {
			t.Fatalf("SaverPanePIDOrAbsent(%q) returned %d — the live %q session's pane pid",
				tmux.PortalSaverName, pid, saverPrefixSibling)
		}
		if pid != 0 || present || err != nil {
			t.Errorf("SaverPanePIDOrAbsent(%q) = %d, %t, %v; want 0, false, nil",
				tmux.PortalSaverName, pid, present, err)
		}
	})

	t.Run("it returns the pane pid of a live _portal-saver", func(t *testing.T) {
		ts, client := seedSaverServer(t, saverPrefixSibling, tmux.PortalSaverName)
		want := livePanePID(t, ts, tmux.PortalSaverName)

		pid, present, err := tmux.SaverPanePIDOrAbsent(client, tmux.PortalSaverName)
		if err != nil {
			t.Fatalf("SaverPanePIDOrAbsent(%q): %v", tmux.PortalSaverName, err)
		}
		if pid != want || !present {
			t.Errorf("SaverPanePIDOrAbsent(%q) = %d, %t; want %d, true",
				tmux.PortalSaverName, pid, present, want)
		}
	})

	t.Run("it collapses a missing session to present=false", func(t *testing.T) {
		_, client := seedSaverServer(t, "unrelated")

		pid, present, err := tmux.SaverPanePIDOrAbsent(client, tmux.PortalSaverName)
		if pid != 0 || present || err != nil {
			t.Errorf("SaverPanePIDOrAbsent(%q) = %d, %t, %v; want 0, false, nil",
				tmux.PortalSaverName, pid, present, err)
		}
	})
}
