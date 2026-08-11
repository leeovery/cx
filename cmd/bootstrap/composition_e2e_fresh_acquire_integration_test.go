//go:build integration

package bootstrap_test

import (
	"errors"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

const freshAcquireConvergenceTimeout = 6 * time.Second

func TestCompositeBootstrap_FreshAcquireDaemonLockRefusesPostBootstrap(t *testing.T) {
	h := setupCompositeHarness(t)

	sweeper := bootstrapadapter.NewOrphanSweeper(h.Client, nil)
	start := time.Now()
	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error "+
			"(best-effort step must return nil): %v", err)
	}
	if err := tmux.BootstrapPortalSaver(h.Client, h.StateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver (post-sweep idempotent re-run): %v", err)
	}

	remaining := freshAcquireConvergenceTimeout - time.Since(start)
	if remaining <= 0 {
		t.Fatalf("post-bootstrap: 6 s budget already exhausted by the bootstrap "+
			"slice itself (elapsed=%s) — cannot assert convergence",
			time.Since(start))
	}
	if !waitForPgrepCount(t, 1, remaining) {
		pids, _ := portaltest.PgrepPortalDaemons()
		t.Fatalf("post-bootstrap: pgrep -fx did not converge to 1 within %s of "+
			"bootstrap-slice entry (elapsed=%s, budget=%s)\n"+
			"  harness saver PID (setup-time): %d (alive=%v)\n"+
			"  harness orphan1 PID: %d (alive=%v)\n"+
			"  harness orphan2 PID: %d (alive=%v)\n"+
			"  current pgrep snapshot: %v",
			freshAcquireConvergenceTimeout, time.Since(start), freshAcquireConvergenceTimeout,
			h.LegitimateDaemonPID, pidAlive(h.LegitimateDaemonPID),
			h.Orphan1PID, pidAlive(h.Orphan1PID),
			h.Orphan2PID, pidAlive(h.Orphan2PID),
			pids)
	}

	currentSaverPID := waitForSaverPanePID(t, h.Sock)

	pidOnDisk, err := state.ReadPIDFile(h.StateDir)
	if err != nil {
		t.Fatalf("ReadPIDFile after bootstrap convergence: %v", err)
	}
	if pidOnDisk != currentSaverPID {
		t.Fatalf("post-bootstrap daemon.pid = %d; want current saver pane PID %d\n"+
			"  harness saver PID (setup-time): %d\n"+
			"  harness orphan1 PID: %d (alive=%v)\n"+
			"  harness orphan2 PID: %d (alive=%v)\n"+
			"  daemon.pid was not refreshed to reference the survivor — "+
			"the pre-check path cannot fire and the assertion below would "+
			"either fail spuriously or pass via the flock fallback only",
			pidOnDisk, currentSaverPID,
			h.LegitimateDaemonPID,
			h.Orphan1PID, pidAlive(h.Orphan1PID),
			h.Orphan2PID, pidAlive(h.Orphan2PID))
	}

	waitForIdentifyDaemon(t, currentSaverPID)

	fd, acquireErr := state.AcquireDaemonLock(h.StateDir)

	if fd != nil {
		_ = fd.Close()
		t.Fatalf("AcquireDaemonLock returned non-nil fd alongside error %v — "+
			"the refusal path must not leak an fd "+
			"(survivor PID = %d)",
			acquireErr, currentSaverPID)
	}

	if !errors.Is(acquireErr, state.ErrDaemonLockHeld) {
		t.Fatalf("AcquireDaemonLock from test goroutine = %v; want ErrDaemonLockHeld "+
			"(Component C pre-check should fire on the live survivor PID %d; "+
			"daemon.pid on disk = %d)",
			acquireErr, currentSaverPID, pidOnDisk)
	}

	pidOnDiskAfter, err := state.ReadPIDFile(h.StateDir)
	if err != nil {
		t.Fatalf("ReadPIDFile after refused AcquireDaemonLock: %v", err)
	}
	if pidOnDiskAfter != currentSaverPID {
		t.Fatalf("daemon.pid after refused AcquireDaemonLock = %d; want survivor PID %d\n"+
			"  the refusal mutated daemon.pid — destructive-coexistence violation",
			pidOnDiskAfter, currentSaverPID)
	}

	if !state.IsProcessAlive(currentSaverPID) {
		t.Fatalf("survivor PID %d is no longer alive after refused AcquireDaemonLock — "+
			"the pre-check appears to have signalled the live holder "+
			"(Component C destructive-coexistence violation)",
			currentSaverPID)
	}
}
