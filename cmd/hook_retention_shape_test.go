package cmd

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

// A key the reaper cannot parse as a token is not evidence of a dead pane, so
// neither entry point may take it — however the sweep is reached.
func TestUnjudgeableHookKeyRetention(t *testing.T) {
	t.Run("it retains a non-token-shaped key across the daemon sweep", func(t *testing.T) {
		seed := `{
  "live:0.0": {"on-resume": "cmd-live"},
  "old-session:3.1": {"on-resume": "cmd-old"}
}`
		store, path := newTempHooksStore(t, seed)
		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read seeded hooks.json: %v", err)
		}

		lister := &stubAllPaneLister{panes: []string{"live:0.0"}}
		if err := runHookStaleCleanup(lister, store, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load()
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun["old-session:3.1"]; !ok {
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
		hookStore, hooksPath := seedHooksJSON(t, "old-session:3.1")
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, fakeHookLister{keys: []string{"live:0.0"}}, hookStore, projectStore)

		outBuf, _, err := runDoctorFixCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; want nil (nothing to repair)", err)
		}

		after, err := os.ReadFile(hooksPath)
		if err != nil {
			t.Fatalf("read hooks.json: %v", err)
		}
		if !strings.Contains(string(after), "old-session:3.1") {
			t.Errorf("doctor --fix pruned the non-token-shaped entry:\n%s", after)
		}
		if strings.Contains(outBuf.String(), "Pruned stale hook: old-session:3.1") {
			t.Errorf("doctor --fix reported pruning a retained entry:\n%s", outBuf.String())
		}
	})

	t.Run("it exits 0 from portal doctor with only retained non-token-shaped entries present", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, _ := seedHooksJSON(t, "old-session:3.1", "another-session:0.0")
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, fakeHookLister{keys: []string{"live:0.0"}}, hookStore, projectStore)

		outBuf, _, err := runDoctorCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; want nil (retained entries are not a failing check)", err)
		}
		if !strings.Contains(outBuf.String(), "✓ stale hooks: no stale hooks") {
			t.Errorf("stale-hooks check did not pass:\n%s", outBuf.String())
		}
	})
}
