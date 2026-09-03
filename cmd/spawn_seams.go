package cmd

import (
	"log/slog"
	"os"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/spawn"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/xdg"
	"github.com/spf13/cobra"
)

var spawnLogger = log.For("spawn")

type TerminalDetector interface {
	Detect() spawn.Identity
}

// The open burst and the picker wire these seams into differently-shaped
// structs, so the compiler cannot catch one side gaining or re-constructing a
// seam the other lacks — hence one bundle both read.
type productionSpawnSeams struct {
	Detector *spawn.Detector
	Resolve  spawn.AdapterResolver
	Ack      spawn.AckChannelFull
	Exe      spawn.ExecutableResolver
	Getenv   func(string) string
	Exists   func(string) bool
	Logger   *slog.Logger
}

func buildProductionSpawnSeams(client *tmux.Client) productionSpawnSeams {
	return productionSpawnSeams{
		Detector: spawn.NewDetector(client),
		Resolve:  buildResolver().Resolve,
		Ack:      spawn.NewServerOptionAckChannel(client, client),
		Exe:      os.Executable,
		Getenv:   os.Getenv,
		Exists:   client.HasSession,
		Logger:   spawnLogger,
	}
}

func spawnDetector(cmd *cobra.Command) TerminalDetector {
	return spawn.NewDetector(tmuxClient(cmd))
}

// Fails safe: an unresolvable config path degrades to an empty config —
// native-only resolution — rather than disabling the feature outright.
func buildResolver() *spawn.Resolver {
	cfg := spawn.TerminalsConfig{}
	if path, err := configFilePath(xdg.TerminalsFile); err == nil {
		cfg = spawn.NewTerminalsStore(path).Load()
	}
	return spawn.NewResolver(cfg)
}
