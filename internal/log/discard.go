package log

import (
	"io"
	"log/slog"
)

var discardLogger = slog.New(slog.NewTextHandler(io.Discard, nil))

// OrDiscard returns l, or the shared silent logger when l is nil. Entry points
// that accept an optional logger call it once at entry: a nil *slog.Logger panics
// on use.
func OrDiscard(l *slog.Logger) *slog.Logger {
	if l == nil {
		return discardLogger
	}
	return l
}

// Discard returns the shared silent logger, for construction sites with no
// candidate logger to fall back from.
func Discard() *slog.Logger {
	return discardLogger
}
