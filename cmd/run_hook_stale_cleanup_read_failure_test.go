package cmd

import (
	"os"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/logtest"
)

// preReadFailureCycle runs a cycle over a hooks.json unreadable from the outset,
// so the snapshot read is the one that fails.
func preReadFailureCycle(t *testing.T) sweepOutcome {
	t.Helper()
	store, _ := hookstest.StageStore(t, hookstest.Staging{Unreadable: true})
	return readFailureCycle(t, &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, store)
}

// deletePhaseReadFailureCycle runs a cycle over a hooks.json that goes
// unreadable while the pane enumeration is out: readable when the snapshot was
// taken, unreadable by the time the deletion reads it under its own hold.
func deletePhaseReadFailureCycle(t *testing.T) sweepOutcome {
	t.Helper()
	store, path := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
	reader := &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA), during: func() {
		if err := os.Remove(path); err != nil {
			t.Errorf("remove the staged hooks.json: %v", err)
			return
		}
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Errorf("stage a directory at the hooks.json path: %v", err)
		}
	}}
	return readFailureCycle(t, reader, store)
}

// readFailureCycle runs one cycle that must stand down rather than fail: a read
// it could not take is a decline, so the error belongs to the emitted line and
// not to the caller.
func readFailureCycle(t *testing.T, reader staleSweepReader, store *hooks.Store) sweepOutcome {
	t.Helper()
	logtest.Install(t)

	outcome, err := runHookStaleCleanup(reader, store)
	if err != nil {
		t.Fatalf("runHookStaleCleanup on an unreadable hooks.json: want a stand-down, got %v", err)
	}
	return outcome
}

// A cycle reads hooks.json twice, and the file is as capable of being
// unreadable at the second read as at the first. Both land on one reason, so
// the words a user meets do not depend on which read happened to be the one
// that failed; only a cycle that genuinely could not write leaves the
// vocabulary.
func TestHookSweepClassifiesEitherReadFailure(t *testing.T) {
	t.Run("it classifies a failed pre-read as store-read-failed", func(t *testing.T) {
		if reason := preReadFailureCycle(t).DeclineReason; reason != skipReasonStoreReadFailed {
			t.Errorf("DeclineReason = %q, want %q", reason, skipReasonStoreReadFailed)
		}
	})

	t.Run("it classifies a failed delete-phase load as store-read-failed", func(t *testing.T) {
		if reason := deletePhaseReadFailureCycle(t).DeclineReason; reason != skipReasonStoreReadFailed {
			t.Errorf("DeclineReason = %q, want %q", reason, skipReasonStoreReadFailed)
		}
	})

	t.Run("it leaves a failed save on the unclassified path", func(t *testing.T) {
		logtest.Install(t)
		store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed, WritesDenied: true})

		outcome, err := runHookStaleCleanup(&stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, store)
		if err == nil {
			t.Fatal("runHookStaleCleanup returned no error for a save it could not make")
		}
		if outcome.DeclineReason != "" {
			t.Errorf("DeclineReason = %q, want none — a failed write is no stand-down", outcome.DeclineReason)
		}
	})

	t.Run("it renders the same stand-down phrase for both read failures on both surfaces", func(t *testing.T) {
		surfaces := map[string]map[skipReason]string{
			"repair":    skippedPrunePhrases,
			"diagnosis": notEvaluableDetails,
		}
		failures := map[string]sweepOutcome{
			"pre-read":     preReadFailureCycle(t),
			"delete-phase": deletePhaseReadFailureCycle(t),
		}

		for surface, phrases := range surfaces {
			for failure, outcome := range failures {
				phrase := phraseFor(phrases, outcome.DeclineReason)
				if phrase != storeReadStandDownPhrase {
					t.Errorf("%s surface renders the %s read failure as %q, want %q",
						surface, failure, phrase, storeReadStandDownPhrase)
				}
			}
		}
	})
}
