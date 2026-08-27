package cmd

import (
	"strings"
	"testing"
)

// The reaper's log line names the removed command; its stdout does not, and a
// user reading a repair still sees exactly the key.
func TestDoctorFixPrunedHookOutput(t *testing.T) {
	t.Run("it leaves doctor --fix stdout unchanged", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, _ := seedHooksJSON(t, reapableSeedA)
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, fakeHookLister{keys: []string{"sessB:0.0"}}, hookStore, projectStore)

		outBuf, _, err := runDoctorFixCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; want nil", err)
		}

		var pruned []string
		for _, line := range strings.Split(outBuf.String(), "\n") {
			if strings.HasPrefix(line, "Pruned stale hook:") {
				pruned = append(pruned, line)
			}
		}
		want := "Pruned stale hook: " + reapableSeedA
		if len(pruned) != 1 || pruned[0] != want {
			t.Errorf("pruned-hook stdout lines = %q, want exactly [%q]\n%s",
				pruned, want, outBuf.String())
		}
	})
}
