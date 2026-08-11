package cmd

// Real tmux rather than a stub lister: the property under test is that tmux's
// own #{?@portal-id,...,#{session_name}} conditional resolves an un-stamped
// session to its name. A stub would hand back the name and prove nothing.

import (
	"slices"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestHookKeyNoRegressionUpgrade_UnstampedNameKeyedHookSurvives(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	// Raw new-session stamps no @portal-id, which is what a legacy or
	// manually-created session looks like.
	ts := tmuxtest.New(t, "ptl-upgrade-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const liveName = "legacy-proj"
	ts.Run(t, "new-session", "-d", "-s", liveName)
	ts.WaitForSession(t, liveName, 2*time.Second)

	// If a future change stamped @portal-id at new-session, this key would flip to
	// an id and the test would stop exercising the name fallback — fail loudly
	// rather than pass for the wrong reason.
	const liveKey = liveName + ":0.0"
	assertLiveHookKeyPresent(t, client, liveKey)

	const staleKey = "gone-session:0.0"

	seed := `{
  "` + liveKey + `": {"on-resume": "echo legacy"},
  "` + staleKey + `": {"on-resume": "echo gone"}
}`
	store, path := newTempHooksStore(t, seed)

	// Non-vacuity guard: without it a "survives" assertion could pass on an entry
	// that was never written, and "swept" on one never present.
	preRun, err := store.Load()
	if err != nil {
		t.Fatalf("pre-cleanup store.Load: %v", err)
	}
	if _, ok := preRun[liveKey]; !ok {
		t.Fatalf("pre-cleanup seed missing %q; keys=%v", liveKey, keysOf(preRun))
	}
	if _, ok := preRun[staleKey]; !ok {
		t.Fatalf("pre-cleanup seed missing %q; keys=%v", staleKey, keysOf(preRun))
	}

	if err := runHookStaleCleanup(client, store, nil, nil); err != nil {
		t.Fatalf("runHookStaleCleanup: %v", err)
	}

	postRun, err := store.Load()
	if err != nil {
		t.Fatalf("post-cleanup store.Load: %v", err)
	}
	if _, ok := postRun[liveKey]; !ok {
		t.Errorf("un-stamped session's name-keyed hook %q was swept; want preserved "+
			"(name-based live key coincides with the on-disk key — no-migration upgrade). "+
			"post-cleanup keys=%v (path=%s)", liveKey, keysOf(postRun), path)
	}
	if _, ok := postRun[staleKey]; ok {
		t.Errorf("truly-stale name-keyed hook %q survived; want swept "+
			"(no matching live pane — cleanup correctness must not be weakened). "+
			"post-cleanup keys=%v (path=%s)", staleKey, keysOf(postRun), path)
	}
}

// assertLiveHookKeyPresent pins the name fallback at the source, so a
// surviving-entry assertion cannot pass merely because the live set came back
// empty or erroring.
func assertLiveHookKeyPresent(t *testing.T, lister AllPaneLister, want string) {
	t.Helper()
	live, err := lister.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}
	if slices.Contains(live, want) {
		return
	}
	t.Fatalf("live hook key %q not enumerated; got %v (an un-stamped session must "+
		"resolve to its name-based key via HookKeyFormat's #{session_name} fallback)", want, live)
}
