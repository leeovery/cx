package alias_test

// Poison the config env vars before any test runs, so a test that forgets to
// isolate fails loudly against /nonexistent instead of silently mutating the
// developer's real configuration. A properly isolated test overrides them.

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Setenv("PORTAL_STATE_DIR", "/nonexistent/portal-test-must-isolate-state")
	os.Setenv("PORTAL_HOOKS_FILE", "/nonexistent/portal-test-must-isolate-hooks.json")
	os.Setenv("PORTAL_PROJECTS_FILE", "/nonexistent/portal-test-must-isolate-projects.json")
	os.Setenv("PORTAL_ALIASES_FILE", "/nonexistent/portal-test-must-isolate-aliases")
	os.Exit(m.Run())
}
