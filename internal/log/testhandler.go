package log

import (
	"log/slog"
	"testing"
)

// SetTestHandler swaps h into the shared handler indirection for the duration of
// t, restoring the previously pinned handler on cleanup. Every For-created logger
// — including those cached at package init — then routes to h. Nested swaps
// unwind LIFO, and restoration never depends on a record having been emitted.
//
// This is the only sanctioned way to replace the handler outside Init; production
// code cannot call it, since it takes a *testing.T.
func SetTestHandler(t *testing.T, h slog.Handler) {
	t.Helper()
	prev := currentHandler()
	setHandler(h)
	t.Cleanup(func() { setHandler(prev) })
}
