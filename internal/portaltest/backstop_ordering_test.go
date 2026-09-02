package portaltest

import (
	"os"
	"path/filepath"
	"testing"
)

// captureBackstop swaps the backstop installer for the duration of one test and
// hands back the arguments IsolateStateForTest passed it.
func captureBackstop(t *testing.T) (dir *string, pre *map[string]Fingerprint) {
	t.Helper()

	var gotDir string
	var gotPre map[string]Fingerprint

	prior := installBackstop
	installBackstop = func(_ backstopT, devStateDir string, snapshot map[string]Fingerprint) {
		gotDir = devStateDir
		gotPre = snapshot
	}
	t.Cleanup(func() { installBackstop = prior })

	return &gotDir, &gotPre
}

func TestIsolateStateForTest_ResolvesDevStateDirUnderScrubbedHome(t *testing.T) {
	hostConfig := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", hostConfig)
	hostStateDir := filepath.Join(hostConfig, "portal", "state")

	gotDir, _ := captureBackstop(t)

	IsolateStateForTest(t)

	want := filepath.Join(os.Getenv("HOME"), ".config", "portal", "state")
	if *gotDir != want {
		t.Errorf("dev state dir resolved to %q, want the scrubbed HOME's %q", *gotDir, want)
	}
	if *gotDir == hostStateDir {
		t.Errorf("dev state dir resolved to the host install %q — the resolution ran before the scrub", hostStateDir)
	}
}

func TestIsolateStateForTest_RegistersBackstopOverResolvedDir(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())

	gotDir, gotPre := captureBackstop(t)

	IsolateStateForTest(t)

	fake := &fakeBackstopT{}
	installBackstopCleanup(fake, *gotDir, *gotPre)

	if err := os.MkdirAll(*gotDir, 0o700); err != nil {
		t.Fatalf("mkdir resolved dev state dir: %v", err)
	}
	writeFile(t, filepath.Join(*gotDir, "leaked.json"), "leak")

	fake.runCleanups()

	if !hasDelta(fake.errorfs, "leaked.json", "created") {
		t.Errorf("expected the backstop over %q to flag leaked.json:created; got: %v", *gotDir, fake.errorfs)
	}
}
