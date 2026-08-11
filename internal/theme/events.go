package theme

import (
	"log/slog"
	"strings"
	"sync"

	"github.com/leeovery/portal/internal/log"
)

// The `theme` log component's vocabulary is closed — attr keys: slug, slot,
// reason, path, token, count, rejected — and extending it is a spec change,
// not a call-site choice. The component name is not bound in this package:
// cmd binds log.For("theme") and injects the logger; diagnose-shaped callers
// inject log.Discard().

// Each deduplicated event's constant is used twice — as the message and as
// the dedup key's discriminator — and the two must never drift.
const (
	eventLoaded            = "loaded"
	eventEnumerated        = "enumerated"
	eventRejected          = "rejected"
	eventDirectoryUnusable = "directory unusable"
	eventFallbackApplied   = "fallback applied"
)

// EventLogger is the `theme` component's emission seam: an injected
// *slog.Logger plus the per-process dedup state its WARN events need. Dedup
// state lives on the instance, not as package state, so one process shares it
// across every enumerating path while concurrent processes each own their
// own. A nil *EventLogger is a valid silent seam: every method returns early.
type EventLogger struct {
	logger *slog.Logger

	mu   sync.Mutex
	seen map[eventKey]struct{}
}

// Dedup is per (event, identity, reason), not per file: the same theme
// reported for a different reason is a different problem and earns its own
// line.
type eventKey struct {
	event    string
	identity string
	reason   Reason
}

// NewEventLogger returns an event logger emitting through l, or silently when
// l is nil.
func NewEventLogger(l *slog.Logger) *EventLogger {
	return &EventLogger{logger: log.OrDiscard(l), seen: map[eventKey]struct{}{}}
}

// Loaded reports one nominated slot's theme actually loading. Deliberately
// not deduplicated: it is per load, not per condition.
func (e *EventLogger) Loaded(slug string, slot Slot) {
	if e == nil {
		return
	}

	e.logger.Info(eventLoaded, themeAttrs(slug, slot)...)
}

// Enumerated reports the panel opening over the assembled union: rows
// produced (built-ins included) and how many are unselectable. Not
// deduplicated — one panel open, one line — and it fires even on an absent or
// unusable themes directory.
func (e *EventLogger) Enumerated(count, rejected int) {
	if e == nil {
		return
	}

	e.logger.Info(eventEnumerated, "count", count, "rejected", rejected)
}

// FallbackApplied reports the mode-matched default standing in for a slot
// whose nomination would not load, once per slug+reason. The slug is the
// nomination that failed, never the default that replaced it.
func (e *EventLogger) FallbackApplied(slug string, slot Slot, r Reason) {
	if e == nil {
		return
	}
	if !e.firstSighting(eventKey{event: eventFallbackApplied, identity: slug, reason: r}) {
		return
	}

	e.logger.Warn(eventFallbackApplied, append(themeAttrs(slug, slot), "reason", string(r))...)
}

// Rejected reports one theme file the rejection ladder refused, once per
// slug+reason — or per path+reason where the file yields no slug (`bad
// name`), so the line always names the file it is about.
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

// DirectoryUnusable reports a misconfigured themes directory, once per
// path+reason. An absent directory is not this event: zero drop-ins is silent
// by decision.
func (e *EventLogger) DirectoryUnusable(path string, r *Rejection) {
	if e == nil {
		return
	}
	if !e.firstSighting(eventKey{event: eventDirectoryUnusable, identity: path, reason: r.Reason}) {
		return
	}

	e.logger.Warn(eventDirectoryUnusable, "path", path, "reason", string(r.Reason))
}

// Check and record are one critical section, so two paths enumerating at once
// cannot both read "unseen" and both emit.
func (e *EventLogger) firstSighting(key eventKey) bool {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, seen := e.seen[key]; seen {
		return false
	}
	e.seen[key] = struct{}{}
	return true
}

func themeAttrs(slug string, slot Slot) []any {
	if name, named := slotAttr(slot); named {
		return []any{"slug", slug, "slot", name}
	}
	return []any{"slug", slug}
}

func slotAttr(slot Slot) (string, bool) {
	return slot.AttrName()
}

// tokenAttr joins Rejection.Tokens — the same list the detail renders — and
// never parses Detail: rendered user-facing copy is not re-parsed downstream.
func tokenAttr(r *Rejection) (string, bool) {
	switch r.Reason {
	case ReasonMissingTokens, ReasonBadColour:
		return strings.Join(r.Tokens, ", "), true
	default:
		return "", false
	}
}
