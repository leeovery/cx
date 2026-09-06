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
		if got := len(injectedSink.Records()); got != 0 {
			t.Errorf("daemon-logger record count = %d, want 0; entries=%+v", got, injectedSink.Records())
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
