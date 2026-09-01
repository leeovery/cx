//go:build integration

package bootstrap_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestPhase5_RestoreCreatesMissingSession(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	binDir := restoretest.BuildPortalBinaryDir(t)

	_, stateDir := newIntegrationStateDir(t)

	ts := tmuxtest.New(t, "ptl-p5-")

	restoretest.SeedSessionsJSON(t, stateDir, "missing-foo")

	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	if _, err := ts.TryRun("has-session", "-t", "missing-foo"); err == nil {
		t.Fatal("missing-foo unexpectedly live before Run")
	}

	logger := restoretest.OpenTestLogger(t, stateDir)

	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{
		Restore: restoretest.StagedRestoreAdapter(t, client, stateDir, logger, binDir),
		// The NoOp opt-out is load-bearing: a real eager signaler would let the
		// pane die and step 9 unset the very marker this test asserts survives.
		EagerSignaler: bootstrap.NoOpEagerHydrateSignaler{},
	})

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	out := ts.Run(t, "list-sessions", "-F", "#{session_name}")
	if !strings.Contains(out, "missing-foo") {
		t.Errorf("expected missing-foo in list-sessions; got %q", out)
	}

	if val, found, err := client.TryGetServerOption(state.RestoringMarkerName); err != nil {
		t.Fatalf("TryGetServerOption: %v", err)
	} else if found {
		t.Errorf("@portal-restoring still set after Run; value=%q", val)
	}

	wantMarker := "@portal-skeleton-" + state.SanitizePaneKey("missing-foo", 0, 0)
	if val, found, err := client.TryGetServerOption(wantMarker); err != nil {
		t.Fatalf("TryGetServerOption %s: %v", wantMarker, err)
	} else if !found || val == "" {
		t.Errorf("expected skeleton marker %q to be set; found=%v value=%q", wantMarker, found, val)
	}
}

func TestPhase5_FIFOSweeperRemovesOrphansAfterRestore(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	binDir := restoretest.BuildPortalBinaryDir(t)

	_, stateDir := newIntegrationStateDir(t)

	ts := tmuxtest.New(t, "ptl-p5-")

	restoretest.SeedSessionsJSON(t, stateDir, "swept-foo")

	liveKey := state.SanitizePaneKey("swept-foo", 0, 0)
	orphanKey := state.SanitizePaneKey("ghost-bar", 0, 0)
	livePath := state.FIFOPath(stateDir, liveKey)
	orphanPath := state.FIFOPath(stateDir, orphanKey)
	if err := state.CreateFIFO(livePath); err != nil {
		t.Fatalf("create live FIFO: %v", err)
	}
	if err := state.CreateFIFO(orphanPath); err != nil {
		t.Fatalf("create orphan FIFO: %v", err)
	}

	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	logger := restoretest.OpenTestLogger(t, stateDir)

	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{
		Restore: restoretest.StagedRestoreAdapter(t, client, stateDir, logger, binDir),
		Sweeper: &bootstrapadapter.FIFOSweeper{
			Client:   client,
			StateDir: stateDir,
			Logger:   logger,
		},
		// The NoOp opt-out is load-bearing: a real eager signaler would let the
		// pane die, step 9 unset its marker, and step 10 sweep the live FIFO
		// this test asserts survives.
		EagerSignaler: bootstrap.NoOpEagerHydrateSignaler{},
	})

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	if _, err := os.Lstat(livePath); err != nil {
		t.Errorf("live FIFO removed (paneKey=%q): %v", liveKey, err)
	}

	if _, err := os.Lstat(orphanPath); !os.IsNotExist(err) {
		t.Errorf("orphan FIFO not removed (paneKey=%q): lstat err = %v", orphanKey, err)
	}
}
