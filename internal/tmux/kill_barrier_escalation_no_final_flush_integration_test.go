//go:build integration

package tmux_test

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const scrollbackEmergenceTimeout = 3 * time.Second

const scrollbackEmergencePollTick = 50 * time.Millisecond

const orphanExitTimeout = 3 * time.Second

const orphanExitPollTick = 20 * time.Millisecond

const postExitSettleWindow = 200 * time.Millisecond

func TestKillBarrierEscalation_NoScrollbackDeltaIn200msPostExit(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	_ = portalbintest.StagePortalBinary(t)

	envSlice, stateDir := portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)

	sock := tmuxtest.New(t, "ptl-killesc-")
	client := sock.Client()

	// The daemon needs one non-internal session to capture; with none it ticks
	// forever without ever writing a .bin, and the snapshot is never taken.
	if err := client.NewDetachedSessionNoCwd(
		"work", "sh -c 'exec tail -f /dev/null'",
	); err != nil {
		t.Fatalf("create work session: %v", err)
	}

	// Spawned by hand rather than through portaltest.SpawnIsolatedDaemon: the
	// orphan must share the test's stateDir, which that helper forces to a
	// per-call tempdir.
	orphanEnv := append([]string{}, envSlice...)
	orphanEnv = append(orphanEnv, "PORTAL_STATE_DIR="+stateDir)
	// Pin the orphan to the TEST server — last-wins over the poisoned TMUX in
	// envSlice. Without it the orphan attaches to the developer's real server
	// and captures their live sessions.
	orphanEnv = append(orphanEnv, "TMUX="+sock.SocketPath()+",0,0")
	scrollbackDir := state.ScrollbackDir(stateDir)

	// The orphan self-ejects after 3 divergent-view ticks (~3s), which under
	// host load can beat its first capture; that case is respawned, while an
	// orphan still alive with no .bin is a genuine capture failure.
	const maxOrphanAttempts = 3
	var orphanPID int
	var reaped <-chan struct{}
	for attempt := 1; ; attempt++ {
		orphan := exec.Command("portal", "state", "daemon")
		orphan.Env = orphanEnv
		if err := orphan.Start(); err != nil {
			t.Fatalf("start orphan daemon: %v", err)
		}
		orphanPID = orphan.Process.Pid
		reaped = portaltest.RegisterSubprocessCleanup(t, orphan)

		// Force a capture on the next tick instead of waiting out maxGap (30s).
		if err := state.TouchSaveRequested(stateDir); err != nil {
			t.Fatalf("touch save.requested: %v", err)
		}

		if tmuxtest.PollUntil(t, scrollbackEmergenceTimeout, scrollbackEmergencePollTick, func() bool {
			return countBinFiles(scrollbackDir) >= 1
		}) {
			break
		}

		select {
		case <-reaped:
			if attempt < maxOrphanAttempts {
				t.Logf("attempt %d/%d: orphan %d self-ejected before first capture (host load); respawning",
					attempt, maxOrphanAttempts, orphanPID)
				continue
			}
			t.Fatalf("snapshot never taken — orphan self-ejected before first capture on all %d attempts "+
				"(host under sustained load?)\n  scrollback dir: %s\n  contents: %v",
				maxOrphanAttempts, scrollbackDir, listDirSafe(scrollbackDir))
		default:
			t.Fatalf("snapshot never taken — orphan %d is ALIVE but wrote no scrollback within %s "+
				"(genuine capture failure, not load)\n  scrollback dir: %s\n  contents: %v",
				orphanPID, scrollbackEmergenceTimeout, scrollbackDir, listDirSafe(scrollbackDir))
		}
	}

	pre, err := portaltest.SnapshotStateDir(scrollbackDir)
	if err != nil {
		t.Fatalf("pre-SIGKILL snapshot: %v", err)
	}
	if !hasAnyBin(pre) {
		t.Fatalf("snapshot never taken — pre-SIGKILL snapshot contained no .bin entries\n"+
			"  scrollback dir: %s\n"+
			"  pre keys: %v",
			scrollbackDir, slices.Sorted(maps.Keys(pre)))
	}

	killErr := syscall.Kill(orphanPID, syscall.SIGKILL)
	// ESRCH is tolerated: a self-eject bypasses the shutdown flush just as
	// SIGKILL does, so the invariant still holds.
	if killErr != nil && !errors.Is(killErr, syscall.ESRCH) {
		t.Logf("SIGKILL syscall returned %v (proceeding; orphan may have already exited)", killErr)
	}

	// Reap before polling kill(pid, 0): an unreaped zombie answers 0 forever
	// and never ESRCH, so process death could not be timed.
	select {
	case <-reaped:
	case <-time.After(orphanExitTimeout):
		t.Fatalf("orphan PID %d was not reaped within %s; "+
			"the no-final-flush window cannot be timed from process death",
			orphanPID, orphanExitTimeout)
	}
	exited := tmuxtest.PollUntil(t, orphanExitTimeout, orphanExitPollTick, func() bool {
		err := syscall.Kill(orphanPID, 0)
		return errors.Is(err, syscall.ESRCH)
	})
	if !exited {
		t.Fatalf("orphan PID %d did not reach ESRCH within %s after reap; "+
			"the no-final-flush window cannot be timed from process death",
			orphanPID, orphanExitTimeout)
	}

	time.Sleep(postExitSettleWindow)

	post, err := portaltest.SnapshotStateDir(scrollbackDir)
	if err != nil {
		t.Fatalf("post-exit snapshot: %v", err)
	}

	if deltas := portaltest.DiffFingerprints(pre, post); len(deltas) > 0 {
		lines := make([]string, len(deltas))
		for i, d := range deltas {
			lines[i] = "  " + portaltest.FormatDelta(d)
		}
		t.Fatalf("scrollback dir mutated between pre-SIGKILL snapshot and "+
			"200 ms post-exit snapshot (spec § Component A: no final-flush "+
			"GC cycle on escalation-killed orphans)\n"+
			"  scrollback dir: %s\n"+
			"  pre keys (%d): %v\n"+
			"  post keys (%d): %v\n"+
			"  delta(s):\n%s",
			scrollbackDir, len(pre), slices.Sorted(maps.Keys(pre)), len(post), slices.Sorted(maps.Keys(post)),
			strings.Join(lines, "\n"))
	}
}

func countBinFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.Type().IsRegular() && filepath.Ext(e.Name()) == ".bin" {
			n++
		}
	}
	return n
}

func listDirSafe(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return []string{fmt.Sprintf("(ReadDir failed: %v)", err)}
	}
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	sort.Strings(out)
	return out
}

func hasAnyBin(snap map[string]portaltest.Fingerprint) bool {
	for k := range snap {
		if filepath.Ext(k) == ".bin" {
			return true
		}
	}
	return false
}
