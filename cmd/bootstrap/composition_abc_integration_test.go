//go:build integration

package bootstrap_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const compositionPGrepConvergenceTimeout = 6 * time.Second

const compositionPreStateTimeout = 3 * time.Second

const compositionScrollbackObservations = 10

const compositionScrollbackInterval = 1 * time.Second

func TestComposition_PhaseFour_ABC_EndToEnd(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	skipIfNoPgrep(t)
	_ = portalbintest.StagePortalBinary(t)

	envSlice, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	sock := tmuxtest.New(t, "ptl-comp-abc-")
	client := sock.Client()

	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver (legitimate saver): %v", err)
	}
	legitimateSaverPID := waitForSaverPanePID(t, sock)
	waitForDaemonPID(t, stateDir, legitimateSaverPID)

	orphan1, _ := portaltest.SpawnIsolatedDaemon(t, envSlice, sock.SocketPath())
	orphan2, _ := portaltest.SpawnIsolatedDaemon(t, envSlice, sock.SocketPath())

	if !waitForPgrepCount(t, 3, compositionPreStateTimeout) {
		pids, _ := portaltest.PgrepPortalDaemons()
		t.Fatalf("pre-state: pgrep -fx did not reach 3 within %s\n"+
			"  legitimate saver PID: %d (alive=%v)\n"+
			"  orphan1 PID: %d (alive=%v)\n"+
			"  orphan2 PID: %d (alive=%v)\n"+
			"  pgrep snapshot: %v\n"+
			"  hint: an orphan may have exited before bootstrap — composition test cannot fire",
			compositionPreStateTimeout,
			legitimateSaverPID, pidAlive(legitimateSaverPID),
			orphan1.Process.Pid, pidAlive(orphan1.Process.Pid),
			orphan2.Process.Pid, pidAlive(orphan2.Process.Pid),
			pids)
	}

	start := time.Now()

	sweeper := bootstrapadapter.NewOrphanSweeper(client, nil)
	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error: %v", err)
	}
	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver (post-sweep idempotent re-run): %v", err)
	}

	remaining := compositionPGrepConvergenceTimeout - time.Since(start)
	if remaining <= 0 {
		t.Fatalf("post-bootstrap: 6 s budget already exhausted by bootstrap step itself "+
			"(elapsed=%s) — cannot assert convergence",
			time.Since(start))
	}
	if !waitForPgrepCount(t, 1, remaining) {
		pids, _ := portaltest.PgrepPortalDaemons()
		t.Fatalf("post-bootstrap: pgrep -fx did not converge to 1 within %s of bootstrap entry "+
			"(elapsed=%s, budget=%s)\n"+
			"  legitimate saver PID: %d (alive=%v)\n"+
			"  orphan1 PID: %d (alive=%v)\n"+
			"  orphan2 PID: %d (alive=%v)\n"+
			"  current pgrep snapshot: %v",
			compositionPGrepConvergenceTimeout, time.Since(start), compositionPGrepConvergenceTimeout,
			legitimateSaverPID, pidAlive(legitimateSaverPID),
			orphan1.Process.Pid, pidAlive(orphan1.Process.Pid),
			orphan2.Process.Pid, pidAlive(orphan2.Process.Pid),
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
	currentSaverPID := readSaverPanePID(t, sock)
	if survivors[0] != currentSaverPID {
		t.Fatalf("post-bootstrap: survivor PID %d != saver pane PID %d\n"+
			"  the surviving daemon is NOT the legitimate saver-pane process\n"+
			"  this is a composition regression — either B killed the wrong daemon "+
			"or A/F failed to recycle the saver pane",
			survivors[0], currentSaverPID)
	}

	scrollbackDir := state.ScrollbackDir(stateDir)
	first, err := portaltest.SnapshotStateDir(scrollbackDir)
	if err != nil {
		t.Fatalf("scrollback snapshot 1/%d: %v", compositionScrollbackObservations, err)
	}
	if len(first) == 0 {
		t.Logf("scrollback dir empty at observation 1 — empty-stays-empty is a valid "+
			"stability proof; remaining %d observations will assert no entries appear",
			compositionScrollbackObservations-1)
	}

	for i := 2; i <= compositionScrollbackObservations; i++ {
		time.Sleep(compositionScrollbackInterval)
		current, snapErr := portaltest.SnapshotStateDir(scrollbackDir)
		if snapErr != nil {
			t.Fatalf("scrollback snapshot %d/%d: %v",
				i, compositionScrollbackObservations, snapErr)
		}
		if deltas := portaltest.DiffFingerprints(first, current); len(deltas) > 0 {
			lines := make([]string, len(deltas))
			for k, d := range deltas {
				lines[k] = "  " + portaltest.FormatDelta(d)
			}
			t.Fatalf("scrollback dir oscillated between first snapshot and "+
				"observation %d/%d (spec § Composite End-to-End Verification "+
				"bullet 6: \"no .bin file deletions or unexpected new files\")\n"+
				"  scrollback dir: %s\n"+
				"  delta(s):\n%s",
				i, compositionScrollbackObservations, scrollbackDir,
				strings.Join(lines, "\n"))
		}
	}

	fd, acquireErr := state.AcquireDaemonLock(stateDir)
	if fd != nil {
		_ = fd.Close()
	}
	if !errors.Is(acquireErr, state.ErrDaemonLockHeld) {
		t.Fatalf("fresh-process AcquireDaemonLock = %v; want ErrDaemonLockHeld "+
			"(Component C pre-check should fire on the live survivor PID %d; "+
			"convergence elapsed: %s)",
			acquireErr, currentSaverPID, convergenceElapsed)
	}
}
