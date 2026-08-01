package theme

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/leeovery/portal/internal/log"
)

// events.go is internal/theme's whole emission surface for the `theme` log
// component (§12.3) — a spec-governed addition to Portal's closed component
// vocabulary, with `spawn` and `resolve` as direct precedent.
//
// The vocabulary is CLOSED and is recorded here so a call site cannot invent
// against it:
//
//	Component: theme
//	Attr keys: slug, slot, reason, path, token, count, rejected
//
// The component name is NOT bound in this package. `cmd` binds it and injects
// the logger — log.For("theme") on the paths where a theme is USED (TUI
// construction, the panel, the theme persister), log.Discard() on `portal
// doctor`, `portal theme export` and capturetool — which is why the loader
// emits but never decides. Extending either list is a spec amendment, not a
// call-site choice.
//
// Two of the seven events are implemented here, both WARN, both deduplicated
// per process. The other five belong to later phases and have no site yet:
//
//	theme: loaded              INFO   Phase 5
//	theme: fallback applied    WARN   Phase 5
//	theme: appearance migrated INFO   Phase 6
//	theme: commit failed       WARN   Phase 6
//	theme: enumerated          INFO   Phase 8
//
// Only four of the seven attr keys are reachable from this phase's two events;
// `slot`, `count` and `rejected` arrive with the events above.

// The two events this phase emits, as their exact log messages. Each is a
// constant rather than a literal because it is used twice — once as the message
// and once as the dedup key's event discriminator — and the two must never
// drift.
const (
	eventRejected          = "rejected"
	eventDirectoryUnusable = "directory unusable"
)

// EventLogger is the `theme` component's emission seam: an INJECTED
// *slog.Logger plus the per-process dedup state its two WARN events need.
//
// Emission is controlled by that injected logger, NOT by this package deciding
// (§12.3). A diagnose-shaped caller is constructed with log.Discard() and writes
// nothing at all: the component records where a theme is USED, never where one
// is DIAGNOSED. Doctor is the run most likely to hit a full reject set, so
// emitting there would put the largest WARN volume on the surface needing it
// least, and a diagnosis command writing WARNs about the state it just printed
// would break its read-only claim.
//
// The dedup state lives HERE, on the injected instance, rather than as package
// state in the leaf. That is what lets one TUI process share it across every
// path that enumerates — the construction-time by-name read and the panel's
// enumeration hit the same condition (§5.5) and must not double up — while
// §8.9's concurrent Portal processes each own their own, and a test controls it
// simply by injecting a fresh one. The mutex is what keeps that safe when a
// single process drives enumeration from more than one path.
//
// A nil *EventLogger is a valid SILENT seam: the zero-value Loader carries one,
// so every method returns early rather than every call site guarding.
type EventLogger struct {
	logger *slog.Logger

	mu   sync.Mutex
	seen map[eventKey]struct{}
}

// eventKey is one already-emitted event: which event it was, the identity it was
// reported under — the slug where there is one, the path where there is not —
// and its reason.
//
// The event is part of the key so the two catalogues cannot collide on a shared
// identity, and the reason is part of it because dedup is per (identity, reason)
// PAIR rather than per file: the same theme reported for a DIFFERENT reason is a
// different problem and earns its own line (§12.3).
type eventKey struct {
	event    string
	identity string
	reason   Reason
}

// NewEventLogger returns an event logger emitting through l, or silently when l
// is nil (log.OrDiscard at entry, the internal/spawn precedent).
func NewEventLogger(l *slog.Logger) *EventLogger {
	return &EventLogger{logger: log.OrDiscard(l), seen: map[eventKey]struct{}{}}
}

// Rejected reports one theme file the §6.2 ladder refused, ONCE per slug+reason
// — or per path+reason where the file yields no slug.
//
// The identity attr is `slug` where the file yields one and `path` where it does
// not: the `bad name` case, the one class whose filename produces no usable
// identity (§6.2 rung 1). A rejection always has one or the other, so the line
// always names the file it is about — and both halves of that choice carry into
// the dedup key, because without the path half the class most likely to recur
// across panel opens would be the one class with no key at all.
//
// Deduplication is what makes this a forensic trail rather than a running
// commentary: enumeration re-reads the directory on every panel open (§5.8), so
// five opens over the same broken file are one WARN, not five.
func (e *EventLogger) Rejected(slug, path string, r *Rejection) {
	if e == nil {
		return
	}

	identityKey, identity := "slug", slug
	if slug == "" {
		identityKey, identity = "path", path
	}
	if !e.firstSighting(eventKey{event: eventRejected, identity: identity, reason: r.Reason}) {
		return
	}

	attrs := []any{identityKey, identity, "reason", string(r.Reason)}
	if token, named := tokenAttr(r); named {
		attrs = append(attrs, "token", token)
	}

	e.logger.Warn(eventRejected, attrs...)
}

// DirectoryUnusable reports §5.5's misconfigured themes directory — unreadable,
// or a regular file where a directory belongs — ONCE per path+reason, for the
// same reason its neighbour dedups.
//
// An ABSENT directory is not this event and never reaches here: zero drop-ins is
// the common case and is silent by decision.
func (e *EventLogger) DirectoryUnusable(path string, r *Rejection) {
	if e == nil {
		return
	}
	if !e.firstSighting(eventKey{event: eventDirectoryUnusable, identity: path, reason: r.Reason}) {
		return
	}

	e.logger.Warn(eventDirectoryUnusable, "path", path, "reason", string(r.Reason))
}

// firstSighting reports whether key has not been emitted before, recording it as
// it answers.
//
// Check and record are ONE critical section, so two paths enumerating at once
// cannot both read "unseen" and both emit — the emitted count, not merely the
// map, is what has to survive concurrency.
func (e *EventLogger) firstSighting(key eventKey) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, seen := e.seen[key]; seen {
		return false
	}
	e.seen[key] = struct{}{}
	return true
}

// tokenAttr returns the value of the `token` attr for a rejection, and reports
// whether the reason names a token at all — true for exactly `missing tokens`
// and `bad colour`, the two §6.2 reasons that do, and this attr's only consumers
// in the whole component.
//
// The value is §14A's comma-separated list, the same one doctor prints for the
// same reason: the log and a doctor run then name the same tokens, and the key
// stays SINGLE-VALUED as §12.3 declares it even when several tokens are missing
// or several keys carry a bad colour. (§12.3 pins the key but not its
// cardinality; this is the decision, recorded so a later phase can revisit it
// deliberately.)
//
// `bad colour`'s detail IS that list — every offending `key = value` pair — so
// it rides verbatim. `missing tokens`' detail is the list behind a fixed
// lead-in, which comes off through the constant that put it there: the reason is
// already its own attr, so carrying the word "missing" inside the value would
// only repeat it. Re-deriving bare token names from `bad colour`'s pairs was
// rejected — it would mean parsing rendered §14A copy back apart, which is
// exactly what Rejection.Detail's contract says nothing downstream does.
func tokenAttr(r *Rejection) (string, bool) {
	switch r.Reason {
	case ReasonMissingTokens:
		return strings.TrimPrefix(r.Detail, detailMissingTokensLeadIn), true
	case ReasonBadColour:
		return r.Detail, true
	default:
		return "", false
	}
}
