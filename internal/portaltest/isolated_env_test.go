package portaltest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portaltest"
)

// HOME and XDG_CONFIG_HOME are redirected for the whole binary so the backstop
// under test targets a hermetic temp dir: on a machine with a live `portal state
// daemon` it would otherwise race that daemon's tick writes and flag them.
func TestMain(m *testing.M) {
	sandbox, err := os.MkdirTemp("", "portaltest-self-sandbox-*")
	if err != nil {
		panic("portaltest: mkdir sandbox: " + err.Error())
	}
	defer func() { _ = os.RemoveAll(sandbox) }()

	_ = os.Setenv("HOME", sandbox)
	_ = os.Setenv("XDG_CONFIG_HOME", filepath.Join(sandbox, "config"))

	os.Exit(m.Run())
}

func envValue(env []string, key string) (string, bool) {
	prefix := key + "="
	for _, e := range env {
		if after, ok := strings.CutPrefix(e, prefix); ok {
			return after, true
		}
	}
	return "", false
}

func envCount(env []string, key string) int {
	prefix := key + "="
	n := 0
	for _, e := range env {
		if strings.HasPrefix(e, prefix) {
			n++
		}
	}
	return n
}

func TestSetsXDGConfigHomeInsideTempDir(t *testing.T) {
	env, _ := portaltest.IsolateStateForTest(t)

	got, ok := envValue(env, "XDG_CONFIG_HOME")
	if !ok {
		t.Fatalf("XDG_CONFIG_HOME absent from returned env")
	}
	if filepath.Base(got) != "config" {
		t.Fatalf("XDG_CONFIG_HOME does not end in /config: %q", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("XDG_CONFIG_HOME path does not exist: %v", err)
	}
}

func TestRemovesPreExistingXDGConfigHome(t *testing.T) {
	decoy := "/decoy/should/not/leak"
	t.Setenv("XDG_CONFIG_HOME", decoy)

	env, _ := portaltest.IsolateStateForTest(t)

	if got := envCount(env, "XDG_CONFIG_HOME"); got != 1 {
		t.Fatalf("expected exactly 1 XDG_CONFIG_HOME entry, got %d", got)
	}
	got, _ := envValue(env, "XDG_CONFIG_HOME")
	if got == decoy {
		t.Fatalf("XDG_CONFIG_HOME leaked decoy value %q", decoy)
	}
	if strings.Contains(got, decoy) {
		t.Fatalf("XDG_CONFIG_HOME contains decoy fragment %q in %q", decoy, got)
	}
}

func TestRemovesEmptyPreExistingXDGConfigHome(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")

	env, _ := portaltest.IsolateStateForTest(t)

	if got := envCount(env, "XDG_CONFIG_HOME"); got != 1 {
		t.Fatalf("expected exactly 1 XDG_CONFIG_HOME entry, got %d", got)
	}
	got, _ := envValue(env, "XDG_CONFIG_HOME")
	if got == "" {
		t.Fatalf("XDG_CONFIG_HOME is empty; helper did not replace inherited empty value")
	}
}

// HOME is deliberately not preserved: the helper re-points it at a fresh
// tempdir.
func TestPreservesPath(t *testing.T) {
	wantPath := os.Getenv("PATH")

	env, _ := portaltest.IsolateStateForTest(t)

	gotPath, okPath := envValue(env, "PATH")
	if !okPath {
		t.Fatalf("PATH missing from returned env")
	}
	if gotPath != wantPath {
		t.Errorf("PATH mutated: got %q want %q", gotPath, wantPath)
	}
}

func TestNeutralizesHomeAndXDGConfigHome(t *testing.T) {
	priorHome := os.Getenv("HOME")

	_, _ = portaltest.IsolateStateForTest(t)

	gotHome := os.Getenv("HOME")
	if gotHome == priorHome {
		t.Fatalf("HOME not scrubbed: still %q (helper must re-point at a fresh tempdir)", gotHome)
	}
	if gotHome == "" {
		t.Fatalf("HOME scrubbed to empty; helper must point at a tempdir, not unset")
	}
	if got := os.Getenv("XDG_CONFIG_HOME"); got != "" {
		t.Fatalf("XDG_CONFIG_HOME not cleared on test process env: got %q", got)
	}
}

func TestStateDirUnderXDGConfigHome(t *testing.T) {
	env, stateDir := portaltest.IsolateStateForTest(t)

	xdg, ok := envValue(env, "XDG_CONFIG_HOME")
	if !ok {
		t.Fatalf("XDG_CONFIG_HOME absent")
	}
	want := filepath.Join(xdg, "portal", "state")
	if stateDir != want {
		t.Fatalf("stateDir mismatch: got %q want %q", stateDir, want)
	}
	info, err := os.Stat(stateDir)
	if err != nil {
		t.Fatalf("stateDir not on disk: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("stateDir is not a directory: %s", stateDir)
	}
}

func TestEnvUsableAsExecCmdEnv(t *testing.T) {
	env, _ := portaltest.IsolateStateForTest(t)
	wantXDG, _ := envValue(env, "XDG_CONFIG_HOME")

	cmd := exec.Command("sh", "-c", "echo $XDG_CONFIG_HOME")
	cmd.Env = env
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("sh exec: %v", err)
	}
	got := strings.TrimSpace(string(out))
	if got != wantXDG {
		t.Fatalf("subprocess saw XDG_CONFIG_HOME=%q, want %q", got, wantXDG)
	}
}

func TestDistinctStateDirPerCall(t *testing.T) {
	var a, b string
	t.Run("first", func(t *testing.T) {
		_, a = portaltest.IsolateStateForTest(t)
	})
	t.Run("second", func(t *testing.T) {
		_, b = portaltest.IsolateStateForTest(t)
	})
	if a == "" || b == "" {
		t.Fatalf("empty stateDir(s): a=%q b=%q", a, b)
	}
	if a == b {
		t.Fatalf("expected distinct stateDirs across subtests, both got %q", a)
	}
}

func TestConfigDirPermissions(t *testing.T) {
	env, _ := portaltest.IsolateStateForTest(t)
	configDir, _ := envValue(env, "XDG_CONFIG_HOME")
	info, err := os.Stat(configDir)
	if err != nil {
		t.Fatalf("stat configDir: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("configDir perm = %#o, want %#o", perm, 0o700)
	}
}
