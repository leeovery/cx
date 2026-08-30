//go:build integration

package bootstrap_test

import (
	"errors"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const pgrepConvergenceTimeout = 3 * time.Second

const pgrepConvergencePollTick = 50 * time.Millisecond

const recycledPIDSettleWindow = 200 * time.Millisecond

func TestSweepOrphanDaemons_Integration_ThreeDaemonsConvergeToOne(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	skipIfNoPgrep(t)
	_ = portalbintest.StagePortalBinary(t)

	envSlice, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-sweep3-")
	client := sock.Client()

	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver: %v", err)
	}
	saverPID := waitForSaverPanePID(t, sock)

	orphan1, _ := portaltest.SpawnIsolatedDaemon(t, envSlice, sock.SocketPath())
	orphan2, _ := portaltest.SpawnIsolatedDaemon(t, envSlice, sock.SocketPath())

	if !waitForPgrepCount(t, 3, pgrepConvergenceTimeout) {
		pids, _ := portaltest.PgrepPortalDaemons()
		t.Fatalf("precondition: pgrep -fxc did not reach 3 within %s\n"+
			"  saverPID: %d\n"+
			"  orphan1.PID: %d (alive=%v)\n"+
			"  orphan2.PID: %d (alive=%v)\n"+
			"  pgrep snapshot: %v\n"+
			"  hint: an orphan may have exited before the sweep — see test diagnostic above",
			pgrepConvergenceTimeout,
			saverPID,
			orphan1.Process.Pid, pidAlive(orphan1.Process.Pid),
			orphan2.Process.Pid, pidAlive(orphan2.Process.Pid),
			pids)
	}

	sweeper := bootstrapadapter.NewOrphanSweeper(client, nil)
	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error (best-effort step must return nil): %v", err)
	}

	if !waitForPgrepCount(t, 1, pgrepConvergenceTimeout) {
		pids, _ := portaltest.PgrepPortalDaemons()
		t.Fatalf("post-sweep: pgrep -fxc did not converge to 1 within %s; current pgrep=%v",
			pgrepConvergenceTimeout, pids)
	}

	survivors, err := portaltest.PgrepPortalDaemons()
	if err != nil {
		t.Fatalf("post-sweep pgrep snapshot: %v", err)
	}
	if len(survivors) != 1 {
		t.Fatalf("post-sweep: expected exactly 1 daemon, got %d: %v", len(survivors), survivors)
	}
	currentSaverPID := readSaverPanePID(t, sock)
	if survivors[0] != currentSaverPID {
		t.Fatalf("post-sweep: survivor pid=%d does not match saver pane pid=%d",
			survivors[0], currentSaverPID)
	}
}

func TestSweepOrphanDaemons_Integration_CleanStateZeroSignals(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	skipIfNoPgrep(t)
	_ = portalbintest.StagePortalBinary(t)

	_, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-sweepclean-")
	client := sock.Client()

	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver: %v", err)
	}
	_ = waitForSaverPanePID(t, sock)

	if !waitForPgrepCount(t, 1, pgrepConvergenceTimeout) {
		pids, _ := portaltest.PgrepPortalDaemons()
		t.Fatalf("precondition: pgrep -fxc did not reach 1 within %s; pgrep=%v",
			pgrepConvergenceTimeout, pids)
	}

	sweeper := bootstrapadapter.NewOrphanSweeper(client, nil)
	core, ok := sweeper.(*bootstrap.OrphanSweepCore)
	if !ok {
		t.Fatalf("NewOrphanSweeper returned %T; want *bootstrap.OrphanSweepCore "+
			"(needed to inject a recording Logger)", sweeper)
	}
	logger := &bootstrap.RecordingLogger{}
	core.Logger = logger.Logger()

	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error: %v", err)
	}

	const forbidden = "sweep: killed orphan daemon"
	for _, entry := range logger.AllEntries() {
		if strings.Contains(entry, forbidden) {
			t.Fatalf("clean-state sweep emitted forbidden log entry containing %q\n"+
				"  entry: %s\n"+
				"  all entries:\n%s",
				forbidden, entry, strings.Join(logger.AllEntries(), "\n"))
		}
	}

	pids, err := portaltest.PgrepPortalDaemons()
	if err != nil {
		t.Fatalf("post-sweep pgrep: %v", err)
	}
	if len(pids) != 1 {
		t.Fatalf("post-sweep: pgrep returned %d daemons, want 1: %v", len(pids), pids)
	}
}

