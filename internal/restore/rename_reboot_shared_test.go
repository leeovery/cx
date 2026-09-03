//go:build integration

package restore_test

import (
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/state"
)

const (
	renamePaneToken = "tok123"
	renameOldName   = "renamesrc"
	renameNewName   = "renamedst"

	// hookFiredMarker is what the fixture's hook echoes, and so what a fired
	// hook is counted by.
	hookFiredMarker = "HOOK_FIRED"
)

// renameRebootPane describes the single stamped pane the rename-reboot suites
// arrange — one resume hook registered against the durable token that must
// survive both the rename and the reboot — alongside the file that hook appends
// its marker to, which is how a fired hook is counted.
func renameRebootPane(t *testing.T) (rebootPane, string) {
	t.Helper()
	fireFile := filepath.Join(t.TempDir(), "hook-fired.txt")
	return rebootPane{
		token:   renamePaneToken,
		hookCmd: "echo " + hookFiredMarker + " >> " + fireFile,
	}, fireFile
}

func capturedPaneToken(t *testing.T, sess state.Session) string {
	t.Helper()
	if len(sess.Windows) == 0 || len(sess.Windows[0].Panes) == 0 {
		t.Fatalf("captured session %q has no pane 0.0: %+v", sess.Name, sess.Windows)
	}
	return sess.Windows[0].Panes[0].PortalPaneID
}

func verifyHookKeyed(t *testing.T, hooksPath, wantKey string) {
	t.Helper()
	persisted, err := hooks.NewStore(hooksPath).Load(hooks.ViaInternal)
	if err != nil {
		t.Fatalf("hooks.Load: %v", err)
	}
	events := persisted[wantKey]
	if _, ok := events["on-resume"]; !ok {
		t.Fatalf("hooks.json missing on-resume entry under stable key %q; got events=%v", wantKey, events)
	}
}
