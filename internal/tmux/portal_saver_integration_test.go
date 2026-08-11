//go:build integration

package tmux_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const singletonRecycleTimeout = 5 * time.Second

const daemonPidPollInterval = 50 * time.Millisecond

func TestEnsurePortalSaverVersion_SingletonInvariantAcrossRecycle(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	// The staged binary must be on PATH: the tmux server exec's the saver pane's
	// `portal state daemon` shell-command, and without it no daemon ever starts.
	_ = portalbintest.StagePortalBinary(t)

	_, dir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", dir)
	portaltest.RegisterStateDirTeardownGuard(t, dir)

	sock := tmuxtest.New(t, "ptl-saver-")
	client := sock.Client()

	if err := tmux.EnsurePortalSaverVersion(client, dir, "v-test-1"); err != nil {
		t.Fatalf("first EnsurePortalSaverVersion: %v", err)
	}
	sock.WaitForSession(t, tmux.PortalSaverName, singletonRecycleTimeout)

	serverPID := captureTmuxServerPID(t, sock)

	priorPID := waitForLiveDaemon(t, dir, singletonRecycleTimeout)

	if err := state.WriteVersionFile(dir, "v-test-0-old", nil); err != nil {
		t.Fatalf("WriteVersionFile (force mismatch): %v", err)
	}

	if err := tmux.EnsurePortalSaverVersion(client, dir, "v-test-1"); err != nil {
		t.Fatalf("second EnsurePortalSaverVersion: %v", err)
	}
	sock.WaitForSession(t, tmux.PortalSaverName, singletonRecycleTimeout)

	currentPID := waitForNewLiveDaemon(t, dir, priorPID, singletonRecycleTimeout)

	if state.IsProcessAlive(priorPID) {
		dumpDiagnostics(t, dir, serverPID, priorPID, currentPID,
			"prior daemon (pid=%d) is still alive after recycle — singleton invariant violated", priorPID)
	}
	if !state.IsProcessAlive(currentPID) {
		dumpDiagnostics(t, dir, serverPID, priorPID, currentPID,
			"current daemon (pid=%d) is not alive after recycle — recycle failed to spawn replacement", currentPID)
	}

	// Re-capture: killing _portal-saver takes the socket's only session with it,
	// so the tmux server exits and is respawned. Keying the pgrep parent filter
	// on the stale pre-recycle PID would exit 1 and over-fail the assertion.
	postRecycleServerPID := captureTmuxServerPID(t, sock)
	count, raw := countDaemonChildren(t, postRecycleServerPID)
	if count != 1 {
		dumpDiagnostics(t, dir, postRecycleServerPID, priorPID, currentPID,
			"expected exactly 1 `portal state daemon` child of tmux server (pid=%d), got %d\npgrep raw output:\n%s",
			postRecycleServerPID, count, raw)
	}
}

func TestEnsurePortalSaverVersion_AliveAndVersionAbsent_NoKill(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	_ = portalbintest.StagePortalBinary(t)

	_, dir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", dir)
	portaltest.RegisterStateDirTeardownGuard(t, dir)

	sock := tmuxtest.New(t, "ptl-aliveabsent-")
	client := sock.Client()

	const currentVersion = "0.5.0-test"
	if err := tmux.EnsurePortalSaverVersion(client, dir, currentVersion); err != nil {
		t.Fatalf("initial EnsurePortalSaverVersion: %v", err)
	}
	sock.WaitForSession(t, tmux.PortalSaverName, singletonRecycleTimeout)

	priorPID := waitForLiveDaemon(t, dir, singletonRecycleTimeout)
	waitForVersionFile(t, dir, singletonRecycleTimeout)

	if err := os.Remove(state.DaemonVersion(dir)); err != nil {
		t.Fatalf("remove daemon.version: %v", err)
	}

	if err := tmux.EnsurePortalSaverVersion(client, dir, currentVersion); err != nil {
		t.Fatalf("EnsurePortalSaverVersion (alive+absent): %v", err)
	}

	if !client.HasSession(tmux.PortalSaverName) {
		t.Fatalf("_portal-saver session does not exist after EnsurePortalSaverVersion on alive+absent input")
	}

	stored, err := state.ReadVersionFile(dir)
	if err != nil {
		t.Fatalf("ReadVersionFile after defensive write: %v", err)
	}
	if stored != currentVersion {
		t.Fatalf("daemon.version contents = %q; want %q", stored, currentVersion)
	}

	currentPID, err := state.ReadPIDFile(dir)
	if err != nil {
		t.Fatalf("ReadPIDFile after action: %v", err)
	}
	if currentPID != priorPID {
		t.Fatalf("daemon PID changed: prior=%d current=%d (expected no respawn)",
			priorPID, currentPID)
	}

	if !state.DaemonAlive(dir) {
		t.Fatalf("DaemonAlive(%s) = false after action; want true", dir)
	}

	assertNoForbiddenLogSubstrings(t, dir)
}

