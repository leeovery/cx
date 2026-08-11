// Package storelog owns the shared clean-stale batch-summary emission for the
// JSON-backed stores, so their batch-summary contract cannot drift.
//
// It exists as its own package because the emission needs both internal/log and
// internal/fileutil, and neither may import the other: log must stay a leaf, and
// fileutil must stay free of logging. This is the thin composition point that
// can import both.
package storelog

import (
	"log/slog"
	"time"

	"github.com/leeovery/portal/internal/fileutil"
	"github.com/leeovery/portal/internal/log"
)

// EmitCleanStaleSummary emits the terminal batch-summary breadcrumb for a
// store's CleanStale: INFO when saveErr is nil, WARN carrying the classified
// error otherwise. The caller owns everything before it — the partition, the
// zero-removal early return, and the per-entry lines.
func EmitCleanStaleSummary(logger *slog.Logger, removed int, start time.Time, saveErr error) {
	if saveErr != nil {
		logger.Warn("clean-stale", "op", "clean-stale", "entries", removed, "via", "internal",
			"error", saveErr, "error_class", fileutil.ClassifyWriteError(saveErr), log.Took(start))
		return
	}

	logger.Info("clean-stale", "op", "clean-stale", "entries", removed, "via", "internal", log.Took(start))
}
