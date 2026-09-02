//go:build integration

package bootstrap_test

import (
	"errors"
	"fmt"
	"os/exec"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const upgradePathPIDFilePollTick = 50 * time.Millisecond

// daemon.pid converges through observable steps (the file appearing, then
// carrying the respawned daemon's PID), so Stall bounds how long the reading may
// sit unchanged rather than how long the whole convergence takes.
var upgradePathPIDFileWait = harnesstest.ProgressWait{
	Stall:   5 * time.Second,
	Ceiling: 30 * time.Second,
	Tick:    upgradePathPIDFilePollTick,
}

const upgradePathNonDestructiveSettleWindow = 200 * time.Millisecond

func TestUpgradePath_TwoBinary_AllComponentsCompose(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	skipIfNoPgrep(t)
	_ = portalbintest.StagePortalBinary(t)

	envSlice, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-upgrade-abc-")
	client := sock.Client()

	vNEnv := append([]string{}, envSlice...)
	vNEnv = append(vNEnv, "PORTAL_STATE_DIR="+stateDir)
	vN := exec.Command("portal", "state", "daemon")
	vN.Env = vNEnv
	if err := vN.Start(); err != nil {
		t.Fatalf("start v(N) daemon: %v", err)
	}
	vNPID := vN.Process.Pid
	_ = portaltest.RegisterSubprocessCleanup(t, vN)

	waitForDaemonPID(t, stateDir, vNPID)

	sweeper := bootstrapadapter.NewOrphanSweeper(client, nil)
	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error: %v", err)
	}
	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver: %v", err)
	}

	if res := waitForPgrepCount(t, 1); !res.Reached {
		t.Fatalf("pgrep -fxc did not converge to 1 (%s)\n"+
			"  v(N) PID: %d (alive=%v)\n"+
			"  state dir: %s",
			res, vNPID, pidAlive(vNPID), stateDir)
	}

	survivors, err := portaltest.PgrepPortalDaemons()
	if err != nil {
		t.Fatalf("post-bootstrap pgrep snapshot: %v", err)
	}
	if len(survivors) != 1 {
		t.Fatalf("post-bootstrap: expected exactly 1 daemon, got %d: %v", len(survivors), survivors)
	}
	survivorPID := survivors[0]
	if survivorPID == vNPID {
		t.Fatalf("post-bootstrap: survivor PID %d equals original v(N) PID %d — "+
			"the v(N) orphan was NOT swept (Component B regression)",
			survivorPID, vNPID)
	}

	pidOnDisk, err := state.ReadPIDFile(stateDir)
	if err != nil {
		t.Fatalf("ReadPIDFile after bootstrap: %v", err)
	}
	if pidOnDisk != survivorPID {
		t.Fatalf("daemon.pid on disk = %d; want survivor PID %d "+
			"(stale daemon.pid from v(N) was not overwritten)",
			pidOnDisk, survivorPID)
	}

	fd, acquireErr := state.AcquireDaemonLock(stateDir)
	if fd != nil {
		_ = fd.Close()
	}
	if !errors.Is(acquireErr, state.ErrDaemonLockHeld) {
		t.Fatalf("AcquireDaemonLock from test goroutine = %v; want ErrDaemonLockHeld "+
			"(Component C pre-check should fire on the live survivor PID %d)",
			acquireErr, survivorPID)
	}
}

func TestUpgradePath_ComponentC_IsolatedRefusesCleanly(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	skipIfNoPgrep(t)
	_ = portalbintest.StagePortalBinary(t)

	envSlice, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	_ = tmuxtest.New(t, "ptl-upgrade-c-iso-")

	vNEnv := append([]string{}, envSlice...)
	vNEnv = append(vNEnv, "PORTAL_STATE_DIR="+stateDir)
	vN := exec.Command("portal", "state", "daemon")
	vN.Env = vNEnv
	if err := vN.Start(); err != nil {
		t.Fatalf("start v(N) daemon: %v", err)
	}
	vNPID := vN.Process.Pid
	_ = portaltest.RegisterSubprocessCleanup(t, vN)

	waitForDaemonPID(t, stateDir, vNPID)
	waitForIdentifyDaemon(t, vNPID)

	fd, acquireErr := state.AcquireDaemonLock(stateDir)
	if fd != nil {
		_ = fd.Close()
	}
	if !errors.Is(acquireErr, state.ErrDaemonLockHeld) {
		t.Fatalf("AcquireDaemonLock against live v(N) PID %d = %v; want ErrDaemonLockHeld "+
			"(Component C pre-check should fire)",
			vNPID, acquireErr)
	}

	time.Sleep(upgradePathNonDestructiveSettleWindow)

	if !state.IsProcessAlive(vNPID) {
		t.Fatalf("v(N) PID %d is no longer alive %s after AcquireDaemonLock refusal — "+
			"the pre-check appears to have signalled the live holder "+
			"(Component C destructive-coexistence violation)",
			vNPID, upgradePathNonDestructiveSettleWindow)
	}
}

func TestUpgradePath_PostBootstrap_FreshAcquireDaemonLockRefuses(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	skipIfNoPgrep(t)
	_ = portalbintest.StagePortalBinary(t)

	_, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-upgrade-fresh-")
	client := sock.Client()

	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver: %v", err)
	}

	saverPID := waitForSaverPanePID(t, sock)
	waitForDaemonPID(t, stateDir, saverPID)

	fd, acquireErr := state.AcquireDaemonLock(stateDir)
	if fd != nil {
		_ = fd.Close()
	}
	if !errors.Is(acquireErr, state.ErrDaemonLockHeld) {
		t.Fatalf("AcquireDaemonLock from fresh test goroutine = %v; want ErrDaemonLockHeld "+
			"(saver PID = %d; layered-enforcement should refuse via pre-check or flock)",
			acquireErr, saverPID)
	}
}

func waitForDaemonPID(t *testing.T, stateDir string, expectedPID int) {
	t.Helper()
	res := harnesstest.AwaitProgress(t, upgradePathPIDFileWait,
		func() portaltest.DaemonPIDObservation { return portaltest.ObserveDaemonPID(stateDir) },
		func(o portaltest.DaemonPIDObservation) bool { return o.Err == "" && o.PID == expectedPID })
	if !res.Reached {
		t.Fatalf("daemon.pid did not converge to expected PID %d (%s)\n"+
			"  state dir: %s",
			expectedPID, res, stateDir)
	}
}

func waitForIdentifyDaemon(t *testing.T, pid int) {
	t.Helper()
	res := harnesstest.AwaitProgress(t, upgradePathPIDFileWait,
		func() identifyObservation { return observeIdentifyDaemon(pid) },
		func(o identifyObservation) bool {
			return o.Err == "" && o.Result == state.IdentifyIsPortalDaemon
		})
	if !res.Reached {
		t.Fatalf("IdentifyDaemon(%d) did not converge to IdentifyIsPortalDaemon (%s)", pid, res)
	}
}

// identifyObservation is comparable so the wait can tell an identity that is
// still settling from one that has stopped changing, and carries the probe error
// rather than discarding it so a red run says why the answer was withheld.
type identifyObservation struct {
	Result state.IdentifyResult
	Err    string
}

func (o identifyObservation) String() string {
	if o.Err != "" {
		return fmt.Sprintf("result=%v err=%s", o.Result, o.Err)
	}
	return fmt.Sprintf("result=%v", o.Result)
}

func observeIdentifyDaemon(pid int) identifyObservation {
	result, err := state.IdentifyDaemon(pid)
	obs := identifyObservation{Result: result}
	if err != nil {
		obs.Err = err.Error()
	}
	return obs
}
