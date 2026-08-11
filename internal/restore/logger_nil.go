package restore

import (
	"log/slog"

	"github.com/leeovery/portal/internal/log"
)

// A nil *slog.Logger panics on use, so these forwarders let callers and tests
// leave Logger unset.

func (o *Orchestrator) logger() *slog.Logger {
	return log.OrDiscard(o.Logger)
}

func (r *SessionRestorer) logger() *slog.Logger {
	return log.OrDiscard(r.Logger)
}
