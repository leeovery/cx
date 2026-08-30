//go:build integration

package bootstrap_test

import (
	"errors"
	"os/exec"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const upgradePathPGrepConvergenceTimeout = 6 * time.Second

const upgradePathPIDFileTimeout = 3 * time.Second

const upgradePathPIDFilePollTick = 50 * time.Millisecond

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

	if !waitForPgrepCount(t, 1, upgradePathPGrepConvergenceTimeout) {
		pids, _ := portaltest.PgrepPortalDaemons()
		t.Fatalf("pgrep -fxc did not converge to 1 within %s\n"+
			"  v(N) PID: %d (alive=%v)\n"+
			"  current pgrep snapshot: %v\n"+
			"  state dir: %s",
			upgradePathPGrepConvergenceTimeout,
			vNPID, pidAlive(vNPID),
			pids, stateDir)
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
	var lastPID int
	var lastErr error
	ok := tmuxtest.PollUntil(t, upgradePathPIDFileTimeout, upgradePathPIDFilePollTick, func() bool {
		pid, err := state.ReadPIDFile(stateDir)
		lastPID = pid
		lastErr = err
		if err != nil {
			return false
		}
		return pid == expectedPID
	})
	if !ok {
		t.Fatalf("daemon.pid did not converge to expected PID %d within %s\n"+
			"  last read: pid=%d err=%v\n"+
			"  state dir: %s",
			expectedPID, upgradePathPIDFileTimeout, lastPID, lastErr, stateDir)
	}
}

func waitForIdentifyDaemon(t *testing.T, pid int) {
	t.Helper()
	var lastResult state.IdentifyResult
	var lastErr error
	ok := tmuxtest.PollUntil(t, upgradePathPIDFileTimeout, upgradePathPIDFilePollTick, func() bool {
		result, err := state.IdentifyDaemon(pid)
		lastResult = result
		lastErr = err
		if err != nil {
			return false
		}
		return result == state.IdentifyIsPortalDaemon
	})
	if !ok {
		t.Fatalf("IdentifyDaemon(%d) did not converge to IdentifyIsPortalDaemon within %s\n"+
			"  last result: %v err=%v",
			pid, upgradePathPIDFileTimeout, lastResult, lastErr)
	}
}
