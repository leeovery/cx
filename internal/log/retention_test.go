package log

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func recordsByMessage(captured *[]capturedRecord, msg string) []capturedRecord {
	var out []capturedRecord
	for _, r := range *captured {
		if r.message == msg {
			out = append(out, r)
		}
	}
	return out
}

func sweptSentinelName(date string) string {
	return portalLogName + ".swept." + date
}

func TestRunRetentionSweep_ReturnsImmediatelyWhenGateLost(t *testing.T) {
	dir := t.TempDir()

	touchFile(t, dir, sweptSentinelName("2026-05-30"))

	old := touchFile(t, dir, "portal.log.2026-01-01")

	rec, captured := newComponentCapture()
	SetTestHandler(t, rec)

	runRetentionSweep(dir, "2026-05-30", true)

	if len(*captured) != 0 {
		t.Errorf("gate-lost sweep emitted %d records; want 0 (run nothing, emit nothing)", len(*captured))
	}
	if _, err := os.Stat(old); err != nil {
		t.Errorf("gate-lost sweep deleted %s; want untouched (sweep must not run)", filepath.Base(old))
	}
}

func TestRunRetentionSweep_EmitsInfoBeforeEachRemove(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "30")

	old := touchFile(t, dir, "portal.log.2026-01-15")

	rec, captured := newComponentCapture()
	SetTestHandler(t, rec)

	runRetentionSweep(dir, "2026-05-30", true)

	infos := recordsByMessage(captured, "deleted")
	if len(infos) != 1 {
		t.Fatalf("got %d 'deleted' INFO records, want 1", len(infos))
	}
	info := infos[0]
	if got := info.attrs["component"]; got != "log-rotate" {
		t.Errorf("INFO component = %q, want log-rotate", got)
	}
	if got := info.attrs["path"]; got != old {
		t.Errorf("INFO path = %q, want %q", got, old)
	}
	if got := info.attrs["retention"]; got != "30" {
		t.Errorf("INFO retention = %q, want 30", got)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("file %s still present after sweep; want deleted", filepath.Base(old))
	}
}

func TestRunRetentionSweep_DeletesOlderKeepsWithinWindow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "30")

	deleted1 := touchFile(t, dir, "portal.log.2026-04-29")
	deleted2 := touchFile(t, dir, "portal.log.2026-04-29.1")
	keptCutoff := touchFile(t, dir, "portal.log.2026-04-30")
	keptRecent := touchFile(t, dir, "portal.log.2026-05-29")

	runRetentionSweep(dir, "2026-05-30", true)

	for _, p := range []string{deleted1, deleted2} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still present; want deleted (date < cutoff)", filepath.Base(p))
		}
	}
	for _, p := range []string{keptCutoff, keptRecent} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s missing; want kept (date >= cutoff)", filepath.Base(p))
		}
	}
}

func TestRunRetentionSweep_FallsBackTo30WithWarnOnInvalidEnv(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "banana")

	old := touchFile(t, dir, "portal.log.2026-01-01")

	rec, captured := newComponentCapture()
	SetTestHandler(t, rec)

	runRetentionSweep(dir, "2026-05-30", true)

	warns := recordsByMessage(captured, "invalid PORTAL_LOG_RETENTION_DAYS")
	if len(warns) != 1 {
		t.Fatalf("got %d invalid-env WARN records, want 1", len(warns))
	}
	w := warns[0]
	if got := w.attrs["component"]; got != "log-rotate" {
		t.Errorf("WARN component = %q, want log-rotate", got)
	}
	if got := w.attrs["raw"]; got != "banana" {
		t.Errorf("WARN raw = %q, want banana (verbatim)", got)
	}
	if got := w.attrs["retention"]; got != "30" {
		t.Errorf("WARN retention = %q, want 30", got)
	}
	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s still present; fallback retention=30 should delete it", filepath.Base(old))
	}
}

func TestRunRetentionSweep_NeverDeletesSymlinkTmpOrSweptSentinel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "0")

	tmp := touchFile(t, dir, "portal.log."+strconv.Itoa(os.Getpid())+".symlink.tmp")
	other := touchFile(t, dir, "portal.log.notes")
	// Not seeded: claimSweepGate creates it as the winner of this sweep.
	sentinel := sweptSentinelFile(dir, "2026-05-30")

	runRetentionSweep(dir, "2026-05-30", true)

	for _, p := range []string{tmp, sentinel, other} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s was deleted by the cutoff walk; strict date-parse must skip non-log siblings", filepath.Base(p))
		}
	}
}

