package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
)

func TestRunHookStaleCleanup(t *testing.T) {
	const entryDebugFmt = "stale-hook cleanup counts"
	const completionDebugFmt = "stale-hook cleanup removed"

	t.Run("hazard guard fires on empty live + non-empty persisted", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-a"},
  %q: {"on-resume": "cmd-b"}
}`, reapableSeedA, reapableSeedB)
		store, path := newStagedHooksStore(t, hooksStoreStaging{seed: seed})
		before := readFileBytes(t, path)

		logger := &recordingLogger{}
		lister := &stubStaleSweepReader{rows: tokenRows(), err: nil}

		if err := sweepErr(lister, store, logger.Logger().With("component", "bootstrap")); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		assertHooksFileUnchanged(t, path, before, "modified by hazard-guard branch")

		if got := countMatching(logger.entries, "debug", "bootstrap", entryDebugFmt); got != 1 {
			t.Errorf("entry-point Debug count = %d, want 1; entries=%+v", got, logger.entries)
		}

		// The guard's own WARN rides the hooks component, not the injected
		// bootstrap/daemon logger.
		for _, e := range logger.entries {
			if e.level == "warn" {
				t.Errorf("unexpected Warn on the injected logger under the hazard guard: %+v", e)
			}
		}

		if got := countMatching(logger.entries, "debug", "bootstrap", completionDebugFmt); got != 0 {
			t.Errorf("completion Debug count = %d, want 0 (must NOT fire on hazard branch); entries=%+v", got, logger.entries)
		}
	})

	t.Run("both-sides-empty no-op", func(t *testing.T) {
		store, path := newStagedHooksStore(t, hooksStoreStaging{seed: ""})
		before := readFileBytes(t, path)

		logger := &recordingLogger{}
		lister := &stubStaleSweepReader{rows: tokenRows(), err: nil}

		if err := sweepErr(lister, store, logger.Logger().With("component", "bootstrap")); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		assertHooksFileUnchanged(t, path, before, "materialised under both-sides-empty path")

		if got := countMatching(logger.entries, "debug", "bootstrap", entryDebugFmt); got != 1 {
			t.Errorf("entry-point Debug count = %d, want 1; entries=%+v", got, logger.entries)
		}

		for _, e := range logger.entries {
			if e.level == "warn" {
				t.Errorf("unexpected Warn under both-sides-empty: %+v", e)
			}
		}

		if got := countMatching(logger.entries, "debug", "bootstrap", completionDebugFmt); got != 0 {
			t.Errorf("completion Debug count = %d, want 0; entries=%+v", got, logger.entries)
		}
	})

	t.Run("ListAllPanes error stands the cycle down and returns nil", func(t *testing.T) {
		seed := fmt.Sprintf(`{%q: {"on-resume": "cmd-a"}}`, reapableSeedA)
		store, path := newStagedHooksStore(t, hooksStoreStaging{seed: seed})
		before := readFileBytes(t, path)

		sentinel := errors.New("tmux dead")
		logger := &recordingLogger{}
		lister := &stubStaleSweepReader{rows: nil, err: sentinel}

		outcome, err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"))
		if err != nil {
			t.Fatalf("runHookStaleCleanup on ListAllPanes error: want nil, got %v", err)
		}
		if outcome.DeclineReason != skipReasonPaneReadFailed {
			t.Errorf("DeclineReason = %q, want %q", outcome.DeclineReason, skipReasonPaneReadFailed)
		}

		assertHooksFileUnchanged(t, path, before, "modified on ListAllPanes-error path")

		if got := countMatching(logger.entries, "debug", "bootstrap", entryDebugFmt); got != 0 {
			t.Errorf("entry-point Debug count = %d, want 0 (must NOT fire on ListAllPanes-error branch); entries=%+v", got, logger.entries)
		}
	})

	t.Run("hookStore.Load error returns err with Warn", func(t *testing.T) {
		// A directory at the hooks.json path is what makes ReadFile fail: malformed
		// JSON would decode to an empty map instead of erroring.
		dir := t.TempDir()
		bogusPath := filepath.Join(dir, "hooks.json")
		if err := os.MkdirAll(bogusPath, 0o755); err != nil {
			t.Fatalf("mkdir bogus path: %v", err)
		}
		store := hooks.NewStore(bogusPath)

		logger := &recordingLogger{}
		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA), err: nil}

		sink := logtest.Install(t)

		outcome, err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"))
		if err == nil {
			t.Fatalf("runHookStaleCleanup: want Load error, got nil")
		}
		if outcome.DeclineReason != skipReasonStoreReadFailed {
			t.Errorf("DeclineReason = %q, want %q", outcome.DeclineReason, skipReasonStoreReadFailed)
		}

		assertStandDown(t, sink, slog.LevelWarn, skipReasonStoreReadFailed)

		if got := countMatching(logger.entries, "debug", "bootstrap", entryDebugFmt); got != 0 {
			t.Errorf("entry-point Debug count = %d, want 0 on Load-error branch; entries=%+v", got, logger.entries)
		}
	})

	t.Run("the outcome names every removed entry", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-a"},
  %q: {"on-resume": "cmd-b"},
  %q: {"on-resume": "cmd-c"},
  %q: {"on-resume": "cmd-d"}
}`, liveSeedA, reapableSeedB, reapableSeedC, reapableSeedD)
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: seed})

		logger := &recordingLogger{}
		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA), err: nil}

		outcome, err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"))
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}
		removedSeen := outcome.Removed

		// CleanStale iterates a map, so removal order varies — assert set equality.
		want := map[string]struct{}{reapableSeedB: {}, reapableSeedC: {}, reapableSeedD: {}}
		if len(removedSeen) != len(want) {
			t.Errorf("Removed = %d (%v), want %d (%v)", len(removedSeen), removedSeen, len(want), want)
		}
		for _, k := range removedSeen {
			if _, ok := want[k]; !ok {
				t.Errorf("Removed carries unexpected key %q; want one of %v", k, want)
			}
		}
	})

	t.Run("happy-path normal removal emits entry + completion Debug", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-a"},
  %q: {"on-resume": "cmd-b"},
  %q: {"on-resume": "cmd-c"},
  %q: {"on-resume": "cmd-d"}
}`, liveSeedA, liveSeedB, liveSeedC, reapableSeedD)
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: seed})

		logger := &recordingLogger{}
		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA, liveSeedB, liveSeedC), err: nil}

		if err := sweepErr(lister, store, logger.Logger().With("component", "bootstrap")); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		wantKeys := map[string]struct{}{liveSeedA: {}, liveSeedB: {}, liveSeedC: {}}
		if len(postRun) != len(wantKeys) {
			t.Errorf("post-run hook count = %d (keys=%v), want %d", len(postRun), keysOf(postRun), len(wantKeys))
		}
		if _, ok := postRun[reapableSeedD]; ok {
			t.Errorf("post-run hooks still contains stale key %s; got %v", reapableSeedD, keysOf(postRun))
		}

		if got := countMatching(logger.entries, "debug", "bootstrap", entryDebugFmt); got != 1 {
			t.Errorf("entry-point Debug count = %d, want 1; entries=%+v", got, logger.entries)
		}

		if got := countMatching(logger.entries, "debug", "bootstrap", completionDebugFmt); got != 1 {
			t.Errorf("completion Debug count = %d, want 1; entries=%+v", got, logger.entries)
		}

		for _, e := range logger.entries {
			if e.level == "warn" {
				t.Errorf("unexpected Warn on normal-removal path: %+v", e)
			}
		}
	})

	t.Run("nil logger does not panic", func(t *testing.T) {
		seed := fmt.Sprintf(`{%q: {"on-resume": "cmd-a"}}`, liveSeedA)
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: seed})
		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA), err: nil}

		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup with nil logger: %v", err)
		}
	})

	t.Run("it enumerates live keys via ListAllPaneHookKeys not ListAllPanes", func(t *testing.T) {
		seed := fmt.Sprintf(`{%q: {"on-resume": "cmd-a"}}`, liveSeedA)
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: seed})

		logger := &recordingLogger{}
		rec := &stubStaleSweepReader{rows: tokenRows(liveSeedA)}

		if err := sweepErr(rec, store, logger.Logger().With("component", "bootstrap")); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if rec.calls != 1 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 1 (the enumeration must switch to the hook-key method)", rec.calls)
		}
	})

	t.Run("it preserves a hook whose token matches the live set", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-live"},
  %q: {"on-resume": "cmd-stale"}
}`, liveSeedA, reapableSeedA)
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: seed})

		logger := &recordingLogger{}
		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA), err: nil}

		if err := sweepErr(lister, store, logger.Logger().With("component", "bootstrap")); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[liveSeedA]; !ok {
			t.Errorf("hook %s keyed on a live pane's token was removed; want preserved (present in live set); got %v", liveSeedA, keysOf(postRun))
		}
		if _, ok := postRun[reapableSeedA]; ok {
			t.Errorf("truly-stale hook %s survived; want removed; got %v", reapableSeedA, keysOf(postRun))
		}
	})
}

