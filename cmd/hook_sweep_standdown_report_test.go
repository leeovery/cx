package cmd

import (
	"bytes"
	"errors"
	"log/slog"
	"os"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
)

// A repair that silently did not run is indistinguishable from a repair that
// found nothing to do, so every stand-down reports itself to its caller.
func TestHookSweepReportsStandDown(t *testing.T) {
	const staleSeed = `{
  "gone01": {"on-resume": "cmd-gone"},
  "live:0.0": {"on-resume": "cmd-live"}
}`

	t.Run("it reports the restore stand-down to the caller", func(t *testing.T) {
		store, path := newTempHooksStore(t, staleSeed)
		before := readFileBytes(t, path)
		lister := &recordingHookKeyLister{panes: []string{"live:0.0"}, restoring: true}

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
		lister := &stubAllPaneLister{panes: []string{}}

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
		restoringLister := &recordingHookKeyLister{panes: []string{"live:0.0"}, restoring: true}
		if err := runHookStaleCleanup(restoringLister, restoringStore, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup on the restore branch: %v", err)
		}

		emptyStore, _ := newTempHooksStore(t, staleSeed)
		emptyLister := &stubAllPaneLister{panes: []string{}}
		if err := runHookStaleCleanup(emptyLister, emptyStore, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup on the empty-pane-read branch: %v", err)
		}
	})

	t.Run("it reports nothing when the live set is empty and no entries are persisted", func(t *testing.T) {
		store, _ := newTempHooksStore(t, "")
		lister := &stubAllPaneLister{panes: []string{}}

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

func TestDoctorFixReportsSkippedHookPrune(t *testing.T) {
	t.Run("it prints the skipped-prune line for a restore window in doctor --fix", func(t *testing.T) {
		out := runDoctorFixWithLister(t, fakeHookLister{keys: []string{"sessB:0.0"}, restoring: true})
		assertSkippedPruneLine(t, out, "Skipped stale hook prune: restore may be in progress")
	})

	t.Run("it prints the skipped-prune line for an empty live read in doctor --fix", func(t *testing.T) {
		out := runDoctorFixWithLister(t, fakeHookLister{keys: []string{}})
		assertSkippedPruneLine(t, out, "Skipped stale hook prune: could not read live panes")
	})

	t.Run("it leaves the doctor --fix exit code to the post-repair diagnosis", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, _ := seedHooksJSON(t)
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, fakeHookLister{keys: []string{"sessB:0.0"}, restoring: true}, hookStore, projectStore)

		outBuf, _, err := runDoctorFixCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; want nil over a healthy post-repair diagnosis\n%s", err, outBuf.String())
		}
		if !strings.Contains(outBuf.String(), "Skipped stale hook prune:") {
			t.Fatalf("fixture did not stand the prune down:\n%s", outBuf.String())
		}

		// The same stand-down with a genuinely failing check still exits non-zero.
		failingDir := t.TempDir()
		seedHealthyStateDir(t, failingDir)
		failingHooks, _ := seedHooksJSON(t)
		failingProjects, _ := seedProjectsJSON(t, t.TempDir())
		failingDeps := staleDeps(failingDir, fakeHookLister{keys: []string{"sessB:0.0"}, restoring: true}, failingHooks, failingProjects)
		failingDeps.SaverPresent = func() (bool, error) { return false, nil }

		failBuf, _, failErr := runDoctorFixCmd(t, failingDeps)
		if !errors.Is(failErr, ErrDoctorUnhealthy) {
			t.Fatalf("Execute err = %v; want ErrDoctorUnhealthy with a failing check\n%s", failErr, failBuf.String())
		}
	})
}

// runDoctorFixWithLister drives `doctor --fix` over an otherwise healthy
// install whose only anomaly is the stand-down the lister provokes.
func runDoctorFixWithLister(t *testing.T, lister fakeHookLister) string {
	t.Helper()

	dir := t.TempDir()
	seedHealthyStateDir(t, dir)
	hookStore, hooksPath := seedHooksJSON(t, "sessA1")
	before, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("read hooks.json: %v", err)
	}
	projectStore, _ := seedProjectsJSON(t, t.TempDir())

	outBuf, _, _ := runDoctorFixCmd(t, staleDeps(dir, lister, hookStore, projectStore))

	after, err := os.ReadFile(hooksPath)
	if err != nil {
		t.Fatalf("re-read hooks.json: %v", err)
	}
	if !bytes.Equal(before, after) {
		t.Errorf("hooks.json rewritten on a stand-down\nbefore: %s\nafter:  %s", before, after)
	}
	return outBuf.String()
}

// assertSkippedPruneLine pins the exact line and its placement in the repair
// block: a stand-down and a removal cannot co-occur, so the line stands where
// the `Pruned stale hook:` lines would have.
func assertSkippedPruneLine(t *testing.T, out, want string) {
	t.Helper()

	if strings.Contains(out, "Pruned stale hook:") {
		t.Errorf("doctor --fix reported a prune on a stand-down:\n%s", out)
	}

	lines := strings.Split(out, "\n")
	skippedAt, projectAt := -1, -1
	for i, line := range lines {
		switch {
		case line == want:
			if skippedAt != -1 {
				t.Errorf("skipped-prune line printed twice:\n%s", out)
			}
			skippedAt = i
		case strings.HasPrefix(line, "Pruned stale project:"):
			projectAt = i
		}
	}
	if skippedAt == -1 {
		t.Fatalf("missing skipped-prune line %q:\n%s", want, out)
	}
	if projectAt != -1 && skippedAt > projectAt {
		t.Errorf("skipped-prune line follows the project prune; want it in the hook-prune block:\n%s", out)
	}
}

func TestSkippedPrunePhrase(t *testing.T) {
	cases := map[string]string{
		skipReasonRestoring:     "restore may be in progress",
		skipReasonEmptyPaneRead: "could not read live panes",
		// An unmapped reason must still print something: a stand-down that
		// renders as an empty line is the silence this reporting removes.
		"lock-timeout": "lock-timeout",
	}
	for reason, want := range cases {
		if got := skippedPrunePhrase(reason); got != want {
			t.Errorf("skippedPrunePhrase(%q) = %q, want %q", reason, got, want)
		}
	}
}
