package cmd

import (
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
)

func TestDoctorFixReportsLockedHookPrune(t *testing.T) {
	// The read side degrades where the write side stands down, so the un-pruned
	// entry is reported as the stale hook it is rather than as a lock problem.
	t.Run("it reports the un-pruned entry as stale in the same window", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, lockBound)
		store, path := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
		hookstest.HoldHooksSidecar(t, path)

		got := checkStaleHooks(&stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, store)
		if got.status != checkFail {
			t.Errorf("status = %v, want checkFail under a held lock", got.status)
		}
		if got.detail != "1 stale hook entry" {
			t.Errorf("detail = %q, want %q", got.detail, "1 stale hook entry")
		}
	})
}
