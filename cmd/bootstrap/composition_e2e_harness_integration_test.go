//go:build integration

package bootstrap_test

import (
	"fmt"
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

const compositeUserSessionCount = 2

const compositeUserSessionPrefix = "u"

const compositeUserSessionSeedScript = `while sleep 0.1; do echo "hello $RANDOM"; done`

const compositePreStatePGrepTimeout = 3 * time.Second

type compositeHarness struct {
	Env []string

	StateDir string

	Sock *tmuxtest.Socket

	Client *tmux.Client

	LegitimateDaemonPID int

	Orphan1PID int

	Orphan2PID int

	UserSessionNames []string
}

// saverPaneProbe reads the PID of the live _portal-saver pane, remembering the
// last one it saw. Its readers want different things from a failed read, so they
// take different methods: livePID reports only what is live right now, and pid
// falls back to the remembered one.
type saverPaneProbe struct {
	sock *tmuxtest.Socket
	seen int
}

// observe records a PID read while the server was live.
func (p *saverPaneProbe) observe(pid int) { p.seen = pid }

// pid is for the teardown wait, which runs after kill-server when there is no
// server left to ask — so the remembered PID is what it actually reads.
func (p *saverPaneProbe) pid() (int, bool) {
	if pid, ok := p.livePID(); ok {
		return pid, true
	}
	return p.seen, p.seen > 0
}

// livePID is for the sandbox ownership source, which decides which PIDs an
// orphan sweep may SIGKILL: it must never name a PID this test no longer owns,
// so a failed read yields nothing rather than the remembered one. The saver is
// killed and respawned by the kill-barrier, and the read fails across that gap.
func (p *saverPaneProbe) livePID() (int, bool) {
	if p.sock == nil {
		return 0, false
	}
	out, err := p.sock.TryRun("list-panes", "-t", tmux.PortalSaverName, "-F", "#{pane_pid}")
	if err != nil {
		return 0, false
	}
	pid, convErr := strconv.Atoi(strings.TrimSpace(out))
	if convErr != nil || pid <= 0 {
		return 0, false
	}
	p.seen = pid
	return pid, true
}

// TestSaverPaneProbe_OwnershipReadNeverYieldsRememberedPID guards the divergence
// between the probe's two reads against a later edit collapsing them into one.
func TestSaverPaneProbe_OwnershipReadNeverYieldsRememberedPID(t *testing.T) {
	const remembered = 4242

	// A nil socket is the standing-in shape of a read that cannot answer: the
	// same branch a killed or respawning server takes.
	probe := &saverPaneProbe{seen: remembered}

	if pid, ok := probe.livePID(); ok {
		t.Errorf("livePID() = (%d, true) with no live read available; it feeds the default-deny "+
			"allowlist that decides which PIDs an orphan sweep may SIGKILL, so it must never "+
			"name a remembered PID this test may no longer own", pid)
	}

	pid, ok := probe.pid()
	if !ok || pid != remembered {
		t.Errorf("pid() = (%d, %v), want (%d, true); the teardown wait runs after kill-server, "+
			"so the remembered PID is the only one it can read", pid, ok, remembered)
	}
}

func setupCompositeHarness(t *testing.T) *compositeHarness {
	t.Helper()

	tmuxtest.SkipIfNoTmux(t)
	skipIfNoPgrep(t)
	// Stage before IsolateStateForTest: that helper re-points HOME at a temp
	// dir, and `go build` would then populate an unremovable module cache there.
	_ = portalbintest.StagePortalBinary(t)

	envSlice, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	// LIFO runs this wait between kill-server and the TempDir RemoveAll. Its pid
	// comes from the probe rather than daemon.pid, which a step below overwrites
	// with an orphan's.
	saver := &saverPaneProbe{}
	portaltest.RegisterStateDirTeardownGuardWithPIDSource(t, stateDir, saver.pid)

	sock := tmuxtest.New(t, "ptl-comp-e2e-")
	saver.sock = sock
	client := sock.Client()

	userSessionNames := seedUserSessions(t, client, compositeUserSessionCount)

	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver (legitimate saver): %v", err)
	}
	legitimateDaemonPID := waitForSaverPanePID(t, sock)
	saver.observe(legitimateDaemonPID)
	waitForDaemonPID(t, stateDir, legitimateDaemonPID)

	// Own the saver by its live pane PID, not daemon.pid: the step below
	// overwrites daemon.pid with an orphan's, and a PID-file-based source would
	// lose sight of the saver at that moment.
	state.RegisterSandboxDaemonSource(saver.livePID)

	orphan1, orphan1StateDir := portaltest.SpawnIsolatedDaemon(t, envSlice, sock.SocketPath())
	waitForDaemonPID(t, orphan1StateDir, orphan1.Process.Pid)

	if err := state.WritePIDFile(stateDir, orphan1.Process.Pid); err != nil {
		t.Fatalf("overwrite legitimate daemon.pid with orphan1 PID: %v", err)
	}

	orphan2, _ := portaltest.SpawnIsolatedDaemon(t, envSlice, sock.SocketPath())

	assertCompositePreState(t, stateDir, sock, legitimateDaemonPID,
		orphan1.Process.Pid, orphan2.Process.Pid)

	return &compositeHarness{
		Env:                 envSlice,
		StateDir:            stateDir,
		Sock:                sock,
		Client:              client,
		LegitimateDaemonPID: legitimateDaemonPID,
		Orphan1PID:          orphan1.Process.Pid,
		Orphan2PID:          orphan2.Process.Pid,
		UserSessionNames:    userSessionNames,
	}
}

