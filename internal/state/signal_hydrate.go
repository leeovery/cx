package state

import (
	"errors"
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/leeovery/portal/internal/log"
)

// Deliberately not named `logger`: that is a pervasive function-parameter name
// across this package and would shadow the binding.
var signalLogger = log.For("signal")

// SignalHydrateRetryDelays is the back-off ladder used when the per-pane FIFO
// has no reader yet. The cumulative budget is 500ms.
var SignalHydrateRetryDelays = []time.Duration{
	10 * time.Millisecond,
	20 * time.Millisecond,
	40 * time.Millisecond,
	80 * time.Millisecond,
	160 * time.Millisecond,
	190 * time.Millisecond,
}

// OpenFIFOForSignal opens path O_WRONLY|O_NONBLOCK, so a missing reader
// surfaces as ENXIO immediately rather than blocking the tmux server —
// signal-hydrate runs under run-shell, which is synchronous.
func OpenFIFOForSignal(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
}

// WriteFIFOSignal opens the per-pane FIFO non-blocking and writes a single
// byte, retrying ENXIO and EAGAIN per SignalHydrateRetryDelays and returning
// any other error immediately. Retry exhaustion is a soft failure returned as
// a wrapped error: the marker stays set and the next attach path re-signals.
// openFIFO and sleep are seams; production callers pass OpenFIFOForSignal and
// time.Sleep.
func WriteFIFOSignal(path string, openFIFO func(string) (*os.File, error), sleep func(time.Duration)) error {
	var lastErr error
	for i := 0; i <= len(SignalHydrateRetryDelays); i++ {
		f, err := openFIFO(path)
		if err == nil {
			if _, werr := f.Write([]byte{1}); werr != nil {
				_ = f.Close()
				return fmt.Errorf("write byte to %s: %w", path, werr)
			}
			_ = f.Close()
			return nil
		}

		if !isRetryableFIFOError(err) {
			return fmt.Errorf("open fifo %s: %w", path, err)
		}

		lastErr = err
		if i < len(SignalHydrateRetryDelays) {
			signalLogger.Debug("fifo signal retrying", "path", path, "error", err)
			sleep(SignalHydrateRetryDelays[i])
		}
	}
	return fmt.Errorf("retries exhausted opening fifo %s: %w", path, lastErr)
}

func isRetryableFIFOError(err error) bool {
	return errors.Is(err, syscall.ENXIO) || errors.Is(err, syscall.EAGAIN)
}

// SendHydrateSignal writes the hydrate signal byte to the FIFO at path with
// the production seams pinned.
func SendHydrateSignal(path string) error {
	return WriteFIFOSignal(path, OpenFIFOForSignal, time.Sleep)
}

// FIFOSignaler is the per-pane FIFO-write seam. SendSignal must be safe to
// invoke from a tmux-hook context — it must not block the server even when the
// pane helper has not yet opened its read end.
type FIFOSignaler interface {
	SendSignal(path string) error
}

// DefaultFIFOSignaler is the production FIFOSignaler; its zero value is ready
// to use.
type DefaultFIFOSignaler struct{}

// SendSignal writes the hydrate signal byte to the FIFO at path.
func (DefaultFIFOSignaler) SendSignal(path string) error { return SendHydrateSignal(path) }
