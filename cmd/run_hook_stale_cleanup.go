package cmd

import (
	"context"
	"errors"
	"log/slog"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// The reasons a cycle declines to run. Both the logged reason attr and the
// caller-facing rendering read them, so the two cannot drift.
const (
	skipReasonRestoring       = "restoring"
	skipReasonStoreReadFailed = "store-read-failed"
	skipReasonPaneReadFailed  = "pane-read-failed"
	skipReasonEmptyPaneRead   = "empty-pane-read"
	skipReasonLockTimeout     = "lock-timeout"
)

// standDownMsg and standDownAttrs give every stand-down one line shape, so a
// single grep answers whether the prune declined and why.
const standDownMsg = "clean-stale-skipped"

func standDownAttrs(reason string, extra ...any) []any {
	return append([]any{"op", standDownMsg, "via", "internal", "reason", reason}, extra...)
}

// staleSweepReader is the whole tmux surface hook-staleness work reads: the
// live pane enumeration keys are judged against, and the @portal-restoring
// marker that stands that judgement down for a restore's duration.
type staleSweepReader interface {
	PaneHookLister
	state.RestoringChecker
}

// standDown is a declined cycle: the reason its caller reports alongside the
// level and attrs its log line carries. The zero value means the cycle ran.
// Emission is the sweep's — a diagnosis that declines has pruned nothing and
// must not claim a stand-down in the reaper's own vocabulary.
type standDown struct {
	reason string
	level  slog.Level
	attrs  []any
}

func (s standDown) declined() bool { return s.reason != "" }

func (s standDown) emit() {
	hooksLogger.Log(context.Background(), s.level, standDownMsg, standDownAttrs(s.reason, s.attrs...)...)
}

// A restore window is an expected state, and warning through every one of them
// would name a hazard being avoided rather than encountered.
func declineDebug(reason string, attrs ...any) standDown {
	return standDown{reason: reason, level: slog.LevelDebug, attrs: attrs}
}

// A lock that will not yield and a tmux read that answered nothing usable are
// both anomalies.
func declineWarn(reason string, attrs ...any) standDown {
	return standDown{reason: reason, level: slog.LevelWarn, attrs: attrs}
}

func errAttr(err error) []any {
	if err == nil {
		return nil
	}
	return []any{"error", err}
}

// stalenessView is what the enumeration could establish about this moment: the
// live token set persisted keys may be judged against, or the reason nothing
// may be judged at all. A declined view's LiveTokens is not authority.
type stalenessView struct {
	LiveTokens []string
	// PaneRows is meaningful only where Enumerated is set: an empty read is a
	// completed read, an unreached or failed one has no count to report.
	PaneRows   int
	Enumerated bool
	Decline    standDown
}

// hookStalenessStandDown reports whether hook-staleness work may run at all.
// Both the sweep and the diagnostic take it before they read their store, so
// neither judges an entry the other protects.
func hookStalenessStandDown(reader state.RestoringChecker) standDown {
	if active, err := restoreWindowActive(reader); active {
		return declineDebug(skipReasonRestoring, errAttr(err)...)
	}
	return standDown{}
}

// judgeAgainstLivePanes enumerates the live pane tokens persisted keys are
// judged against. entries is how many keys the file holds, which is the whole
// of what the guard below needs from it.
func judgeAgainstLivePanes(reader PaneHookLister, entries int) stalenessView {
	rows, err := reader.ListAllPaneHookKeys()
	if err != nil {
		return stalenessView{Decline: declineWarn(skipReasonPaneReadFailed, "error", err)}
	}

	view := stalenessView{PaneRows: len(rows), Enumerated: true}

	// A pane-less read is a bad read, not authority: it must never reach the
	// staleness rule, which would judge every key it can parse stale. Defer to
	// the next run. The count is of rows — under lazy stamping a live server
	// carrying no token at all is ordinary, not a failure — and with nothing
	// persisted there is nothing to protect, so the guard has no work.
	if len(rows) == 0 && entries > 0 {
		view.Decline = declineWarn(skipReasonEmptyPaneRead, "entries", entries)
		return view
	}

	view.LiveTokens = liveTokensFrom(rows)
	return view
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
// tmux server the read fails. The read error is returned alongside for a caller
// that reports why, and is already folded into the bool.
func restoreWindowActive(checker state.RestoringChecker) (bool, error) {
	restoring, err := state.IsRestoringSet(checker)
	return restoring || err != nil, err
}

// sweepOutcome is everything one cycle has to say: the keys it removed, or the
// reason it declined to remove any. A caller renders both halves from it, so a
// cycle that declined can never look like one that found nothing.
type sweepOutcome struct {
	Removed       []string
	DeclineReason string
}

// The answers an enumeration gives that are not failures: a cycle that declined
// on a guard, and a store holding nothing to sweep. Both abort the clean with
// the file untouched, which is the point of returning them as errors.
var (
	errCycleDeclined    = errors.New("hook staleness cycle declined")
	errNothingPersisted = errors.New("no hook entries to sweep")
)

func runHookStaleCleanup(reader staleSweepReader, store *hooks.Store, logger *slog.Logger) (sweepOutcome, error) {
	if logger == nil {
		logger = bootstrapLogger
	}

	// Taken before the store is read at all: a restore window is no time to
	// wait on this file's lock for an answer that cannot be acted on.
	if decline := hookStalenessStandDown(reader); decline.declined() {
		decline.emit()
		return sweepOutcome{DeclineReason: decline.reason}, nil
	}

	var decline standDown
	removed, err := store.CleanStale(func(snapshot hooks.Snapshot) ([]string, error) {
		view := judgeAgainstLivePanes(reader, len(snapshot))
		if view.Enumerated {
			logger.Debug("stale-hook cleanup counts", "panes", view.PaneRows, "entries", len(snapshot))
		}
		if view.Decline.declined() {
			decline = view.Decline
			return nil, errCycleDeclined
		}
		// Nothing persisted is nothing to sweep, and the deletion that would
		// follow is not free: it creates the config directory, creates the
		// sidecar and takes an exclusive hold. An install that has never
		// registered a hook would otherwise pay all three on every cycle, in
		// every `hook set`'s way.
		if len(snapshot) == 0 {
			return nil, errNothingPersisted
		}
		return view.LiveTokens, nil
	})
	if err != nil {
		return declinedSweep(decline, err)
	}

	logger.Debug("stale-hook cleanup removed", "reaped", len(removed))

	return sweepOutcome{Removed: removed}, nil
}

// declinedSweep renders the outcome of a clean that wrote nothing. A guard's
// own stand-down carries the reason it decided on; the store's own failure
// modes are named here because only this cycle knows what they cost it.
func declinedSweep(decline standDown, err error) (sweepOutcome, error) {
	switch {
	case errors.Is(err, errNothingPersisted):
		return sweepOutcome{}, nil
	case errors.Is(err, errCycleDeclined):
		decline.emit()
		return sweepOutcome{DeclineReason: decline.reason}, nil
	case errors.Is(err, hooks.ErrLockHeld):
		// Another writer held the sidecar past the bound: nothing was written
		// and nothing is wrong, so the cycle stands down rather than reporting a
		// defect, and the next cadence retries. The nil error keeps the caller
		// from adding a second report for the same event.
		lockDecline := declineWarn(skipReasonLockTimeout, "error", err)
		lockDecline.emit()
		return sweepOutcome{DeclineReason: lockDecline.reason}, nil
	case errors.Is(err, hooks.ErrSnapshotRead):
		// The one decline a caller must also see as a failure: nothing but
		// repair clears an unreadable store, so the daemon reports it while the
		// reason still rides the outcome for a caller that only renders.
		readDecline := declineWarn(skipReasonStoreReadFailed, "error", err)
		readDecline.emit()
		return sweepOutcome{DeclineReason: readDecline.reason}, err
	}
	return sweepOutcome{}, err
}
