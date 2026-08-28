package cmd

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/transienttest"
)

// The hook-key seed vocabulary the cmd suites share: a reapable key is one the
// staleness rule can judge, so it is swept once absent from the live set; an
// unjudgeable key is retained whatever the live set says.
var (
	reapableSeedA = transienttest.ReapableHookKey(0)
	reapableSeedB = transienttest.ReapableHookKey(1)
	reapableSeedC = transienttest.ReapableHookKey(2)
	reapableSeedD = transienttest.ReapableHookKey(3)

	unjudgeableSeedA = transienttest.UnjudgeableHookKey(0)
	unjudgeableSeedB = transienttest.UnjudgeableHookKey(1)

	// The live half of the vocabulary: token-shaped keys the enumeration
	// reports, so an entry under one is preserved because its pane is live and
	// not because the reaper cannot judge its shape.
	liveSeedA = transienttest.ReapableHookKey(4)
	liveSeedB = transienttest.ReapableHookKey(5)
	liveSeedC = transienttest.ReapableHookKey(6)
)

// restoringOption models the @portal-restoring read for the sweep's seam fakes:
// absent by default, so a fake that says nothing about a restore is not
// restoring.
func restoringOption(restoring bool, readErr error) (string, bool, error) {
	if readErr != nil {
		return "", false, readErr
	}
	if !restoring {
		return "", false, nil
	}
	return "1", true, nil
}

type recordingHookKeyLister struct {
	rows         []tmux.PaneHookRow
	err          error
	hookKeyCalls int
	restoring    bool
	restoringErr error
}

func (r *recordingHookKeyLister) ListAllPaneHookKeys() ([]tmux.PaneHookRow, error) {
	r.hookKeyCalls++
	return r.rows, r.err
}

func (r *recordingHookKeyLister) TryGetServerOption(string) (string, bool, error) {
	return restoringOption(r.restoring, r.restoringErr)
}

var _ AllPaneLister = (*recordingHookKeyLister)(nil)