func assertCompositePreState(t *testing.T, stateDir string, sock *tmuxtest.Socket,
	legitimateDaemonPID, orphan1PID, orphan2PID int,
) {
	t.Helper()

	if !waitForPgrepCount(t, 3, compositePreStatePGrepTimeout) {
		pids, _ := portaltest.PgrepPortalDaemons()
		t.Fatalf("harness pre-state: pgrep -fx did not reach 3 within %s\n"+
			"  legitimate saver PID: %d (alive=%v)\n"+
			"  orphan1 PID: %d (alive=%v)\n"+
			"  orphan2 PID: %d (alive=%v)\n"+
			"  pgrep snapshot: %v\n"+
			"  hint: a daemon may have exited before pre-state assertion — harness is broken",
			compositePreStatePGrepTimeout,
			legitimateDaemonPID, pidAlive(legitimateDaemonPID),
			orphan1PID, pidAlive(orphan1PID),
			orphan2PID, pidAlive(orphan2PID),
			pids)
	}

	recordedPID, err := state.ReadPIDFile(stateDir)
	if err != nil {
		t.Fatalf("harness pre-state: read legitimate daemon.pid: %v", err)
	}
	if recordedPID != orphan1PID {
		t.Fatalf("harness pre-state: legitimate daemon.pid = %d; want orphan1 PID = %d\n"+
			"  legitimate saver PID: %d\n"+
			"  orphan2 PID: %d\n"+
			"  the daemon.pid overwrite step did not produce the reporter-scenario shape",
			recordedPID, orphan1PID, legitimateDaemonPID, orphan2PID)
	}

	if orphan1PID == legitimateDaemonPID {
		t.Fatalf("harness pre-state: orphan1 PID == saver pane PID == %d\n"+
			"  orphans MUST differ from the saver-pane process for the scenario to fire",
			orphan1PID)
	}
	if orphan2PID == legitimateDaemonPID {
		t.Fatalf("harness pre-state: orphan2 PID == saver pane PID == %d\n"+
			"  orphans MUST differ from the saver-pane process for the scenario to fire",
			orphan2PID)
	}

	if !pidAlive(orphan1PID) {
		t.Fatalf("harness pre-state: orphan1 PID %d not alive at pre-state assertion\n"+
			"  hint: orphan subprocess exited during harness setup", orphan1PID)
	}
	if !pidAlive(orphan2PID) {
		t.Fatalf("harness pre-state: orphan2 PID %d not alive at pre-state assertion\n"+
			"  hint: orphan subprocess exited during harness setup", orphan2PID)
	}

	currentSaverPID := readSaverPanePID(t, sock)
	if currentSaverPID != legitimateDaemonPID {
		t.Fatalf("harness pre-state: saver pane PID changed during setup\n"+
			"  setup-time PID: %d\n"+
			"  current PID: %d\n"+
			"  hint: the saver pane may have been respawned — harness assumptions invalid",
			legitimateDaemonPID, currentSaverPID)
	}
}

