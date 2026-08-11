package bootstrap

import (
	"log/slog"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/state"
)

// Deliberately a second component in this package: FIFO-signal outcomes are
// grepped as `signal:`, not as bootstrap's.
var signalLogger = log.For("signal")

// EagerSignalCore signals every freshly-armed `@portal-skeleton-*` pane's FIFO.
// Markers and Signaler are mandatory — a nil either panics on first dereference.
// Logger is unread; it is retained so the DI wiring matches the sibling cores.
type EagerSignalCore struct {
	Markers  state.ServerOptionLister
	StateDir string
	Signaler state.FIFOSignaler
	Logger   *slog.Logger
}

var _ EagerHydrateSignaler = (*EagerSignalCore)(nil)

// EagerSignalHydrate logs and skips per-FIFO write failures so one stuck FIFO
// cannot strand the remaining panes; only marker enumeration failures are
// returned, and never as a *FatalError.
func (c *EagerSignalCore) EagerSignalHydrate() error {
	markers, err := state.ListSkeletonMarkers(c.Markers)
	if err != nil {
		return err
	}
	if len(markers) == 0 {
		return nil
	}

	for paneKey := range markers {
		fifoPath := state.FIFOPath(c.StateDir, paneKey)
		if err := c.Signaler.SendSignal(fifoPath); err != nil {
			signalLogger.Warn("eager-signal write fifo failed", "path", fifoPath, "error", err, "error_class", "unexpected")
			continue
		}
		signalLogger.Debug("fifo signalled", "path", fifoPath)
	}
	return nil
}
