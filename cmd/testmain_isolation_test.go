package cmd

// TestMain poisons every PORTAL_*_FILE / PORTAL_STATE_DIR env var to a
// deliberately-invalid path before any test in the cmd package binary runs.
// Tests that correctly isolate (via t.Setenv("PORTAL_STATE_DIR", t.TempDir())
// and siblings) override the poison normally — no behaviour change for
// correctly-written tests. Tests that forget to isolate hit the poisoned
// paths and fail loudly trying to read or write, rather than silently
// mutating the developer's real ~/.config/portal/ configuration.
//
// Subprocess inheritance: when a test spawns exec.Command(...) without
// explicitly setting cmd.Env, the subprocess inherits os.Environ() — which
// includes the poisoned values from this TestMain. So the symptom-fixture
// class of bug (subprocess inherits developer's real env because PORTAL_*_FILE
// wasn't t.Setenv'd) becomes structurally impossible to ship: the subprocess
// either uses a poisoned (failing) path or a test-supplied (isolated) one.
//
// The poison paths use /nonexistent/portal-test-must-isolate-* so a writer
// fails at the parent-dir-missing stage (AtomicWrite temp-create) rather
// than silently succeeding. A reader against an absent file gets ENOENT,
// which some production code paths tolerate (e.g. hooks.Store.Load returns
// empty map on ENOENT) — those silently-tolerant paths simply behave as if
// the config is empty, which is the correct test-isolation semantic.
//
// This file is the structural counterpart to portaltest.IsolateStateForTest:
// IsolateStateForTest is opt-in per-test; this TestMain makes opt-out the
// failing default. Together they close the test-fixture-env-isolation class
// of bug (cleanup-purge-test-no-state-isolation, symptom-fixture wipes hooks)
// at the package level rather than relying on per-test contributor discipline.

import (
	"os"
	"testing"

	"github.com/leeovery/portal/internal/prefs"
)

func TestMain(m *testing.M) {
	os.Setenv("PORTAL_STATE_DIR", "/nonexistent/portal-test-must-isolate-state")
	os.Setenv("PORTAL_HOOKS_FILE", "/nonexistent/portal-test-must-isolate-hooks.json")
	os.Setenv("PORTAL_PROJECTS_FILE", "/nonexistent/portal-test-must-isolate-projects.json")
	os.Setenv("PORTAL_ALIASES_FILE", "/nonexistent/portal-test-must-isolate-aliases")
	// PORTAL_THEMES_DIR resolves a directory rather than a file, but it is
	// poisoned for the identical structural reason: a cmd body that forgets to
	// isolate must fail loudly instead of enumerating the developer's real
	// ~/.config/portal/themes/. Tests that exercise the themes chain set it
	// explicitly with t.Setenv.
	os.Setenv("PORTAL_THEMES_DIR", "/nonexistent/portal-test-must-isolate-themes")
	// PORTAL_PREFS_FILE joins the poison set with doctor's persisted-theme
	// advisory, which is the first production path to resolve prefs.json from a
	// command body a test Executes: `portal doctor` now reads the file on every
	// run, so an unpoisoned resolution would read the DEVELOPER's real
	// ~/.config/portal/prefs.json and let their own theme choice decide whether
	// the doctor report tests pass. (The one-shot old-macOS-path migrate that
	// rides the same resolution would also be a genuine write outside a temp
	// dir.) Tests that exercise prefs set it explicitly with t.Setenv — see
	// setPrefsFile — and prefsFilePath's fallback chain is covered by tests that
	// set it to the empty string, which reads as unset.
	os.Setenv("PORTAL_PREFS_FILE", "/nonexistent/portal-test-must-isolate-prefs.json")
	// TMUX poison — the tmux-boundary counterpart of the path poisons above.
	// Tests usually run inside the developer's real tmux, so any test that
	// Executes a real command body whose production wiring builds
	// tmux.DefaultClient() (state cleanup, clean, hooks, commit-now, daemon,
	// signal-hydrate, hydrate, the production orchestrator) would otherwise
	// inherit the ambient TMUX and operate on the REAL server. Incident of
	// record: two tests ran the real `portal state cleanup` body uninjected
	// and kill-sessioned the developer's live _portal-saver on every
	// `go test ./cmd`. With the poison, a missed injection dials a dead
	// socket and fails loudly. Tests that need a real server use tmuxtest's
	// explicit per-test -S sockets (which override TMUX) and set their own
	// TMUX for subprocesses; tests asserting inside/outside-tmux BEHAVIOUR
	// must t.Setenv TMUX themselves (they already had to, to be runnable
	// both inside and outside a tmux window).
	os.Setenv("TMUX", "/nonexistent/portal-test-must-set-tmux-socket,0,0")
	// The §10.5 appearance-translation dispatch is neutralised package-wide for
	// the same structural reason, and it is a PROCESS poison rather than a path
	// one: production runs the persist on its own goroutine, so any test that
	// reaches loadPrefsStore would otherwise leave a write racing its own
	// teardown — mutating a t.TempDir prefs.json after the test returned (and
	// after the TempDir cleanup started removing it), and emitting a `theme`
	// record into whatever sink the NEXT test installs. Tests that exercise the
	// dispatch install their own implementation and restore this no-op with
	// t.Cleanup (see syncPersistTranslation / recordPersistTranslation).
	persistTranslation = func(*prefs.Store, string) {}
	os.Exit(m.Run())
}
