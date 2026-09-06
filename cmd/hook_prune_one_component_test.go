package cmd

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/logtest"
)

// componentOf names the component a captured record was emitted under, so a
// case asserting where a whole cycle landed reads the attribution off every
// record rather than one it selected in advance.
func componentOf(rec logtest.Record) string { return rec.AttrOrEmpty("component") }

// sweepFailureFixture stages a store whose clean fails for a reason the cycle
// does not classify: the sidecar is absent and the directory denies writes, so
// the deletion's own lock open fails. The store's own line on that path is a
// degraded-read DEBUG, so the cycle's failure is the only record at WARN.
func sweepFailureFixture(t *testing.T) *hooks.Store {
	t.Helper()
	store, _ := hookstest.StageStore(t, hookstest.Staging{
		Dir:           filepath.Join(t.TempDir(), "lock-open-denied"),
		Seed:          hookstest.StaleHookSeed,
		SidecarAbsent: true,
		WritesDenied:  true,
	})
	return store
}

// One cycle is one story, and reconstructing it must not mean correlating three
// components: whatever a cycle has to say about itself belongs to the subsystem
// it sweeps, whichever caller drove it. That the cycle emits under one component
// at all is internal/hooksweep's own to pin; what these cases add is that its two
// callers reach it identically and neither adds a line of its own over it.
func TestHookSweepEmitsOneCycleUnderOneComponent(t *testing.T) {
	t.Run("it emits the same component and messages from the daemon and from doctor --fix", func(t *testing.T) {
		daemonSink := logtest.Install(t)
		daemonStore, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
		deps := hookCleanupDeps(&daemonFakeCommander{panesOut: livePaneRowOut}, daemonStore, discardDaemonLogger())
		deps.lastCleanup = time.Now().Add(-2 * hookCleanupInterval)

		maybeRunHookCleanup(deps)
		daemonLines := componentMessages(daemonSink)

		doctorSink := logtest.Install(t)
		doctorStore, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
		fixDeps := staleDeps(t.TempDir(), &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, doctorStore, nil)

		pruneDoctorStaleHooks(new(bytes.Buffer), fixDeps)
		doctorLines := componentMessages(doctorSink)

		if len(daemonLines) == 0 {
			t.Fatal("the daemon cycle emitted nothing; the comparison measures nothing")
		}
		if !slices.Equal(daemonLines, doctorLines) {
			t.Errorf("daemon lines = %v, doctor --fix lines = %v; want the same component and messages", daemonLines, doctorLines)
		}
	})

	t.Run("it emits exactly one record for an unclassified sweep failure", func(t *testing.T) {
		sink := logtest.Install(t)
		deps := hookCleanupDeps(&daemonFakeCommander{panesOut: livePaneRowOut}, sweepFailureFixture(t), discardDaemonLogger())
		deps.lastCleanup = time.Now().Add(-2 * hookCleanupInterval)

		maybeRunHookCleanup(deps)

		rec := sink.Records().AtOrAboveLevel(slog.LevelWarn).Only(t, "record at or above WARN")
		if got := componentOf(rec); got != "hooks" {
			t.Errorf("failure record component = %q, want hooks: %+v", got, rec)
		}
		if rec.Msg != sweepFailedMsg {
			t.Errorf("failure record message = %q, want %q", rec.Msg, sweepFailedMsg)
		}
		if rec.ErrorAttr(t, "error") == nil {
			t.Error("failure record carries no error")
		}
	})

	t.Run("it still renders the caller-facing skipped-prune line for a failed sweep", func(t *testing.T) {
		sink := logtest.Install(t)
		deps := staleDeps(t.TempDir(), &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, sweepFailureFixture(t), nil)

		var out bytes.Buffer
		pruneDoctorStaleHooks(&out, deps)

		want := "Skipped stale hook prune: " + sweepFailedStandDownPhrase + "\n"
		if out.String() != want {
			t.Errorf("stdout = %q, want %q", out.String(), want)
		}

		// The rendered line is this caller's own output; the log line for the
		// same failure is the sweep's, and adding a second one here would put
		// two records in the log for one event.
		rec := sink.Records().AtOrAboveLevel(slog.LevelWarn).Only(t, "record at or above WARN")
		if got := componentOf(rec); got != "hooks" {
			t.Errorf("failure record component = %q, want hooks: %+v", got, rec)
		}
		if rec.Msg != sweepFailedMsg {
			t.Errorf("failure record message = %q, want %q", rec.Msg, sweepFailedMsg)
		}
	})
}

// componentMessages renders a capture as the "<component> <message>" pairs it
// holds, in capture order, which is the whole of what a parity case compares.
func componentMessages(sink *logtest.Sink) []string {
	lines := make([]string, 0, len(sink.Records()))
	for _, rec := range sink.Records() {
		lines = append(lines, componentOf(rec)+" "+rec.Msg)
	}
	return lines
}
