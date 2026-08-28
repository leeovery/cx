//go:build integration

package restore_test

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestRenameRebootHook_ExternalRename(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	runRenameRebootFire(t, func(t *testing.T, ts *tmuxtest.Socket, _ *tmux.Client) {
		t.Helper()
		ts.Run(t, "rename-session", "-t", renameOldName, renameNewName)
	})
}

func TestRenameRebootHook_RenameSessionEquivalent(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	runRenameRebootFire(t, func(t *testing.T, _ *tmuxtest.Socket, client *tmux.Client) {
		t.Helper()
		if err := client.RenameSession(renameOldName, renameNewName); err != nil {
			t.Fatalf("RenameSession(%q, %q): %v", renameOldName, renameNewName, err)
		}
	})
}

func TestRenameRebootHook_PaneProcessKeptRunning(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	runRenameRebootFire(t, func(t *testing.T, ts *tmuxtest.Socket, client *tmux.Client) {
		t.Helper()
		pidBefore := strings.TrimSpace(ts.Run(t, "display-message", "-p",
			"-t", tmux.PaneTarget(renameOldName, 0, 0), "#{pane_pid}"))

		if err := client.RenameSession(renameOldName, renameNewName); err != nil {
			t.Fatalf("RenameSession(%q, %q): %v", renameOldName, renameNewName, err)
		}

		pidAfter := strings.TrimSpace(ts.Run(t, "display-message", "-p",
			"-t", tmux.PaneTarget(renameNewName, 0, 0), "#{pane_pid}"))
		if pidBefore == "" || pidAfter == "" {
			t.Fatalf("pane_pid read empty (before=%q after=%q); pane addressing broke", pidBefore, pidAfter)
		}
		if pidBefore != pidAfter {
			t.Fatalf("pane process restarted across rename (pid %q → %q); "+
				"the bug only bites when the process keeps running", pidBefore, pidAfter)
		}
	})
}

func runRenameRebootFire(t *testing.T, rename func(t *testing.T, ts *tmuxtest.Socket, client *tmux.Client)) {
	t.Helper()

	binDir := restoretest.BuildPortalBinaryDir(t)
	restoretest.PrependPATH(t, binDir)

	portaltest.IsolateStateForTest(t)

	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	hooksPath := filepath.Join(t.TempDir(), "hooks.json")
	t.Setenv("PORTAL_HOOKS_FILE", hooksPath)

	hookFireFile := filepath.Join(t.TempDir(), "hook-fired.txt")
	hookCmd := "echo HOOK_FIRED >> " + hookFireFile

	const stableKey = renamePaneToken
	store := hooks.NewStore(hooksPath)
	if err := store.Set(stableKey, "on-resume", hookCmd, "cli"); err != nil {
		t.Fatalf("hooks.Set: %v", err)
	}

	ts := tmuxtest.New(t, "ptl-3-5-")
	client := ts.Client()

	cwd := t.TempDir()
	ts.Run(t, "new-session", "-d", "-s", renameOldName, "-c", cwd, "sleep", "infinity")
	ts.WaitForSession(t, renameOldName, 2*time.Second)

	ts.StampPaneToken(t, tmux.PaneTarget(renameOldName, 0, 0), renamePaneToken)

	rename(t, ts, client)

	if _, err := ts.TryRun("has-session", "-t", "="+renameNewName); err != nil {
		t.Fatalf("session %q not live after rename: %v", renameNewName, err)
	}
	if liveToken := readPaneToken(t, ts, renameNewName); liveToken != renamePaneToken {
		t.Fatalf("pane token after rename = %q; want %q (must be rename-immune)", liveToken, renamePaneToken)
	}

	idx, err := state.CaptureStructure(client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}

	sess := restoretest.FindCapturedSession(t, idx, renameNewName)
	if got := capturedPaneToken(t, sess); got != renamePaneToken {
		t.Fatalf("captured session %q pane token = %q; want %q (token must persist under the post-rename name)",
			renameNewName, got, renamePaneToken)
	}
	verifyHookKeyed(t, hooksPath, stableKey)

	restoretest.SeedScrollback(t, stateDir, renameNewName, 0, 0, []byte(rebootScrollback))
	persistIndex(t, idx, stateDir)

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
	if err := restoretest.RestoreWithMarker(t, client, o); err != nil {
		t.Fatalf("restoreWithMarker: %v", err)
	}

	restoredPanes := ts.Run(t, "list-panes", "-s", "-t", renameNewName,
		"-F", "#{window_index}:#{pane_index}")
	if !strings.Contains(restoredPanes, "0:0") {
		t.Fatalf("restored session %q missing live pane 0:0; got %q", renameNewName, restoredPanes)
	}

	restoretest.DriveSignalHydrate(t, client, stateDir, []string{renameNewName})

	restoretest.WaitForSkeletonMarkersCleared(t, client, 10*time.Second, 50*time.Millisecond)

	assertHookFireCount(t, hookFireFile, 1)
}
