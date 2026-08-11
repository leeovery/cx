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

const endStateReadyTimeout = 2 * time.Second

const endStatePollTick = 50 * time.Millisecond

const lockLoserCascadeWindow = 2500 * time.Millisecond

func TestBootstrapPortalSaver_CleanBootstrap_EndState(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	_ = portalbintest.StagePortalBinary(t)

	_, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-cleanboot-")
	client := sock.Client()

	if client.HasSession(tmux.PortalSaverName) {
		t.Fatalf("pre-condition failed: %s present on fresh tmux server", tmux.PortalSaverName)
	}

	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver: %v", err)
	}

	if !client.HasSession(tmux.PortalSaverName) {
		t.Fatalf("HasSession(%s) = false; want true after BootstrapPortalSaver",
			tmux.PortalSaverName)
	}

	opt := sock.Run(t, "show-options", "-t", tmux.PortalSaverName, "destroy-unattached")
	if !strings.Contains(opt, "off") {
		t.Fatalf("show-options destroy-unattached = %q; want substring %q", opt, "off")
	}

	var lastArgs string
	if !tmuxtest.PollUntil(t, endStateReadyTimeout, endStatePollTick, func() bool {
		out, err := sock.TryRun("list-panes", "-t", tmux.PortalSaverName, "-F", "#{pane_pid}")
		if err != nil {
			return false
		}
		pidStr := strings.TrimSpace(out)
		if pidStr == "" {
			return false
		}
		pid, perr := strconv.Atoi(pidStr)
		if perr != nil {
			return false
		}
		args, perr := psArgsForPID(pid)
		if perr != nil {
			return false
		}
		lastArgs = args
		return strings.Contains(args, "portal state daemon")
	}) {
		t.Fatalf("pane process did not converge on `portal state daemon` within %s; last ps args = %q",
			endStateReadyTimeout, lastArgs)
	}

	// Deliberately not folded into the poll above: a substring match could pass
	// on a transient state where both command lines are briefly visible.
	if strings.Contains(lastArgs, "tail -f /dev/null") {
		t.Fatalf("pane process is still the placeholder `tail -f /dev/null`; ps args = %q",
			lastArgs)
	}

	assertNoNoSuchSessionEntries(t, stateDir)
}

func TestBootstrapPortalSaver_LockLoser_NoNoSuchSessionLogNoise(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	_ = portalbintest.StagePortalBinary(t)

	envSlice, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-lockloser-")
	client := sock.Client()

	// A second, never-exiting session keeps the server up once _portal-saver is
	// destroyed; otherwise tmux tears the server down too and later calls fail
	// with "no server running" instead of the cascade this test detects.
	if err := client.NewDetachedSessionNoCwd(
		"ptl-keepalive", "sh -c 'exec tail -f /dev/null'",
	); err != nil {
		t.Fatalf("create keepalive dummy session: %v", err)
	}

	// Spawned by hand rather than through portaltest.SpawnIsolatedDaemon: the
	// seeded daemon must hold daemon.lock under the SAME stateDir the respawned
	// daemon will try to acquire, and that helper forces a per-call tempdir.
	seededEnv := append([]string{}, envSlice...)
	seededEnv = append(seededEnv, "PORTAL_STATE_DIR="+stateDir)
	seeded := exec.Command("portal", "state", "daemon")
	seeded.Env = seededEnv
	if startErr := seeded.Start(); startErr != nil {
		t.Fatalf("start seeded competing daemon: %v", startErr)
	}
	portaltest.RegisterSubprocessCleanup(t, seeded)

	if !tmuxtest.PollUntil(t, endStateReadyTimeout, endStatePollTick, func() bool {
		pid, readErr := state.ReadPIDFile(stateDir)
		if readErr != nil {
			return false
		}
		result, idErr := state.IdentifyDaemon(pid)
		if idErr != nil {
			return false
		}
		return result == state.IdentifyIsPortalDaemon
	}) {
		t.Fatalf("seeded competing daemon did not become observable within %s "+
			"(state dir=%s)", endStateReadyTimeout, stateDir)
	}

	bootstrapErr := tmux.BootstrapPortalSaver(client, stateDir)

	// A flat sleep, not a poll: the assertion is the ABSENCE of a substring, so
	// there is nothing to poll for — only a window to let late writes land.
	time.Sleep(lockLoserCascadeWindow)

	assertNoNoSuchSessionEntries(t, stateDir)

	if bootstrapErr != nil {
		if strings.Contains(bootstrapErr.Error(), "no such session: "+tmux.PortalSaverName) {
			t.Fatalf("BootstrapPortalSaver returned the load-bearing cascade error: %v\n"+
				"This indicates the create-then-set-option-then-respawn ordering of "+
				"Component F (Task 3-2) has regressed: SetSessionOption ran against "+
				"a session that was destroyed by an immediately-exiting lock-loser daemon",
				bootstrapErr)
		}
		t.Fatalf("BootstrapPortalSaver returned unexpected error %v; want nil "+
			"(SetSessionOption ran against the live placeholder pane; readiness "+
			"barrier WARN-swallows its timeout)", bootstrapErr)
	}
}

