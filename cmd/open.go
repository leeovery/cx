package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

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

		finalModel, err := p.Run()
		if err != nil {
			return err
		}

		model, ok := finalModel.(tui.Model)
		if !ok {
			return fmt.Errorf("unexpected model type: %T", finalModel)
		}

		selected := model.Selected()
		if selected == "" {
			return nil
		}

		tmuxPath, err := exec.LookPath("tmux")
		if err != nil {
			return fmt.Errorf("tmux not found: %w", err)
		}

		return syscall.Exec(tmuxPath, []string{"tmux", "attach-session", "-t", selected}, os.Environ())
	},
}

func init() {
	rootCmd.AddCommand(openCmd)
}
