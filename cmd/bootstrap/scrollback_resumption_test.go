//go:build integration

package bootstrap_test

import (
	"context"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func newProductionMarkerCleaner(client *tmux.Client, logger *slog.Logger) *bootstrap.MarkerCleanupCore {
	return &bootstrap.MarkerCleanupCore{
		Markers:  client,
		Panes:    client,
		Unsetter: client,
		Logger:   logger,
	}
}

func seedLeakedMarker(t *testing.T, ts *tmuxtest.Socket, client *tmux.Client, sessionName string) (paneKey, markerName string) {
	t.Helper()
	ts.Run(t, "new-session", "-d", "-s", sessionName, "sleep", "infinity")
	ts.WaitForSession(t, sessionName, 2*time.Second)
	paneKey = state.SanitizePaneKey(sessionName, 0, 0)
	markerName = state.SkeletonMarkerPrefix + paneKey
	ts.Run(t, "kill-session", "-t", sessionName)
	if err := client.SetServerOption(markerName, "1"); err != nil {
		t.Fatalf("SetServerOption seed marker for %s: %v", sessionName, err)
	}
	return paneKey, markerName
}

func seedKeepAlivePane(t *testing.T, ts *tmuxtest.Socket) {
	t.Helper()
	ts.Run(t, "new-session", "-d", "-s", "_seed")
	ts.WaitForSession(t, "_seed", 2*time.Second)
	tmuxtest.ApplyBaseIndices(t, ts, 0, 0)
}

func TestScrollbackResumption_DaemonTickSavesScrollbackAfterCleanup(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	_, stateDir := newIntegrationStateDir(t)

	ts := tmuxtest.New(t, "ptl-sbres-")
	client := ts.Client()

	seedKeepAlivePane(t, ts)
	paneKey, markerName := seedLeakedMarker(t, ts, client, "foo")

	logger := restoretest.OpenTestLogger(t, stateDir)
	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{
		StaleMarkers: newProductionMarkerCleaner(client, logger),
		Logger:       logger,
	})
	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Orchestrator.Run: %v", err)
	}

	_, found, err := client.TryGetServerOption(markerName)
	if err != nil {
		t.Fatalf("TryGetServerOption %s: %v", markerName, err)
	}
	if found {
		t.Fatalf("marker %s present after cleanup; want absent", markerName)
	}

	ts.Run(t, "new-session", "-d", "-s", "foo", "sleep", "infinity")
	ts.WaitForSession(t, "foo", 2*time.Second)

	runDaemonTick(t, client, stateDir)

	scrollbackPath := state.ScrollbackFile(stateDir, paneKey)
	info, err := os.Stat(scrollbackPath)
	if err != nil {
		t.Fatalf("scrollback file %s missing after daemon tick "+
			"(spec AC #8 regression — cleanup step did not "+
			"unblock scrollback save): %v", scrollbackPath, err)
	}
	if info.Size() == 0 {
		t.Fatalf("scrollback file %s is empty after daemon tick; "+
			"want non-empty (capture-pane should produce at least "+
			"one byte for a live pane)", scrollbackPath)
	}
}

func TestScrollbackResumption_WithoutCleanupScrollbackNotSaved(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	_, stateDir := newIntegrationStateDir(t)

	ts := tmuxtest.New(t, "ptl-sbres-noop-")
	client := ts.Client()

	seedKeepAlivePane(t, ts)
	paneKey, markerName := seedLeakedMarker(t, ts, client, "foo")

	logger := restoretest.OpenTestLogger(t, stateDir)

	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{
		Logger: logger,
	})
	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Orchestrator.Run: %v", err)
	}

	_, found, err := client.TryGetServerOption(markerName)
	if err != nil {
		t.Fatalf("TryGetServerOption %s: %v", markerName, err)
	}
	if !found {
		t.Fatalf("marker %s absent after no-op cleanup; want present "+
			"(regression-guard contract requires the marker to "+
			"survive when step 9 is disabled)", markerName)
	}

	ts.Run(t, "new-session", "-d", "-s", "foo", "sleep", "infinity")
	ts.WaitForSession(t, "foo", 2*time.Second)

	runDaemonTick(t, client, stateDir)

	scrollbackPath := state.ScrollbackFile(stateDir, paneKey)
	if _, err := os.Stat(scrollbackPath); !os.IsNotExist(err) {
		t.Fatalf("scrollback file %s exists after no-op cleanup tick; "+
			"want absent (skip-save guard should suppress writes "+
			"while marker is set). stat err=%v", scrollbackPath, err)
	}
}

func TestScrollbackResumption_LiveHydrateInProgressMarkerPreserved(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	_, stateDir := newIntegrationStateDir(t)

	ts := tmuxtest.New(t, "ptl-sbres-mix-")
	client := ts.Client()

	seedKeepAlivePane(t, ts)

	stalePaneKey, staleMarker := seedLeakedMarker(t, ts, client, "stalefoo")

	ts.Run(t, "new-session", "-d", "-s", "livebar", "sleep", "infinity")
	ts.WaitForSession(t, "livebar", 2*time.Second)
	livePaneKey := state.SanitizePaneKey("livebar", 0, 0)
	liveMarker := state.SkeletonMarkerPrefix + livePaneKey
	if err := client.SetServerOption(liveMarker, "1"); err != nil {
		t.Fatalf("SetServerOption live marker: %v", err)
	}

	logger := restoretest.OpenTestLogger(t, stateDir)
	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{
		StaleMarkers: newProductionMarkerCleaner(client, logger),
		Logger:       logger,
	})
	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Orchestrator.Run: %v", err)
	}

	if _, found, err := client.TryGetServerOption(staleMarker); err != nil {
		t.Fatalf("TryGetServerOption %s: %v", staleMarker, err)
	} else if found {
		t.Errorf("stale marker %s present after cleanup; want absent",
			staleMarker)
	}

	if _, found, err := client.TryGetServerOption(liveMarker); err != nil {
		t.Fatalf("TryGetServerOption %s: %v", liveMarker, err)
	} else if !found {
		t.Errorf("live marker %s absent after cleanup; want preserved "+
			"(hydrate-in-progress pane must keep its marker)",
			liveMarker)
	}

	ts.Run(t, "new-session", "-d", "-s", "stalefoo", "sleep", "infinity")
	ts.WaitForSession(t, "stalefoo", 2*time.Second)

	runDaemonTick(t, client, stateDir)

	if info, err := os.Stat(state.ScrollbackFile(stateDir, stalePaneKey)); err != nil {
		t.Errorf("scrollback file for unset-marker pane %s missing "+
			"after daemon tick: %v", stalePaneKey, err)
	} else if info.Size() == 0 {
		t.Errorf("scrollback file for unset-marker pane %s is empty; "+
			"want non-empty", stalePaneKey)
	}

	if _, err := os.Stat(state.ScrollbackFile(stateDir, livePaneKey)); !os.IsNotExist(err) {
		t.Errorf("scrollback file for preserved-marker pane %s exists "+
			"after daemon tick; want absent (skip-save guard "+
			"should suppress writes while marker is set). "+
			"stat err=%v", livePaneKey, err)
	}
}
