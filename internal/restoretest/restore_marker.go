package restoretest

import (
	"testing"

	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// RestoreWithMarker brackets a restore with @portal-restoring exactly as
// bootstrap does, so the daemon's capture stand-down window opens and closes
// around it. The unset is deferred and its failure only logged: it is fixture
// teardown, not the behaviour under test, and reporting it as the restore's own
// error would mask the returned one.
//
// Untagged on purpose: internal/restore's untagged integration_test.go calls it,
// so the unit lane has to compile it.
func RestoreWithMarker(t *testing.T, client *tmux.Client, o *restore.Orchestrator) error {
	t.Helper()
	if err := client.SetServerOption(state.RestoringMarkerName, "1"); err != nil {
		return err
	}
	defer func() {
		if err := client.UnsetServerOption(state.RestoringMarkerName); err != nil {
			t.Logf("UnsetServerOption(%s): %v", state.RestoringMarkerName, err)
		}
	}()
	assertRestoringSet(t, client)
	_, err := o.Restore()
	return err
}

// assertRestoringSet reads the marker back from the server before the restore
// runs, so the bracket's set half is observed for every caller at once: without
// it a restore driven with no marker set would be indistinguishable from one
// driven with it, and the daemon stand-down window this fixture exists to open
// would never be exercised.
func assertRestoringSet(t harnesstest.NamingT, client *tmux.Client) {
	t.Helper()
	set, err := state.IsRestoringSet(client)
	if err != nil {
		t.Fatalf("IsRestoringSet: %v", err)
	}
	if !set {
		t.Fatalf("%s is not set entering the restore; the capture stand-down window never opened",
			state.RestoringMarkerName)
	}
}
