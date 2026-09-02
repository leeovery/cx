package logtest_test

import (
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
)

func TestAssertRecord_PassesOnAMatchingRecord(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.With("component", "hooks").Info("set", "op", "set", "via", "cli")

	logtest.AssertRecord(t, sink.Records().Only(t, "log record"), logtest.RecordWant{
		Level:     slog.LevelInfo,
		Msg:       "set",
		Component: "hooks",
		Op:        "set",
		Via:       "cli",
	})
}

func TestAssertRecord_ReportsEveryMismatchedProperty(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.With("component", "aliases").Warn("set", "op", "set", "via", "cli")
	rec := sink.Records().Only(t, "log record")

	want := logtest.RecordWant{
		Level:     slog.LevelInfo,
		Msg:       "rm",
		Component: "hooks",
		Op:        "rm",
		Via:       "internal",
	}

	spy := &fakeT{}
	logtest.AssertRecord(spy, rec, want)

	if spy.errors != 5 {
		t.Errorf("AssertRecord reported %d failures, want 5 (level, msg, component, op, via)", spy.errors)
	}
}
