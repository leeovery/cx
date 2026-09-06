package hooksweep

import (
	"context"
	"log/slog"

	"github.com/leeovery/portal/internal/hooks"
)

// standDownMsg and standDownAttrs give every stand-down one line shape, so a
// single grep answers whether the prune declined and why.
const standDownMsg = "clean-stale-skipped"

func standDownAttrs(reason Reason, extra ...any) []any {
	return append([]any{"op", standDownMsg, "via", hooks.ViaInternal.String(), "reason", string(reason)}, extra...)
}

// StandDown is a declined cycle: the reason its caller reports alongside the
// level and attrs its log line carries. The zero value means the cycle ran.
// Emission is the sweep's — a diagnosis that declines has pruned nothing and
// must not claim a stand-down in the reaper's own vocabulary.
type StandDown struct {
	reason Reason
	level  slog.Level
	attrs  []any
}

// Reason names the condition that declined the cycle, empty for a cycle that
// ran.
func (s StandDown) Reason() Reason { return s.reason }

// Declined reports whether the cycle stood down at all.
func (s StandDown) Declined() bool { return s.reason != "" }

func (s StandDown) emit() {
	logger.Log(context.Background(), s.level, standDownMsg, standDownAttrs(s.reason, s.attrs...)...)
}

// A restore window is an expected state, and warning through every one of them
// would name a hazard being avoided rather than encountered.
func declineDebug(reason Reason, attrs ...any) StandDown {
	return StandDown{reason: reason, level: slog.LevelDebug, attrs: attrs}
}

// A lock that will not yield and a tmux read that answered nothing usable are
// both anomalies.
func declineWarn(reason Reason, attrs ...any) StandDown {
	return StandDown{reason: reason, level: slog.LevelWarn, attrs: attrs}
}

// declinedError is a stand-down in transit — the reason its guard decided on,
// carried by the very error that aborts the clean, so a decline names its
// reason at the site that returns it rather than in a variable beside it.
type declinedError struct {
	StandDown
}

func (e declinedError) Error() string {
	return "hook staleness cycle declined: " + string(e.reason)
}
