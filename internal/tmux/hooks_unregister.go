package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/leeovery/portal/internal/log"
)

var bootstrapLogger = log.For("bootstrap")

var portalCommandSubstrings = teardownFingerprints()

var portalEvents = managedEventNames()

// UnregisterPortalHooks removes every Portal-owned hook entry from the global
// tmux hook table, leaving user and other-plugin entries on the same events
// untouched. It never short-circuits: every removal is attempted and failures
// are aggregated via errors.Join.
//
// The per-event loop must not be collapsed into a single whole-scope read (see
// ShowGlobalHooksForEvent): the blind events' entries would survive teardown.
func UnregisterPortalHooks(c *Client) error {
	return unregisterPortalHooks(c, bootstrapLogger)
}

func unregisterPortalHooks(c *Client, logger *slog.Logger) error {
	logger = log.OrDiscard(logger)

	var errs []error
	for _, event := range portalEvents {
		raw, err := c.ShowGlobalHooksForEvent(event)
		if err != nil {
			warnShowHooksFailure(logger, err)
			errs = append(errs, fmt.Errorf("show-hooks failed on %s: %w", event, err))
			continue
		}

		portal := portalEntriesFor(parseEventEntries(raw, event))
		_, failures := evictPortalEntries(c, event, portal)
		for _, f := range failures {
			errs = append(errs, fmt.Errorf("unset hook on %s[%d]: %w", event, f.index, f.err))
		}
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

func portalEntriesFor(entries []HookEntry) []HookEntry {
	var out []HookEntry
	for _, entry := range entries {
		if containsAny(entry.Command, portalCommandSubstrings) {
			out = append(out, entry)
		}
	}
	return out
}

func containsAny(s string, substrings []string) bool {
	for _, sub := range substrings {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
