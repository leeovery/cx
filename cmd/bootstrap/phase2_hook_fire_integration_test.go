//go:build integration

package bootstrap_test

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestPhase2_HookFiresOnNonAttachedSession_AC2(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	// Restored panes respawn into `portal state hydrate`, so the binary must be
	// on PATH or the helper never reaches the on-resume hook waited on below.
	binDir := restoretest.BuildPortalBinaryDir(t)
	restoretest.PrependPATH(t, binDir)

	stateDir := newIntegrationStateDir(t)

	hooksPath := filepath.Join(t.TempDir(), "hooks.json")
	t.Setenv("PORTAL_HOOKS_FILE", hooksPath)

	sentinelDir := t.TempDir()
	sentinelFile := filepath.Join(sentinelDir, "hook-fired")

	sessions := []string{"alpha", "beta"}
	restoretest.SeedSessionsJSON(t, stateDir, sessions...)

	betaHookKey := tmux.PaneTarget("beta", 0, 0)
	hookCmd := fmt.Sprintf("touch %s", sentinelFile)
	store := hooks.NewStore(hooksPath)
	if err := store.Set(betaHookKey, "on-resume", hookCmd, "cli"); err != nil {
		t.Fatalf("hooks.Set: %v", err)
	}

	ts := tmuxtest.New(t, "ptl-p2-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	for _, name := range sessions {
		if _, err := ts.TryRun("has-session", "-t", name); err == nil {
			t.Fatalf("session %q unexpectedly live before Run", name)
		}
	}

	logger := restoretest.OpenTestLogger(t, stateDir)

	o := buildIntegrationOrchestrator(t, client, orchestratorOpts{
		Restore: bootstrapadapter.NewRestoreAdapter(client, stateDir, logger),
		Logger:  logger,
	})

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Orchestrator.Run: %v", err)
	}

	liveOut := ts.Run(t, "list-sessions", "-F", "#{session_name}")
	for _, name := range sessions {
		if !strings.Contains(liveOut, name) {
			t.Fatalf("session %q not live after Run; list-sessions=%q "+
				"(non-vacuity guard cannot be evaluated — Restore did "+
				"not skeleton-create)", name, liveOut)
		}
	}

	defer dumpPortalLogOnFailure(t, stateDir)
	restoretest.WaitForFileExists(t, sentinelFile, 2*time.Second, 50*time.Millisecond)
}
