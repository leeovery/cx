package restoretest

import (
	"os"
	"testing"
	"time"
)

// fataller lets a unit test drive the timeout branch without aborting itself.
type fataller interface {
	Helper()
	Name() string
	Fatalf(format string, args ...any)
}

// WaitForFileExists fails the test on timeout. tick is mandatory so every
// caller states its own polling cadence.
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
