//go:build integration

package restore_test

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const (
	renamePaneToken = "tok123"
	renameOldName   = "renamesrc"
	renameNewName   = "renamedst"

	// hookFiredMarker is what the fixture's hook echoes, and so what a fired
	// hook is counted by.
	hookFiredMarker = "HOOK_FIRED"
)

// renameRebootFixture is one stamped pane under its pre-rename name, with a
// resume hook already registered against its durable token, ready to be renamed
// and rebooted.
type renameRebootFixture struct {
	ts     *tmuxtest.Socket
	client *tmux.Client

	stateDir string
	binDir   string

	hooksPath    string
	hookFireFile string
}

// newRenameRebootFixture builds that pane on its own tmux socket, isolating the
// state and hooks the reboot will run against. The hook is keyed on the
// token the pane is stamped with, which is what must survive both the rename and
// the reboot.
func newRenameRebootFixture(t *testing.T, socketPrefix string) *renameRebootFixture {
	t.Helper()

	fx := &renameRebootFixture{binDir: restoretest.BuildPortalBinaryDir(t)}

	_, fx.stateDir = portaltest.IsolateStateForTest(t)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	fx.hooksPath = filepath.Join(t.TempDir(), "hooks.json")
	t.Setenv("PORTAL_HOOKS_FILE", fx.hooksPath)

	fx.hookFireFile = filepath.Join(t.TempDir(), "hook-fired.txt")
	hookCmd := "echo " + hookFiredMarker + " >> " + fx.hookFireFile
	if err := hooks.NewStore(fx.hooksPath).Set(renamePaneToken, "on-resume", hookCmd, hooks.ViaCLI); err != nil {
		t.Fatalf("hooks.Set: %v", err)
	}

	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, fx.stateDir)

	fx.ts = tmuxtest.New(t, socketPrefix)
	fx.client = fx.ts.Client()

	cwd := t.TempDir()
	fx.ts.Run(t, "new-session", "-d", "-s", renameOldName, "-c", cwd, "sleep", "infinity")
	fx.ts.WaitForSession(t, renameOldName, 2*time.Second)
	fx.ts.StampPaneToken(t, tmux.PaneTarget(renameOldName, 0, 0), renamePaneToken)

	return fx
}

// captureAndPersist saves the live topology as the index the next reboot will
// restore from, having first checked the pane token really made it into the
// capture — a capture that lost the token would restore a hookless pane and the
// reboot would prove nothing.
func (fx *renameRebootFixture) captureAndPersist(t *testing.T, name string) {
	t.Helper()

	idx, err := state.CaptureStructure(fx.client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}

	sess := restoretest.FindCapturedSession(t, idx, name)
	if got := capturedPaneToken(t, sess); got != renamePaneToken {
		t.Fatalf("captured session %q pane token = %q; want %q (token must persist under the post-rename name)",
			name, got, renamePaneToken)
	}

	fx.persist(t, idx, name)
}

// persist writes the index and the scrollback a restore of it will replay.
func (fx *renameRebootFixture) persist(t *testing.T, idx state.Index, name string) {
	t.Helper()
	restoretest.SeedScrollback(t, fx.stateDir, name, 0, 0, []byte(restoretest.ANSIScrollback))
	restoretest.WriteIndex(t, fx.stateDir, idx)
}

// rebootAndHydrate takes the fixture through one whole reboot cycle: the server
// is killed, the saved index restored onto a fresh one, and every restored pane
// driven through its hydrate helper to the point the helper hands over to the
// hook or the shell.
func (fx *renameRebootFixture) rebootAndHydrate(t *testing.T) error {
	t.Helper()

	restoretest.RebootServer(t, fx.ts, fx.client)
	if err := restoretest.RestoreFromState(t, fx.client, fx.stateDir, fx.binDir); err != nil {
		return err
	}

	restoredPanes := restoretest.LivePaneCoords(t, fx.ts, renameNewName)
	if !slices.Contains(restoredPanes, "0:0") {
		t.Fatalf("restored session %q missing live pane 0:0; got %q", renameNewName, restoredPanes)
	}

	restoretest.DriveSignalHydrate(t, fx.client, fx.stateDir, []string{renameNewName})
	restoretest.WaitForSkeletonMarkersCleared(t, fx.client, restoretest.HydrateBudget, restoretest.HydrateTick)
	return nil
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
