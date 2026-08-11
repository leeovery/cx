package cmd

import (
	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/tmux"
)

// ensureSaverLiveness re-probes the _portal-saver session on the abridged warm
// path and revives it when absent. A revive failure is soft: it logs the cause,
// funnels a SaverDownWarning into the shared sink and lets the command proceed.
//
// It must never run a kill-barrier or the version gate. A satisfied
// @portal-bootstrapped latch already proves the running daemon is the current
// binary, so the re-check would only add a concurrency hazard under a reopen
// burst; the version gate belongs to the full-bootstrap step alone.
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
