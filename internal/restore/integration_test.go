package restore_test

import (
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestPhase3Integration_SaveRestoreRoundTrip(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-")
	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	ts.Run(t, "new-session", "-d", "-s", "alpha")
	ts.WaitForSession(t, "alpha", 2*time.Second)

	client := ts.Client()

	idx, err := state.CaptureStructure(client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}
	if len(idx.Sessions) != 1 || idx.Sessions[0].Name != "alpha" {
		t.Fatalf("expected one captured session named alpha, got %+v", idx.Sessions)
	}

	data, err := state.EncodeIndex(idx)
	if err != nil {
		t.Fatalf("EncodeIndex: %v", err)
	}
	if err := writeFile(state.SessionsJSON(stateDir), data); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}

	ts.KillServer()
	if _, err := ts.TryRun("list-sessions"); err == nil {
		t.Fatalf("expected list-sessions to error after kill-server")
	}

	// The orchestrator assumes a live server, and set-option does not start one.
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

	out := ts.Run(t, "list-sessions", "-F", "#{session_name}")
	if !strings.Contains(out, "alpha") {
		t.Fatalf("expected alpha in list-sessions; got %q", out)
	}

	wantMarker := "@portal-skeleton-" + state.SanitizePaneKey("alpha", 0, 0)
	markerOut := ts.Run(t, "show-options", "-sv", wantMarker)
	if strings.TrimSpace(markerOut) == "" {
		t.Errorf("expected marker %q to be set; got empty value", wantMarker)
	}

	if out, err := ts.TryRun("show-options", "-sv", state.RestoringMarkerName); err == nil && strings.TrimSpace(out) != "" {
		t.Errorf("%s should be unset after marker block; got %q", state.RestoringMarkerName, out)
	}

	if _, err := o.Restore(); err != nil {
		t.Fatalf("second Restore: %v", err)
	}
	out2 := ts.Run(t, "list-sessions", "-F", "#{session_name}")
	if got := strings.Count(out2, "alpha"); got != 1 {
		t.Errorf("expected exactly one alpha session after second Restore; got %d in %q", got, out2)
	}
}

func TestPhase3Integration_SweepOrphanFIFOs(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	liveKey := state.SanitizePaneKey("alpha", 0, 0)
	orphanKey := state.SanitizePaneKey("ghost", 0, 0)

	live := state.FIFOPath(stateDir, liveKey)
	orphan := state.FIFOPath(stateDir, orphanKey)
	if err := state.CreateFIFO(live); err != nil {
		t.Fatalf("CreateFIFO live: %v", err)
	}
	if err := state.CreateFIFO(orphan); err != nil {
		t.Fatalf("CreateFIFO orphan: %v", err)
	}

	liveSet := map[string]struct{}{liveKey: {}}
	if err := state.SweepOrphanFIFOs(stateDir, liveSet, nil); err != nil {
		t.Fatalf("SweepOrphanFIFOs: %v", err)
	}

	if !pathExists(live) {
		t.Errorf("live FIFO %s was unexpectedly removed", live)
	}
	if pathExists(orphan) {
		t.Errorf("orphan FIFO %s was not removed", orphan)
	}
}

func TestPhase3Integration_CorruptSessionsJSON(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-")
	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	if err := writeFile(state.SessionsJSON(stateDir), []byte("{not json")); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}

	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
	logger := restoretest.OpenTestLogger(t, stateDir)

	o := &restore.Orchestrator{
		Client:   client,
		StateDir: stateDir,
		Logger:   logger,
	}
	rwmErr := restoretest.RestoreWithMarker(t, client, o)
	if rwmErr == nil {
		t.Fatal("restoreWithMarker returned nil; expected wrapped state.ErrCorruptIndex")
	}
	if !errors.Is(rwmErr, state.ErrCorruptIndex) {
		t.Errorf("restoreWithMarker err = %v; want errors.Is(err, state.ErrCorruptIndex) = true", rwmErr)
	}

	out, err := ts.TryRun("list-sessions", "-F", "#{session_name}")
	if err == nil {
		for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
			line = strings.TrimSpace(line)
			if line == "" || line == tmux.PortalBootstrapName {
				continue
			}
			t.Errorf("unexpected session %q after corrupt-index restore; out=%q", line, out)
		}
	}
}

func TestPhase3Integration_RestoreUsesLiveIndicesUnderBaseIndexDrift(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-")
	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	ts.Run(t, "new-session", "-d", "-s", "alpha")
	ts.WaitForSession(t, "alpha", 2*time.Second)

	client := ts.Client()
	idx, err := state.CaptureStructure(client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}
	if len(idx.Sessions) != 1 {
		t.Fatalf("expected 1 captured session, got %d", len(idx.Sessions))
	}
	savedPane := idx.Sessions[0].Windows[0].Panes[0]
	if savedPane.Index != 0 {
		t.Fatalf("saved pane Index = %d, want 0 (capture should reflect server default)", savedPane.Index)
	}

	data, err := state.EncodeIndex(idx)
	if err != nil {
		t.Fatalf("EncodeIndex: %v", err)
	}
	if err := writeFile(state.SessionsJSON(stateDir), data); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}

	ts.KillServer()

	ts.Run(t, "new-session", "-d", "-s", "_bootstrap")
	ts.WaitForSession(t, "_bootstrap", 2*time.Second)
	tmuxtest.ApplyBaseIndices(t, ts, 1, 1)

	logger := restoretest.OpenTestLogger(t, stateDir)

	o := &restore.Orchestrator{
		Client:   client,
		StateDir: stateDir,
		Logger:   logger,
	}
	if err := restoretest.RestoreWithMarker(t, client, o); err != nil {
		t.Fatalf("restoreWithMarker: %v", err)
	}

	out := ts.Run(t, "list-sessions", "-F", "#{session_name}")
	if !strings.Contains(out, "alpha") {
		t.Fatalf("expected alpha in list-sessions; got %q", out)
	}

	livePanesOut := ts.Run(t, "list-panes", "-s", "-t", "alpha", "-F", "#{window_index}:#{pane_index}")
	livePanesOut = strings.TrimSpace(livePanesOut)
	if livePanesOut != "1:1" {
		t.Fatalf("alpha live panes = %q, want %q (base-index drift)", livePanesOut, "1:1")
	}

	liveKey := state.SanitizePaneKey("alpha", 1, 1)
	liveFIFO := state.FIFOPath(stateDir, liveKey)
	if _, err := os.Lstat(liveFIFO); err != nil {
		t.Errorf("expected FIFO at live key %s, missing: %v", liveFIFO, err)
	}
	savedKey := state.SanitizePaneKey("alpha", 0, 0)
	savedFIFO := state.FIFOPath(stateDir, savedKey)
	if _, err := os.Lstat(savedFIFO); err == nil {
		t.Errorf("did not expect FIFO at saved-key path %s under index drift", savedFIFO)
	}

	wantMarker := "@portal-skeleton-" + liveKey
	markerOut := ts.Run(t, "show-options", "-sv", wantMarker)
	if strings.TrimSpace(markerOut) == "" {
		t.Errorf("expected marker %q to be set; got empty value", wantMarker)
	}
	dontWantMarker := "@portal-skeleton-" + savedKey
	if out, err := ts.TryRun("show-options", "-sv", dontWantMarker); err == nil && strings.TrimSpace(out) != "" {
		t.Errorf("did not expect marker %q (saved-key); got %q", dontWantMarker, out)
	}
}

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
