package harnesstest

import "time"

// PollUntil invokes cond at the tick cadence until it returns true or timeout
// elapses, reporting which happened. It never fails the test itself.
func PollUntil(t TestingT, timeout, tick time.Duration, cond func() bool) bool {
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
