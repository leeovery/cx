package log

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestSweepLogsForClean_DeletesEveryPriorDayKeepsTodayWithCutoffToday(t *testing.T) {
	dir := t.TempDir()
	fixedClock(t, mustDate(2026, 5, 30))

	priorDay := touchFile(t, dir, "portal.log.2026-05-29")
	priorSeg := touchFile(t, dir, "portal.log.2026-05-29.1")
	older := touchFile(t, dir, "portal.log.2026-01-01")
	todayBase := touchFile(t, dir, "portal.log.2026-05-30")
	todaySeg := touchFile(t, dir, "portal.log.2026-05-30.1")

	if err := SweepLogsForClean(dir); err != nil {
		t.Fatalf("SweepLogsForClean returned error: %v", err)
	}

	for _, p := range []string{priorDay, priorSeg, older} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still present; cutoff=today must delete every prior-day file", filepath.Base(p))
		}
	}
	for _, p := range []string{todayBase, todaySeg} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s missing; today's file (date == cutoff, strict <) must survive", filepath.Base(p))
		}
	}
}

func TestSweepLogsForClean_BypassesGateWhenSentinelPresent(t *testing.T) {
	dir := t.TempDir()
	fixedClock(t, mustDate(2026, 5, 30))

	touchFile(t, dir, sweptSentinelName("2026-05-30"))
	old := touchFile(t, dir, "portal.log.2026-01-15")

	if err := SweepLogsForClean(dir); err != nil {
		t.Fatalf("SweepLogsForClean returned error: %v", err)
	}

	if _, err := os.Stat(old); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s still present; --logs sweep must bypass the swept.<today> gate", filepath.Base(old))
	}
}

func TestSweepLogsForClean_RemovesAllSweptSentinelsIncludingToday(t *testing.T) {
	dir := t.TempDir()
	fixedClock(t, mustDate(2026, 5, 30))

	stale1 := touchFile(t, dir, sweptSentinelName("2026-05-28"))
	stale2 := touchFile(t, dir, sweptSentinelName("2026-05-29"))
	today := touchFile(t, dir, sweptSentinelName("2026-05-30"))

	if err := SweepLogsForClean(dir); err != nil {
		t.Fatalf("SweepLogsForClean returned error: %v", err)
	}

	for _, p := range []string{stale1, stale2, today} {
		if _, err := os.Stat(p); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("%s still present; --logs must remove ALL swept.* sentinels (today included)", filepath.Base(p))
		}
	}
}

func TestSweepLogsForClean_ForcesCutoffTodayRegardlessOfEnv(t *testing.T) {
	dir := t.TempDir()
	fixedClock(t, mustDate(2026, 5, 30))
	t.Setenv("PORTAL_LOG_RETENTION_DAYS", "365")

	recent := touchFile(t, dir, "portal.log.2026-05-29")

	if err := SweepLogsForClean(dir); err != nil {
		t.Fatalf("SweepLogsForClean returned error: %v", err)
	}

	if _, err := os.Stat(recent); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("%s still present; --logs forces cutoff=today regardless of PORTAL_LOG_RETENTION_DAYS", filepath.Base(recent))
	}
}
