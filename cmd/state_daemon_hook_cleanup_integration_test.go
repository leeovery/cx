//go:build integration

package cmd_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
	"github.com/leeovery/portal/internal/transienttest"
)

const (
	// hookCleanupIntervalMirror duplicates the unexported cmd.hookCleanupInterval
	// (this file is package cmd_test). Drift cannot false-fail: the observation
	// budget below is this value plus generous slack.
	hookCleanupIntervalMirror = 10 * time.Second

	daemonReadyBudget = 3 * time.Second
	daemonReadyPoll   = 50 * time.Millisecond

	hookCleanupObservationBudget = hookCleanupIntervalMirror + 15*time.Second
	hookCleanupPollTick          = 250 * time.Millisecond

	// preIntervalSafetyCeiling: past this much elapsed wall time the
	// no-reap-before-interval window cannot be established, so that single
	// sub-assertion is skipped rather than false-failed.
	preIntervalSafetyCeiling = hookCleanupIntervalMirror - 2*time.Second

	// staleHookKey has no matching live pane on the test server, and its token
	// shape is one the reaper can judge, so it is genuinely reapable.
	staleHookKey = "gonXxX"

	liveWorkSession = "work"
)

func TestDaemon_ThrottledHookCleanup_ReapsStaleRetainsLiveOnIdleServer(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	// PATH-prepends the built binary so the tmux-server-respawned `portal state
	// daemon` resolves its bare argv-0.
	binDir := portalbintest.StagePortalBinary(t)
	if _, err := exec.LookPath("portal"); err != nil {
		t.Skipf("portal not on PATH after build+prepend; skipping: %v", err)
	}

	// Must be set BEFORE IsolateStateForTest: the TestMain poison points
	// PORTAL_HOOKS_FILE at a nonexistent path, and IsolateStateForTest scrubs
	// XDG_CONFIG_HOME but not this var, so the env slice it derives would carry
	// the poison instead of a writable path.
	hooksPath := filepath.Join(t.TempDir(), "portal", "hooks.json")
	t.Setenv("PORTAL_HOOKS_FILE", hooksPath)

	env, stateDir := portaltest.IsolateStateForTest(t)

	// Set on the test process so the tmux server started below — and the
	// saver-pane daemon it hosts — inherits them.
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	t.Setenv("PORTAL_HOOKS_FILE", hooksPath)
	t.Setenv("PORTAL_LOG_LEVEL", "INFO")
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	sock := tmuxtest.New(t, "ptl-daemon-hookclean-")
	client := sock.Client()

	// Read the live key back from tmux rather than assuming it, so the seeded
	// entry matches exactly what the daemon's ListAllPanes enumerates.
	sock.Run(t, "new-session", "-d", "-s", liveWorkSession, "sh", "-c", "exec tail -f /dev/null")
	liveHookKey := strings.TrimSpace(sock.Run(t, "list-panes",
		"-t", liveWorkSession, "-F", tmux.StructuralKeyFormat))
	if liveHookKey == "" {
		t.Fatalf("could not read live pane structural key for session %q", liveWorkSession)
	}
	if liveHookKey == staleHookKey {
		t.Fatalf("test setup collision: live key %q equals the stale key constant", liveHookKey)
	}

	transienttest.SeedHooksJSON(t, env, map[string]string{
		staleHookKey: "echo stale-should-be-reaped",
		liveHookKey:  "echo live-should-be-retained",
	})

	// Guards against a seed that silently landed on the wrong path.
	preKeys := readHookKeys(t, env)
	if _, ok := preKeys[staleHookKey]; !ok {
		t.Fatalf("pre-spawn: stale key %q absent after seed; keys=%v\n"+
			"  hooks.json path resolution mismatch — seed did not land where the daemon reads",
			staleHookKey, sortedKeys(preKeys))
	}
	if _, ok := preKeys[liveHookKey]; !ok {
		t.Fatalf("pre-spawn: live key %q absent after seed; keys=%v",
			liveHookKey, sortedKeys(preKeys))
	}

	// Registered after tmuxtest.New so LIFO SIGHUPs the daemon before
	// kill-server and before the isolated tempdir goes away.
	t.Cleanup(func() {
		_, _ = sock.TryRun("kill-session", "-t", tmux.PortalSaverName)
	})

	// The daemon must be hosted AS the _portal-saver pane, not exec.Command'd:
	// its saver-membership probe ejects a process that is not the saver pane
	// after ~3 ticks, well before the ~10s cleanup interval this test observes.
	// t0 is a lower bound on daemon start (it starts inside this call).
	t0 := time.Now()
	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver: %v\n--- portal.log ---\n%s",
			err, portaltest.ReadPortalLogSafe(stateDir))
	}

	if !tmuxtest.PollUntil(t, daemonReadyBudget, daemonReadyPoll, func() bool {
		return state.DaemonAlive(stateDir)
	}) {
		t.Fatalf("daemon did not become alive within %s of BootstrapPortalSaver return\n"+
			"--- portal.log ---\n%s", daemonReadyBudget, portaltest.ReadPortalLogSafe(stateDir))
	}

	pidData, err := os.ReadFile(state.DaemonPID(stateDir))
	if err != nil {
		t.Fatalf("read daemon.pid: %v\n--- portal.log ---\n%s",
			err, portaltest.ReadPortalLogSafe(stateDir))
	}
	daemonPID, err := strconv.Atoi(strings.TrimSpace(string(pidData)))
	if err != nil {
		t.Fatalf("parse daemon.pid contents %q: %v", string(pidData), err)
	}

	// Fail fast on a binding mismatch: the daemon would self-eject within ~3
	// ticks and the reap assertion below would time out with an opaque cause.
	panePIDStr := strings.TrimSpace(sock.Run(t, "list-panes",
		"-t", tmux.PortalSaverName, "-F", "#{pane_pid}"))
	panePID, err := strconv.Atoi(panePIDStr)
	if err != nil {
		t.Fatalf("parse _portal-saver pane pid %q: %v", panePIDStr, err)
	}
	if daemonPID != panePID {
		t.Fatalf("structural-binding divergence: daemon.pid (%d) != _portal-saver pane pid (%d)\n"+
			"  the daemon must BE the saver pane process or Component D self-supervision "+
			"ejects it before the ~10s cleanup interval\n--- portal.log ---\n%s",
			daemonPID, panePID, portaltest.ReadPortalLogSafe(stateDir))
	}
	t.Logf("daemon alive as _portal-saver pane (pid=%d); live key=%q, stale key=%q",
		daemonPID, liveHookKey, staleHookKey)

	elapsedA := time.Since(t0)
	earlyKeys := readHookKeys(t, env)
	if elapsedA >= preIntervalSafetyCeiling {
		t.Logf("slow host: %s already elapsed since daemon-start lower bound (>= %s); "+
			"skipping the no-reap-before-interval sub-assertion (reap + retain below still pin behaviour)",
			elapsedA, preIntervalSafetyCeiling)
	} else {
		if _, ok := earlyKeys[staleHookKey]; !ok {
			t.Fatalf("stale key %q reaped only %s after daemon start (< interval %s); "+
				"lastCleanup must be anchored to daemon-START so the first cleanup fires "+
				"~one interval later, not immediately\n--- portal.log ---\n%s",
				staleHookKey, elapsedA, hookCleanupIntervalMirror, portaltest.ReadPortalLogSafe(stateDir))
		}
		if _, ok := earlyKeys[liveHookKey]; !ok {
			t.Fatalf("live key %q missing %s after daemon start (before any cleanup)\n"+
				"--- portal.log ---\n%s",
				liveHookKey, elapsedA, portaltest.ReadPortalLogSafe(stateDir))
		}
		t.Logf("no-reap-before-interval confirmed: both keys present at %s after daemon start", elapsedA)
	}

	// The server must stay idle throughout: the cleanup gate lives on the
	// daemon tick's idle branch, so anything making a tick dirty skips it.
	reaped := tmuxtest.PollUntil(t, hookCleanupObservationBudget, hookCleanupPollTick, func() bool {
		_, present := readHookKeys(t, env)[staleHookKey]
		return !present
	})
	if !reaped {
		finalKeys := readHookKeys(t, env)
		t.Fatalf("stale key %q was NOT reaped within %s of daemon start on an idle server\n"+
			"  the daemon's throttled (~%s) idle-branch "+
			"cleanup MUST reap entries whose paneKey is not in the live pane set\n"+
			"  remaining hooks.json keys: %v\n"+
			"--- hooks.json (%s) ---\n%s\n--- portal.log ---\n%s",
			staleHookKey, hookCleanupObservationBudget, hookCleanupIntervalMirror,
			sortedKeys(finalKeys), hooksPath, string(transienttest.HooksJSONBytes(t, env)),
			portaltest.ReadPortalLogSafe(stateDir))
	}
	t.Logf("stale key %q reaped after the throttle interval on the idle server", staleHookKey)

	postKeys := readHookKeys(t, env)
	if _, ok := postKeys[liveHookKey]; !ok {
		t.Fatalf("live key %q was removed by the cleanup; CleanStale must RETAIN entries "+
			"whose paneKey is present in the live pane set (ListAllPanes)\n"+
			"  remaining keys: %v\n--- hooks.json (%s) ---\n%s\n--- portal.log ---\n%s",
			liveHookKey, sortedKeys(postKeys), hooksPath,
			string(transienttest.HooksJSONBytes(t, env)), portaltest.ReadPortalLogSafe(stateDir))
	}
	if _, ok := postKeys[staleHookKey]; ok {
		t.Fatalf("stale key %q reappeared after reap; keys=%v", staleHookKey, sortedKeys(postKeys))
	}
	t.Logf("live key %q retained after stale reap; final keys=%v", liveHookKey, sortedKeys(postKeys))
}

// readHookKeys reads the isolated hooks.json and returns its structural keys, or
// an empty set when the file is missing. The daemon's writes are atomic, so a
// concurrent read never sees a partial file.
func readHookKeys(t *testing.T, env []string) map[string]struct{} {
	t.Helper()
	raw := transienttest.HooksJSONBytes(t, env)
	keys := make(map[string]struct{})
	if len(raw) == 0 {
		return keys
	}
	var m map[string]map[string]string
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal hooks.json: %v\n--- raw ---\n%s", err, string(raw))
	}
	for k := range m {
		keys[k] = struct{}{}
	}
	return keys
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	slices.Sort(out)
	return out
}
