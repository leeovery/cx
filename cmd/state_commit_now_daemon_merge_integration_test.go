//go:build integration

package cmd_test

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const daemonTickBudget = 4 * time.Second

const daemonTickPollInterval = 50 * time.Millisecond

// TestCommitNowDaemonMergeStability asserts on the SET of session names rather
// than byte-equivalence: the daemon legitimately repopulates per-pane scrollback
// hashes and content references that commit-now carries over verbatim from prev.
func TestCommitNowDaemonMergeStability(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	// PATH-prepend so every portal subprocess resolves to the freshly built binary.
	binDir := portalbintest.StagePortalBinary(t)
	binary, err := exec.LookPath("portal")
	if err != nil {
		t.Skipf("portal not on PATH after build+prepend; skipping: %v", err)
	}

	fixture := newSymptomFixture(t, binary, binDir, "ptl-merge-stable-")

	// Pre-condition: commit-now must have already produced a sessions.json that
	// omits B, so the question below is whether the daemon's next tick respects it.
	fixture.sock.Run(t, "kill-session", "-t", "B")

	ctx, cancel := context.WithTimeout(context.Background(), symptomKillBudget)
	defer cancel()
	if perr := pollSessionsJSON(ctx, fixture.stateDir, []string{"A"}, []string{"B"}); perr != nil {
		t.Fatalf(
			"commit-now did not remove B from sessions.json within %s "+
				"(pre-condition for merge-stability assertion): %v\n%s",
			symptomKillBudget, perr, fixture.diagnostic(),
		)
	}

	// Forces the daemon's next tick.
	if err := state.TouchSaveRequested(fixture.stateDir); err != nil {
		t.Fatalf("touch save.requested to force daemon tick: %v\n%s", err, fixture.diagnostic())
	}

	tickCtx, tickCancel := context.WithTimeout(context.Background(), daemonTickBudget)
	defer tickCancel()
	if err := waitForSaveRequestedConsumed(tickCtx, fixture.stateDir); err != nil {
		t.Fatalf(
			"daemon did not consume save.requested within %s "+
				"(daemon likely not running or wedged): %v\n%s",
			daemonTickBudget, err, fixture.diagnostic(),
		)
	}

	t.Run("daemon's next tick after commit-now does not re-introduce the killed session by name", func(t *testing.T) {
		idx, skip, err := state.ReadIndex(fixture.stateDir)
		if err != nil || skip {
			t.Fatalf(
				"post-daemon-tick ReadIndex: skip=%v err=%v\n%s",
				skip, err, fixture.diagnostic(),
			)
		}
		present := sessionNames(idx)
		if _, reintroduced := present["B"]; reintroduced {
			t.Fatalf(
				"daemon-merge regression: killed session B re-introduced into sessions.json "+
					"after daemon's post-commit-now tick; "+
					"present session names = %v\n%s",
				keysOf(present), fixture.diagnostic(),
			)
		}
	})

	t.Run("daemon's next tick after commit-now retains all live sessions by name", func(t *testing.T) {
		idx, skip, err := state.ReadIndex(fixture.stateDir)
		if err != nil || skip {
			t.Fatalf(
				"post-daemon-tick ReadIndex: skip=%v err=%v\n%s",
				skip, err, fixture.diagnostic(),
			)
		}
		present := sessionNames(idx)
		if _, ok := present["A"]; !ok {
			t.Fatalf(
				"daemon-merge regression: live session A dropped from sessions.json "+
					"after daemon's post-commit-now tick; "+
					"present session names = %v\n%s",
				keysOf(present), fixture.diagnostic(),
			)
		}
	})
}

// waitForSaveRequestedConsumed waits for save.requested to disappear: a
// successful captureAndCommit cycle removes it as its post-commit step, which is
// the readiness signal that the daemon's tick completed.
func waitForSaveRequestedConsumed(ctx context.Context, stateDir string) error {
	ticker := time.NewTicker(daemonTickPollInterval)
	defer ticker.Stop()
	path := state.SaveRequested(stateDir)
	for {
		_, err := os.Stat(path)
		switch {
		case err == nil:
		case errors.Is(err, fs.ErrNotExist):
			return nil
		default:
			return fmt.Errorf("stat save.requested during poll: %w", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}
