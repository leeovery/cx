//go:build integration

package bootstrap_test

import (
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/tmux"
)

const convergencePGrepTimeout = 6 * time.Second

func TestCompositeBootstrap_ConvergesPgrepToOneWithin6s(t *testing.T) {
	h := setupCompositeHarness(t)

	sweeper := bootstrapadapter.NewOrphanSweeper(h.Client, nil)
	core, ok := sweeper.(*bootstrap.OrphanSweepCore)
	if !ok {
		t.Fatalf("NewOrphanSweeper returned %T; want *bootstrap.OrphanSweepCore "+
			"(needed to inject a recording Logger for the forbidden-string assertion)",
			sweeper)
	}
	logger := &bootstrap.RecordingLogger{}
	core.Logger = logger.Logger()

	start := time.Now()

	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error "+
			"(best-effort step must return nil): %v", err)
	}

	if err := tmux.BootstrapPortalSaver(h.Client, h.StateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver (post-sweep idempotent re-run): %v", err)
	}

	remaining := convergencePGrepTimeout - time.Since(start)
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
			convergencePGrepTimeout, time.Since(start), convergencePGrepTimeout,
			h.LegitimateDaemonPID, pidAlive(h.LegitimateDaemonPID),
			h.Orphan1PID, pidAlive(h.Orphan1PID),
			h.Orphan2PID, pidAlive(h.Orphan2PID),
			pids)
	}
	convergenceElapsed := time.Since(start)

	survivors, err := portaltest.PgrepPortalDaemons()
	if err != nil {
		t.Fatalf("post-bootstrap pgrep snapshot: %v", err)
	}
	if len(survivors) != 1 {
		t.Fatalf("post-bootstrap: expected exactly 1 daemon, got %d: %v "+
			"(convergence elapsed: %s)",
			len(survivors), survivors, convergenceElapsed)
	}
	currentSaverPID := waitForSaverPanePID(t, h.Sock)
	if survivors[0] != currentSaverPID {
		t.Fatalf("post-bootstrap: survivor PID %d != current saver pane PID %d\n"+
			"  harness saver PID (setup-time): %d\n"+
			"  harness orphan1 PID: %d\n"+
			"  harness orphan2 PID: %d\n"+
			"  the surviving daemon is NOT the saver-pane process — composition regression",
			survivors[0], currentSaverPID,
			h.LegitimateDaemonPID, h.Orphan1PID, h.Orphan2PID)
	}

	const forbiddenNoSuchSession = "no such session: _portal-saver"
	const forbiddenPriorDaemonExit = "prior daemon did not exit"
	for _, entry := range logger.AllEntries() {
		if strings.Contains(entry, forbiddenNoSuchSession) {
			t.Fatalf("bootstrap logger emitted forbidden entry containing %q\n"+
				"  entry: %s\n"+
				"  all entries:\n%s",
				forbiddenNoSuchSession, entry,
				strings.Join(logger.AllEntries(), "\n"))
		}
		if strings.Contains(entry, forbiddenPriorDaemonExit) {
			t.Fatalf("bootstrap logger emitted forbidden entry containing %q\n"+
				"  entry: %s\n"+
				"  all entries:\n%s",
				forbiddenPriorDaemonExit, entry,
				strings.Join(logger.AllEntries(), "\n"))
		}
	}
}
