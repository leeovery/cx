package cmd

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/tmux"
)

// A daemon-sweep fixture that expects an entry to survive must key it on a
// token the enumeration reports, or the entry survives on the reaper's
// retention of keys it cannot judge and the fixture stops measuring liveness.
var livePaneRowOut = hookstest.LiveSeedA + "|live:0.0"

func hookCleanupDeps(fc *daemonFakeCommander, store *hooks.Store, logger *slog.Logger) *daemonDeps {
	return &daemonDeps{
		Client:    tmux.NewClient(fc),
		HookStore: store,
		Logger:    logger,
	}
}

func discardDaemonLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func TestMaybeRunHookCleanup_DoesNotRunBeforeInterval(t *testing.T) {
	seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-stale"}
}`, hookstest.ReapableSeedA)
	store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: seed})
	fc := &daemonFakeCommander{panesOut: livePaneRowOut}
	deps := hookCleanupDeps(fc, store, discardDaemonLogger())

	anchor := time.Now()
	deps.lastCleanup = anchor

	maybeRunHookCleanup(deps)

	postRun, err := store.Load(hooks.ViaInternal)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if _, ok := postRun[hookstest.ReapableSeedA]; !ok {
		t.Errorf("stale entry reaped before interval elapsed; hooks=%v", keysOf(postRun))
	}

	if got := fc.callsContaining("list-panes"); len(got) != 0 {
		t.Errorf("list-panes invoked before interval elapsed: %v", got)
	}

	if !deps.lastCleanup.Equal(anchor) {
		t.Errorf("lastCleanup advanced before interval elapsed: got %v, want %v", deps.lastCleanup, anchor)
	}
}

func TestMaybeRunHookCleanup_RunsAndResetsOnceIntervalElapsed(t *testing.T) {
	store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: hookstest.StaleHookSeed})
	fc := &daemonFakeCommander{panesOut: livePaneRowOut}
	deps := hookCleanupDeps(fc, store, discardDaemonLogger())

	deps.lastCleanup = time.Now().Add(-hookCleanupInterval - time.Second)
	beforeCall := time.Now()

	maybeRunHookCleanup(deps)

	postRun, err := store.Load(hooks.ViaInternal)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if _, ok := postRun[hookstest.ReapableSeedA]; ok {
		t.Errorf("stale entry not reaped once interval elapsed; hooks=%v", keysOf(postRun))
	}
	if _, ok := postRun[hookstest.LiveSeedA]; !ok {
		t.Errorf("live entry wrongly reaped; hooks=%v", keysOf(postRun))
	}

	if deps.lastCleanup.Before(beforeCall) {
		t.Errorf("lastCleanup not advanced after cleanup: got %v, want >= %v", deps.lastCleanup, beforeCall)
	}
}

func TestMaybeRunHookCleanup_FiresAtIntervalBoundary(t *testing.T) {
	seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-stale"}
}`, hookstest.ReapableSeedA)
	store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: seed})
	fc := &daemonFakeCommander{panesOut: livePaneRowOut}
	deps := hookCleanupDeps(fc, store, discardDaemonLogger())

	deps.lastCleanup = time.Now().Add(-hookCleanupInterval)

	maybeRunHookCleanup(deps)

	postRun, err := store.Load(hooks.ViaInternal)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if _, ok := postRun[hookstest.ReapableSeedA]; ok {
		t.Errorf("cleanup did not fire at the interval boundary; hooks=%v", keysOf(postRun))
	}
}

// A failure the sweep does not name as a stand-down is the daemon's to report:
// the cycle is swallowed so the tick survives it, and the throttle anchor still
// advances so a failing prune backs off instead of retrying every tick.
func TestMaybeRunHookCleanup_LogsWarnAndSwallowsCleanupError(t *testing.T) {
	// A denied write is a failure the sweep returns rather than standing down
	// on: the mutation takes its lock and reads cleanly, then fails at the temp
	// create.
	store, _ := hookstest.StageStore(t, hookstest.Staging{
		Dir:          filepath.Join(t.TempDir(), "write-denied"),
		Seed:         hookstest.StaleHookSeed,
		WritesDenied: true,
	})

	// Non-empty panesOut keeps the mass-deletion guard off the path, so the
	// write is reached and the returned-error branch is the one exercised.
	fc := &daemonFakeCommander{panesOut: livePaneRowOut}
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps := hookCleanupDeps(fc, store, logger)

	deps.lastCleanup = time.Now().Add(-hookCleanupInterval - time.Second)
	beforeCall := time.Now()

	maybeRunHookCleanup(deps)

	if got := sink.Body(); !strings.Contains(got, "hooks stale-cleanup failed") {
		t.Errorf("expected gate WARN 'hooks stale-cleanup failed' under daemon component; got:\n%s", got)
	}

	if deps.lastCleanup.Before(beforeCall) {
		t.Errorf("lastCleanup not advanced after failing cleanup: got %v, want >= %v", deps.lastCleanup, beforeCall)
	}
}

