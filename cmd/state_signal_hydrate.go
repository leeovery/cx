package cmd

import (
	"fmt"
	"log/slog"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/spf13/cobra"
)

type signalHydrateConfig struct {
	Session  string
	StateDir string
	Client   *tmux.Client
	Logger   *slog.Logger
	Signaler state.FIFOSignaler
}

// Fired from a tmux hook: every failure path is soft so it can never block the
// server or fail the user's attach. The skeleton marker is deliberately left set —
// the hydrate helper unsets it, closing the capture-mid-dump race.
func runSignalHydrate(cfg signalHydrateConfig) error {
	cfg.Logger = signalLoggerOrDefault(cfg.Logger)
	markers, err := state.ListSkeletonMarkers(cfg.Client)
	if err != nil {
		cfg.Logger.Warn("list skeleton markers failed", "error", err)
		return nil
	}

	panes, err := cfg.Client.ListPanesInSession(cfg.Session)
	if err != nil {
		cfg.Logger.Warn("list panes for session failed", "session", cfg.Session, "error", err)
		return nil
	}

	for _, p := range panes {
		livePaneKey := state.SanitizePaneKey(cfg.Session, p.Window, p.Pane)
		if _, found := markers[livePaneKey]; !found {
			continue
		}
		fifoPath := state.FIFOPath(cfg.StateDir, livePaneKey)
		if err := cfg.Signaler.SendSignal(fifoPath); err != nil {
			cfg.Logger.Warn("write fifo failed", "path", fifoPath, "error", err)
		}
	}

	return nil
}

func signalLoggerOrDefault(logger *slog.Logger) *slog.Logger {
	if logger == nil {
		return signalLogger
	}
	return logger
}

var signalHydrateRunFunc = runSignalHydrate

var stateSignalHydrateCmd = &cobra.Command{
	Use:    "signal-hydrate <session-name>",
	Short:  "Signal hydrate helpers for the named session (internal)",
	Args:   cobra.ExactArgs(1),
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		sessionName := args[0]
		dir, err := state.EnsureDir()
		if err != nil {
			return fmt.Errorf("ensure state dir: %w", err)
		}

		cfg := signalHydrateConfig{
			Session:  sessionName,
			StateDir: dir,
			Client:   tmux.DefaultClient(),
			Logger:   signalLogger,
			Signaler: state.DefaultFIFOSignaler{},
		}
		return signalHydrateRunFunc(cfg)
	},
}

func init() {
	stateCmd.AddCommand(stateSignalHydrateCmd)
}
