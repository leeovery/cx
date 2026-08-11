package bootstrapadapter

import (
	"log/slog"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// NewOrphanSweeper builds the production orphan sweeper, leaving the identify
// and kill seams at their defaults. client must be non-nil; logger may be nil.
func NewOrphanSweeper(client *tmux.Client, logger *slog.Logger) bootstrap.OrphanSweeper {
	return &bootstrap.OrphanSweepCore{
		Pgrep: state.PgrepPortalDaemons,
		SaverPanePID: func() (pid int, present bool, err error) {
			return tmux.SaverPanePIDOrAbsent(client, tmux.PortalSaverName)
		},
		Logger: logger,
	}
}
