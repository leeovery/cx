package state_test

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/statetest"
)

func TestSignalHydrateRetryDelays_MatchesSpecLadder(t *testing.T) {
	want := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		40 * time.Millisecond,
		80 * time.Millisecond,
		160 * time.Millisecond,
		190 * time.Millisecond,
	}
	if !reflect.DeepEqual([]time.Duration(state.SignalHydrateRetryDelays), want) {
		t.Errorf("SignalHydrateRetryDelays = %v, want %v", state.SignalHydrateRetryDelays, want)
	}
}

func TestSignalHydrateRetryDelays_CumulativeBudget500ms(t *testing.T) {
	var total time.Duration
	for _, d := range state.SignalHydrateRetryDelays {
		total += d
	}
	const want = 500 * time.Millisecond
	if total != want {
		t.Errorf("cumulative SignalHydrateRetryDelays = %v, want %v", total, want)
	}
}

func TestWriteFIFOSignal_WritesOneByteOnFirstTrySuccess(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	openCalls := 0
	open := func(_ string) (*os.File, error) {
		openCalls++
		return w, nil
	}
	sleep := &statetest.RecordingSleep{}

	if err := state.WriteFIFOSignal("/tmp/example.fifo", open, sleep.Fn()); err != nil {
		t.Fatalf("WriteFIFOSignal: %v", err)
	}
	if openCalls != 1 {
		t.Errorf("OpenFIFO calls = %d, want 1", openCalls)
	}
	if len(sleep.Durations) != 0 {
		t.Errorf("Sleep called %v times on first-try success, want 0", len(sleep.Durations))
	}

	_ = w.Close()
	buf := make([]byte, 8)
	n, _ := r.Read(buf)
	if n != 1 {
		t.Errorf("read %d bytes, want 1", n)
	}
}

func TestWriteFIFOSignal_RetriesOnENXIOPerLadder(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	openCalls := 0
	open := func(_ string) (*os.File, error) {
		openCalls++
		switch openCalls {
		case 1, 2:
			return nil, syscall.ENXIO
		default:
			return w, nil
		}
	}
	sleep := &statetest.RecordingSleep{}

	if err := state.WriteFIFOSignal("/tmp/example.fifo", open, sleep.Fn()); err != nil {
		t.Fatalf("WriteFIFOSignal: %v", err)
	}
	if openCalls != 3 {
		t.Errorf("OpenFIFO calls = %d, want 3", openCalls)
	}
	want := []time.Duration{
		state.SignalHydrateRetryDelays[0],
		state.SignalHydrateRetryDelays[1],
	}
	if !reflect.DeepEqual(sleep.Durations, want) {
		t.Errorf("Sleep durations = %v, want %v", sleep.Durations, want)
	}
}

func TestWriteFIFOSignal_EmitsRetryDebugUnderSignal(t *testing.T) {
	sink := logtest.Install(t)

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	openCalls := 0
	open := func(_ string) (*os.File, error) {
		openCalls++
		if openCalls == 1 {
			return nil, syscall.ENXIO
		}
		return w, nil
	}
	sleep := &statetest.RecordingSleep{}

	const path = "/tmp/example.fifo"
	if err := state.WriteFIFOSignal(path, open, sleep.Fn()); err != nil {
		t.Fatalf("WriteFIFOSignal: %v", err)
	}

	dbg := sink.RecordsAtExactLevelWith(slog.LevelDebug, "signal", "fifo signal retrying").
		Only(t, "DEBUG 'fifo signal retrying' under component=signal (one retryable transition)")
	if p := dbg.AttrString(t, "path"); p != path {
		t.Errorf("retry DEBUG path attr = %q; want %q", p, path)
	}
	if kind := dbg.Attrs["error"].Kind(); kind != slog.KindAny {
		t.Errorf("retry DEBUG error attr kind = %v; want Any (wrapped err passed directly)", kind)
	}
	if gotErr := dbg.ErrorAttr(t, "error"); !errors.Is(gotErr, syscall.ENXIO) {
		t.Errorf("retry DEBUG error attr = %v; want errors.Is(err, ENXIO)=true", gotErr)
	}
}

