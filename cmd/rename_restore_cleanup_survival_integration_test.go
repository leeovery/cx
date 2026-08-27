//go:build integration

package cmd

import (
	"slices"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// The recreated session's name is the post-rename one; restore re-stamps
// @portal-id onto it, and the hook was registered under that immutable id.
const (
	renameRestorePortalID = "tok123"
	renameRestoreName     = "renamedst"
)

func TestRenameRestoreCleanupSurvival_KeepsRestoredIdKeyedHook(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-3-6-clean-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	ts.Run(t, "new-session", "-d", "-s", renameRestoreName, "sleep", "infinity")
	ts.WaitForSession(t, renameRestoreName, 2*time.Second)

	// Restore's re-stamp: re-seed the recreated live session with its saved
	// @portal-id. Omitting it drops the live key back to the name and cleanup
	// deletes the id-keyed hook.
	if err := client.SetSessionOption(renameRestoreName, session.PortalIDOption, renameRestorePortalID); err != nil {
		t.Fatalf("SetSessionOption %s=%s: %v", session.PortalIDOption, renameRestorePortalID, err)
	}

	liveKey := tmux.HookKey(renameRestorePortalID, renameRestoreName, 0, 0)
	if liveKey != renameRestorePortalID+":0.0" {
		t.Fatalf("id-key = %q; want %q (id-key must not embed the name)", liveKey, renameRestorePortalID+":0.0")
	}

	assertLiveHookKeyPresent(t, client, liveKey)
	assertLiveHookKeyAbsent(t, client, renameRestoreName+":0.0")

	// Truly-stale entry with no matching live pane, token-shaped so the reaper
	// can judge it: must still be swept.
	const staleKey = "gone01"

	seed := `{
  "` + liveKey + `": {"on-resume": "echo restored"},
  "` + staleKey + `": {"on-resume": "echo gone"}
}`
	store, path := newTempHooksStore(t, seed)

	preRun, err := store.Load()
	if err != nil {
		t.Fatalf("pre-cleanup store.Load: %v", err)
	}
	if _, ok := preRun[liveKey]; !ok {
		t.Fatalf("pre-cleanup seed missing id-key %q; keys=%v", liveKey, keysOf(preRun))
	}
	if _, ok := preRun[staleKey]; !ok {
		t.Fatalf("pre-cleanup seed missing stale key %q; keys=%v", staleKey, keysOf(preRun))
	}

	if err := runHookStaleCleanup(client, store, nil, nil, nil); err != nil {
		t.Fatalf("runHookStaleCleanup: %v", err)
	}

	postRun, err := store.Load()
	if err != nil {
		t.Fatalf("post-cleanup store.Load: %v", err)
	}
	if _, ok := postRun[liveKey]; !ok {
		t.Errorf("freshly-restored id-keyed hook %q was swept; want preserved "+
			"(re-stamped live @portal-id makes the live key match the on-disk id-key — chain (b)). "+
			"post-cleanup keys=%v (path=%s)", liveKey, keysOf(postRun), path)
	}
	if _, ok := postRun[staleKey]; ok {
		t.Errorf("truly-stale hook %q survived; want swept "+
			"(no matching live pane — cleanup correctness must not be weakened). "+
			"post-cleanup keys=%v (path=%s)", staleKey, keysOf(postRun), path)
	}
}

// The re-stamp makes the id-key win, so the post-rename name key must be absent
// from the live-key set.
func assertLiveHookKeyAbsent(t *testing.T, lister AllPaneLister, notWant string) {
	t.Helper()
	live, err := lister.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}
	if slices.Contains(live, notWant) {
		t.Fatalf("live hook key %q WAS enumerated; want absent (a stamped session must resolve "+
			"to its @portal-id key, not the name — got %v)", notWant, live)
	}
}
