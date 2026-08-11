package log

import (
	"log/slog"
	"testing"
	"time"
)

// Fixed rather than derived from the live process, so a rendered fixture is
// identical across runs and machines. Single-token values need no quoting.
const (
	testRenderPID         = 0
	testRenderVersion     = "test"
	testRenderProcessRole = "test"
)

// RenderLineForTest renders one record to its canonical portal.log line through
// the production render path, without driving Init. It mutates no process-global
// state and writes to no sink.
//
// Production code must never call it; the *testing.T-first parameter makes that
// structural.
func RenderLineForTest(t *testing.T, ts time.Time, level slog.Level, component, message string, attrs ...slog.Attr) string {
	t.Helper()

	h := newTextHandler(nil, level, testRenderPID, testRenderVersion, testRenderProcessRole).(*textHandler)
	h = h.WithAttrs([]slog.Attr{slog.String(componentKey, component)}).(*textHandler)

	r := slog.NewRecord(ts, level, message, 0)
	r.AddAttrs(attrs...)

	return h.render(r)
}
