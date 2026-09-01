package cmd

import (
	"bytes"
	"errors"
	"log/slog"
	"strings"
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
		if got := len(injectedSink.RecordsAtExactLevelWith(slog.LevelWarn, "daemon", "hooks stale-cleanup failed")); got != 0 {
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

	t.Run("it still prints the skipped-prune line for a store-read stand-down", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, _ := hookstest.StageStore(t, hookstest.Staging{Unreadable: true})
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, staleHookLister(), hookStore, projectStore)

		outBuf, _, err := runDoctorWith(t, deps, "--fix")
		if err != nil {
			t.Fatalf("Execute err = %v\n%s", err, outBuf.String())
		}

		assertSkippedPruneLine(t, outBuf.String(), "Skipped stale hook prune: could not read hooks.json")
	})

	t.Run("it leaves the doctor exit code to the post-repair diagnosis", func(t *testing.T) {
		// An unreadable hooks.json is not evaluable rather than failing, so an
		// otherwise healthy install still exits zero over the stand-down.
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		hookStore, _ := hookstest.StageStore(t, hookstest.Staging{Unreadable: true})
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, staleHookLister(), hookStore, projectStore)

		outBuf, _, err := runDoctorWith(t, deps, "--fix")
		if err != nil {
			t.Fatalf("Execute err = %v; want nil over a healthy post-repair diagnosis\n%s", err, outBuf.String())
		}
		if !strings.Contains(outBuf.String(), "Skipped stale hook prune: could not read hooks.json") {
			t.Fatalf("fixture did not stand the prune down:\n%s", outBuf.String())
		}

		// The same stand-down with a genuinely failing check still exits non-zero.
		failingDir := t.TempDir()
		seedHealthyStateDir(t, failingDir)
		failingHooks, _ := hookstest.StageStore(t, hookstest.Staging{Unreadable: true})
		failingProjects, _ := seedProjectsJSON(t, t.TempDir())
		failingDeps := staleDeps(failingDir, staleHookLister(), failingHooks, failingProjects)
		failingDeps.SaverPresent = func() (bool, error) { return false, nil }

		failBuf, _, failErr := runDoctorWith(t, failingDeps, "--fix")
		if !errors.Is(failErr, ErrDoctorUnhealthy) {
			t.Fatalf("Execute err = %v; want ErrDoctorUnhealthy with a failing check\n%s", failErr, failBuf.String())
		}
	})

	t.Run("it still reports a lock-timeout stand-down exactly once", func(t *testing.T) {
		store, _, _ := lockedSweepFixture(t, lockBound)

		sink := logtest.Install(t)
		injected, injectedSink := newCaptureLoggerForComponent(t, "daemon")
		deps := hookCleanupDeps(&daemonFakeCommander{panesOut: livePaneRowOut}, store, injected)
		deps.lastCleanup = time.Now().Add(-2 * hookCleanupInterval)

		maybeRunHookCleanup(deps)

		assertStandDown(t, sink, slog.LevelWarn, skipReasonLockTimeout)
		if got := len(injectedSink.RecordsAtExactLevel(slog.LevelWarn)); got != 0 {
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
			if got := len(injectedSink.RecordsAtExactLevelWith(slog.LevelDebug, "daemon", msg)); got != 1 {
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
