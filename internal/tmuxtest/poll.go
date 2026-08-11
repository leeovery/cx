package tmuxtest

import (
	"testing"
	"time"
)

// PollUntil invokes cond at the given tick cadence until it returns true or
// timeout elapses, reporting which happened. It never fails the test itself: the
// caller owns the diagnostics on timeout.
func PollUntil(t *testing.T, timeout, tick time.Duration, cond func() bool) bool {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(tick)
	}
}
