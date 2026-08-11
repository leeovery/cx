package bootstrap

import (
	"log/slog"
	"os"
	"syscall"
	"time"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/state"
)

// SIGKILL, not SIGTERM: an orphan daemon's view of the world is untrusted, so
// it must not get the chance to flush a final state on the way out.
func defaultKill(pid int) error {
	return syscall.Kill(pid, syscall.SIGKILL)
}

// OrphanSweeper kills leftover `portal state daemon` processes. Best-effort:
// SweepOrphanDaemons logs and swallows every failure and always returns nil.
type OrphanSweeper interface {
	SweepOrphanDaemons() error
}

// OrphanSweepCore SIGKILLs every identity-checked `portal state daemon` other
// than the `_portal-saver` pane's process. Pgrep and SaverPanePID must be
// supplied; Identify and Kill fall back to production defaults.
type OrphanSweepCore struct {
	// No default: shelling out would pull os/exec into this pure-orchestration
	// package, so production wiring must supply it.
	Pgrep func() ([]int, error)

	// Absence rides the present bool rather than pid == 0, so a defensive zero
	// PID on a non-error path cannot become a legitimate PID.
	SaverPanePID func() (pid int, present bool, err error)

	Identify func(pid int) (state.IdentifyResult, error)
	Kill     func(pid int) error
	Logger   *slog.Logger
}

var _ OrphanSweeper = (*OrphanSweepCore)(nil)

func (c *OrphanSweepCore) SweepOrphanDaemons() error {
	logger := log.OrDiscard(c.Logger)
	identify := c.Identify
	if identify == nil {
		identify = state.IdentifyDaemon
	}
	kill := c.Kill
	if kill == nil {
		kill = defaultKill
	}

	start := time.Now()
	var killed int

	candidates, err := c.Pgrep()
	if err != nil {
		logger.Warn("sweep: pgrep failed", "error", err)
		return nil
	}

	legitimate := map[int]struct{}{}
	saverPID, saverPresent, saverErr := c.SaverPanePID()
	switch {
	case saverErr != nil:
		logger.Warn("sweep: list-panes _portal-saver failed, legitimate set empty", "error", saverErr)
	case !saverPresent:
		// Deliberately silent: an absent saver just leaves the set empty.
	default:
		legitimate[saverPID] = struct{}{}
	}

	ownPID := os.Getpid()
	for _, pid := range candidates {
		if pid == ownPID {
			// Unreachable in practice, but the cost of a false positive here
			// is killing ourselves.
			continue
		}
		if _, isLegit := legitimate[pid]; isLegit {
			continue
		}
		res, err := identify(pid)
		if err != nil {
			logger.Warn("sweep: identity-check failed, skipping", "target_pid", pid, "error", err)
			continue
		}
		if res != state.IdentifyIsPortalDaemon {
			logger.Debug("sweep: pid not identity-checked as portal daemon, skipping", "target_pid", pid)
			continue
		}
		if err := kill(pid); err != nil {
			logger.Warn("sweep: kill failed", "target_pid", pid, "error", err)
			continue
		}
		cleanLogger.Debug("orphan killed", "target_pid", pid)
		killed++
	}
	cleanLogger.Info("orphan-daemon sweep complete", "killed", killed, log.Took(start))
	return nil
}
