package hookstest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/nanoid"
)

// reapableSeedPrefix keeps a reapable seed key legible in test output. It is
// fitted to whatever room the token width leaves beside the n disambiguator.
const reapableSeedPrefix = "reap"

// disambiguatorWidth is the number of trailing bytes ReapableHookKey spends on
// distinguishing one seed key from the next.
const disambiguatorWidth = 2

// paneTokenWidth is taken from the mint rather than declared here, so a seed
// key is authored at whatever width recognition reads.
var paneTokenWidth = sync.OnceValue(func() int {
	token, err := nanoid.NewPaneTokenGenerator()()
	if err != nil {
		panic(fmt.Sprintf("hookstest: mint a pane token to size the seed vocabulary: %v", err))
	}
	return len(token)
})

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
		t.Fatalf("hookstest.ResolveHooksFilePathFromEnv: env slice contains neither PORTAL_HOOKS_FILE nor XDG_CONFIG_HOME — IsolateStateForTest isolation regression")
	}
	return filepath.Join(xdg, "portal", "hooks.json")
}

// SeedHooksJSON writes a hooks.json of {hookKey: onResumeCommand} entries via
// the production store, so the on-disk layout matches what the CLI produces.
func SeedHooksJSON(t *testing.T, env []string, entries map[string]string) {
	t.Helper()
	path := ResolveHooksFilePathFromEnv(t, env)
	t.Logf("hookstest.SeedHooksJSON: resolved hooks.json path = %s", path)

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("hookstest.SeedHooksJSON: mkdir %s: %v", filepath.Dir(path), err)
	}

	store := hooks.NewStore(path)
	for key, cmd := range entries {
		if err := store.Set(key, "on-resume", cmd, hooks.ViaCLI); err != nil {
			t.Fatalf("hookstest.SeedHooksJSON: set %s=%q: %v", key, cmd, err)
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
		t.Fatalf("hookstest.HooksJSONBytes: read %s: %v", path, err)
	}
	return data
}

// ReapableHookKey returns a seed hook key the staleness rule can judge, so an
// entry under it is reaped once it is absent from the live pane set. The
// returned value is authored at the pane-token width and asserted token-shaped,
// so a change to the id charset panics here rather than silently turning a reap
// fixture into a retention one. n only disambiguates — distinct n give distinct
// keys.
func ReapableHookKey(n int) string {
	radix := len(nanoid.Alphabet)
	if n < 0 || n >= radix*radix {
		panic(fmt.Sprintf("hookstest.ReapableHookKey: n = %d out of range [0,%d)", n, radix*radix))
	}
	key := fitPrefix(paneTokenWidth()-disambiguatorWidth) + string([]byte{
		nanoid.Alphabet[n/radix],
		nanoid.Alphabet[n%radix],
	})
	if !nanoid.IsTokenShaped(key) {
		panic(fmt.Sprintf("hookstest.ReapableHookKey: %q is not token-shaped — the seed vocabulary has drifted from nanoid.IsTokenShaped", key))
	}
	return key
}

// fitPrefix returns the legible seed prefix as exactly width bytes, padded from
// the alphabet when the token width leaves more room than the prefix fills.
func fitPrefix(width int) string {
	if width < 1 {
		panic(fmt.Sprintf("hookstest: a pane token of %d bytes leaves no room for a legible seed prefix", paneTokenWidth()))
	}
	if width <= len(reapableSeedPrefix) {
		return reapableSeedPrefix[:width]
	}
	return reapableSeedPrefix + strings.Repeat(string(nanoid.Alphabet[0]), width-len(reapableSeedPrefix))
}

// UnjudgeableHookKey returns a seed hook key of the legacy `<name>:window.pane`
// shape, which the staleness rule cannot judge and therefore retains even when
// absent from the live pane set. n only disambiguates.
func UnjudgeableHookKey(n int) string {
	return fmt.Sprintf("unjudgeable-session-%d:0.0", n)
}
