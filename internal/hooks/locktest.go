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

// SetSnapshotLockTimeoutForTest sets the sweep's advisory pre-read bound for the
// duration of the test, so a test can raise it to make that read contend.
func SetSnapshotLockTimeoutForTest(t *testing.T, d time.Duration) {
	t.Helper()
	prev := snapshotLockTimeout
	snapshotLockTimeout = d
	t.Cleanup(func() { snapshotLockTimeout = prev })
}
