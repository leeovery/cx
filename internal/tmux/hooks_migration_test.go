package tmux_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/commandertest"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const staleSignalHydrateCommand = `run-shell "command -v portal >/dev/null 2>&1 && portal state signal-hydrate #{session_name}"`

func countSignalHydrateEntries(t *testing.T, client *tmux.Client) map[string]int {
	t.Helper()
	counts := make(map[string]int)
	for _, ev := range tmux.HydrationTriggerEvents {
		counts[ev] = countPortalEntriesForEvent(t, client, ev, "portal state signal-hydrate")
	}
	return counts
}

func installStaleHooks(t *testing.T, client *tmux.Client) {
	t.Helper()
	for _, ev := range tmux.HydrationTriggerEvents {
		if err := client.AppendGlobalHook(ev, staleSignalHydrateCommand); err != nil {
			t.Fatalf("AppendGlobalHook(%s): %v", ev, err)
		}
	}
}

func TestRegisterPortalHooks_HydrationConvergesUnSeparatedToDashForm(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-mig-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
	installStaleHooks(t, client)

	log := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, log.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("RegisterPortalHooks: %v", err)
	}

	counts := countSignalHydrateEntries(t, client)
	for _, ev := range tmux.HydrationTriggerEvents {
		if counts[ev] != 1 {
			t.Errorf("event %q: signal-hydrate entry count = %d, want 1", ev, counts[ev])
		}
	}

	if infos := log.infos(); len(infos) != 1 {
		t.Errorf("INFO line count = %d, want 1; infos=%v", len(infos), infos)
	} else if !strings.Contains(infos[0], "collapsed stacked portal hooks") || log.infoReaped()[0] < 1 {
		t.Errorf("INFO line = %q reaped=%d, missing eviction summary", infos[0], log.infoReaped()[0])
	}

	for _, ev := range tmux.HydrationTriggerEvents {
		fixed := portalEntryCommandsForEvent(t, client, ev, "portal state signal-hydrate -- ")
		if len(fixed) == 0 {
			t.Errorf("event %q: no entry containing `signal-hydrate -- `; fixed entries=%v", ev, fixed)
		}
	}
}

func TestRegisterPortalHooks_HydrationSecondBootstrapIsSilentNoOp(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-mig-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
	installStaleHooks(t, client)

	first := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, first.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("first RegisterPortalHooks: %v", err)
	}

	second := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, second.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("second RegisterPortalHooks: %v", err)
	}

	if len(second.infos()) != 0 {
		t.Errorf("second bootstrap INFO count = %d, want 0; infos=%v", len(second.infos()), second.infos())
	}
	if len(second.warns()) != 0 {
		t.Errorf("second bootstrap WARN count = %d, want 0; warns=%v", len(second.warns()), second.warns())
	}

	counts := countSignalHydrateEntries(t, client)
	for _, ev := range tmux.HydrationTriggerEvents {
		if counts[ev] != 1 {
			t.Errorf("event %q: signal-hydrate entry count = %d, want 1", ev, counts[ev])
		}
	}
}

func TestRegisterPortalHooks_HydrationFreshInstallIsSilentAndInstallsFixed(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-mig-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	log := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, log.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("RegisterPortalHooks: %v", err)
	}

	if len(log.infos()) != 0 {
		t.Errorf("INFO count = %d, want 0 (zero-eviction bootstrap silent); infos=%v", len(log.infos()), log.infos())
	}
	if len(log.warns()) != 0 {
		t.Errorf("WARN count = %d, want 0; warns=%v", len(log.warns()), log.warns())
	}

	counts := countSignalHydrateEntries(t, client)
	for _, ev := range tmux.HydrationTriggerEvents {
		if counts[ev] != 1 {
			t.Errorf("event %q: signal-hydrate entry count = %d, want 1", ev, counts[ev])
		}
	}
}

func TestRegisterPortalHooks_HydrationCollapsesMultipleStaleEntriesOnOneEvent(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-mig-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	for i := range 3 {
		if err := client.AppendGlobalHook("client-attached", staleSignalHydrateCommand); err != nil {
			t.Fatalf("AppendGlobalHook[client-attached][%d]: %v", i, err)
		}
	}
	if err := client.AppendGlobalHook("client-session-changed", staleSignalHydrateCommand); err != nil {
		t.Fatalf("AppendGlobalHook[client-session-changed]: %v", err)
	}

	log := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, log.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("RegisterPortalHooks: %v", err)
	}

	counts := countSignalHydrateEntries(t, client)
	if counts["client-attached"] != 1 {
		t.Errorf("client-attached: signal-hydrate entry count = %d, want 1", counts["client-attached"])
	}
	if counts["client-session-changed"] != 1 {
		t.Errorf("client-session-changed: signal-hydrate entry count = %d, want 1", counts["client-session-changed"])
	}

	if len(log.infos()) != 1 {
		t.Fatalf("INFO count = %d, want 1; infos=%v", len(log.infos()), log.infos())
	}
	if log.infoReaped()[0] != 4 {
		t.Errorf("reaped attr = %d, want eviction count 4", log.infoReaped()[0])
	}
}

