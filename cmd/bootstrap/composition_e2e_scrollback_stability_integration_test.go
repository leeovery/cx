//go:build integration

package bootstrap_test

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

const stabilityPostBootstrapBufferTick = 1 * time.Second

const stabilityObservationCount = 10

const stabilityObservationInterval = 1 * time.Second

func TestCompositeBootstrap_ScrollbackDirPathSetStableAcross10Observations(t *testing.T) {
	h := setupCompositeHarness(t)

	sweeper := bootstrapadapter.NewOrphanSweeper(h.Client, nil)
	if err := sweeper.SweepOrphanDaemons(); err != nil {
		t.Fatalf("SweepOrphanDaemons returned non-nil error "+
			"(best-effort step must return nil): %v", err)
	}
	if err := tmux.BootstrapPortalSaver(h.Client, h.StateDir); err != nil {
		t.Fatalf("BootstrapPortalSaver (post-sweep idempotent re-run): %v", err)
	}

	time.Sleep(stabilityPostBootstrapBufferTick)

	scrollbackDir := state.ScrollbackDir(h.StateDir)

	baseline, dirExists := snapshotScrollbackPaths(t, scrollbackDir)
	if !dirExists {
		t.Fatalf("scrollback dir does not exist: %s", scrollbackDir)
	}
	if len(baseline) == 0 {
		t.Fatalf("scrollback baseline empty after first post-bootstrap " +
			"tick — capture pipeline may be broken or seed activity " +
			"insufficient")
	}

	for i := 1; i <= stabilityObservationCount; i++ {
		time.Sleep(stabilityObservationInterval)
		observation, _ := snapshotScrollbackPaths(t, scrollbackDir)
		assertPathSetEqual(t, baseline, observation, i, scrollbackDir)
	}
}

func snapshotScrollbackPaths(t *testing.T, dir string) (map[string]struct{}, bool) {
	t.Helper()
	if _, err := os.Stat(dir); err != nil {
		if os.IsNotExist(err) {
			return nil, false
		}
		t.Fatalf("snapshotScrollbackPaths stat(%q): %v", dir, err)
	}
	paths := make(map[string]struct{})
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if path == dir {
			return nil
		}
		if !d.Type().IsRegular() {
			return nil
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		paths[filepath.ToSlash(rel)] = struct{}{}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshotScrollbackPaths(%q): %v", dir, err)
	}
	return paths, true
}

func assertPathSetEqual(t *testing.T, baseline, observation map[string]struct{},
	observationIndex int, scrollbackDir string,
) {
	t.Helper()
	added := setDifference(observation, baseline)
	removed := setDifference(baseline, observation)
	if len(added) == 0 && len(removed) == 0 {
		return
	}
	t.Fatalf("scrollback path-set diverged at observation %d/%d "+
		"(baseline = %d paths, observation = %d paths)\n"+
		"  added paths (appeared in observation, absent from baseline): %v\n"+
		"  removed paths (absent from observation, present in baseline): %v\n"+
		"  baseline: %v\n"+
		"  observation: %v\n"+
		"  scrollback dir: %s\n"+
		"  hint: additions point at an unexpected writer; removals point at "+
		"a GC race (Components A+B+E composition regression)",
		observationIndex, stabilityObservationCount,
		len(baseline), len(observation),
		added, removed,
		slices.Sorted(maps.Keys(baseline)), slices.Sorted(maps.Keys(observation)),
		scrollbackDir)
}

func setDifference(a, b map[string]struct{}) []string {
	out := make([]string, 0)
	for k := range a {
		if _, ok := b[k]; !ok {
			out = append(out, k)
		}
	}
	sort.Strings(out)
	return out
}
