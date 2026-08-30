//go:build integration

// Measurement harness for the daemon's self-supervision hysteresis: against
// real tmux and a real daemon subprocess, it measures the worst-case run of
// consecutive saver-membership probe failures a healthy daemon can observe,
// and re-derives N as clamp(ceil(max × 2), 3, 9). The probe is implemented
// inline here rather than reusing the production seam, so the harness measures
// the condition independently of the code it validates.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"os/exec"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// Mirrors the production TickerPeriod, so a measured count reads directly as
// "how many production ticks the probe would have failed for".
const hysteresisTickerPeriod = 1 * time.Second

const hysteresisRunsPerScenario = 5

const hysteresisSteadyStateDuration = 30 * time.Second

const hysteresisAttachDetachDuration = 15 * time.Second

const hysteresisClientAttachedDuration = 15 * time.Second

const hysteresisBootstrapRecreateDuration = 10 * time.Second

const daemonStartupBudget = 5 * time.Second

const daemonStartupPollInterval = 50 * time.Millisecond

type scenarioResult struct {
	name   string
	runs   []int
	minObs int
	maxObs int
	median int
}

func (s *scenarioResult) summarise() {
	if len(s.runs) == 0 {
		return
	}
	sorted := append([]int(nil), s.runs...)
	sort.Ints(sorted)
	s.minObs = sorted[0]
	s.maxObs = sorted[len(sorted)-1]
	s.median = sorted[len(sorted)/2]
}

func TestSelfSupervisionHysteresisMeasurement(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)
	_ = portalbintest.StagePortalBinary(t)
	binary, err := exec.LookPath("portal")
	if err != nil {
		t.Skipf("portal not on PATH after build+prepend; skipping: %v", err)
	}

	scenarios := []struct {
		name string
		fn   func(t *testing.T, binary string) int
	}{
		{"steady-state", measureSteadyState},
		{"attach-detach", measureAttachDetach},
		{"client-attached-hook", measureClientAttached},
		{"bootstrap-kill-and-recreate", measureBootstrapRecreate},
	}

	results := make([]*scenarioResult, 0, len(scenarios))
	for _, sc := range scenarios {
		r := &scenarioResult{name: sc.name}
		for i := 0; i < hysteresisRunsPerScenario; i++ {
			worst := sc.fn(t, binary)
			r.runs = append(r.runs, worst)
			t.Logf("scenario=%s run=%d worst-consecutive-failures=%d", sc.name, i+1, worst)
		}
		r.summarise()
		results = append(results, r)
		t.Logf("scenario=%s min=%d max=%d median=%d", r.name, r.minObs, r.maxObs, r.median)
	}

	maxObserved := 0
	for _, r := range results {
		if r.maxObs > maxObserved {
			maxObserved = r.maxObs
		}
	}
	doubled := int(math.Ceil(float64(maxObserved) * 2))
	chosen := doubled
	if chosen < 3 {
		chosen = 3
	}
	if chosen > 9 {
		chosen = 9
	}
	upstreamDefect := doubled > 5

	t.Logf("aggregate: max-observed=%d, 2x=%d, chosen-N=%d, upstream-defect-flag=%v",
		maxObserved, doubled, chosen, upstreamDefect)

	// The locked-in constant must stay at least 2x the worst observed
	// transient, so anything that lengthens it (slower tmux, a slower hook, a
	// new bootstrap step) fails here rather than as a production false eject.
	if selfSupervisionHysteresisTicks < doubled {
		t.Errorf("safety-factor invariant violated: "+
			"selfSupervisionHysteresisTicks=%d but max-observed×2=%d "+
			"(max-observed=%d across %d scenarios)",
			selfSupervisionHysteresisTicks, doubled, maxObserved, len(scenarios))
	}
	if selfSupervisionHysteresisTicks < 3 || selfSupervisionHysteresisTicks > 9 {
		t.Errorf("clamp invariant violated: selfSupervisionHysteresisTicks=%d, "+
			"required 3 ≤ N ≤ 9", selfSupervisionHysteresisTicks)
	}
}

func measureSteadyState(t *testing.T, binary string) int {
	t.Helper()
	h := newHarness(t, binary)
	defer h.shutdown()
	return h.sampleWorstCaseConsecutive(t, hysteresisSteadyStateDuration, nil)
}

// `tmux attach` exits immediately without an interactive PTY, so
// refresh-client stands in as the closest thing that touches client state.
// Neither changes the saver pane pid, so any non-zero count here is genuine
// tmux-command flakiness rather than a real membership loss.
func measureAttachDetach(t *testing.T, binary string) int {
	t.Helper()
	h := newHarness(t, binary)
	defer h.shutdown()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_, _ = h.sock.TryRun("new-session", "-d", "-s", "probe-ad",
			"sh", "-c", "sleep infinity")
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_, _ = h.sock.TryRun("refresh-client")
			time.Sleep(200 * time.Millisecond)
		}
	}()
	return h.sampleWorstCaseConsecutive(t, hysteresisAttachDetachDuration, nil)
}

// Client attach does not change the pane pid, so any non-zero count reflects
// transient tmux flakiness while the hook fires. Without a PTY the run-shell
// payload stands in for a real `tmux attach -d`.
func measureClientAttached(t *testing.T, binary string) int {
	t.Helper()
	h := newHarness(t, binary)
	defer h.shutdown()
	if err := tmux.RegisterPortalHooks(h.client, nil); err != nil {
		t.Fatalf("RegisterPortalHooks: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		_, _ = h.sock.TryRun("new-session", "-d", "-s", "probe-ca",
			"sh", "-c", "sleep infinity")
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}
			_, _ = h.sock.TryRun("run-shell", "-b", "true")
			time.Sleep(500 * time.Millisecond)
		}
	}()
	return h.sampleWorstCaseConsecutive(t, hysteresisClientAttachedDuration, nil)
}

