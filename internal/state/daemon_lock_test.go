package state

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func withLockAcquireFake(t *testing.T, fake func(fd int, how int) error) {
	t.Helper()
	prev := lockAcquire
	lockAcquire = fake
	t.Cleanup(func() { lockAcquire = prev })
}

func withLockAcquireIdentifyDaemonFake(t *testing.T, fake func(pid int) (IdentifyResult, error)) {
	t.Helper()
	prev := lockAcquireIdentifyDaemon
	lockAcquireIdentifyDaemon = fake
	t.Cleanup(func() { lockAcquireIdentifyDaemon = prev })
}

func withLockAcquireReadPIDFileFake(t *testing.T, fake func(dir string) (int, error)) {
	t.Helper()
	prev := lockAcquireReadPIDFile
	lockAcquireReadPIDFile = fake
	t.Cleanup(func() { lockAcquireReadPIDFile = prev })
}

func TestAcquireDaemonLock_ReturnsErrDaemonLockHeldOnEWOULDBLOCK(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireFake(t, func(_ int, _ int) error {
		return unix.EWOULDBLOCK
	})

	f, err := AcquireDaemonLock(dir)
	if f != nil {
		_ = f.Close()
		t.Fatalf("expected nil *os.File on contention, got %v", f)
	}
	if !errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; want errors.Is ErrDaemonLockHeld", err)
	}
}

func TestAcquireDaemonLock_WrapsNonEWOULDBLOCKFlockErrors(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireFake(t, func(_ int, _ int) error {
		return unix.EBADF
	})

	f, err := AcquireDaemonLock(dir)
	if f != nil {
		_ = f.Close()
		t.Fatalf("expected nil *os.File on non-EWOULDBLOCK flock error, got %v", f)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; must NOT be ErrDaemonLockHeld for non-EWOULDBLOCK", err)
	}
	if !errors.Is(err, unix.EBADF) {
		t.Fatalf("err = %v; expected wrapped unix.EBADF", err)
	}
}

func TestAcquireDaemonLock_WrapsOpenErrorWhenStateDirMissing(t *testing.T) {
	withLockAcquireFake(t, func(_ int, _ int) error {
		t.Fatal("lockAcquire must not be called when open fails")
		return nil
	})

	missing := filepath.Join(t.TempDir(), "does-not-exist")

	f, err := AcquireDaemonLock(missing)
	if f != nil {
		_ = f.Close()
		t.Fatalf("expected nil *os.File on open error, got %v", f)
	}
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; must NOT be ErrDaemonLockHeld for open(2) failure", err)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("err = %v; expected wrapped os.ErrNotExist", err)
	}
}

func TestAcquireDaemonLock_CreatesLockFileWithMode0600(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })

	f, err := AcquireDaemonLock(dir)
	if err != nil {
		t.Fatalf("AcquireDaemonLock: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	path := DaemonLock(dir)
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat lock file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("lock file mode = %o; want %o", got, 0o600)
	}
}

func TestAcquireDaemonLock_SetsFDCLOEXEC(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })

	f, err := AcquireDaemonLock(dir)
	if err != nil {
		t.Fatalf("AcquireDaemonLock: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	flags, err := unix.FcntlInt(f.Fd(), unix.F_GETFD, 0)
	if err != nil {
		t.Fatalf("F_GETFD: %v", err)
	}
	if flags&unix.FD_CLOEXEC == 0 {
		t.Errorf("FD_CLOEXEC not set on returned fd; flags = %#x", flags)
	}
}

func TestAcquireDaemonLock_DoesNotCreateStateDirIfMissing(t *testing.T) {
	withLockAcquireFake(t, func(_ int, _ int) error {
		t.Fatal("lockAcquire must not be called when open fails")
		return nil
	})

	parent := t.TempDir()
	missing := filepath.Join(parent, "missing-state-dir")

	_, err := AcquireDaemonLock(missing)
	if err == nil {
		t.Fatal("expected error when stateDir does not exist, got nil")
	}

	if _, statErr := os.Stat(missing); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("stateDir was created; stat err = %v; want os.ErrNotExist", statErr)
	}
}

