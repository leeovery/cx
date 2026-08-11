package state

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/leeovery/portal/internal/log"
)

var cleanLogger = log.For("clean")

// SweepOrphanFIFOs removes hydrate-*.fifo files in dir whose paneKey is absent
// from liveMarkerKeys, which must hold bare paneKeys without the
// @portal-skeleton- prefix. Glob matches that are not FIFOs are preserved,
// per-file errors are logged and skipped rather than aborting the sweep, and a
// missing dir returns nil so callers may sweep unconditionally.
//
// The logging split is deliberate — do not consolidate it: per-item warnings go
// to callerLogger so they carry the calling step's component, while the cycle
// summary and reaped breadcrumbs belong to the clean component.
func SweepOrphanFIFOs(dir string, liveMarkerKeys map[string]struct{}, callerLogger *slog.Logger) error {
	callerLogger = loggerOrDiscard(callerLogger)
	start := time.Now()
	var reaped, skipped int
	pattern := filepath.Join(dir, "hydrate-*.fifo")
	matches, err := filepath.Glob(pattern)
	if err != nil {
		return fmt.Errorf("glob fifos in %s: %w", dir, err)
	}

	for _, path := range matches {
		fi, err := os.Lstat(path)
		if err != nil {
			callerLogger.Warn("orphan fifo lstat failed", "path", path, "error", err)
			skipped++
			continue
		}
		if fi.Mode()&os.ModeNamedPipe == 0 {
			skipped++
			continue
		}
		paneKey := PaneKeyFromFIFOPath(path)
		if _, alive := liveMarkerKeys[paneKey]; alive {
			skipped++
			continue
		}
		if err := os.Remove(path); err != nil {
			callerLogger.Warn("remove orphan fifo failed", "path", path, "error", err)
			skipped++
			continue
		}
		reaped++
		cleanLogger.Debug("orphan fifo reaped", "path", path)
	}
	cleanLogger.Info("orphan-fifo sweep complete", "reaped", reaped, "skipped", skipped, log.Took(start))
	return nil
}
