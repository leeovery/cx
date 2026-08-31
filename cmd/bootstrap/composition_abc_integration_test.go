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

const compositionScrollbackObservations = 10

const compositionScrollbackInterval = 1 * time.Second

func TestComposition_PhaseFour_ABC_EndToEnd(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	skipIfNoPgrep(t)
	_ = portalbintest.StagePortalBinary(t)

	envSlice, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-comp-abc-")
	client := sock.Client()

	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver (legitimate saver): %v", err)
	}
	legitimateSaverPID := waitForSaverPanePID(t, sock)
	waitForDaemonPID(t, stateDir, legitimateSaverPID)

	orphan1, _ := portaltest.SpawnIsolatedDaemon(t, envSlice, sock.SocketPath())
	orphan2, _ := portaltest.SpawnIsolatedDaemon(t, envSlice, sock.SocketPath())

	if res := waitForPgrepCount(t, 3); !res.Reached {
		t.Fatalf("pre-state: pgrep -fx did not reach 3 (%s)\n"+
			"  legitimate saver PID: %d (alive=%v)\n"+
			"  orphan1 PID: %d (alive=%v)\n"+
			"  orphan2 PID: %d (alive=%v)\n"+
			"  hint: an orphan may have exited before bootstrap — composition test cannot fire",
			res,
			legitimateSaverPID, pidAlive(legitimateSaverPID),
			orphan1.Process.Pid, pidAlive(orphan1.Process.Pid),
			orphan2.Process.Pid, pidAlive(orphan2.Process.Pid))
	}

	start := time.Now()

	sweeper := bootstrapadapter.NewOrphanSweeper(client, nil)
	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error: %v", err)
	}
	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver (post-sweep idempotent re-run): %v", err)
	}

	if res := waitForPgrepCount(t, 1); !res.Reached {
		t.Fatalf("post-bootstrap: pgrep -fx did not converge to 1 (%s, %s since bootstrap entry)\n"+
			"  legitimate saver PID: %d (alive=%v)\n"+
			"  orphan1 PID: %d (alive=%v)\n"+
			"  orphan2 PID: %d (alive=%v)",
			res, time.Since(start),
			legitimateSaverPID, pidAlive(legitimateSaverPID),
			orphan1.Process.Pid, pidAlive(orphan1.Process.Pid),
			orphan2.Process.Pid, pidAlive(orphan2.Process.Pid))
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
				"observation %d/%d (there must be no .bin file deletions "+
				"or unexpected new files)\n"+
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
