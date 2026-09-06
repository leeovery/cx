package hooksweep

import (
	"log/slog"
	"path/filepath"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/logtest"
)

// The cycle's caller now reaches it across a package boundary, so what it
// answers with — and what it emits doing so — is this package's own contract.
func TestSweepCycle(t *testing.T) {
	t.Run("it runs one sweep cycle from the new package and reports what it removed", func(t *testing.T) {
		store, _ := hookstest.StageStore(t, hookstest.Staging{
			Seed: hooksBody(hookstest.LiveSeedA, hookstest.ReapableSeedB, hookstest.ReapableSeedC),
		})

		outcome, err := Run(&stubReader{rows: tokenRows(hookstest.LiveSeedA)}, store)
		if err != nil {
			t.Fatalf("Run: %v", err)
		}

		if outcome.DeclineReason != "" {
			t.Errorf("DeclineReason = %q, want none for a cycle that ran", outcome.DeclineReason)
		}
		removed := slices.Clone(outcome.Removed)
		slices.Sort(removed)
		want := []string{hookstest.ReapableSeedB, hookstest.ReapableSeedC}
		slices.Sort(want)
		if !slices.Equal(removed, want) {
			t.Errorf("Removed = %v, want %v", removed, want)
		}

		postRun, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[hookstest.LiveSeedA]; !ok {
			t.Errorf("the live key was reaped; file holds %v", keysOf(postRun))
		}
	})

	t.Run("it stands the cycle down under each reason from the new package", func(t *testing.T) {
		cases := []struct {
			name   string
			reader *stubReader
			store  func(*testing.T) *hooks.Store
			reason Reason
		}{
			{
				name:   "restoring",
				reader: &stubReader{rows: tokenRows(hookstest.LiveSeedA), restoring: true},
				reason: ReasonRestoring,
			},
			{
				name:   "marker read failed",
				reader: &stubReader{rows: tokenRows(hookstest.LiveSeedA), restoringErr: errTmuxDead},
				reason: ReasonMarkerReadFailed,
			},
			{
				name:   "pane read failed",
				reader: &stubReader{err: errTmuxDead},
				reason: ReasonPaneReadFailed,
			},
			{
				name:   "empty pane read",
				reader: &stubReader{rows: nil},
				reason: ReasonEmptyPaneRead,
			},
			{
				name:   "store read failed",
				reader: &stubReader{rows: tokenRows(hookstest.LiveSeedA)},
				store: func(t *testing.T) *hooks.Store {
					store, _ := hookstest.StageStore(t, hookstest.Staging{Unreadable: true})
					return store
				},
				reason: ReasonStoreReadFailed,
			},
			{
				name:   "lock timeout",
				reader: &stubReader{rows: tokenRows(hookstest.LiveSeedA)},
				store: func(t *testing.T) *hooks.Store {
					hooks.SetLockTimeoutForTest(t, lockBound)
					store, path := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
					hookstest.HoldHooksSidecar(t, path)
					return store
				},
				reason: ReasonLockTimeout,
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				store := tc.store
				if store == nil {
					store = func(t *testing.T) *hooks.Store {
						s, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
						return s
					}
				}
				staged := store(t)
				sink := logtest.Install(t)

				outcome, err := Run(tc.reader, staged)
				if err != nil {
					t.Fatalf("Run: want a stand-down, got %v", err)
				}
				if outcome.DeclineReason != tc.reason {
					t.Fatalf("DeclineReason = %q, want %q", outcome.DeclineReason, tc.reason)
				}
				if len(outcome.Removed) != 0 {
					t.Errorf("Removed = %v, want none on a stand-down", outcome.Removed)
				}

				level := slog.LevelWarn
				if tc.reason == ReasonRestoring || tc.reason == ReasonMarkerReadFailed {
					level = slog.LevelDebug
				}
				assertStandDown(t, sink, level, tc.reason)
			})
		}

		// Every declared reason must be reachable from the cases above, or a
		// stand-down path ships with nothing driving it.
		covered := map[Reason]bool{}
		for _, tc := range cases {
			covered[tc.reason] = true
		}
		for _, reason := range Reasons {
			if !covered[reason] {
				t.Errorf("no case stands the cycle down under %q", reason)
			}
		}
	})

	t.Run("it emits the whole cycle under the hooks component from the new package's own binding", func(t *testing.T) {
		sink := logtest.Install(t)

		reaping, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
		if _, err := Run(&stubReader{rows: tokenRows(hookstest.LiveSeedA)}, reaping); err != nil {
			t.Fatalf("Run on the reaping cycle: %v", err)
		}

		standingDown, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
		if _, err := Run(&stubReader{rows: tokenRows(hookstest.LiveSeedA), restoring: true}, standingDown); err != nil {
			t.Fatalf("Run on the stand-down cycle: %v", err)
		}

		failing, _ := hookstest.StageStore(t, hookstest.Staging{
			Dir:           filepath.Join(t.TempDir(), "lock-open-denied"),
			Seed:          hookstest.StaleHookSeed,
			SidecarAbsent: true,
			WritesDenied:  true,
		})
		if _, err := Run(&stubReader{rows: tokenRows(hookstest.LiveSeedA)}, failing); err == nil {
			t.Fatal("Run on the failing cycle: want an error, got nil")
		}

		for _, msg := range []string{countsMsg, removedMsg, standDownMsg, sweepFailedMsg} {
			if got := len(sink.Records().Matching("hooks", msg)); got == 0 {
				t.Errorf("records matching hooks/%q = 0, want at least one; records=%+v", msg, sink.Records())
			}
		}
		for _, rec := range sink.Records() {
			if got := rec.AttrOrEmpty("component"); got != "hooks" {
				t.Errorf("record emitted under component %q, want hooks: %+v", got, rec)
			}
		}
	})
}
