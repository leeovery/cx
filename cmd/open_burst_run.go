package cmd

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/leeovery/portal/internal/spawn"
	"github.com/spf13/cobra"
)

var openBurstDeps *OpenBurstDeps

// OpenBurstDeps allows injecting dependencies for the multi-target open burst.
// Every unset field falls back to its production implementation.
type OpenBurstDeps struct {
	Detector   TerminalDetector
	Resolve    spawn.AdapterResolver
	Connector  SessionConnector
	ExePath    spawn.ExecutableResolver
	Getenv     func(string) string
	Ack        spawn.AckChannelFull
	NewBurster func(adapter spawn.Adapter) *spawn.Burster
	Logger     *slog.Logger
	// LocalMint self-connects a mint trigger in the invoking terminal.
	LocalMint func(cmd *cobra.Command, dir string, command []string) error
}

// buildOpenBurstDeps keeps injected fields and defaults the rest. The shared
// seam bundle is memoised lazily so a fully-injected caller never resolves a tmux
// client (there is none in context) nor loads terminals.json.
func buildOpenBurstDeps(cmd *cobra.Command) *OpenBurstDeps {
	deps := &OpenBurstDeps{}
	if openBurstDeps != nil {
		*deps = *openBurstDeps
	}

	var (
		seams      productionSpawnSeams
		seamsBuilt bool
	)
	sharedSeams := func() productionSpawnSeams {
		if !seamsBuilt {
			seams = buildProductionSpawnSeams(tmuxClient(cmd))
			seamsBuilt = true
		}
		return seams
	}

	if deps.Detector == nil {
		deps.Detector = spawnDetector(cmd)
	}
	if deps.Resolve == nil {
		deps.Resolve = sharedSeams().Resolve
	}
	if deps.Connector == nil {
		deps.Connector = buildSessionConnector(tmuxClient(cmd))
	}
	if deps.ExePath == nil {
		deps.ExePath = sharedSeams().Exe
	}
	if deps.Getenv == nil {
		deps.Getenv = sharedSeams().Getenv
	}
	if deps.Ack == nil {
		deps.Ack = sharedSeams().Ack
	}
	if deps.NewBurster == nil {
		// Reads the defaulted seams at burst time rather than here, so it never
		// re-resolves the tmux client.
		deps.NewBurster = func(adapter spawn.Adapter) *spawn.Burster {
			return spawn.NewBurster(adapter, deps.Ack, deps.ExePath, deps.Getenv)
		}
	}
	if deps.Logger == nil {
		deps.Logger = sharedSeams().Logger
	}
	if deps.LocalMint == nil {
		deps.LocalMint = func(c *cobra.Command, dir string, command []string) error {
			return openPathFunc(c, dir, command)
		}
	}
	return deps
}

// runOpenBurstWithDeps opens the N≥2 resolved surfaces of a multi-target open:
// the trigger (the first surface in command-line order) absorbs the invoking
// terminal and self-connects last, while the N−1 non-trigger surfaces are spawned
// first into host-terminal windows. Requires len(surfaces) >= 2.
//
// The order is load-bearing outside tmux: the trigger's connector exec-replaces
// the Portal process, so connecting first would destroy the burster and open only
// one surface. Duplicates in the surface set are honoured, never deduped.
func runOpenBurstWithDeps(cmd *cobra.Command, surfaces []spawn.Surface, command []string, deps *OpenBurstDeps) error {
	// A command rides mint windows only, so a set of pure attaches has nowhere to
	// run it. Refused before detect/resolve/spawn.
	if len(command) > 0 && !hasMintSurface(surfaces) {
		return NewUsageError(commandAttachOnlyMessage)
	}

	trigger, external := spawn.SplitTriggerFirst(surfaces)

	id := deps.Detector.Detect()
	adapter, resolution := deps.Resolve(id)

	// Atomic no-op: with no adapter the N−1 external windows cannot open, so the
	// trigger must not half-connect either — a "trigger only" open would violate
	// the all-surfaces intent.
	if resolution == spawn.ResolutionUnsupported {
		spawn.LogUnsupported(deps.Logger, id)
		return errors.New(spawn.UnsupportedNoopMessage(id))
	}

	batch, results, err := deps.NewBurster(adapter).Run(context.Background(), external, command, nil)
	if err != nil {
		return err
	}

	// Cleaned before the trigger handoff, which is a point of no return outside
	// tmux. Best-effort: a leaked marker expires with the tmux server.
	_ = deps.Ack.Clean(batch)

	// The trigger's self-connect below is independent of these outcomes: its target
	// is unrelated to the externals', so reporting must not gate the connect. This
	// diverges from the picker's burst, which skips its self-attach on any
	// not-all-confirmed batch.
	confirmed, failed := spawn.PartitionResults(results)
	total := len(surfaces)
	if perm, ok := spawn.FirstPermission(results); ok {
		// A permission wall stops the burst — every later window hits the identical
		// wall — and takes the no-batch-summary arm, surfacing guidance once.
		spawn.LogWindowResults(deps.Logger, results)
		spawn.LogPermission(deps.Logger, id, resolution, perm.Result.Detail)
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), perm.Result.Guidance)
	} else if len(failed) > 0 {
		// Leave-what-opened: confirmed windows stay, failed surfaces are neither
		// retried nor torn down.
		spawn.LogBatchSummary(deps.Logger, id, resolution, results, total, true, batch)
		_, _ = fmt.Fprintln(cmd.ErrOrStderr(), spawn.PartialFailureMessage(failed, len(confirmed) > 0))
	} else {
		spawn.LogBatchSummary(deps.Logger, id, resolution, results, total, true, batch)
	}

	// The summary above optimistically counted the trigger's self-attach because it
	// must be emitted before this connect (a successful outside-tmux attach
	// exec-replaces the process and never returns). Only the connect-failure path
	// reaches the corrective WARN, so portal.log is not left claiming an `opened
	// N/N` that counts a trigger which never landed.
	if err := connectTrigger(cmd, trigger, command, deps); err != nil {
		spawn.LogTriggerConnectFailed(deps.Logger, trigger.Value, err.Error())
		return err
	}
	return nil
}

func hasMintSurface(surfaces []spawn.Surface) bool {
	for _, s := range surfaces {
		if s.Kind == spawn.SurfaceMint {
			return true
		}
	}
	return false
}

// connectTrigger self-connects the trigger surface in the invoking terminal.
// This is where the inside/outside-tmux split is selected; the N−1 external
// windows always run the spawned out-of-tmux argv.
func connectTrigger(cmd *cobra.Command, trigger spawn.Surface, command []string, deps *OpenBurstDeps) error {
	if trigger.Kind == spawn.SurfaceMint {
		return deps.LocalMint(cmd, trigger.Value, command)
	}
	return deps.Connector.Connect(trigger.Value)
}
