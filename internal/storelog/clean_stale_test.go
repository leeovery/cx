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
	rec.RequireDuration(t, "took")
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
	rec.RequireDuration(t, "took")
	loggedErr := rec.ErrorAttr(t, "error")
	if loggedErr != saveErr {
		t.Errorf("logged error attr = %v, want the raw saveErr %v", loggedErr, saveErr)
	}
}
