package portaltest

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
)

// Generous against observed writer lifetimes: a healthy teardown exits after one
// or two polls.
const teardownGuardBudget = 3 * time.Second

const teardownGuardPollTick = 50 * time.Millisecond

// RegisterStateDirTeardownGuard registers a bounded cleanup wait for stateDir's
// writers to finish, so a daemon's shutdown flush or a straggler hook subprocess
// cannot land a file mid-RemoveAll and fail the test with "directory not empty".
//
// Call order is load-bearing: register after the state dir exists and before
// tmuxtest.New, so LIFO cleanup runs this after kill-server and before the
// RemoveAll it protects.
func RegisterStateDirTeardownGuard(t *testing.T, stateDir string) {
	t.Helper()
	t.Cleanup(func() {
		deadline := time.Now().Add(teardownGuardBudget)

		// The saver daemon's shutdown flush is the biggest writer.
		if pid, err := state.ReadPIDFile(stateDir); err == nil && pid > 0 {
			for state.IsProcessAlive(pid) && time.Now().Before(deadline) {
				time.Sleep(teardownGuardPollTick)
			}
		}

		// Two identical snapshots mean nothing landed for a full tick. On budget
		// exhaustion just return and let RemoveAll take its chances.
		prev := dirShapeSnapshot(stateDir)
		for time.Now().Before(deadline) {
			time.Sleep(teardownGuardPollTick)
			cur := dirShapeSnapshot(stateDir)
			if cur == prev {
				return
			}
			prev = cur
		}
	})
}

// dirShapeSnapshot folds errors into the returned string, so two identical error
// states also count as quiescent.
func dirShapeSnapshot(dir string) string {
	var b strings.Builder
	var walk func(string)
	walk = func(d string) {
		entries, err := os.ReadDir(d)
		if err != nil {
			fmt.Fprintf(&b, "%s!%v\n", d, err)
			return
		}
		for _, e := range entries {
			info, err := e.Info()
			if err != nil {
				fmt.Fprintf(&b, "%s/%s!%v\n", d, e.Name(), err)
				continue
			}
			fmt.Fprintf(&b, "%s/%s|%d|%d\n", d, e.Name(), info.Size(), info.ModTime().UnixNano())
			if e.IsDir() {
				walk(d + "/" + e.Name())
			}
		}
	}
	walk(dir)
	return b.String()
}
