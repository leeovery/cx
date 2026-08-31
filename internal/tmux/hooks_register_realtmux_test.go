package tmux_test

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

type managedEventFingerprint struct {
	event       string
	fingerprint string
}

var managedEventFingerprints = []managedEventFingerprint{
	{event: "session-created", fingerprint: notifyFingerprint},
	{event: "session-closed", fingerprint: commitNowFingerprint},
	{event: "session-renamed", fingerprint: notifyFingerprint},
	{event: "window-linked", fingerprint: notifyFingerprint},
	{event: "window-unlinked", fingerprint: notifyFingerprint},
	{event: "window-layout-changed", fingerprint: notifyFingerprint},
	{event: "pane-focus-out", fingerprint: notifyFingerprint},
	{event: "client-attached", fingerprint: signalHydrateFingerprint},
	{event: "client-session-changed", fingerprint: signalHydrateFingerprint},
}

// Must stay on the per-event read: the no-arg global `show-hooks -g` is blind
// to pane-focus-out / window-layout-changed, so assertions built on it would be
// vacuously satisfied.
func portalEntryCommandsForEvent(t *testing.T, client *tmux.Client, event, fingerprint string) []string {
	t.Helper()
	raw, err := client.ShowGlobalHooksForEvent(event)
	if err != nil {
		t.Fatalf("ShowGlobalHooksForEvent(%s): %v", event, err)
	}
	parsed := tmux.ParseShowHooks(raw)
	var commands []string
	for _, e := range parsed[event] {
		if strings.Contains(e.Command, fingerprint) {
			commands = append(commands, e.Command)
		}
	}
	return commands
}

func countPortalEntriesForEvent(t *testing.T, client *tmux.Client, event, fingerprint string) int {
	t.Helper()
	return len(portalEntryCommandsForEvent(t, client, event, fingerprint))
}

func TestRegisterPortalHooks_NoGrowthAcrossBootstraps(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hooks-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const runs = 3
	for run := 1; run <= runs; run++ {
		if err := tmux.RegisterPortalHooks(client, nil); err != nil {
			t.Fatalf("run %d: RegisterPortalHooks: %v", run, err)
		}

		for _, me := range managedEventFingerprints {
			got := countPortalEntriesForEvent(t, client, me.event, me.fingerprint)
			if got != 1 {
				t.Errorf("run %d: event %q: Portal entry count = %d, want 1", run, me.event, got)
			}
		}

		if got := countPortalEntriesForEvent(t, client, "pane-focus-out", notifyFingerprint); got != 1 {
			t.Errorf("run %d: pane-focus-out (blind event): Portal entry count = %d, want 1 (no growth)", run, got)
		}
		if got := countPortalEntriesForEvent(t, client, "window-layout-changed", notifyFingerprint); got != 1 {
			t.Errorf("run %d: window-layout-changed (blind event): Portal entry count = %d, want 1 (no growth)", run, got)
		}
	}

	for _, ev := range []string{"pane-focus-out", "window-layout-changed"} {
		raw, err := client.ShowGlobalHooksForEvent(ev)
		if err != nil {
			t.Fatalf("ShowGlobalHooksForEvent(%s): %v", ev, err)
		}
		entries := tmux.ParseShowHooks(raw)[ev]
		if len(entries) != 1 {
			t.Fatalf("event %q: entry count = %d, want exactly 1; entries=%v", ev, len(entries), entries)
		}
		if entries[0].Command != expectedNotifyCommand {
			t.Errorf("event %q: desired body = %q, want %q", ev, entries[0].Command, expectedNotifyCommand)
		}
	}
}

