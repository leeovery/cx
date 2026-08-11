package bootstrap_test

import (
	"log/slog"
	"testing"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

type orchestratorOpts struct {
	Hooks         bootstrap.HookRegistrar
	OrphanSweeper bootstrap.OrphanSweeper
	Saver         bootstrap.SaverBootstrapper
	Restore       bootstrap.Restorer
	EagerSignaler bootstrap.EagerHydrateSignaler
	StaleMarkers  bootstrap.MarkerCleaner
	Sweeper       bootstrap.FIFOSweeper
	Logger        *slog.Logger
}

func buildIntegrationOrchestrator(t *testing.T, client *tmux.Client, opts orchestratorOpts) *bootstrap.Orchestrator {
	t.Helper()

	stateDir, _ := state.Dir()

	var withOpts []bootstrap.Option
	if opts.Hooks != nil {
		withOpts = append(withOpts, bootstrap.WithHooks(opts.Hooks))
	}
	if opts.OrphanSweeper != nil {
		withOpts = append(withOpts, bootstrap.WithOrphanSweeper(opts.OrphanSweeper))
	}
	if opts.Saver != nil {
		withOpts = append(withOpts, bootstrap.WithSaver(opts.Saver))
	}
	if opts.Restore != nil {
		withOpts = append(withOpts, bootstrap.WithRestore(opts.Restore))
	}
	if opts.EagerSignaler != nil {
		withOpts = append(withOpts, bootstrap.WithEagerSignaler(opts.EagerSignaler))
	}
	if opts.StaleMarkers != nil {
		withOpts = append(withOpts, bootstrap.WithStaleMarkers(opts.StaleMarkers))
	}
	if opts.Sweeper != nil {
		withOpts = append(withOpts, bootstrap.WithSweeper(opts.Sweeper))
	}

	return bootstrap.NewWithDefaults(
		client,
		stateDir,
		opts.Logger,
		&bootstrapadapter.RestoringMarker{Client: client},
		withOpts...,
	)
}

func newIntegrationStateDir(t *testing.T) string {
	t.Helper()
	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	return stateDir
}
