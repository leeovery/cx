//go:build integration

package bootstrap_test

import (
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/bootstrapadapter"
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

func newIntegrationStateDir(t *testing.T) string {
	t.Helper()
	stateDir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", stateDir)
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	return stateDir
}