func TestShowHooksGlobalEnumeration_OmitsPaneAndGeometryEvents(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hooks-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const enumeratedEvent = "session-created"
	blindEvents := []string{"pane-focus-out", "window-layout-changed"}

	allEvents := append([]string{enumeratedEvent}, blindEvents...)
	for _, ev := range allEvents {
		if err := client.AppendGlobalHook(ev, expectedNotifyCommand); err != nil {
			t.Fatalf("AppendGlobalHook(%s): %v", ev, err)
		}
	}

	globalRaw := ts.Run(t, "show-hooks", "-g")
	globalParsed := tmux.ParseShowHooks(globalRaw)

	if !hasPortalEntry(globalParsed[enumeratedEvent], notifyFingerprint) {
		t.Errorf("no-arg `show-hooks -g` omitted %q, but tmux 3.6b is expected to enumerate it; global entries=%v",
			enumeratedEvent, globalParsed[enumeratedEvent])
	}

	for _, ev := range blindEvents {
		if len(globalParsed[ev]) != 0 {
			t.Errorf("no-arg `show-hooks -g` enumerated %q (entries=%v); tmux 3.6b is expected to OMIT it — "+
				"the blind-spot assumption may have changed (not necessarily a Portal bug)", ev, globalParsed[ev])
		}
	}

	for _, ev := range blindEvents {
		raw, err := client.ShowGlobalHooksForEvent(ev)
		if err != nil {
			t.Fatalf("ShowGlobalHooksForEvent(%s): %v", ev, err)
		}
		parsed := tmux.ParseShowHooks(raw)
		if !hasPortalEntry(parsed[ev], notifyFingerprint) {
			t.Errorf("per-event `show-hooks -g %s` did not return the Portal entry; per-event entries=%v — "+
				"the per-event seam must never be blind", ev, parsed[ev])
		}
	}
}

func hasPortalEntry(entries []tmux.HookEntry, fingerprint string) bool {
	for _, e := range entries {
		if strings.Contains(e.Command, fingerprint) {
			return true
		}
	}
	return false
}

// A small depth traverses the same depth-N collapse path at bounded wall-clock:
// each seeded entry is a real set-hook round-trip.
const stackDepth = 5

const userHookFingerprint = "echo user pane-focus-out hook"

const userHookBody = `run-shell "echo user pane-focus-out hook"`

func TestRegisterPortalHooks_SelfHealsKDeepStackLeavingUserHookIntact(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hooks-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const event = "pane-focus-out"

	for i := range stackDepth {
		if err := client.AppendGlobalHook(event, expectedNotifyCommand); err != nil {
			t.Fatalf("seed Portal entry %d: AppendGlobalHook(%s): %v", i, event, err)
		}
	}
	if err := client.AppendGlobalHook(event, userHookBody); err != nil {
		t.Fatalf("seed user hook: AppendGlobalHook(%s): %v", event, err)
	}

	if got := countPortalEntriesForEvent(t, client, event, notifyFingerprint); got != stackDepth {
		t.Fatalf("pre-seed: Portal entry count = %d, want %d", got, stackDepth)
	}

	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("RegisterPortalHooks: %v", err)
	}

	raw, err := client.ShowGlobalHooksForEvent(event)
	if err != nil {
		t.Fatalf("ShowGlobalHooksForEvent(%s): %v", event, err)
	}
	entries := tmux.ParseShowHooks(raw)[event]

	var portal []tmux.HookEntry
	for _, e := range entries {
		if strings.Contains(e.Command, notifyFingerprint) {
			portal = append(portal, e)
		}
	}
	if len(portal) != 1 {
		t.Fatalf("after self-heal: Portal entry count = %d, want 1; entries=%v", len(portal), entries)
	}
	if portal[0].Command != expectedNotifyCommand {
		t.Errorf("after self-heal: surviving body = %q, want %q", portal[0].Command, expectedNotifyCommand)
	}

	if got := countPortalEntriesForEvent(t, client, event, userHookFingerprint); got != 1 {
		t.Errorf("after self-heal: user hook count = %d, want 1 (must survive untouched); entries=%v", got, entries)
	}
}

const sessionClosedUserHookFingerprint = "echo user session-closed hook"

const sessionClosedUserHookBody = `run-shell "echo user session-closed hook"`