func TestRunRetentionSweep_PrunesStaleSweptSentinelsKeepsToday(t *testing.T) {
	dir := t.TempDir()

	stale1 := touchFile(t, dir, sweptSentinelName("2026-05-28"))
	stale2 := touchFile(t, dir, sweptSentinelName("2026-05-29"))
	// Today's sentinel is deliberately not seeded: claimSweepGate creates it as
	// the winner, and seeding it would lose the gate before the prune runs.
	todaySentinel := sweptSentinelFile(dir, "2026-05-30")

	runRetentionSweep(dir, "2026-05-30", true)

	for _, p := range []string{stale1, stale2} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("stale sentinel %s still present; want pruned (date != today)", filepath.Base(p))
		}
	}
	if _, err := os.Stat(todaySentinel); err != nil {
		t.Errorf("today's sentinel %s pruned; want kept (live claim)", filepath.Base(todaySentinel))
	}
}

func TestRunRetentionSweep_WarnsAndContinuesOnRemoveFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "0")

	failPath := filepath.Join(dir, "portal.log.2026-04-01")
	okPath := touchFile(t, dir, "portal.log.2026-04-02")
	touchFile(t, dir, "portal.log.2026-04-01")

	prev := removeFunc
	removeFunc = func(path string) error {
		if path == failPath {
			return errors.New("synthetic remove failure")
		}
		return os.Remove(path)
	}
	t.Cleanup(func() { removeFunc = prev })

	rec, captured := newComponentCapture()
	SetTestHandler(t, rec)

	runRetentionSweep(dir, "2026-05-30", true)

	warns := recordsByMessage(captured, "delete failed")
	if len(warns) != 1 {
		t.Fatalf("got %d 'delete failed' WARN records, want 1", len(warns))
	}
	w := warns[0]
	if got := w.attrs["component"]; got != "log-rotate" {
		t.Errorf("WARN component = %q, want log-rotate", got)
	}
	if got := w.attrs["path"]; got != failPath {
		t.Errorf("WARN path = %q, want %q", got, failPath)
	}
	if _, ok := w.attrs["error"]; !ok {
		t.Errorf("WARN missing error attr")
	}

	if _, err := os.Stat(okPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s still present; sweep must continue past a remove failure", filepath.Base(okPath))
	}
}

func TestRunRetentionSweep_SingleSourcesBreadcrumbsAcrossConcurrentStartups(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "30")

	touchFile(t, dir, "portal.log.2026-01-15")

	rec, captured := newComponentCapture()
	SetTestHandler(t, rec)

	runRetentionSweep(dir, "2026-05-30", true)
	runRetentionSweep(dir, "2026-05-30", true)

	infos := recordsByMessage(captured, "deleted")
	if len(infos) != 1 {
		t.Errorf("got %d 'deleted' breadcrumbs across two startups, want 1 (single-sourced)", len(infos))
	}
	warns := recordsByMessage(captured, "delete failed")
	if len(warns) != 0 {
		t.Errorf("got %d 'delete failed' WARNs, want 0 (second process must not re-attempt deletions)", len(warns))
	}
}

func TestRunRetentionSweep_UngatedAlwaysRunsRegardlessOfSentinel(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "30")

	touchFile(t, dir, sweptSentinelName("2026-05-30"))
	old := touchFile(t, dir, "portal.log.2026-01-15")

	runRetentionSweep(dir, "2026-05-30", false)

	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s still present; ungated sweep must run regardless of the sentinel", filepath.Base(old))
	}
}

func TestRotatingSink_RunsRetentionSweepOnRealDayRoll(t *testing.T) {
	day1 := mustDate(2026, 5, 29)
	set := fixedClock(t, day1)
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "30")

	dir := t.TempDir()

	s := newRotatingSink(dir, defaultRotateSize)
	t.Cleanup(func() { _ = s.close() })

	if _, err := s.Write([]byte("day-one\n")); err != nil {
		t.Fatalf("day-one Write: %v", err)
	}

	old := touchFile(t, dir, "portal.log.2026-01-01")
	if _, err := os.Stat(old); err != nil {
		t.Fatalf("aged file removed before the roll; want untouched until the day roll")
	}

	set(mustDate(2026, 5, 30))
	if _, err := s.Write([]byte("day-two\n")); err != nil {
		t.Fatalf("day-two Write: %v", err)
	}

	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("aged file still present after day roll; sink must wire runRetentionSweep into dayRoll")
	}
}
