//go:build integration

package bootstrap_test

import (
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/tmux"
)

func TestCompositeBootstrap_ConvergesPgrepToOneDaemon(t *testing.T) {
	h := setupCompositeHarness(t)

	sweeper := bootstrapadapter.NewOrphanSweeper(h.Client, nil)
	core, ok := sweeper.(*bootstrap.OrphanSweepCore)
	if !ok {
		t.Fatalf("NewOrphanSweeper returned %T; want *bootstrap.OrphanSweepCore "+
			"(needed to inject a capturing Logger for the forbidden-string assertion)",
			sweeper)
	}
	sink := &logtest.Sink{}
	core.Logger = slog.New(sink)

	start := time.Now()

	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error "+
			"(best-effort step must return nil): %v", err)
	}

	if err := tmux.BootstrapPortalSaver(h.Client, h.StateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver (post-sweep idempotent re-run): %v", err)
	}

	if res := waitForPgrepCount(t, 1); !res.Reached {
		t.Fatalf("post-bootstrap: pgrep -fx did not converge to 1 (%s, %s since "+
			"bootstrap-slice entry)\n"+
			"  harness saver PID (setup-time): %d (alive=%v)\n"+
			"  harness orphan1 PID: %d (alive=%v)\n"+
			"  harness orphan2 PID: %d (alive=%v)",
			res, time.Since(start),
			h.LegitimateDaemonPID, pidAlive(h.LegitimateDaemonPID),
			h.Orphan1PID, pidAlive(h.Orphan1PID),
			h.Orphan2PID, pidAlive(h.Orphan2PID))
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
	for _, entry := range sink.Lines() {
		if strings.Contains(entry, forbiddenNoSuchSession) {
			t.Fatalf("bootstrap logger emitted forbidden entry containing %q\n"+
				"  entry: %s\n"+
				"  all entries:\n%s",
				forbiddenNoSuchSession, entry,
				sink.Body())
		}
		if strings.Contains(entry, forbiddenPriorDaemonExit) {
			t.Fatalf("bootstrap logger emitted forbidden entry containing %q\n"+
				"  entry: %s\n"+
				"  all entries:\n%s",
				forbiddenPriorDaemonExit, entry,
				sink.Body())
		}
	}
}
