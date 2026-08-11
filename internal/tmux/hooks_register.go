package tmux

import (
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sort"

	"github.com/leeovery/portal/internal/log"
)

// An entry is Portal-authored iff its command contains any of fingerprints;
// session-closed carries an extra one so a body left by an older binary
// converges onto the current one.
type managedEvent struct {
	event        string
	fingerprints []string
	desiredBody  string
}

var managedEvents = []managedEvent{
	{event: "session-created", fingerprints: []string{notifySubstring}, desiredBody: notifyCommand},
	{event: sessionClosedEvent, fingerprints: []string{notifySubstring, commitNowSubstring}, desiredBody: commitNowCommand},
	{event: "session-renamed", fingerprints: []string{notifySubstring}, desiredBody: notifyCommand},
	{event: "window-linked", fingerprints: []string{notifySubstring}, desiredBody: notifyCommand},
	{event: "window-unlinked", fingerprints: []string{notifySubstring}, desiredBody: notifyCommand},
	{event: "window-layout-changed", fingerprints: []string{notifySubstring}, desiredBody: notifyCommand},
	{event: "pane-focus-out", fingerprints: []string{notifySubstring}, desiredBody: notifyCommand},
	{event: "client-attached", fingerprints: []string{signalHydrateMarker}, desiredBody: signalHydrateCommand},
	{event: "client-session-changed", fingerprints: []string{signalHydrateMarker}, desiredBody: signalHydrateCommand},
}

func managedEventNames() []string {
	names := make([]string, len(managedEvents))
	for i, me := range managedEvents {
		names[i] = me.event
	}
	return names
}

// Registration never installs this body, so it is absent from managedEvents;
// teardown still needs it because older binaries left an inert migrate-rename
// hook on session-renamed that must be reaped.
const migrateRenameSubstring = "portal state migrate-rename"

func managedEventFingerprintUnion() []string {
	seen := make(map[string]bool)
	var out []string
	for _, me := range managedEvents {
		for _, fp := range me.fingerprints {
			if seen[fp] {
				continue
			}
			seen[fp] = true
			out = append(out, fp)
		}
	}
	return out
}

func teardownFingerprints() []string {
	union := managedEventFingerprintUnion()
	if slices.Contains(union, migrateRenameSubstring) {
		return union
	}
	return append(union, migrateRenameSubstring)
}

// HydrationTriggerEvents lists the events Portal registers a signal-hydrate hook
// on. Treat the slice as read-only.
var HydrationTriggerEvents = []string{
	"client-attached",
	"client-session-changed",
}

// The `command -v portal` guard keeps tmux from logging "command not found"
// spam while the binary is swapped or after uninstall.
const notifyCommand = `run-shell "command -v portal >/dev/null 2>&1 && portal state notify"`

// session-closed commits synchronously rather than touching the dirty flag:
// it is the one tmux seam that fires on every kill path, and a deferred write
// would let the daemon's next tick resurrect the killed session.
const commitNowCommand = `run-shell "command -v portal >/dev/null 2>&1 && portal state commit-now"`

// The ` -- ` end-of-flags separator is load-bearing: a session name beginning
// with `-` would otherwise be parsed by pflag as a short-flag cluster and the
// hook would exit non-zero before signal-hydrate runs.
const signalHydrateCommand = `run-shell "command -v portal >/dev/null 2>&1 && portal state signal-hydrate -- #{session_name}"`

const notifySubstring = "portal state notify"

const commitNowSubstring = "portal state commit-now"

// Deliberately the bare verb, without the `--` separator, so it also matches
// hydration bodies written by older binaries and converges them.
const signalHydrateMarker = "portal state signal-hydrate"

const sessionClosedEvent = "session-closed"

func parseEventEntries(raw, event string) []HookEntry {
	return ParseShowHooks(raw)[event]
}

// Callers must have normalised logger through log.OrDiscard.
func warnShowHooksFailure(logger *slog.Logger, err error) {
	logger.Warn("show-hooks failed", "error", err, "error_class", "unexpected")
}

type evictFailure struct {
	index int
	err   error
}

// portalEntries must already be filtered to Portal-authored entries; callers own
// the fingerprint choice. Indices are unset in descending order so a removal can
// never shift an index still to be processed. A failure neither stops the loop
// nor counts towards the returned evicted total.
func evictPortalEntries(c *Client, event string, portalEntries []HookEntry) (int, []evictFailure) {
	indices := make([]int, len(portalEntries))
	for i, entry := range portalEntries {
		indices[i] = entry.Index
	}
	sort.Sort(sort.Reverse(sort.IntSlice(indices)))

	var evicted int
	var failures []evictFailure
	for _, idx := range indices {
		if err := c.UnsetGlobalHookAt(event, idx); err != nil {
			failures = append(failures, evictFailure{index: idx, err: err})
			continue
		}
		evicted++
	}
	return evicted, failures
}

// convergeEvent leaves the event's global hook array holding exactly one Portal
// entry carrying desiredBody, returning how many entries it had to unset. Entries
// matching no fingerprint belong to the user or another plugin and are never
// touched.
func convergeEvent(c *Client, logger *slog.Logger, event string, fingerprints []string, desiredBody string) (int, error) {
	logger = log.OrDiscard(logger)

	raw, err := c.ShowGlobalHooksForEvent(event)
	if err != nil {
		warnShowHooksFailure(logger, err)
		return 0, fmt.Errorf("show-hooks failed: %w", err)
	}

	var portalEntries []HookEntry
	var alreadyConverged bool
	for _, entry := range parseEventEntries(raw, event) {
		if !containsAny(entry.Command, fingerprints) {
			continue
		}
		portalEntries = append(portalEntries, entry)
		if entry.Command == desiredBody {
			alreadyConverged = true
		}
	}

	if len(portalEntries) == 1 && alreadyConverged {
		return 0, nil
	}

	evicted, failures := evictPortalEntries(c, event, portalEntries)
	for _, f := range failures {
		// The closed log attr vocabulary has no event or index key.
		logger.Warn("failed to evict portal hook", "error", f.err)
	}

	if err := c.AppendGlobalHook(event, desiredBody); err != nil {
		return evicted, fmt.Errorf("append hook: %w", err)
	}
	return evicted, nil
}

// RegisterPortalHooks converges Portal's whole hook table to exactly one entry
// per managed event, tolerating a nil logger. It never short-circuits: a
// per-event failure is folded into an errors.Join aggregate naming the event, and
// every other event is still converged. The per-event loop must not be collapsed
// into a single whole-scope read (see ShowGlobalHooksForEvent): on the blind
// events every bootstrap would append another duplicate.
func RegisterPortalHooks(c *Client, logger *slog.Logger) error {
	logger = log.OrDiscard(logger)

	var errs []error
	var totalEvicted int

	for _, me := range managedEvents {
		evicted, err := convergeEvent(c, logger, me.event, me.fingerprints, me.desiredBody)
		if err != nil {
			errs = append(errs, fmt.Errorf("register hook on %s: %w", me.event, err))
		}
		totalEvicted += evicted
	}

	if totalEvicted > 0 {
		logger.Info("collapsed stacked portal hooks", "reaped", totalEvicted)
	}

	if len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}
