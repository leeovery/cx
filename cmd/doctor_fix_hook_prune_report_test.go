package cmd

import (
	"log/slog"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/logtest"
)

// A user who asked for a repair is owed a sentence about it whatever the sweep
// did, so every path out of the hook prune reaches the same one line: the keys
// it removed, the reason it stood down, or the failure that stopped it.
func TestDoctorFixAlwaysReportsTheHookPrune(t *testing.T) {
	t.Run("it prints one pruned line per reaped key", func(t *testing.T) {
		deps, _, _, _, _ := seedStalePruneFixture(t, t.TempDir(), staleHookLister())

		outBuf, _, _ := runDoctorWith(t, deps, "--fix")

		want := "Pruned stale hook: " + hookstest.ReapableSeedA
		if got := countExactLines(outBuf.String(), want); got != 1 {
			t.Errorf("lines equal to %q = %d, want 1:\n%s", want, got, outBuf.String())
		}
		if got := countPrefixedLines(outBuf.String(), "Pruned stale hook:"); got != 1 {
			t.Errorf("`Pruned stale hook:` lines = %d, want one per reaped key:\n%s", got, outBuf.String())
		}
	})

	// This case is about the path reaching a line at all rather than about its
	// words, so it renders the line it expects through the same vocabulary the
	// report does instead of restating it.
	t.Run("it prints a skipped line when the sweep stands down", func(t *testing.T) {
		deps, _, _, _, _ := seedStalePruneFixture(t, t.TempDir(), restoringHookLister())

		outBuf, _, _ := runDoctorWith(t, deps, "--fix")

		assertSkippedPruneLine(t, outBuf.String(), renderSkippedPruneLine(skipReasonRestoring))
	})

	t.Run("it prints a skipped line when the stale-hook sweep fails", func(t *testing.T) {
		sink := logtest.Install(t)
		deps := failingSweepDeps(t)

		outBuf, _, _ := runDoctorWith(t, deps, "--fix")

		assertSkippedPruneLine(t, outBuf.String(), "Skipped stale hook prune: the sweep could not complete")
		if len(sink.Records().Matching("hooks", sweepFailedMsg).AtExactLevel(slog.LevelWarn)) != 1 {
			t.Errorf("want one WARN naming the failed sweep under the hooks component; records=%+v", sink.Records())
		}

		// The stand-down copy table cannot hold this reason, so the raw-slug
		// guard every other reason gets from it lives here instead.
		if phrase := phraseFor(skippedPrunePhrases, skipReasonSweepFailed); phrase == string(skipReasonSweepFailed) || strings.Contains(phrase, string(skipReasonSweepFailed)) {
			t.Errorf("phraseFor(skippedPrunePhrases, %q) = %q; want a user-facing phrase, not the enum value", skipReasonSweepFailed, phrase)
		}
	})
}

// failingSweepDeps stages a genuinely stale entry the sweep cannot delete: the
// reads all succeed and the write fails, which is the one path that leaves the
// cycle with an error rather than a stand-down.
func failingSweepDeps(t *testing.T) *DoctorDeps {
	t.Helper()

	dir := t.TempDir()
	seedHealthyStateDir(t, dir)
	projectStore, _ := seedProjectsJSON(t, t.TempDir())
	store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hooksBody(hookstest.ReapableSeedA), WritesDenied: true})
	return staleDeps(dir, staleHookLister(), store, projectStore)
}

func countExactLines(out, want string) int {
	n := 0
	for line := range strings.SplitSeq(out, "\n") {
		if line == want {
			n++
		}
	}
	return n
}

func countPrefixedLines(out, prefix string) int {
	n := 0
	for line := range strings.SplitSeq(out, "\n") {
		if strings.HasPrefix(line, prefix) {
			n++
		}
	}
	return n
}
