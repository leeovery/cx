//go:build integration

package bootstrap_test

import (
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/bootstrapadapter"
	"github.com/leeovery/portal/internal/portaltest"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// restoreAdapterFor points the restore's pane-arming at the binary staged in
// binDir. Production resolves it from os.Executable, which under `go test` is
// the test binary — which would re-run its own suite inside the pane and exit,
// taking the restored session with it.
func restoreAdapterFor(t *testing.T, client *tmux.Client, stateDir string, logger *slog.Logger, binDir string) *bootstrapadapter.RestoreAdapter {
	t.Helper()
	a := bootstrapadapter.NewRestoreAdapter(client, stateDir, logger)
	a.Inner.Exe = restoretest.StagedHydrateExe(t, binDir)
	return a
}

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
