package cmd

import (
	"bytes"
	"log/slog"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/logtest"
)

// A stand-down is one event, and an operator raising the level because a hook
// vanished must be able to count them: a caller that adds a failure report of
// its own over the sweep's own line makes one decline look like two.
func TestHookSweepReportsAStoreReadStandDownOnce(t *testing.T) {
	t.Run("it emits one WARN for an unreadable hooks.json on the daemon path", func(t *testing.T) {
		store, _ := hookstest.StageStore(t, hookstest.Staging{Unreadable: true})

		sink := logtest.Install(t)
		injected, injectedSink := newCaptureLoggerForComponent(t, "daemon")
		deps := hookCleanupDeps(&daemonFakeCommander{panesOut: livePaneRowOut}, store, injected)
		deps.lastCleanup = time.Now().Add(-2 * hookCleanupInterval)

		maybeRunHookCleanup(deps)

		assertStandDown(t, sink, slog.LevelWarn, skipReasonStoreReadFailed)
		if got := len(injectedSink.Records().Matching("daemon", "hooks stale-cleanup failed").AtExactLevel(slog.LevelWarn)); got != 0 {
			t.Errorf("daemon generic-failure WARN count = %d, want 0; entries=%+v", got, injectedSink.Records())
		}
	})

	t.Run("it emits one WARN for an unreadable hooks.json on the doctor --fix path", func(t *testing.T) {
		store, _ := hookstest.StageStore(t, hookstest.Staging{Unreadable: true})
		deps := staleDeps(t.TempDir(), staleHookLister(), store, nil)

		sink := logtest.Install(t)

		pruneDoctorStaleHooks(new(bytes.Buffer), deps)

		assertStandDown(t, sink, slog.LevelWarn, skipReasonStoreReadFailed)
	})

	// The daemon's throttled sweep under a held lock: one WARN for the cycle,
	// none of the daemon's own over it, and the file left as it was.
	t.Run("it still reports a lock-timeout stand-down exactly once", func(t *testing.T) {
		store, path, _ := lockedSweepFixture(t, lockBound)
		before := readFileBytes(t, path)

		sink := logtest.Install(t)
		injected, injectedSink := newCaptureLoggerForComponent(t, "daemon")
		deps := hookCleanupDeps(&daemonFakeCommander{panesOut: livePaneRowOut}, store, injected)
		deps.lastCleanup = time.Now().Add(-2 * hookCleanupInterval)

		maybeRunHookCleanup(deps)

		assertHooksFileUnchanged(t, path, before, "rewritten by the daemon under a held lock")
		assertStandDown(t, sink, slog.LevelWarn, skipReasonLockTimeout)
		if got := len(injectedSink.Records().AtExactLevel(slog.LevelWarn)); got != 0 {
			t.Errorf("injected-logger WARN count = %d, want 0; entries=%+v", got, injectedSink.Records())
		}
	})
}

// countsLogger is named for the two DEBUG lines it carries, and the sweep's
// stand-downs are not among them: they ride the hooks component whatever a
// caller injects, which is why a suite asserting on one also installs the
// process handler.
func TestHookSweepCountsLogger(t *testing.T) {
	t.Run("it emits the count lines under the component the signature names", func(t *testing.T) {
		store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
		injected, injectedSink := newCaptureLoggerForComponent(t, "daemon")

		outcome, err := runHookStaleCleanup(&stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA)}, store, injected)
		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}
		if len(outcome.Removed) != 1 {
			t.Fatalf("Removed = %v, want the one stale key (the reaped line has no cycle to report otherwise)", outcome.Removed)
		}

		for _, msg := range []string{"stale-hook cleanup counts", "stale-hook cleanup removed"} {
			if got := len(injectedSink.Records().Matching("daemon", msg).AtExactLevel(slog.LevelDebug)); got != 1 {
				t.Errorf("records matching debug/daemon/%q = %d, want 1; records=%+v", msg, got, injectedSink.Records())
			}
		}
		if got := len(injectedSink.Records()); got != 2 {
			t.Errorf("injected-logger record count = %d, want exactly the two counts; records=%+v", got, injectedSink.Records())
		}
	})

	t.Run("it emits no stand-down under the injected component", func(t *testing.T) {
		store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
		sink := logtest.Install(t)
		injected, injectedSink := newCaptureLoggerForComponent(t, "daemon")

		lister := &stubStaleSweepReader{rows: tokenRows(hookstest.LiveSeedA), restoring: true}
		if _, err := runHookStaleCleanup(lister, store, injected); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		assertStandDown(t, sink, slog.LevelDebug, skipReasonRestoring)
		if got := len(injectedSink.Records()); got != 0 {
			t.Errorf("injected-logger record count = %d, want 0; records=%+v", got, injectedSink.Records())
		}
	})
}
