package transienttest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
)

// ResolveHooksFilePathFromEnv resolves hooks.json the way production does, but
// against a supplied env slice rather than os.Getenv: PORTAL_HOOKS_FILE wins,
// else XDG_CONFIG_HOME. Neither present is an isolation regression and fatals.
func ResolveHooksFilePathFromEnv(t *testing.T, env []string) string {
	t.Helper()
	const (
		hooksFileKey = "PORTAL_HOOKS_FILE="
		xdgKey       = "XDG_CONFIG_HOME="
	)
	var xdg string
	for _, e := range env {
		if after, ok := strings.CutPrefix(e, hooksFileKey); ok {
			return after
		}
		if after, ok := strings.CutPrefix(e, xdgKey); ok {
			xdg = after
		}
	}
	if xdg == "" {
		t.Fatalf("transienttest.ResolveHooksFilePathFromEnv: env slice contains neither PORTAL_HOOKS_FILE nor XDG_CONFIG_HOME — IsolateStateForTest isolation regression")
	}
	return filepath.Join(xdg, "portal", "hooks.json")
}

// SeedHooksJSON writes a hooks.json of {hookKey: onResumeCommand} entries via
// the production store, so the on-disk layout matches what the CLI produces.
func SeedHooksJSON(t *testing.T, env []string, entries map[string]string) {
	t.Helper()
	path := ResolveHooksFilePathFromEnv(t, env)
	t.Logf("transienttest.SeedHooksJSON: resolved hooks.json path = %s", path)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("transienttest.SeedHooksJSON: mkdir %s: %v", filepath.Dir(path), err)
	}

	store := hooks.NewStore(path)
	for key, cmd := range entries {
		if err := store.Set(key, "on-resume", cmd, "cli"); err != nil {
			t.Fatalf("transienttest.SeedHooksJSON: set %s=%q: %v", key, cmd, err)
		}
	}
}

// HooksJSONBytes returns the raw on-disk bytes of hooks.json, or nil when the
// file is absent. Any other read error fails the test.
func HooksJSONBytes(t *testing.T, env []string) []byte {
	t.Helper()
	path := ResolveHooksFilePathFromEnv(t, env)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		t.Fatalf("transienttest.HooksJSONBytes: read %s: %v", path, err)
	}
	return data
}
