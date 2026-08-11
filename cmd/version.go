package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version is set at build time via ldflags; renaming it breaks the release
// build's -X target.
var version = "dev"

func Version() string { return version }

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show Portal version",
	RunE: func(cmd *cobra.Command, args []string) error {
		_, err := fmt.Fprintf(cmd.OutOrStdout(), "portal version %s\n", version)
		return err
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
