package log

import (
	"bytes"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"
)

func recordAttrs(r slog.Record) map[string]slog.Value {
	attrs := make(map[string]slog.Value, r.NumAttrs())
	r.Attrs(func(a slog.Attr) bool {
		attrs[a.Key] = a.Value.Resolve()
		return true
	})
	return attrs
}

func exitRecords(rec *recordingHandler) []slog.Record {
	rec.mu.Lock()
	defer rec.mu.Unlock()
	var out []slog.Record
	for _, r := range rec.records {
		if r.Message == "exit" {
			out = append(out, r)
		}
	}
	return out
}

func TestClose_EmitsProcessExitWithCodeAndTook(t *testing.T) {
	snapshotInitState(t)

	rec := &recordingHandler{}
	SetTestHandler(t, rec)

	startTime = time.Now().Add(-2 * time.Second)

	Close(0)

	exits := exitRecords(rec)
	if len(exits) != 1 {
		t.Fatalf("expected exactly 1 process: exit record, got %d", len(exits))
	}
	r := exits[0]

	if r.Level != slog.LevelInfo {
		t.Errorf("process: exit must be INFO level, got %v", r.Level)
	}

	attrs := recordAttrs(r)

	codeVal, ok := attrs["code"]
	if !ok {
		t.Fatalf("process: exit record missing code attr; attrs=%v", attrs)
	}
	if got := codeVal.Int64(); got != 0 {
		t.Errorf("code attr = %d, want 0", got)
	}

	tookVal, ok := attrs["took"]
	if !ok {
		t.Fatalf("process: exit record missing took attr; attrs=%v", attrs)
	}
	if tookVal.Kind() != slog.KindDuration {
		t.Fatalf("took attr must be a duration, got kind %v", tookVal.Kind())
	}
	if tookVal.Duration() < 0 {
		t.Errorf("took attr = %v, want non-negative", tookVal.Duration())
	}
	if tookVal.Duration() < time.Second {
		t.Errorf("took attr = %v, want >= the ~2s startTime offset", tookVal.Duration())
	}
}

func TestClose_RendersPassedExitCode(t *testing.T) {
	for _, code := range []int{0, 1, 2} {
		t.Run("code="+strconv.Itoa(code), func(t *testing.T) {
			snapshotInitState(t)

			rec := &recordingHandler{}
			SetTestHandler(t, rec)

			startTime = time.Now()

			Close(code)

			exits := exitRecords(rec)
			if len(exits) != 1 {
				t.Fatalf("expected exactly 1 process: exit record, got %d", len(exits))
			}
			attrs := recordAttrs(exits[0])
			codeVal, ok := attrs["code"]
			if !ok {
				t.Fatalf("process: exit record missing code attr; attrs=%v", attrs)
			}
			if got := int(codeVal.Int64()); got != code {
				t.Errorf("code attr = %d, want %d", got, code)
			}
		})
	}
}

func TestClose_EmitsProcessComponent(t *testing.T) {
	snapshotInitState(t)

	rec := &recordingHandler{}
	SetTestHandler(t, rec)

	startTime = time.Now()

	Close(0)

	exits := exitRecords(rec)
	if len(exits) != 1 {
		t.Fatalf("expected exactly 1 process: exit record, got %d", len(exits))
	}
	if got := exits[0].Message; got != "exit" {
		t.Errorf("process exit message = %q, want %q", got, "exit")
	}
}

func TestClose_NonNegativeTookForNormalInitThenClose(t *testing.T) {
	snapshotInitState(t)

	dir := t.TempDir()
	if err := Init(dir, "0.5.0", "tui"); err != nil {
		t.Fatalf("Init returned error: %v", err)
	}

	// The swap must follow Init: Init's own setHandler would otherwise displace it.
	rec := &recordingHandler{}
	SetTestHandler(t, rec)

	Close(0)

	exits := exitRecords(rec)
	if len(exits) != 1 {
		t.Fatalf("expected exactly 1 process: exit record, got %d", len(exits))
	}
	attrs := recordAttrs(exits[0])
	tookVal, ok := attrs["took"]
	if !ok {
		t.Fatalf("process: exit record missing took attr; attrs=%v", attrs)
	}
	if tookVal.Duration() < 0 {
		t.Errorf("took for a normal Init->Close sequence = %v, want non-negative", tookVal.Duration())
	}
}

func TestClose_SafeBeforeInitEmitsBoundedTook(t *testing.T) {
	snapshotInitState(t)

	rec := &recordingHandler{}
	SetTestHandler(t, rec)

	startTime = time.Time{}

	Close(0)

	exits := exitRecords(rec)
	if len(exits) != 1 {
		t.Fatalf("expected exactly 1 process: exit record, got %d", len(exits))
	}
	attrs := recordAttrs(exits[0])
	tookVal, ok := attrs["took"]
	if !ok {
		t.Fatalf("process: exit record missing took attr; attrs=%v", attrs)
	}
	if tookVal.Duration() <= 0 {
		t.Errorf("took before Init = %v, want a large positive bounded duration", tookVal.Duration())
	}
}

func TestClose_EmitsExactlyOneExitLinePerCall(t *testing.T) {
	snapshotInitState(t)

	rec := &recordingHandler{}
	SetTestHandler(t, rec)

	startTime = time.Now()

	Close(0)
	Close(1)

	exits := exitRecords(rec)
	if len(exits) != 2 {
		t.Fatalf("expected exactly 2 process: exit records across 2 Close calls, got %d", len(exits))
	}
	if got := int(recordAttrs(exits[0])["code"].Int64()); got != 0 {
		t.Errorf("first Close code = %d, want 0", got)
	}
	if got := int(recordAttrs(exits[1])["code"].Int64()); got != 1 {
		t.Errorf("second Close code = %d, want 1", got)
	}
}

func TestClose_ExitLineVisibleAtConfiguredWarn(t *testing.T) {
	snapshotInitState(t)

	var buf bytes.Buffer
	SetTestHandler(t, newTextHandler(&buf, slog.LevelWarn, 12345, "0.5.0", "tui"))

	startTime = time.Now().Add(-2100 * time.Millisecond)

	Close(0)

	line := buf.String()
	if !strings.Contains(line, " INFO process: exit ") {
		t.Fatalf("process: exit must bypass the WARN level filter and remain INFO, got: %q", line)
	}
	if !strings.Contains(line, "code=0") {
		t.Errorf("rendered exit line missing code=0, got: %q", line)
	}
	if !strings.Contains(line, "took=") {
		t.Errorf("rendered exit line missing took=, got: %q", line)
	}
	for _, want := range []string{"pid=12345", "version=0.5.0", "process_role=tui"} {
		if !strings.Contains(line, want) {
			t.Errorf("rendered exit line missing baseline %q, got: %q", want, line)
		}
	}
	if got := strings.Count(line, "process: exit"); got != 1 {
		t.Errorf("expected exactly one rendered exit line, got %d in: %q", got, line)
	}
}
