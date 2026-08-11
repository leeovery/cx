//go:build integration

package cmd_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const reentrancyHookBudget = 1500 * time.Millisecond

const reentrancyPollInterval = 25 * time.Millisecond

// reentrancyConsecutiveReads guards the poll loop against the hook
// subprocess's atomic rename: one read can land on the pre-rename file,
// two spaced reads prove the rename settled.
const reentrancyConsecutiveReads = 2

func TestCommitNowFromSessionClosedHook_NoDeadlockUnderRealTmux(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	// PATH-prepend so the hook subprocess resolves `portal state commit-now` to
	// the freshly built binary; the tmux server inherits PATH from this process
	// and run-shell propagates it onward.
	_ = portalbintest.StagePortalBinary(t)

	_, stateDir := portaltest.IsolateStateForTest(t)
	// Set on the test process so the tmux server forked below inherits it and
	// passes it to every hook subprocess.
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	// LIFO position matters: after IsolateStateForTest, before tmuxtest.New, so
	// straggler hook subprocesses cannot race the stateDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-reentr-")
	client := sock.Client()

	// The anchor must exist before hooks are registered: set-hook -g needs a live
	// server, and it also keeps tmux alive after B is killed below, which would
	// otherwise race the hook subprocess. The leading underscore keeps it out of
	// sessions.json.
	sock.Run(t, "new-session", "-d", "-s", "_anchor", "sh", "-c", "sleep infinity")

	// Registered through the production path rather than a hand-authored hook
	// string, so a regression that reverts session-closed to the cheap notify
	// command surfaces here.
	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("RegisterPortalHooks: %v", err)
	}

	sock.Run(t, "new-session", "-d", "-s", "A", "sh", "-c", "sleep infinity")
	sock.Run(t, "new-session", "-d", "-s", "B", "sh", "-c", "sleep infinity")

	// kill-session dispatches session-closed before returning, but the hook's
	// run-shell fork-execs asynchronously, so the poll below is the real wait.
	killStart := time.Now()
	sock.Run(t, "kill-session", "-t", "B")

	ctx, cancel := context.WithTimeout(context.Background(), reentrancyHookBudget)
	defer cancel()

	if err := pollSessionsJSON(ctx, stateDir, []string{"A"}, []string{"B"}); err != nil {
		elapsed := time.Since(killStart)
		t.Fatalf(
			"commit-now hook did not produce sessions.json reflecting kill within %s "+
				"(elapsed=%s): %v\n"+
				"--- state directory contents ---\n%s\n"+
				"--- live tmux sessions ---\n%s\n"+
				"--- live tmux panes ---\n%s\n",
			reentrancyHookBudget, elapsed, err,
			dumpStateDir(stateDir),
			dumpTmuxSessions(sock),
			dumpTmuxPanes(sock),
		)
	}

	elapsed := time.Since(killStart)

	if elapsed >= reentrancyHookBudget {
		t.Errorf("hook subprocess elapsed %s exceeds budget %s "+
			"(test would have timed out — pollSessionsJSON returned nil but the elapsed "+
			"measurement disagrees, indicating a clock race or budget-edge condition)",
			elapsed, reentrancyHookBudget)
	}

	idx, skip, err := state.ReadIndex(stateDir)
	if err != nil || skip {
		t.Fatalf("post-poll ReadIndex: skip=%v err=%v", skip, err)
	}
	names := sessionNames(idx)
	if _, ok := names["A"]; !ok {
		t.Errorf("sessions.json missing surviving session %q; sessions=%v", "A", keysOf(names))
	}
	if _, ok := names["B"]; ok {
		t.Errorf("sessions.json still contains killed session %q; sessions=%v", "B", keysOf(names))
	}

	t.Logf("commit-now hook completed in %s (budget=%s, sessions=%v)",
		elapsed, reentrancyHookBudget, keysOf(names))
}

func sessionNames(idx state.Index) map[string]struct{} {
	out := make(map[string]struct{}, len(idx.Sessions))
	for _, s := range idx.Sessions {
		out[s.Name] = struct{}{}
	}
	return out
}

func dumpStateDir(stateDir string) string {
	var lines []string
	entries, err := os.ReadDir(stateDir)
	if err != nil {
		return fmt.Sprintf("(readdir %s: %v)", stateDir, err)
	}
	for _, e := range entries {
		info, ierr := e.Info()
		if ierr != nil {
			lines = append(lines, fmt.Sprintf("%s (stat err: %v)", e.Name(), ierr))
			continue
		}
		lines = append(lines, fmt.Sprintf("%s (size=%d, mode=%s)", e.Name(), info.Size(), info.Mode()))
		if e.IsDir() {
			sub, serr := os.ReadDir(filepath.Join(stateDir, e.Name()))
			if serr != nil {
				lines = append(lines, fmt.Sprintf("  (readdir %s: %v)", e.Name(), serr))
				continue
			}
			for _, se := range sub {
				si, sierr := se.Info()
				if sierr != nil {
					lines = append(lines, fmt.Sprintf("  %s (stat err: %v)", se.Name(), sierr))
					continue
				}
				lines = append(lines, fmt.Sprintf("  %s (size=%d)", se.Name(), si.Size()))
			}
		}
	}
	if data, rerr := os.ReadFile(state.SessionsJSON(stateDir)); rerr == nil {
		const cap = 2048
		blob := string(data)
		if len(blob) > cap {
			blob = blob[:cap] + "...(truncated)"
		}
		lines = append(lines, "--- sessions.json contents ---", blob)
	}
	return strings.Join(lines, "\n")
}

func dumpTmuxSessions(sock *tmuxtest.Socket) string {
	out, err := sock.TryRun("list-sessions", "-F", "#{session_name}")
	if err != nil {
		return fmt.Sprintf("(list-sessions error: %v)\n%s", err, out)
	}
	return out
}

func dumpTmuxPanes(sock *tmuxtest.Socket) string {
	out, err := sock.TryRun("list-panes", "-a", "-F",
		"#{session_name}:#{window_index}.#{pane_index} #{pane_current_command}")
	if err != nil {
		return fmt.Sprintf("(list-panes error: %v)\n%s", err, out)
	}
	return out
}
