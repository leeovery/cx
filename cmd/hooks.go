package cmd

import (
	"fmt"
	"os"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/spf13/cobra"
)

// HookKeyResolver maps a tmux pane ID ("%3") to its hook key — the pane's
// durable token, empty for a pane that has never been stamped.
type HookKeyResolver interface {
	ResolveHookKey(paneID string) (string, error)
}

// PaneOptionSetter writes one tmux option onto one pane.
type PaneOptionSetter interface {
	SetPaneOption(target, name, value string) error
}

var (
	_ HookKeyResolver  = (*tmux.Client)(nil)
	_ PaneOptionSetter = (*tmux.Client)(nil)
)

var hooksDeps *HooksDeps

type HooksDeps struct {
	KeyResolver HookKeyResolver
	PaneStamper PaneOptionSetter
	TokenMinter session.IDGenerator
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

// resolveCurrentPaneKey returns the current pane's hook key alongside the pane
// it was read from, so a caller that must stamp acts on that same pane.
func resolveCurrentPaneKey() (hookKey, paneID string, err error) {
	paneID, err = requireTmuxPane()
	if err != nil {
		return "", "", err
	}

	var keyResolver HookKeyResolver
	if hooksDeps != nil && hooksDeps.KeyResolver != nil {
		keyResolver = hooksDeps.KeyResolver
	} else {
		keyResolver = buildHooksTmuxClient()
	}

	hookKey, err = keyResolver.ResolveHookKey(paneID)
	if err != nil {
		return "", "", fmt.Errorf("failed to resolve hook key for current pane: %w", err)
	}

	return hookKey, paneID, nil
}

func hooksPaneStamper() PaneOptionSetter {
	if hooksDeps != nil && hooksDeps.PaneStamper != nil {
		return hooksDeps.PaneStamper
	}
	return buildHooksTmuxClient()
}

func hooksTokenMinter() session.IDGenerator {
	if hooksDeps != nil && hooksDeps.TokenMinter != nil {
		return hooksDeps.TokenMinter
	}
	return session.NewPaneToken
}

// stampPaneToken mints a token for an un-stamped pane and writes it, returning
// tmux's own error unaltered so a target naming no pane reads as tmux said it.
func stampPaneToken(paneID string) (string, error) {
	token, err := hooksTokenMinter()()
	if err != nil {
		return "", fmt.Errorf("failed to mint a pane token: %w", err)
	}

	if err := hooksPaneStamper().SetPaneOption(paneID, state.PortalPaneIDOption, token); err != nil {
		return "", err
	}

	return token, nil
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
		hookKey, paneID, err := resolveCurrentPaneKey()
		if err != nil {
			return err
		}

		command, err := cmd.Flags().GetString("on-resume")
		if err != nil {
			return err
		}

		// The stamp lands before the entry: an entry written first, or after a
		// stamp that failed, is keyed to a token no pane carries.
		if hookKey == "" {
			if hookKey, err = stampPaneToken(paneID); err != nil {
				return err
			}
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
			hookKey, _, err = resolveCurrentPaneKey()
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
