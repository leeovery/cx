package portaltest

import (
	"os/exec"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/harnesstest"
)

// Generous: a healthy server stops answering within a poll or two of kill-server.
const tmuxServerGoneBudget = 3 * time.Second

const tmuxServerGonePollTick = 50 * time.Millisecond

// AwaitTmuxServerGone blocks until the tmux server on socketPath answers
// nothing, or its budget elapses. Call it straight after kill-server: pane
// shells outlive the server briefly, and one flushing its per-session files into
// the test's isolated HOME races the framework's RemoveAll of that temp dir.
// Best-effort — a server still answering at the budget surfaces as that
// RemoveAll's error rather than as a failure here.
func AwaitTmuxServerGone(t *testing.T, socketPath string) {
	t.Helper()
	awaitTmuxServerGone(t, socketPath, tmuxServerGoneBudget, tmuxServerGonePollTick)
}

// awaitTmuxServerGone reports whether the server disappeared inside budget.
func awaitTmuxServerGone(t *testing.T, socketPath string, budget, tick time.Duration) bool {
	t.Helper()
	return harnesstest.PollUntil(t, budget, tick, tmuxServerUnreachable(socketPath))
}

// -f /dev/null keeps the probe off the user's ~/.tmux.conf; any error from
// list-sessions means nothing is listening on the socket.
func tmuxServerUnreachable(socketPath string) func() bool {
	return func() bool {
		return exec.Command("tmux", "-S", socketPath, "-f", "/dev/null", "list-sessions").Run() != nil
	}
}