func TestWriteFIFOSignal_RetryDebugOncePerRetryTransition(t *testing.T) {
	sink := logtest.Install(t)

	open := func(_ string) (*os.File, error) { return nil, syscall.ENXIO }
	sleep := &statetest.RecordingSleep{}

	const path = "/tmp/never-ready.fifo"
	if err := state.WriteFIFOSignal(path, open, sleep.Fn()); err == nil {
		t.Fatalf("expected retry-exhaustion error, got nil")
	}

	dbg := sink.RecordsAtExactLevelWith(slog.LevelDebug, "signal", "fifo signal retrying")
	if len(dbg) != len(state.SignalHydrateRetryDelays) {
		t.Errorf("retry DEBUG count = %d; want %d (one per sleep+retry transition)", len(dbg), len(state.SignalHydrateRetryDelays))
	}
}

func TestWriteFIFOSignal_RetriesOnEAGAINPerLadder(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	t.Cleanup(func() {
		_ = r.Close()
		_ = w.Close()
	})

	openCalls := 0
	open := func(_ string) (*os.File, error) {
		openCalls++
		if openCalls == 1 {
			return nil, syscall.EAGAIN
		}
		return w, nil
	}
	sleep := &statetest.RecordingSleep{}

	if err := state.WriteFIFOSignal("/tmp/example.fifo", open, sleep.Fn()); err != nil {
		t.Fatalf("WriteFIFOSignal: %v", err)
	}
	if openCalls != 2 {
		t.Errorf("OpenFIFO calls = %d, want 2", openCalls)
	}
	want := []time.Duration{state.SignalHydrateRetryDelays[0]}
	if !reflect.DeepEqual(sleep.Durations, want) {
		t.Errorf("Sleep durations = %v, want %v", sleep.Durations, want)
	}
}

func TestWriteFIFOSignal_ENOENTReturnsImmediatelyWithOpenFifoWrap(t *testing.T) {
	openCalls := 0
	open := func(_ string) (*os.File, error) {
		openCalls++
		return nil, syscall.ENOENT
	}
	sleep := &statetest.RecordingSleep{}

	const path = "/tmp/missing.fifo"
	err := state.WriteFIFOSignal(path, open, sleep.Fn())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if openCalls != 1 {
		t.Errorf("OpenFIFO calls = %d, want 1 (no retry on ENOENT)", openCalls)
	}
	if len(sleep.Durations) != 0 {
		t.Errorf("Sleep called %d times on ENOENT, want 0", len(sleep.Durations))
	}
	if !errors.Is(err, syscall.ENOENT) {
		t.Errorf("err does not wrap syscall.ENOENT: %v", err)
	}
	wantPrefix := fmt.Sprintf("open fifo %s:", path)
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("err %q does not start with %q", err.Error(), wantPrefix)
	}
}

func TestWriteFIFOSignal_NonRetryableErrorReturnsImmediately(t *testing.T) {
	sentinel := errors.New("permission denied (sentinel)")
	openCalls := 0
	open := func(_ string) (*os.File, error) {
		openCalls++
		return nil, sentinel
	}
	sleep := &statetest.RecordingSleep{}

	const path = "/tmp/forbidden.fifo"
	err := state.WriteFIFOSignal(path, open, sleep.Fn())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if openCalls != 1 {
		t.Errorf("OpenFIFO calls = %d, want 1 (no retry on non-retryable err)", openCalls)
	}
	if len(sleep.Durations) != 0 {
		t.Errorf("Sleep called %d times on non-retryable err, want 0", len(sleep.Durations))
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("err does not wrap sentinel: %v", err)
	}
	wantPrefix := fmt.Sprintf("open fifo %s:", path)
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("err %q does not start with %q", err.Error(), wantPrefix)
	}
}

func TestWriteFIFOSignal_RetryExhaustionWrapsLastErrWithRetriesExhausted(t *testing.T) {
	openCalls := 0
	open := func(_ string) (*os.File, error) {
		openCalls++
		return nil, syscall.ENXIO
	}
	sleep := &statetest.RecordingSleep{}

	const path = "/tmp/never-ready.fifo"
	err := state.WriteFIFOSignal(path, open, sleep.Fn())
	if err == nil {
		t.Fatalf("expected error, got nil")
	}

	wantOpens := 1 + len(state.SignalHydrateRetryDelays)
	if openCalls != wantOpens {
		t.Errorf("OpenFIFO calls = %d, want %d (initial + 6 retries)", openCalls, wantOpens)
	}
	if len(sleep.Durations) != len(state.SignalHydrateRetryDelays) {
		t.Errorf("Sleep called %d times, want %d", len(sleep.Durations), len(state.SignalHydrateRetryDelays))
	}
	if !reflect.DeepEqual(sleep.Durations, []time.Duration(state.SignalHydrateRetryDelays)) {
		t.Errorf("Sleep durations = %v, want %v", sleep.Durations, state.SignalHydrateRetryDelays)
	}

	if !errors.Is(err, syscall.ENXIO) {
		t.Errorf("retries-exhausted err does not wrap ENXIO: %v", err)
	}
	wantPrefix := fmt.Sprintf("retries exhausted opening fifo %s:", path)
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("err %q does not start with %q", err.Error(), wantPrefix)
	}
}

