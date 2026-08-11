package cmd

// TestMain poisons the PORTAL_* config paths and TMUX package-wide so a test that
// forgets to isolate fails loudly instead of reading or mutating the developer's
// real config and tmux server; subprocesses inherit the poison via os.Environ().

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
	os.Setenv("PORTAL_THEMES_DIR", "/nonexistent/portal-test-must-isolate-themes")
	os.Setenv("PORTAL_PREFS_FILE", "/nonexistent/portal-test-must-isolate-prefs.json")
	os.Setenv("TMUX", "/nonexistent/portal-test-must-set-tmux-socket,0,0")
	// Neutralised package-wide: production dispatches this on its own goroutine, so
	// a test reaching loadPrefsStore would leave a write racing its own teardown.
	persistTranslation = func(*prefs.Store, string) {}
	os.Exit(m.Run())
}
