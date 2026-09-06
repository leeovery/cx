package hooksweep

import (
	"errors"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// The whole cycle rides the component of the subsystem it sweeps, whichever
// caller drove it, so one grep reconstructs the whole of it.
var logger = log.For("hooks")

// The messages the cycle reports itself under. countsMsg and removedMsg are its
// per-cycle detail; sweepFailedMsg reports a cycle that neither ran to
// completion nor stood down for a reason it could name, so no caller has to
// word the sweep's internals in a vocabulary of its own.
const (
	countsMsg      = "stale-hook cleanup counts"
	removedMsg     = "stale-hook cleanup removed"
	sweepFailedMsg = "stale-hook cleanup failed"
)

// PaneHookLister returns one row per live pane: the pane's hook token, empty
// for an unstamped pane, alongside its display-only location. The row count
// answers whether the tmux read succeeded and the non-empty tokens answer which
// panes are protected — two questions no consumer may conflate.
type PaneHookLister interface {
	ListAllPaneHookKeys() ([]tmux.PaneHookRow, error)
}

// Reader is the whole tmux surface hook-staleness work reads: the live pane
// enumeration keys are judged against, and the @portal-restoring marker that
// stands that judgement down for a restore's duration.
type Reader interface {
	PaneHookLister
	state.RestoringChecker
}

// View is what the enumeration could establish about this moment: the live
// token set persisted keys may be judged against, or the reason nothing may be
// judged at all. A declined view's LiveTokens is not authority.
type View struct {
	LiveTokens []string
	// PaneRows is meaningful only where Enumerated is set: an empty read is a
	// completed read, an unreached or failed one has no count to report.
	PaneRows   int
	Enumerated bool
	Decline    StandDown
}

// Outcome is everything one cycle has to say: the keys it removed, or the
// reason it declined to remove any. A caller renders both halves from it, so a
// cycle that declined can never look like one that found nothing.
type Outcome struct {
	Removed       []string
	DeclineReason Reason
}

// StalenessStandDown reports whether hook-staleness work may run at all.
// Both the sweep and the diagnostic take it before they judge a single key, so
// neither judges an entry the other protects. A restore's panes carry no token
// until the re-stamp, so work landing in that window would treat every
// token-keyed entry on the machine as lost; a deferred prune costs nothing, so
// a failed read reports no louder than the stand-down itself. The read that
// failed folds into the same stand-down under its own reason: it establishes
// nothing about the marker, so the reason it is reported under must not claim a
// restore the user is not running.
func StalenessStandDown(reader state.RestoringChecker) StandDown {
	active, err := state.RestoreWindowActive(state.IsRestoringSet(reader))
	switch {
	case err != nil:
		return declineDebug(ReasonMarkerReadFailed, "error", err)
	case active:
		return declineDebug(ReasonRestoring)
	}
	return StandDown{}
}

// JudgeAgainstLivePanes enumerates the live pane tokens persisted keys are
// judged against. entries is how many keys the file holds, which is the whole
// of what the guard below needs from it.
func JudgeAgainstLivePanes(reader PaneHookLister, entries int) View {
	rows, err := reader.ListAllPaneHookKeys()
	if err != nil {
		return View{Decline: declineWarn(ReasonPaneReadFailed, "error", err)}
	}

	view := View{PaneRows: len(rows), Enumerated: true}

	// A pane-less read is a bad read, not authority: it must never reach the
	// staleness rule, which would judge every key it can parse stale. Defer to
	// the next run. The count is of rows — under lazy stamping a live server
	// carrying no token at all is ordinary, not a failure — and with nothing
	// persisted there is nothing to protect, so the guard has no work.
	if len(rows) == 0 && entries > 0 {
		view.Decline = declineWarn(ReasonEmptyPaneRead, "entries", entries)
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

// A store holding nothing to sweep is an answer, not a failure: it aborts the
// clean with the file untouched, which is the point of returning it as an error.
var errNothingPersisted = errors.New("no hook entries to sweep")

// liveTokenEnumeration answers the clean with the live token set persisted keys
// are judged against, or with the stand-down that forbids judging them at all.
func liveTokenEnumeration(reader PaneHookLister) func(hooks.Snapshot) ([]string, error) {
	return func(snapshot hooks.Snapshot) ([]string, error) {
		// Nothing persisted is nothing to sweep, and everything past this point
		// costs: a whole-server pane enumeration here, and a deletion that
		// creates the config directory, creates the sidecar and takes an
		// exclusive hold. An install that has never registered a hook would
		// otherwise pay all four on every cycle, in every `hook set`'s way. The
		// counts line carries no pane figure, because none was taken.
		if len(snapshot) == 0 {
			logger.Debug(countsMsg, "entries", 0)
			return nil, errNothingPersisted
		}

		view := JudgeAgainstLivePanes(reader, len(snapshot))
		if view.Enumerated {
			logger.Debug(countsMsg, "panes", view.PaneRows, "entries", len(snapshot))
		}
		if view.Decline.Declined() {
			return nil, declinedError{view.Decline}
		}
		return view.LiveTokens, nil
	}
}

// Run runs one hook-staleness cycle, reporting what it removed or the reason it
// declined to remove anything.
func Run(reader Reader, store *hooks.Store) (Outcome, error) {
	// Taken before the store is read at all: a restore window is no time to
	// wait on this file's lock for an answer that cannot be acted on.
	if decline := StalenessStandDown(reader); decline.Declined() {
		return standDownOutcome(decline)
	}

	removed, err := store.CleanStale(liveTokenEnumeration(reader))
	if err != nil {
		return declinedSweep(err)
	}

	logger.Debug(removedMsg, "reaped", len(removed))

	return Outcome{Removed: removed}, nil
}

// declinedSweep renders the outcome of a clean that wrote nothing. A guard's
// own stand-down carries the reason it decided on; the store's own failure
// modes are named here because only this cycle knows what they cost it. A
// failure none of them classifies is reported here and still returned: the
// error is what drives a caller's own rendered output, not a second log line.
func declinedSweep(err error) (Outcome, error) {
	var declined declinedError
	switch {
	case errors.Is(err, errNothingPersisted):
		return Outcome{}, nil
	case errors.As(err, &declined):
		return standDownOutcome(declined.StandDown)
	case errors.Is(err, hooks.ErrLockHeld):
		// Another writer held the sidecar past the bound: nothing was written
		// and nothing is wrong, so the cycle stands down rather than reporting a
		// defect, and the next cadence retries.
		return standDownOutcome(declineWarn(ReasonLockTimeout, "error", err))
	case errors.Is(err, hooks.ErrStoreRead):
		// A hooks.json the cycle could not read, at either of the two reads it
		// takes, is a decline like any other: it wrote nothing, and its own WARN
		// names the condition and the read error that produced it.
		return standDownOutcome(declineWarn(ReasonStoreReadFailed, "error", err))
	}
	logger.Warn(sweepFailedMsg, "error", err)
	return Outcome{}, err
}

// standDownOutcome emits a decline and renders it as the cycle's outcome. The
// nil error is the point: the emitted line is the whole report, so a caller
// cannot add a second one for the same event.
func standDownOutcome(decline StandDown) (Outcome, error) {
	decline.emit()
	return Outcome{DeclineReason: decline.reason}, nil
}
