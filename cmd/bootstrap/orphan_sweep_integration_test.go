//go:build integration

package bootstrap_test

import (
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const pgrepConvergencePollTick = 50 * time.Millisecond

// The daemon population converges through several observable steps (orphans
// dying, the respawned saver re-registering), so Stall bounds how long it may
// sit unchanged rather than how long the whole convergence takes, and Ceiling
// only backstops a population that churns without ever settling: a loaded
// machine makes convergence slower without making it wrong.
var pgrepConvergenceWait = harnesstest.ProgressWait{
	Stall:   10 * time.Second,
	Ceiling: 45 * time.Second,
	Tick:    pgrepConvergencePollTick,
}

// The saver pane becomes readable through several observable steps (the session
// appearing, its pane answering, the respawn landing), so Stall bounds how long
// the read may sit unchanged rather than how long the whole sequence takes.
var saverPanePIDWait = harnesstest.ProgressWait{
	Stall:   5 * time.Second,
	Ceiling: 30 * time.Second,
	Tick:    pgrepConvergencePollTick,
}

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

	if res := waitForPgrepCount(t, 3); !res.Reached {
		t.Fatalf("precondition: pgrep -fxc did not reach 3 (%s)\n"+
			"  saverPID: %d\n"+
			"  orphan1.PID: %d (alive=%v)\n"+
			"  orphan2.PID: %d (alive=%v)\n"+
			"  hint: an orphan may have exited before the sweep — see test diagnostic above",
			res,
			saverPID,
			orphan1.Process.Pid, pidAlive(orphan1.Process.Pid),
			orphan2.Process.Pid, pidAlive(orphan2.Process.Pid))
	}

	sweeper := bootstrapadapter.NewOrphanSweeper(client, nil)
	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error (best-effort step must return nil): %v", err)
	}

	if res := waitForPgrepCount(t, 1); !res.Reached {
		t.Fatalf("post-sweep: pgrep -fxc did not converge to 1 (%s)", res)
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

	if res := waitForPgrepCount(t, 1); !res.Reached {
		t.Fatalf("precondition: pgrep -fxc did not reach 1 (%s)", res)
	}

	sweeper := bootstrapadapter.NewOrphanSweeper(client, nil)
	core, ok := sweeper.(*bootstrap.OrphanSweepCore)
	if !ok {
		t.Fatalf("NewOrphanSweeper returned %T; want *bootstrap.OrphanSweepCore "+
			"(needed to inject a capturing Logger)", sweeper)
	}
	sink := &logtest.Sink{}
	core.Logger = slog.New(sink)

	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error: %v", err)
	}

	const forbidden = "sweep: killed orphan daemon"
	for _, entry := range sink.Lines() {
		if strings.Contains(entry, forbidden) {
			t.Fatalf("clean-state sweep emitted forbidden log entry containing %q\n"+
				"  entry: %s\n"+
				"  all entries:\n%s",
				forbidden, entry, sink.Body())
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
	res := harnesstest.AwaitProgress(t, saverPanePIDWait,
		func() panePIDObservation { return observeSaverPanePID(sock) },
		func(o panePIDObservation) bool { return o.pid() > 0 })
	if !res.Reached {
		t.Fatalf("saver pane PID did not become observable (%s)", res)
	}
	pid := res.Last.pid()
	state.RegisterSandboxDaemon(pid)
	return pid
}

// panePIDObservation is comparable so the wait can tell a saver pane that is
// still coming up from one that has stopped changing, and carries the read
// error rather than discarding it so a red run says why the PID was unreadable.
type panePIDObservation struct {
	Out string
	Err string
}

func (o panePIDObservation) String() string {
	if o.Err != "" {
		return fmt.Sprintf("out=%q err=%s", o.Out, o.Err)
	}
	return fmt.Sprintf("out=%q", o.Out)
}

// pid gives the PID the observation carries, or 0 for a reading that is not one.
func (o panePIDObservation) pid() int {
	if o.Err != "" {
		return 0
	}
	pid, err := strconv.Atoi(o.Out)
	if err != nil || pid <= 0 {
		return 0
	}
	return pid
}

func observeSaverPanePID(sock *tmuxtest.Socket) panePIDObservation {
	out, err := sock.TryRun("list-panes", "-t", tmux.PortalSaverName, "-F", "#{pane_pid}")
	obs := panePIDObservation{Out: strings.TrimSpace(out)}
	if err != nil {
		obs.Err = err.Error()
	}
	return obs
}

func readSaverPanePID(t *testing.T, sock *tmuxtest.Socket) int {
	t.Helper()
	for range 2 {
		if pid := observeSaverPanePID(sock).pid(); pid > 0 {
			state.RegisterSandboxDaemon(pid)
			return pid
		}
		time.Sleep(pgrepConvergencePollTick)
	}
	t.Fatalf("saver pane PID unreadable after retry; saver pane may have died between setup and sweep")
	return 0
}

// pgrepObservation is comparable so the wait can tell a daemon population that
// is still moving from one that has stopped, and carries the enumeration error
// rather than discarding it so a red run says why the snapshot was empty.
type pgrepObservation struct {
	Count int
	Pids  string
	Err   string
}

func (o pgrepObservation) String() string {
	if o.Err != "" {
		return fmt.Sprintf("count=%d pids=%s err=%s", o.Count, o.Pids, o.Err)
	}
	return fmt.Sprintf("count=%d pids=%s", o.Count, o.Pids)
}

func waitForPgrepCount(t *testing.T, target int) harnesstest.ProgressResult[pgrepObservation] {
	t.Helper()
	return harnesstest.AwaitProgress(t, pgrepConvergenceWait, observePgrepDaemons,
		func(o pgrepObservation) bool { return o.Err == "" && o.Count == target })
}

func observePgrepDaemons() pgrepObservation {
	pids, err := portaltest.PgrepPortalDaemons()
	obs := pgrepObservation{Count: len(pids), Pids: fmt.Sprint(pids)}
	if err != nil {
		obs.Err = err.Error()
	}
	return obs
}

func pidAlive(pid int) bool {
	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	return !errors.Is(err, syscall.ESRCH)
}
