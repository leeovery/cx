package cmd

import (
	"log/slog"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/state"
)

// The reasons a cycle declines to run. Both the logged reason attr and the
// caller-facing rendering read them, so the two cannot drift.
const (
	skipReasonRestoring     = "restoring"
	skipReasonEmptyPaneRead = "empty-pane-read"
)

// standDownMsg and standDownAttrs give every stand-down one line shape, so a
// single grep answers whether the prune declined and why. The level is the call
// site's: a restore window is an expected state and warning through every one
// of them would name a hazard being avoided rather than encountered, while an
// empty pane read is an anomaly.
const standDownMsg = "clean-stale-skipped"

func standDownAttrs(reason string, extra ...any) []any {
	return append([]any{"op", standDownMsg, "via", "internal", "reason", reason}, extra...)
}

// AllPaneLister returns every live pane's hook key, in the same
// <@portal-id or session_name>:window.pane form registration writes — a divergent
// form reaps freshly-registered entries as stale. It also carries the
// @portal-restoring read that stands the sweep down for a restore's duration.
type AllPaneLister interface {
	ListAllPaneHookKeys() ([]string, error)
	state.RestoringChecker
}

// restoreWindowActive reports whether hook-staleness work must stand down. A
// restore's panes carry no token until the re-stamp, so work landing in that
// window would treat every token-keyed entry on the machine as lost. A failed
// read counts as set: a deferred prune costs nothing, and on a machine with no
// tmux server the read fails — hence "may be" in what a caller renders. The
// read error is returned alongside for a caller that reports why, and is
// already folded into the bool.
func restoreWindowActive(checker state.RestoringChecker) (bool, error) {
	restoring, err := state.IsRestoringSet(checker)
	return restoring || err != nil, err
}

func runHookStaleCleanup(
	lister AllPaneLister,
	store *hooks.Store,
	logger *slog.Logger,
	onRemoved func(string),
	onSkipped func(reason string),
) error {
	if logger == nil {
		logger = bootstrapLogger
	}
	reportSkip := func(reason string) {
		if onSkipped != nil {
			onSkipped(reason)
		}
	}

	if active, err := restoreWindowActive(lister); active {
		var extra []any
		if err != nil {
			extra = append(extra, "error", err)
		}
		hooksLogger.Debug(standDownMsg, standDownAttrs(skipReasonRestoring, extra...)...)
		reportSkip(skipReasonRestoring)
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
		hooksLogger.Warn(standDownMsg, standDownAttrs(skipReasonEmptyPaneRead, "entries", len(persisted))...)
		reportSkip(skipReasonEmptyPaneRead)
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
