package log

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
)

func TestSwingSymlink_PointsLinkAtTargetAtomically(t *testing.T) {
	dir := t.TempDir()

	if err := swingSymlink(dir, "portal.log.2026-05-30"); err != nil {
		t.Fatalf("swingSymlink: %v", err)
	}

	target, err := os.Readlink(symlinkPath(dir))
	if err != nil {
		t.Fatalf("readlink portal.log: %v", err)
	}
	if target != "portal.log.2026-05-30" {
		t.Errorf("symlink target = %q, want %q (relative bare filename)", target, "portal.log.2026-05-30")
	}

	tmp := pidSymlinkTmp(dir, os.Getpid())
	if _, err := os.Lstat(tmp); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("pid tmp %q lingered after swing (lstat err = %v); want removed by rename", tmp, err)
	}
}

func TestSwingSymlink_ReclaimsStaleSamePidTmpFromPriorCrash(t *testing.T) {
	dir := t.TempDir()

	tmp := pidSymlinkTmp(dir, os.Getpid())
	if err := os.Symlink("portal.log.2026-05-01", tmp); err != nil {
		t.Fatalf("seed stale pid tmp: %v", err)
	}

	if err := swingSymlink(dir, "portal.log.2026-05-30"); err != nil {
		t.Fatalf("swingSymlink with stale tmp present: %v", err)
	}

	target, err := os.Readlink(symlinkPath(dir))
	if err != nil {
		t.Fatalf("readlink portal.log: %v", err)
	}
	if target != "portal.log.2026-05-30" {
		t.Errorf("symlink target = %q, want %q", target, "portal.log.2026-05-30")
	}
	if _, err := os.Lstat(tmp); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("pid tmp %q lingered after reclaim+swing (lstat err = %v)", tmp, err)
	}
}

func TestSwingSymlink_ConcurrentSameTargetConvergesToOneLinkNoOrphan(t *testing.T) {
	dir := t.TempDir()
	const target = "portal.log.2026-05-30"

	const processes = 8
	var wg sync.WaitGroup
	wg.Add(processes)
	for g := range processes {
		pid := 9000 + g
		go func() {
			defer wg.Done()
			if err := swingSymlinkAs(dir, target, pid); err != nil {
				t.Errorf("concurrent swingSymlinkAs(pid): %v", err)
			}
		}()
	}
	wg.Wait()

	resolved, err := os.Readlink(symlinkPath(dir))
	if err != nil {
		t.Fatalf("readlink portal.log: %v", err)
	}
	if resolved != target {
		t.Errorf("symlink target = %q, want %q", resolved, target)
	}

	for g := range processes {
		tmp := pidSymlinkTmp(dir, 9000+g)
		if _, err := os.Lstat(tmp); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("pid tmp %q orphaned after concurrent swings (lstat err = %v)", tmp, err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".tmp" {
			t.Errorf("orphaned temp entry left in state dir: %q", e.Name())
		}
	}
}

func TestPidSymlinkTmp_EmbedsPidAndCannotCollideAcrossPids(t *testing.T) {
	dir := t.TempDir()

	pidA := 1111
	pidB := 2222

	gotA := pidSymlinkTmp(dir, pidA)
	gotB := pidSymlinkTmp(dir, pidB)

	wantA := filepath.Join(dir, "portal.log."+strconv.Itoa(pidA)+".symlink.tmp")
	wantB := filepath.Join(dir, "portal.log."+strconv.Itoa(pidB)+".symlink.tmp")

	if gotA != wantA {
		t.Errorf("pidSymlinkTmp(pidA) = %q, want %q", gotA, wantA)
	}
	if gotB != wantB {
		t.Errorf("pidSymlinkTmp(pidB) = %q, want %q", gotB, wantB)
	}
	if gotA == gotB {
		t.Errorf("pid tmp names collided across pids: %q == %q", gotA, gotB)
	}
}

func TestSwingSymlink_LeavesPriorSymlinkInPlaceOnFailure(t *testing.T) {
	dir := t.TempDir()

	if err := swingSymlink(dir, "portal.log.2026-05-29"); err != nil {
		t.Fatalf("seed prior swing: %v", err)
	}

	prev := symlinkFunc
	symlinkFunc = func(string, string) error { return errors.New("boom") }
	t.Cleanup(func() { symlinkFunc = prev })

	err := swingSymlink(dir, "portal.log.2026-05-30")
	if err == nil {
		t.Fatalf("swingSymlink succeeded; want error from forced Symlink failure")
	}

	target, rlErr := os.Readlink(symlinkPath(dir))
	if rlErr != nil {
		t.Fatalf("readlink portal.log after failed swing: %v", rlErr)
	}
	if target != "portal.log.2026-05-29" {
		t.Errorf("prior symlink target = %q after failed swing, want %q (unchanged)", target, "portal.log.2026-05-29")
	}
}