func TestRunHookStaleCleanup(t *testing.T) {
	const entryDebugFmt = "stale-hook cleanup counts"
	const completionDebugFmt = "stale-hook cleanup removed"
	const listPanesWarnFmt = "stale-hook cleanup: list-panes failed"
	const loadWarnFmt = "stale-hook cleanup: hookStore.Load failed"

	t.Run("hazard guard fires on empty live + non-empty persisted", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-a"},
  %q: {"on-resume": "cmd-b"}
}`, reapableSeedA, reapableSeedB)
		store, path := newTempHooksStore(t, seed)
		before := readFileBytes(t, path)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{rows: tokenRows(), err: nil}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		after := readFileBytes(t, path)
		if !reflect.DeepEqual(before, after) {
			t.Errorf("hooks.json modified by hazard-guard branch: before=%q after=%q", before, after)
		}

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
		store, path := newTempHooksStore(t, "")
		before := readFileBytes(t, path)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{rows: tokenRows(), err: nil}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		after := readFileBytes(t, path)
		if !reflect.DeepEqual(before, after) {
			t.Errorf("hooks.json materialised under both-sides-empty path: before=%v after=%v", before, after)
		}

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

	t.Run("ListAllPanes error logs Warn and returns nil", func(t *testing.T) {
		seed := fmt.Sprintf(`{%q: {"on-resume": "cmd-a"}}`, reapableSeedA)
		store, path := newTempHooksStore(t, seed)
		before := readFileBytes(t, path)

		sentinel := errors.New("tmux dead")
		logger := &recordingLogger{}
		lister := &stubAllPaneLister{rows: nil, err: sentinel}

		err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil, nil)
		if err != nil {
			t.Fatalf("runHookStaleCleanup on ListAllPanes error: want nil, got %v", err)
		}

		after := readFileBytes(t, path)
		if !reflect.DeepEqual(before, after) {
			t.Errorf("hooks.json modified on ListAllPanes-error path: before=%q after=%q", before, after)
		}

		if got := countMatching(logger.entries, "warn", "bootstrap", listPanesWarnFmt); got != 1 {
			t.Errorf("list-panes Warn count = %d, want 1; entries=%+v", got, logger.entries)
		}

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
		lister := &stubAllPaneLister{rows: tokenRows(liveSeedA), err: nil}

		err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil, nil)
		if err == nil {
			t.Fatalf("runHookStaleCleanup: want Load error, got nil")
		}

		if got := countMatching(logger.entries, "warn", "bootstrap", loadWarnFmt); got != 1 {
			t.Errorf("hookStore.Load Warn count = %d, want 1; entries=%+v", got, logger.entries)
		}

		if got := countMatching(logger.entries, "debug", "bootstrap", entryDebugFmt); got != 0 {
			t.Errorf("entry-point Debug count = %d, want 0 on Load-error branch; entries=%+v", got, logger.entries)
		}
	})

	t.Run("onRemoved invoked once per removed entry", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-a"},
  %q: {"on-resume": "cmd-b"},
  %q: {"on-resume": "cmd-c"},
  %q: {"on-resume": "cmd-d"}
}`, liveSeedA, reapableSeedB, reapableSeedC, reapableSeedD)
		store, _ := newTempHooksStore(t, seed)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{rows: tokenRows(liveSeedA), err: nil}

		var removedSeen []string
		onRemoved := func(name string) {
			removedSeen = append(removedSeen, name)
		}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), onRemoved, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		// CleanStale iterates a map, so removal order varies — assert set equality.
		want := map[string]struct{}{reapableSeedB: {}, reapableSeedC: {}, reapableSeedD: {}}
		if len(removedSeen) != len(want) {
			t.Errorf("onRemoved invocations = %d (%v), want %d (%v)", len(removedSeen), removedSeen, len(want), want)
		}
		for _, k := range removedSeen {
			if _, ok := want[k]; !ok {
				t.Errorf("onRemoved invoked with unexpected key %q; want one of %v", k, want)
			}
		}
	})

	t.Run("nil onRemoved is safe under normal removal", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-a"},
  %q: {"on-resume": "cmd-b"}
}`, liveSeedA, reapableSeedB)
		store, _ := newTempHooksStore(t, seed)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{rows: tokenRows(liveSeedA), err: nil}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if got := countMatching(logger.entries, "debug", "bootstrap", completionDebugFmt); got != 1 {
			t.Errorf("completion Debug count = %d, want 1; entries=%+v", got, logger.entries)
		}
	})

	t.Run("happy-path normal removal emits entry + completion Debug", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-a"},
  %q: {"on-resume": "cmd-b"},
  %q: {"on-resume": "cmd-c"},
  %q: {"on-resume": "cmd-d"}
}`, liveSeedA, liveSeedB, liveSeedC, reapableSeedD)
		store, _ := newTempHooksStore(t, seed)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{rows: tokenRows(liveSeedA, liveSeedB, liveSeedC), err: nil}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load()
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
		store, _ := newTempHooksStore(t, seed)
		lister := &stubAllPaneLister{rows: tokenRows(liveSeedA), err: nil}

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup with nil logger: %v", err)
		}
	})

	t.Run("it enumerates live keys via ListAllPaneHookKeys not ListAllPanes", func(t *testing.T) {
		seed := fmt.Sprintf(`{%q: {"on-resume": "cmd-a"}}`, liveSeedA)
		store, _ := newTempHooksStore(t, seed)

		logger := &recordingLogger{}
		rec := &recordingHookKeyLister{rows: tokenRows(liveSeedA)}

		if err := runHookStaleCleanup(rec, store, logger.Logger().With("component", "bootstrap"), nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if rec.hookKeyCalls != 1 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 1 (the enumeration must switch to the hook-key method)", rec.hookKeyCalls)
		}
	})

	t.Run("it preserves a hook whose token matches the live set", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-live"},
  %q: {"on-resume": "cmd-stale"}
}`, liveSeedA, reapableSeedA)
		store, _ := newTempHooksStore(t, seed)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{rows: tokenRows(liveSeedA), err: nil}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load()
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
		store, path := newTempHooksStore(t, seed)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read seeded hooks.json: %v", err)
		}

		lister := &stubAllPaneLister{rows: tokenRows(liveSeedA)}
		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load()
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[retained]; !ok {
			t.Errorf("non-token-shaped key was swept; got %v", keysOf(postRun))
		}

		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("re-read hooks.json: %v", err)
		}
		if !bytes.Equal(before, after) {
			t.Errorf("hooks.json rewritten with nothing to remove\nbefore: %s\nafter:  %s", before, after)
		}
	})

	t.Run("it retains a non-token-shaped key across doctor --fix", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, hooksPath := seedHooksJSON(t, unjudgeableSeedA)
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, fakeHookLister{rows: tokenRows(liveSeedA)}, hookStore, projectStore)

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
		hookStore, _ := seedHooksJSON(t, unjudgeableSeedA, unjudgeableSeedB)
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, fakeHookLister{rows: tokenRows(liveSeedA)}, hookStore, projectStore)

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
	staleKey := reapableSeedA
	staleSeed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-gone"},
  %q: {"on-resume": "cmd-live"}
}`, staleKey, liveSeedA)

	t.Run("it deletes nothing while the restore marker is set", func(t *testing.T) {
		store, path := newTempHooksStore(t, staleSeed)
		before := readFileBytes(t, path)

		lister := &recordingHookKeyLister{rows: tokenRows(liveSeedA), restoring: true}

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if after := readFileBytes(t, path); !bytes.Equal(before, after) {
			t.Errorf("hooks.json rewritten during a restore\nbefore: %s\nafter:  %s", before, after)
		}
		if lister.hookKeyCalls != 0 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 0 (the sweep must stand down before enumerating)", lister.hookKeyCalls)
		}
	})

	t.Run("it treats a failed marker read as a set marker", func(t *testing.T) {
		store, path := newTempHooksStore(t, staleSeed)
		before := readFileBytes(t, path)

		sentinel := errors.New("tmux dead")
		lister := &recordingHookKeyLister{rows: tokenRows(liveSeedA), restoringErr: sentinel}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if after := readFileBytes(t, path); !bytes.Equal(before, after) {
			t.Errorf("hooks.json rewritten on a failed marker read\nbefore: %s\nafter:  %s", before, after)
		}
		if lister.hookKeyCalls != 0 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 0 on a failed marker read", lister.hookKeyCalls)
		}

		rec := standDownRecord(t, sink)
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

		lister := &recordingHookKeyLister{rows: tokenRows(liveSeedA), restoring: true}

		var removed []string
		onRemoved := func(key string) { removed = append(removed, key) }

		if err := runHookStaleCleanup(lister, store, nil, onRemoved, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: want nil (store untouched), got %v", err)
		}
		if lister.hookKeyCalls != 0 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 0", lister.hookKeyCalls)
		}
		if len(removed) != 0 {
			t.Errorf("onRemoved invoked with %v, want no invocations", removed)
		}
	})

	t.Run("it logs the stand-down at DEBUG and never WARN", func(t *testing.T) {
		store, _ := newTempHooksStore(t, staleSeed)
		lister := &recordingHookKeyLister{rows: tokenRows(liveSeedA), restoring: true}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		rec := standDownRecord(t, sink)
		if rec.HasAttr("error") {
			t.Errorf("stand-down record carries an error attr with no read failure: %+v", rec.Attrs)
		}
	})

	t.Run("it sweeps normally when the marker is absent", func(t *testing.T) {
		store, _ := newTempHooksStore(t, staleSeed)
		lister := &recordingHookKeyLister{rows: tokenRows(liveSeedA)}

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if lister.hookKeyCalls != 1 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 1", lister.hookKeyCalls)
		}
		postRun, err := store.Load()
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[staleKey]; ok {
			t.Errorf("stale key %s survived a marker-absent sweep; got %v", staleKey, keysOf(postRun))
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
		store, _ := newTempHooksStore(t, staleSeed)
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
		postRun, err := store.Load()
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[staleKey]; !ok {
			t.Errorf("tick reaped a hook despite the restoring marker; got %v", keysOf(postRun))
		}
	})
}