func TestBootstrapPortalSaver_LockContention_CascadeChainReachable(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	_ = portalbintest.StagePortalBinary(t)

	_, dir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", dir)
	portaltest.RegisterStateDirTeardownGuard(t, dir)

	// The sentinel takes daemon.lock before BootstrapPortalSaver so the spawned
	// daemon is guaranteed to lose the race; its cleanup is registered before any
	// path that can t.Fatal so the fd never leaks into the next test.
	ready := make(chan struct{})
	var sentinelErr error
	var sentinelFile *os.File
	go func() {
		f, err := state.AcquireDaemonLock(dir)
		if err != nil {
			sentinelErr = err
			close(ready)
			return
		}
		sentinelFile = f
		close(ready)
	}()
	<-ready
	if sentinelErr != nil {
		t.Fatalf("sentinel AcquireDaemonLock: %v", sentinelErr)
	}
	if sentinelFile == nil {
		t.Fatal("sentinel returned nil *os.File without error")
	}
	t.Cleanup(func() {
		_ = sentinelFile.Close()
	})

	sock := tmuxtest.New(t, "ptl-cascade-")
	client := sock.Client()

	// A second, long-lived session keeps the server up once _portal-saver is
	// destroyed; otherwise tmux exits with its last session and SetSessionOption
	// below fails with "no server running" instead of "no such session".
	if err := client.NewDetachedSessionNoCwd("_cascade-holder", "sleep infinity"); err != nil {
		t.Fatalf("create holder session: %v", err)
	}

	// The error is deliberately ignored: if the lock-loser's pane exit destroys
	// the session before the internal SetSessionOption runs, Bootstrap returns
	// the cascade error itself — both outcomes are the behaviour under test.
	_ = tmux.BootstrapPortalSaver(client, dir)

	if !waitForDaemonNotAlive(t, dir, 1*time.Second, 50*time.Millisecond) {
		t.Fatalf("state.DaemonAlive(%s) did not return false within 1s "+
			"(lock-loser daemon should exit before writing daemon.pid or "+
			"exit shortly after writing it)", dir)
	}

	if !waitForSessionAbsent(t, client, tmux.PortalSaverName, 2*time.Second, 100*time.Millisecond) {
		t.Fatalf("_portal-saver session did not disappear within 2s " +
			"(pane process should have exited, destroying the session)")
	}

	err := client.SetSessionOption(tmux.PortalSaverName, "destroy-unattached", "off")
	if err == nil {
		t.Fatalf("SetSessionOption returned nil; expected error after session destruction")
	}
	msg := err.Error()
	if !strings.Contains(msg, "exit 1") {
		t.Fatalf("SetSessionOption error %q does not contain %q", msg, "exit 1")
	}
	if !strings.Contains(msg, "no such session") {
		t.Fatalf("SetSessionOption error %q does not contain %q", msg, "no such session")
	}
}

func waitForDaemonNotAlive(t *testing.T, dir string, timeout, tick time.Duration) bool {
	t.Helper()
	return tmuxtest.PollUntil(t, timeout, tick, func() bool {
		return !state.DaemonAlive(dir)
	})
}

func waitForSessionAbsent(t *testing.T, client *tmux.Client, name string, timeout, tick time.Duration) bool {
	t.Helper()
	return tmuxtest.PollUntil(t, timeout, tick, func() bool {
		return !client.HasSession(name)
	})
}

func waitForVersionFile(t *testing.T, dir string, timeout time.Duration) {
	t.Helper()
	if tmuxtest.PollUntil(t, timeout, daemonPidPollInterval, func() bool {
		_, err := state.ReadVersionFile(dir)
		return err == nil
	}) {
		return
	}
	t.Fatalf("daemon.version did not appear within %s (state dir=%s)", timeout, dir)
}

