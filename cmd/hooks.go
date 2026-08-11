package cmd

import (
	"fmt"
	"os"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/spf13/cobra"
)

// HookKeyResolver maps a tmux pane ID ("%3") to its rename-immune hook key.
type HookKeyResolver interface {
	ResolveHookKey(paneID string) (string, error)
}

var _ HookKeyResolver = (*tmux.Client)(nil)

var hooksDeps *HooksDeps

type HooksDeps struct {
	KeyResolver HookKeyResolver
}

func requireTmuxPane() (string, error) {
	paneID := os.Getenv("TMUX_PANE")
	if paneID == "" {
		return "", fmt.Errorf("must be run from inside a tmux pane")
	}
	return paneID, nil
}

func buildHooksTmuxClient() *tmux.Client {
	return tmux.DefaultClient()
}

func resolveCurrentPaneKey() (string, error) {
	paneID, err := requireTmuxPane()
	if err != nil {
		return "", err
	}

	var keyResolver HookKeyResolver
	if hooksDeps != nil && hooksDeps.KeyResolver != nil {
		keyResolver = hooksDeps.KeyResolver
	} else {
		keyResolver = buildHooksTmuxClient()
	}

	hookKey, err := keyResolver.ResolveHookKey(paneID)
	if err != nil {
		return "", fmt.Errorf("failed to resolve hook key for current pane: %w", err)
	}

	return hookKey, nil
}

// `hooks` is a permanent back-compat alias for machine-written invocations. It
// must stay a plain Aliases entry — cmd.Deprecated would print a notice.
var hookCmd = &cobra.Command{
	Use:     "hook",
	Aliases: []string{"hooks"},
	Short:   "Manage resume hooks",
}

var hooksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all registered hooks",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := loadHookStore()
		if err != nil {
			return err
		}

		list, err := store.List()
		if err != nil {
			return err
		}

		for _, h := range list {
			if _, err := fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\n", h.Key, h.Event, h.Command); err != nil {
				return err
			}
		}

		return nil
	},
}

var hooksSetCmd = &cobra.Command{
	Use:   "set",
	Short: "Register a resume hook for the current pane",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		hookKey, err := resolveCurrentPaneKey()
		if err != nil {
			return err
		}

		command, err := cmd.Flags().GetString("on-resume")
		if err != nil {
			return err
		}

		store, err := loadHookStore()
		if err != nil {
			return err
		}

		return store.Set(hookKey, "on-resume", command, "cli")
	},
}

func loadHookStore() (*hooks.Store, error) {
	path, err := hooksFilePath()
	if err != nil {
		return nil, err
	}

	return hooks.NewStore(path), nil
}

func hooksFilePath() (string, error) {
	return configFilePath("PORTAL_HOOKS_FILE", "hooks.json")
}

var hooksRmCmd = &cobra.Command{
	Use:   "rm",
	Short: "Remove a resume hook for the current pane",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		paneKey, err := cmd.Flags().GetString("pane-key")
		if err != nil {
			return err
		}

		var hookKey string
		if paneKey != "" {
			hookKey = paneKey
		} else {
			hookKey, err = resolveCurrentPaneKey()
			if err != nil {
				return err
			}
		}

		store, err := loadHookStore()
		if err != nil {
			return err
		}

		return store.Remove(hookKey, "on-resume", "cli")
	},
}

func init() {
	hooksSetCmd.Flags().String("on-resume", "", "Command to run when resuming the pane")
	_ = hooksSetCmd.MarkFlagRequired("on-resume")

	hooksRmCmd.Flags().Bool("on-resume", false, "Remove the on-resume hook")
	_ = hooksRmCmd.MarkFlagRequired("on-resume")
	hooksRmCmd.Flags().String("pane-key", "", "Structural key of the pane whose hook should be removed (defaults to the current pane)")

	hookCmd.AddCommand(hooksListCmd)
	hookCmd.AddCommand(hooksSetCmd)
	hookCmd.AddCommand(hooksRmCmd)
	rootCmd.AddCommand(hookCmd)
}
