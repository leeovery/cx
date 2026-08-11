package spawn

import (
	"fmt"
	"log/slog"

	"github.com/leeovery/portal/internal/log"
)

// LogWindowResults emits one record per external window, nil-logger tolerant. A
// failed window emits at WARN unless its outcome is permission-required, which
// LogPermission already reports — the exclusion prevents a double-report.
func LogWindowResults(logger *slog.Logger, results []WindowResult) {
	logger = log.OrDiscard(logger)
	for _, r := range results {
		failed := !r.Confirmed()
		nonPermission := r.Result.Outcome != OutcomePermissionRequired
		if failed && nonPermission {
			logger.Warn("external window failed", "session", r.Session, "ack", string(r.Ack), "detail", r.Result.Detail)
			continue
		}
		logger.Debug("external window", "session", r.Session, "ack", string(r.Ack), "detail", r.Result.Detail)
	}
}

// LogBatchSummary emits the per-window records followed by one INFO cycle
// summary. total is passed in rather than derived from len(results): a pre-spawn
// abort or a cancelled burst leaves fewer results than external windows.
func LogBatchSummary(logger *slog.Logger, id Identity, resolution Resolution, results []WindowResult, total int, triggerAttached bool, batch string) {
	logger = log.OrDiscard(logger)
	LogWindowResults(logger, results)

	confirmed, _ := PartitionResults(results)
	opened := len(confirmed)
	if triggerAttached {
		opened++
	}

	logger.Info(fmt.Sprintf("opened %d/%d", opened, total),
		"resolution", string(resolution),
		"terminal", id.Name,
		"bundle_id", id.BundleID,
		"opened", opened,
		"total", total,
		"batch", batch,
	)
}

// LogTriggerConnectFailed records a trigger self-connect that failed after the
// batch summary was emitted. The summary has to precede the self-connect — a
// successful outside-tmux attach exec-replaces the process and never returns —
// so it counts the trigger optimistically and this WARN corrects the record.
func LogTriggerConnectFailed(logger *slog.Logger, session, detail string) {
	log.OrDiscard(logger).Warn("trigger did not attach", "session", session, "detail", detail)
}

// LogPermission emits the permission-required outcome line. The burst stopped at
// the first wall, so it carries no cycle-summary attrs.
func LogPermission(logger *slog.Logger, id Identity, resolution Resolution, detail string) {
	log.OrDiscard(logger).Info("permission required — nothing self-attached",
		"resolution", string(resolution),
		"terminal", id.Name,
		"bundle_id", id.BundleID,
		"detail", detail,
	)
}

// LogUnsupported emits the outcome line for an atomic no-op on an unsupported or
// NULL terminal. Nothing was attempted, so there are no per-window records.
func LogUnsupported(logger *slog.Logger, id Identity) {
	log.OrDiscard(logger).Info("unsupported terminal — nothing opened",
		"resolution", string(ResolutionUnsupported),
		"terminal", id.Name,
		"bundle_id", id.BundleID,
	)
}

// LogGone emits the single outcome line for a pre-flight abort, naming the gone
// sessions. Detection never ran, so it carries no resolution or count attrs.
func LogGone(logger *slog.Logger, gone []string) {
	log.OrDiscard(logger).Info(GoneMessage(gone))
}
