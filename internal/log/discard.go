package log

import (
	"io"
	"log/slog"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// OrDiscard substitutes the shared silent logger for a nil l, which would
// otherwise panic on use.
func OrDiscard(l *slog.Logger) *slog.Logger {
	if l == nil {
		return discardLogger
	}
	return l
}

func Discard() *slog.Logger {
	return discardLogger
}
