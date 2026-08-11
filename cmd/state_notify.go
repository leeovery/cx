package cmd

import (
	"fmt"

	"github.com/leeovery/portal/internal/state"
	"github.com/spf13/cobra"
)

// stateNotifyCmd must stay trivially fast and side-effect minimal — tmux
// invokes it from hook contexts on every structural event — so it makes no tmux
// calls and reads no state files beyond the marker it touches.
var stateNotifyCmd = &cobra.Command{
	Use:    "notify",
	Short:  "Bump the save-requested marker (internal, invoked by tmux hooks)",
	Args:   cobra.NoArgs,
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := state.EnsureDir()
		if err != nil {
			// Pre-logger: with no state dir there is nowhere to open portal.log, so
			// cobra printing the wrapped error to stderr is the only channel.
			return fmt.Errorf("ensure state dir: %w", err)
		}

		if err := state.TouchSaveRequested(dir); err != nil {
			notifyLogger.Warn("touch save.requested failed", "path", state.SaveRequested(dir), "error", err)
			return fmt.Errorf("notify: %w", err)
		}
		return nil
	},
}

func init() {
	stateCmd.AddCommand(stateNotifyCmd)
}
