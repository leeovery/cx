//go:build integration

package restoretest

import (
	"testing"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// OpenRebootGap is the half of the reboot every fixture shares: it kills the
// tmux server and confirms the kill landed, so whatever is reconstructed
// afterwards came from saved state rather than from a session that simply
// survived. A reboot that never happened would let every later assertion pass
// for the wrong reason, which is why the confirmation is not optional.
//
// It exists apart from RebootServer for the fixture whose subject IS the server
// start — a bootstrap driven across the gap decides for itself whether the
// server was already up, so starting one here would answer its question for it.
// Every other caller wants RebootServer.
func OpenRebootGap(t *testing.T, ts *tmuxtest.Socket) {
	t.Helper()
	ts.KillServer()
	if _, err := ts.TryRun("list-sessions"); err == nil {
		t.Fatal("list-sessions succeeded after kill-server; the reboot gap was never opened")
	}
}

// RebootServer opens that gap and starts a fresh server across it, for a
// fixture that needs a live server on the far side — whether it drives the
// restore directly or through a bootstrap that must find the server already up.
// The start goes through EnsureServer, so the server it leaves carries the
// _portal-bootstrap anchor and nothing else: a fixture asserting on the raw
// session set must allow for it.
//
// Server-lifetime options (renumber-windows, base indices) do not survive the
// kill, so a fixture that depends on one must re-apply it between this call and
// the restore.
func RebootServer(t *testing.T, ts *tmuxtest.Socket, client *tmux.Client) {
	t.Helper()
	OpenRebootGap(t, ts)
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
	return RestoreWithMarker(t, client, NewRestoreOrchestrator(t, client, stateDir, binDir))
}
