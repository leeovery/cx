package logtest_test

import (
	"errors"
	"fmt"
	"log/slog"
	"testing"

	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/logtest"
)

var errSentinel = errors.New("write failed")

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

	spy := &harnesstest.Recorder{}
	logtest.AssertRecord(spy, rec, want)

	if len(spy.Errors) != 5 {
		t.Errorf("AssertRecord reported %d failures, want 5 (level, msg, component, op, via): %v", len(spy.Errors), spy.Errors)
	}
}

func TestAssertWriteFailure_PassesOnAMatchingRecord(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.Warn("set", "error", fmt.Errorf("persist: %w", errSentinel), "error_class", "write-failed-write")

	logtest.AssertWriteFailure(t, sink.Records().Only(t, "log record"), "write-failed-write", errSentinel)
}

func TestAssertWriteFailure_FailsWhenTheErrorClassAttrDiffers(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.Warn("set", "error", fmt.Errorf("persist: %w", errSentinel), "error_class", "write-failed-rename")
	rec := sink.Records().Only(t, "log record")

	spy := &harnesstest.Recorder{}
	logtest.AssertWriteFailure(spy, rec, "write-failed-write", errSentinel)

	if len(spy.Errors) != 1 {
		t.Errorf("AssertWriteFailure reported %d failures on a mismatched error_class, want 1: %v", len(spy.Errors), spy.Errors)
	}
}

func TestAssertWriteFailure_FailsWhenTheCarriedErrorDoesNotWrapTheSentinel(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.Warn("set", "error", errors.New("unrelated"), "error_class", "write-failed-write")
	rec := sink.Records().Only(t, "log record")

	spy := &harnesstest.Recorder{}
	logtest.AssertWriteFailure(spy, rec, "write-failed-write", errSentinel)

	if len(spy.Errors) != 1 {
		t.Errorf("AssertWriteFailure reported %d failures on an error that does not wrap the sentinel, want 1: %v", len(spy.Errors), spy.Errors)
	}
}
