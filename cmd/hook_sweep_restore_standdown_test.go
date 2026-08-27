package cmd

import (
	"bytes"
	"context"
	"errors"
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

// A restore window is a hole in the reaper's judgement: every live pane carries
// no token between skeleton construction and the re-stamp, so a sweep landing
// there would reap every token-keyed entry on the machine.
func TestHookSweepStandsDownWhileRestoring(t *testing.T) {
	const staleSeed = `{
  "gone01": {"on-resume": "cmd-gone"},
  "live:0.0": {"on-resume": "cmd-live"}
}`

	t.Run("it deletes nothing while the restore marker is set", func(t *testing.T) {
		store, path := newTempHooksStore(t, staleSeed)
		before := readFileBytes(t, path)

		lister := &recordingHookKeyLister{panes: []string{"live:0.0"}, restoring: true}

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
		lister := &recordingHookKeyLister{panes: []string{"live:0.0"}, restoringErr: sentinel}

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

		lister := &recordingHookKeyLister{panes: []string{"live:0.0"}, restoring: true}

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
		lister := &recordingHookKeyLister{panes: []string{"live:0.0"}, restoring: true}

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
		lister := &recordingHookKeyLister{panes: []string{"live:0.0"}}

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
		if _, ok := postRun["gone01"]; ok {
			t.Errorf("stale key gone01 survived a marker-absent sweep; got %v", keysOf(postRun))
		}
		if _, ok := postRun["live:0.0"]; !ok {
			t.Errorf("live key was reaped; got %v", keysOf(postRun))
		}
	})

	t.Run("it stands the doctor --fix prune down while restoring", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, hooksPath := seedHooksJSON(t, "sessA1")
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		lister := fakeHookLister{keys: []string{"sessB:0.0"}, restoring: true}
		deps := staleDeps(dir, lister, hookStore, projectStore)

		// The exit code is the diagnosis's business, not the prune's.
		outBuf, _, _ := runDoctorFixCmd(t, deps)

		after, err := os.ReadFile(hooksPath)
		if err != nil {
			t.Fatalf("read hooks.json: %v", err)
		}
		if !strings.Contains(string(after), "sessA1") {
			t.Errorf("doctor --fix pruned a hook during a restore:\n%s", after)
		}
		if strings.Contains(outBuf.String(), "Pruned stale hook:") {
			t.Errorf("doctor --fix reported a prune during a restore:\n%s", outBuf.String())
		}
	})

	t.Run("it leaves the daemon's tick behaviour unchanged", func(t *testing.T) {
		store, _ := newTempHooksStore(t, staleSeed)
		fc := &daemonFakeCommander{
			panesOut:     "live:0.0",
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
		if _, ok := postRun["gone01"]; !ok {
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
