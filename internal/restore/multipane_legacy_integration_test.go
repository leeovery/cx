//go:build integration

package restore_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const legacyName = "legacy-proj"

// legacyPane is one pane of the arranged session, described by the two things
// the fixtures in this file differ in: the durable token it is stamped with, and
// the resume hook registered against that token. Both are optional — an empty
// token leaves the pane un-stamped, which is the legacy shape a restore must
// still land on a bare shell.
type legacyPane struct {
	token   string
	hookCmd string
}

// legacyFixture is a live single-window session of len(panes) panes, with the
// state, hooks and tmux socket the reboot will run against already isolated.
type legacyFixture struct {
	ts     *tmuxtest.Socket
	client *tmux.Client

	stateDir string
	binDir   string

	hooksPath string
	panes     []legacyPane
}

// newLegacyFixture builds the live session both multipane fixtures arrange
// against. What they vary — how many panes, and which of those carry a token
// and a hook — arrives as parameters.
func newLegacyFixture(t *testing.T, socketPrefix, sessionName string, panes []legacyPane) *legacyFixture {
	t.Helper()

	if len(panes) == 0 {
		t.Fatal("newLegacyFixture: no panes; a session has at least the one it is created with")
	}

	fx := &legacyFixture{
		binDir: restoretest.BuildPortalBinaryDir(t),
		panes:  panes,
	}

	portaltest.IsolateStateForTest(t)

	fx.stateDir = t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", fx.stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}

	fx.hooksPath = filepath.Join(t.TempDir(), "hooks.json")
	t.Setenv("PORTAL_HOOKS_FILE", fx.hooksPath)

	store := hooks.NewStore(fx.hooksPath)
	for i, p := range panes {
		if p.hookCmd == "" {
			continue
		}
		if err := store.Set(p.token, "on-resume", p.hookCmd, hooks.ViaCLI); err != nil {
			t.Fatalf("hooks.Set pane %d: %v", i, err)
		}
		verifyHookKeyed(t, fx.hooksPath, p.token)
	}

	// LIFO runs this wait between kill-server and the TempDir RemoveAll.
	portaltest.RegisterStateDirTeardownGuard(t, fx.stateDir)

	fx.ts = tmuxtest.New(t, socketPrefix)
	fx.client = fx.ts.Client()

	cwd := t.TempDir()
	fx.ts.Run(t, "new-session", "-d", "-s", sessionName, "-c", cwd, "sleep", "infinity")
	fx.ts.WaitForSession(t, sessionName, 2*time.Second)
	for range panes[1:] {
		fx.ts.Run(t, "split-window", "-t", tmux.PaneTarget(sessionName, 0, 0), "-c", cwd, "sleep", "infinity")
	}
	fx.assertLivePanes(t, sessionName)

	for i, p := range panes {
		if p.token == "" {
			continue
		}
		fx.ts.StampPaneToken(t, tmux.PaneTarget(sessionName, 0, i), p.token)
	}

	return fx
}

// assertLivePanes pins the pane coordinates the arrange produced, so a fixture
// that silently built the wrong topology fails here rather than as a puzzling
// assertion after the reboot.
func (fx *legacyFixture) assertLivePanes(t *testing.T, sessionName string) {
	t.Helper()
	want := make([]string, 0, len(fx.panes))
	for i := range fx.panes {
		want = append(want, "0:"+strconv.Itoa(i))
	}
	got := strings.Fields(fx.ts.Run(t, "list-panes", "-s", "-t", tmux.ExactCoordTarget(sessionName),
		"-F", "#{window_index}:#{pane_index}"))
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("session %q live panes = %v; want %v", sessionName, got, want)
	}
}