// Deliberately drives the real unix.Flock syscall against a real lockfile, no
// lockAcquire seam: the lock-release-on-fd-close property is a kernel one.
func TestAcquireDaemonLock_KernelReleasesOnFDClose(t *testing.T) {
	stateDir := t.TempDir()

	f1, err := AcquireDaemonLock(stateDir)
	if err != nil {
		t.Fatalf("first AcquireDaemonLock: %v", err)
	}
	if f1 == nil {
		t.Fatal("first AcquireDaemonLock returned nil *os.File")
	}

	f2, err := AcquireDaemonLock(stateDir)
	if f2 != nil {
		_ = f2.Close()
		t.Fatalf("second AcquireDaemonLock returned non-nil *os.File while lock held")
	}
	if !errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("second AcquireDaemonLock err = %v; want errors.Is ErrDaemonLockHeld", err)
	}

	if err := f1.Close(); err != nil {
		t.Fatalf("close f1: %v", err)
	}

	f3, err := AcquireDaemonLock(stateDir)
	if err != nil {
		t.Fatalf("third AcquireDaemonLock after f1.Close: %v", err)
	}
	if f3 == nil {
		t.Fatal("third AcquireDaemonLock returned nil *os.File")
	}
	t.Cleanup(func() { _ = f3.Close() })
}

func TestAcquireDaemonLock_PreCheck_PIDFileAbsent_Proceeds(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireIdentifyDaemonFake(t, func(pid int) (IdentifyResult, error) {
		t.Fatalf("lockAcquireIdentifyDaemon must not be called when daemon.pid is absent; got pid=%d", pid)
		return 0, nil
	})
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })

	f, err := AcquireDaemonLock(dir)
	if err != nil {
		t.Fatalf("AcquireDaemonLock: %v", err)
	}
	if f == nil {
		t.Fatal("AcquireDaemonLock returned nil *os.File")
	}
	t.Cleanup(func() { _ = f.Close() })
}

func TestAcquireDaemonLock_PreCheck_DeadPID_Proceeds(t *testing.T) {
	dir := t.TempDir()
	if err := WritePIDFile(dir, 99999); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	identifyCalled := false
	withLockAcquireIdentifyDaemonFake(t, func(pid int) (IdentifyResult, error) {
		identifyCalled = true
		if pid != 99999 {
			t.Errorf("lockAcquireIdentifyDaemon pid = %d; want 99999", pid)
		}
		return IdentifyDead, nil
	})
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })

	f, err := AcquireDaemonLock(dir)
	if err != nil {
		t.Fatalf("AcquireDaemonLock: %v", err)
	}
	if f == nil {
		t.Fatal("AcquireDaemonLock returned nil *os.File")
	}
	t.Cleanup(func() { _ = f.Close() })

	if !identifyCalled {
		t.Errorf("lockAcquireIdentifyDaemon was not called")
	}
}

func TestAcquireDaemonLock_PreCheck_LivePortalDaemon_ReturnsErrDaemonLockHeld(t *testing.T) {
	dir := t.TempDir()
	if err := WritePIDFile(dir, 4242); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	withLockAcquireIdentifyDaemonFake(t, func(pid int) (IdentifyResult, error) {
		if pid != 4242 {
			t.Errorf("lockAcquireIdentifyDaemon pid = %d; want 4242", pid)
		}
		return IdentifyIsPortalDaemon, nil
	})
	withLockAcquireFake(t, func(_ int, _ int) error {
		t.Fatal("lockAcquire must not be called when pre-check identifies a live portal daemon")
		return nil
	})

	f, err := AcquireDaemonLock(dir)
	if f != nil {
		_ = f.Close()
		t.Fatalf("expected nil *os.File on pre-check refusal, got %v", f)
	}
	if !errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; want errors.Is ErrDaemonLockHeld", err)
	}

	if _, statErr := os.Stat(DaemonLock(dir)); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("daemon.lock exists after pre-check refusal; stat err = %v; want os.ErrNotExist", statErr)
	}
}

