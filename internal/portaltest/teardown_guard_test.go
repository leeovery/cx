package portaltest

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"
)

// cleanupRecorder stands in for *testing.T at the guard's registration seam,
// holding the cleanups so a test can run them on demand instead of at test end.
type cleanupRecorder struct {
	fns []func()
}

func (c *cleanupRecorder) Cleanup(fn func()) { c.fns = append(c.fns, fn) }

func (c *cleanupRecorder) runLIFO() {
	for _, fn := range slices.Backward(c.fns) {
		fn()
	}
}

func TestTeardownGuardWaitsOutCallerSuppliedSaverPID(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stateDir, "sessions.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("seed sessions.json: %v", err)
	}

	const writerLifetime = 300 * time.Millisecond
	writer := exec.Command("sleep", "0.3")
	if err := writer.Start(); err != nil {
		t.Fatalf("start stand-in writer: %v", err)
	}
	reaped := make(chan struct{})
	go func() {
		_ = writer.Wait()
		close(reaped)
	}()
	t.Cleanup(func() { <-reaped })

	pid := writer.Process.Pid
	rec := &cleanupRecorder{}
	registerTeardownGuard(rec, stateDir, func() (int, bool) { return pid, true })

	start := time.Now()
	rec.runLIFO()
	elapsed := time.Since(start)

	if elapsed < writerLifetime-teardownGuardPollTick {
		t.Fatalf("guard returned after %s; expected it to wait out the caller-supplied pid %d "+
			"living for %s", elapsed, pid, writerLifetime)
	}
}

func TestTeardownGuardReturnsOnceStateDirStopsChanging(t *testing.T) {
	stateDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(stateDir, "sessions.json"), []byte("{}"), 0o600); err != nil {
		t.Fatalf("seed sessions.json: %v", err)
	}

	rec := &cleanupRecorder{}
	registerTeardownGuard(rec, stateDir, func() (int, bool) { return 0, false })

	start := time.Now()
	rec.runLIFO()
	elapsed := time.Since(start)

	if elapsed >= teardownGuardBudget {
		t.Fatalf("guard burned its whole %s budget over a quiescent state dir (took %s)",
			teardownGuardBudget, elapsed)
	}
}

// captureHomeQuiescenceGuard swaps the HOME-quiescence registrar for one test
// and hands back the directory IsolateStateForTest passed it.
func captureHomeQuiescenceGuard(t *testing.T) *string {
	t.Helper()

	var gotDir string
	prior := registerHomeQuiescenceGuard
	registerHomeQuiescenceGuard = func(_ cleanupT, dir string) { gotDir = dir }
	t.Cleanup(func() { registerHomeQuiescenceGuard = prior })

	return &gotDir
}

func TestIsolateStateForTest_RegistersAQuiescenceWaitOverTheTempHome(t *testing.T) {
	gotDir := captureHomeQuiescenceGuard(t)

	IsolateStateForTest(t)

	if want := os.Getenv("HOME"); *gotDir != want {
		t.Errorf("quiescence wait registered over %q, want the temp HOME %q", *gotDir, want)
	}
}

func TestDirQuiescenceGuardReturnsOnceTheDirStopsChanging(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".zsh_sessions"), []byte("x"), 0o600); err != nil {
		t.Fatalf("seed shell session file: %v", err)
	}

	rec := &cleanupRecorder{}
	registerDirQuiescenceGuard(rec, dir)

	start := time.Now()
	rec.runLIFO()
	elapsed := time.Since(start)

	if elapsed >= teardownGuardBudget {
		t.Fatalf("guard burned its whole %s budget over a quiescent dir (took %s)",
			teardownGuardBudget, elapsed)
	}
}