func TestRegisterPortalHooks_HydrationPreservesUserHookLackingFingerprint(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-mig-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	userHook := `run-shell "tmux-resurrect restore"`
	if err := client.AppendGlobalHook("client-attached", userHook); err != nil {
		t.Fatalf("AppendGlobalHook(user): %v", err)
	}

	log := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, log.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("RegisterPortalHooks: %v", err)
	}

	survivingUser := portalEntryCommandsForEvent(t, client, "client-attached", "tmux-resurrect restore")
	if len(survivingUser) == 0 {
		t.Errorf("user hook was evicted; surviving user entries=%v", survivingUser)
	}

	if len(log.infos()) != 0 {
		t.Errorf("INFO count = %d, want 0 (user hook not Portal-fingerprinted, no eviction); infos=%v", len(log.infos()), log.infos())
	}
}

func TestRegisterPortalHooks_HydrationPerIndexEvictFailureWarnsAndContinues(t *testing.T) {
	var raw strings.Builder
	for _, ev := range tmux.HydrationTriggerEvents {
		fmt.Fprintf(&raw, "%s[0] => %q\n", ev, staleSignalHydrateCommand)
	}

	failingTarget := "client-attached[0]"
	sentinel := errors.New("tmux unset failure")

	mock := commandertest.FromFunc(perEventDispatchWithFaults(t, raw.String(), nil, nil,
		map[string]error{failingTarget: sentinel}))
	client := tmux.NewClient(mock)

	log := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, log.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("RegisterPortalHooks returned err: %v (per-index migration failures must not error)", err)
	}

	var sawFailureWarn bool
	for _, w := range log.warns() {
		if strings.Contains(w, "failed to evict") {
			sawFailureWarn = true
			break
		}
	}
	if !sawFailureWarn {
		t.Errorf("no WARN line with `failed to evict`; warns=%v", log.warns())
	}

	if infos := log.infos(); len(infos) != 1 {
		t.Fatalf("INFO count = %d, want 1; infos=%v", len(infos), infos)
	} else if !strings.Contains(infos[0], "collapsed stacked portal hooks") || log.infoReaped()[0] < 1 {
		t.Errorf("INFO line = %q reaped=%d, missing eviction summary", infos[0], log.infoReaped()[0])
	}
}

func TestRegisterPortalHooks_HydrationScansEveryRuntimeTriggerEvent(t *testing.T) {
	var raw strings.Builder
	for _, ev := range tmux.HydrationTriggerEvents {
		fmt.Fprintf(&raw, "%s[0] => %q\n", ev, staleSignalHydrateCommand)
	}
	mock := commandertest.FromFunc(perEventDispatch(t, raw.String(), nil))
	client := tmux.NewClient(mock)

	log := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, log.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("RegisterPortalHooks: %v", err)
	}

	gotEvents := map[string]bool{}
	for _, u := range unsetHookCalls(mock.Calls()) {
		gotEvents[eventOfUnsetTarget(u)] = true
	}
	for _, want := range tmux.HydrationTriggerEvents {
		if !gotEvents[want] {
			t.Errorf("event %q in HydrationTriggerEvents was NOT scanned by migration; got=%v", want, gotEvents)
		}
	}

	if len(log.infos()) != 1 {
		t.Fatalf("INFO count = %d, want 1; infos=%v", len(log.infos()), log.infos())
	}
	if want := int64(len(tmux.HydrationTriggerEvents)); log.infoReaped()[0] != want {
		t.Errorf("reaped attr = %d, want eviction count = %d", log.infoReaped()[0], want)
	}
}

func TestRegisterPortalHooks_HydrationReadFailureWrapsErrorAndSkipsSetHook(t *testing.T) {
	sentinel := errors.New("tmux show-hooks failure")
	mock := commandertest.FromFunc(perEventDispatchWithFaults(t, "", nil, readErrForAllManagedEvents(sentinel), nil))
	client := tmux.NewClient(mock)

	log := &migrationLog{}
	err := tmux.RegisterPortalHooks(client, log.Logger().With("component", "bootstrap"))

	if err == nil {
		t.Fatal("expected error from RegisterPortalHooks, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
	}

	assertNoSetHookCalls(t, mock.Calls())
	if !strings.Contains(err.Error(), "show-hooks failed") {
		t.Errorf("error %q does not contain expected wrap %q", err.Error(), "show-hooks failed")
	}
	if !strings.Contains(err.Error(), "register hook on client-attached") {
		t.Errorf("error %q missing per-event leg wrap %q", err.Error(), "register hook on client-attached")
	}
}