func TestAcquireDaemonLock_PreCheck_LiveNonPortalPID_Proceeds(t *testing.T) {
	dir := t.TempDir()
	if err := WritePIDFile(dir, 5151); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	withLockAcquireIdentifyDaemonFake(t, func(pid int) (IdentifyResult, error) {
		return IdentifyNotPortalDaemon, nil
	})
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })

	f, err := AcquireDaemonLock(dir)
	if err != nil {
		t.Fatalf("AcquireDaemonLock: %v", err)
	}
	if f == nil {
		t.Fatal("AcquireDaemonLock returned nil *os.File")
	}
	t.Cleanup(func() { _ = f.Close() })
}

func TestAcquireDaemonLock_PreCheck_TransientIdentifyError_Proceeds(t *testing.T) {
	dir := t.TempDir()
	if err := WritePIDFile(dir, 6262); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	withLockAcquireIdentifyDaemonFake(t, func(pid int) (IdentifyResult, error) {
		return 0, fmt.Errorf("transient ps failure")
	})
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })

	f, err := AcquireDaemonLock(dir)
	if err != nil {
		t.Fatalf("AcquireDaemonLock: %v", err)
	}
	if f == nil {
		t.Fatal("AcquireDaemonLock returned nil *os.File")
	}
	t.Cleanup(func() { _ = f.Close() })
}

func TestAcquireDaemonLock_PreCheck_ReadPIDFileNonAbsentError_Proceeds(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireReadPIDFileFake(t, func(d string) (int, error) {
		return 0, fmt.Errorf("parse daemon.pid: malformed")
	})
	withLockAcquireIdentifyDaemonFake(t, func(pid int) (IdentifyResult, error) {
		t.Fatalf("lockAcquireIdentifyDaemon must not be called when ReadPIDFile errors; got pid=%d", pid)
		return 0, nil
	})
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })

	f, err := AcquireDaemonLock(dir)
	if err != nil {
		t.Fatalf("AcquireDaemonLock: %v", err)
	}
	if f == nil {
		t.Fatal("AcquireDaemonLock returned nil *os.File")
	}
	t.Cleanup(func() { _ = f.Close() })
}

func TestAcquireDaemonLock_PreCheck_DoesNotOpenLockFile_OnRefusal(t *testing.T) {
	dir := t.TempDir()
	if err := WritePIDFile(dir, 7373); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}

	withLockAcquireIdentifyDaemonFake(t, func(pid int) (IdentifyResult, error) {
		return IdentifyIsPortalDaemon, nil
	})
	withLockAcquireFake(t, func(_ int, _ int) error {
		t.Fatal("lockAcquire must NOT be called when pre-check returns ErrDaemonLockHeld")
		return nil
	})

	_, err := AcquireDaemonLock(dir)
	if !errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; want errors.Is ErrDaemonLockHeld", err)
	}
	if _, statErr := os.Stat(DaemonLock(dir)); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("daemon.lock exists after pre-check refusal; stat err = %v; want os.ErrNotExist", statErr)
	}
}

func TestAcquireDaemonLock_EWOULDBLOCK_PreCheckSeesNoHolder_FlockFallback(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireFake(t, func(_ int, _ int) error {
		return unix.EWOULDBLOCK
	})

	f, err := AcquireDaemonLock(dir)
	if f != nil {
		_ = f.Close()
		t.Fatalf("expected nil *os.File on contention, got %v", f)
	}
	if !errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; want errors.Is ErrDaemonLockHeld", err)
	}
}

func withLockAcquireFstatFake(t *testing.T, fake func(fd int, st *unix.Stat_t) error) {
	t.Helper()
	prev := lockAcquireFstat
	lockAcquireFstat = fake
	t.Cleanup(func() { lockAcquireFstat = prev })
}

