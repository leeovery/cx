//go:build integration

package restore_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const legacyName = "legacy-proj"

func TestMultiPaneLegacy_PerPaneHookRouting(t *testing.T) {
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

	sideEffectDir := t.TempDir()
	pane0File := filepath.Join(sideEffectDir, "hook-pane0.txt")
	pane1File := filepath.Join(sideEffectDir, "hook-pane1.txt")
	const (
		pane0Marker = "PANE0_HOOK_FIRED"
		pane1Marker = "PANE1_HOOK_FIRED"
	)
	pane0Cmd := "echo " + pane0Marker + " >> " + pane0File
	pane1Cmd := "echo " + pane1Marker + " >> " + pane1File

	pane0Key := tmux.HookKey(renamePortalID, renameOldName, 0, 0)
	pane1Key := tmux.HookKey(renamePortalID, renameOldName, 0, 1)
	if pane0Key != renamePortalID+":0.0" {
		t.Fatalf("pane 0 hook key = %q; want %q", pane0Key, renamePortalID+":0.0")
	}
	if pane1Key != renamePortalID+":0.1" {
		t.Fatalf("pane 1 hook key = %q; want %q", pane1Key, renamePortalID+":0.1")
	}
	if pane0Key == pane1Key {
		t.Fatalf("multi-pane hook keys are not distinct: both %q", pane0Key)
	}
	store := hooks.NewStore(hooksPath)
	if err := store.Set(pane0Key, "on-resume", pane0Cmd, "cli"); err != nil {
		t.Fatalf("hooks.Set pane 0: %v", err)
	}
	if err := store.Set(pane1Key, "on-resume", pane1Cmd, "cli"); err != nil {
		t.Fatalf("hooks.Set pane 1: %v", err)
	}
	verifyHookKeyed(t, hooksPath, pane0Key)
	verifyHookKeyed(t, hooksPath, pane1Key)

	ts := tmuxtest.New(t, "ptl-3-7-mp-")
	client := ts.Client()

	cwd := t.TempDir()
	ts.Run(t, "new-session", "-d", "-s", renameOldName, "-c", cwd, "sleep", "infinity")
	ts.WaitForSession(t, renameOldName, 2*time.Second)
	ts.Run(t, "split-window", "-t", tmux.PaneTarget(renameOldName, 0, 0), "-c", cwd, "sleep", "infinity")

	panesOut := strings.TrimSpace(ts.Run(t, "list-panes", "-s", "-t", renameOldName,
		"-F", "#{window_index}:#{pane_index}"))
	if !strings.Contains(panesOut, "0:0") || !strings.Contains(panesOut, "0:1") {
		t.Fatalf("expected panes 0:0 and 0:1 pre-rename; got %q", panesOut)
	}

	if err := client.SetSessionOption(renameOldName, session.PortalIDOption, renamePortalID); err != nil {
		t.Fatalf("SetSessionOption %s=%s: %v", session.PortalIDOption, renamePortalID, err)
	}

	ts.Run(t, "rename-session", "-t", renameOldName, renameNewName)
	if _, err := ts.TryRun("has-session", "-t", "="+renameNewName); err != nil {
		t.Fatalf("session %q not live after rename: %v", renameNewName, err)
	}

	idx, err := state.CaptureStructure(client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}
	sess := findCapturedSession(t, idx, renameNewName)
	if sess.PortalID != renamePortalID {
		t.Fatalf("captured session %q PortalID = %q; want %q", renameNewName, sess.PortalID, renamePortalID)
	}
	if len(sess.Windows) != 1 || len(sess.Windows[0].Panes) != 2 {
		t.Fatalf("captured session %q topology = %d window(s) / %v panes; want 1 window / 2 panes",
			renameNewName, len(sess.Windows), paneIndices(sess))
	}
	verifyHookKeyed(t, hooksPath, pane0Key)
	verifyHookKeyed(t, hooksPath, pane1Key)

	seedPaneScrollback(t, stateDir, renameNewName, 0, 0)
	seedPaneScrollback(t, stateDir, renameNewName, 0, 1)

	persistIndex(t, idx, stateDir)

	ts.KillServer()
	if _, err := ts.TryRun("list-sessions"); err == nil {
		t.Fatalf("list-sessions succeeded after kill-server; expected error")
	}
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	logger := restoretest.OpenTestLogger(t, stateDir)
	o := &restore.Orchestrator{Client: client, StateDir: stateDir, Logger: logger}
	if err := restoreWithMarker(t, client, o); err != nil {
		t.Fatalf("restoreWithMarker: %v", err)
	}

	restoredPanes := strings.TrimSpace(ts.Run(t, "list-panes", "-s", "-t", renameNewName,
		"-F", "#{window_index}:#{pane_index}"))
	if !strings.Contains(restoredPanes, "0:0") || !strings.Contains(restoredPanes, "0:1") {
		t.Fatalf("restored session %q missing panes 0:0/0:1; got %q", renameNewName, restoredPanes)
	}

	restoretest.DriveSignalHydrate(t, client, stateDir, []string{renameNewName})
	restoretest.WaitForSkeletonMarkersCleared(t, client, 10*time.Second, 50*time.Millisecond)

	assertMarkerFiredOnce(t, pane0File, pane0Marker)
	assertMarkerFiredOnce(t, pane1File, pane1Marker)
	assertMarkerAbsent(t, pane0File, pane1Marker)
	assertMarkerAbsent(t, pane1File, pane0Marker)
}

