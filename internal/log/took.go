package log

import (
	"log/slog"
	"time"
)

// Took pins the reserved "took" attr's key and time.Duration type for every
// cycle, sweep and tick summary line.
func Took(start time.Time) slog.Attr {
	return slog.Duration("took", time.Since(start))
}
