package logtest_test

import (
	"log/slog"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/logtest"
)

func TestNewCaptureLogger_RendersLevelMessageKeyValue(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)

	logger.Info("hello world", "session", "demo", "count", 3)

	if got, want := sink.Body(), "INFO hello world session=demo count=3"; got != want {
		t.Errorf("Body() = %q, want %q", got, want)
	}
}

func TestSink_RecordsLevelMessageAndOrderedKeys(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)

	logger.Warn("geometry complete", "panes", 4, "took", "5ms", "anomalous", false)

	recs := sink.Records()
	if len(recs) != 1 {
		t.Fatalf("Records() len = %d, want 1", len(recs))
	}
	r := recs[0]
	if r.Level != slog.LevelWarn {
		t.Errorf("Level = %v, want WARN", r.Level)
	}
	if r.Msg != "geometry complete" {
		t.Errorf("Msg = %q, want %q", r.Msg, "geometry complete")
	}
	wantKeys := []string{"panes", "took", "anomalous"}
	if len(r.Keys) != len(wantKeys) {
		t.Fatalf("Keys = %v, want %v", r.Keys, wantKeys)
	}
	for i, k := range wantKeys {
		if r.Keys[i] != k {
			t.Errorf("Keys[%d] = %q, want %q", i, r.Keys[i], k)
		}
	}
}

func TestSink_RendersBoundComponentOnEveryLine(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	bound := logger.With("component", "daemon")

	bound.Info("lock acquired", "tmux_pane", "%42")

	if got, want := sink.Body(), "INFO lock acquired component=daemon tmux_pane=%42"; got != want {
		t.Errorf("Body() = %q, want %q", got, want)
	}
}

func TestSink_Lines_ReturnsCopy(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)

	logger.Info("one")
	snapshot := sink.Lines()
	logger.Info("two")

	if len(snapshot) != 1 {
		t.Fatalf("snapshot len = %d, want 1 (must not see later writes)", len(snapshot))
	}
	if snapshot[0] != "INFO one" {
		t.Errorf("snapshot[0] = %q, want %q", snapshot[0], "INFO one")
	}
}

func TestSink_RecordsAttrValues(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	bound := logger.With("component", "daemon")

	bound.Info("tick complete", "panes", 4, "took", 3*time.Second)

	recs := sink.Records()
	if len(recs) != 1 {
		t.Fatalf("Records() len = %d, want 1", len(recs))
	}
	r := recs[0]

	comp, ok := r.Attrs["component"]
	if !ok {
		t.Fatalf("Attrs missing bound component: %+v", r.Attrs)
	}
	if comp.String() != "daemon" {
		t.Errorf("component = %q, want %q", comp.String(), "daemon")
	}
	panes, ok := r.Attrs["panes"]
	if !ok || panes.Kind() != slog.KindInt64 || panes.Int64() != 4 {
		t.Errorf("panes attr = %+v, want Int64 4", panes)
	}
	took, ok := r.Attrs["took"]
	if !ok || took.Kind() != slog.KindDuration || took.Duration() != 3*time.Second {
		t.Errorf("took attr = %+v, want Duration 3s", took)
	}

	if len(r.Attrs) != len(r.Keys) {
		t.Fatalf("Attrs key count %d != Keys count %d (%v / %v)", len(r.Attrs), len(r.Keys), r.Attrs, r.Keys)
	}
	for _, k := range r.Keys {
		if _, ok := r.Attrs[k]; !ok {
			t.Errorf("key %q present in Keys but missing from Attrs", k)
		}
	}
}

func TestSink_RecordsAttrValues_LastWriteWins(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	bound := logger.With("via", "internal")

	bound.Info("set", "via", "cli")

	recs := sink.Records()
	if len(recs) != 1 {
		t.Fatalf("Records() len = %d, want 1", len(recs))
	}
	via, ok := recs[0].Attrs["via"]
	if !ok {
		t.Fatalf("Attrs missing via: %+v", recs[0].Attrs)
	}
	if via.String() != "cli" {
		t.Errorf("via = %q, want %q (per-call attr must win over bound)", via.String(), "cli")
	}
}

func TestRecord_AttrString(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.Info("set", "op", "set")

	r := sink.OnlyRecord(t)
	if got := r.AttrString(t, "op"); got != "set" {
		t.Errorf("AttrString(op) = %q, want %q", got, "set")
	}

	if !expectFail(func(sub logtest.TestingT) { r.AttrString(sub, "nope") }) {
		t.Errorf("AttrString must fail the test for a missing key")
	}
}

func TestRecord_IntAttr(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.Info("tick complete", "panes", 7, "msg", "text")

	r := sink.OnlyRecord(t)
	if got := r.IntAttr(t, "panes"); got != 7 {
		t.Errorf("IntAttr(panes) = %d, want 7", got)
	}

	if !expectFail(func(sub logtest.TestingT) { r.IntAttr(sub, "msg") }) {
		t.Errorf("IntAttr must fail for a non-Int64 attr")
	}
}

