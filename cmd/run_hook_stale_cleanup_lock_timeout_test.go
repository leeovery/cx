package cmd

import (
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/fileutil"
	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/logtest"
)

// lockedSweepFixture stages the stale seed under a sidecar held exclusively
// from an independent open file description, so the sweep's mutation contends
// with a writer it cannot see and times out at the lowered bound. The returned
// release frees the lock for a retry case.
func lockedSweepFixture(t *testing.T, bound time.Duration) (*hooks.Store, string, func()) {
	t.Helper()
	hooks.SetLockTimeoutForTest(t, bound)
	store, path := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
	return store, path, hookstest.HoldHooksSidecar(t, path)
}

func TestHookSweepStandsDownOnLockTimeout(t *testing.T) {
	lister := &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}

	t.Run("it deletes nothing when the sweep cannot take the lock", func(t *testing.T) {
		store, path, _ := lockedSweepFixture(t, lockBound)
		before := readFileBytes(t, path)

		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: want nil on a lock timeout, got %v", err)
		}

		assertHooksFileUnchanged(t, path, before, "rewritten under a held lock")
	})

	t.Run("it logs the stand-down at WARN with reason=lock-timeout", func(t *testing.T) {
		store, _, _ := lockedSweepFixture(t, lockBound)
		sink := logtest.Install(t)

		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		rec := assertStandDown(t, sink, slog.LevelWarn, "lock-timeout")
		if got := rec.AttrString(t, "error"); got == "" {
			t.Errorf("error attr = %q, want the lock error", got)
		} else if !strings.Contains(got, hooks.ErrLockHeld.Error()) {
			t.Errorf("error attr = %q, want it to carry %q", got, hooks.ErrLockHeld.Error())
		}
	})

	t.Run("it emits exactly one WARN per stood-down cycle", func(t *testing.T) {
		store, _, _ := lockedSweepFixture(t, lockBound)
		sink := logtest.Install(t)
		injected, injectedSink := newCaptureLoggerForComponent(t, "daemon")

		if err := sweepErr(lister, store, injected); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		assertStandDown(t, sink, slog.LevelWarn, "lock-timeout")
		for _, rec := range injectedSink.RecordsAtExactLevel(slog.LevelWarn) {
			t.Errorf("unexpected WARN on the injected logger: %+v", rec)
		}
	})

	t.Run("it reports the lock stand-down to the caller", func(t *testing.T) {
		store, _, _ := lockedSweepFixture(t, lockBound)

		outcome, err := runHookStaleCleanup(lister, store, nil)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if outcome.DeclineReason != "lock-timeout" {
			t.Errorf("DeclineReason = %q, want %q", outcome.DeclineReason, "lock-timeout")
		}
		if len(outcome.Removed) != 0 {
			t.Errorf("Removed = %v, want none", outcome.Removed)
		}
	})

	// The sweep takes the sidecar twice, and only the mutation waits at the full
	// bound: the pre-read's near-zero snapshot bound is what keeps a contended
	// cycle from parking the daemon's 1s tick for two of them.
	t.Run("it costs one bound for a fully contended cycle", func(t *testing.T) {
		const bound = 300 * time.Millisecond
		store, _, _ := lockedSweepFixture(t, bound)

		start := time.Now()
		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}
		elapsed := time.Since(start)

		if elapsed < bound {
			t.Errorf("cycle took %v, want at least one full bound of %v (the mutation must wait)", elapsed, bound)
		}
		if elapsed > bound*3/2 {
			t.Errorf("cycle took %v, want well under two bounds of %v (the pre-read must not wait at the full bound)", elapsed, 2*bound)
		}
	})

	t.Run("it retries on the next cadence", func(t *testing.T) {
		store, _, release := lockedSweepFixture(t, lockBound)

		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup under the lock: %v", err)
		}
		release()

		outcome, err := runHookStaleCleanup(lister, store, nil)
		if err != nil {
			t.Fatalf("runHookStaleCleanup after release: %v", err)
		}

		if len(outcome.Removed) != 1 || outcome.Removed[0] != hookstest.ReapableSeedA {
			t.Errorf("Removed = %v, want [%s]", outcome.Removed, hookstest.ReapableSeedA)
		}
		postRun, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[hookstest.ReapableSeedA]; ok {
			t.Errorf("stale key %s survived the retry; got %v", hookstest.ReapableSeedA, keysOf(postRun))
		}
		if _, ok := postRun[hookstest.LiveSeedA]; !ok {
			t.Errorf("live key was reaped by the retry; got %v", keysOf(postRun))
		}
	})

	// The daemon's call shape: its own logger, and no second WARN of its own
	// over the one the sweep already emitted.
	t.Run("it stands the daemon's throttled sweep down without a second WARN", func(t *testing.T) {
		store, path, _ := lockedSweepFixture(t, lockBound)
		before := readFileBytes(t, path)

		sink := logtest.Install(t)
		injected, injectedSink := newCaptureLoggerForComponent(t, "daemon")
		fc := &daemonFakeCommander{panesOut: livePaneRowOut}
		deps := hookCleanupDeps(fc, store, injected)
		deps.lastCleanup = time.Now().Add(-2 * hookCleanupInterval)

		maybeRunHookCleanup(deps)

		assertHooksFileUnchanged(t, path, before, "rewritten by the daemon under a held lock")
		assertStandDown(t, sink, slog.LevelWarn, "lock-timeout")
		if got := len(injectedSink.RecordsWith("daemon", "hooks stale-cleanup failed").AtExactLevel(slog.LevelWarn)); got != 0 {
			t.Errorf("daemon generic-failure WARN count = %d, want 0; entries=%+v", got, injectedSink.Records())
		}
	})
}

