package cmd

import (
	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/tmux"
)

// The abridged warm path's saver revive. A failure is soft — the command still
// proceeds. It must never run a kill-barrier or the version gate: a satisfied
// @portal-bootstrapped latch already proves the running daemon is the current
// binary, so the re-check would only add a concurrency hazard under a reopen
// burst, and the version gate belongs to the full-bootstrap step.
func ensureSaverLiveness(client *tmux.Client, stateDir string) {
	// present && err == nil is the only "alive" shape: an absent saver and a
	// transient probe error both fold into "needs revive".
	if _, present, err := tmux.SaverPanePIDOrAbsent(client, tmux.PortalSaverName); present && err == nil {
		return
	}

	if err := tmux.BootstrapPortalSaver(client, stateDir); err != nil {
		bootstrapLogger.Warn("abridged EnsureSaver: saver revive failed", "error", err)
		bootstrapWarnings.Add(bootstrap.SaverDownWarning())
	}
}