func TestRecord_RequireDuration(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.Info("tick complete", "took", 5*time.Millisecond, "entries", "2")

	r := sink.OnlyRecord(t)
	r.RequireDuration(t, "took")

	if !expectFail(func(sub logtest.TestingT) { r.RequireDuration(sub, "entries") }) {
		t.Errorf("RequireDuration must fail for a non-Duration attr")
	}
}

func TestRecord_HasAttr(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.Info("rm", "op", "rm")

	r := sink.OnlyRecord(t)
	if !r.HasAttr("op") {
		t.Errorf("HasAttr(op) = false, want true")
	}
	if r.HasAttr("value") {
		t.Errorf("HasAttr(value) = true, want false")
	}
}

func TestSink_OnlyRecord(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.Info("first")

	r := sink.OnlyRecord(t)
	if r.Msg != "first" {
		t.Errorf("OnlyRecord().Msg = %q, want %q", r.Msg, "first")
	}

	logger.Info("second")
	if !expectFail(func(sub logtest.TestingT) { sink.OnlyRecord(sub) }) {
		t.Errorf("OnlyRecord must fail when more than one record was captured")
	}
}

func TestSink_RecordsAtOrAboveLevel(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.Debug("below")
	logger.Warn("at")
	logger.Error("above")

	got := sink.RecordsAtOrAboveLevel(slog.LevelWarn)
	if len(got) != 2 {
		t.Fatalf("RecordsAtOrAboveLevel(WARN) returned %d records, want 2: %+v", len(got), got)
	}
	if got[0].Msg != "at" || got[1].Msg != "above" {
		t.Errorf("RecordsAtOrAboveLevel(WARN) messages = [%q %q], want [\"at\" \"above\"]", got[0].Msg, got[1].Msg)
	}

	if none := sink.RecordsAtOrAboveLevel(slog.LevelError + 1); none != nil {
		t.Errorf("RecordsAtOrAboveLevel above every captured level = %+v, want nil", none)
	}
}

func TestSink_RecordsAtExactLevelExcludesHigherLevels(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.Debug("below")
	logger.Warn("at")
	logger.Error("above")

	got := sink.RecordsAtExactLevel(slog.LevelWarn)
	if len(got) != 1 {
		t.Fatalf("RecordsAtExactLevel(WARN) returned %d records, want 1: %+v", len(got), got)
	}
	if got[0].Msg != "at" {
		t.Errorf("RecordsAtExactLevel(WARN) message = %q, want %q", got[0].Msg, "at")
	}

	if none := sink.RecordsAtExactLevel(slog.LevelInfo); none != nil {
		t.Errorf("RecordsAtExactLevel(INFO) = %+v, want nil", none)
	}
}

func TestSink_RecordsWithFiltersOnComponentAndMessage(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	hooks := logger.With("component", "hooks")
	hooks.Info("clean-stale")
	hooks.Info("clean-stale")
	hooks.Info("set")
	logger.With("component", "projects").Info("clean-stale")
	logger.Info("clean-stale")

	got := sink.RecordsWith("hooks", "clean-stale")
	if len(got) != 2 {
		t.Fatalf("RecordsWith(hooks, clean-stale) returned %d records, want 2: %+v", len(got), got)
	}

	if none := sink.RecordsWith("hooks", "nothing"); none != nil {
		t.Errorf("RecordsWith on an unemitted message = %+v, want nil", none)
	}
}

func TestSink_OnlyRecordWith(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.With("component", "hooks").Info("set", "op", "set")
	logger.With("component", "projects").Info("set", "op", "set")

	rec := sink.OnlyRecordWith(t, "hooks", "set")
	if got := rec.AttrString(t, "component"); got != "hooks" {
		t.Errorf("component = %q, want %q", got, "hooks")
	}

	if !expectFail(func(sub logtest.TestingT) { sink.OnlyRecordWith(sub, "hooks", "rm") }) {
		t.Errorf("OnlyRecordWith must fail when no record matches")
	}

	logger.With("component", "hooks").Info("set", "op", "set")
	if !expectFail(func(sub logtest.TestingT) { sink.OnlyRecordWith(sub, "hooks", "set") }) {
		t.Errorf("OnlyRecordWith must fail when more than one record matches")
	}
}

func TestRecords_FilterChainCombinesLevelAndComponent(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	logger.With("component", "clean").Warn("orphan killed")
	logger.With("component", "clean").Error("orphan killed")
	logger.With("component", "bootstrap").Warn("orphan killed")

	got := sink.RecordsWith("clean", "orphan killed").AtExactLevel(slog.LevelWarn)
	if len(got) != 1 {
		t.Fatalf("chained filter returned %d records, want 1: %+v", len(got), got)
	}
	if got[0].Level != slog.LevelWarn {
		t.Errorf("Level = %v, want WARN", got[0].Level)
	}
}

type fakeT struct {
	failed bool
	errors int
}

func (f *fakeT) Helper() {}

func (f *fakeT) Errorf(string, ...any) {
	f.errors++
}

func (f *fakeT) Fatalf(string, ...any) {
	f.failed = true
	panic(fakeFatal{})
}

type fakeFatal struct{}

func expectFail(fn func(logtest.TestingT)) (failed bool) {
	t := &fakeT{}
	defer func() {
		_ = recover()
		failed = t.failed
	}()
	fn(t)
	return t.failed
}