func TestMultiPaneLegacy_GracefulLegacyDegradation(t *testing.T) {
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
	const legacyMarker = "LEGACY_HOOK_FIRED"
	hookCmd := "echo " + legacyMarker + " >> " + hookFireFile

	legacyKey := tmux.HookKey("", legacyName, 0, 0)
	if legacyKey != legacyName+":0.0" {
		t.Fatalf("legacy hook key = %q; want %q (empty id must fall back to the name)",
			legacyKey, legacyName+":0.0")
	}
	store := hooks.NewStore(hooksPath)
	if err := store.Set(legacyKey, "on-resume", hookCmd, "cli"); err != nil {
		t.Fatalf("hooks.Set: %v", err)
	}
	verifyHookKeyed(t, hooksPath, legacyKey)

	ts := tmuxtest.New(t, "ptl-3-7-legacy-")
	client := ts.Client()

	cwd := t.TempDir()
	ts.Run(t, "new-session", "-d", "-s", legacyName, "-c", cwd, "sleep", "infinity")
	ts.WaitForSession(t, legacyName, 2*time.Second)

	if liveID := unsetOptionValue(t, ts, legacyName); liveID != "" {
		t.Fatalf("un-stamped session %q unexpectedly has @portal-id = %q", legacyName, liveID)
	}

	idx, err := state.CaptureStructure(client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}
	sess := findCapturedSession(t, idx, legacyName)

	t.Run("it degrades an un-stamped session to the name-based key end-to-end", func(t *testing.T) {
		if sess.PortalID != "" {
			t.Fatalf("captured un-stamped session %q PortalID = %q; want \"\" (name-fallback path)",
				legacyName, sess.PortalID)
		}
		bakedKey := tmux.HookKey(sess.PortalID, sess.Name, 0, 0)
		if bakedKey != legacyKey {
			t.Fatalf("baked --hook-key = %q; want name-based %q", bakedKey, legacyKey)
		}
	})

	seedPaneScrollback(t, stateDir, legacyName, 0, 0)
	persistIndex(t, idx, stateDir)

	ts.KillServer()
	if _, err := ts.TryRun("list-sessions"); err == nil {
		t.Fatalf("list-sessions succeeded after kill-server; expected error")
	}
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	logger := restoretest.OpenTestLogger(t, stateDir)
	o := &restore.Orchestrator{Client: client, StateDir: stateDir, Logger: logger}

	t.Run("it does not panic on an empty PortalID anywhere in the chain", func(t *testing.T) {
		if err := restoreWithMarker(t, client, o); err != nil {
			t.Fatalf("restoreWithMarker on un-stamped session: %v", err)
		}
	})

	if _, err := ts.TryRun("has-session", "-t", "="+legacyName); err != nil {
		t.Fatalf("un-stamped session %q not restored: %v", legacyName, err)
	}
	if restampedID := unsetOptionValue(t, ts, legacyName); restampedID != "" {
		t.Errorf("re-stamp was NOT skipped: restored un-stamped session %q now has @portal-id = %q",
			legacyName, restampedID)
	}

	restoretest.DriveSignalHydrate(t, client, stateDir, []string{legacyName})
	restoretest.WaitForSkeletonMarkersCleared(t, client, 10*time.Second, 50*time.Millisecond)

	assertHookFireCount(t, hookFireFile, 1)
}

