package cmd

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
)

// For tests that drive a command body without going through main: records from
// the package-level component loggers land in <dir>/portal.log with the
// baseline attrs.
func initTestLogToStateDir(t *testing.T, dir, version string) {
	t.Helper()
	initTestLogToStateDirAs(t, dir, version, "daemon")
}

// It creates dir (log.Init's writer does not create parents) and brackets
// log.Init with a handler snapshot-and-restore so the process-wide swap does
// not leak into sibling tests.
func initTestLogToStateDirAs(t *testing.T, dir, version, processRole string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir state dir: %v", err)
	}
	log.SetTestHandler(t, slog.New(slog.NewTextHandler(io.Discard, nil)).Handler())
	if err := log.Init(dir, version, processRole); err != nil {
		t.Fatalf("log.Init: %v", err)
	}
}

func newCaptureLoggerForComponent(t *testing.T, component string) (*slog.Logger, *logtest.Sink) {
	t.Helper()
	sink := &logtest.Sink{}
	return slog.New(sink).With("component", component), sink
}

// hooksRecordWant is the shape of one hooks-component record. The message and
// the op attr are separate fields because they are separate parts of the line's
// contract, even though the component's emissions currently set both to the
// same string.
type hooksRecordWant struct {
	level slog.Level
	msg   string
	op    string
	via   string
}

// assertHooksRecord checks the parts every hooks-component record shares. The
// attrs that belong to one emission — a stand-down's reason, a failure's error
// — stay with their own caller.
func assertHooksRecord(t *testing.T, rec logtest.Record, want hooksRecordWant) {
	t.Helper()
	if rec.Level != want.level {
		t.Errorf("level = %v, want %v", rec.Level, want.level)
	}
	if rec.Msg != want.msg {
		t.Errorf("message = %q, want %q", rec.Msg, want.msg)
	}
	if got := rec.AttrString(t, "component"); got != "hooks" {
		t.Errorf("component = %q, want %q", got, "hooks")
	}
	if got := rec.AttrString(t, "op"); got != want.op {
		t.Errorf("op = %q, want %q", got, want.op)
	}
	if got := rec.AttrString(t, "via"); got != want.via {
		t.Errorf("via = %q, want %q", got, want.via)
	}
}