// captureAndPersist saves the live topology as the index the reboot restores
// from, having first checked each pane's token survived into the capture: a
// capture that lost one would restore a hookless pane and prove nothing.
func (fx *legacyFixture) captureAndPersist(t *testing.T, sessionName string) {
	t.Helper()

	idx, err := state.CaptureStructure(fx.client, nil, nil, nil)
	if err != nil {
		t.Fatalf("CaptureStructure: %v", err)
	}
	sess := restoretest.FindCapturedSession(t, idx, sessionName)
	if len(sess.Windows) != 1 || len(sess.Windows[0].Panes) != len(fx.panes) {
		t.Fatalf("captured session %q topology = %d window(s) / %v panes; want 1 window / %d panes",
			sessionName, len(sess.Windows), paneIndices(sess), len(fx.panes))
	}
	for i, p := range fx.panes {
		if got := sess.Windows[0].Panes[i].PortalPaneID; got != p.token {
			t.Fatalf("captured session %q pane %d token = %q; want %q", sessionName, i, got, p.token)
		}
	}

	for i := range fx.panes {
		restoretest.SeedScrollback(t, fx.stateDir, sessionName, 0, i, []byte(restoretest.ANSIScrollback))
	}
	restoretest.WriteIndex(t, fx.stateDir, idx)
}

// rebootAndHydrate opens the reboot gap, restores the saved index onto the fresh
// server, and drives every restored pane through its hydrate helper to the point
// the helper hands over to the hook or the shell.
func (fx *legacyFixture) rebootAndHydrate(t *testing.T, sessionName string) {
	t.Helper()

	restoretest.RebootServer(t, fx.ts, fx.client)
	if err := restoretest.RestoreFromState(t, fx.client, fx.stateDir, fx.binDir); err != nil {
		t.Fatalf("RestoreFromState: %v", err)
	}
	fx.assertLivePanes(t, sessionName)

	restoretest.DriveSignalHydrate(t, fx.client, fx.stateDir, []string{sessionName})
	restoretest.WaitForSkeletonMarkersCleared(t, fx.client, restoretest.HydrateBudget, restoretest.HydrateTick)
}

func TestMultiPaneLegacy_PerPaneHookRouting(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	const (
		pane0Marker = "PANE0_HOOK_FIRED"
		pane1Marker = "PANE1_HOOK_FIRED"
	)
	sideEffectDir := t.TempDir()
	pane0File := filepath.Join(sideEffectDir, "hook-pane0.txt")
	pane1File := filepath.Join(sideEffectDir, "hook-pane1.txt")

	fx := newLegacyFixture(t, "ptl-3-7-mp-", renameOldName, []legacyPane{
		{token: "mpPaneToken0", hookCmd: "echo " + pane0Marker + " >> " + pane0File},
		{token: "mpPaneToken1", hookCmd: "echo " + pane1Marker + " >> " + pane1File},
	})

	fx.ts.Run(t, "rename-session", "-t", tmux.ExactSessionTarget(renameOldName), renameNewName)
	if _, err := fx.ts.TryRun("has-session", "-t", tmux.ExactSessionTarget(renameNewName)); err != nil {
		t.Fatalf("session %q not live after rename: %v", renameNewName, err)
	}

	fx.captureAndPersist(t, renameNewName)
	for _, p := range fx.panes {
		verifyHookKeyed(t, fx.hooksPath, p.token)
	}

	fx.rebootAndHydrate(t, renameNewName)

	// Each pane's own hook fired exactly once, and neither fired in the other's
	// pane: the token, not the coordinates, is what routed it.
	restoretest.AssertMarkerCount(t, pane0File, pane0Marker, 1)
	restoretest.AssertMarkerCount(t, pane1File, pane1Marker, 1)
	restoretest.AssertMarkerCount(t, pane0File, pane1Marker, 0)
	restoretest.AssertMarkerCount(t, pane1File, pane0Marker, 0)
}

func TestMultiPaneLegacy_UnstampedNoHookLandsOnBareShell(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test; -short")
	}
	tmuxtest.SkipIfNoTmux(t)

	fx := newLegacyFixture(t, "ptl-3-7-bare-", legacyName, []legacyPane{{}})

	fx.captureAndPersist(t, legacyName)

	fx.rebootAndHydrate(t, legacyName)

	if _, err := fx.ts.TryRun("has-session", "-t", tmux.ExactSessionTarget(legacyName)); err != nil {
		t.Fatalf("un-stamped no-hook session %q not restored: %v", legacyName, err)
	}
}

func paneIndices(sess state.Session) [][]int {
	var out [][]int
	for _, w := range sess.Windows {
		var panes []int
		for _, p := range w.Panes {
			panes = append(panes, p.Index)
		}
		out = append(out, panes)
	}
	return out
}
