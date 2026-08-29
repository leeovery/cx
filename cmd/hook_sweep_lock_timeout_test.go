package cmd

import (
	"errors"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/transienttest"
)

// lockedSweepFixture stages the stale seed under a sidecar held exclusively
// from an independent open file description, so the sweep's mutation contends
// with a writer it cannot see and times out at the lowered bound. The returned
// release frees the lock for a retry case.
func lockedSweepFixture(t *testing.T, bound time.Duration) (*hooks.Store, string, func()) {
	t.Helper()
	hooks.SetLockTimeoutForTest(t, bound)
	store, path := newTempHooksStore(t, staleHookSeed)
	return store, path, transienttest.HoldHooksSidecar(t, path)
}

// lockStandDownRecord asserts the sweep left exactly one WARN and returns it.
// The degraded pre-read's own DEBUG breadcrumb shares the sink, so the count is
// taken at WARN rather than over every record.
func lockStandDownRecord(t *testing.T, sink *logtest.Sink) logtest.Record {
	t.Helper()
	warns := sink.RecordsAtLevel(slog.LevelWarn)
	if len(warns) != 1 {
		t.Fatalf("WARN record count = %d, want exactly 1: %+v", len(warns), warns)
	}
	rec := warns[0]
	assertHooksRecord(t, rec, standDownWant(slog.LevelWarn))
	if got := rec.AttrString(t, "reason"); got != "lock-timeout" {
		t.Errorf("reason = %q, want %q", got, "lock-timeout")
	}
	return rec
}

func TestHookSweepStandsDownOnLockTimeout(t *testing.T) {
	lister := &stubAllPaneLister{rows: tokenRows(liveSeedA)}

	t.Run("it deletes nothing when the sweep cannot take the lock", func(t *testing.T) {
		store, path, _ := lockedSweepFixture(t, lockBound)
		before := readFileBytes(t, path)

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: want nil on a lock timeout, got %v", err)
		}

		assertHooksFileUnchanged(t, path, before, "rewritten under a held lock")
	})

	t.Run("it logs the stand-down at WARN with reason=lock-timeout", func(t *testing.T) {
		store, _, _ := lockedSweepFixture(t, lockBound)
		sink := installHooksSink(t)

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		rec := lockStandDownRecord(t, sink)
		if got := rec.AttrString(t, "error"); got == "" {
			t.Errorf("error attr = %q, want the lock error", got)
		} else if !strings.Contains(got, hooks.ErrLockHeld.Error()) {
			t.Errorf("error attr = %q, want it to carry %q", got, hooks.ErrLockHeld.Error())
		}
	})

	t.Run("it emits exactly one WARN per stood-down cycle", func(t *testing.T) {
		store, _, _ := lockedSweepFixture(t, lockBound)
		sink := installHooksSink(t)
		injected := &recordingLogger{}

		if err := runHookStaleCleanup(lister, store, injected.Logger().With("component", "daemon"), nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		lockStandDownRecord(t, sink)
		for _, e := range injected.entries {
			if e.level == "warn" {
				t.Errorf("unexpected WARN on the injected logger: %+v", e)
			}
		}
	})

	t.Run("it reports the lock stand-down to the caller", func(t *testing.T) {
		store, _, _ := lockedSweepFixture(t, lockBound)

		var skipped, removed []string
		err := runHookStaleCleanup(lister, store, nil,
			func(key string) { removed = append(removed, key) },
			func(reason string) { skipped = append(skipped, reason) },
		)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if len(skipped) != 1 || skipped[0] != "lock-timeout" {
			t.Errorf("onSkipped invocations = %v, want [lock-timeout]", skipped)
		}
		if len(removed) != 0 {
			t.Errorf("onRemoved invoked with %v, want no invocations", removed)
		}
	})

	t.Run("it survives a nil onSkipped on the lock branch", func(t *testing.T) {
		store, _, _ := lockedSweepFixture(t, lockBound)

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup with a nil onSkipped: %v", err)
		}
	})

	// The sweep takes the sidecar twice, and only the mutation waits at the full
	// bound: the pre-read's near-zero snapshot bound is what keeps a contended
	// cycle from parking the daemon's 1s tick for two of them.
	t.Run("it costs one bound for a fully contended cycle", func(t *testing.T) {
		const bound = 300 * time.Millisecond
		store, _, _ := lockedSweepFixture(t, bound)

		start := time.Now()
		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
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

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup under the lock: %v", err)
		}
		release()

		var removed []string
		err := runHookStaleCleanup(lister, store, nil, func(key string) { removed = append(removed, key) }, nil)
		if err != nil {
			t.Fatalf("runHookStaleCleanup after release: %v", err)
		}

		if len(removed) != 1 || removed[0] != reapableSeedA {
			t.Errorf("onRemoved invocations = %v, want [%s]", removed, reapableSeedA)
		}
		postRun, err := store.Load("internal")
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[reapableSeedA]; ok {
			t.Errorf("stale key %s survived the retry; got %v", reapableSeedA, keysOf(postRun))
		}
		if _, ok := postRun[liveSeedA]; !ok {
			t.Errorf("live key was reaped by the retry; got %v", keysOf(postRun))
		}
	})

	// The daemon's call shape: a nil onSkipped, its own logger, and no second
	// WARN of its own over the one the sweep already emitted.
	t.Run("it stands the daemon's throttled sweep down without a second WARN", func(t *testing.T) {
		store, path, _ := lockedSweepFixture(t, lockBound)
		before := readFileBytes(t, path)

		sink := installHooksSink(t)
		injected := &recordingLogger{}
		fc := &daemonFakeCommander{panesOut: livePaneRowOut}
		deps := hookCleanupDeps(fc, store, injected.Logger().With("component", "daemon"))
		deps.lastCleanup = time.Now().Add(-2 * hookCleanupInterval)

		maybeRunHookCleanup(deps)

		assertHooksFileUnchanged(t, path, before, "rewritten by the daemon under a held lock")
		lockStandDownRecord(t, sink)
		if got := countMatching(injected.entries, "warn", "daemon", "hooks stale-cleanup failed"); got != 0 {
			t.Errorf("daemon generic-failure WARN count = %d, want 0; entries=%+v", got, injected.entries)
		}
	})
}

