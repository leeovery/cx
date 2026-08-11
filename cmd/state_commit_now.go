package cmd

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/spf13/cobra"
)

// errCommitNowFailed drives a non-zero exit while keeping stderr silent: the
// tmux hook subprocess has nowhere meaningful to surface it, so diagnostics go
// to portal.log instead.
var errCommitNowFailed = errors.New("commit-now failed")

// IsSilentExitError reports whether err is a cmd-package sentinel whose stderr
// emission must be suppressed at the top-level error handler.
func IsSilentExitError(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, errCommitNowFailed) ||
		errors.Is(err, ErrDoctorUnhealthy)
}

// A non-nil commitNowDeps need not populate every field: each unset field falls
// back to its production implementation.
var commitNowDeps *CommitNowDeps

// CommitNowDeps exposes commit-now's collaborators as swappable function
// fields. CaptureStructure is always called with a nil skipSet and Commit with
// anyScrollbackChanged=false - commit-now writes no scrollback bytes.
type CommitNowDeps struct {
	ReadIndex        func(dir string) (state.Index, bool, error)
	CaptureStructure func(c state.CaptureClient, skipSet map[string]struct{}, prev *state.Index, logger *slog.Logger) (state.Index, error)
	Commit           func(dir string, idx state.Index, anyScrollbackChanged bool, logger *slog.Logger) error
	NewClient        func() state.CaptureClient

	// When @portal-restoring is set, commit-now short-circuits as a no-op: the
	// daemon owns sessions.json during restoration.
	IsRestoring func() (bool, error)

	// TouchSaveRequested is called on the short-circuit so the daemon's first
	// post-restoration tick commits without waiting out the 30s gap rule.
	TouchSaveRequested func(dir string) error
}

func resolveCommitNowDeps() *CommitNowDeps {
	deps := &CommitNowDeps{
		ReadIndex:          state.ReadIndex,
		CaptureStructure:   state.CaptureStructure,
		Commit:             state.Commit,
		NewClient:          func() state.CaptureClient { return tmux.DefaultClient() },
		IsRestoring:        func() (bool, error) { return state.IsRestoringSet(tmux.DefaultClient()) },
		TouchSaveRequested: state.TouchSaveRequested,
	}

	if commitNowDeps == nil {
		return deps
	}
	if commitNowDeps.ReadIndex != nil {
		deps.ReadIndex = commitNowDeps.ReadIndex
	}
	if commitNowDeps.CaptureStructure != nil {
		deps.CaptureStructure = commitNowDeps.CaptureStructure
	}
	if commitNowDeps.Commit != nil {
		deps.Commit = commitNowDeps.Commit
	}
	if commitNowDeps.NewClient != nil {
		deps.NewClient = commitNowDeps.NewClient
	}
	if commitNowDeps.IsRestoring != nil {
		deps.IsRestoring = commitNowDeps.IsRestoring
	}
	if commitNowDeps.TouchSaveRequested != nil {
		deps.TouchSaveRequested = commitNowDeps.TouchSaveRequested
	}
	return deps
}

// stateCommitNowCmd is invoked by the tmux session-closed hook, so
// externally-killed sessions leave sessions.json before the next bootstrap can
// resurrect them.
var stateCommitNowCmd = &cobra.Command{
	Use:    "commit-now",
	Short:  "Synchronously commit sessions.json from live tmux state (internal, invoked by tmux hooks)",
	Args:   cobra.NoArgs,
	Hidden: true,
	// Defensive restatement of the rootCmd settings: a tmux hook subprocess has
	// nowhere meaningful to send stderr, so reparenting must not lose this.
	SilenceErrors: true,
	SilenceUsage:  true,
	RunE: func(cmd *cobra.Command, args []string) error {
		dir, err := state.EnsureDir()
		if err != nil {
			// Pre-logger: without a state dir there is nowhere to open portal.log.
			return fmt.Errorf("ensure state dir: %w", err)
		}

		logger := daemonLogger

		deps := resolveCommitNowDeps()

		// A query failure is treated like (true, nil): absent proof the marker is
		// clear, presume it set and protect the in-flight restore. The cost is a
		// slightly longer resurrection window, recovered on the daemon's next tick
		// via the save.requested touch.
		restoring, err := deps.IsRestoring()
		switch {
		case err != nil:
			logger.Warn("isRestoring query failed; presuming @portal-restoring set to protect in-flight restore", "error", err)
			touchAfterShortCircuit(logger, dir, deps.TouchSaveRequested)
			return nil
		case restoring:
			logger.Info("commit-now skipped: @portal-restoring set")
			touchAfterShortCircuit(logger, dir, deps.TouchSaveRequested)
			return nil
		}

		prev := loadPrevIndex(dir, deps.ReadIndex, logger)

		client := deps.NewClient()
		idx, err := deps.CaptureStructure(client, nil, &prev, logger)
		if err != nil {
			return failCommitNow(logger, dir, deps.TouchSaveRequested, "capture structure", err)
		}

		if err := deps.Commit(dir, idx, false, logger); err != nil {
			return failCommitNow(logger, dir, deps.TouchSaveRequested, "commit sessions.json", err)
		}

		return nil
	},
}

// touchAfterShortCircuit performs the best-effort save.requested touch shared by
// both short-circuit branches. A touch failure is logged and swallowed; the
// exit-0 status dominates.
func touchAfterShortCircuit(logger *slog.Logger, dir string, touch func(string) error) {
	if terr := touch(dir); terr != nil {
		logger.Warn("touch save.requested during short-circuit failed", "error", terr)
	}
}

// failCommitNow logs the failure, best-effort touches save.requested so the
// daemon's next tick retries, and returns an error wrapping errCommitNowFailed.
//
// Only the sentinel is wrapped: the cause survives as interpolated text, since
// portal.log is the authoritative diagnostic sink and stderr stays silent.
func failCommitNow(logger *slog.Logger, dir string, touch func(string) error, stage string, cause error) error {
	logger.Error(stage+" failed", "error", cause)
	if terr := touch(dir); terr != nil {
		logger.Warn("touch save.requested after commit-now failure failed", "error", terr)
	}
	return fmt.Errorf("%w: %s: %v", errCommitNowFailed, stage, cause)
}

// loadPrevIndex returns the prior on-disk Index for CaptureStructure's prev
// argument. Both an absent and an unusable sessions.json yield a zero-value
// Index with a WARN: dropping killed sessions does not depend on PrevIndex, so a
// read failure must never abort the synchronous commit.
func loadPrevIndex(dir string, readIndex func(string) (state.Index, bool, error), logger *slog.Logger) state.Index {
	idx, skip, err := readIndex(dir)
	if err != nil {
		logger.Warn("read sessions.json failed; proceeding with zero-value PrevIndex", "error", err)
		return state.Index{}
	}
	if skip {
		logger.Warn("sessions.json absent; proceeding with zero-value PrevIndex")
		return state.Index{}
	}
	return idx
}

func init() {
	stateCmd.AddCommand(stateCommitNowCmd)
}
