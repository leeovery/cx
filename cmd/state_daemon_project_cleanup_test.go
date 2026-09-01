package cmd

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/project"
)

// lastProjectCleanup is left zero for the caller to drive to a controlled
// instant.
func projectCleanupDeps(store *project.Store, logger *slog.Logger) *daemonDeps {
	return &daemonDeps{
		ProjectStore: store,
		Logger:       logger,
	}
}

func projectPaths(t *testing.T, store *project.Store) []string {
	t.Helper()
	loaded, err := store.Load()
	if err != nil {
		t.Fatalf("store.Load: %v", err)
	}
	out := make([]string, 0, len(loaded))
	for _, p := range loaded {
		out = append(out, p.Path)
	}
	return out
}

func TestMaybeRunProjectCleanup_PrunesGoneDirOnceIntervalElapsed(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone-dir-does-not-exist")
	store, _ := seedProjectsJSON(t, gone)
	deps := projectCleanupDeps(store, discardDaemonLogger())

	deps.lastProjectCleanup = time.Now().Add(-projectCleanupInterval - time.Second) // elapsed
	beforeCall := time.Now()

	maybeRunProjectCleanup(deps)

	if paths := projectPaths(t, store); len(paths) != 0 {
		t.Errorf("gone-dir project not pruned once interval elapsed; paths=%v", paths)
	}

	if deps.lastProjectCleanup.Before(beforeCall) {
		t.Errorf("lastProjectCleanup not advanced after cleanup: got %v, want >= %v", deps.lastProjectCleanup, beforeCall)
	}
}

func TestMaybeRunProjectCleanup_RetainsLiveDirProject(t *testing.T) {
	// EACCES is hard to simulate portably, so the live-dir survivor stands in for
	// the retention path.
	live := t.TempDir()
	store, _ := seedProjectsJSON(t, live)
	deps := projectCleanupDeps(store, discardDaemonLogger())

	deps.lastProjectCleanup = time.Now().Add(-projectCleanupInterval - time.Second) // elapsed

	maybeRunProjectCleanup(deps)

	paths := projectPaths(t, store)
	if len(paths) != 1 || paths[0] != live {
		t.Errorf("live-dir project wrongly pruned; paths=%v want [%s]", paths, live)
	}
}

func TestMaybeRunProjectCleanup_DoesNotRunBeforeInterval(t *testing.T) {
	gone := filepath.Join(t.TempDir(), "gone-dir-does-not-exist")
	store, _ := seedProjectsJSON(t, gone)
	deps := projectCleanupDeps(store, discardDaemonLogger())

	anchor := time.Now() // NOT elapsed: time.Since(anchor) < projectCleanupInterval
	deps.lastProjectCleanup = anchor

	maybeRunProjectCleanup(deps)

	if paths := projectPaths(t, store); len(paths) != 1 {
		t.Errorf("prune ran before interval elapsed; paths=%v", paths)
	}

	if !deps.lastProjectCleanup.Equal(anchor) {
		t.Errorf("lastProjectCleanup advanced before interval elapsed: got %v, want %v", deps.lastProjectCleanup, anchor)
	}
}

func TestMaybeRunProjectCleanup_LogsWarnAndSwallowsCleanStaleError(t *testing.T) {
	// Force a Load error by pointing the store at a directory: os.ReadFile on one
	// errors with a non-ErrNotExist error, which CleanStale surfaces.
	dir := t.TempDir()
	bogusPath := filepath.Join(dir, "projects.json")
	if err := os.MkdirAll(bogusPath, 0o755); err != nil {
		t.Fatalf("mkdir bogus projects path: %v", err)
	}
	store := project.NewStore(bogusPath)

	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps := projectCleanupDeps(store, logger)

	deps.lastProjectCleanup = time.Now().Add(-projectCleanupInterval - time.Second) // elapsed
	beforeCall := time.Now()

	maybeRunProjectCleanup(deps)

	if got := sink.Body(); !strings.Contains(got, "projects stale-cleanup failed") {
		t.Errorf("expected gate WARN 'projects stale-cleanup failed' under daemon component; got:\n%s", got)
	}

	// The throttle still advances after a failing cleanup — retry next cadence,
	// not every tick.
	if deps.lastProjectCleanup.Before(beforeCall) {
		t.Errorf("lastProjectCleanup not advanced after failing cleanup: got %v, want >= %v", deps.lastProjectCleanup, beforeCall)
	}
}

