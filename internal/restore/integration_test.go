package restore_test

import (
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

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

func writeFile(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}