// standDownRecord asserts the single captured record is the stand-down line and
// returns it for per-case attr assertions.
func standDownRecord(t *testing.T, sink *logtest.Sink) logtest.Record {
	t.Helper()

	for _, r := range sink.Records() {
		if r.Level >= slog.LevelWarn {
			t.Errorf("stand-down emitted at %v: %+v", r.Level, r)
		}
	}

	rec := sink.OnlyRecord(t)
	if rec.Level != slog.LevelDebug {
		t.Errorf("stand-down level = %v, want DEBUG", rec.Level)
	}
	if rec.Msg != "clean-stale-skipped" {
		t.Errorf("stand-down message = %q, want %q", rec.Msg, "clean-stale-skipped")
	}
	if got := rec.AttrString(t, "component"); got != "hooks" {
		t.Errorf("component = %q, want %q", got, "hooks")
	}
	if got := rec.AttrString(t, "op"); got != "clean-stale-skipped" {
		t.Errorf("op = %q, want %q", got, "clean-stale-skipped")
	}
	if got := rec.AttrString(t, "via"); got != "internal" {
		t.Errorf("via = %q, want %q", got, "internal")
	}
	if got := rec.AttrString(t, "reason"); got != "restoring" {
		t.Errorf("reason = %q, want %q", got, "restoring")
	}
	return rec
}

