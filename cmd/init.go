package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

var supportedShells = map[string]bool{
	"bash": true,
	"zsh":  true,
	"fish": true,
}

var initCmd = &cobra.Command{
	Use:       "init [shell]",
	Short:     "Output shell integration script",
	Long:      "Output shell functions and tab completions for eval. Usage: eval \"$(portal init zsh)\"",
	Args:      cobra.ExactArgs(1),
	ValidArgs: []string{"bash", "zsh", "fish"},
	RunE: func(cmd *cobra.Command, args []string) error {
		shell := args[0]
		if !supportedShells[shell] {
			return NewUsageError(fmt.Sprintf("unsupported shell: %s (supported: bash, zsh, fish)", shell))
		}

		cmdName, _ := cmd.Flags().GetString("cmd")

		w := cmd.OutOrStdout()

		switch shell {
		case "bash":
			return emitBashInit(w, cmdName)
		case "zsh":
			return emitZshInit(w, cmdName)
		case "fish":
			return emitFishInit(w, cmdName)
		default:
			// unreachable: the supportedShells check above rejects these.
			return NewUsageError(fmt.Sprintf("unsupported shell: %s (supported: bash, zsh, fish)", shell))
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().String("cmd", "x", "Custom name for shell functions (e.g., --cmd p creates p() and pctl())")
}

func emitBashInit(w io.Writer, cmdName string) error {
	ctlName := cmdName + "ctl"

	if _, err := fmt.Fprintf(w, "%s() { portal open \"$@\"; }\n", cmdName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "%s() { portal \"$@\"; }\n", ctlName); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if err := rootCmd.GenBashCompletionV2(w, true); err != nil {
		return fmt.Errorf("generating bash completions: %w", err)
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "complete -o default -F __start_portal %s\n", cmdName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "complete -o default -F __start_portal %s\n", ctlName); err != nil {
		return err
	}

	return nil
}

func emitFishInit(w io.Writer, cmdName string) error {
	ctlName := cmdName + "ctl"

	if _, err := fmt.Fprintf(w, "function %s\n    portal open $argv\nend\n", cmdName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "function %s\n    portal $argv\nend\n", ctlName); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if err := rootCmd.GenFishCompletion(w, true); err != nil {
		return fmt.Errorf("generating fish completions: %w", err)
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "complete -c %s -w portal\n", cmdName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "complete -c %s -w portal\n", ctlName); err != nil {
		return err
	}

	return nil
}

func emitZshInit(w io.Writer, cmdName string) error {
	ctlName := cmdName + "ctl"

	if _, err := fmt.Fprintf(w, "function %s() { portal open \"$@\" }\n", cmdName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "function %s() { portal \"$@\" }\n", ctlName); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if err := rootCmd.GenZshCompletion(w); err != nil {
		return fmt.Errorf("generating zsh completions: %w", err)
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "compdef _portal %s\n", cmdName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "compdef _portal %s\n", ctlName); err != nil {
		return err
	}

	return nil
}