func TestBootstrapPortalSaver_EnvironmentInheritanceAcrossRespawn(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	_ = portalbintest.StagePortalBinary(t)

	_, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-envparity-")
	client := sock.Client()

	const baselineName = "_env-baseline"
	if err := client.NewDetachedSessionNoCwd(
		baselineName, "sh -c 'exec tail -f /dev/null'",
	); err != nil {
		t.Fatalf("create baseline session: %v", err)
	}

	baselineRaw, err := sock.TryRun("show-environment", "-t", baselineName)
	if err != nil {
		t.Fatalf("show-environment baseline: %v\n%s", err, baselineRaw)
	}
	baseline := parseShowEnvironmentKeys(baselineRaw, "XDG_CONFIG_HOME", "HOME", "PATH")

	// Kill the baseline before bootstrapping so the saver's create path runs
	// against the same session population it would in production.
	if err := client.KillSession(baselineName); err != nil {
		t.Fatalf("kill baseline session: %v", err)
	}

	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver: %v", err)
	}

	observedRaw, err := sock.TryRun("show-environment", "-t", tmux.PortalSaverName)
	if err != nil {
		t.Fatalf("show-environment %s: %v\n%s", tmux.PortalSaverName, err, observedRaw)
	}
	observed := parseShowEnvironmentKeys(observedRaw, "XDG_CONFIG_HOME", "HOME", "PATH")

	for _, key := range []string{"XDG_CONFIG_HOME", "HOME", "PATH"} {
		if baseline[key] != observed[key] {
			t.Fatalf("environment-inheritance parity violated for key %q\n"+
				"  baseline: %s\n"+
				"  observed: %s\n"+
				"--- full baseline map (3 keys) ---\n%s\n"+
				"--- full observed map (3 keys) ---\n%s\n"+
				"--- raw show-environment %s ---\n%s\n"+
				"--- raw show-environment %s ---\n%s",
				key,
				baseline[key],
				observed[key],
				dumpEnvMap(baseline),
				dumpEnvMap(observed),
				baselineName, baselineRaw,
				tmux.PortalSaverName, observedRaw,
			)
		}
	}
}

type envValue struct {
	unset bool
	value string
}

func (v envValue) String() string {
	if v.unset {
		return "(unset)"
	}
	return strconv.Quote(v.value)
}

// show-environment emits "NAME=value" for a set entry and "-NAME" for one
// explicitly removed from the session environment; the two must stay distinct
// so "both unset" compares equal without collapsing into the empty-value case.
func parseShowEnvironmentKeys(raw string, keys ...string) map[string]envValue {
	want := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		want[k] = struct{}{}
	}
	out := map[string]envValue{}
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimRight(line, "\r")
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "-") {
			name := line[1:]
			if _, ok := want[name]; ok {
				out[name] = envValue{unset: true}
			}
			continue
		}
		before, after, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		name := before
		val := after
		if _, ok := want[name]; ok {
			out[name] = envValue{value: val}
		}
	}
	return out
}

func dumpEnvMap(m map[string]envValue) string {
	var b strings.Builder
	for _, k := range []string{"XDG_CONFIG_HOME", "HOME", "PATH"} {
		v, ok := m[k]
		if !ok {
			fmt.Fprintf(&b, "  %s: (absent)\n", k)
			continue
		}
		fmt.Fprintf(&b, "  %s: %s\n", k, v)
	}
	return b.String()
}

// Deliberately not state.IdentifyDaemon: the assertion is on the byte-level
// shape an operator sees from `ps -o args= -p <pid>`.
func psArgsForPID(pid int) (string, error) {
	out, err := exec.Command("ps", "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", fmt.Errorf("ps -p %d: %w", pid, err)
	}
	return strings.TrimSpace(string(out)), nil
}

func assertNoNoSuchSessionEntries(t *testing.T, stateDir string) {
	t.Helper()
	data, err := os.ReadFile(state.PortalLog(stateDir))
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return
		}
		t.Fatalf("read portal.log: %v", err)
	}
	const forbidden = "no such session: _portal-saver"
	contents := string(data)
	if strings.Contains(contents, forbidden) {
		t.Fatalf("portal.log contains forbidden substring %q\n--- portal.log (path=%s) ---\n%s",
			forbidden, filepath.Join(stateDir, "portal.log"), contents)
	}
}
