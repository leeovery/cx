package restoretest

import (
	"os"
	"testing"
	"time"
)

// fataller lets a unit test drive the timeout branch with an in-memory fake
// instead of aborting the test process.
type fataller interface {
	Helper()
	Name() string
	Fatalf(format string, args ...any)
}

// WaitForFileExists polls path every tick until it exists or budget elapses,
// failing the test on timeout. tick is mandatory: the call sites it replaced
// disagreed on cadence, so the caller must be explicit.
func WaitForFileExists(t *testing.T, path string, budget, tick time.Duration) {
	t.Helper()
	waitForFileExists(t, path, budget, tick)
}

func waitForFileExists(t fataller, path string, budget, tick time.Duration) {
	t.Helper()
	deadline := time.Now().Add(budget)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(path); err == nil {
			return
		}
		time.Sleep(tick)
	}
	t.Fatalf("WaitForFileExists(%s): %s did not appear within %v", t.Name(), path, budget)
}
