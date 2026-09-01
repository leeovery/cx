package hooks

import (
	"testing"
	"time"
)

// SetLockTimeoutForTest lowers the mutation and read acquisition bound for the
// duration of the test, so a test can drive a timeout without waiting out the
// production figure.
func SetLockTimeoutForTest(t *testing.T, d time.Duration) {
	t.Helper()
	prev := lockTimeout
	lockTimeout = d
	t.Cleanup(func() { lockTimeout = prev })
}

// SnapshotLockBoundForTest reports the sweep's advisory pre-read bound as it is
// derived from the current lockTimeout. There is no setter: the bound has no
// value of its own to set, and a test that needs it elsewhere moves it by
// lowering the mutation bound, which is the derivation under test.
func SnapshotLockBoundForTest(t *testing.T) time.Duration {
	t.Helper()
	return snapshotLockBound()
}
