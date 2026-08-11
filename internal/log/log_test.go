package log

import (
	"bytes"
	"context"
	"log/slog"
	"os"
	"os/exec"
	"sync"
	"testing"
)

var initLogger = For("init-probe")

func TestFor_ReturnsNonNilBeforeInit(t *testing.T) {
	logger := For("daemon")
	if logger == nil {
		t.Fatal("For returned a nil logger before Init")
	}
}

func TestFor_BoundAtPackageInit_IsNonNil(t *testing.T) {
	if initLogger == nil {
		t.Fatal("For called at package init returned nil; package init did not construct root first")
	}
}

func TestFor_EmptyComponentReturnsValidLogger(t *testing.T) {
	logger := For("")
	if logger == nil {
		t.Fatal("For(\"\") returned a nil logger")
	}
	logger.Info("probe")
}

func TestFor_CachedLoggerRoutesToHandlerInstalledAfterSwap(t *testing.T) {
	restore := snapshotHandler()
	t.Cleanup(restore)

	cached := For("daemon")

	rec := &recordingHandler{}
	setHandler(rec)

	cached.Info("after swap")

	if len(rec.records) != 1 {
		t.Fatalf("expected 1 record routed to swapped-in handler, got %d", len(rec.records))
	}
	if got := rec.records[0].Message; got != "after swap" {
		t.Errorf("routed record message = %q, want %q", got, "after swap")
	}
}

func TestFor_RaceFreeUnderConcurrentForAndSwap(t *testing.T) {
	restore := snapshotHandler()
	t.Cleanup(restore)

	var wg sync.WaitGroup
	const goroutines = 16

	for range goroutines {
		wg.Go(func() {
			for range 100 {
				logger := For("daemon")
				logger.Info("concurrent")
			}
		})
	}
	for range goroutines {
		wg.Go(func() {
			for range 100 {
				setHandler(&recordingHandler{})
			}
		})
	}

	wg.Wait()
}

func TestDefaultHandler_DropsDebugEmitsInfoToStderr(t *testing.T) {
	if os.Getenv("PORTAL_LOG_DEFAULT_HANDLER_PROBE") == "1" {
		logger := For("daemon")
		logger.Debug("debug-line")
		logger.Info("info-line")
		return
	}

	cmd := exec.Command(os.Args[0], "-test.run", "TestDefaultHandler_DropsDebugEmitsInfoToStderr")
	cmd.Env = append(os.Environ(), "PORTAL_LOG_DEFAULT_HANDLER_PROBE=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("subprocess failed: %v\nstderr:\n%s", err, stderr.String())
	}

	out := stderr.String()
	if bytes.Contains(stderr.Bytes(), []byte("debug-line")) {
		t.Errorf("DEBUG line should be dropped by the pre-Init default handler; stderr:\n%s", out)
	}
	if !bytes.Contains(stderr.Bytes(), []byte("info-line")) {
		t.Errorf("INFO line should be emitted to stderr by the pre-Init default handler; stderr:\n%s", out)
	}
}

func snapshotHandler() func() {
	prev := swap.load()
	return func() { setHandler(prev) }
}

type recordingHandler struct {
	mu      sync.Mutex
	records []slog.Record
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }

func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.records = append(h.records, r)
	return nil
}

func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler { return h }

func (h *recordingHandler) WithGroup(string) slog.Handler { return h }
