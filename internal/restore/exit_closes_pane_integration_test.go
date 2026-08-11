//go:build integration

package restore_test

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const exitClosesPaneBudget = 2 * time.Second

func TestExitClosesRestoredPane_NoHook(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	stateDir, ts, sessionName := setupExitClosesPane(t, "")

	driveAndAwaitMarkerClear(t, ts.Client(), stateDir, sessionName)

	target := tmux.PaneTarget(sessionName, 0, 0)
	if err := ts.Client().SendKeys(target, "exit"); err != nil {
		t.Fatalf("SendKeys exit: %v", err)
	}

	awaitPaneGone(t, ts, sessionName, 0, 0, exitClosesPaneBudget)
}

func TestExitClosesRestoredPane_WithHook(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	sentinel := filepath.Join(t.TempDir(), "hook-fired")
	hookCmd := fmt.Sprintf("echo hook-fired > %s", sentinel)

	stateDir, ts, sessionName := setupExitClosesPane(t, hookCmd)

	driveAndAwaitMarkerClear(t, ts.Client(), stateDir, sessionName)

	restoretest.WaitForFileExists(t, sentinel, exitClosesPaneBudget, 50*time.Millisecond)

	target := tmux.PaneTarget(sessionName, 0, 0)
	if err := ts.Client().SendKeys(target, "exit"); err != nil {
		t.Fatalf("SendKeys exit: %v", err)
	}

	awaitPaneGone(t, ts, sessionName, 0, 0, exitClosesPaneBudget)
}

func TestNoParkedShWrapperPostRestore(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	stateDir, ts, sessionName := setupExitClosesPane(t, "")

	driveAndAwaitMarkerClear(t, ts.Client(), stateDir, sessionName)

	time.Sleep(exitClosesPaneBudget)

	paneSuffix := state.SanitizePaneKey(sessionName, 0, 0)
	pattern := "sh -c.*portal state hydrate.*" + paneSuffix

	cmd := exec.Command("pgrep", "-fl", pattern)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("AC5 violation: parked sh -c wrapper found around "+
			"portal state hydrate (Fix 3 → Side Effects: orphan sh "+
			"parent must be eliminated). pattern=%q\npgrep output:\n%s",
			pattern, out)
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		switch code := exitErr.ExitCode(); code {
		case 1: // pgrep found nothing: the pass condition.
		default:
			t.Fatalf("pgrep -fl %q: unexpected exit code %d "+
				"(want 0=match[fail] or 1=no-match[pass]); err=%v\n"+
				"output:\n%s", pattern, code, err, out)
		}
	} else {
		t.Fatalf("pgrep -fl %q: non-exec.ExitError failure (pgrep "+
			"binary missing or unrunnable?): %v\noutput:\n%s",
			pattern, err, out)
	}
}

func setupExitClosesPane(t *testing.T, hookCmd string) (string, *tmuxtest.Socket, string) {
	t.Helper()

	binDir := restoretest.BuildPortalBinaryDir(t)
	restoretest.PrependPATH(t, binDir)

	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	hooksPath := filepath.Join(t.TempDir(), "hooks.json")
	t.Setenv("PORTAL_HOOKS_FILE", hooksPath)

	sessionName := sanitizeSessionForTmux(t.Name())

	ts := tmuxtest.New(t, "ptl-exit-")
	client := ts.Client()

	ts.Run(t, "new-session", "-d", "-s", sessionName, "sleep", "infinity")
	ts.WaitForSession(t, sessionName, 2*time.Second)

	idx, err := state.CaptureStructure(client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}
	if len(idx.Sessions) != 1 || idx.Sessions[0].Name != sessionName {
		t.Fatalf("expected one captured session named %q; got %+v",
			sessionName, idx.Sessions)
	}

	if hookCmd != "" {
		store := hooks.NewStore(hooksPath)
		hookKey := tmux.PaneTarget(sessionName, 0, 0)
		if err := store.Set(hookKey, "on-resume", hookCmd, "cli"); err != nil {
			t.Fatalf("hooks.Set: %v", err)
		}
	}

	data, err := state.EncodeIndex(idx)
	if err != nil {
		t.Fatalf("EncodeIndex: %v", err)
	}
	if err := os.WriteFile(state.SessionsJSON(stateDir), data, 0o600); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}

	ts.KillServer()
	if _, err := ts.TryRun("list-sessions"); err == nil {
		t.Fatalf("list-sessions succeeded after kill-server; expected error")
	}
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	logger := restoretest.OpenTestLogger(t, stateDir)

	o := &restore.Orchestrator{
		Client:   client,
		StateDir: stateDir,
		Logger:   logger,
	}
	if err := restoreWithMarker(t, client, o); err != nil {
		t.Fatalf("restoreWithMarker: %v", err)
	}

	return stateDir, ts, sessionName
}

func driveAndAwaitMarkerClear(t *testing.T, client *tmux.Client, stateDir, sessionName string) {
	t.Helper()
	restoretest.DriveSignalHydrate(t, client, stateDir, []string{sessionName})
	restoretest.WaitForSkeletonMarkersCleared(t, client, 10*time.Second, 50*time.Millisecond)
}

func awaitPaneGone(t *testing.T, ts *tmuxtest.Socket, sessionName string, window, pane int, budget time.Duration) {
	t.Helper()
	deadline := time.Now().Add(budget)
	wantPane := fmt.Sprintf("%d:%d", window, pane)
	var lastOut string
	var lastErr error
	for time.Now().Before(deadline) {
		out, err := ts.TryRun("list-panes", "-s", "-t", sessionName,
			"-F", "#{window_index}:#{pane_index}")
		lastOut, lastErr = out, err
		if err != nil {
			return
		}
		if !strings.Contains(out, wantPane) {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("AC5 violation: pane %s:%s survived %s after `exit` "+
		"(spec AC5: pane closes on first invocation). last "+
		"list-panes output=%q err=%v",
		sessionName, wantPane, budget, lastOut, lastErr)
}

// tmux rejects ':' outright and '.' at the start of a session name.
func sanitizeSessionForTmux(name string) string {
	replacer := strings.NewReplacer(
		":", "-",
		".", "-",
		"/", "-",
		" ", "-",
		"\t", "-",
	)
	out := replacer.Replace(name)
	if out == "" {
		out = "exit-test"
	}
	return out
}
