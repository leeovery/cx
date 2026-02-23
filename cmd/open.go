package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tui"
	"github.com/spf13/cobra"
)

var openCmd = &cobra.Command{
	Use:   "open",
	Short: "Open the interactive session picker",
	RunE: func(cmd *cobra.Command, args []string) error {
		client := tmux.NewClient(&tmux.RealCommander{})
		m := tui.New(client)
		p := tea.NewProgram(m, tea.WithAltScreen())
		_, err := p.Run()
		return err
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
}