func TestMaybeRunProjectCleanup_NilStoreNoOps(t *testing.T) {
	// A nil store means loadProjectStore failed at startup and the prune is
	// disabled for the daemon's lifetime: the gate must be a pure no-op even
	// once the interval has elapsed.
	logger, sink := newCaptureLoggerForComponent(t, "daemon")
	deps := projectCleanupDeps(nil, logger) // nil store — prune disabled

	anchor := time.Now().Add(-projectCleanupInterval - time.Second) // elapsed
	deps.lastProjectCleanup = anchor

	maybeRunProjectCleanup(deps)

	if !deps.lastProjectCleanup.Equal(anchor) {
		t.Errorf("lastProjectCleanup mutated with a nil store: got %v, want %v", deps.lastProjectCleanup, anchor)
	}

	if got := sink.Body(); strings.Contains(got, "projects stale-cleanup failed") {
		t.Errorf("gate WARN fired with a nil store; got:\n%s", got)
	}
}

func TestStateDaemon_ProjectCleanupWiring(t *testing.T) {
	t.Run("it builds the project store from loadProjectStore at startup", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("PORTAL_STATE_DIR", dir)
		t.Setenv("PORTAL_PROJECTS_FILE", filepath.Join(t.TempDir(), "projects.json"))

		holder := withImmediateRun(t)
		withDaemonLockFileReset(t)

		if _, _, err := runStateDaemon(t); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		deps := *holder
		if deps == nil {
			t.Fatal("daemon deps not captured")
		}
		if deps.ProjectStore == nil {
			t.Fatal("deps.ProjectStore is nil; want a non-nil store built from loadProjectStore()")
		}
	})

	t.Run("it initialises lastProjectCleanup to a non-zero start time", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("PORTAL_STATE_DIR", dir)
		t.Setenv("PORTAL_PROJECTS_FILE", filepath.Join(t.TempDir(), "projects.json"))

		holder := withImmediateRun(t)
		withDaemonLockFileReset(t)

		if _, _, err := runStateDaemon(t); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		now := time.Now()

		deps := *holder
		if deps == nil {
			t.Fatal("daemon deps not captured")
		}
		if deps.lastProjectCleanup.IsZero() {
			t.Fatal("deps.lastProjectCleanup is the zero time.Time; want the daemon-start instant so the first prune fires one interval after start, not on the first idle tick")
		}
		if delta := now.Sub(deps.lastProjectCleanup); delta < 0 || delta > 2*time.Second {
			t.Errorf("deps.lastProjectCleanup = %v; want within 2s of %v (delta %v)", deps.lastProjectCleanup, now, delta)
		}
	})

	// A loadProjectStore failure must not abort the daemon — capture cannot be
	// gated on the best-effort prune, so the failure only disables it. With
	// PORTAL_PROJECTS_FILE unset the path resolution reduces to os.UserHomeDir(),
	// which blanking $HOME fails deterministically.
	t.Run("it disables the prune with a WARN rather than aborting the daemon on a loadProjectStore error", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("PORTAL_STATE_DIR", dir)
		t.Setenv("PORTAL_PROJECTS_FILE", "") // force fall-through to home-dir resolution
		t.Setenv("HOME", "")                 // os.UserHomeDir() now errors → loadProjectStore errors

		sink := logtest.Install(t)

		holder := withImmediateRun(t)
		withDaemonLockFileReset(t)

		if _, _, err := runStateDaemon(t); err != nil {
			t.Fatalf("RunE must proceed to the tick loop despite loadProjectStore failure; got error: %v", err)
		}

		deps := *holder
		if deps == nil {
			t.Fatal("daemon deps not captured")
		}
		if deps.ProjectStore != nil {
			t.Errorf("deps.ProjectStore = %v; want nil (prune disabled on loadProjectStore failure)", deps.ProjectStore)
		}

		body := sink.Body()
		const warnMsg = "load project store failed; stale-project prune disabled"
		if !strings.Contains(body, warnMsg) {
			t.Errorf("expected disabled-prune WARN %q; got:\n%s", warnMsg, body)
		}
		if !strings.Contains(body, "component=daemon") {
			t.Errorf("expected the disabled-prune WARN under the daemon component; got:\n%s", body)
		}
	})
}
