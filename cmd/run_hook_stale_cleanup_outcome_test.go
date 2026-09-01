package cmd

import (
	"errors"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
)

// A cycle that declined and a cycle that found nothing to do are the same
// silence to a caller, so every way of declining has to name itself.
func TestHookSweepOutcomeNamesEveryDecline(t *testing.T) {
	cases := []struct {
		name  string
		setup func(t *testing.T) (staleSweepReader, *hooks.Store)
		want  string
	}{
		{
			name: "restore marker set",
			setup: func(t *testing.T) (staleSweepReader, *hooks.Store) {
				store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
				return &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA), restoring: true}, store
			},
			want: skipReasonRestoring,
		},
		{
			name: "hooks.json unreadable",
			setup: func(t *testing.T) (staleSweepReader, *hooks.Store) {
				store, _ := hookstest.StageStore(t, hookstest.Staging{Unreadable: true})
				return &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, store
			},
			want: skipReasonStoreReadFailed,
		},
		{
			name: "pane enumeration failed",
			setup: func(t *testing.T) (staleSweepReader, *hooks.Store) {
				store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
				return &stubStaleSweepReader{err: errors.New("tmux dead")}, store
			},
			want: skipReasonPaneReadFailed,
		},
		{
			name: "pane enumeration came back empty",
			setup: func(t *testing.T) (staleSweepReader, *hooks.Store) {
				store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
				return &stubStaleSweepReader{rows: nil}, store
			},
			want: skipReasonEmptyPaneRead,
		},
		{
			name: "hooks.json locked",
			setup: func(t *testing.T) (staleSweepReader, *hooks.Store) {
				store, _, _ := lockedSweepFixture(t, lockBound)
				return &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, store
			},
			want: skipReasonLockTimeout,
		},
	}

	seen := map[string]string{}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reader, store := tc.setup(t)

			outcome, _ := runHookStaleCleanup(reader, store, nil)

			if outcome.DeclineReason != tc.want {
				t.Fatalf("DeclineReason = %q, want %q", outcome.DeclineReason, tc.want)
			}
			if len(outcome.Removed) != 0 {
				t.Errorf("Removed = %v, want none on a declined cycle", outcome.Removed)
			}
			if phrase := skippedPrunePhrase(outcome.DeclineReason); phrase == outcome.DeclineReason {
				t.Errorf("reason %q renders as its own raw value; want a user-facing phrase", outcome.DeclineReason)
			}
		})

		if prior, ok := seen[tc.want]; ok {
			t.Errorf("case %q reuses the reason %q already used by %q; want one reason per decline path", tc.name, tc.want, prior)
		}
		seen[tc.want] = tc.name
	}
	if len(seen) != len(cases) {
		t.Errorf("distinct reasons = %d, want %d", len(seen), len(cases))
	}
}

// A repair that printed nothing is indistinguishable from a repair that found
// nothing, and both read failures used to print exactly that.
func TestDoctorFixReportsStandDownOnReadFailures(t *testing.T) {
	t.Run("it prints the skipped-prune line when the pane enumeration fails", func(t *testing.T) {
		deps, _, _, _, _ := seedStalePruneFixture(t, t.TempDir(), &stubStaleSweepReader{err: errors.New("tmux transient")})

		outBuf, _, _ := runDoctorWith(t, deps, "--fix")

		assertSkippedPruneLine(t, outBuf.String(), "Skipped stale hook prune: could not read live panes")
	})

	t.Run("it prints the skipped-prune line when hooks.json cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		unreadableHooks, _ := hookstest.StageStore(t, hookstest.Staging{Unreadable: true})
		deps := staleDeps(dir, staleHookLister(), unreadableHooks, projectStore)

		outBuf, _, _ := runDoctorWith(t, deps, "--fix")

		assertSkippedPruneLine(t, outBuf.String(), "Skipped stale hook prune: could not read hooks.json")
	})
}

// The diagnostic must answer the reaper's question with the reaper's own
// answer: a verdict the two disagree on is a user told a hook is lost by one
// command and reaped by the other.
func TestStaleHookVerdictParity(t *testing.T) {
	cases := []struct {
		name      string
		lister    *stubStaleSweepReader
		persisted []string
		judgeable bool
	}{
		{"empty rows with entries present", &stubStaleSweepReader{rows: tokenRows()}, []string{hookstest.ReapableSeedA}, false},
		{"enumeration error", &stubStaleSweepReader{err: errors.New("tmux transient")}, []string{hookstest.ReapableSeedA}, false},
		{"restore marker set", restoringHookLister(), []string{hookstest.ReapableSeedA}, false},
		{"live rows with unstamped panes", &stubStaleSweepReader{rows: unstampedRows(2)}, []string{hookstest.UnjudgeableSeedA}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkStore, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(tc.persisted...)})
			sweepStore, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(tc.persisted...)})

			got := checkStaleHooks(tc.lister, checkStore)
			checkJudgeable := got.status != checkNotEvaluable

			outcome, _ := runHookStaleCleanup(tc.lister, sweepStore, nil)
			sweepJudgeable := outcome.DeclineReason == ""

			if checkJudgeable != tc.judgeable {
				t.Errorf("checkStaleHooks judgeable = %v (status %v, detail %q), want %v", checkJudgeable, got.status, got.detail, tc.judgeable)
			}
			if sweepJudgeable != tc.judgeable {
				t.Errorf("sweep judgeable = %v (reason %q), want %v", sweepJudgeable, outcome.DeclineReason, tc.judgeable)
			}
			if checkJudgeable != sweepJudgeable {
				t.Errorf("the diagnostic and the sweep disagree: check judgeable = %v, sweep judgeable = %v", checkJudgeable, sweepJudgeable)
			}
		})
	}
}

// The copy is fixed: a user reading a stood-down repair is told the state, not
// a hedge about it.
func TestDoctorFixRestoreStandDownCopy(t *testing.T) {
	deps, _, _, _, _ := seedStalePruneFixture(t, t.TempDir(), restoringHookLister())

	outBuf, _, _ := runDoctorWith(t, deps, "--fix")

	const want = "Skipped stale hook prune: restore in progress"
	assertSkippedPruneLine(t, outBuf.String(), want)
	for line := range strings.SplitSeq(outBuf.String(), "\n") {
		if strings.HasPrefix(line, "Skipped stale hook prune:") && line != want {
			t.Errorf("skipped-prune line = %q, want %q", line, want)
		}
	}
}
