package cmd

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
)

// bogusHooksStore points a store at a directory, so every read of it fails:
// malformed JSON would decode to an empty map instead of erroring.
func bogusHooksStore(t *testing.T) *hooks.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir bogus hooks path: %v", err)
	}
	return hooks.NewStore(path)
}

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
				store, _ := newTempHooksStore(t, staleHookSeed)
				return &stubAllPaneLister{rows: tokenRows(liveSeedA), restoring: true}, store
			},
			want: skipReasonRestoring,
		},
		{
			name: "hooks.json unreadable",
			setup: func(t *testing.T) (staleSweepReader, *hooks.Store) {
				return &stubAllPaneLister{rows: tokenRows(liveSeedA)}, bogusHooksStore(t)
			},
			want: skipReasonStoreReadFailed,
		},
		{
			name: "pane enumeration failed",
			setup: func(t *testing.T) (staleSweepReader, *hooks.Store) {
				store, _ := newTempHooksStore(t, staleHookSeed)
				return &stubAllPaneLister{err: errors.New("tmux dead")}, store
			},
			want: skipReasonPaneReadFailed,
		},
		{
			name: "pane enumeration came back empty",
			setup: func(t *testing.T) (staleSweepReader, *hooks.Store) {
				store, _ := newTempHooksStore(t, staleHookSeed)
				return &stubAllPaneLister{rows: nil}, store
			},
			want: skipReasonEmptyPaneRead,
		},
		{
			name: "hooks.json locked",
			setup: func(t *testing.T) (staleSweepReader, *hooks.Store) {
				store, _, _ := lockedSweepFixture(t, lockBound)
				return &stubAllPaneLister{rows: tokenRows(liveSeedA)}, store
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
		deps, _, _, _, _ := seedStalePruneFixture(t, t.TempDir(), fakeHookLister{err: errors.New("tmux transient")})

		outBuf, _, _ := runDoctorFixCmd(t, deps)

		assertSkippedPruneLine(t, outBuf.String(), "Skipped stale hook prune: could not read live panes")
	})

	t.Run("it prints the skipped-prune line when hooks.json cannot be read", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, staleHookLister(), bogusHooksStore(t), projectStore)

		outBuf, _, _ := runDoctorFixCmd(t, deps)

		assertSkippedPruneLine(t, outBuf.String(), "Skipped stale hook prune: could not read hooks.json")
	})
}

// The diagnostic must answer the reaper's question with the reaper's own
// answer: a verdict the two disagree on is a user told a hook is lost by one
// command and reaped by the other.
func TestStaleHookVerdictParity(t *testing.T) {
	cases := []struct {
		name      string
		lister    fakeHookLister
		persisted []string
		judgeable bool
	}{
		{"empty rows with entries present", fakeHookLister{rows: tokenRows()}, []string{reapableSeedA}, false},
		{"enumeration error", fakeHookLister{err: errors.New("tmux transient")}, []string{reapableSeedA}, false},
		{"restore marker set", restoringHookLister(), []string{reapableSeedA}, false},
		{"live rows with unstamped panes", fakeHookLister{rows: unstampedRows(2)}, []string{unjudgeableSeedA}, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			checkStore, _ := seedHooksJSON(t, tc.persisted...)
			sweepStore, _ := seedHooksJSON(t, tc.persisted...)

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

	outBuf, _, _ := runDoctorFixCmd(t, deps)

	const want = "Skipped stale hook prune: restore in progress"
	assertSkippedPruneLine(t, outBuf.String(), want)
	for line := range strings.SplitSeq(outBuf.String(), "\n") {
		if strings.HasPrefix(line, "Skipped stale hook prune:") && line != want {
			t.Errorf("skipped-prune line = %q, want %q", line, want)
		}
	}
}
