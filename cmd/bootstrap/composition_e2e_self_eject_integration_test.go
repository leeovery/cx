//go:build integration

package bootstrap_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

const selfEjectComposite_HysteresisTicksMirror = 3

const selfEjectComposite_TickerPeriodMirror = 1 * time.Second

const selfEjectComposite_ExitBudget = (selfEjectComposite_HysteresisTicksMirror+1)*selfEjectComposite_TickerPeriodMirror + 2*time.Second

const selfEjectComposite_ExitPollTick = 100 * time.Millisecond

const selfEjectComposite_ConvergenceTimeout = 6 * time.Second

const selfEjectComposite_PlaceholderCommand = `exec tail -f /dev/null`

const selfEjectComposite_LogMarker = "daemon: self-eject"

func TestCompositeBootstrap_ExternalSaverKillTriggersSelfEject(t *testing.T) {
	t.Setenv("PORTAL_LOG_LEVEL", "INFO")

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

	remaining := selfEjectComposite_ConvergenceTimeout - time.Since(start)
	if remaining <= 0 {
		t.Fatalf("post-bootstrap: 6 s budget already exhausted by the bootstrap "+
			"slice itself (elapsed=%s) — cannot assert convergence",
			time.Since(start))
	}
	if !waitForPgrepCount(t, 1, remaining) {
		pids, _ := portaltest.PgrepPortalDaemons()
		t.Fatalf("post-bootstrap: pgrep -fx did not converge to 1 within %s of "+
			"bootstrap-slice entry (elapsed=%s)\n"+
			"  harness saver PID (setup-time): %d (alive=%v)\n"+
			"  harness orphan1 PID: %d (alive=%v)\n"+
			"  harness orphan2 PID: %d (alive=%v)\n"+
			"  current pgrep snapshot: %v",
			selfEjectComposite_ConvergenceTimeout, time.Since(start),
			h.LegitimateDaemonPID, pidAlive(h.LegitimateDaemonPID),
			h.Orphan1PID, pidAlive(h.Orphan1PID),
			h.Orphan2PID, pidAlive(h.Orphan2PID),
			pids)
	}

	survivorPID := waitForSaverPanePID(t, h.Sock)

	if !pidAlive(survivorPID) {
		t.Fatalf("convergence survivor PID %d not alive after pgrep convergence — "+
			"composition regression: the surviving daemon died before the "+
			"self-eject test sequence could start", survivorPID)
	}

	waitForIdentifyDaemon(t, survivorPID)

	preEjectDaemonPID, err := state.ReadPIDFile(h.StateDir)
	if err != nil {
		t.Fatalf("ReadPIDFile pre-eject: %v\n"+
			"  the post-eject assertion needs the pre-eject value to "+
			"verify stale-stays-stale", err)
	}
	if preEjectDaemonPID != survivorPID {
		t.Fatalf("pre-eject daemon.pid = %d; want survivor PID %d\n"+
			"  daemon.pid is not yet refreshed to reference the survivor — "+
			"the post-eject stale-stays-stale assertion would be ambiguous",
			preEjectDaemonPID, survivorPID)
	}

	if out, runErr := h.Sock.TryRun("new-window", "-t", tmux.PortalSaverName+":",
		"sh", "-c", selfEjectComposite_PlaceholderCommand); runErr != nil {
		t.Fatalf("tmux new-window -t %s: failed: %v\n  output: %s\n"+
			"  the external-mismatch mechanism requires a successful new-window "+
			"call; without it the daemon's saver-membership probe still observes "+
			"a matching pid and the self-eject path cannot fire",
			tmux.PortalSaverName, runErr, out)
	}
	mismatchInstant := time.Now()

	newActivePID, present, err := tmux.SaverPanePIDOrAbsent(h.Client, tmux.PortalSaverName)
	if err != nil {
		t.Fatalf("tmux.SaverPanePIDOrAbsent post-new-window: %v\n"+
			"  the external-mismatch verification requires reading the saver "+
			"session's active-window pane pid; a read failure here means the "+
			"saver session was destroyed by the new-window call (unexpected)",
			err)
	}
	if !present {
		t.Fatalf("tmux.SaverPanePIDOrAbsent post-new-window: present=false\n" +
			"  the external-mismatch verification requires the saver session " +
			"to still host a pane after the new-window call; absent here means " +
			"the saver session was destroyed (unexpected)")
	}
	if newActivePID == survivorPID {
		t.Fatalf("post-new-window saver pane pid (%d) STILL equals survivor PID "+
			"(%d) — new-window did NOT switch the session's active window "+
			"to the placeholder. The daemon's saver-membership probe would "+
			"observe a matching pid every tick and never self-eject. "+
			"Likely cause: tmux version drift in new-window's default-active "+
			"behaviour. Re-evaluate the external-mismatch mechanism.",
			newActivePID, survivorPID)
	}

	if !pidAlive(survivorPID) {
		t.Fatalf("survivor PID %d not alive immediately after external mismatch — "+
			"the new-window mechanism appears to have killed the daemon (defeats "+
			"the os.Exit(0) self-eject path under test)", survivorPID)
	}

	exited, exitLatency := pollForPIDExit(survivorPID, mismatchInstant, selfEjectComposite_ExitBudget, selfEjectComposite_ExitPollTick)
	if !exited {
		logBlob := portaltest.ReadPortalLogSafe(h.StateDir)
		t.Fatalf("daemon (PID %d) did not exit within %s of external mismatch event; "+
			"the daemon must self-eject within (N+1)*TickerPeriod "+
			"= %s for N=%d (TickerPeriod=%s) plus slack\n"+
			"  elapsed: %s\n"+
			"  budget: %s\n"+
			"  daemon still alive: %v\n"+
			"--- portal.log ---\n%s",
			survivorPID, selfEjectComposite_ExitBudget,
			time.Duration(selfEjectComposite_HysteresisTicksMirror+1)*selfEjectComposite_TickerPeriodMirror,
			selfEjectComposite_HysteresisTicksMirror, selfEjectComposite_TickerPeriodMirror,
			exitLatency, selfEjectComposite_ExitBudget,
			pidAlive(survivorPID), logBlob)
	}
	t.Logf("daemon self-eject latency: %s (budget=%s, pid=%d)",
		exitLatency, selfEjectComposite_ExitBudget, survivorPID)

	logBlob := portaltest.ReadPortalLogSafe(h.StateDir)

	if ejectIdx := strings.LastIndex(logBlob, selfEjectComposite_LogMarker); ejectIdx >= 0 &&
		strings.Contains(logBlob[ejectIdx:], "capture: tick complete") {
		t.Fatalf("a capture ran AFTER daemon: self-eject — the self-eject path must "+
			"perform NO final flush (the eject tick MUST osExit(0) before tick())\n"+
			"--- portal.log from the self-eject marker onward ---\n%s", logBlob[ejectIdx:])
	}

	pidPath := state.DaemonPID(h.StateDir)
	if _, statErr := os.Stat(pidPath); statErr != nil {
		t.Fatalf("daemon.pid missing post-eject: %v\n"+
			"  the stale daemon.pid is "+
			"intentional — os.Exit(0) MUST NOT trigger any cleanup defer\n"+
			"--- portal.log ---\n%s", statErr, logBlob)
	}
	postEjectDaemonPID, err := state.ReadPIDFile(h.StateDir)
	if err != nil {
		t.Fatalf("ReadPIDFile post-eject: %v\n--- portal.log ---\n%s", err, logBlob)
	}
	if postEjectDaemonPID != survivorPID {
		t.Fatalf("post-eject daemon.pid = %d; want survivor PID %d (stale-stays-stale)\n"+
			"  the file was rewritten by some other writer between the daemon's "+
			"WritePIDFile and its self-eject — the stale pidfile must retain "+
			"the ejecting daemon's PID\n"+
			"--- portal.log ---\n%s",
			postEjectDaemonPID, survivorPID, logBlob)
	}

	if !strings.Contains(logBlob, selfEjectComposite_LogMarker) {
		t.Errorf("portal.log missing self-eject marker %q\n"+
			"  the eject path MUST emit this INFO line\n"+
			"  observed eject (PID %d gone within %s), but the marker is absent — "+
			"the daemon may have exited via a different path (SIGHUP, lock loss, ...)\n"+
			"--- portal.log ---\n%s",
			selfEjectComposite_LogMarker, survivorPID, exitLatency, logBlob)
	}
}

func pollForPIDExit(pid int, startInstant time.Time, budget, tick time.Duration) (bool, time.Duration) {
	deadline := startInstant.Add(budget)
	for {
		if !pidAlive(pid) {
			return true, time.Since(startInstant)
		}
		if time.Now().After(deadline) {
			return false, time.Since(startInstant)
		}
		time.Sleep(tick)
	}
}
