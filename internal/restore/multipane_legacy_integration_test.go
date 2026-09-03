//go:build integration

package restore_test

import (
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const legacyName = "legacy-proj"

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

	fx := newRebootFixture(t, "ptl-3-7-mp-", renameOldName, []rebootPane{
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

	if err := fx.rebootAndHydrate(t, renameNewName); err != nil {
		t.Fatalf("RestoreFromState: %v", err)
	}

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

	fx := newRebootFixture(t, "ptl-3-7-bare-", legacyName, []rebootPane{{}})

	fx.captureAndPersist(t, legacyName)

	if err := fx.rebootAndHydrate(t, legacyName); err != nil {
		t.Fatalf("RestoreFromState: %v", err)
	}

	if _, err := fx.ts.TryRun("has-session", "-t", tmux.ExactSessionTarget(legacyName)); err != nil {
		t.Fatalf("un-stamped no-hook session %q not restored: %v", legacyName, err)
	}
}
