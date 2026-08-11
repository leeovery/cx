package state

import (
	"log/slog"

	"github.com/leeovery/portal/internal/log"
)

func loggerOrDiscard(logger *slog.Logger) *slog.Logger {
	return log.OrDiscard(logger)
}