// The scenario most likely to produce a non-zero transient: the recreate
// carries a ~2s readiness barrier plus tmux respawn settle time, and the
// chosen N must absorb it.
func measureBootstrapRecreate(t *testing.T, binary string) int {
	t.Helper()
	h := newHarness(t, binary)
	defer h.shutdown()
	disturb := func() {
		// The session is killed first so BootstrapPortalSaver sees no session
		// and takes the create branch; there is no exported entry point for
		// the kill-and-recreate path itself.
		_, _ = h.sock.TryRun("kill-session", "-t", tmux.PortalSaverName)
		if err := tmux.BootstrapPortalSaver(h.client, h.stateDir); err != nil {
			t.Logf("BootstrapPortalSaver re-invoke: %v", err)
		}
	}
	return h.sampleWorstCaseConsecutive(t, hysteresisBootstrapRecreateDuration, disturb)
}

type harness struct {
	t        *testing.T
	sock     *tmuxtest.Socket
	client   *tmux.Client
	stateDir string
	binary   string
}

func newHarness(t *testing.T, binary string) *harness {
	t.Helper()
	env, stateDir := portaltest.IsolateStateForTest(t)
	// The isolated env must reach the test process before the tmux server
	// forks: tmux inherits it at server start and the saver pane inherits it
	// from tmux. Without the mirror the daemon subprocess resolves its state
	// dir against the developer's real XDG_CONFIG_HOME.
	for _, e := range env {
		idx := strings.IndexByte(e, '=')
		if idx < 0 {
			continue
		}
		k, v := e[:idx], e[idx+1:]
		if k == "XDG_CONFIG_HOME" {
			t.Setenv(k, v)
		}
	}
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	t.Setenv("PATH", stagedBinDir(binary)+string(os.PathListSeparator)+os.Getenv("PATH"))

	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)

	sock := tmuxtest.New(t, "ptl-hyst-")
	// tmux needs a live session before set-option / set-hook can run. The
	// leading underscore keeps _anchor out of CaptureStructure, so it does not
	// perturb daemon state.
	if _, err := sock.TryRun("new-session", "-d", "-s", "_anchor",
		"sh", "-c", "sleep infinity"); err != nil {
		t.Fatalf("new-session _anchor: %v", err)
	}

	client := sock.Client()
	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver: %v", err)
	}
	h := &harness{
		t:        t,
		sock:     sock,
		client:   client,
		stateDir: stateDir,
		binary:   binary,
	}
	h.waitForLegitimateState(t)
	return h
}

func stagedBinDir(binary string) string {
	idx := strings.LastIndexByte(binary, '/')
	if idx < 0 {
		return "."
	}
	return binary[:idx]
}

func (h *harness) waitForLegitimateState(t *testing.T) {
	t.Helper()
	deadline := time.Now().Add(daemonStartupBudget + 3*time.Second)
	for time.Now().Before(deadline) {
		probe, _ := h.probeSaverMembership()
		if probe {
			return
		}
		time.Sleep(daemonStartupPollInterval)
	}
	t.Fatalf("daemon did not reach legitimate saver-membership within %s",
		daemonStartupBudget+3*time.Second)
}

func (h *harness) shutdown() {
	pid, err := state.ReadPIDFile(h.stateDir)
	if err == nil && pid > 0 {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	_, _ = h.sock.TryRun("kill-server")
}

// tmux failures and a missing session report false with a nil error, so any
// tmux uncertainty counts toward the consecutive-failure tally.
func (h *harness) probeSaverMembership() (bool, error) {
	pid, err := state.ReadPIDFile(h.stateDir)
	if err != nil {
		if errors.Is(err, state.ErrPIDFileAbsent) {
			return false, nil
		}
		return false, fmt.Errorf("read daemon.pid: %w", err)
	}
	if _, herr := h.sock.TryRun("has-session", "-t", tmux.PortalSaverName); herr != nil {
		return false, nil
	}
	out, err := h.sock.TryRun("list-panes", "-t", tmux.PortalSaverName,
		"-F", "#{pane_pid}")
	if err != nil {
		return false, nil
	}
	paneLine := strings.TrimSpace(out)
	if paneLine == "" {
		return false, nil
	}
	first := paneLine
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = first[:i]
	}
	panePID, perr := strconv.Atoi(strings.TrimSpace(first))
	if perr != nil {
		return false, nil
	}
	return panePID == pid, nil
}

// Returns the longest consecutive run of false probes over duration. A non-nil
// disturb fires exactly once, ~2s in, so its effect lands inside the window.
func (h *harness) sampleWorstCaseConsecutive(t *testing.T, duration time.Duration, disturb func()) int {
	t.Helper()
	ticker := time.NewTicker(hysteresisTickerPeriod)
	defer ticker.Stop()
	deadline := time.Now().Add(duration)
	disturbAt := time.Now().Add(2 * time.Second)
	disturbed := disturb == nil

	worst := 0
	current := 0
	for time.Now().Before(deadline) {
		<-ticker.C
		if !disturbed && time.Now().After(disturbAt) {
			disturb()
			disturbed = true
		}
		ok, err := h.probeSaverMembership()
		if err != nil {
			ok = false
			t.Logf("probe error (counted as failure): %v", err)
		}
		if ok {
			current = 0
		} else {
			current++
			if current > worst {
				worst = current
			}
		}
	}
	return worst
}
