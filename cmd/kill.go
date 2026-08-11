package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var killDeps *KillDeps

type SessionKiller interface {
	KillSession(name string) error
}

type SessionValidator interface {
	HasSession(name string) bool
}

type KillDeps struct {
	Killer    SessionKiller
	Validator SessionValidator
}

var killCmd = &cobra.Command{
	Use:   "kill [name]",
	Short: "Kill a tmux session",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		killer, validator := buildKillDeps(cmd)

		if !validator.HasSession(name) {
			return fmt.Errorf("No session found: %s", name) //nolint:staticcheck // user-facing message per spec
		}

		return killer.KillSession(name)
	},
}

func buildKillDeps(cmd *cobra.Command) (SessionKiller, SessionValidator) {
	if killDeps != nil {
		return killDeps.Killer, killDeps.Validator
	}

	client := tmuxClient(cmd)
	return client, client
}

func init() {
	// kill takes exactly one positional, so there is nothing left to complete
	// once it is present.
	killCmd.ValidArgsFunction = func(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return completeSessionNames(toComplete)
	}

	rootCmd.AddCommand(killCmd)
}
