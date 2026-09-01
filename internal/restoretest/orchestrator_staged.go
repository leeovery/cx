//go:build integration

package restoretest

import (
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/tmux"
)

// NewRestoreOrchestrator is how a test builds an orchestrator it will drive a
// real restore with against a live server. binDir must hold a built portal
// binary: Exe is pinned to it through StagedHydrateExe, so the arming of every
// restored pane reaches that binary rather than os.Executable(). Under `go test`
// os.Executable() is the test binary, and a pane armed with it re-runs the suite
// inside itself and exits 0 — a vanished session, with no error anywhere.
//
// Taking binDir rather than a resolver is what makes the pinning structural: a
// caller cannot reach this constructor and still forget the field.
func NewRestoreOrchestrator(t *testing.T, client *tmux.Client, stateDir, binDir string) *restore.Orchestrator {
	t.Helper()
	return &restore.Orchestrator{
		Client:   client,
		StateDir: stateDir,
		Logger:   OpenTestLogger(t, stateDir),
		Exe:      StagedHydrateExe(t, binDir),
	}
}

// StagedRestoreAdapter is the same pinning one layer up, for a test driving the
// bootstrap orchestrator instead of the restore one: the adapter production
// builds, with its inner restore's pane-arming pointed at the staged binary.
func StagedRestoreAdapter(t *testing.T, client *tmux.Client, stateDir string, logger *slog.Logger, binDir string) *bootstrapadapter.RestoreAdapter {
	t.Helper()
	a := bootstrapadapter.NewRestoreAdapter(client, stateDir, logger)
	a.Inner.Exe = StagedHydrateExe(t, binDir)
	return a
}
