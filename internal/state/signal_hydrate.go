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
// has no reader yet.
var SignalHydrateRetryDelays = []time.Duration{
	10 * time.Millisecond,
	20 * time.Millisecond,
	40 * time.Millisecond,
	80 * time.Millisecond,
	160 * time.Millisecond,
	190 * time.Millisecond,
}

// OpenFIFOForSignal opens non-blocking, so a missing reader surfaces as ENXIO
// immediately rather than blocking the tmux server: signal-hydrate runs under
// run-shell, which is synchronous.
func OpenFIFOForSignal(path string) (*os.File, error) {
	return os.OpenFile(path, os.O_WRONLY|syscall.O_NONBLOCK, 0)
}

// WriteFIFOSignal retries ENXIO and EAGAIN per SignalHydrateRetryDelays and
// returns any other error immediately. Retry exhaustion is a soft failure
// returned as a wrapped error: the marker stays set and the next attach path
// re-signals. openFIFO and sleep are seams; production passes OpenFIFOForSignal
// and time.Sleep.
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

// SendHydrateSignal is WriteFIFOSignal with the production seams pinned.
func SendHydrateSignal(path string) error {
	return WriteFIFOSignal(path, OpenFIFOForSignal, time.Sleep)
}

// FIFOSignaler's SendSignal must be safe to invoke from a tmux-hook context — it
// must not block the server even when the pane helper has no read end open yet.
type FIFOSignaler interface {
	SendSignal(path string) error
}

// DefaultFIFOSignaler's zero value is ready to use.
type DefaultFIFOSignaler struct{}

func (DefaultFIFOSignaler) SendSignal(path string) error { return SendHydrateSignal(path) }
