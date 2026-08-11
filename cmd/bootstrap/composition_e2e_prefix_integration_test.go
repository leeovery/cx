//go:build integration

package bootstrap_test

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
)

const prefixOscillationSampleCount = 4

const prefixOscillationSampleInterval = 900 * time.Millisecond

func TestCompositeHarness_PreFixDysfunctionReproduces(t *testing.T) {
	h := setupCompositeHarness(t)

	pids, err := portaltest.PgrepPortalDaemons()
	if err != nil {
		t.Fatalf("pgrep snapshot: %v", err)
	}
	if len(pids) != 3 {
		t.Fatalf("pgrep -fx returned %d daemons, want 3: %v\n"+
			"  h.LegitimateDaemonPID = %d (alive=%v)\n"+
			"  h.Orphan1PID = %d (alive=%v)\n"+
			"  h.Orphan2PID = %d (alive=%v)\n"+
			"  hint: a daemon exited between harness setup and the pre-fix observation",
			len(pids), pids,
			h.LegitimateDaemonPID, pidAlive(h.LegitimateDaemonPID),
			h.Orphan1PID, pidAlive(h.Orphan1PID),
			h.Orphan2PID, pidAlive(h.Orphan2PID))
	}

	if len(pids) == 1 {
		t.Fatalf("pgrep -fx returned 1 daemon — pre-fix harness must observe 3, not the converged-healthy count\n"+
			"  PIDs: %v", pids)
	}

	scrollbackDir := state.ScrollbackDir(h.StateDir)
	samples := sampleScrollbackBinCounts(t, scrollbackDir,
		prefixOscillationSampleCount, prefixOscillationSampleInterval)

	assertScrollbackOscillation(t, scrollbackDir, samples)
}

func sampleScrollbackBinCounts(t *testing.T, dir string, count int, interval time.Duration) []int {
	t.Helper()
	samples := make([]int, 0, count)
	for i := 0; i < count; i++ {
		if i > 0 {
			time.Sleep(interval)
		}
		samples = append(samples, countBinFiles(dir))
	}
	return samples
}

func countBinFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasSuffix(e.Name(), ".bin") {
			n++
		}
	}
	return n
}

func assertScrollbackOscillation(t *testing.T, dir string, samples []int) {
	t.Helper()

	totalActivity := 0
	for _, s := range samples {
		totalActivity += s
	}
	if totalActivity == 0 {
		listing, _ := os.ReadDir(dir)
		t.Fatalf("no scrollback activity observed across %d samples in %s\n"+
			"  samples: %v\n"+
			"  dir listing: %v\n"+
			"  hint: the daemons may not be running, or capture loop never wrote to scrollback/",
			len(samples), dir, samples, dirNames(listing))
	}

	distinct := make(map[int]struct{}, len(samples))
	for _, s := range samples {
		distinct[s] = struct{}{}
	}
	if len(distinct) < 2 {
		t.Fatalf("scrollback dir did not oscillate across %d samples\n"+
			"  samples: %v\n"+
			"  distinct counts: %d (want >= 2)\n"+
			"  dir: %s\n"+
			"  hint: three racing daemons should produce visible .bin churn; "+
			"a stable count suggests only one daemon is writing",
			len(samples), samples, len(distinct), dir)
	}
}

func dirNames(entries []os.DirEntry) []string {
	out := make([]string, 0, len(entries))
	for _, e := range entries {
		out = append(out, e.Name())
	}
	return out
}