// A key the reaper cannot parse as a token is not evidence of a dead pane, so
// neither entry point may take it — however the sweep is reached.
func TestUnjudgeableHookKeyRetention(t *testing.T) {
	t.Run("it retains a non-token-shaped key across the daemon sweep", func(t *testing.T) {
		retained := unjudgeableSeedA
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-live"},
  %q: {"on-resume": "cmd-old"}
}`, liveSeedA, retained)
		store, path := newStagedHooksStore(t, hooksStoreStaging{seed: seed})
		before := readFileBytes(t, path)

		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA)}
		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[retained]; !ok {
			t.Errorf("non-token-shaped key was swept; got %v", keysOf(postRun))
		}

		assertHooksFileUnchanged(t, path, before, "rewritten with nothing to remove")
	})

	t.Run("it retains a non-token-shaped key across doctor --fix", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, hooksPath := newStagedHooksStore(t, hooksStoreStaging{seed: hooksBody(unjudgeableSeedA)})
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, &stubStaleSweepReader{rows: tokenRows(liveSeedA)}, hookStore, projectStore)

		outBuf, _, err := runDoctorFixCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; want nil (nothing to repair)", err)
		}

		after, err := os.ReadFile(hooksPath)
		if err != nil {
			t.Fatalf("read hooks.json: %v", err)
		}
		if !strings.Contains(string(after), unjudgeableSeedA) {
			t.Errorf("doctor --fix pruned the non-token-shaped entry:\n%s", after)
		}
		if strings.Contains(outBuf.String(), "Pruned stale hook: "+unjudgeableSeedA) {
			t.Errorf("doctor --fix reported pruning a retained entry:\n%s", outBuf.String())
		}
	})

	t.Run("it exits 0 from portal doctor with only retained non-token-shaped entries present", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, _ := newStagedHooksStore(t, hooksStoreStaging{seed: hooksBody(unjudgeableSeedA, unjudgeableSeedB)})
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, &stubStaleSweepReader{rows: tokenRows(liveSeedA)}, hookStore, projectStore)

		outBuf, _, err := runDoctorCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; want nil (retained entries are not a failing check)", err)
		}
		if !strings.Contains(outBuf.String(), "✓ stale hooks: no stale hooks") {
			t.Errorf("stale-hooks check did not pass:\n%s", outBuf.String())
		}
	})
}

// A restore window is a hole in the reaper's judgement: every live pane carries
// no token between skeleton construction and the re-stamp, so a sweep landing
// there would reap every token-keyed entry on the machine.
func TestHookSweepStandsDownWhileRestoring(t *testing.T) {
	t.Run("it deletes nothing while the restore marker is set", func(t *testing.T) {
		store, path := newStagedHooksStore(t, hooksStoreStaging{seed: staleHookSeed})
		before := readFileBytes(t, path)

		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA), restoring: true}

		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		assertHooksFileUnchanged(t, path, before, "rewritten during a restore")
		if lister.calls != 0 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 0 (the sweep must stand down before enumerating)", lister.calls)
		}
	})

	t.Run("it treats a failed marker read as a set marker", func(t *testing.T) {
		store, path := newStagedHooksStore(t, hooksStoreStaging{seed: staleHookSeed})
		before := readFileBytes(t, path)

		sentinel := errors.New("tmux dead")
		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA), restoringErr: sentinel}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		assertHooksFileUnchanged(t, path, before, "rewritten on a failed marker read")
		if lister.calls != 0 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 0 on a failed marker read", lister.calls)
		}

		rec := assertStandDown(t, sink, slog.LevelDebug, "restoring")
		if got := rec.AttrString(t, "error"); got != sentinel.Error() {
			t.Errorf("error attr = %q, want %q", got, sentinel.Error())
		}
	})

	t.Run("it skips before loading the store", func(t *testing.T) {
		// A directory at the hooks.json path makes any read fail loudly, so a
		// nil return proves the store was never loaded.
		bogusPath := filepath.Join(t.TempDir(), "hooks.json")
		if err := os.MkdirAll(bogusPath, 0o755); err != nil {
			t.Fatalf("mkdir bogus hooks path: %v", err)
		}
		store := hooks.NewStore(bogusPath)

		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA), restoring: true}

		outcome, err := runHookStaleCleanup(lister, store, nil)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: want nil (store untouched), got %v", err)
		}
		if lister.calls != 0 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 0", lister.calls)
		}
		if len(outcome.Removed) != 0 {
			t.Errorf("Removed = %v, want none", outcome.Removed)
		}
	})

	t.Run("it logs the stand-down at DEBUG and never WARN", func(t *testing.T) {
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: staleHookSeed})
		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA), restoring: true}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		rec := assertStandDown(t, sink, slog.LevelDebug, "restoring")
		if rec.HasAttr("error") {
			t.Errorf("stand-down record carries an error attr with no read failure: %+v", rec.Attrs)
		}
	})

	t.Run("it sweeps normally when the marker is absent", func(t *testing.T) {
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: staleHookSeed})
		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA)}

		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if lister.calls != 1 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 1", lister.calls)
		}
		postRun, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[reapableSeedA]; ok {
			t.Errorf("stale key %s survived a marker-absent sweep; got %v", reapableSeedA, keysOf(postRun))
		}
		if _, ok := postRun[liveSeedA]; !ok {
			t.Errorf("live key was reaped; got %v", keysOf(postRun))
		}
	})

	t.Run("it stands the doctor --fix prune down while restoring", func(t *testing.T) {
		deps, hooksPath, _, _, _ := seedStalePruneFixture(t, t.TempDir(), restoringHookLister())

		// The exit code is the diagnosis's business, not the prune's.
		outBuf, _, _ := runDoctorFixCmd(t, deps)

		after, err := os.ReadFile(hooksPath)
		if err != nil {
			t.Fatalf("read hooks.json: %v", err)
		}
		if !strings.Contains(string(after), reapableSeedA) {
			t.Errorf("doctor --fix pruned a hook during a restore:\n%s", after)
		}
		if strings.Contains(outBuf.String(), "Pruned stale hook:") {
			t.Errorf("doctor --fix reported a prune during a restore:\n%s", outBuf.String())
		}
	})

	t.Run("it leaves the daemon's tick behaviour unchanged", func(t *testing.T) {
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: staleHookSeed})
		fc := &daemonFakeCommander{
			panesOut:     livePaneRowOut,
			optionByName: map[string]string{state.RestoringMarkerName: "1"},
		}
		deps := hookCleanupDeps(fc, store, discardDaemonLogger())
		deps.Dir = t.TempDir()

		tick(context.Background(), deps)

		if got := fc.callsContaining("show-option"); len(got) != 1 {
			t.Errorf("tick made %d show-option reads, want 1 (its own marker read; the sweep must not be reached): %v", len(got), got)
		}
		if got := fc.callsContaining("list-panes"); len(got) != 0 {
			t.Errorf("tick enumerated panes despite the restoring marker: %v", got)
		}
		postRun, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[reapableSeedA]; !ok {
			t.Errorf("tick reaped a hook despite the restoring marker; got %v", keysOf(postRun))
		}
	})
}

// assertStandDown pins the stand-down breadcrumb at the level the sweep is
// expected to report it, and returns it for per-case attr assertions. The level
// picks the record set the count is taken over: a DEBUG stand-down is the only
// line the sink holds, so anything at WARN or above is itself a failure, while a
// WARN stand-down shares the sink with the degraded pre-read's own DEBUG
// breadcrumb and can only be counted among the WARNs.
func assertStandDown(t *testing.T, sink *logtest.Sink, level slog.Level, reason string) logtest.Record {
	t.Helper()

	var rec logtest.Record
	if level < slog.LevelWarn {
		for _, r := range sink.RecordsAtOrAboveLevel(slog.LevelWarn) {
			t.Errorf("stand-down emitted at %v: %+v", r.Level, r)
		}
		rec = sink.OnlyRecord(t)
	} else {
		at := sink.RecordsAtOrAboveLevel(level)
		if len(at) != 1 {
			t.Fatalf("%v record count = %d, want exactly 1: %+v", level, len(at), at)
		}
		rec = at[0]
	}

	assertStandDownRecord(t, rec, level, reason)
	return rec
}

// assertStandDownRecord pins the shape of a stand-down line already selected by
// its caller, for a case whose own selection is stronger than the level alone.
func assertStandDownRecord(t *testing.T, rec logtest.Record, level slog.Level, reason string) {
	t.Helper()
	assertHooksRecord(t, rec, standDownWant(level))
	if got := rec.AttrString(t, "reason"); got != reason {
		t.Errorf("reason = %q, want %q", got, reason)
	}
}

// standDownWant is the stand-down's half of the hooks record shape: every
// stand-down carries the same message, op and via, and differs only in the
// level it is reported at.
func standDownWant(level slog.Level) hooksRecordWant {
	return hooksRecordWant{level: level, msg: "clean-stale-skipped", op: "clean-stale-skipped", via: "internal"}
}

// A repair that silently did not run is indistinguishable from a repair that
// found nothing to do, so every stand-down reports itself to its caller.
func TestHookSweepReportsStandDown(t *testing.T) {
	t.Run("it reports the restore stand-down to the caller", func(t *testing.T) {
		store, path := newStagedHooksStore(t, hooksStoreStaging{seed: staleHookSeed})
		before := readFileBytes(t, path)
		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA), restoring: true}

		outcome, err := runHookStaleCleanup(lister, store, nil)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if outcome.DeclineReason != "restoring" {
			t.Errorf("DeclineReason = %q, want %q", outcome.DeclineReason, "restoring")
		}
		if len(outcome.Removed) != 0 {
			t.Errorf("Removed = %v, want none", outcome.Removed)
		}
		assertHooksFileUnchanged(t, path, before, "rewritten on the restore stand-down")
	})

	t.Run("it reports the empty-pane-read stand-down to the caller", func(t *testing.T) {
		store, path := newStagedHooksStore(t, hooksStoreStaging{seed: staleHookSeed})
		before := readFileBytes(t, path)
		lister := &stubStaleSweepReader{rows: tokenRows()}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		injected := &recordingLogger{}
		outcome, err := runHookStaleCleanup(lister, store, injected.Logger().With("component", "bootstrap"))
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if outcome.DeclineReason != "empty-pane-read" {
			t.Errorf("DeclineReason = %q, want %q", outcome.DeclineReason, "empty-pane-read")
		}
		if len(outcome.Removed) != 0 {
			t.Errorf("Removed = %v, want none", outcome.Removed)
		}
		assertHooksFileUnchanged(t, path, before, "rewritten on the empty-pane-read stand-down")

		rec := sink.OnlyRecord(t)
		assertStandDownRecord(t, rec, slog.LevelWarn, "empty-pane-read")
		if got := rec.IntAttr(t, "entries"); got != 2 {
			t.Errorf("entries = %d, want 2", got)
		}
	})

	t.Run("it reports nothing when the live set is empty and no entries are persisted", func(t *testing.T) {
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: ""})
		lister := &stubStaleSweepReader{rows: tokenRows()}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		injected := &recordingLogger{}
		outcome, err := runHookStaleCleanup(lister, store, injected.Logger().With("component", "bootstrap"))
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if outcome.DeclineReason != "" {
			t.Errorf("DeclineReason = %q, want none with nothing to protect", outcome.DeclineReason)
		}
		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("hooks-component records = %+v, want none", recs)
		}
	})

	t.Run("it reports the failed enumeration as a stand-down on the hooks component", func(t *testing.T) {
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: staleHookSeed})
		sentinel := errors.New("tmux dead")
		lister := &stubStaleSweepReader{err: sentinel}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		injected := &recordingLogger{}
		outcome, err := runHookStaleCleanup(lister, store, injected.Logger().With("component", "bootstrap"))
		if err != nil {
			t.Fatalf("runHookStaleCleanup on an enumeration error: want nil, got %v", err)
		}

		if outcome.DeclineReason != skipReasonPaneReadFailed {
			t.Errorf("DeclineReason = %q, want %q", outcome.DeclineReason, skipReasonPaneReadFailed)
		}
		rec := sink.OnlyRecord(t)
		assertStandDownRecord(t, rec, slog.LevelWarn, skipReasonPaneReadFailed)
		if got := rec.AttrString(t, "error"); got != sentinel.Error() {
			t.Errorf("error attr = %q, want %q", got, sentinel.Error())
		}
		for _, e := range injected.entries {
			if e.level == "warn" {
				t.Errorf("stand-down emitted on the injected logger as well as the hooks component: %+v", e)
			}
		}
	})
}

// Under lazy stamping a server with no stamped pane at all is the ordinary
// steady state, so the guard's question — did the tmux read succeed? — is
// answered by the pane rows, never by the tokens they carry.
func TestHookSweepGuardCountsPaneRowsNotTokens(t *testing.T) {
	t.Run("it does not fire the mass-deletion guard when no pane is stamped", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-old"},
  %q: {"on-resume": "cmd-live"}
}`, unjudgeableSeedA, unjudgeableSeedB)
		store, path := newStagedHooksStore(t, hooksStoreStaging{seed: seed})
		before := readFileBytes(t, path)

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		injected := &recordingLogger{}
		outcome, err := runHookStaleCleanup(&stubStaleSweepReader{rows: unstampedRows(3)}, store,
			injected.Logger().With("component", "bootstrap"))
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if outcome.DeclineReason != "" {
			t.Errorf("DeclineReason = %q, want none (rows present means the read succeeded)", outcome.DeclineReason)
		}
		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("hooks-component records = %+v, want none (no hazard exists at zero stamped panes)", recs)
		}
		for _, e := range injected.entries {
			if e.level == "warn" {
				t.Errorf("unexpected Warn with rows present and no token: %+v", e)
			}
		}
		assertHooksFileUnchanged(t, path, before, "rewritten with only unjudgeable entries present")
	})

	t.Run("it still deletes a token-shaped key absent from the live token set", func(t *testing.T) {
		seed := fmt.Sprintf(`{%q: {"on-resume": "cmd-gone"}}`, reapableSeedA)
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: seed})

		lister := &stubStaleSweepReader{rows: append(tokenRows(reapableSeedB), unstampedRows(1)...)}
		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[reapableSeedA]; ok {
			t.Errorf("token-shaped key %s absent from the live token set survived; got %v", reapableSeedA, keysOf(postRun))
		}
	})

	t.Run("it takes the empty-pane-read branch only on zero rows", func(t *testing.T) {
		seed := fmt.Sprintf(`{%q: {"on-resume": "cmd-gone"}}`, reapableSeedA)
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: seed})

		outcome, err := runHookStaleCleanup(&stubStaleSweepReader{rows: nil}, store, nil)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if outcome.DeclineReason != skipReasonEmptyPaneRead {
			t.Errorf("DeclineReason = %q, want %q", outcome.DeclineReason, skipReasonEmptyPaneRead)
		}
		postRun, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[reapableSeedA]; !ok {
			t.Errorf("a zero-row read reaped an entry; got %v", keysOf(postRun))
		}
	})

	t.Run("it counts the rows, not the tokens, on the counts line", func(t *testing.T) {
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: fmt.Sprintf(`{%q: {"on-resume": "cmd-live"}}`, unjudgeableSeedB)})

		logger := &recordingLogger{}
		lister := &stubStaleSweepReader{rows: unstampedRows(4)}
		if err := sweepErr(lister, store, logger.Logger().With("component", "bootstrap")); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		rec := onlyMatching(t, logger.entries, "debug", "bootstrap", "stale-hook cleanup counts")
		if got := rec.intAttr(t, "panes"); got != 4 {
			t.Errorf("panes = %d, want 4 (the count is of live panes; none of these carries a token)", got)
		}
	})

	t.Run("it reaps an empty key when a pane carries no token", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  "": {"on-resume": "cmd-empty"},
  %q: {"on-resume": "cmd-old"}
}`, unjudgeableSeedA)
		store, _ := newStagedHooksStore(t, hooksStoreStaging{seed: seed})

		lister := &stubStaleSweepReader{rows: unstampedRows(2)}
		if err := sweepErr(lister, store, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		// An unstamped pane protects no key. Were its empty token to enter the
		// live set, this entry would be permanently unreachable by the reaper
		// and would fire its command on every unstamped restored pane.
		if _, ok := postRun[""]; ok {
			t.Errorf("empty-keyed entry survived alongside unstamped panes; got %v", keysOf(postRun))
		}
		if _, ok := postRun[unjudgeableSeedA]; !ok {
			t.Errorf("unjudgeable entry was swept; got %v", keysOf(postRun))
		}
	})
}

// sweepErr drives the sweep and keeps only its error, for the cases that assert
// on hooks.json or on the log rather than on what the cycle reported.
func sweepErr(reader staleSweepReader, store *hooks.Store, logger *slog.Logger) error {
	_, err := runHookStaleCleanup(reader, store, logger)
	return err
}
