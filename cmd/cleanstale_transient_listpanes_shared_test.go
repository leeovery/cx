//go:build integration

package cmd

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/transienttest"
)

// XDG_CONFIG_HOME must be re-pushed onto the test process because
// IsolateStateForTest deliberately scrubs it there and injects the isolated
// value only into the returned env slice — but the doctor --fix prune runs
// in-process, so its config resolution would miss the seeded hooks.json.
// Re-pushing after IsolateStateForTest is safe: the backstop has already
// snapshotted its baseline.
func isolateCleanStaleTestEnv(t *testing.T) (env []string, stateDir string) {
	t.Helper()
	// IsolateStateForTest scrubs XDG_CONFIG_HOME but not PORTAL_HOOKS_FILE, so
	// TestMain's poisoned /nonexistent value would survive into the derived env
	// slice and the seeder would try to mkdir it. Must be set before the call.
	t.Setenv("PORTAL_HOOKS_FILE", filepath.Join(t.TempDir(), "portal", "hooks.json"))
	env, stateDir = portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)
	t.Setenv("PORTAL_LOG_LEVEL", "debug")
	t.Setenv("XDG_CONFIG_HOME", configDirFromEnvSlice(t, env))

	// Stands in for main's log.Init so the in-process prune's breadcrumbs land
	// in the isolated state dir's portal.log; the handler swap is bracketed so
	// it does not leak into sibling subtests.
	initTestLogToStateDirAs(t, stateDir, "test", "clean")
	return env, stateDir
}

type transientModeSpec struct {
	name        string
	mode        transienttest.FailureMode
	invoke      func(t *testing.T, env []string, stateDir string) (output string, err error)
	extraAssert func(t *testing.T, output string, seededKeys []string)
}

// The count is load-bearing: the log-fingerprint needles below match on
// entries=3. The keys are token-shaped so it is the transient-failure handling,
// not the reaper's retention of an unjudgeable shape, that spares them.
var transientModeSeedEntries = map[string]string{
	reapableSeedA: "echo a",
	reapableSeedB: "echo b",
	reapableSeedC: "echo c",
}

func runTransientCleanStaleModeSubtest(t *testing.T, spec transientModeSpec) {
	t.Helper()

	env, stateDir := isolateCleanStaleTestEnv(t)

	transienttest.SeedHooksJSON(t, env, transientModeSeedEntries)
	before := transienttest.HooksJSONBytes(t, env)
	if len(before) == 0 {
		t.Fatalf("precondition: hooksJSONBytes returned empty slice after seed (subtest %s)", spec.name)
	}

	output, err := spec.invoke(t, env, stateDir)
	if err != nil {
		t.Fatalf("entry point returned error under %s; want nil (Warn-and-swallow contract): %v\n  output:\n%s",
			spec.name, err, output)
	}

	after := transienttest.HooksJSONBytes(t, env)
	if !bytes.Equal(before, after) {
		t.Fatalf("hooks.json mutated under %s — the wipe regression has returned\n"+
			"  before: %s\n"+
			"  after:  %s",
			spec.name, before, after)
	}

	seededKeys := make([]string, 0, len(transientModeSeedEntries))
	for k := range transientModeSeedEntries {
		seededKeys = append(seededKeys, k)
	}
	if spec.extraAssert != nil {
		spec.extraAssert(t, output, seededKeys)
	}

	fullLog := portaltest.ReadPortalLogSafe(stateDir)
	lines := staleHookCleanupLogLines(fullLog)

	switch spec.mode {
	case transienttest.FailExitNonZero:
		// A failed read is a stand-down like any other, so it rides the shared
		// `clean-stale-skipped` shape rather than the sweep's prose lines.
		standDown := logLinesContaining(fullLog, "clean-stale-skipped")
		if !containsLineMatching(standDown, "WARN hooks:", "reason=pane-read-failed", "simulated transient") {
			t.Fatalf("missing mode (a) stand-down Warn under %s; want a `WARN hooks:` line containing `reason=pane-read-failed` and `simulated transient`\n"+
				"  matched stand-down lines:\n%s",
				spec.name, strings.Join(standDown, "\n"))
		}
		// The entry-point Debug must be absent: the failed enumeration declines
		// before there is any count to report.
		for _, line := range lines {
			if strings.Contains(line, "stale-hook cleanup counts") {
				t.Fatalf("mode (a) emitted entry-point Debug (`stale-hook cleanup counts ...`) under %s; must be absent — a failed enumeration has no counts to report\n"+
					"  offending line: %s",
					spec.name, line)
			}
		}
	case transienttest.FailEmptyStdout:
		if len(lines) == 0 {
			t.Fatalf("no `stale-hook cleanup:` lines found in portal.log under %s; want at least one\n"+
				"  full log:\n%s",
				spec.name, fullLog)
		}
		if !containsLineMatching(lines, "stale-hook cleanup counts", "panes=0", "entries=3") {
			t.Fatalf("missing mode (b) entry-point Debug under %s; want a `stale-hook cleanup counts` line containing `panes=0` and `entries=3`\n"+
				"  matched stale-hook lines:\n%s",
				spec.name, strings.Join(lines, "\n"))
		}
		// The guard's own WARN rides the shared stand-down shape, so it is
		// keyed off `clean-stale-skipped` rather than the sweep's prose lines.
		standDown := logLinesContaining(fullLog, "clean-stale-skipped")
		if !containsLineMatching(standDown, "WARN hooks:", "reason=empty-pane-read", "entries=3") {
			t.Fatalf("missing mode (b) hazard-guard Warn under %s; want a `WARN hooks:` stand-down line containing `reason=empty-pane-read` and `entries=3`\n"+
				"  matched stand-down lines:\n%s",
				spec.name, strings.Join(standDown, "\n"))
		}
	default:
		t.Fatalf("runTransientCleanStaleModeSubtest: unsupported FailureMode %v for subtest %s — driver supports only FailExitNonZero / FailEmptyStdout",
			spec.mode, spec.name)
	}
}

func configDirFromEnvSlice(t *testing.T, env []string) string {
	t.Helper()
	const key = "XDG_CONFIG_HOME="
	for _, e := range env {
		if strings.HasPrefix(e, key) {
			return strings.TrimPrefix(e, key)
		}
	}
	t.Fatalf("configDirFromEnvSlice: XDG_CONFIG_HOME not present in env slice — IsolateStateForTest contract regression")
	return ""
}

// The prefix deliberately omits the trailing colon so it captures both the
// colon-suffixed Warn lines and the no-colon Debug ones.
func staleHookCleanupLogLines(portalLog string) []string {
	return logLinesContaining(portalLog, "stale-hook cleanup")
}

func logLinesContaining(portalLog, needle string) []string {
	var matches []string
	for _, line := range strings.Split(portalLog, "\n") {
		if strings.Contains(line, needle) {
			matches = append(matches, line)
		}
	}
	return matches
}

func containsLineMatching(lines []string, needles ...string) bool {
	for _, line := range lines {
		matched := true
		for _, n := range needles {
			if !strings.Contains(line, n) {
				matched = false
				break
			}
		}
		if matched {
			return true
		}
	}
	return false
}
