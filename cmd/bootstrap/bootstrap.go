// Package bootstrap runs the tmux server bootstrap sequence invoked from root's
// PersistentPreRunE. Step ordering is load-bearing.
package bootstrap

import (
	"context"
	"log/slog"
	"time"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/state"
)

// Deliberately a second component in this package: the clean component is
// shared with internal/state's sweeps so one grep reconstructs them all.
var cleanLogger = log.For("clean")

const totalSteps = 10

const (
	stepEnsureServer       = "EnsureServer"
	stepRegisterHooks      = "RegisterPortalHooks"
	stepSetRestoring       = "SetRestoring"
	stepSweepOrphanDaemons = "SweepOrphanDaemons"
	stepEnsureSaver        = "EnsureSaver"
	stepRestore            = "Restore"
	stepEagerSignalHydrate = "EagerSignalHydrate"
	stepClearRestoring     = "ClearRestoring"
	stepCleanStaleMarkers  = "CleanStaleMarkers"
	stepSweepOrphanFIFOs   = "SweepOrphanFIFOs"
)

// Runner abstracts the bootstrap run so PersistentPreRunE need not import the
// concrete *Orchestrator.
type Runner interface {
	Run(ctx context.Context) (bool, []Warning, error)
}

// ServerBootstrapper starts the tmux server when not already running; the bool
// reports whether Portal itself was the one that started it.
type ServerBootstrapper interface {
	EnsureServer() (bool, error)
}

// HookRegistrar registers Portal's global tmux hooks idempotently.
type HookRegistrar interface {
	RegisterPortalHooks() error
}

// RestoringMarker manages the @portal-restoring server option that
// suppresses the save daemon while skeleton restoration is in flight.
type RestoringMarker interface {
	Set() error
	Clear() error
}

// SaverBootstrapper ensures the _portal-saver detached session exists
// and matches the current binary version.
type SaverBootstrapper interface {
	EnsureSaver() error
}

// Restorer performs skeleton-only session restoration. err is non-nil only when
// corrupt is true, and must then wrap state.ErrCorruptIndex; per-session
// failures are logged and swallowed inside the implementation.
type Restorer interface {
	Restore() (corrupt bool, err error)
}

// RestoreProgressSink is the optional seam a Restorer may also satisfy. The
// restore step calls SetProgress only when a progress emitter is wired.
type RestoreProgressSink interface {
	SetProgress(fn func(n, m int))
}

// EagerHydrateSignaler writes the hydrate signal byte to every freshly-armed
// `@portal-skeleton-*` pane's FIFO, because the client-attached hook fires only
// for the attached session and the other helpers would time out and leak
// markers. Every non-nil return is soft.
type EagerHydrateSignaler interface {
	EagerSignalHydrate() error
}

// MarkerCleaner unsets every `@portal-skeleton-*` marker whose paneKey is no
// longer represented by a live pane. A non-nil return is soft.
type MarkerCleaner interface {
	CleanStaleMarkers() error
}

// FIFOSweeper removes stale hydrate-*.fifo files whose paneKey is no longer
// represented by a live `@portal-skeleton-*` marker. Implementations swallow
// per-file failures; a non-nil return means enumeration itself failed.
type FIFOSweeper interface {
	Sweep() error
}

// LatchWriter records the version-stamped @portal-bootstrapped latch as the
// final action of a successful bootstrap. Nil-tolerant, and best-effort.
type LatchWriter interface {
	SetServerOption(name, value string) error
}

// Orchestrator runs the bootstrap sequence. A nil Logger is tolerated — Run
// substitutes a discard sink.
type Orchestrator struct {
	Server        ServerBootstrapper
	Hooks         HookRegistrar
	Restoring     RestoringMarker
	OrphanSweeper OrphanSweeper
	Saver         SaverBootstrapper
	Restore       Restorer
	EagerSignaler EagerHydrateSignaler
	StaleMarkers  MarkerCleaner
	Sweeper       FIFOSweeper
	Latch         LatchWriter
	Version       string
	Logger        *slog.Logger
}

