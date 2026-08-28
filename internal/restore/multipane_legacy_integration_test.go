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

	const (
		pane0Key = "mpPaneToken0"
		pane1Key = "mpPaneToken1"
	)
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

	ts.StampPaneToken(t, tmux.PaneTarget(renameOldName, 0, 0), pane0Key)
	ts.StampPaneToken(t, tmux.PaneTarget(renameOldName, 0, 1), pane1Key)

	ts.Run(t, "rename-session", "-t", renameOldName, renameNewName)
	if _, err := ts.TryRun("has-session", "-t", "="+renameNewName); err != nil {
		t.Fatalf("session %q not live after rename: %v", renameNewName, err)
	}

	idx, err := state.CaptureStructure(client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}
	sess := findCapturedSession(t, idx, renameNewName)
	if len(sess.Windows) != 1 || len(sess.Windows[0].Panes) != 2 {
		t.Fatalf("captured session %q topology = %d window(s) / %v panes; want 1 window / 2 panes",
			renameNewName, len(sess.Windows), paneIndices(sess))
	}
	for i, want := range []string{pane0Key, pane1Key} {
		if got := sess.Windows[0].Panes[i].PortalPaneID; got != want {
			t.Fatalf("captured session %q pane %d token = %q; want %q", renameNewName, i, got, want)
		}
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
	if got := capturedPaneToken(t, sess); got != "" {
		t.Fatalf("captured un-stamped session %q pane token = %q; want \"\"", legacyName, got)
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
