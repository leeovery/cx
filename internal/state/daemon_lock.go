package state

import (
	"errors"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/unix"
)

// ErrDaemonLockHeld reports that another process already holds the advisory
// lock on <stateDir>/daemon.lock. Callers match it with errors.Is to separate
// expected contention from genuine open(2)/flock failures, which are fatal.
var ErrDaemonLockHeld = errors.New("daemon.lock held by another process")

var lockAcquire = unix.Flock

var lockAcquireReadPIDFile = ReadPIDFile

var lockAcquireIdentifyDaemon = IdentifyDaemon

var lockAcquireFstat = unix.Fstat

var lockAcquireStat = unix.Stat

const lockAcquireInodeRetryAttempts = 3

const lockAcquireInodeRetrySleep = 10 * time.Millisecond

// AcquireDaemonLock takes an exclusive, non-blocking advisory lock on
// <stateDir>/daemon.lock, enforcing at most one daemon per state directory.
//
// The caller must retain the returned *os.File for the daemon's whole
// lifetime — letting it be finalized closes the fd, releasing the kernel lock
// and reopening the race. Contention returns ErrDaemonLockHeld; every other
// failure is a wrapped error. stateDir is never created.
func AcquireDaemonLock(stateDir string) (*os.File, error) {
	path := DaemonLock(stateDir)

	for attempt := 1; attempt <= lockAcquireInodeRetryAttempts; attempt++ {
		// flock is per-inode: an unlinked-and-recreated lock file would let two
		// daemons lock different inodes, so the pid check runs first, every attempt.
		if preAcquirePIDIdentifiesLiveDaemon(stateDir) {
			return nil, ErrDaemonLockHeld
		}

		f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, 0o600)
		if err != nil {
			return nil, fmt.Errorf("open daemon.lock %s: %w", path, err)
		}

		if err := lockAcquire(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
			_ = f.Close()
			if errors.Is(err, unix.EWOULDBLOCK) {
				return nil, ErrDaemonLockHeld
			}
			return nil, fmt.Errorf("flock daemon.lock %s: %w", path, err)
		}

		// An unlink+recreate between the open and the flock leaves us locking a
		// detached inode, which a second daemon can then bypass.
		var fdStat unix.Stat_t
		if err := lockAcquireFstat(int(f.Fd()), &fdStat); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("fstat daemon.lock %s: %w", path, err)
		}
		var pathStat unix.Stat_t
		if err := lockAcquireStat(path, &pathStat); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("stat daemon.lock %s: %w", path, err)
		}

		if fdStat.Ino != pathStat.Ino {
			_ = f.Close()
			if attempt < lockAcquireInodeRetryAttempts {
				time.Sleep(lockAcquireInodeRetrySleep)
				continue
			}
			return nil, fmt.Errorf("daemon.lock %s inode mismatch after %d attempts: fd inode != path inode", path, lockAcquireInodeRetryAttempts)
		}

		if _, err := unix.FcntlInt(f.Fd(), unix.F_SETFD, unix.FD_CLOEXEC); err != nil {
			_ = f.Close()
			return nil, fmt.Errorf("set FD_CLOEXEC on daemon.lock %s: %w", path, err)
		}

		return f, nil
	}

	// Unreachable: the final iteration returns rather than continuing.
	return nil, fmt.Errorf("daemon.lock %s inode mismatch after %d attempts: fd inode != path inode", path, lockAcquireInodeRetryAttempts)
}

func preAcquirePIDIdentifiesLiveDaemon(stateDir string) bool {
	// Every failure shape fails open, so legitimate succession is never blocked;
	// the flock EWOULDBLOCK path is the fallback for real contention.
	pid, err := lockAcquireReadPIDFile(stateDir)
	if err != nil {
		return false
	}

	result, idErr := lockAcquireIdentifyDaemon(pid)
	if idErr != nil {
		return false
	}
	return result == IdentifyIsPortalDaemon
}
