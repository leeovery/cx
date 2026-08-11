package bootstrap

import (
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
)

type latchCall struct {
	name  string
	value string
}

type recordingLatch struct {
	calls []latchCall
	err   error
	seq   *[]string
}

func (l *recordingLatch) SetServerOption(name, value string) error {
	l.calls = append(l.calls, latchCall{name: name, value: value})
	if l.seq != nil {
		*l.seq = append(*l.seq, "latch")
	}
	return l.err
}

var _ LatchWriter = (*recordingLatch)(nil)

type orchestrationSeqHandler struct {
	seq *[]string
}

func (h *orchestrationSeqHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *orchestrationSeqHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *orchestrationSeqHandler) WithGroup(string) slog.Handler            { return h }
func (h *orchestrationSeqHandler) Handle(_ context.Context, r slog.Record) error {
	if r.Message == "orchestration complete" {
		*h.seq = append(*h.seq, "log:orchestration complete")
	}
	return nil
}

func TestOrchestratorRun_stampsLatchWithVersionAfterSoftWarning(t *testing.T) {
	r := &stepRecorder{EnsureSaverErr: errors.New("saver boom")}
	latch := &recordingLatch{}
	o := newOrchestrator(r, nil)
	o.Latch = latch
	o.Version = "v1.2.3"

	_, warnings, err := o.Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(latch.calls) != 1 {
		t.Fatalf("latch SetServerOption call count = %d, want 1; calls=%+v", len(latch.calls), latch.calls)
	}
	if latch.calls[0].name != state.BootstrappedMarkerName {
		t.Errorf("latch name = %q, want %q", latch.calls[0].name, state.BootstrappedMarkerName)
	}
	if latch.calls[0].value != "v1.2.3" {
		t.Errorf("latch value = %q, want %q", latch.calls[0].value, "v1.2.3")
	}

	if len(warnings) != 1 {
		t.Fatalf("warnings len = %d, want 1; got %#v", len(warnings), warnings)
	}
	if warnings[0].Lines[0] != SaverDownWarning().Lines[0] {
		t.Errorf("warnings[0] = %q, want SaverDownWarning", warnings[0].Lines[0])
	}
}

func TestOrchestratorRun_doesNotStampLatchOnFatalAbort(t *testing.T) {
	r := &stepRecorder{SetErr: errors.New("set marker boom")}
	latch := &recordingLatch{}
	o := newOrchestrator(r, nil)
	o.Latch = latch
	o.Version = "v1.2.3"

	_, _, err := o.Run(context.Background())

	var fatal *FatalError
	if !errors.As(err, &fatal) {
		t.Fatalf("expected *FatalError from a fatal-step abort, got %T (%v)", err, err)
	}
	if len(latch.calls) != 0 {
		t.Errorf("latch must not be written on a fatal abort; got %+v", latch.calls)
	}
}

func TestOrchestratorRun_swallowsLatchWriteFailureAsWarn(t *testing.T) {
	r := &stepRecorder{}
	latch := &recordingLatch{err: errors.New("latch write boom")}

	sink := &logtest.Sink{}
	logger := slog.New(sink).With("component", "bootstrap")

	var events []StepEvent
	ctx := WithProgressEmitter(context.Background(), func(ev StepEvent) {
		events = append(events, ev)
	})

	o := newOrchestrator(r, logger)
	o.Latch = latch
	o.Version = "v9.9.9"

	_, warnings, err := o.Run(ctx)
	if err != nil {
		t.Fatalf("latch-write failure must not be fatal; got %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("latch-write failure must NOT append a warning; got %#v", warnings)
	}

	stepCompletes := 0
	for _, rec := range sink.Records() {
		if rec.Msg == "step complete" {
			stepCompletes++
		}
	}
	if stepCompletes == 0 {
		t.Fatal("expected at least one 'step complete' record")
	}
	if len(events) != stepCompletes {
		t.Errorf("emitted %d StepEvents but %d steps completed — the latch write must emit no event; events=%+v",
			len(events), stepCompletes, events)
	}

	foundWarn := false
	for _, rec := range sink.Records() {
		if rec.Level != slog.LevelWarn {
			continue
		}
		comp, ok := rec.Attrs["component"]
		if !ok || comp.String() != "bootstrap" {
			continue
		}
		if !strings.Contains(rec.Msg, state.BootstrappedMarkerName) {
			continue
		}
		if _, hasMarker := rec.Attrs["marker"]; hasMarker {
			t.Errorf("latch-write WARN must not carry a non-vocabulary \"marker\" attr; rec=%+v", rec)
		}
		foundWarn = true
		break
	}
	if !foundWarn {
		t.Errorf("expected a WARN under component=bootstrap whose message names %q; records=%+v",
			state.BootstrappedMarkerName, sink.Records())
	}
}

func TestOrchestratorRun_stampsLatchBeforeOrchestrationComplete(t *testing.T) {
	r := &stepRecorder{}
	var seq []string
	latch := &recordingLatch{seq: &seq}
	logger := slog.New(&orchestrationSeqHandler{seq: &seq})

	o := newOrchestrator(r, logger)
	o.Latch = latch
	o.Version = "v1.2.3"

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	if len(latch.calls) != 1 {
		t.Fatalf("latch call count = %d, want exactly 1; calls=%+v", len(latch.calls), latch.calls)
	}

	latchIdx, completeIdx := -1, -1
	for i, ev := range seq {
		switch ev {
		case "latch":
			latchIdx = i
		case "log:orchestration complete":
			completeIdx = i
		}
	}
	if latchIdx == -1 {
		t.Fatalf("latch marker not recorded; seq=%v", seq)
	}
	if completeIdx == -1 {
		t.Fatalf("orchestration-complete marker not recorded; seq=%v", seq)
	}
	if latchIdx >= completeIdx {
		t.Errorf("latch write must precede the orchestration-complete summary; latchIdx=%d completeIdx=%d seq=%v",
			latchIdx, completeIdx, seq)
	}
}
