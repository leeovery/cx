package log

import (
	"log/slog"
	"testing"
)

// SetTestHandler routes every For-created logger — including those cached at
// package init — to h for the duration of t. Nested swaps unwind LIFO.
func SetTestHandler(t *testing.T, h slog.Handler) {
	t.Helper()
	prev := currentHandler()
	setHandler(h)
	t.Cleanup(func() { setHandler(prev) })
}