func TestOpenFIFOForSignal_NonBlockingFlags(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFOs are not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "no-reader.fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	start := time.Now()
	f, err := state.OpenFIFOForSignal(path)
	elapsed := time.Since(start)

	if f != nil {
		_ = f.Close()
		t.Fatal("OpenFIFOForSignal returned non-nil file with no reader; expected ENXIO")
	}
	if !errors.Is(err, syscall.ENXIO) {
		t.Fatalf("OpenFIFOForSignal err = %v, want syscall.ENXIO", err)
	}
	if elapsed >= 100*time.Millisecond {
		t.Errorf("OpenFIFOForSignal blocked for %v; expected ~immediate return (O_NONBLOCK missing?)", elapsed)
	}
}

func TestSendHydrateSignal_WritesOneByteToReadyFIFO(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFOs are not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "ready.fifo")
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("mkfifo: %v", err)
	}

	// A FIFO supports no read deadline, so the blocking reader needs a goroutine
	// and a channel timeout rather than SetReadDeadline.
	type readResult struct {
		n   int
		err error
	}
	readDone := make(chan readResult, 1)
	go func() {
		reader, openErr := os.OpenFile(path, os.O_RDONLY, 0)
		if openErr != nil {
			readDone <- readResult{err: openErr}
			return
		}
		defer func() { _ = reader.Close() }()
		buf := make([]byte, 8)
		n, err := reader.Read(buf)
		readDone <- readResult{n: n, err: err}
	}()

	if err := state.SendHydrateSignal(path); err != nil {
		t.Fatalf("SendHydrateSignal: %v", err)
	}

	select {
	case r := <-readDone:
		if r.err != nil {
			t.Fatalf("reader goroutine: %v", r.err)
		}
		if r.n != 1 {
			t.Errorf("read %d bytes, want 1", r.n)
		}
	case <-time.After(time.Second):
		t.Fatal("reader goroutine did not receive byte within 1s")
	}
}

func TestSendHydrateSignal_PropagatesNonRetryableError(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFOs are not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.fifo")
	// Deliberately no mkfifo: the open must surface ENOENT.

	err := state.SendHydrateSignal(path)
	if err == nil {
		t.Fatal("SendHydrateSignal returned nil; want ENOENT-wrapped error")
	}
	if !errors.Is(err, syscall.ENOENT) {
		t.Errorf("SendHydrateSignal err = %v; want errors.Is(err, syscall.ENOENT)=true", err)
	}
	wantPrefix := fmt.Sprintf("open fifo %s:", path)
	if !strings.HasPrefix(err.Error(), wantPrefix) {
		t.Errorf("err %q does not start with %q", err.Error(), wantPrefix)
	}
}

func TestDefaultFIFOSignaler_SendSignalDelegatesToSendHydrateSignal(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("FIFOs are not supported on Windows")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "missing.fifo")
	// Deliberately no mkfifo: both calls must surface the same wrapped ENOENT.

	directErr := state.SendHydrateSignal(path)
	adapterErr := state.DefaultFIFOSignaler{}.SendSignal(path)

	if directErr == nil || adapterErr == nil {
		t.Fatalf("expected non-nil errors; directErr=%v adapterErr=%v", directErr, adapterErr)
	}
	if !errors.Is(adapterErr, syscall.ENOENT) {
		t.Errorf("adapterErr = %v; want errors.Is(adapterErr, syscall.ENOENT)=true", adapterErr)
	}
	if directErr.Error() != adapterErr.Error() {
		t.Errorf("DefaultFIFOSignaler.SendSignal err %q diverges from state.SendHydrateSignal err %q",
			adapterErr.Error(), directErr.Error())
	}
}
