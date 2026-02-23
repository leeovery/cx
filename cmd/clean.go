package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/leeovery/portal/internal/project"
	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Remove stale projects whose directories no longer exist",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := loadProjectStore()
		if err != nil {
			return err
		}

		removed, err := store.CleanStale()
		if err != nil {
			return err
		}

		w := cmd.OutOrStdout()
		for _, p := range removed {
			if _, err := fmt.Fprintf(w, "Removed stale project: %s (%s)\n", p.Name, p.Path); err != nil {
				return err
			}
		}

		return nil
	},
}

// loadProjectStore creates a project store from the configured file path.
// Uses PORTAL_PROJECTS_FILE env var if set (for testing), otherwise
// defaults to ~/.config/portal/projects.json.
func loadProjectStore() (*project.Store, error) {
	path, err := projectsFilePath()
	if err != nil {
		return nil, err
	}
	return project.NewStore(path), nil
}

// projectsFilePath returns the path to the projects.json file.
// Uses PORTAL_PROJECTS_FILE env var if set (for testing), otherwise
// defaults to ~/.config/portal/projects.json.
func projectsFilePath() (string, error) {
	if envPath := os.Getenv("PORTAL_PROJECTS_FILE"); envPath != "" {
		return envPath, nil
	}

	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to determine config directory: %w", err)
	}

	return filepath.Join(configDir, "portal", "projects.json"), nil
}

func init() {
	rootCmd.AddCommand(cleanCmd)
}