func withLockAcquireStatFake(t *testing.T, fake func(path string, st *unix.Stat_t) error) {
	t.Helper()
	prev := lockAcquireStat
	lockAcquireStat = fake
	t.Cleanup(func() { lockAcquireStat = prev })
}

func inodeSequence(values []uint64) func() uint64 {
	i := 0
	return func() uint64 {
		v := values[i]
		i++
		return v
	}
}

func TestAcquireDaemonLock_InodeCheck_HappyPath(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })

	fstatCalls := 0
	withLockAcquireFstatFake(t, func(_ int, st *unix.Stat_t) error {
		fstatCalls++
		st.Ino = 42
		return nil
	})
	statCalls := 0
	withLockAcquireStatFake(t, func(_ string, st *unix.Stat_t) error {
		statCalls++
		st.Ino = 42
		return nil
	})

	f, err := AcquireDaemonLock(dir)
	if err != nil {
		t.Fatalf("AcquireDaemonLock: %v", err)
	}
	if f == nil {
		t.Fatal("AcquireDaemonLock returned nil *os.File")
	}
	t.Cleanup(func() { _ = f.Close() })

	if fstatCalls != 1 {
		t.Errorf("fstat called %d times; want 1", fstatCalls)
	}
	if statCalls != 1 {
		t.Errorf("stat called %d times; want 1", statCalls)
	}
}

func TestAcquireDaemonLock_InodeCheck_MismatchThenMatch(t *testing.T) {
	dir := t.TempDir()
	flockCalls := 0
	withLockAcquireFake(t, func(_ int, _ int) error {
		flockCalls++
		return nil
	})

	fdInodes := inodeSequence([]uint64{100, 200})
	pathInodes := inodeSequence([]uint64{999, 200})

	withLockAcquireFstatFake(t, func(_ int, st *unix.Stat_t) error {
		st.Ino = fdInodes()
		return nil
	})
	withLockAcquireStatFake(t, func(_ string, st *unix.Stat_t) error {
		st.Ino = pathInodes()
		return nil
	})

	f, err := AcquireDaemonLock(dir)
	if err != nil {
		t.Fatalf("AcquireDaemonLock: %v", err)
	}
	if f == nil {
		t.Fatal("AcquireDaemonLock returned nil *os.File")
	}
	t.Cleanup(func() { _ = f.Close() })

	if flockCalls != 2 {
		t.Errorf("flock called %d times; want 2", flockCalls)
	}
}

func TestAcquireDaemonLock_InodeCheck_ExhaustsRetries(t *testing.T) {
	dir := t.TempDir()
	flockCalls := 0
	withLockAcquireFake(t, func(_ int, _ int) error {
		flockCalls++
		return nil
	})

	withLockAcquireFstatFake(t, func(_ int, st *unix.Stat_t) error {
		st.Ino = 1
		return nil
	})
	withLockAcquireStatFake(t, func(_ string, st *unix.Stat_t) error {
		st.Ino = 2
		return nil
	})

	start := time.Now()
	f, err := AcquireDaemonLock(dir)
	elapsed := time.Since(start)

	if f != nil {
		_ = f.Close()
		t.Fatalf("expected nil *os.File on persistent inode mismatch, got %v", f)
	}
	if err == nil {
		t.Fatal("expected error after 3 mismatches, got nil")
	}
	if errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; must NOT be ErrDaemonLockHeld for persistent inode mismatch", err)
	}
	if flockCalls != 3 {
		t.Errorf("flock called %d times; want 3", flockCalls)
	}
	if elapsed >= 100*time.Millisecond {
		t.Errorf("elapsed = %v; want < 100ms", elapsed)
	}
}

