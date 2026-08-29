package hooks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/unix"
)

// ErrLockHeld separates expected contention on the sidecar lock from a genuine
// open(2)/flock failure, so a caller can tell a timeout from a save failure.
var ErrLockHeld = errors.New("hooks lock held by another process")

// lockTimeout bounds every mutation and ordinary read. An unbounded acquire
// would park the daemon's tick loop behind a holder that is alive but stuck, so
// waiting is bounded well above the sub-millisecond critical section and well
// inside the sweep's own cadence.
var lockTimeout = 2 * time.Second

// snapshotLockTimeout bounds the clean's advisory pre-read alone. A clean takes
// the sidecar twice — shared for that pre-read, exclusive for the deletion — so
// at lockTimeout a writer that is alive but stuck would park the daemon's 1s
// tick for 4s every cycle, which is the stall the bound exists to prevent. The
// pre-read is advisory: it may degrade to an unlocked read at no cost to
// correctness, so it is bounded at the cheapest figure that still grants an
// uncontended lock — 20ms, four poll intervals above the sub-millisecond
// critical section.
var snapshotLockTimeout = 20 * time.Millisecond

var lockPollInterval = 5 * time.Millisecond

// lockPath is the sidecar the lock is taken on. It is never hooks.json itself:
// AtomicWrite renames a temp file over the target, so a lock on the target is a
// lock on an unlinked inode the moment anyone writes.
func (s *Store) lockPath() string {
	return s.path + ".lock"
}

// acquireLock opens path with openFlags and polls for the flockMode lock until
// it is granted or bound elapses, returning ErrLockHeld on expiry. The returned
// file must be closed by the caller, which releases the lock.
func acquireLock(path string, openFlags, flockMode int, bound time.Duration) (*os.File, error) {
	f, err := os.OpenFile(path, openFlags, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open hooks lock: %w", err)
	}

	deadline := time.Now().Add(bound)
	for {
		err := unix.Flock(int(f.Fd()), flockMode|unix.LOCK_NB)
		if err == nil {
			return f, nil
		}
		if !errors.Is(err, unix.EWOULDBLOCK) {
			_ = f.Close()
			return nil, fmt.Errorf("flock hooks lock %s: %w", path, err)
		}
		if !time.Now().Before(deadline) {
			_ = f.Close()
			return nil, fmt.Errorf("%w: %s", ErrLockHeld, path)
		}
		time.Sleep(lockPollInterval)
	}
}

// acquireMutationLock takes the exclusive hold a whole load-mutate-save runs
// under. The config directory is created first because the sidecar cannot be
// created inside a directory that does not exist; its error is deliberately
// discarded so an uncreatable directory fails through the acquire below, on the
// one branch that reports a lock failure.
func (s *Store) acquireMutationLock() (*os.File, error) {
	_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
	return acquireLock(s.lockPath(), os.O_RDWR|os.O_CREATE, unix.LOCK_EX, lockTimeout)
}

// acquireSharedLock takes the shared hold a read runs under, at the bound its
// caller supplies. It passes no O_CREATE deliberately: a read must create
// nothing — not the config directory, not the sidecar — so a read-only `portal
// doctor` and a display-only `hook list` keep having no write side effect on a
// fresh install, and an absent sidecar degrades through the caller instead.
func (s *Store) acquireSharedLock(bound time.Duration) (*os.File, error) {
	return acquireLock(s.lockPath(), os.O_RDONLY, unix.LOCK_SH, bound)
}
