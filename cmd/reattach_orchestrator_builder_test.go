//go:build integration

package cmd

// Integration-tagged because buildReattachOrchestrator itself lives in an
// integration-tagged file; tmux is never invoked here.

import (
	"testing"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/tmux"
)

func TestBuildReattachOrchestrator_EagerSignalerDefaultsToReal(t *testing.T) {
	// The builder resolves its own stateDir through state.Dir(), which reads
	// PORTAL_STATE_DIR.
	t.Setenv("PORTAL_STATE_DIR", t.TempDir())

	stateDir := t.TempDir()
	client := &tmux.Client{}

	o := buildReattachOrchestrator(t, client, stateDir)

	if _, ok := o.EagerSignaler.(*bootstrap.EagerSignalCore); !ok {
		t.Errorf("EagerSignaler type = %T; want *bootstrap.EagerSignalCore (real adapter — buildReattachOrchestrator always wires a real RestoreAdapter)", o.EagerSignaler)
	}
}