func TestAcquireDaemonLock_InodeCheck_FstatSyscallError(t *testing.T) {
	dir := t.TempDir()
	flockCalls := 0
	withLockAcquireFake(t, func(_ int, _ int) error {
		flockCalls++
		return nil
	})

	withLockAcquireFstatFake(t, func(_ int, _ *unix.Stat_t) error {
		return unix.EBADF
	})
	withLockAcquireStatFake(t, func(_ string, _ *unix.Stat_t) error {
		t.Fatal("stat must not be called when fstat fails")
		return nil
	})

	f, err := AcquireDaemonLock(dir)
	if f != nil {
		_ = f.Close()
		t.Fatalf("expected nil *os.File on fstat error, got %v", f)
	}
	if err == nil {
		t.Fatal("expected error on fstat failure, got nil")
	}
	if errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; must NOT be ErrDaemonLockHeld for fstat error", err)
	}
	if !errors.Is(err, unix.EBADF) {
		t.Fatalf("err = %v; expected wrapped unix.EBADF", err)
	}
	if flockCalls != 1 {
		t.Errorf("flock called %d times; want 1 (no retry on syscall error)", flockCalls)
	}
}

func TestAcquireDaemonLock_InodeCheck_StatSyscallError(t *testing.T) {
	dir := t.TempDir()
	flockCalls := 0
	withLockAcquireFake(t, func(_ int, _ int) error {
		flockCalls++
		return nil
	})

	withLockAcquireFstatFake(t, func(_ int, st *unix.Stat_t) error {
		st.Ino = 5
		return nil
	})
	withLockAcquireStatFake(t, func(_ string, _ *unix.Stat_t) error {
		return unix.ENOENT
	})

	f, err := AcquireDaemonLock(dir)
	if f != nil {
		_ = f.Close()
		t.Fatalf("expected nil *os.File on stat error, got %v", f)
	}
	if err == nil {
		t.Fatal("expected error on stat failure, got nil")
	}
	if errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; must NOT be ErrDaemonLockHeld for stat error", err)
	}
	if !errors.Is(err, unix.ENOENT) {
		t.Fatalf("err = %v; expected wrapped unix.ENOENT", err)
	}
	if flockCalls != 1 {
		t.Errorf("flock called %d times; want 1 (no retry on syscall error)", flockCalls)
	}
}

func TestAcquireDaemonLock_InodeCheck_MismatchClosesFDBeforeRetry(t *testing.T) {
	dir := t.TempDir()

	flockCalls := 0
	withLockAcquireFake(t, func(_ int, _ int) error {
		flockCalls++
		return nil
	})

	fdInodes := inodeSequence([]uint64{1, 2})
	pathInodes := inodeSequence([]uint64{9, 2})
	withLockAcquireFstatFake(t, func(_ int, st *unix.Stat_t) error {
		st.Ino = fdInodes()
		return nil
	})
	withLockAcquireStatFake(t, func(_ string, st *unix.Stat_t) error {
		st.Ino = pathInodes()
		return nil
	})

	f, err := AcquireDaemonLock(dir)
	if err != nil {
		t.Fatalf("AcquireDaemonLock: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })

	if flockCalls != 2 {
		t.Errorf("flock called %d times; want 2 (one per attempt)", flockCalls)
	}

	f2, err := os.OpenFile(DaemonLock(dir), os.O_RDONLY, 0)
	if err != nil {
		t.Fatalf("re-open lock file: %v", err)
	}
	_ = f2.Close()
}

func TestAcquireDaemonLock_InodeCheck_NoFDCLOEXECOnPersistentMismatch(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })

	withLockAcquireFstatFake(t, func(_ int, st *unix.Stat_t) error {
		st.Ino = 7
		return nil
	})
	withLockAcquireStatFake(t, func(_ string, st *unix.Stat_t) error {
		st.Ino = 8
		return nil
	})

	f, err := AcquireDaemonLock(dir)
	if f != nil {
		_ = f.Close()
		t.Fatalf("persistent mismatch must return nil *os.File; got %v", f)
	}
	if err == nil {
		t.Fatal("expected wrapped error on persistent mismatch")
	}
	if errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; must NOT be ErrDaemonLockHeld", err)
	}
}

