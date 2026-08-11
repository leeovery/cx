package log

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func forceSegmentEEXIST(t *testing.T, taken map[int]bool) {
	t.Helper()
	prev := openSegmentFunc
	openSegmentFunc = func(path string, flag int, perm os.FileMode) (*os.File, error) {
		base := filepath.Base(path)
		idx := lastDot(base)
		if idx >= 0 {
			if n, err := atoiSafe(base[idx+1:]); err == nil && taken[n] {
				return nil, os.ErrExist
			}
		}
		return os.OpenFile(path, flag, perm)
	}
	t.Cleanup(func() { openSegmentFunc = prev })
}

func lastDot(s string) int {
	for i := len(s) - 1; i >= 0; i-- {
		if s[i] == '.' {
			return i
		}
	}
	return -1
}

func atoiSafe(s string) (int, error) {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, os.ErrInvalid
		}
		n = n*10 + int(r-'0')
	}
	if s == "" {
		return 0, os.ErrInvalid
	}
	return n, nil
}

func sizeCapDay(t *testing.T) {
	t.Helper()
	fixedClock(t, time.Date(2026, 5, 29, 10, 0, 0, 0, time.UTC))
}

func segmentTarget(t *testing.T, dir string) string {
	t.Helper()
	target, err := os.Readlink(filepath.Join(dir, "portal.log"))
	if err != nil {
		t.Fatalf("readlink portal.log: %v", err)
	}
	return filepath.Base(target)
}

func TestRotatingSink_RotatesToSegment1WhenNextRecordReachesCap(t *testing.T) {
	sizeCapDay(t)
	dir := t.TempDir()

	s := newRotatingSink(dir, 6)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("aaaa\n")); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	if got := segmentTarget(t, dir); got != "portal.log.2026-05-29" {
		t.Fatalf("after first write symlink = %q, want portal.log.2026-05-29", got)
	}

	if _, err := s.Write([]byte("bbbb\n")); err != nil {
		t.Fatalf("second Write: %v", err)
	}

	if got := segmentTarget(t, dir); got != "portal.log.2026-05-29.1" {
		t.Errorf("after overflow symlink = %q, want portal.log.2026-05-29.1", got)
	}
	seg1 := filepath.Join(dir, "portal.log.2026-05-29.1")
	b, err := os.ReadFile(seg1)
	if err != nil {
		t.Fatalf("read segment .1: %v", err)
	}
	if string(b) != "bbbb\n" {
		t.Errorf("segment .1 = %q, want %q", string(b), "bbbb\n")
	}
	base, err := os.ReadFile(filepath.Join(dir, "portal.log.2026-05-29"))
	if err != nil {
		t.Fatalf("read base file: %v", err)
	}
	if string(base) != "aaaa\n" {
		t.Errorf("base file = %q, want %q", string(base), "aaaa\n")
	}
}

func TestRotatingSink_DiscoversNextNAsMaxPlusOneAcrossGaps(t *testing.T) {
	sizeCapDay(t)
	dir := t.TempDir()

	for _, n := range []string{"1", "3"} {
		if err := os.WriteFile(filepath.Join(dir, "portal.log.2026-05-29."+n), []byte("seed\n"), 0o600); err != nil {
			t.Fatalf("seed segment .%s: %v", n, err)
		}
	}

	s := newRotatingSink(dir, 1)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("overflow\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got := segmentTarget(t, dir); got != "portal.log.2026-05-29.4" {
		t.Errorf("symlink = %q, want portal.log.2026-05-29.4 (max+1 across the gap)", got)
	}
	b, err := os.ReadFile(filepath.Join(dir, "portal.log.2026-05-29.4"))
	if err != nil {
		t.Fatalf("read segment .4: %v", err)
	}
	if string(b) != "overflow\n" {
		t.Errorf("segment .4 = %q, want %q", string(b), "overflow\n")
	}
	if _, err := os.Stat(filepath.Join(dir, "portal.log.2026-05-29.2")); !os.IsNotExist(err) {
		t.Errorf("segment .2 exists (stat err = %v); the gap must NOT be filled", err)
	}
}

func TestRotatingSink_OpensSegment1WhenNoExistingSegments(t *testing.T) {
	sizeCapDay(t)
	dir := t.TempDir()

	s := newRotatingSink(dir, 1)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("first\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got := segmentTarget(t, dir); got != "portal.log.2026-05-29.1" {
		t.Errorf("symlink = %q, want portal.log.2026-05-29.1 (no existing .N -> 1)", got)
	}
	b, err := os.ReadFile(filepath.Join(dir, "portal.log.2026-05-29.1"))
	if err != nil {
		t.Fatalf("read segment .1: %v", err)
	}
	if string(b) != "first\n" {
		t.Errorf("segment .1 = %q, want %q", string(b), "first\n")
	}
}

func TestRotatingSink_RetriesNextNOnEEXISTUntilFreeSegmentClaimed(t *testing.T) {
	sizeCapDay(t)
	dir := t.TempDir()

	forceSegmentEEXIST(t, map[int]bool{1: true, 2: true})

	s := newRotatingSink(dir, 1)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("retry\n")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	if got := segmentTarget(t, dir); got != "portal.log.2026-05-29.3" {
		t.Errorf("symlink = %q, want portal.log.2026-05-29.3 (EEXIST retry 1->2->3)", got)
	}
	b, err := os.ReadFile(filepath.Join(dir, "portal.log.2026-05-29.3"))
	if err != nil {
		t.Fatalf("read segment .3: %v", err)
	}
	if string(b) != "retry\n" {
		t.Errorf("segment .3 = %q, want %q", string(b), "retry\n")
	}
}

func TestRotatingSink_DoesNotChmodPriorSegmentAfterSizeCapRotation(t *testing.T) {
	sizeCapDay(t)
	dir := t.TempDir()

	s := newRotatingSink(dir, 6)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("aaaa\n")); err != nil {
		t.Fatalf("first Write: %v", err)
	}
	if _, err := s.Write([]byte("bbbb\n")); err != nil {
		t.Fatalf("second Write (overflow): %v", err)
	}

	basePath := filepath.Join(dir, "portal.log.2026-05-29")
	info, err := os.Stat(basePath)
	if err != nil {
		t.Fatalf("stat base file: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("prior segment perm = %o after size-cap rotation, want 0600 (must NOT be chmod'd)", got)
	}
}

func TestRotatingSink_NeverRotatesInSteadyStateBelowCap(t *testing.T) {
	sizeCapDay(t)
	dir := t.TempDir()

	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	for i := range 100 {
		if _, err := s.Write([]byte("steady-state line\n")); err != nil {
			t.Fatalf("Write %d: %v", i, err)
		}
	}

	matches, err := filepath.Glob(filepath.Join(dir, "portal.log.2026-05-29*"))
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("found %d portal.log.2026-05-29* files, want 1 (no overflow): %v", len(matches), matches)
	}
	if filepath.Base(matches[0]) != "portal.log.2026-05-29" {
		t.Errorf("sole file = %q, want portal.log.2026-05-29", filepath.Base(matches[0]))
	}
	if got := segmentTarget(t, dir); got != "portal.log.2026-05-29" {
		t.Errorf("symlink = %q, want portal.log.2026-05-29 (no rotation in steady state)", got)
	}
}
