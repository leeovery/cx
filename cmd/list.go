package cmd

import (
	"fmt"
	"os"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/spf13/cobra"
)

var listDeps *ListDeps

type SessionLister interface {
	ListSessions() ([]tmux.Session, error)
}

type ListDeps struct {
	Lister SessionLister
	IsTTY  func() bool
}

func isTTY() bool {
	stat, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
}

func formatSessionLong(s tmux.Session) string {
	status := "detached"
	if s.Attached {
		status = "attached"
	}
	windowWord := "windows"
	if s.Windows == 1 {
		windowWord = "window"
	}
	return fmt.Sprintf("%s    %s    %d %s", s.Name, status, s.Windows, windowWord)
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List running tmux sessions",
	RunE: func(cmd *cobra.Command, args []string) error {
		shortFlag, _ := cmd.Flags().GetBool("short")
		longFlag, _ := cmd.Flags().GetBool("long")

		if shortFlag && longFlag {
			return fmt.Errorf("--short and --long are mutually exclusive")
		}

		lister, ttyDetect := buildListDeps(cmd)

		sessions, err := lister.ListSessions()
		if err != nil {
			return err
		}

		if len(sessions) == 0 {
			return nil
		}

		useLong := ttyDetect()
		if shortFlag {
			useLong = false
		}
		if longFlag {
			useLong = true
		}

		w := cmd.OutOrStdout()
		for _, s := range sessions {
			var err error
			if useLong {
				_, err = fmt.Fprintln(w, formatSessionLong(s))
			} else {
				_, err = fmt.Fprintln(w, s.Name)
			}
			if err != nil {
				return err
			}
		}

		return nil
	},
}

func buildListDeps(cmd *cobra.Command) (SessionLister, func() bool) {
	if listDeps != nil {
		return listDeps.Lister, listDeps.IsTTY
	}
	return tmuxClient(cmd), isTTY
}

func init() {
	listCmd.Flags().Bool("short", false, "Output session names only")
	listCmd.Flags().Bool("long", false, "Output full session details")
	rootCmd.AddCommand(listCmd)
}
