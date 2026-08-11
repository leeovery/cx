package cmd

import (
	"io"
	"log/slog"
	"os"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
)

// Wires the production log handler so records from the package-level
// component loggers land in <dir>/portal.log with the baseline attrs, for
// tests that drive a command body without going through main.
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

// Binds the component so it renders on every line, matching production text
// output.
func newCaptureLoggerForComponent(t *testing.T, component string) (*slog.Logger, *logtest.Sink) {
	t.Helper()
	sink := &logtest.Sink{}
	return slog.New(sink).With("component", component), sink
}
