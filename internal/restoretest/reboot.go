//go:build integration

package restoretest

import (
	"testing"

	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// RebootServer opens the reboot gap: it kills the tmux server and starts a fresh
// empty one, so whatever a restore then reconstructs came from saved state
// rather than from a session that simply survived. The kill is confirmed before
// the new server starts — a reboot that never happened would let every later
// assertion pass for the wrong reason.
//
// Server-lifetime options (renumber-windows, base indices) do not survive the
// kill, so a fixture that depends on one must re-apply it between this call and
// the restore.
func RebootServer(t *testing.T, ts *tmuxtest.Socket, client *tmux.Client) {
	t.Helper()
	ts.KillServer()
	if _, err := ts.TryRun("list-sessions"); err == nil {
		t.Fatal("list-sessions succeeded after kill-server; the reboot gap was never opened")
	}
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
}

// RestoreFromState runs a real restore of stateDir's saved index against the
// live server, bracketed by @portal-restoring, and returns the restore's own
// error.
//
// binDir must hold a built portal binary: the arming of every restored pane is
// pointed at it through StagedHydrateExe, which is what keeps a restored pane
// from respawning into the test binary.
func RestoreFromState(t *testing.T, client *tmux.Client, stateDir, binDir string) error {
	t.Helper()
	o := &restore.Orchestrator{
		Client:   client,
		StateDir: stateDir,
		Logger:   OpenTestLogger(t, stateDir),
		Exe:      StagedHydrateExe(t, binDir),
	}
	return RestoreWithMarker(t, client, o)
}