func TestMultiPaneLegacy_UnstampedNoHookLandsOnBareShell(t *testing.T) {
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

	ts := tmuxtest.New(t, "ptl-3-7-bare-")
	client := ts.Client()

	cwd := t.TempDir()
	ts.Run(t, "new-session", "-d", "-s", legacyName, "-c", cwd, "sleep", "infinity")
	ts.WaitForSession(t, legacyName, 2*time.Second)

	idx, err := state.CaptureStructure(client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}
	sess := findCapturedSession(t, idx, legacyName)
	if sess.PortalID != "" {
		t.Fatalf("captured un-stamped session %q PortalID = %q; want \"\"", legacyName, sess.PortalID)
	}

	seedPaneScrollback(t, stateDir, legacyName, 0, 0)
	persistIndex(t, idx, stateDir)

	ts.KillServer()
	if _, err := ts.TryRun("list-sessions"); err == nil {
		t.Fatalf("list-sessions succeeded after kill-server; expected error")
	}
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	logger := restoretest.OpenTestLogger(t, stateDir)
	o := &restore.Orchestrator{Client: client, StateDir: stateDir, Logger: logger}
	if err := restoreWithMarker(t, client, o); err != nil {
		t.Fatalf("restoreWithMarker (no-hook clean miss): %v", err)
	}

	restoretest.DriveSignalHydrate(t, client, stateDir, []string{legacyName})
	restoretest.WaitForSkeletonMarkersCleared(t, client, 10*time.Second, 50*time.Millisecond)

	if _, err := ts.TryRun("has-session", "-t", "="+legacyName); err != nil {
		t.Fatalf("un-stamped no-hook session %q not restored: %v", legacyName, err)
	}
}

// An unset user-option makes `show-options -v` exit non-zero, so absent reads
// back as the empty string.
func unsetOptionValue(t *testing.T, ts *tmuxtest.Socket, name string) string {
	t.Helper()
	out, err := ts.TryRun("show-options", "-t", name, "-v", session.PortalIDOption)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func paneIndices(sess state.Session) [][]int {
	var out [][]int
	for _, w := range sess.Windows {
		var panes []int
		for _, p := range w.Panes {
			panes = append(panes, p.Index)
		}
		out = append(out, panes)
	}
	return out
}

func seedPaneScrollback(t *testing.T, stateDir, name string, window, pane int) {
	t.Helper()
	scrollbackKey := state.SanitizePaneKey(name, window, pane)
	scrollbackPath := state.ScrollbackFile(stateDir, scrollbackKey)
	if err := os.MkdirAll(filepath.Dir(scrollbackPath), 0o700); err != nil {
		t.Fatalf("mkdir scrollback dir: %v", err)
	}
	if err := os.WriteFile(scrollbackPath, []byte("\x1b[31mred\x1b[0m\nbefore reboot\n"), 0o600); err != nil {
		t.Fatalf("write fixture scrollback: %v", err)
	}
}

func assertMarkerFiredOnce(t *testing.T, path, marker string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read hook fire file %s (bare-shell miss leaves it absent): %v", path, err)
	}
	if got := strings.Count(string(data), marker); got != 1 {
		t.Errorf("marker %q fired %d times in %s; want exactly 1\ncontents:\n%s", marker, got, path, data)
	}
}

func assertMarkerAbsent(t *testing.T, path, marker string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		t.Fatalf("read hook fire file %s: %v", path, err)
	}
	if strings.Contains(string(data), marker) {
		t.Errorf("CROSS-FIRE: marker %q leaked into %s (the :w.p suffix did not route hooks per-pane)\ncontents:\n%s",
			marker, path, data)
	}
}
