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

// hooksRecordWant is the hooks-component half of a logtest.RecordWant: its
// callers name only the parts that vary.
type hooksRecordWant struct {
	level slog.Level
	msg   string
	op    string
	via   string
}

func assertHooksRecord(t *testing.T, rec logtest.Record, want hooksRecordWant) {
	t.Helper()
	logtest.AssertRecord(t, rec, logtest.RecordWant{
		Level:     want.level,
		Msg:       want.msg,
		Component: "hooks",
		Op:        want.op,
		Via:       want.via,
	})
}
