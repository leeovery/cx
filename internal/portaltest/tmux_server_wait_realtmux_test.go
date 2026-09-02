package portaltest

import (
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmuxtest"
)

// One real server, driven across its own death: the probe's argv is only
// pinned by a server that actually answers it, and an argv that never answers
// would make the wait return instantly with nothing to show for it.
func TestAwaitTmuxServerGone(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-await-gone-")
	ts.Run(t, "new-session", "-d", "-s", "await-gone", "sleep", "infinity")
	probe := tmuxServerUnreachable(ts.SocketPath())

	t.Run("it reports a live server as reachable", func(t *testing.T) {
		if probe() {
			t.Fatal("probe reported the server unreachable while it was answering; its argv or its error direction is wrong")
		}
	})

	t.Run("it returns at its bound rather than hanging on a server that will not exit", func(t *testing.T) {
		const budget = 100 * time.Millisecond

		start := time.Now()
		observed := awaitTmuxServerGone(t, ts.SocketPath(), budget, 10*time.Millisecond)
		elapsed := time.Since(start)

		if observed {
			t.Fatal("the wait reported the server gone while it was still answering")
		}
		if elapsed < budget {
			t.Fatalf("the wait returned after %s; want it to spend its %s budget", elapsed, budget)
		}
		if elapsed > budget+2*time.Second {
			t.Fatalf("the wait took %s; want it bounded near its %s budget rather than hanging on the server", elapsed, budget)
		}
	})

	t.Run("it blocks until the tmux server is unreachable", func(t *testing.T) {
		// Killed on a delay, so a wait that does not poll returns while the
		// server is still answering rather than after it has already gone.
		const settle = 300 * time.Millisecond
		go func() {
			time.Sleep(settle)
			ts.KillServer()
		}()

		start := time.Now()
		AwaitTmuxServerGone(t, ts.SocketPath())
		elapsed := time.Since(start)

		if elapsed < settle {
			t.Fatalf("the wait returned after %s, before the server exited at %s", elapsed, settle)
		}
		if !probe() {
			t.Fatalf("the wait returned after %s with the server still answering", elapsed)
		}
		if elapsed >= tmuxServerGoneBudget {
			t.Fatalf("the wait burned its whole %s budget over a killed server (took %s)", tmuxServerGoneBudget, elapsed)
		}
	})
}