func TestAcquireDaemonLock_InodeCheck_RetryBoundedWallTime(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })
	withLockAcquireFstatFake(t, func(_ int, st *unix.Stat_t) error {
		st.Ino = 1
		return nil
	})
	withLockAcquireStatFake(t, func(_ string, st *unix.Stat_t) error {
		st.Ino = 2
		return nil
	})

	start := time.Now()
	_, _ = AcquireDaemonLock(dir)
	elapsed := time.Since(start)

	if elapsed >= 100*time.Millisecond {
		t.Errorf("retry loop took %v; want < 100ms", elapsed)
	}
}

func TestAcquireDaemonLock_InodeCheck_NotReachedOnEWOULDBLOCK(t *testing.T) {
	dir := t.TempDir()
	withLockAcquireFake(t, func(_ int, _ int) error { return unix.EWOULDBLOCK })
	withLockAcquireFstatFake(t, func(_ int, _ *unix.Stat_t) error {
		t.Fatal("fstat must not be called when flock returns EWOULDBLOCK")
		return nil
	})
	withLockAcquireStatFake(t, func(_ string, _ *unix.Stat_t) error {
		t.Fatal("stat must not be called when flock returns EWOULDBLOCK")
		return nil
	})

	f, err := AcquireDaemonLock(dir)
	if f != nil {
		_ = f.Close()
		t.Fatalf("expected nil *os.File on contention, got %v", f)
	}
	if !errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; want errors.Is ErrDaemonLockHeld", err)
	}
}

func TestAcquireDaemonLock_InodeCheck_NotReachedOnPreCheckRefusal(t *testing.T) {
	dir := t.TempDir()
	if err := WritePIDFile(dir, 1234); err != nil {
		t.Fatalf("WritePIDFile: %v", err)
	}
	withLockAcquireIdentifyDaemonFake(t, func(_ int) (IdentifyResult, error) {
		return IdentifyIsPortalDaemon, nil
	})
	withLockAcquireFake(t, func(_ int, _ int) error {
		t.Fatal("flock must not be called on pre-check refusal")
		return nil
	})
	withLockAcquireFstatFake(t, func(_ int, _ *unix.Stat_t) error {
		t.Fatal("fstat must not be called on pre-check refusal")
		return nil
	})
	withLockAcquireStatFake(t, func(_ string, _ *unix.Stat_t) error {
		t.Fatal("stat must not be called on pre-check refusal")
		return nil
	})

	f, err := AcquireDaemonLock(dir)
	if f != nil {
		_ = f.Close()
		t.Fatalf("expected nil *os.File on pre-check refusal, got %v", f)
	}
	if !errors.Is(err, ErrDaemonLockHeld) {
		t.Fatalf("err = %v; want errors.Is ErrDaemonLockHeld", err)
	}
}

func TestAcquireDaemonLock_AcceptsArbitraryStateDirParameter(t *testing.T) {
	withLockAcquireFake(t, func(_ int, _ int) error { return nil })

	dirA := t.TempDir()
	dirB := t.TempDir()

	fa, err := AcquireDaemonLock(dirA)
	if err != nil {
		t.Fatalf("AcquireDaemonLock(dirA): %v", err)
	}
	t.Cleanup(func() { _ = fa.Close() })

	fb, err := AcquireDaemonLock(dirB)
	if err != nil {
		t.Fatalf("AcquireDaemonLock(dirB): %v", err)
	}
	t.Cleanup(func() { _ = fb.Close() })

	if _, err := os.Stat(DaemonLock(dirA)); err != nil {
		t.Errorf("lock file missing under dirA: %v", err)
	}
	if _, err := os.Stat(DaemonLock(dirB)); err != nil {
		t.Errorf("lock file missing under dirB: %v", err)
	}
}
