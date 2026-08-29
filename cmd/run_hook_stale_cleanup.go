package cmd

import (
	"context"
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

// stalenessView is what the guards could establish about this moment: the live
// token set persisted keys may be judged against, or the reason nothing may be
// judged at all. A declined view's LiveTokens is not authority.
type stalenessView struct {
	Persisted  map[string]map[string]string
	LiveTokens []string
	// PaneRows is meaningful only where Enumerated is set: an empty read is a
	// completed read, an unreached or failed one has no count to report.
	PaneRows   int
	Enumerated bool
	Decline    standDown
}

// evaluateHookStaleness runs the guard ladder in the one order the sweep and
// the diagnostic agree on, so neither can judge an entry the other protects.
//
// loadPersisted is called only after the restore gate, not before it: a caller
// whose store read must not happen inside a restore window hands over the read
// itself rather than its result, and one that has already read hands back what
// it holds.
func evaluateHookStaleness(reader staleSweepReader, loadPersisted func() (map[string]map[string]string, error)) (stalenessView, error) {
	if active, err := restoreWindowActive(reader); active {
		return stalenessView{Decline: declineDebug(skipReasonRestoring, errAttr(err)...)}, nil
	}

	persisted, err := loadPersisted()
	if err != nil {
		// The one decline a caller must also see as a failure: nothing but
		// repair clears an unreadable store, so the daemon reports it while the
		// reason still rides the view for a caller that only renders.
		return stalenessView{Decline: declineWarn(skipReasonStoreReadFailed, "error", err)}, err
	}

	rows, err := reader.ListAllPaneHookKeys()
	if err != nil {
		return stalenessView{Persisted: persisted, Decline: declineWarn(skipReasonPaneReadFailed, "error", err)}, nil
	}

	view := stalenessView{Persisted: persisted, PaneRows: len(rows), Enumerated: true}

	// A pane-less read is a bad read, not authority: it must never reach the
	// staleness rule, which would judge every key it can parse stale. Defer to
	// the next run. The count is of rows — under lazy stamping a live server
	// carrying no token at all is ordinary, not a failure — and with nothing
	// persisted there is nothing to protect, so the guard has no work.
	if len(rows) == 0 && len(persisted) > 0 {
		view.Decline = declineWarn(skipReasonEmptyPaneRead, "entries", len(persisted))
		return view, nil
	}

	view.LiveTokens = liveTokensFrom(rows)
	return view, nil
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

func runHookStaleCleanup(reader staleSweepReader, store *hooks.Store, logger *slog.Logger) (sweepOutcome, error) {
	if logger == nil {
		logger = bootstrapLogger
	}

	view, err := evaluateHookStaleness(reader, func() (map[string]map[string]string, error) {
		// The advisory pre-read's shared hold is released when it returns, so
		// the enumeration after it runs with no lock held. Order is
		// load-bearing: the enumeration is older than any mutation that lands
		// after it, so a registration written in that window is absent from
		// this snapshot and CleanStale retains it rather than reaping it on
		// shape alone. The read takes the short bound because this cycle takes
		// the sidecar again inside CleanStale, and at the full bound a wedged
		// writer would park the daemon's tick for two of them.
		return store.LoadSnapshot("internal")
	})

	if view.Enumerated {
		logger.Debug("stale-hook cleanup counts", "panes", view.PaneRows, "entries", len(view.Persisted))
	}
	if view.Decline.declined() {
		view.Decline.emit()
		return sweepOutcome{DeclineReason: view.Decline.reason}, err
	}

	// Nothing persisted is nothing to sweep, and reaching CleanStale is not
	// free: it creates the config directory, creates the sidecar and takes an
	// exclusive hold. An install that has never registered a hook would
	// otherwise pay all three on every cycle, in every `hook set`'s way.
	snapshot := slices.Collect(maps.Keys(view.Persisted))
	if len(snapshot) == 0 {
		return sweepOutcome{}, nil
	}

	removed, err := store.CleanStale(view.LiveTokens, snapshot)
	if err != nil {
		// Another writer held the sidecar past the bound: nothing was written
		// and nothing is wrong, so the cycle stands down rather than reporting a
		// defect, and the next cadence retries. The nil error keeps the caller
		// from adding a second report for the same event. Every other failure
		// stays a failure.
		if errors.Is(err, hooks.ErrLockHeld) {
			decline := declineWarn(skipReasonLockTimeout, "error", err)
			decline.emit()
			return sweepOutcome{DeclineReason: decline.reason}, nil
		}
		return sweepOutcome{}, err
	}
	logger.Debug("stale-hook cleanup removed", "reaped", len(removed))

	return sweepOutcome{Removed: removed}, nil
}
