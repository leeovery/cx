package log

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"sync"
	"syscall"
	"testing"
	"time"
)

func fixedClock(t *testing.T, initial time.Time) (set func(time.Time)) {
	t.Helper()
	cur := initial
	var mu sync.Mutex
	prev := nowFunc
	nowFunc = func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return cur
	}
	t.Cleanup(func() { nowFunc = prev })
	return func(at time.Time) {
		mu.Lock()
		cur = at
		mu.Unlock()
	}
}

func statDevIno(t *testing.T, path string) (uint64, uint64) {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("stat %s: Sys() is %T, want *syscall.Stat_t", path, info.Sys())
	}
	return uint64(st.Dev), st.Ino
}

func TestRotatingSink_FirstHandleCreatesDayFileViaExclusive(t *testing.T) {
	day := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	fixedClock(t, day)

	dir := t.TempDir()
	s := newRotatingSink(dir, defaultRotateSize)

	if _, err := s.Write([]byte("line one\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	t.Cleanup(func() { _ = s.close() })

	dayPath := filepath.Join(dir, "portal.log.2026-05-29")
	b, err := os.ReadFile(dayPath)
	if err != nil {
		t.Fatalf("reading day file: %v", err)
	}
	if string(b) != "line one\n" {
		t.Errorf("day file contents = %q, want %q", string(b), "line one\n")
	}

	target, err := os.Readlink(filepath.Join(dir, "portal.log"))
	if err != nil {
		t.Fatalf("readlink portal.log: %v", err)
	}
	if filepath.Base(target) != "portal.log.2026-05-29" {
		t.Errorf("symlink target = %q, want portal.log.2026-05-29", target)
	}
}

func TestRotatingSink_ReusesFdAcrossSameDayWritesWithMatchingInode(t *testing.T) {
	day := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	fixedClock(t, day)

	dir := t.TempDir()
	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("first\n")); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	fd1 := s.file.Fd()

	if _, err := s.Write([]byte("second\n")); err != nil {
		t.Fatalf("second Write: %v", err)
	}
	fd2 := s.file.Fd()

	if fd1 != fd2 {
		t.Errorf("fd changed across same-day writes: %d -> %d (expected reuse)", fd1, fd2)
	}

	b, err := os.ReadFile(filepath.Join(dir, "portal.log.2026-05-29"))
	if err != nil {
		t.Fatalf("read day file: %v", err)
	}
	if string(b) != "first\nsecond\n" {
		t.Errorf("day file = %q, want %q", string(b), "first\nsecond\n")
	}
}

func TestRotatingSink_ReopensOnSameDayInodeMismatchWithoutSweeps(t *testing.T) {
	day := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	fixedClock(t, day)

	dir := t.TempDir()
	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	sweeps := 0
	s.dayRoll = func(string) { sweeps++ }

	if _, err := s.Write([]byte("before\n")); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	baselineSweeps := sweeps
	origIno := s.ino

	dayPath := filepath.Join(dir, "portal.log.2026-05-29")
	if err := os.Remove(dayPath); err != nil {
		t.Fatalf("remove day file: %v", err)
	}
	nf, err := os.OpenFile(dayPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatalf("recreate day file: %v", err)
	}
	_ = nf.Close()
	newIno := mustIno(t, dayPath)
	if newIno == origIno {
		t.Fatalf("test setup: recreated file reused inode %d", newIno)
	}

	if _, err := s.Write([]byte("after\n")); err != nil {
		t.Fatalf("second Write: %v", err)
	}

	if sweeps != baselineSweeps {
		t.Errorf("day-roll sweeps ran %d times across the same-day inode mismatch; want %d (mismatch reopen must not sweep)", sweeps, baselineSweeps)
	}
	if s.ino != newIno {
		t.Errorf("sink inode = %d after reopen, want live inode %d", s.ino, newIno)
	}
	b, err := os.ReadFile(dayPath)
	if err != nil {
		t.Fatalf("read live day file: %v", err)
	}
	if string(b) != "after\n" {
		t.Errorf("live day file = %q, want %q (reopened onto live target)", string(b), "after\n")
	}
}

func TestRotatingSink_RecreatesDayFileWhenSymlinkTargetENOENT(t *testing.T) {
	day := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	fixedClock(t, day)

	dir := t.TempDir()
	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	sweeps := 0
	s.dayRoll = func(string) { sweeps++ }

	if _, err := s.Write([]byte("before\n")); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	baselineSweeps := sweeps

	dayPath := filepath.Join(dir, "portal.log.2026-05-29")
	if err := os.Remove(dayPath); err != nil {
		t.Fatalf("remove day file: %v", err)
	}

	if _, err := s.Write([]byte("after\n")); err != nil {
		t.Fatalf("second Write: %v", err)
	}

	if sweeps != baselineSweeps {
		t.Errorf("day-roll sweeps ran %d times across the ENOENT mid-day reopen; want %d (same-day reopen must not sweep)", sweeps, baselineSweeps)
	}
	b, err := os.ReadFile(dayPath)
	if err != nil {
		t.Fatalf("read recreated day file: %v", err)
	}
	if string(b) != "after\n" {
		t.Errorf("recreated day file = %q, want %q", string(b), "after\n")
	}
}

func TestRotatingSink_OpensNewDayFileAndFlagsSweepsOnDateChange(t *testing.T) {
	day1 := time.Date(2026, 5, 29, 23, 59, 0, 0, time.UTC)
	set := fixedClock(t, day1)

	dir := t.TempDir()
	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	sweeps := 0
	s.dayRoll = func(string) { sweeps++ }

	if _, err := s.Write([]byte("day-one\n")); err != nil {
		t.Fatalf("day-one Write: %v", err)
	}
	if sweeps != 1 {
		t.Fatalf("sweeps ran %d times on first-ever write; want 1 (first-of-day)", sweeps)
	}

	set(time.Date(2026, 5, 30, 0, 0, 1, 0, time.UTC))

	if _, err := s.Write([]byte("day-two\n")); err != nil {
		t.Fatalf("day-two Write: %v", err)
	}

	if sweeps != 2 {
		t.Errorf("day-roll sweeps ran %d times after a date change; want 2 (first-of-day + roll)", sweeps)
	}

	b2, err := os.ReadFile(filepath.Join(dir, "portal.log.2026-05-30"))
	if err != nil {
		t.Fatalf("read day-two file: %v", err)
	}
	if string(b2) != "day-two\n" {
		t.Errorf("day-two file = %q, want %q", string(b2), "day-two\n")
	}
	b1, err := os.ReadFile(filepath.Join(dir, "portal.log.2026-05-29"))
	if err != nil {
		t.Fatalf("read day-one file: %v", err)
	}
	if string(b1) != "day-one\n" {
		t.Errorf("day-one file = %q, want %q (untouched after roll)", string(b1), "day-one\n")
	}
	target, err := os.Readlink(filepath.Join(dir, "portal.log"))
	if err != nil {
		t.Fatalf("readlink: %v", err)
	}
	if filepath.Base(target) != "portal.log.2026-05-30" {
		t.Errorf("symlink target = %q, want portal.log.2026-05-30", target)
	}
}

func TestRotatingSink_FallsBackToAppendOnEEXISTWhenLosingCreateRace(t *testing.T) {
	day := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	fixedClock(t, day)

	dir := t.TempDir()

	dayPath := filepath.Join(dir, "portal.log.2026-05-29")
	if err := os.WriteFile(dayPath, []byte("peer-line\n"), 0o600); err != nil {
		t.Fatalf("seed peer file: %v", err)
	}

	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("our-line\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	b, err := os.ReadFile(dayPath)
	if err != nil {
		t.Fatalf("read day file: %v", err)
	}
	if string(b) != "peer-line\nour-line\n" {
		t.Errorf("day file = %q, want %q (append fallback must preserve peer line)", string(b), "peer-line\nour-line\n")
	}
}

func TestRotatingSink_RaceFreeUnderConcurrentWrite(t *testing.T) {
	day := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	fixedClock(t, day)

	dir := t.TempDir()
	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	const goroutines = 8
	const perGoroutine = 50

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for g := range goroutines {
		go func(id int) {
			defer wg.Done()
			for i := range perGoroutine {
				line := "g" + strconv.Itoa(id) + "-" + strconv.Itoa(i) + "\n"
				if _, err := s.Write([]byte(line)); err != nil {
					t.Errorf("concurrent Write: %v", err)
					return
				}
			}
		}(g)
	}
	wg.Wait()

	b, err := os.ReadFile(filepath.Join(dir, "portal.log.2026-05-29"))
	if err != nil {
		t.Fatalf("read day file: %v", err)
	}
	lines := 0
	for _, c := range b {
		if c == '\n' {
			lines++
		}
	}
	if want := goroutines * perGoroutine; lines != want {
		t.Errorf("wrote %d lines, want %d", lines, want)
	}
}

func TestRotatingSink_MigratesLegacyRegularFilePortalLogToSymlinkOnReopen(t *testing.T) {
	day := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	fixedClock(t, day)

	dir := t.TempDir()

	link := filepath.Join(dir, "portal.log")
	if err := os.WriteFile(link, []byte("legacy regular log\n"), 0o600); err != nil {
		t.Fatalf("seed legacy regular-file portal.log: %v", err)
	}
	oldPath := filepath.Join(dir, "portal.log.old")
	if err := os.WriteFile(oldPath, []byte("legacy old\n"), 0o600); err != nil {
		t.Fatalf("seed legacy portal.log.old: %v", err)
	}

	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("first line\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	info, err := os.Lstat(link)
	if err != nil {
		t.Fatalf("lstat portal.log after reopen: %v", err)
	}
	if info.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("portal.log mode = %v after reopen, want a symlink (guard+swing failed to compose)", info.Mode())
	}
	target, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("readlink portal.log: %v", err)
	}
	if filepath.Base(target) != "portal.log.2026-05-29" {
		t.Errorf("symlink target = %q, want portal.log.2026-05-29", target)
	}

	if _, err := os.Lstat(oldPath); !os.IsNotExist(err) {
		t.Errorf("portal.log.old still present after reopen (lstat err = %v); want removed by guard", err)
	}

	b, err := os.ReadFile(filepath.Join(dir, "portal.log.2026-05-29"))
	if err != nil {
		t.Fatalf("read day file: %v", err)
	}
	if string(b) != "first line\n" {
		t.Errorf("day file = %q, want %q", string(b), "first line\n")
	}
}

func TestRotatingSink_FirstEverWriteRunsGatedRetentionSweep(t *testing.T) {
	fixedClock(t, mustDate(2026, 5, 30))
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "30")

	dir := t.TempDir()

	old := touchFile(t, dir, "portal.log.2026-01-01")

	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("first-ever\n")); err != nil {
		t.Fatalf("first Write: %v", err)
	}

	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("aged file %s still present; the first-ever write must run the gated retention sweep", filepath.Base(old))
	}
	sentinel := sweptSentinelFile(dir, "2026-05-30")
	if _, err := os.Stat(sentinel); err != nil {
		t.Errorf("single-winner sentinel %s missing; the first-ever write's sweep must claim the day's gate", filepath.Base(sentinel))
	}
}

