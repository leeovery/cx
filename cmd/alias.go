package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leeovery/portal/internal/alias"
	"github.com/leeovery/portal/internal/resolver"
	"github.com/spf13/cobra"
)

var aliasCmd = &cobra.Command{
	Use:   "alias",
	Short: "Manage path aliases",
}

var aliasSetCmd = &cobra.Command{
	Use:   "set [name] [path]",
	Short: "Set a path alias",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		rawPath := args[1]

		normalised := resolver.NormalisePath(rawPath)

		aliasFile, err := aliasFilePath()
		if err != nil {
			return err
		}

		store := alias.NewStore(aliasFile)
		if _, err := store.Load(); err != nil {
			return fmt.Errorf("failed to load aliases: %w", err)
		}

		store.Set(name, normalised)

		if err := store.Save(); err != nil {
			return fmt.Errorf("failed to save aliases: %w", err)
		}

		return nil
	},
}

// aliasFilePath returns the path to the aliases file.
// Uses PORTAL_ALIASES_FILE env var if set (for testing), otherwise
// defaults to ~/.config/portal/aliases.
func aliasFilePath() (string, error) {
	if envPath := os.Getenv("PORTAL_ALIASES_FILE"); envPath != "" {
		return envPath, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine config directory: %w", err)
	}

	return filepath.Join(configDir, "portal", "aliases"), nil
}

func init() {
	aliasCmd.AddCommand(aliasSetCmd)
	rootCmd.AddCommand(aliasCmd)
}
