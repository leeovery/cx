package cmd

// These adapters live in cmd/ rather than cmd/bootstrap so the bootstrap package
// stays free of internal/restore and internal/state dependencies.

import (
	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// Carries the binary's ldflags-injected version so the version-marker upgrade
// protocol fires on release-build mismatches.
type saverAdapter struct {
	client   *tmux.Client
	stateDir string
}

func (a *saverAdapter) EnsureSaver() error {
	return tmux.EnsurePortalSaverVersion(a.client, a.stateDir, version)
}

// The default is byte-identical to what tmux.DefaultClient builds. Do not cache
// the Client built from this factory across builds: it is invoked once per
// buildProductionOrchestrator call, and a caller that flips it between phases
// expects the new Commander in the next build.
var commanderFactory = func() tmux.Commander { return &tmux.RealCommander{} }

func buildProductionOrchestrator() (*bootstrap.Orchestrator, *tmux.Client) {
	client := tmux.NewClient(commanderFactory())

	// An error here does not abort bootstrap: state.EnsureDir is retried inside
	// individual subsystems.
	stateDir, _ := state.Dir()

	logger := bootstrapLogger

	// Progress is left nil: the per-session callback is installed at Run time,
	// and only on the concurrent cold-boot route where an emitter exists. On the
	// synchronous route it stays nil and the restore loop is unchanged.
	restoreInner := &restore.Orchestrator{
		Client:   client,
		StateDir: stateDir,
		Logger:   restoreLogger,
	}

	orch := &bootstrap.Orchestrator{
		Server:        client,
		Hooks:         &bootstrapadapter.HookRegistrar{Client: client, Logger: logger, VersionLogger: daemonLogger},
		Restoring:     &bootstrapadapter.RestoringMarker{Client: client},
		OrphanSweeper: bootstrapadapter.NewOrphanSweeper(client, logger),
		Saver:         &saverAdapter{client: client, stateDir: stateDir},
		Restore:       &bootstrapadapter.RestoreAdapter{Inner: restoreInner},
		// The stateDir resolved above is reused across Restore, FIFOSweeper and
		// EagerSignalCore so they observe the same directory.
		EagerSignaler: &bootstrap.EagerSignalCore{
			Markers:  client,
			StateDir: stateDir,
			Signaler: state.DefaultFIFOSignaler{},
			Logger:   hydrateLogger,
		},
		StaleMarkers: &bootstrap.MarkerCleanupCore{
			Markers:  client,
			Panes:    client,
			Unsetter: client,
			Logger:   logger,
		},
		Sweeper: &bootstrapadapter.FIFOSweeper{
			Client:   client,
			StateDir: stateDir,
			Logger:   logger,
		},
		Latch:   client,
		Version: version,
		Logger:  logger,
	}
	return orch, client
}
