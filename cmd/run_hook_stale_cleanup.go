package cmd

import (
	"log/slog"

	"github.com/leeovery/portal/internal/hooks"
)

// AllPaneLister returns every live pane's hook key, in the same
// <@portal-id or session_name>:window.pane form registration writes — a divergent
// form reaps freshly-registered entries as stale.
type AllPaneLister interface {
	ListAllPaneHookKeys() ([]string, error)
}

func runHookStaleCleanup(
	lister AllPaneLister,
	store *hooks.Store,
	logger *slog.Logger,
	onRemoved func(string),
) error {
	if logger == nil {
		logger = bootstrapLogger
	}

	livePanes, err := lister.ListAllPaneHookKeys()
	if err != nil {
		logger.Warn("stale-hook cleanup: list-panes failed", "error", err)
		return nil
	}

	persisted, err := store.Load()
	if err != nil {
		logger.Warn("stale-hook cleanup: hookStore.Load failed", "error", err)
		return err
	}

	logger.Debug("stale-hook cleanup counts", "panes", len(livePanes), "entries", len(persisted))

	// An empty live set is a bad read, not authority: it must never reach
	// CleanStale, which would delete every entry. Defer to the next run.
	if len(livePanes) == 0 {
		if len(persisted) == 0 {
			return nil
		}
		logger.Warn("stale-hook cleanup: zero live panes parsed with hooks present; skipping to avoid mass-deletion hazard (next bootstrap retries)", "entries", len(persisted))
		return nil
	}

	removed, err := store.CleanStale(livePanes)
	if err != nil {
		return err
	}
	logger.Debug("stale-hook cleanup removed", "reaped", len(removed))

	if onRemoved != nil {
		for _, name := range removed {
			onRemoved(name)
		}
	}

	return nil
}