func TestSweepOrphanDaemons_Integration_RecycledPIDRefusal(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	skipIfNoPgrep(t)
	_ = portalbintest.StagePortalBinary(t)

	_, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-sweeprecycle-")
	client := sock.Client()

	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver: %v", err)
	}
	saverPID := waitForSaverPanePID(t, sock)

	sleeper := exec.Command("sh", "-c", "exec sleep 30")
	if err := sleeper.Start(); err != nil {
		t.Fatalf("start sleep process: %v", err)
	}
	sleeperPID := sleeper.Process.Pid
	reaped := portaltest.RegisterSubprocessCleanup(t, sleeper)

	sweeper := bootstrapadapter.NewOrphanSweeper(client, nil)
	core, ok := sweeper.(*bootstrap.OrphanSweepCore)
	if !ok {
		t.Fatalf("NewOrphanSweeper returned %T; want *bootstrap.OrphanSweepCore", sweeper)
	}
	core.Pgrep = func() ([]int, error) {
		return []int{saverPID, sleeperPID}, nil
	}

	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error: %v", err)
	}

	time.Sleep(recycledPIDSettleWindow)

	killErr := syscall.Kill(sleeperPID, 0)
	if errors.Is(killErr, syscall.ESRCH) {
		t.Fatalf("sleeper PID %d was killed by SweepOrphanDaemons; "+
			"the identity check failed to refuse the non-daemon PID\n"+
			"  this is a Component B recycled-PID safety violation",
			sleeperPID)
	}
	if killErr != nil {
		t.Logf("sleeper PID %d: kill(pid, 0) returned %v (proceeding; non-ESRCH means process still exists)",
			sleeperPID, killErr)
	}

	_ = reaped
}

func skipIfNoPgrep(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("pgrep"); err != nil {
		t.Skipf("pgrep not available; skipping orphan-sweep integration test: %v", err)
	}
}

func waitForSaverPanePID(t *testing.T, sock *tmuxtest.Socket) int {
	t.Helper()
	var pid int
	ok := tmuxtest.PollUntil(t, pgrepConvergenceTimeout, pgrepConvergencePollTick, func() bool {
		out, err := sock.TryRun("list-panes", "-t", tmux.PortalSaverName, "-F", "#{pane_pid}")
		if err != nil {
			return false
		}
		p, perr := strconv.Atoi(strings.TrimSpace(out))
		if perr != nil || p <= 0 {
			return false
		}
		pid = p
		return true
	})
	if !ok {
		t.Fatalf("saver pane PID did not become observable within %s", pgrepConvergenceTimeout)
	}
	state.RegisterSandboxDaemon(pid)
	return pid
}

func readSaverPanePID(t *testing.T, sock *tmuxtest.Socket) int {
	t.Helper()
	for attempt := 0; attempt < 2; attempt++ {
		out, err := sock.TryRun("list-panes", "-t", tmux.PortalSaverName, "-F", "#{pane_pid}")
		if err == nil {
			p, perr := strconv.Atoi(strings.TrimSpace(out))
			if perr == nil && p > 0 {
				state.RegisterSandboxDaemon(p)
				return p
			}
		}
		time.Sleep(pgrepConvergencePollTick)
	}
	t.Fatalf("saver pane PID unreadable after retry; saver pane may have died between setup and sweep")
	return 0
}

func waitForPgrepCount(t *testing.T, target int, timeout time.Duration) bool {
	t.Helper()
	return tmuxtest.PollUntil(t, timeout, pgrepConvergencePollTick, func() bool {
		pids, err := portaltest.PgrepPortalDaemons()
		if err != nil {
			return false
		}
		return len(pids) == target
	})
}

func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH)
}

var _ slog.Handler = (*bootstrap.RecordingLogger)(nil)