func TestRotatingSink_SecondFreshSinkSameDayDoesNotResweep(t *testing.T) {
	fixedClock(t, mustDate(2026, 5, 30))
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "30")

	dir := t.TempDir()
	old := touchFile(t, dir, "portal.log.2026-01-01")

	s1 := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s1.close() })
	if _, err := s1.Write([]byte("winner\n")); err != nil {
		t.Fatalf("winner Write: %v", err)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("first winner did not delete the aged file; test precondition unmet")
	}

	sentinel := sweptSentinelFile(dir, "2026-05-30")
	sentinelIno := mustIno(t, sentinel)

	resed := touchFile(t, dir, "portal.log.2026-01-02")

	s2 := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s2.close() })
	if _, err := s2.Write([]byte("loser\n")); err != nil {
		t.Fatalf("loser Write: %v", err)
	}

	if _, err := os.Stat(resed); err != nil {
		t.Errorf("re-seeded aged file %s deleted; the gate-losing second sink must NOT re-run the sweep", filepath.Base(resed))
	}
	if got := mustIno(t, sentinel); got != sentinelIno {
		t.Errorf("sentinel inode changed (%d -> %d); the loser must NOT recreate the sentinel", sentinelIno, got)
	}
}

func TestRotatingSink_FirstWriteCreatesNonExistentStateDir(t *testing.T) {
	day := time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC)
	fixedClock(t, day)

	dir := filepath.Join(t.TempDir(), "state-not-created-yet")
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("test precondition: %s must not exist yet (stat err = %v)", dir, err)
	}

	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("process: start\n")); err != nil {
		t.Fatalf("Write to non-existent state dir: %v", err)
	}

	if info, err := os.Stat(dir); err != nil {
		t.Fatalf("state dir not created by first Write: %v", err)
	} else if !info.IsDir() {
		t.Fatalf("state dir path is not a directory: mode %v", info.Mode())
	}

	dayPath := filepath.Join(dir, "portal.log.2026-05-29")
	b, err := os.ReadFile(dayPath)
	if err != nil {
		t.Fatalf("day file not created in fresh state dir: %v", err)
	}
	if string(b) != "process: start\n" {
		t.Errorf("day file = %q, want %q (record must land in the file, not stderr)", string(b), "process: start\n")
	}

	target, err := os.Readlink(filepath.Join(dir, "portal.log"))
	if err != nil {
		t.Fatalf("readlink portal.log: %v", err)
	}
	if filepath.Base(target) != "portal.log.2026-05-29" {
		t.Errorf("symlink target = %q, want portal.log.2026-05-29", target)
	}
}

