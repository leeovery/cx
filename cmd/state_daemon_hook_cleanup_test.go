package cmd

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/tmux"
)

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
	seed := `{
  "stale:0.0": {"on-resume": "cmd-stale"}
}`
	store, _ := newTempHooksStore(t, seed)
	fc := &daemonFakeCommander{panesOut: "live:0.0"}
	deps := hookCleanupDeps(fc, store, discardDaemonLogger())

	anchor := time.Now()
	deps.lastCleanup = anchor

	maybeRunHookCleanup(deps)

	postRun, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if _, ok := postRun["stale:0.0"]; !ok {
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
	seed := `{
  "stale:0.0": {"on-resume": "cmd-stale"},
  "live:0.0": {"on-resume": "cmd-live"}
}`
	store, _ := newTempHooksStore(t, seed)
	fc := &daemonFakeCommander{panesOut: "live:0.0"}
	deps := hookCleanupDeps(fc, store, discardDaemonLogger())

	deps.lastCleanup = time.Now().Add(-hookCleanupInterval - time.Second)
	beforeCall := time.Now()

	maybeRunHookCleanup(deps)

	postRun, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if _, ok := postRun["stale:0.0"]; ok {
		t.Errorf("stale entry not reaped once interval elapsed; hooks=%v", keysOf(postRun))
	}
	if _, ok := postRun["live:0.0"]; !ok {
		t.Errorf("live entry wrongly reaped; hooks=%v", keysOf(postRun))
	}

	if deps.lastCleanup.Before(beforeCall) {
		t.Errorf("lastCleanup not advanced after cleanup: got %v, want >= %v", deps.lastCleanup, beforeCall)
	}
}

func TestMaybeRunHookCleanup_FiresAtIntervalBoundary(t *testing.T) {
	seed := `{
  "stale:0.0": {"on-resume": "cmd-stale"}
}`
	store, _ := newTempHooksStore(t, seed)
	fc := &daemonFakeCommander{panesOut: "live:0.0"}
	deps := hookCleanupDeps(fc, store, discardDaemonLogger())

	deps.lastCleanup = time.Now().Add(-hookCleanupInterval)

	maybeRunHookCleanup(deps)

	postRun, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if _, ok := postRun["stale:0.0"]; ok {
		t.Errorf("cleanup did not fire at the interval boundary; hooks=%v", keysOf(postRun))
	}
}

func TestMaybeRunHookCleanup_LogsWarnAndSwallowsCleanupError(t *testing.T) {
	// hooks.Store.Load returns an empty map (not an error) for malformed JSON, so
	// pointing it at a directory is the only reliable way to force a Load failure.
	dir := t.TempDir()
	bogusPath := filepath.Join(dir, "hooks.json")
	if err := os.MkdirAll(bogusPath, 0o755); err != nil {
		t.Fatalf("mkdir bogus hooks path: %v", err)
	}
	store := hooks.NewStore(bogusPath)

	// Non-empty panesOut keeps the mass-deletion guard off the path, so Load fails
	// first and the returned-error branch is the one exercised.
	fc := &daemonFakeCommander{panesOut: "live:0.0"}
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
	seed := `{
  "stale:0.0": {"on-resume": "cmd-stale"}
}`
	store, _ := newTempHooksStore(t, seed)
	fc := &daemonFakeCommander{panesErr: errors.New("tmux dead")}
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps := hookCleanupDeps(fc, store, logger)

	deps.lastCleanup = time.Now().Add(-hookCleanupInterval - time.Second)
	beforeCall := time.Now()

	maybeRunHookCleanup(deps)

	postRun, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if _, ok := postRun["stale:0.0"]; !ok {
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
	fc := &daemonFakeCommander{panesOut: "live:0.0"}
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
	seed := `{
  "a:0.0": {"on-resume": "cmd-a"},
  "b:0.0": {"on-resume": "cmd-b"}
}`
	store, _ := newTempHooksStore(t, seed)
	fc := &daemonFakeCommander{panesOut: ""}
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps := hookCleanupDeps(fc, store, logger)

	deps.lastCleanup = time.Now().Add(-hookCleanupInterval - time.Second)

	maybeRunHookCleanup(deps)

	postRun, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	if len(postRun) != 2 {
		t.Errorf("mass-deletion guard did not defer; post-run hooks=%v", keysOf(postRun))
	}

	if got := sink.Body(); !strings.Contains(got, "mass-deletion hazard") {
		t.Errorf("expected mass-deletion hazard WARN; got:\n%s", got)
	}
}