// A repair that silently did not run is indistinguishable from a repair that
// found nothing to do, so every stand-down reports itself to its caller.
func TestHookSweepReportsStandDown(t *testing.T) {
	staleKey := reapableSeedA
	staleSeed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-gone"},
  %q: {"on-resume": "cmd-live"}
}`, staleKey, liveSeedA)

	t.Run("it reports the restore stand-down to the caller", func(t *testing.T) {
		store, path := newTempHooksStore(t, staleSeed)
		before := readFileBytes(t, path)
		lister := &recordingHookKeyLister{rows: tokenRows(liveSeedA), restoring: true}

		var skipped, removed []string
		err := runHookStaleCleanup(lister, store, nil,
			func(key string) { removed = append(removed, key) },
			func(reason string) { skipped = append(skipped, reason) },
		)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if len(skipped) != 1 || skipped[0] != "restoring" {
			t.Errorf("onSkipped invocations = %v, want [restoring]", skipped)
		}
		if len(removed) != 0 {
			t.Errorf("onRemoved invoked with %v, want no invocations", removed)
		}
		if after := readFileBytes(t, path); !bytes.Equal(before, after) {
			t.Errorf("hooks.json rewritten on the restore stand-down\nbefore: %s\nafter:  %s", before, after)
		}
	})

	t.Run("it reports the empty-pane-read stand-down to the caller", func(t *testing.T) {
		store, path := newTempHooksStore(t, staleSeed)
		before := readFileBytes(t, path)
		lister := &stubAllPaneLister{rows: tokenRows()}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		injected := &recordingLogger{}
		var skipped, removed []string
		err := runHookStaleCleanup(lister, store, injected.Logger().With("component", "bootstrap"),
			func(key string) { removed = append(removed, key) },
			func(reason string) { skipped = append(skipped, reason) },
		)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if len(skipped) != 1 || skipped[0] != "empty-pane-read" {
			t.Errorf("onSkipped invocations = %v, want [empty-pane-read]", skipped)
		}
		if len(removed) != 0 {
			t.Errorf("onRemoved invoked with %v, want no invocations", removed)
		}
		if after := readFileBytes(t, path); !bytes.Equal(before, after) {
			t.Errorf("hooks.json rewritten on the empty-pane-read stand-down\nbefore: %s\nafter:  %s", before, after)
		}

		rec := sink.OnlyRecord(t)
		if rec.Level != slog.LevelWarn {
			t.Errorf("stand-down level = %v, want WARN", rec.Level)
		}
		if rec.Msg != "clean-stale-skipped" {
			t.Errorf("stand-down message = %q, want %q", rec.Msg, "clean-stale-skipped")
		}
		if got := rec.AttrString(t, "component"); got != "hooks" {
			t.Errorf("component = %q, want %q", got, "hooks")
		}
		if got := rec.AttrString(t, "op"); got != "clean-stale-skipped" {
			t.Errorf("op = %q, want %q", got, "clean-stale-skipped")
		}
		if got := rec.AttrString(t, "via"); got != "internal" {
			t.Errorf("via = %q, want %q", got, "internal")
		}
		if got := rec.AttrString(t, "reason"); got != "empty-pane-read" {
			t.Errorf("reason = %q, want %q", got, "empty-pane-read")
		}
		if got := rec.IntAttr(t, "entries"); got != 2 {
			t.Errorf("entries = %d, want 2", got)
		}
	})

	t.Run("it survives a nil onSkipped on both stand-down branches", func(t *testing.T) {
		restoringStore, _ := newTempHooksStore(t, staleSeed)
		restoringLister := &recordingHookKeyLister{rows: tokenRows(liveSeedA), restoring: true}
		if err := runHookStaleCleanup(restoringLister, restoringStore, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup on the restore branch: %v", err)
		}

		emptyStore, _ := newTempHooksStore(t, staleSeed)
		emptyLister := &stubAllPaneLister{rows: tokenRows()}
		if err := runHookStaleCleanup(emptyLister, emptyStore, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup on the empty-pane-read branch: %v", err)
		}
	})

	t.Run("it reports nothing when the live set is empty and no entries are persisted", func(t *testing.T) {
		store, _ := newTempHooksStore(t, "")
		lister := &stubAllPaneLister{rows: tokenRows()}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		injected := &recordingLogger{}
		var skipped []string
		err := runHookStaleCleanup(lister, store, injected.Logger().With("component", "bootstrap"), nil,
			func(reason string) { skipped = append(skipped, reason) },
		)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if len(skipped) != 0 {
			t.Errorf("onSkipped invocations = %v, want none with nothing to protect", skipped)
		}
		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("hooks-component records = %+v, want none", recs)
		}
	})

	t.Run("it emits no skipped line when the enumeration errors", func(t *testing.T) {
		store, _ := newTempHooksStore(t, staleSeed)
		lister := &stubAllPaneLister{err: errors.New("tmux dead")}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		injected := &recordingLogger{}
		var skipped []string
		err := runHookStaleCleanup(lister, store, injected.Logger().With("component", "bootstrap"), nil,
			func(reason string) { skipped = append(skipped, reason) },
		)
		if err != nil {
			t.Fatalf("runHookStaleCleanup on an enumeration error: want nil, got %v", err)
		}

		if len(skipped) != 0 {
			t.Errorf("onSkipped invocations = %v, want none on a hard read error", skipped)
		}
		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("hooks-component records = %+v, want none on a hard read error", recs)
		}
		if got := countMatching(injected.entries, "warn", "bootstrap", "stale-hook cleanup: list-panes failed"); got != 1 {
			t.Errorf("list-panes Warn count = %d, want 1; entries=%+v", got, injected.entries)
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
  "live:0.0": {"on-resume": "cmd-live"}
}`, unjudgeableSeedA)
		store, path := newTempHooksStore(t, seed)
		before := readFileBytes(t, path)

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		injected := &recordingLogger{}
		var skipped []string
		err := runHookStaleCleanup(&stubAllPaneLister{rows: unstampedRows(3)}, store,
			injected.Logger().With("component", "bootstrap"), nil,
			func(reason string) { skipped = append(skipped, reason) },
		)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if len(skipped) != 0 {
			t.Errorf("onSkipped invocations = %v, want none (rows present means the read succeeded)", skipped)
		}
		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("hooks-component records = %+v, want none (no hazard exists at zero stamped panes)", recs)
		}
		for _, e := range injected.entries {
			if e.level == "warn" {
				t.Errorf("unexpected Warn with rows present and no token: %+v", e)
			}
		}
		if after := readFileBytes(t, path); !bytes.Equal(before, after) {
			t.Errorf("hooks.json rewritten with only unjudgeable entries present\nbefore: %s\nafter:  %s", before, after)
		}
	})

	t.Run("it still deletes a token-shaped key absent from the live token set", func(t *testing.T) {
		seed := fmt.Sprintf(`{%q: {"on-resume": "cmd-gone"}}`, reapableSeedA)
		store, _ := newTempHooksStore(t, seed)

		lister := &stubAllPaneLister{rows: append(tokenRows(reapableSeedB), unstampedRows(1)...)}
		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load()
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[reapableSeedA]; ok {
			t.Errorf("token-shaped key %s absent from the live token set survived; got %v", reapableSeedA, keysOf(postRun))
		}
	})

	t.Run("it takes the empty-pane-read branch only on zero rows", func(t *testing.T) {
		seed := fmt.Sprintf(`{%q: {"on-resume": "cmd-gone"}}`, reapableSeedA)
		store, _ := newTempHooksStore(t, seed)

		var skipped []string
		err := runHookStaleCleanup(&stubAllPaneLister{rows: nil}, store, nil, nil,
			func(reason string) { skipped = append(skipped, reason) },
		)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if len(skipped) != 1 || skipped[0] != skipReasonEmptyPaneRead {
			t.Errorf("onSkipped invocations = %v, want [%s]", skipped, skipReasonEmptyPaneRead)
		}
		postRun, err := store.Load()
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[reapableSeedA]; !ok {
			t.Errorf("a zero-row read reaped an entry; got %v", keysOf(postRun))
		}
	})

	t.Run("it counts the rows, not the tokens, on the counts line", func(t *testing.T) {
		store, _ := newTempHooksStore(t, `{"live:0.0": {"on-resume": "cmd-live"}}`)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{rows: unstampedRows(4)}
		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil, nil); err != nil {
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
		store, _ := newTempHooksStore(t, seed)

		lister := &stubAllPaneLister{rows: unstampedRows(2)}
		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load()
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
