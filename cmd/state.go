package cmd

import "github.com/spf13/cobra"

// Hidden is visibility-only: `state` and its children stay argv-invocable, which
// the daemon and hydrate helpers depend on. Do not rename them — tmux idempotency
// matchers and PortalDaemonArgvPattern match the literal `state …` argv.
var stateCmd = &cobra.Command{
	Use:    "state",
	Short:  "Manage Portal session resurrection state",
	Hidden: true,
}

func init() {
	rootCmd.AddCommand(stateCmd)
}
