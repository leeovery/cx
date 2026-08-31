//go:build integration

package restore_test

import (
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// Pinning the skeleton target must not cost the ordinary restore: a session
// whose name is the prefix of a live one still reconstructs its own windows and
// panes, and the sibling gains none of them.
func TestSessionRestorer_MultiWindowSessionWithAPrefixSiblingLive(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	const (
		restored = "sib"
		sibling  = "sib-2"
	)

	binDir := restoretest.BuildPortalBinaryDir(t)
	portaltest.IsolateStateForTest(t)

	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	ts := tmuxtest.New(t, "ptl-3-7-sib-")
	client := ts.Client()
	cwd := t.TempDir()
	ts.Run(t, "new-session", "-d", "-s", sibling, "-c", cwd, "sleep", "infinity")
	ts.WaitForSession(t, sibling, 2*time.Second)

	r := &restore.SessionRestorer{
		Client:   client,
		StateDir: stateDir,
		Logger:   restoretest.OpenTestLogger(t, stateDir),
		Exe:      restoretest.StagedHydrateExe(t, binDir),
	}
	sess := newSession(restored, nil,
		newWindow(0, "main",
			newPane(0, cwd, "scrollback/sib__0.0.bin"),
			newPane(1, cwd, "scrollback/sib__0.1.bin"),
		),
		newWindow(1, "logs",
			newPane(0, cwd, "scrollback/sib__1.0.bin"),
		),
	)

	if _, err := r.Restore(sess); err != nil {
		t.Fatalf("Restore(%q): %v", restored, err)
	}

	if got := livePaneCoords(t, ts, restored); got != "0:0 0:1 1:0" {
		t.Errorf("restored %q panes = %q, want \"0:0 0:1 1:0\"", restored, got)
	}
	if got := livePaneCoords(t, ts, sibling); got != "0:0" {
		t.Errorf("live %q panes = %q, want the single pane it started with", sibling, got)
	}
}

func livePaneCoords(t *testing.T, ts *tmuxtest.Socket, session string) string {
	t.Helper()
	out := ts.Run(t, "list-panes", "-s", "-t", tmux.ExactCoordTarget(session),
		"-F", "#{window_index}:#{pane_index}")
	return strings.Join(strings.Fields(out), " ")
}
