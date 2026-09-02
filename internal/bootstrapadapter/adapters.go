// Package bootstrapadapter wires tmux-client primitives to the
// bootstrap.Orchestrator step interfaces. It sits outside cmd/ so tests can
// import production-equivalent wiring without pulling in the rest of cmd/.
package bootstrapadapter

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// RestoringMarker manages the @portal-restoring server-option lifecycle that
// suppresses the save daemon during skeleton restore. Client must be non-nil.
type RestoringMarker struct {
	Client *tmux.Client
}

func (m *RestoringMarker) Set() error {
	return m.Client.SetServerOption(state.RestoringMarkerName, "1")
}

// Clear is idempotent: unsetting an absent option is a tmux no-op.
func (m *RestoringMarker) Clear() error {
	return m.Client.UnsetServerOption(state.RestoringMarkerName)
}

// HookRegistrar registers Portal's global tmux hooks idempotently. VersionLogger
// is a separate field because the daemon.version breadcrumb belongs to the daemon
// component, not bootstrap's. Both loggers tolerate nil.
type HookRegistrar struct {
	Client        *tmux.Client
	Logger        *slog.Logger
	VersionLogger *slog.Logger
}

// RegisterPortalHooks registers the hooks and, as a side effect, installs the
// internal/tmux package-level barrier and version-writer logger sinks. Both
// setters are idempotent and nil-tolerant.
func (r *HookRegistrar) RegisterPortalHooks() error {
	tmux.SetBarrierLogger(r.Logger)
	tmux.SetVersionWriterLogger(r.VersionLogger)
	return tmux.RegisterPortalHooks(r.Client, r.Logger)
}

// RestoreAdapter wraps a *restore.Orchestrator so its Restore method satisfies
// bootstrap.Restorer. The bootstrap orchestrator owns the @portal-restoring
// marker lifecycle, so the inner Restore must not bundle marker management.
type RestoreAdapter struct {
	Inner *restore.Orchestrator
}

func (a *RestoreAdapter) Restore() (bool, error) { return a.Inner.Restore() }

// SetProgress installs the per-session restore progress callback. It is kept off
// the Restorer interface so Restore's contract is unchanged; a route that never
// calls it leaves Progress nil and the restore loop unaltered.
func (a *RestoreAdapter) SetProgress(fn func(n, m int)) { a.Inner.Progress = fn }

// ErrRestoreExeRequired is returned by NewRestoreAdapter for a nil exe.
var ErrRestoreExeRequired = errors.New("restore adapter: an executable resolver is required")

// NewRestoreAdapter builds a *RestoreAdapter over a fresh inner orchestrator,
// with the arming of every restored pane pinned to exe.
//
// exe is a required parameter rather than an optional field because an unset one
// is silent: the restore falls back to os.Executable, which for anything but the
// shipped binary — a test binary above all — arms panes with a process that is
// not the hydrate helper, and the session disappears with no error. Production
// composes the orchestrator directly, where that fallback is the intent.
func NewRestoreAdapter(client *tmux.Client, stateDir string, logger *slog.Logger, exe restore.ExecutableResolver) (*RestoreAdapter, error) {
	if exe == nil {
		return nil, ErrRestoreExeRequired
	}
	return &RestoreAdapter{
		Inner: &restore.Orchestrator{
			Client:   client,
			StateDir: stateDir,
			Logger:   logger,
			Exe:      exe,
		},
	}, nil
}

// FIFOSweeper removes hydrate FIFOs left behind by panes that no longer exist.
// Client is typed as state.ServerOptionLister rather than *tmux.Client so a test
// can inject a failing stub without standing up a real server.
type FIFOSweeper struct {
	Client   state.ServerOptionLister
	StateDir string
	Logger   *slog.Logger
}

func (s *FIFOSweeper) Sweep() error {
	markers, err := state.ListSkeletonMarkers(s.Client)
	if err != nil {
		return fmt.Errorf("list skeleton markers: %w", err)
	}
	return state.SweepOrphanFIFOs(s.StateDir, markers, s.Logger)
}