func TestHookSweepDiscriminatesLockTimeoutFromFailure(t *testing.T) {
	lister := &stubAllPaneLister{rows: tokenRows(liveSeedA)}

	t.Run("it still returns an error for a save failure", func(t *testing.T) {
		store, _ := newStagedHooksStore(t, hooksStoreStaging{
			dir:          filepath.Join(t.TempDir(), "write-denied"),
			seed:         staleHookSeed,
			writesDenied: true,
		})
		sink := installHooksSink(t)

		var skipped []string
		err := runHookStaleCleanup(lister, store, nil, nil,
			func(reason string) { skipped = append(skipped, reason) })
		if err == nil {
			t.Fatal("runHookStaleCleanup: want an error on a save failure, got nil")
		}
		if errors.Is(err, hooks.ErrLockHeld) {
			t.Errorf("save failure %v reported as a lock timeout", err)
		}
		if len(skipped) != 0 {
			t.Errorf("onSkipped invocations = %v, want none on a save failure", skipped)
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
		store, _ := newStagedHooksStore(t, hooksStoreStaging{
			dir:          filepath.Join(t.TempDir(), hooks.ErrLockHeld.Error()),
			seed:         staleHookSeed,
			writesDenied: true,
		})
		sink := installHooksSink(t)

		var skipped []string
		err := runHookStaleCleanup(lister, store, nil, nil,
			func(reason string) { skipped = append(skipped, reason) })
		if err == nil {
			t.Fatal("runHookStaleCleanup: want an error, got nil")
		}
		if !strings.Contains(err.Error(), hooks.ErrLockHeld.Error()) {
			t.Fatalf("fixture error %q does not carry the sentinel's text; the case measures nothing", err)
		}
		if len(skipped) != 0 {
			t.Errorf("onSkipped invocations = %v, want none for a non-sentinel error", skipped)
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
		transienttest.HoldHooksSidecar(t, hooksPath)
		before := readFileBytes(t, hooksPath)

		outBuf, _, _ := runDoctorFixCmd(t, deps)

		assertSkippedPruneLine(t, outBuf.String(), "Skipped stale hook prune: hooks.json is locked")
		assertHooksFileUnchanged(t, hooksPath, before, "rewritten on a lock stand-down")
	})

	// The read side degrades where the write side stands down, so the un-pruned
	// entry is reported as the stale hook it is rather than as a lock problem.
	t.Run("it reports the un-pruned entry as stale in the same window", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		store, path := newTempHooksStore(t, staleHookSeed)
		transienttest.HoldHooksSidecar(t, path)

		got := checkStaleHooks(&stubAllPaneLister{rows: tokenRows(liveSeedA)}, store)
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
		hookStore, hooksPath := seedHooksJSON(t, liveSeedA)
		transienttest.HoldHooksSidecar(t, hooksPath)
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, fakeHookLister{rows: tokenRows(liveSeedA)}, hookStore, projectStore)

		outBuf, _, err := runDoctorFixCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; want nil over a healthy post-repair diagnosis\n%s", err, outBuf.String())
		}
		if !strings.Contains(outBuf.String(), "Skipped stale hook prune: hooks.json is locked") {
			t.Fatalf("fixture did not stand the prune down:\n%s", outBuf.String())
		}

		// The same stand-down with a genuinely failing check still exits non-zero.
		failingDir := t.TempDir()
		seedHealthyStateDir(t, failingDir)
		failingHooks, failingPath := seedHooksJSON(t, liveSeedA)
		transienttest.HoldHooksSidecar(t, failingPath)
		failingProjects, _ := seedProjectsJSON(t, t.TempDir())
		failingDeps := staleDeps(failingDir, fakeHookLister{rows: tokenRows(liveSeedA)}, failingHooks, failingProjects)
		failingDeps.SaverPresent = func() (bool, error) { return false, nil }

		failBuf, _, failErr := runDoctorFixCmd(t, failingDeps)
		if !errors.Is(failErr, ErrDoctorUnhealthy) {
			t.Fatalf("Execute err = %v; want ErrDoctorUnhealthy with a failing check\n%s", failErr, failBuf.String())
		}
	})
}