// Run executes the bootstrap steps in order and reports whether Portal started
// the tmux server, plus the soft warnings in step order. Only EnsureServer,
// RegisterPortalHooks and the @portal-restoring steps are fatal; every other
// step logs its failure and continues.
func (o *Orchestrator) Run(ctx context.Context) (bool, []Warning, error) {
	emit := progressEmitterFromContext(ctx)
	emitStep := func(index int, name string) {
		if emit != nil {
			emit(StepEvent{Index: index, Name: name})
		}
	}

	o.Logger = log.OrDiscard(o.Logger)

	orchestrationStart := time.Now()

	var warnings []Warning

	o.Logger.Debug("step entering", "step", stepEnsureServer)
	stepStart := time.Now()
	serverStarted, err := o.Server.EnsureServer()
	if err != nil {
		return false, nil, o.fatalf("start tmux server", err)
	}
	o.Logger.Info("step complete", "step", stepEnsureServer, log.Took(stepStart))
	emitStep(1, stepEnsureServer)

	o.Logger.Debug("step entering", "step", stepRegisterHooks)
	stepStart = time.Now()
	if err := o.Hooks.RegisterPortalHooks(); err != nil {
		return serverStarted, nil, o.fatalf("register tmux hooks", err)
	}
	o.Logger.Info("step complete", "step", stepRegisterHooks, log.Took(stepStart))
	emitStep(2, stepRegisterHooks)

	// Must precede the orphan sweep and the saver: the daemon and the hydrate
	// helpers key their suppression window off this marker.
	o.Logger.Debug("step entering", "step", stepSetRestoring)
	stepStart = time.Now()
	if err := o.Restoring.Set(); err != nil {
		return serverStarted, nil, o.fatalf("set @portal-restoring marker", err)
	}
	o.Logger.Info("step complete", "step", stepSetRestoring, log.Took(stepStart))
	emitStep(3, stepSetRestoring)

	// Must precede EnsureSaver so the new saver-pane daemon's first tick is
	// uncontested by leftover daemons from prior server lifetimes.
	o.Logger.Debug("step entering", "step", stepSweepOrphanDaemons)
	stepStart = time.Now()
	if err := o.OrphanSweeper.SweepOrphanDaemons(); err != nil {
		o.Logger.Warn("step failed", "step", stepSweepOrphanDaemons, "error", err)
	}
	o.Logger.Info("step complete", "step", stepSweepOrphanDaemons, log.Took(stepStart))
	emitStep(4, stepSweepOrphanDaemons)

	o.Logger.Debug("step entering", "step", stepEnsureSaver)
	stepStart = time.Now()
	if err := o.Saver.EnsureSaver(); err != nil {
		warnings = append(warnings, SaverDownWarning())
		o.Logger.Warn("step failed", "step", stepEnsureSaver, "error", err)
	}
	o.Logger.Info("step complete", "step", stepEnsureSaver, log.Took(stepStart))
	emitStep(5, stepEnsureSaver)

	o.Logger.Debug("step entering", "step", stepRestore)
	stepStart = time.Now()
	if emit != nil {
		if sink, ok := o.Restore.(RestoreProgressSink); ok {
			sink.SetProgress(func(n, m int) {
				emit(StepEvent{Index: 6, Name: stepRestore, RestoreN: n, RestoreM: m})
			})
		}
	}
	corrupt, restoreErr := o.Restore.Restore()
	switch {
	case corrupt:
		warnings = append(warnings, CorruptSessionsJSONWarning())
		if restoreErr != nil {
			o.Logger.Warn("step failed: corrupt sessions.json", "step", stepRestore, "error", restoreErr)
		}
	case restoreErr != nil:
		// Contract violation (corrupt=false with a non-nil err): swallowed so
		// restore can never escalate to a bootstrap abort.
		o.Logger.Warn("step returned non-corrupt error (treated as soft per Restorer contract)", "step", stepRestore, "error", restoreErr)
	}
	o.Logger.Info("step complete", "step", stepRestore, log.Took(stepStart))
	emitStep(6, stepRestore)

	// Must run before the marker is cleared, so the daemon's capture
	// suppression still covers the helper-driven scrollback replay.
	o.Logger.Debug("step entering", "step", stepEagerSignalHydrate)
	stepStart = time.Now()
	if err := o.EagerSignaler.EagerSignalHydrate(); err != nil {
		o.Logger.Warn("step failed", "step", stepEagerSignalHydrate, "error", err)
	}
	o.Logger.Info("step complete", "step", stepEagerSignalHydrate, log.Took(stepStart))
	emitStep(7, stepEagerSignalHydrate)

	o.Logger.Debug("step entering", "step", stepClearRestoring)
	stepStart = time.Now()
	if err := o.Restoring.Clear(); err != nil {
		return serverStarted, warnings, o.fatalf("clear @portal-restoring marker", err)
	}
	o.Logger.Info("step complete", "step", stepClearRestoring, log.Took(stepStart))
	emitStep(8, stepClearRestoring)

	// Must run after the marker is cleared, so it observes post-restore tmux
	// state, and before the FIFO sweep, which cannot reclaim a FIFO whose
	// marker is still set.
	o.Logger.Debug("step entering", "step", stepCleanStaleMarkers)
	stepStart = time.Now()
	if err := o.StaleMarkers.CleanStaleMarkers(); err != nil {
		o.Logger.Warn("step failed", "step", stepCleanStaleMarkers, "error", err)
	}
	o.Logger.Info("step complete", "step", stepCleanStaleMarkers, log.Took(stepStart))
	emitStep(9, stepCleanStaleMarkers)

	o.Logger.Debug("step entering", "step", stepSweepOrphanFIFOs)
	stepStart = time.Now()
	if err := o.Sweeper.Sweep(); err != nil {
		o.Logger.Warn("step failed", "step", stepSweepOrphanFIFOs, "error", err)
	}
	o.Logger.Info("step complete", "step", stepSweepOrphanFIFOs, log.Took(stepStart))
	emitStep(10, stepSweepOrphanFIFOs)

	// Must stay Run's last action: the latch has to land before the concurrent
	// route's terminal Done event, so a present latch always means a bootstrap
	// that ran to completion.
	if o.Latch != nil {
		if err := o.Latch.SetServerOption(state.BootstrappedMarkerName, o.Version); err != nil {
			o.Logger.Warn("latch write failed for "+state.BootstrappedMarkerName, "error", err)
		}
	}

	o.Logger.Info("orchestration complete", "steps", totalSteps, "warnings", len(warnings), log.Took(orchestrationStart))
	return serverStarted, warnings, nil
}

func (o *Orchestrator) fatal(userMsg string, cause error) error {
	o.Logger.Error(userMsg, "error", cause)
	return NewFatal(userMsg, cause)
}

func (o *Orchestrator) fatalf(verb string, err error) error {
	return o.fatal("Portal failed to "+verb+": "+err.Error(), err)
}