func assertNoForbiddenLogSubstrings(t *testing.T, dir string) {
	t.Helper()
	// portal.log only: these tests are too short-lived to rotate. A longer
	// scenario would need the rotated file scanned too.
	data, err := os.ReadFile(state.PortalLog(dir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		t.Fatalf("read portal.log: %v", err)
	}
	contents := string(data)
	forbidden := []string{
		"prior daemon (pid=",
		"another daemon holds the lock; exiting",
		"step 5 (EnsureSaver) failed:",
	}
	for _, sub := range forbidden {
		if strings.Contains(contents, sub) {
			t.Fatalf("portal.log contains forbidden substring %q\n--- portal.log ---\n%s",
				sub, contents)
		}
	}
}

func captureTmuxServerPID(t *testing.T, sock *tmuxtest.Socket) int {
	t.Helper()
	out := sock.Run(t, "display-message", "-p", "#{pid}")
	pid, err := strconv.Atoi(strings.TrimSpace(out))
	if err != nil {
		t.Fatalf("parse tmux server PID %q: %v", out, err)
	}
	return pid
}

func countDaemonChildren(t *testing.T, serverPID int) (int, string) {
	t.Helper()
	out, err := exec.Command("pgrep", "-P", strconv.Itoa(serverPID), "-f", "portal state daemon").Output()
	if err != nil {
		// pgrep exit 1 is "no matches", not a failure.
		var ee *exec.ExitError
		if errors.As(err, &ee) && ee.ExitCode() == 1 {
			return 0, ""
		}
		return 0, fmt.Sprintf("pgrep error: %v\nstderr/stdout: %s", err, string(out))
	}
	trimmed := strings.TrimSpace(string(out))
	if trimmed == "" {
		return 0, ""
	}
	return len(strings.Split(trimmed, "\n")), string(out)
}

func waitForLiveDaemon(t *testing.T, dir string, timeout time.Duration) int {
	t.Helper()
	var livePID int
	if tmuxtest.PollUntil(t, timeout, daemonPidPollInterval, func() bool {
		pid, err := state.ReadPIDFile(dir)
		if err == nil && state.IsProcessAlive(pid) {
			livePID = pid
			return true
		}
		return false
	}) {
		return livePID
	}
	t.Fatalf("daemon.pid did not point at a live process within %s "+
		"(state dir=%s)", timeout, dir)
	return 0
}

func waitForNewLiveDaemon(t *testing.T, dir string, prior int, timeout time.Duration) int {
	t.Helper()
	var newPID int
	if tmuxtest.PollUntil(t, timeout, daemonPidPollInterval, func() bool {
		pid, err := state.ReadPIDFile(dir)
		if err == nil && pid != prior && state.IsProcessAlive(pid) {
			newPID = pid
			return true
		}
		return false
	}) {
		return newPID
	}
	t.Fatalf("daemon.pid did not converge on a new live PID (prior=%d) "+
		"within %s (state dir=%s)", prior, timeout, dir)
	return 0
}

func dumpDiagnostics(t *testing.T, dir string, serverPID, priorPID, currentPID int, format string, args ...any) {
	t.Helper()
	var b strings.Builder

	fmt.Fprintf(&b, "tmux server PID: %d\n", serverPID)
	fmt.Fprintf(&b, "prior daemon PID: %d (alive=%v)\n",
		priorPID, state.IsProcessAlive(priorPID))
	fmt.Fprintf(&b, "current daemon PID: %d (alive=%v)\n",
		currentPID, state.IsProcessAlive(currentPID))

	if pidData, err := os.ReadFile(filepath.Join(dir, "daemon.pid")); err == nil {
		fmt.Fprintf(&b, "daemon.pid contents: %q\n", string(pidData))
	} else {
		fmt.Fprintf(&b, "daemon.pid read error: %v\n", err)
	}
	if verData, err := os.ReadFile(filepath.Join(dir, "daemon.version")); err == nil {
		fmt.Fprintf(&b, "daemon.version contents: %q\n", string(verData))
	} else {
		fmt.Fprintf(&b, "daemon.version read error: %v\n", err)
	}

	// Scoped by -P to the test's own server so even this read-only diagnostic
	// never enumerates the developer's live daemon.
	out, _ := exec.Command("pgrep", "-P", strconv.Itoa(serverPID), "-fl", "portal state daemon").CombinedOutput()
	fmt.Fprintf(&b, "pgrep -P %d -fl 'portal state daemon':\n%s", serverPID, string(out))

	t.Fatalf(format+"\n\nDiagnostics:\n%s", append(args, b.String())...)
}
