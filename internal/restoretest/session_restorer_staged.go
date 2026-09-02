//go:build integration

package restoretest

import (
	"testing"

	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/tmux"
)

// NewSessionRestorer is how a test builds a restorer it will drive a real
// restore with against a live server, for a fixture reaching past the
// orchestrator to the session layer. binDir must hold a built portal binary:
// Exe is pinned to it through StagedHydrateExe, so every armed pane reaches
// that binary rather than os.Executable(). Under `go test` os.Executable() is
// the test binary, and a pane armed with it re-runs the suite inside itself and
// exits 0 — a vanished session, with no error anywhere.
//
// Taking binDir rather than a resolver is what makes the pinning structural: a
// caller cannot reach this constructor and still forget the field.
func NewSessionRestorer(t *testing.T, client *tmux.Client, stateDir, binDir string) *restore.SessionRestorer {
	t.Helper()
	return &restore.SessionRestorer{
		Client:   client,
		StateDir: stateDir,
		Logger:   OpenTestLogger(t, stateDir),
		Exe:      StagedHydrateExe(t, binDir),
	}
}
