// Package cmd defines the CLI commands for Portal.
package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

// supportedShells lists the shells that portal init supports.
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
			return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
		}

		cmdName, _ := cmd.Flags().GetString("cmd")

		w := cmd.OutOrStdout()

		switch shell {
		case "zsh":
			return emitZshInit(w, cmdName)
		default:
			return fmt.Errorf("unsupported shell: %s (supported: bash, zsh, fish)", shell)
		}
	},
}

func init() {
	rootCmd.AddCommand(initCmd)
	initCmd.Flags().String("cmd", "x", "Custom name for shell functions (e.g., --cmd p creates p() and pctl())")
}

// emitZshInit writes the zsh shell integration script to w.
// It emits shell functions, Cobra-generated completions, and compdef wiring.
// The cmdName parameter controls the function names: cmdName becomes the launcher
// and cmdName+"ctl" becomes the control plane function.
func emitZshInit(w io.Writer, cmdName string) error {
	ctlName := cmdName + "ctl"

	// Shell functions
	if _, err := fmt.Fprintf(w, "function %s() { portal open \"$@\" }\n", cmdName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "function %s() { portal \"$@\" }\n", ctlName); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Cobra-generated zsh completions for the portal binary
	if err := rootCmd.GenZshCompletion(w); err != nil {
		return fmt.Errorf("generating zsh completions: %w", err)
	}
	if _, err := fmt.Fprintln(w); err != nil {
		return err
	}

	// Wire completions to shell function names
	if _, err := fmt.Fprintf(w, "compdef _portal %s\n", cmdName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "compdef _portal %s\n", ctlName); err != nil {
		return err
	}

	return nil
}
