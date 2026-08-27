package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
)

type recordingHookKeyLister struct {
	panes        []string
	err          error
	hookKeyCalls int
}

func (r *recordingHookKeyLister) ListAllPaneHookKeys() ([]string, error) {
	r.hookKeyCalls++
	return r.panes, r.err
}

var _ AllPaneLister = (*recordingHookKeyLister)(nil)

func TestRunHookStaleCleanup(t *testing.T) {
	const entryDebugFmt = "stale-hook cleanup counts"
	const completionDebugFmt = "stale-hook cleanup removed"
	const hazardWarnFmt = "stale-hook cleanup: zero live panes parsed with hooks present; skipping to avoid mass-deletion hazard (next bootstrap retries)"
	const listPanesWarnFmt = "stale-hook cleanup: list-panes failed"
	const loadWarnFmt = "stale-hook cleanup: hookStore.Load failed"

	t.Run("hazard guard fires on empty live + non-empty persisted", func(t *testing.T) {
		seed := `{
  "keyA00": {"on-resume": "cmd-a"},
  "keyB00": {"on-resume": "cmd-b"}
}`
		store, path := newTempHooksStore(t, seed)
		before := readFileBytes(t, path)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{panes: []string{}, err: nil}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		after := readFileBytes(t, path)
		if !reflect.DeepEqual(before, after) {
			t.Errorf("hooks.json modified by hazard-guard branch: before=%q after=%q", before, after)
		}

		if got := countMatching(logger.entries, "debug", "bootstrap", entryDebugFmt); got != 1 {
			t.Errorf("entry-point Debug count = %d, want 1; entries=%+v", got, logger.entries)
		}

		if got := countMatching(logger.entries, "warn", "bootstrap", hazardWarnFmt); got != 1 {
			t.Errorf("hazard Warn count = %d, want 1; entries=%+v", got, logger.entries)
		}

		if got := countMatching(logger.entries, "debug", "bootstrap", completionDebugFmt); got != 0 {
			t.Errorf("completion Debug count = %d, want 0 (must NOT fire on hazard branch); entries=%+v", got, logger.entries)
		}
	})

	t.Run("both-sides-empty no-op", func(t *testing.T) {
		store, path := newTempHooksStore(t, "")
		before := readFileBytes(t, path)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{panes: []string{}, err: nil}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil); err != nil {
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
		seed := `{"keyA00": {"on-resume": "cmd-a"}}`
		store, path := newTempHooksStore(t, seed)
		before := readFileBytes(t, path)

		sentinel := errors.New("tmux dead")
		logger := &recordingLogger{}
		lister := &stubAllPaneLister{panes: nil, err: sentinel}

		err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil)
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

		if got := countMatching(logger.entries, "warn", "bootstrap", hazardWarnFmt); got != 0 {
			t.Errorf("hazard Warn count = %d, want 0 on ListAllPanes-error branch; entries=%+v", got, logger.entries)
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
		lister := &stubAllPaneLister{panes: []string{"a:0.0"}, err: nil}

		err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil)
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
		seed := `{
  "a:0.0": {"on-resume": "cmd-a"},
  "keyB00": {"on-resume": "cmd-b"},
  "keyC00": {"on-resume": "cmd-c"},
  "keyD00": {"on-resume": "cmd-d"}
}`
		store, _ := newTempHooksStore(t, seed)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{panes: []string{"a:0.0"}, err: nil}

		var removedSeen []string
		onRemoved := func(name string) {
			removedSeen = append(removedSeen, name)
		}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), onRemoved); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		// CleanStale iterates a map, so removal order varies — assert set equality.
		want := map[string]struct{}{"keyB00": {}, "keyC00": {}, "keyD00": {}}
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
		seed := `{
  "a:0.0": {"on-resume": "cmd-a"},
  "keyB00": {"on-resume": "cmd-b"}
}`
		store, _ := newTempHooksStore(t, seed)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{panes: []string{"a:0.0"}, err: nil}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if got := countMatching(logger.entries, "debug", "bootstrap", completionDebugFmt); got != 1 {
			t.Errorf("completion Debug count = %d, want 1; entries=%+v", got, logger.entries)
		}
	})

	t.Run("happy-path normal removal emits entry + completion Debug", func(t *testing.T) {
		seed := `{
  "a:0.0": {"on-resume": "cmd-a"},
  "b:0.0": {"on-resume": "cmd-b"},
  "c:0.0": {"on-resume": "cmd-c"},
  "keyD00": {"on-resume": "cmd-d"}
}`
		store, _ := newTempHooksStore(t, seed)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{panes: []string{"a:0.0", "b:0.0", "c:0.0"}, err: nil}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load()
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		wantKeys := map[string]struct{}{"a:0.0": {}, "b:0.0": {}, "c:0.0": {}}
		if len(postRun) != len(wantKeys) {
			t.Errorf("post-run hook count = %d (keys=%v), want %d", len(postRun), keysOf(postRun), len(wantKeys))
		}
		if _, ok := postRun["keyD00"]; ok {
			t.Errorf("post-run hooks still contains stale key keyD00; got %v", keysOf(postRun))
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
		seed := `{"a:0.0": {"on-resume": "cmd-a"}}`
		store, _ := newTempHooksStore(t, seed)
		lister := &stubAllPaneLister{panes: []string{"a:0.0"}, err: nil}

		if err := runHookStaleCleanup(lister, store, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup with nil logger: %v", err)
		}
	})

	t.Run("it enumerates live keys via ListAllPaneHookKeys not ListAllPanes", func(t *testing.T) {
		seed := `{"a:0.0": {"on-resume": "cmd-a"}}`
		store, _ := newTempHooksStore(t, seed)

		logger := &recordingLogger{}
		rec := &recordingHookKeyLister{panes: []string{"a:0.0"}}

		if err := runHookStaleCleanup(rec, store, logger.Logger().With("component", "bootstrap"), nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		if rec.hookKeyCalls != 1 {
			t.Errorf("ListAllPaneHookKeys call count = %d, want 1 (the enumeration must switch to the hook-key method)", rec.hookKeyCalls)
		}
	})

	t.Run("it preserves a stamped-session hook whose id-key matches the live set", func(t *testing.T) {
		seed := `{
  "tok123:0.0": {"on-resume": "cmd-live"},
  "orpha1": {"on-resume": "cmd-stale"}
}`
		store, _ := newTempHooksStore(t, seed)

		logger := &recordingLogger{}
		lister := &stubAllPaneLister{panes: []string{"tok123:0.0"}, err: nil}

		if err := runHookStaleCleanup(lister, store, logger.Logger().With("component", "bootstrap"), nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load()
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun["tok123:0.0"]; !ok {
			t.Errorf("stamped-session hook tok123:0.0 was removed; want preserved (present in live set); got %v", keysOf(postRun))
		}
		if _, ok := postRun["orpha1"]; ok {
			t.Errorf("truly-stale hook orpha1 survived; want removed; got %v", keysOf(postRun))
		}
	})
}
