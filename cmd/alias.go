package cmd

import (
	"fmt"

	"github.com/leeovery/portal/internal/alias"
	"github.com/leeovery/portal/internal/resolver"
	"github.com/leeovery/portal/internal/xdg"
	"github.com/spf13/cobra"
)

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage path aliases",
}

var aliasRmCmd = &cobra.Command{
	Use:   "rm [name]",
	Short: "Remove a path alias",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		store, err := loadAliasStore()
		if err != nil {
			return err
		}

		existed, err := store.DeleteAndSave(name, "cli")
		if !existed {
			return fmt.Errorf("alias not found: %s", name)
		}
		if err != nil {
			return fmt.Errorf("failed to save aliases: %w", err)
		}

		return nil
	},
}

var aliasListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all path aliases",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := loadAliasStore()
		if err != nil {
			return err
		}

		for _, a := range store.List() {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s=%s\n", a.Name, a.Path); err != nil {
				return err
			}
		}

		return nil
	},
}

var aliasSetCmd = &cobra.Command{
	Use:   "set [name] [path]",
	Short: "Set a path alias",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		normalised := resolver.NormalisePath(args[1])

		store, err := loadAliasStore()
		if err != nil {
			return err
		}

		if err := store.SetAndSave(name, normalised, "cli"); err != nil {
			return fmt.Errorf("failed to save aliases: %w", err)
		}

		return nil
	},
}

func loadAliasStore() (*alias.Store, error) {
	aliasFile, err := aliasFilePath()
	if err != nil {
		return nil, err
	}

	store := alias.NewStore(aliasFile)
	if _, err := store.Load(); err != nil {
		return nil, fmt.Errorf("failed to load aliases: %w", err)
	}

	return store, nil
}

func aliasFilePath() (string, error) {
	return configFilePath(xdg.AliasesFile)
}

func init() {
	aliasCmd.AddCommand(aliasSetCmd)
	aliasCmd.AddCommand(aliasRmCmd)
	aliasCmd.AddCommand(aliasListCmd)
	rootCmd.AddCommand(aliasCmd)
}
