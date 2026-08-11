package bootstrap

import (
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// LivePaneLister must propagate errors: a silently-empty result would read as
// "no live panes" and mass-unset every marker.
type LivePaneLister interface {
	ListAllPanesWithFormat(format string) (string, error)
}

type MarkerUnsetter interface {
	UnsetServerOption(name string) error
}

// MarkerCleanupCore unsets stale `@portal-skeleton-*` markers. Logger is
// nil-tolerant.
type MarkerCleanupCore struct {
	Markers  state.ServerOptionLister
	Panes    LivePaneLister
	Unsetter MarkerUnsetter
	Logger   *slog.Logger
}

var _ MarkerCleaner = (*MarkerCleanupCore)(nil)

// CleanStaleMarkers joins per-marker unset failures and returns them after the
// loop rather than aborting it, so one transient tmux error cannot strand the
// remaining stale markers. Every non-nil return is soft, never a *FatalError.
func (c *MarkerCleanupCore) CleanStaleMarkers() error {
	// Local, not c.Logger: the receiver must not be rewritten across calls.
	logger := log.OrDiscard(c.Logger)

	start := time.Now()
	var unset int
	summarise := func() {
		cleanLogger.Info("marker sweep complete", "unset", unset, log.Took(start))
	}

	markers, err := state.ListSkeletonMarkers(c.Markers)
	if err != nil {
		return err
	}

	// Pinned to the shared format constant: drift in the paneKey shape would
	// silently desync stale-marker cleanup from orphan-FIFO cleanup.
	raw, err := c.Panes.ListAllPanesWithFormat(tmux.StructuralKeyFormat)
	if err != nil {
		return err
	}

	live := parseLivePaneSet(raw, logger)

	// An empty live-pane result must never be read as "nothing is alive" and
	// mass-unset every marker; the skip rides the log, not the return value.
	if len(live) == 0 {
		if len(markers) == 0 {
			summarise()
			return nil
		}
		logger.Warn("stale-marker cleanup: zero live panes parsed with markers present; skipping to avoid mass-unset hazard (next bootstrap retries)")
		summarise()
		return nil
	}

	var unsetErrs []error
	for paneKey := range markers {
		if _, alive := live[paneKey]; alive {
			continue
		}
		name := state.SkeletonMarkerPrefix + paneKey
		if err := c.Unsetter.UnsetServerOption(name); err != nil {
			logger.Warn("stale-marker cleanup: unset marker failed", "pane_key", paneKey, "error", err)
			unsetErrs = append(unsetErrs, fmt.Errorf("unset %s: %w", name, err))
			continue
		}
		unset++
	}
	summarise()
	if len(unsetErrs) > 0 {
		return errors.Join(unsetErrs...)
	}
	return nil
}

// Malformed lines are skipped, never fatal: a bad line must not become a
// spurious "live" entry, and aborting would leave genuinely stale markers.
func parseLivePaneSet(raw string, logger *slog.Logger) map[string]struct{} {
	set := map[string]struct{}{}
	for line := range strings.SplitSeq(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Split on rightmost ':' so session names containing ':' survive.
		colon := strings.LastIndex(line, ":")
		if colon < 0 {
			logger.Warn("stale-marker cleanup: malformed live-pane line (missing colon)")
			continue
		}
		session := line[:colon]
		rest := line[colon+1:]
		before, after, ok := strings.Cut(rest, ".")
		if !ok {
			logger.Warn("stale-marker cleanup: malformed live-pane line (missing dot)")
			continue
		}
		window, err := strconv.Atoi(before)
		if err != nil {
			logger.Warn("stale-marker cleanup: malformed live-pane line (window not int)", "error", err)
			continue
		}
		pane, err := strconv.Atoi(after)
		if err != nil {
			logger.Warn("stale-marker cleanup: malformed live-pane line (pane not int)", "error", err)
			continue
		}
		set[state.SanitizePaneKey(session, window, pane)] = struct{}{}
	}
	return set
}
