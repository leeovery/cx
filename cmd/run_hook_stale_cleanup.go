package cmd

import (
	"context"
	"errors"
	"log/slog"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// skipReason is the closed vocabulary of reasons a cycle removed nothing. It is
// a type rather than a convention over plain strings so no string-typed value
// can be reported as one; an untyped constant still converts implicitly.
type skipReason string

// The reasons a cycle removed nothing: the ones it declines to run under,
// which the logged reason attr also names, and the failure only the
// caller-facing rendering reports. Every surface that renders one works from
// this set, so their vocabularies cannot drift from it.
const (
	skipReasonRestoring        skipReason = "restoring"
	skipReasonMarkerReadFailed skipReason = "marker-read-failed"
	skipReasonStoreReadFailed  skipReason = "store-read-failed"
	skipReasonPaneReadFailed   skipReason = "pane-read-failed"
	skipReasonEmptyPaneRead    skipReason = "empty-pane-read"
	skipReasonLockTimeout      skipReason = "lock-timeout"
	skipReasonSweepFailed      skipReason = "sweep-failed"
)

// skipReasons makes the reasons above enumerable, so anything that must cover
// every one of them ranges over the set rather than restating it. A reason
// declared and left out of it is invisible to everything that works from it.
//
// The set is deliberately complete rather than fully reachable from every
// surface that renders it, so a reader meeting a phrase with no path to it is
// not hunting for one that was never written:
//
//   - lock-timeout cannot reach the read-only diagnosis at all. A read that
//     cannot take the sidecar reads anyway, unlocked, so checkStaleHooks never
//     stands down for the lock — only the sweep, which takes it exclusively to
//     delete, can. Its not-evaluable phrase exists for vocabulary completeness,
//     not for an observed leak, and making it reachable would mean reversing the
//     degrade-to-unlocked read, which is settled behaviour and stays as it is.
//   - store-read-failed reaches the diagnosis only through this vocabulary:
//     checkStaleHooks names the reason and lets the tables word it, rather than
//     carrying copy of its own.
//   - sweep-failed is the --fix repair's line alone. A sweep that failed
//     declined nothing — it ran and left the entry it could not delete — so the
//     post-repair diagnosis reports that entry as stale rather than as a
//     stand-down.
var skipReasons = []skipReason{
	skipReasonRestoring,
	skipReasonMarkerReadFailed,
	skipReasonStoreReadFailed,
	skipReasonPaneReadFailed,
	skipReasonEmptyPaneRead,
	skipReasonLockTimeout,
	skipReasonSweepFailed,
}

// The phrases both surface vocabularies share are written once here, so
// re-wording one moves every line that prints it. A restore that is running and
// a marker that could not be read stand the cycle down alike, but they are
// different conditions: the server is routinely down when a user reaches for a
// diagnosis, and reporting that as a restore would assert something that is not
// happening.
const (
	restoreStandDownPhrase     = "restore in progress"
	markerReadStandDownPhrase  = "could not read the restore marker"
	storeReadStandDownPhrase   = "could not read hooks.json"
	paneReadStandDownPhrase    = "could not enumerate live panes"
	sweepFailedStandDownPhrase = "the sweep could not complete"
)

// skippedPrunePhrases completes "Skipped stale hook prune: …" for a user who
// asked for a repair. A failed enumeration and a successful one that answered
// nothing are separate conditions, so neither borrows the other's words.
var skippedPrunePhrases = map[skipReason]string{
	skipReasonRestoring:        restoreStandDownPhrase,
	skipReasonMarkerReadFailed: markerReadStandDownPhrase,
	skipReasonStoreReadFailed:  storeReadStandDownPhrase,
	skipReasonPaneReadFailed:   paneReadStandDownPhrase,
	skipReasonEmptyPaneRead:    "live pane list came back empty",
	skipReasonLockTimeout:      "hooks.json is locked",
	skipReasonSweepFailed:      sweepFailedStandDownPhrase,
}

// notEvaluableDetails renders a stand-down reason as the reason the count cannot
// be taken, so the diagnostic reports exactly what the reaper declined to judge.
var notEvaluableDetails = map[skipReason]string{
	skipReasonRestoring:        restoreStandDownPhrase + " (not evaluable)",
	skipReasonMarkerReadFailed: markerReadStandDownPhrase,
	skipReasonStoreReadFailed:  storeReadStandDownPhrase,
	skipReasonPaneReadFailed:   paneReadStandDownPhrase,
	skipReasonEmptyPaneRead:    "zero live panes with hooks present (not evaluable)",
	skipReasonLockTimeout:      "hooks.json is locked (not evaluable)",
	skipReasonSweepFailed:      sweepFailedStandDownPhrase,
}

// phraseFor renders a stand-down reason through one of the surface vocabularies
// above. An unmapped reason falls through to its raw value, so no stand-down can
// print nothing — a last-resort net, not a licence to leave a reason unmapped.
func phraseFor(m map[skipReason]string, reason skipReason) string {
	if phrase, ok := m[reason]; ok {
		return phrase
	}
	return string(reason)
}

// standDownMsg and standDownAttrs give every stand-down one line shape, so a
// single grep answers whether the prune declined and why.
const standDownMsg = "clean-stale-skipped"

func standDownAttrs(reason skipReason, extra ...any) []any {
	return append([]any{"op", standDownMsg, "via", hooks.ViaInternal.String(), "reason", string(reason)}, extra...)
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
	reason skipReason
	level  slog.Level
	attrs  []any
}

func (s standDown) declined() bool { return s.reason != "" }

func (s standDown) emit() {
	hooksLogger.Log(context.Background(), s.level, standDownMsg, standDownAttrs(s.reason, s.attrs...)...)
}

// A restore window is an expected state, and warning through every one of them
// would name a hazard being avoided rather than encountered.
func declineDebug(reason skipReason, attrs ...any) standDown {
	return standDown{reason: reason, level: slog.LevelDebug, attrs: attrs}
}

// A lock that will not yield and a tmux read that answered nothing usable are
// both anomalies.
func declineWarn(reason skipReason, attrs ...any) standDown {
	return standDown{reason: reason, level: slog.LevelWarn, attrs: attrs}
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
// Both the sweep and the diagnostic take it before they judge a single key, so
// neither judges an entry the other protects. A restore's panes carry no token
// until the re-stamp, so work landing in that window would treat every
// token-keyed entry on the machine as lost; a deferred prune costs nothing, so
// a failed read reports no louder than the stand-down itself. The read that
// failed folds into the same stand-down under its own reason: it establishes
// nothing about the marker, so the reason it is reported under must not claim a
// restore the user is not running.
func hookStalenessStandDown(reader state.RestoringChecker) standDown {
	active, err := state.RestoreWindowActive(state.IsRestoringSet(reader))
	switch {
	case err != nil:
		return declineDebug(skipReasonMarkerReadFailed, "error", err)
	case active:
		return declineDebug(skipReasonRestoring)
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

// sweepOutcome is everything one cycle has to say: the keys it removed, or the
// reason it declined to remove any. A caller renders both halves from it, so a
// cycle that declined can never look like one that found nothing.
type sweepOutcome struct {
	Removed       []string
	DeclineReason skipReason
}

// A store holding nothing to sweep is an answer, not a failure: it aborts the
// clean with the file untouched, which is the point of returning it as an error.
var errNothingPersisted = errors.New("no hook entries to sweep")

// declinedError is a stand-down in transit — the reason its guard decided on,
// carried by the very error that aborts the clean, so a decline names its
// reason at the site that returns it rather than in a variable beside it.
type declinedError struct {
	standDown
}

func (e declinedError) Error() string {
	return "hook staleness cycle declined: " + string(e.reason)
}

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
			hooksLogger.Debug("stale-hook cleanup counts", "entries", 0)
			return nil, errNothingPersisted
		}

		view := judgeAgainstLivePanes(reader, len(snapshot))
		if view.Enumerated {
			hooksLogger.Debug("stale-hook cleanup counts", "panes", view.PaneRows, "entries", len(snapshot))
		}
		if view.Decline.declined() {
			return nil, declinedError{view.Decline}
		}
		return view.LiveTokens, nil
	}
}

// sweepFailedMsg reports a cycle that neither ran to completion nor stood down
// for a reason it could name. It rides the hooks component with every other
// line of the cycle, so one grep reconstructs the whole of it and no caller has
// to word the sweep's internals in a vocabulary of its own.
const sweepFailedMsg = "stale-hook cleanup failed"

// runHookStaleCleanup runs one hook-staleness cycle, reporting what it removed
// or the reason it declined to remove anything.
//
// The whole cycle — its counts, its stand-downs and its failures alike — is
// emitted under the hooks component, whichever caller drove it. A caller keeps
// only the cycle-summary line its own component owes.
func runHookStaleCleanup(reader staleSweepReader, store *hooks.Store) (sweepOutcome, error) {
	// Taken before the store is read at all: a restore window is no time to
	// wait on this file's lock for an answer that cannot be acted on.
	if decline := hookStalenessStandDown(reader); decline.declined() {
		return standDownOutcome(decline)
	}

	removed, err := store.CleanStale(liveTokenEnumeration(reader))
	if err != nil {
		return declinedSweep(err)
	}

	hooksLogger.Debug("stale-hook cleanup removed", "reaped", len(removed))

	return sweepOutcome{Removed: removed}, nil
}

// declinedSweep renders the outcome of a clean that wrote nothing. A guard's
// own stand-down carries the reason it decided on; the store's own failure
// modes are named here because only this cycle knows what they cost it. A
// failure none of them classifies is reported here and still returned: the
// error is what drives a caller's own rendered output, not a second log line.
func declinedSweep(err error) (sweepOutcome, error) {
	var declined declinedError
	switch {
	case errors.Is(err, errNothingPersisted):
		return sweepOutcome{}, nil
	case errors.As(err, &declined):
		return standDownOutcome(declined.standDown)
	case errors.Is(err, hooks.ErrLockHeld):
		// Another writer held the sidecar past the bound: nothing was written
		// and nothing is wrong, so the cycle stands down rather than reporting a
		// defect, and the next cadence retries.
		return standDownOutcome(declineWarn(skipReasonLockTimeout, "error", err))
	case errors.Is(err, hooks.ErrSnapshotRead):
		// An unreadable store is a decline like any other: the cycle wrote
		// nothing, and its own WARN names the condition and the read error that
		// produced it.
		return standDownOutcome(declineWarn(skipReasonStoreReadFailed, "error", err))
	}
	hooksLogger.Warn(sweepFailedMsg, "error", err)
	return sweepOutcome{}, err
}

// standDownOutcome emits a decline and renders it as the cycle's outcome. The
// nil error is the point: the emitted line is the whole report, so a caller
// cannot add a second one for the same event.
func standDownOutcome(decline standDown) (sweepOutcome, error) {
	decline.emit()
	return sweepOutcome{DeclineReason: decline.reason}, nil
}
