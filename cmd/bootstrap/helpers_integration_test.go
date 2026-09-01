//go:build integration

package bootstrap_test

import (
	"testing"

	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/state"
)

// newIntegrationStateDir stages an isolated state directory and the env slice a
// subprocess needs to share it. It registers the teardown guard itself, so a
// caller must call it before starting a tmux server: LIFO then runs that wait
// after kill-server and before the TempDir RemoveAll it protects. It also
// re-points HOME at a temp dir, so build any binary the test needs first — a
// `go build` afterwards populates an unremovable module cache there.
func newIntegrationStateDir(t *testing.T) (env []string, stateDir string) {
	t.Helper()
	env, stateDir = portaltest.IsolateStateForTest(t)
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	portaltest.RegisterStateDirTeardownGuard(t, stateDir)
	return env, stateDir
}
