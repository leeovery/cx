package cmd

import (
	"os"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hookstest"
)

// A key the reaper cannot parse as a token is not evidence of a dead pane, so
// neither command a user reaches for may take it: `--fix` must leave it where
// it is without claiming a prune, and a plain diagnosis must call an install
// holding nothing else healthy. The sweep's own half of the invariant is pinned
// in internal/hooksweep; what these two cases add is the face the user meets.
func TestUnjudgeableHookKeyRetention(t *testing.T) {
	t.Run("it retains a non-token-shaped key across doctor --fix", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, hooksPath := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(hookstest.UnjudgeableSeedA)})
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, hookStore, projectStore)

		outBuf, _, err := runDoctorWith(t, deps, "--fix")
		if err != nil {
			t.Fatalf("Execute err = %v; want nil (nothing to repair)", err)
		}

		after, err := os.ReadFile(hooksPath)
		if err != nil {
			t.Fatalf("read hooks.json: %v", err)
		}
		if !strings.Contains(string(after), hookstest.UnjudgeableSeedA) {
			t.Errorf("doctor --fix pruned the non-token-shaped entry:\n%s", after)
		}
		if strings.Contains(outBuf.String(), "Pruned stale hook: "+hookstest.UnjudgeableSeedA) {
			t.Errorf("doctor --fix reported pruning a retained entry:\n%s", outBuf.String())
		}
	})

	t.Run("it exits 0 from portal doctor with only retained non-token-shaped entries present", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(hookstest.UnjudgeableSeedA, hookstest.UnjudgeableSeedB)})
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, hookStore, projectStore)

		outBuf, _, err := runDoctorWith(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; want nil (retained entries are not a failing check)", err)
		}
		if !strings.Contains(outBuf.String(), "✓ stale hooks: no stale hooks") {
			t.Errorf("stale-hooks check did not pass:\n%s", outBuf.String())
		}
	})
}