func TestMaybeRunHookCleanup_ListPanesErrorSwallowedNoReap(t *testing.T) {
	seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-stale"}
}`, hookstest.ReapableSeedA)
	store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: seed})
	fc := &daemonFakeCommander{panesErr: errors.New("tmux dead")}
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps := hookCleanupDeps(fc, store, logger)

	deps.lastCleanup = time.Now().Add(-hookCleanupInterval - time.Second)
	beforeCall := time.Now()

	maybeRunHookCleanup(deps)

	postRun, err := store.Load(hooks.ViaInternal)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if _, ok := postRun[hookstest.ReapableSeedA]; !ok {
		t.Errorf("entry reaped despite ListAllPanes error; hooks=%v", keysOf(postRun))
	}

	if got := sink.Body(); strings.Contains(got, "hooks stale-cleanup failed") {
		t.Errorf("gate WARN fired despite swallowed ListAllPanes error; got:\n%s", got)
	}

	if deps.lastCleanup.Before(beforeCall) {
		t.Errorf("lastCleanup not advanced after swallowed list error: got %v, want >= %v", deps.lastCleanup, beforeCall)
	}
}

func TestMaybeRunHookCleanup_NilStoreNoOps(t *testing.T) {
	// A nil HookStore is what daemon startup leaves behind when loadHookStore fails.
	fc := &daemonFakeCommander{panesOut: livePaneRowOut}
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps := hookCleanupDeps(fc, nil, logger)

	anchor := time.Now().Add(-hookCleanupInterval - time.Second)
	deps.lastCleanup = anchor

	maybeRunHookCleanup(deps)

	if got := fc.callsContaining("list-panes"); len(got) != 0 {
		t.Errorf("list-panes invoked with a nil store: %v", got)
	}

	if !deps.lastCleanup.Equal(anchor) {
		t.Errorf("lastCleanup mutated with a nil store: got %v, want %v", deps.lastCleanup, anchor)
	}

	if got := sink.Body(); strings.Contains(got, "hooks stale-cleanup failed") {
		t.Errorf("gate WARN fired with a nil store; got:\n%s", got)
	}
}

func TestMaybeRunHookCleanup_ReusesMassDeletionGuard(t *testing.T) {
	seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-a"},
  %q: {"on-resume": "cmd-b"}
}`, hookstest.ReapableSeedA, hookstest.ReapableSeedB)
	store, _ := hookstest.StageStore(t, hookstest.Staging{Seed: seed})
	fc := &daemonFakeCommander{panesOut: ""}
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps := hookCleanupDeps(fc, store, logger)

	hooksSink := logtest.Install(t)

	deps.lastCleanup = time.Now().Add(-hookCleanupInterval - time.Second)

	maybeRunHookCleanup(deps)

	postRun, err := store.Load(hooks.ViaInternal)
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if len(postRun) != 2 {
		t.Errorf("mass-deletion guard did not defer; post-run hooks=%v", keysOf(postRun))
	}

	// The guard's WARN rides the hooks component under the shared stand-down
	// shape, not the daemon logger the cleanup is handed.
	if got := sink.Body(); strings.Contains(got, "clean-stale-skipped") {
		t.Errorf("stand-down WARN landed on the daemon logger; got:\n%s", got)
	}
	rec := hooksSink.OnlyRecord(t)
	if rec.Level != slog.LevelWarn {
		t.Errorf("stand-down level = %v, want WARN", rec.Level)
	}
	if got := rec.AttrString(t, "reason"); got != skipReasonEmptyPaneRead {
		t.Errorf("reason = %q, want %q", got, skipReasonEmptyPaneRead)
	}
}
