//go:build integration

package bootstrap_test

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxout"
)

func TestCompositeBootstrap_FObservables(t *testing.T) {
	h := setupCompositeHarness(t)

	sweeper := bootstrapadapter.NewOrphanSweeper(h.Client, nil)
	start := time.Now()
	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error "+
			"(best-effort step must return nil): %v", err)
	}
	if err := tmux.BootstrapPortalSaver(h.Client, h.StateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver (post-sweep idempotent re-run): %v", err)
	}

	if res := waitForPgrepCount(t, 1); !res.Reached {
		t.Fatalf("post-bootstrap: pgrep -fx did not converge to 1 (%s, %s since "+
			"bootstrap-slice entry)\n"+
			"  harness saver PID (setup-time): %d (alive=%v)\n"+
			"  harness orphan1 PID: %d (alive=%v)\n"+
			"  harness orphan2 PID: %d (alive=%v)",
			res, time.Since(start),
			h.LegitimateDaemonPID, pidAlive(h.LegitimateDaemonPID),
			h.Orphan1PID, pidAlive(h.Orphan1PID),
			h.Orphan2PID, pidAlive(h.Orphan2PID))
	}

	panePIDRaw, err := h.Sock.TryRun("list-panes", "-t", tmux.PortalSaverName, "-F", "#{pane_pid}")
	if err != nil {
		t.Fatalf("list-panes -t %s -F #{pane_pid}: %v\n%s",
			tmux.PortalSaverName, err, panePIDRaw)
	}
	panePIDStr := strings.TrimSpace(panePIDRaw)
	if panePIDStr == "" {
		t.Fatalf("list-panes returned empty pane_pid output for %s\n--- raw ---\n%q",
			tmux.PortalSaverName, panePIDRaw)
	}
	if strings.Contains(panePIDStr, "\n") {
		t.Fatalf("list-panes returned multiple pane_pid lines for %s "+
			"(want exactly 1):\n--- raw ---\n%q",
			tmux.PortalSaverName, panePIDRaw)
	}
	panePID, err := strconv.Atoi(panePIDStr)
	if err != nil {
		t.Fatalf("parse pane_pid %q as int: %v\n--- raw ---\n%q",
			panePIDStr, err, panePIDRaw)
	}
	if panePID <= 0 {
		t.Fatalf("pane_pid = %d; want > 0\n--- raw ---\n%q",
			panePID, panePIDRaw)
	}

	args, err := psArgsForPIDInline(panePID)
	if err != nil {
		t.Fatalf("ps -o args= -p %d: %v", panePID, err)
	}
	const wantDaemonArgs = "portal state daemon"
	if !strings.Contains(args, wantDaemonArgs) {
		t.Fatalf("pane process args do not contain %q\n"+
			"  pane_pid: %d\n"+
			"  ps args: %q\n"+
			"  hint: Component F's respawn-pane swap from placeholder to "+
			"`portal state daemon` did not fire, or the daemon process exited "+
			"and tmux respawned the placeholder",
			wantDaemonArgs, panePID, args)
	}
	const forbiddenPlaceholder = "tail -f /dev/null"
	if strings.Contains(args, forbiddenPlaceholder) {
		t.Fatalf("pane process args still contain placeholder %q\n"+
			"  pane_pid: %d\n"+
			"  ps args: %q\n"+
			"  hint: Component F's respawn-pane swap appears to have NOT replaced "+
			"the placeholder command — F's ordering regression",
			forbiddenPlaceholder, panePID, args)
	}

	const optKey = "destroy-unattached"
	optRaw, err := h.Sock.TryRun("show-options", "-t", tmux.PortalSaverName, optKey)
	if err != nil {
		t.Fatalf("show-options -t %s %s: %v\n--- raw ---\n%q",
			tmux.PortalSaverName, optKey, err, optRaw)
	}
	optLine := strings.TrimSpace(optRaw)
	if !strings.HasPrefix(optLine, optKey) {
		t.Fatalf("show-options output missing %q key prefix\n--- raw ---\n%q\n--- trimmed ---\n%q",
			optKey, optRaw, optLine)
	}
	value := strings.TrimSpace(strings.TrimPrefix(optLine, optKey))
	value = tmuxout.StripMatchedOuterQuotes(value)
	if value != "off" {
		t.Fatalf("destroy-unattached parsed value = %q; want %q\n--- raw ---\n%q",
			value, "off", optRaw)
	}
}

func psArgsForPIDInline(pid int) (string, error) {
	out, err := exec.Command("ps", "-o", "args=", "-p", strconv.Itoa(pid)).Output()
	if err != nil {
		return "", fmt.Errorf("ps -p %d: %w", pid, err)
	}
	return strings.TrimSpace(string(out)), nil
}
