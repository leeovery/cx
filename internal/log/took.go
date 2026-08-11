package log

import (
	"log/slog"
	"time"
)

// Took builds the reserved "took" cycle-summary attr, pinning its key and
// time.Duration type in one place for every cycle, sweep and tick summary line.
func Took(start time.Time) slog.Attr {
	return slog.Duration("took", time.Since(start))
}