func seedUserSessions(t *testing.T, client *tmux.Client, count int) []string {
	t.Helper()
	names := make([]string, 0, count)
	for i := 0; i < count; i++ {
		name := fmt.Sprintf("%s%d", compositeUserSessionPrefix, i)
		shellCmd := fmt.Sprintf("sh -c %q", compositeUserSessionSeedScript)
		if err := client.NewSessionWithCommand(name, "", shellCmd); err != nil {
			t.Fatalf("seed user session %q: %v", name, err)
		}
		names = append(names, name)
	}
	return names
}

func TestCompositeHarness_PreState(t *testing.T) {
	h := setupCompositeHarness(t)

	if h.LegitimateDaemonPID <= 0 {
		t.Fatalf("h.LegitimateDaemonPID = %d; want > 0", h.LegitimateDaemonPID)
	}
	if h.Orphan1PID <= 0 {
		t.Fatalf("h.Orphan1PID = %d; want > 0", h.Orphan1PID)
	}
	if h.Orphan2PID <= 0 {
		t.Fatalf("h.Orphan2PID = %d; want > 0", h.Orphan2PID)
	}
	if h.Orphan1PID == h.LegitimateDaemonPID {
		t.Fatalf("h.Orphan1PID == h.LegitimateDaemonPID == %d; PIDs must be distinct", h.Orphan1PID)
	}
	if h.Orphan2PID == h.LegitimateDaemonPID {
		t.Fatalf("h.Orphan2PID == h.LegitimateDaemonPID == %d; PIDs must be distinct", h.Orphan2PID)
	}
	if h.Orphan1PID == h.Orphan2PID {
		t.Fatalf("h.Orphan1PID == h.Orphan2PID == %d; orphan PIDs must be distinct", h.Orphan1PID)
	}

	recordedPID, err := state.ReadPIDFile(h.StateDir)
	if err != nil {
		t.Fatalf("read legitimate daemon.pid: %v", err)
	}
	if recordedPID != h.Orphan1PID {
		t.Fatalf("legitimate daemon.pid = %d; want h.Orphan1PID = %d\n"+
			"  h.LegitimateDaemonPID = %d, h.Orphan2PID = %d",
			recordedPID, h.Orphan1PID, h.LegitimateDaemonPID, h.Orphan2PID)
	}

	pids, err := portaltest.PgrepPortalDaemons()
	if err != nil {
		t.Fatalf("pgrep snapshot: %v", err)
	}
	if len(pids) != 3 {
		t.Fatalf("pgrep -fx returned %d daemons, want 3: %v\n"+
			"  h.LegitimateDaemonPID = %d (alive=%v)\n"+
			"  h.Orphan1PID = %d (alive=%v)\n"+
			"  h.Orphan2PID = %d (alive=%v)",
			len(pids), pids,
			h.LegitimateDaemonPID, pidAlive(h.LegitimateDaemonPID),
			h.Orphan1PID, pidAlive(h.Orphan1PID),
			h.Orphan2PID, pidAlive(h.Orphan2PID))
	}

	if !pidAlive(h.Orphan1PID) {
		t.Fatalf("h.Orphan1PID %d not alive at consumer-observation time", h.Orphan1PID)
	}
	if !pidAlive(h.Orphan2PID) {
		t.Fatalf("h.Orphan2PID %d not alive at consumer-observation time", h.Orphan2PID)
	}

	if len(h.UserSessionNames) != compositeUserSessionCount {
		t.Fatalf("len(h.UserSessionNames) = %d; want %d",
			len(h.UserSessionNames), compositeUserSessionCount)
	}
	out, err := h.Sock.TryRun("list-sessions", "-F", "#{session_name}")
	if err != nil {
		t.Fatalf("list-sessions on isolated socket: %v\n%s", err, out)
	}
	listed := strings.Split(strings.TrimSpace(out), "\n")
	for _, want := range h.UserSessionNames {
		found := false
		for _, got := range listed {
			if strings.TrimSpace(got) == want {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("seeded user session %q not present in list-sessions output\n"+
				"  list-sessions output: %v", want, listed)
		}
	}
}
