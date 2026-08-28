//go:build integration

package cmd

import (
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmuxtest"
	"github.com/leeovery/portal/internal/transienttest"
)

// The pane token is stamped once and read back after the session is renamed:
// the key a restored hook was registered under must still name a live pane, or
// the sweep reaps it the way the mutable session name used to let it.
const (
	renameRestoreToken   = "tokrst"
	renameRestoreName    = "renamedst"
	renameRestoreNewName = "renamedst2"
)

func TestRenameRestoreCleanupSurvival_KeepsRestoredTokenKeyedHook(t *testing.T) {
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

	// Restore's re-stamp: the recreated pane carries the token its saved state
	// held. Omitting it leaves the pane unstamped and the entry unprotected.
	ts.Run(t, "set-option", "-p", "-t", renameRestoreName+":0.0", state.PortalPaneIDOption, renameRestoreToken)

	if err := client.RenameSession(renameRestoreName, renameRestoreNewName); err != nil {
		t.Fatalf("RenameSession: %v", err)
	}
	ts.WaitForSession(t, renameRestoreNewName, 2*time.Second)

	assertLiveTokenPresent(t, client, renameRestoreToken)

	// Truly-stale entry with no matching live pane, token-shaped so the reaper
	// can judge it: must still be swept.
	staleKey := transienttest.ReapableHookKey(0)

	seed := `{
  "` + renameRestoreToken + `": {"on-resume": "echo restored"},
  "` + staleKey + `": {"on-resume": "echo gone"}
}`
	store, path := newTempHooksStore(t, seed)

	preRun, err := store.Load()
	if err != nil {
		t.Fatalf("pre-cleanup store.Load: %v", err)
	}
	if _, ok := preRun[renameRestoreToken]; !ok {
		t.Fatalf("pre-cleanup seed missing token key %q; keys=%v", renameRestoreToken, keysOf(preRun))
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
	if _, ok := postRun[renameRestoreToken]; !ok {
		t.Errorf("token-keyed hook %q was swept after a rename; want preserved "+
			"(the token is stamped on the pane and a rename cannot move it). "+
			"post-cleanup keys=%v (path=%s)", renameRestoreToken, keysOf(postRun), path)
	}
	if _, ok := postRun[staleKey]; ok {
		t.Errorf("truly-stale hook %q survived; want swept "+
			"(no matching live pane — cleanup correctness must not be weakened). "+
			"post-cleanup keys=%v (path=%s)", staleKey, keysOf(postRun), path)
	}
}

func assertLiveTokenPresent(t *testing.T, lister AllPaneLister, want string) {
	t.Helper()
	rows, err := lister.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}
	for _, row := range rows {
		if row.Token == want {
			return
		}
	}
	t.Fatalf("token %q not enumerated; got %+v (a stamped pane must report its token)", want, rows)
}
