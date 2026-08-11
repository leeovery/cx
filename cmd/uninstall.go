package cmd

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/spf13/cobra"
)

// UninstallDeps injects test dependencies for the uninstall command. Client is
// required; Unregister and Logger fall back to their production values when nil.
type UninstallDeps struct {
	Client     *tmux.Client
	Unregister func(*tmux.Client) error
	Logger     *slog.Logger
}

var uninstallDeps *UninstallDeps

// The log-component taxonomy is closed, so uninstall borrows the daemon
// component's logger rather than introducing one.
func buildUninstallDeps() (*tmux.Client, func(*tmux.Client) error, *slog.Logger) {
	if uninstallDeps != nil {
		unregister := uninstallDeps.Unregister
		if unregister == nil {
			unregister = tmux.UnregisterPortalHooks
		}
		logger := uninstallDeps.Logger
		if logger == nil {
			logger = daemonLogger
		}
		return uninstallDeps.Client, unregister, logger
	}
	return tmux.DefaultClient(), tmux.UnregisterPortalHooks, daemonLogger
}

const uninstallCompletionLine1 = "Portal's tmux runtime removed. Your saved sessions and config are untouched at ~/.config/portal/."
const uninstallCompletionLine2 = "To remove Portal completely, uninstall the binary and delete that directory."

// Order matters: killSaver runs before the hooks are unregistered so the
// daemon's SIGHUP flush observes the pre-teardown world. Neither failure
// short-circuits the other.
var uninstallCmd = &cobra.Command{
	Use:           "uninstall",
	Short:         "Remove Portal's tmux runtime (save daemon + global hooks); leaves saved sessions and config",
	Args:          cobra.NoArgs,
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		client, unregister, logger := buildUninstallDeps()

		var errs []error

		if client.ServerRunning() {
			if err := killSaver(client, logger); err != nil {
				errs = append(errs, fmt.Errorf("daemon kill: %w", err))
			}
			if err := unregister(client); err != nil {
				errs = append(errs, fmt.Errorf("hook removal: %w", err))
			}
		}

		// Printed before the joined error returns: the message must appear even
		// on a partial failure, since nothing here is irreversible.
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), uninstallCompletionLine1)
		_, _ = fmt.Fprintln(cmd.OutOrStdout(), uninstallCompletionLine2)

		return errors.Join(errs...)
	},
}

const killSaverInfoMessage = "killed _portal-saver; daemon will flush final state on SIGHUP"

// killSaver probes with the discriminating HasSessionProbe, not the
// error-collapsing HasSession, so a transient tmux fault is never mistaken for
// "saver absent". A session that auto-destroys between probe and kill is success.
func killSaver(c *tmux.Client, logger *slog.Logger) error {
	present, err := c.HasSessionProbe(tmux.PortalSaverName)
	switch {
	case err == nil:
		// Confirmed present — fall through to the kill below.
	case !present:
		// A genuine non-zero tmux exit: the saver is gone, nothing to kill.
		return nil
	default:
		// The probe could not confirm either way; surface it rather than claim
		// the saver was removed.
		logger.Warn("kill _portal-saver probe failed", "error", err)
		return fmt.Errorf("saver probe: %w", err)
	}

	if err := c.KillSession(tmux.PortalSaverName); err != nil {
		if isSessionAbsentError(err) {
			logger.Info(killSaverInfoMessage)
			return nil
		}
		logger.Warn("kill _portal-saver failed", "error", err)
		return err
	}
	logger.Info(killSaverInfoMessage)
	return nil
}

// Substring match on tmux's own wording: stable across tmux 3.0+, matched
// case-insensitively against future capitalisation changes.
func isSessionAbsentError(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "can't find session")
}

func init() {
	rootCmd.AddCommand(uninstallCmd)
}
