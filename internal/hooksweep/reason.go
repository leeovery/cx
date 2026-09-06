// Package hooksweep holds the hook-staleness cycle: the judgement of persisted
// hook keys against the live pane tokens that protect them, the conditions that
// forbid that judgement, and the vocabulary a caller reports either in. It owns
// the whole cycle's emission — its counts, its stand-downs and its failures
// alike ride the hooks component, whichever caller drove it — so a caller keeps
// only the rendering its own surface owes.
package hooksweep

// Reason is the closed vocabulary of reasons a cycle declined to run under.
// It is a type rather than a convention over plain strings so no string-typed
// value can be reported as one; an untyped constant still converts implicitly.
type Reason string

// The reasons a cycle declined to run under, which the logged reason attr also
// names. Every surface that renders one works from this set, so their
// vocabularies cannot drift from it. A sweep that ran and failed is not among
// them: it declined nothing, and its caller-facing line is the repair's own.
const (
	ReasonRestoring        Reason = "restoring"
	ReasonMarkerReadFailed Reason = "marker-read-failed"
	ReasonStoreReadFailed  Reason = "store-read-failed"
	ReasonPaneReadFailed   Reason = "pane-read-failed"
	ReasonEmptyPaneRead    Reason = "empty-pane-read"
	ReasonLockTimeout      Reason = "lock-timeout"
)

// Reasons makes the reasons above enumerable, so anything that must cover
// every one of them ranges over the set rather than restating it. A reason
// declared and left out of it is invisible to everything that works from it.
//
// The set is deliberately complete rather than fully reachable from every
// surface that renders it, so a reader meeting a phrase with no path to it is
// not hunting for one that was never written:
//
//   - lock-timeout cannot reach the read-only diagnosis at all. A read that
//     cannot take the sidecar reads anyway, unlocked, so the diagnosis never
//     stands down for the lock — only the sweep, which takes it exclusively to
//     delete, can. Its not-evaluable phrase exists for vocabulary completeness,
//     not for an observed leak, and making it reachable would mean reversing the
//     degrade-to-unlocked read, which is settled behaviour and stays as it is.
//   - store-read-failed reaches the diagnosis only through this vocabulary:
//     the check names the reason and lets the tables word it, rather than
//     carrying copy of its own.
var Reasons = []Reason{
	ReasonRestoring,
	ReasonMarkerReadFailed,
	ReasonStoreReadFailed,
	ReasonPaneReadFailed,
	ReasonEmptyPaneRead,
	ReasonLockTimeout,
}
