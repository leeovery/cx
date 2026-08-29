package transienttest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/nanoid"
)

// reapableSeedPrefix keeps a reapable seed key legible in test output while
// leaving room for the n disambiguator inside the token width.
const reapableSeedPrefix = "reap"

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
		if err := store.Set(key, "on-resume", cmd, hooks.ViaCLI); err != nil {
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

// ReapableHookKey returns a seed hook key the staleness rule can judge, so an
// entry under it is reaped once it is absent from the live pane set. The
// returned value is asserted token-shaped: a change to the token width or
// charset panics here rather than silently turning a reap fixture into a
// retention one. n only disambiguates — distinct n give distinct keys.
func ReapableHookKey(n int) string {
	radix := len(nanoid.Alphabet)
	if n < 0 || n >= radix*radix {
		panic(fmt.Sprintf("transienttest.ReapableHookKey: n = %d out of range [0,%d)", n, radix*radix))
	}
	key := reapableSeedPrefix + string([]byte{
		nanoid.Alphabet[n/radix],
		nanoid.Alphabet[n%radix],
	})
	if !nanoid.IsTokenShaped(key) {
		panic(fmt.Sprintf("transienttest.ReapableHookKey: %q is not token-shaped — the seed vocabulary has drifted from nanoid.IsTokenShaped", key))
	}
	return key
}

// UnjudgeableHookKey returns a seed hook key of the legacy `<name>:window.pane`
// shape, which the staleness rule cannot judge and therefore retains even when
// absent from the live pane set. n only disambiguates.
func UnjudgeableHookKey(n int) string {
	return fmt.Sprintf("unjudgeable-session-%d:0.0", n)
}