func TestHookSweepDiscriminatesLockTimeoutFromFailure(t *testing.T) {
	lister := &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}

	t.Run("it still returns an error for a save failure", func(t *testing.T) {
		store, _ := hookstest.StageStore(t, hookstest.Staging{
			Dir:          filepath.Join(t.TempDir(), "write-denied"),
			Seed:         hookstest.StaleHookSeed,
			WritesDenied: true,
		})
		sink := logtest.Install(t)

		outcome, err := runHookStaleCleanup(lister, store, nil)
		if err == nil {
			t.Fatal("runHookStaleCleanup: want an error on a save failure, got nil")
		}
		if errors.Is(err, hooks.ErrLockHeld) {
			t.Errorf("save failure %v reported as a lock timeout", err)
		}
		// The failure the fixture exists to produce: the mutation took its lock
		// and read cleanly, then failed at the temp create. A staging change
		// that failed at the lock open instead would carry a different class.
		if !errors.Is(err, fileutil.ErrWriteTempCreate) {
			t.Errorf("err = %v, want a write-phase failure carrying %v", err, fileutil.ErrWriteTempCreate)
		}
		if outcome.DeclineReason != "" {
			t.Errorf("DeclineReason = %q, want none on a save failure", outcome.DeclineReason)
		}
		for _, rec := range sink.Records() {
			if rec.Msg == standDownMsg {
				t.Errorf("save failure emitted a stand-down record: %+v", rec)
			}
		}
	})

	// The directory name puts the sentinel's own words into the save failure's
	// message, so a substring check would mistake it for a lock timeout.
	t.Run("it never matches on error text", func(t *testing.T) {
		store, _ := hookstest.StageStore(t, hookstest.Staging{
			Dir:          filepath.Join(t.TempDir(), hooks.ErrLockHeld.Error()),
			Seed:         hookstest.StaleHookSeed,
			WritesDenied: true,
		})
		sink := logtest.Install(t)

		outcome, err := runHookStaleCleanup(lister, store, nil)
		if err == nil {
			t.Fatal("runHookStaleCleanup: want an error, got nil")
		}
		if !strings.Contains(err.Error(), hooks.ErrLockHeld.Error()) {
			t.Fatalf("fixture error %q does not carry the sentinel's text; the case measures nothing", err)
		}
		if outcome.DeclineReason != "" {
			t.Errorf("DeclineReason = %q, want none for a non-sentinel error", outcome.DeclineReason)
		}
		for _, rec := range sink.Records() {
			if rec.Msg == standDownMsg {
				t.Errorf("a non-sentinel error whose text carries the sentinel was treated as a stand-down: %+v", rec)
			}
		}
	})
}

func TestDoctorFixReportsLockedHookPrune(t *testing.T) {
	t.Run("it prints the skipped-prune line for a locked file in doctor --fix", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		deps, hooksPath, _, _, _ := seedStalePruneFixture(t, t.TempDir(), staleHookLister())
		hookstest.HoldHooksSidecar(t, hooksPath)
		before := readFileBytes(t, hooksPath)

		outBuf, _, _ := runDoctorWith(t, deps, "--fix")

		assertSkippedPruneLine(t, outBuf.String(), "Skipped stale hook prune: hooks.json is locked")
		assertHooksFileUnchanged(t, hooksPath, before, "rewritten on a lock stand-down")
	})

	// The read side degrades where the write side stands down, so the un-pruned
	// entry is reported as the stale hook it is rather than as a lock problem.
	t.Run("it reports the un-pruned entry as stale in the same window", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		store, path := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
		hookstest.HoldHooksSidecar(t, path)

		got := checkStaleHooks(&stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, store)
		if got.status != checkFail {
			t.Errorf("status = %v, want checkFail under a held lock", got.status)
		}
		if got.detail != "1 stale hook entry" {
			t.Errorf("detail = %q, want %q", got.detail, "1 stale hook entry")
		}
	})

	t.Run("it leaves the doctor --fix exit code to the post-repair diagnosis", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)

		// A healthy install whose only anomaly is the lock: the entry is live,
		// so the post-repair diagnosis finds nothing stale.
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, hooksPath := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(hookstest.LiveSeedA)})
		hookstest.HoldHooksSidecar(t, hooksPath)
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, hookStore, projectStore)

		outBuf, _, err := runDoctorWith(t, deps, "--fix")
		if err != nil {
			t.Fatalf("Execute err = %v; want nil over a healthy post-repair diagnosis\n%s", err, outBuf.String())
		}
		if !strings.Contains(outBuf.String(), "Skipped stale hook prune: hooks.json is locked") {
			t.Fatalf("fixture did not stand the prune down:\n%s", outBuf.String())
		}

		// The same stand-down with a genuinely failing check still exits non-zero.
		failingDir := t.TempDir()
		seedHealthyStateDir(t, failingDir)
		failingHooks, failingPath := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(hookstest.LiveSeedA)})
		hookstest.HoldHooksSidecar(t, failingPath)
		failingProjects, _ := seedProjectsJSON(t, t.TempDir())
		failingDeps := staleDeps(failingDir, &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, failingHooks, failingProjects)
		failingDeps.SaverPresent = func() (bool, error) { return false, nil }

		failBuf, _, failErr := runDoctorWith(t, failingDeps, "--fix")
		if !errors.Is(failErr, ErrDoctorUnhealthy) {
			t.Fatalf("Execute err = %v; want ErrDoctorUnhealthy with a failing check\n%s", failErr, failBuf.String())
		}
	})
}
