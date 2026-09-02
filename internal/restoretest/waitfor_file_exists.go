package restoretest

import (
	"os"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/harnesstest"
)

// WaitForFileExists fails the test on timeout. tick is mandatory so every
// caller states its own polling cadence.
func WaitForFileExists(t *testing.T, path string, budget, tick time.Duration) {
	t.Helper()
	waitForFileExists(t, path, budget, tick)
}

func waitForFileExists(t harnesstest.NamingT, path string, budget, tick time.Duration) {
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
