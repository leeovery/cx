//go:build integration

package bootstrap_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestPhase5_RestoringMarkerSuppressesCaptures_NonVacuous(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	binDir := restoretest.BuildPortalBinaryDir(t)

	ts := tmuxtest.New(t, "ptl-p5-")
	stateDir := newIntegrationStateDir(t)

	probeFile := filepath.Join(t.TempDir(), "session-created.events")

	preRunSavedAt := time.Date(2026, 4, 27, 12, 0, 0, 0, time.UTC)
	restoretest.SeedSessionsJSONWithSavedAt(t, stateDir, preRunSavedAt, "probe-target")

	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	probeCmd := "run-shell \"echo $(date +%s%N) >> " + probeFile + "\""
	if out, err := ts.TryRun("set-hook", "-ga", "session-created", probeCmd); err != nil {
		t.Fatalf("install probe hook: %v\n%s", err, out)
	}
	t.Cleanup(func() {
		_, _ = ts.TryRun("set-hook", "-gu", "session-created")
	})

	logger := restoretest.OpenTestLogger(t, stateDir)

	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{
		Restore: restoreAdapterFor(t, client, stateDir, logger, binDir),
	})

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := ts.Run(t, "list-sessions", "-F", "#{session_name}")
	if !strings.Contains(out, "probe-target") {
		t.Fatalf("Restore did not create probe-target; list-sessions=%q — non-vacuity guard cannot be evaluated", out)
	}

	probeBytes, err := os.ReadFile(probeFile)
	if err != nil {
		if !os.IsNotExist(err) {
			t.Fatalf("read probe file %s: %v", probeFile, err)
		}
		probeBytes = nil
	}
	probeLines := countNonEmptyLines(string(probeBytes))
	if probeLines == 0 {
		t.Fatal("non-vacuity guard failed: probe recorded zero session-created events during the @portal-restoring window — the suppression assertion below would be vacuously true")
	}

	postIdx, skip, err := state.ReadIndex(stateDir)
	if err != nil {
		t.Fatalf("ReadIndex post-Run: %v", err)
	}
	if skip {
		t.Fatal("ReadIndex post-Run reported skip=true; sessions.json was unexpectedly removed during Run")
	}
	if !postIdx.SavedAt.Equal(preRunSavedAt) {
		t.Errorf("sessions.json.saved_at advanced during the marker window: pre=%v post=%v",
			preRunSavedAt, postIdx.SavedAt)
	}

	if val, found, err := client.TryGetServerOption(state.RestoringMarkerName); err != nil {
		t.Fatalf("TryGetServerOption final: %v", err)
	} else if found {
		t.Errorf("@portal-restoring still set after Run; value=%q", val)
	}
}

func countNonEmptyLines(s string) int {
	count := 0
	for _, line := range strings.Split(s, "\n") {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}
