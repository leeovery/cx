package main

import (
	"context"
	"errors"
	"log/slog"
	"sync"
	"testing"

	"github.com/leeovery/portal/internal/log"
)

// captureHandler is an in-memory slog.Handler recording every Handle call.
// WithAttrs returns the receiver, so a bound component attr is observed at
// Handle time rather than re-bound.
type captureHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *captureHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *captureHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *captureHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *captureHandler) WithGroup(string) slog.Handler { return h }

func (h *captureHandler) messages(msg string) []slog.Record {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []slog.Record
	for _, r := range h.records {
		if r.Message == msg {
			out = append(out, r)
		}
	}
	return out
}

func attrValue(r slog.Record, key string) (slog.Value, bool) {
	var (
		v     slog.Value
		found bool
	)
	r.Attrs(func(a slog.Attr) bool {
		if a.Key == key {
			v = a.Value.Resolve()
			found = true
			return false
		}
		return true
	})
	return v, found
}

func TestRunPanicEmission(t *testing.T) {
	t.Run("it emits ERROR process: panic with reason on a recovered panic", func(t *testing.T) {
		rec := &captureHandler{}
		log.SetTestHandler(t, rec)
		withSeams(t, func() error { panic("kaboom") })

		code, panicked := run()

		if code != 2 {
			t.Errorf("code = %d, want 2", code)
		}
		if !panicked {
			t.Error("panicked = false, want true")
		}

		panics := rec.messages("panic")
		if len(panics) != 1 {
			t.Fatalf("expected exactly 1 process: panic record, got %d", len(panics))
		}
		r := panics[0]
		if r.Level != slog.LevelError {
			t.Errorf("process: panic level = %v, want ERROR", r.Level)
		}
		reason, ok := attrValue(r, "reason")
		if !ok {
			t.Fatalf("process: panic record missing reason attr")
		}
		if got := reason.String(); got != "kaboom" {
			t.Errorf("reason attr = %q, want %q", got, "kaboom")
		}
	})

	t.Run("it skips Close on the panic path so no process: exit fires", func(t *testing.T) {
		rec := &captureHandler{}
		log.SetTestHandler(t, rec)
		withSeams(t, func() error { panic("boom") })

		_, panicked := run()

		if !panicked {
			t.Fatal("panicked = false, want true (so main skips Close)")
		}
		mainEmitClose(2, panicked)

		if exits := rec.messages("exit"); len(exits) != 0 {
			t.Errorf("expected no process: exit on the panic path, got %d", len(exits))
		}
	})

	t.Run("it emits process: exit (not panic) on a clean run", func(t *testing.T) {
		rec := &captureHandler{}
		log.SetTestHandler(t, rec)
		withSeams(t, func() error { return nil })

		code, panicked := run()
		if code != 0 {
			t.Errorf("code = %d, want 0", code)
		}
		if panicked {
			t.Error("panicked = true, want false")
		}
		mainEmitClose(code, panicked)

		if panics := rec.messages("panic"); len(panics) != 0 {
			t.Errorf("expected no process: panic on a clean run, got %d", len(panics))
		}
		exits := rec.messages("exit")
		if len(exits) != 1 {
			t.Fatalf("expected exactly 1 process: exit on a clean run, got %d", len(exits))
		}
		codeVal, ok := attrValue(exits[0], "code")
		if !ok {
			t.Fatalf("process: exit record missing code attr")
		}
		if got := codeVal.Int64(); got != 0 {
			t.Errorf("exit code attr = %d, want 0", got)
		}
	})

	t.Run("it emits process: exit code=N on an error run", func(t *testing.T) {
		rec := &captureHandler{}
		log.SetTestHandler(t, rec)
		withSeams(t, func() error { return errors.New("boom") })

		code, panicked := run()
		if code != 1 {
			t.Errorf("code = %d, want 1", code)
		}
		if panicked {
			t.Error("panicked = true, want false")
		}
		mainEmitClose(code, panicked)

		if panics := rec.messages("panic"); len(panics) != 0 {
			t.Errorf("expected no process: panic on an error run, got %d", len(panics))
		}
		exits := rec.messages("exit")
		if len(exits) != 1 {
			t.Fatalf("expected exactly 1 process: exit on an error run, got %d", len(exits))
		}
		codeVal, ok := attrValue(exits[0], "code")
		if !ok {
			t.Fatalf("process: exit record missing code attr")
		}
		if got := codeVal.Int64(); got != 1 {
			t.Errorf("exit code attr = %d, want 1", got)
		}
	})

	t.Run("the four-way classification stays mutually exclusive (panic XOR exit)", func(t *testing.T) {
		cases := []struct {
			name      string
			execute   func() error
			wantPanic bool
		}{
			{"clean", func() error { return nil }, false},
			{"error", func() error { return errors.New("boom") }, false},
			{"panic", func() error { panic("x") }, true},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				rec := &captureHandler{}
				log.SetTestHandler(t, rec)
				withSeams(t, tc.execute)

				code, panicked := run()
				mainEmitClose(code, panicked)

				panics := len(rec.messages("panic"))
				exits := len(rec.messages("exit"))
				if panicked != tc.wantPanic {
					t.Fatalf("panicked = %v, want %v", panicked, tc.wantPanic)
				}
				if panics+exits != 1 {
					t.Fatalf("terminal markers panic=%d exit=%d, want exactly one total", panics, exits)
				}
				if tc.wantPanic && panics != 1 {
					t.Errorf("panic path: panic markers = %d, want 1 (exit must be skipped)", panics)
				}
				if !tc.wantPanic && exits != 1 {
					t.Errorf("non-panic path: exit markers = %d, want 1", exits)
				}
			})
		}
	})
}

// mainEmitClose models main()'s post-run !panicked gate, so the panic/exit
// mutual exclusivity can be asserted without invoking the real main, which
// calls os.Exit.
func mainEmitClose(code int, panicked bool) {
	if !panicked {
		log.Close(code)
	}
}