func TestRotatingSink_DayRollFiresOutsideWriteCriticalSection(t *testing.T) {
	set := fixedClock(t, mustDate(2026, 5, 29))

	dir := t.TempDir()
	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	var rolls []string
	s.dayRoll = func(today string) {
		rolls = append(rolls, today)
		if _, err := s.Write([]byte("sweep-breadcrumb\n")); err != nil {
			t.Errorf("re-entrant Write from dayRoll: %v", err)
		}
	}

	write := func(line string) {
		t.Helper()
		done := make(chan struct{})
		go func() {
			defer close(done)
			if _, err := s.Write([]byte(line)); err != nil {
				t.Errorf("Write %q: %v", line, err)
			}
		}()
		select {
		case <-done:
		case <-time.After(10 * time.Second):
			t.Fatalf("Write %q deadlocked: dayRoll must fire outside the sink mutex", line)
		}
	}

	write("first-ever\n")
	set(mustDate(2026, 5, 30))
	write("cross-midnight\n")

	if want := []string{"2026-05-29", "2026-05-30"}; !slices.Equal(rolls, want) {
		t.Errorf("dayRoll fired with dates %v, want %v", rolls, want)
	}

	b, err := os.ReadFile(filepath.Join(dir, "portal.log.2026-05-30"))
	if err != nil {
		t.Fatalf("read day-two file: %v", err)
	}
	if got, want := string(b), "cross-midnight\nsweep-breadcrumb\n"; got != want {
		t.Errorf("day-two file = %q, want %q (roll fires after the triggering write)", got, want)
	}
}

func TestRotatingSink_ProbeQueuesFirstOfDayRollForFirstWrite(t *testing.T) {
	fixedClock(t, mustDate(2026, 5, 29))

	dir := t.TempDir()
	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	var rolls []string
	s.dayRoll = func(today string) { rolls = append(rolls, today) }

	if err := s.probe(); err != nil {
		t.Fatalf("probe: %v", err)
	}
	if len(rolls) != 0 {
		t.Fatalf("probe fired the day roll %d times; want 0 (queued for the first Write)", len(rolls))
	}

	if _, err := s.Write([]byte("process: start\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if want := []string{"2026-05-29"}; !slices.Equal(rolls, want) {
		t.Errorf("first Write after probe fired the day roll with %v, want %v", rolls, want)
	}
}

func mustIno(t *testing.T, path string) uint64 {
	t.Helper()
	_, ino := statDevIno(t, path)
	return ino
}
