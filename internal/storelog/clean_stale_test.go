package storelog_test

import (
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/fileutil"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/storelog"
)

func TestEmitCleanStaleSummary_SuccessInfo(t *testing.T) {
	logger := log.For("hooks")
	sink := logtest.Install(t)

	storelog.EmitCleanStaleSummary(logger, 2, time.Now().Add(-5*time.Millisecond), nil)

	rec := sink.OnlyRecord(t)
	logtest.AssertRecord(t, rec, logtest.RecordWant{
		Level:     slog.LevelInfo,
		Msg:       "clean-stale",
		Component: "hooks",
		Op:        "clean-stale",
		Via:       "internal",
	})
	if got := rec.AttrString(t, "entries"); got != "2" {
		t.Errorf("entries = %q, want %q", got, "2")
	}
	tookVal, ok := rec.Attrs["took"]
	if !ok {
		t.Fatalf("summary missing took attr: %+v", rec.Attrs)
	}
	if tookVal.Kind() != slog.KindDuration {
		t.Errorf("took attr kind = %v, want Duration", tookVal.Kind())
	}
	if _, ok := rec.Attrs["error"]; ok {
		t.Errorf("success summary must omit error attr: %+v", rec.Attrs)
	}
	if _, ok := rec.Attrs["error_class"]; ok {
		t.Errorf("success summary must omit error_class attr: %+v", rec.Attrs)
	}
}

func TestEmitCleanStaleSummary_FailureWarn(t *testing.T) {
	logger := log.For("projects")
	sink := logtest.Install(t)

	saveErr := fmt.Errorf("%w: boom", fileutil.ErrWriteTempCreate)
	storelog.EmitCleanStaleSummary(logger, 3, time.Now().Add(-5*time.Millisecond), saveErr)

	rec := sink.OnlyRecord(t)
	logtest.AssertRecord(t, rec, logtest.RecordWant{
		Level:     slog.LevelWarn,
		Msg:       "clean-stale",
		Component: "projects",
		Op:        "clean-stale",
		Via:       "internal",
	})
	if got := rec.AttrString(t, "entries"); got != "3" {
		t.Errorf("entries = %q, want %q", got, "3")
	}
	if got := rec.AttrString(t, "error_class"); got != "write-failed-temp-create" {
		t.Errorf("error_class = %q, want %q", got, "write-failed-temp-create")
	}
	tookVal, ok := rec.Attrs["took"]
	if !ok {
		t.Fatalf("WARN missing took attr: %+v", rec.Attrs)
	}
	if tookVal.Kind() != slog.KindDuration {
		t.Errorf("took attr kind = %v, want Duration", tookVal.Kind())
	}
	errVal, ok := rec.Attrs["error"]
	if !ok {
		t.Fatalf("WARN record missing error attr: %+v", rec.Attrs)
	}
	loggedErr, ok := errVal.Any().(error)
	if !ok {
		t.Fatalf("error attr is not an error value: %T", errVal.Any())
	}
	if loggedErr != saveErr {
		t.Errorf("logged error attr = %v, want the raw saveErr %v", loggedErr, saveErr)
	}
}
