package cmd

import (
	"log/slog"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/state"
)

// A restore window is an expected state, so the stand-down is DEBUG: warning
// through every one of them would name a hazard being avoided, not encountered.
func logRestoreStandDown(readErr error) {
	attrs := []any{"op", "clean-stale-skipped", "via", "internal", "reason", "restoring"}
	if readErr != nil {
		attrs = append(attrs, "error", readErr)
	}
	hooksLogger.Debug("clean-stale-skipped", attrs...)
}

// AllPaneLister returns every live pane's hook key, in the same
// <@portal-id or session_name>:window.pane form registration writes — a divergent
// form reaps freshly-registered entries as stale. It also carries the
// @portal-restoring read that stands the sweep down for a restore's duration.
type AllPaneLister interface {
	ListAllPaneHookKeys() ([]string, error)
	state.RestoringChecker
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

	// A restore's panes carry no token until the re-stamp, so a sweep landing in
	// that window would reap every token-keyed entry on the machine. A failed
	// read counts as set: a deferred prune costs nothing.
	if restoring, err := state.IsRestoringSet(lister); restoring || err != nil {
		logRestoreStandDown(err)
		return nil
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