func TestUnregisterPortalHooks_ReapsAtDepthOnBlindEventsLeavingUserHookIntact(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hooks-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	blindEvents := []string{"pane-focus-out", "window-layout-changed"}

	for _, event := range blindEvents {
		for i := range stackDepth {
			if err := client.AppendGlobalHook(event, expectedNotifyCommand); err != nil {
				t.Fatalf("seed Portal entry %d on %s: AppendGlobalHook: %v", i, event, err)
			}
		}
		if err := client.AppendGlobalHook(event, userHookBody); err != nil {
			t.Fatalf("seed user hook on %s: AppendGlobalHook: %v", event, err)
		}
		if got := countPortalEntriesForEvent(t, client, event, notifyFingerprint); got != stackDepth {
			t.Fatalf("pre-seed %s: Portal entry count = %d, want %d", event, got, stackDepth)
		}
	}

	const sessionClosedEvent = "session-closed"
	for i := range stackDepth {
		if err := client.AppendGlobalHook(sessionClosedEvent, expectedCommitNowCommand); err != nil {
			t.Fatalf("seed commit-now entry %d on %s: AppendGlobalHook: %v", i, sessionClosedEvent, err)
		}
	}
	if err := client.AppendGlobalHook(sessionClosedEvent, sessionClosedUserHookBody); err != nil {
		t.Fatalf("seed user hook on %s: AppendGlobalHook: %v", sessionClosedEvent, err)
	}
	if got := countPortalEntriesForEvent(t, client, sessionClosedEvent, commitNowFingerprint); got != stackDepth {
		t.Fatalf("pre-seed %s: commit-now entry count = %d, want %d", sessionClosedEvent, got, stackDepth)
	}

	if err := tmux.UnregisterPortalHooks(client); err != nil {
		t.Fatalf("UnregisterPortalHooks: %v", err)
	}

	// Hand-authored on purpose, not derived from PortalTeardownFingerprints():
	// an independent oracle that still catches a fingerprint dropped from the
	// production list. Do not "DRY" it back onto the production helper.
	teardownFingerprints := []string{
		"portal state notify",
		"portal state commit-now",
		"portal state signal-hydrate",
		"portal state migrate-rename",
	}
	for _, event := range blindEvents {
		for _, fp := range teardownFingerprints {
			if got := countPortalEntriesForEvent(t, client, event, fp); got != 0 {
				t.Errorf("after teardown: event %q still holds %d entries matching %q, want 0", event, got, fp)
			}
		}
		if got := countPortalEntriesForEvent(t, client, event, userHookFingerprint); got != 1 {
			t.Errorf("after teardown: event %q user hook count = %d, want 1 (must survive untouched)", event, got)
		}
	}

	if got := countPortalEntriesForEvent(t, client, sessionClosedEvent, commitNowFingerprint); got != 0 {
		t.Errorf("after teardown: event %q still holds %d commit-now entries, want 0 — "+
			"the converged session-closed commit-now hook survived teardown (AC #5 seam)", sessionClosedEvent, got)
	}
	if got := countPortalEntriesForEvent(t, client, sessionClosedEvent, sessionClosedUserHookFingerprint); got != 1 {
		t.Errorf("after teardown: event %q user hook count = %d, want 1 (must survive untouched)", sessionClosedEvent, got)
	}
}

func TestRegisterPortalHooks_SecondRegistrationIsChurnFree(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hooks-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	r1 := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, r1.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("first RegisterPortalHooks: %v", err)
	}

	before := snapshotEventIndices(t, client)

	r2 := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, r2.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("second RegisterPortalHooks: %v", err)
	}

	after := snapshotEventIndices(t, client)

	for _, me := range managedEventFingerprints {
		b, a := before[me.event], after[me.event]
		if !equalInts(b, a) {
			t.Errorf("event %q: entry indices changed across churn-free run: before=%v after=%v "+
				"(an unset+append would renumber — the fast path regressed)", me.event, b, a)
		}
	}

	infos := r2.infos()
	for i, reaped := range r2.infoReaped() {
		if reaped > 0 {
			t.Errorf("second run emitted an eviction INFO line %q with reaped=%d, want no eviction line",
				infos[i], reaped)
		}
	}

	if len(r2.warns()) != 0 {
		t.Errorf("second run emitted %d WARN line(s): %v, want none", len(r2.warns()), r2.warns())
	}
}

func snapshotEventIndices(t *testing.T, client *tmux.Client) map[string][]int {
	t.Helper()
	out := make(map[string][]int, len(managedEventFingerprints))
	for _, me := range managedEventFingerprints {
		raw, err := client.ShowGlobalHooksForEvent(me.event)
		if err != nil {
			t.Fatalf("ShowGlobalHooksForEvent(%s): %v", me.event, err)
		}
		var indices []int
		for _, e := range tmux.ParseShowHooks(raw)[me.event] {
			indices = append(indices, e.Index)
		}
		out[me.event] = indices
	}
	return out
}

func equalInts(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
