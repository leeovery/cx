package cmd

import (
	"errors"
	"log/slog"
	"maps"
	"slices"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// The reasons a cycle declines to run. Both the logged reason attr and the
// caller-facing rendering read them, so the two cannot drift.
const (
	skipReasonRestoring     = "restoring"
	skipReasonEmptyPaneRead = "empty-pane-read"
	skipReasonLockTimeout   = "lock-timeout"
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

// AllPaneLister returns one row per live pane: the pane's hook token, empty
// for an unstamped pane, alongside its display-only location. The row count
// answers whether the tmux read succeeded and the non-empty tokens answer which
// panes are protected — two questions no consumer may conflate. It also carries
// the @portal-restoring read that stands the sweep down for a restore's
// duration.
type AllPaneLister interface {
	ListAllPaneHookKeys() ([]tmux.PaneHookRow, error)
	state.RestoringChecker
}

// liveTokensFrom projects the enumeration onto the staleness rule's vocabulary:
// an unstamped pane protects no key, so its empty token must never enter the
// live set.
func liveTokensFrom(rows []tmux.PaneHookRow) []string {
	tokens := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Token != "" {
			tokens = append(tokens, row.Token)
		}
	}
	return tokens
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

	// The advisory pre-read comes first, and its shared hold is released when it
	// returns, so the enumeration below runs with no lock held. Order is
	// load-bearing: the enumeration is older than any mutation that lands after
	// it, so a registration written in that window is absent from this snapshot
	// and CleanStale retains it rather than reaping it on shape alone. The read
	// takes the short bound because this cycle takes the sidecar again inside
	// CleanStale, and at the full bound a wedged writer would park the daemon's
	// tick for two of them.
	persisted, err := store.LoadSnapshot("internal")
	if err != nil {
		logger.Warn("stale-hook cleanup: hookStore.Load failed", "error", err)
		return err
	}

	rows, err := lister.ListAllPaneHookKeys()
	if err != nil {
		logger.Warn("stale-hook cleanup: list-panes failed", "error", err)
		return nil
	}

	logger.Debug("stale-hook cleanup counts", "panes", len(rows), "entries", len(persisted))

	// A pane-less read is a bad read, not authority: it must never reach
	// CleanStale, which would delete every key the staleness rule can judge.
	// Defer to the next run. The count is of rows — under lazy stamping a live
	// server carrying no token at all is ordinary, not a failure.
	if len(rows) == 0 {
		if len(persisted) == 0 {
			return nil
		}
		hooksLogger.Warn(standDownMsg, standDownAttrs(skipReasonEmptyPaneRead, "entries", len(persisted))...)
		reportSkip(skipReasonEmptyPaneRead)
		return nil
	}

	tokens := liveTokensFrom(rows)

	// Nothing persisted is nothing to sweep, and reaching CleanStale is not
	// free: it creates the config directory, creates the sidecar and takes an
	// exclusive hold. An install that has never registered a hook would
	// otherwise pay all three on every cycle, in every `hook set`'s way.
	snapshot := slices.Collect(maps.Keys(persisted))
	if len(snapshot) == 0 {
		return nil
	}

	removed, err := store.CleanStale(tokens, snapshot)
	if err != nil {
		// Another writer held the sidecar past the bound: nothing was written
		// and nothing is wrong, so the cycle stands down on the shared line
		// rather than reporting a defect, and the next cadence retries. The
		// level is WARN because a lock that will not yield is an anomaly, and
		// the nil return keeps the caller from adding a second line for the
		// same event. Every other failure stays a failure.
		if errors.Is(err, hooks.ErrLockHeld) {
			hooksLogger.Warn(standDownMsg, standDownAttrs(skipReasonLockTimeout, "error", err)...)
			reportSkip(skipReasonLockTimeout)
			return nil
		}
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
