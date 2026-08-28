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

func TestRenameRebootHook_DurableAcrossRepeatedReboots(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

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

	ts := tmuxtest.New(t, "ptl-3-6-dur-")
	client := ts.Client()

	cwd := t.TempDir()
	ts.Run(t, "new-session", "-d", "-s", renameOldName, "-c", cwd, "sleep", "infinity")
	ts.WaitForSession(t, renameOldName, 2*time.Second)

	ts.StampPaneToken(t, tmux.PaneTarget(renameOldName, 0, 0), renamePaneToken)

	ts.Run(t, "rename-session", "-t", renameOldName, renameNewName)
	if _, err := ts.TryRun("has-session", "-t", "="+renameNewName); err != nil {
		t.Fatalf("session %q not live after rename: %v", renameNewName, err)
	}

	idx := captureAndPersist(t, client, stateDir, renameNewName, renamePaneToken)

	if err := rebootAndHydrate(t, ts, client, stateDir); err != nil {
		t.Fatalf("cycle 1 rebootAndHydrate: %v", err)
	}
	assertHookFireCount(t, hookFireFile, 1)

	verifyHookKeyed(t, hooksPath, stableKey)

	// The same saved snapshot drives the second cycle: the token rides
	// sessions.json, so a reboot resolves the hook from saved state alone.
	persistIndex(t, idx, stateDir)
	seedScrollback(t, stateDir, renameNewName)

	if err := rebootAndHydrate(t, ts, client, stateDir); err != nil {
		t.Fatalf("cycle 2 rebootAndHydrate: %v", err)
	}

	t.Run("it fires the resume hook again on a second reboot cycle", func(t *testing.T) {
		assertHookFireCount(t, hookFireFile, 2)
	})
}

func captureAndPersist(t *testing.T, client *tmux.Client, stateDir, name, wantPaneToken string) state.Index {
	t.Helper()

	idx, err := state.CaptureStructure(client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}

	sess := findCapturedSession(t, idx, name)
	if got := capturedPaneToken(t, sess); got != wantPaneToken {
		t.Fatalf("captured session %q pane token = %q; want %q (token must persist under the post-rename name)",
			name, got, wantPaneToken)
	}

	seedScrollback(t, stateDir, name)
	persistIndex(t, idx, stateDir)
	return idx
}

func rebootAndHydrate(t *testing.T, ts *tmuxtest.Socket, client *tmux.Client, stateDir string) error {
	t.Helper()

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
		return err
	}

	restoredPanes := ts.Run(t, "list-panes", "-s", "-t", renameNewName,
		"-F", "#{window_index}:#{pane_index}")
	if !strings.Contains(restoredPanes, "0:0") {
		t.Fatalf("restored session %q missing live pane 0:0; got %q", renameNewName, restoredPanes)
	}

	restoretest.DriveSignalHydrate(t, client, stateDir, []string{renameNewName})
	restoretest.WaitForSkeletonMarkersCleared(t, client, 10*time.Second, 50*time.Millisecond)
	return nil
}
