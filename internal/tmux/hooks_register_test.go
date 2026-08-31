package tmux_test

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/tmux"
)

var expectedSaveTriggerEvents = []string{
	"session-created",
	"session-closed",
	"session-renamed",
	"window-linked",
	"window-unlinked",
	"window-layout-changed",
	"pane-focus-out",
}

const notifyFingerprint = "portal state notify"

const commitNowFingerprint = "portal state commit-now"

const signalHydrateFingerprint = "portal state signal-hydrate"

const expectedNotifyCommand = `run-shell "command -v portal >/dev/null 2>&1 && ` + notifyFingerprint + `"`

const expectedCommitNowCommand = `run-shell "command -v portal >/dev/null 2>&1 && ` + commitNowFingerprint + `"`

const expectedSignalHydrateCommand = `run-shell "command -v portal >/dev/null 2>&1 && ` + signalHydrateFingerprint + ` -- #{session_name}"`

var expectedManagedEventCount = len(expectedSaveTriggerEvents) + len(tmux.HydrationTriggerEvents)

var nonSessionClosedSaveTriggerEvents = []string{
	"session-created",
	"session-renamed",
	"window-linked",
	"window-unlinked",
	"window-layout-changed",
	"pane-focus-out",
}

func perEventDispatch(t *testing.T, seededTable string, setHookErrFor map[string]error) func(args ...string) (string, error) {
	t.Helper()
	return perEventDispatchWithFaults(t, seededTable, setHookErrFor, nil, nil)
}

func perEventDispatchWithFaults(t *testing.T, seededTable string, setHookErrFor, readErrFor, unsetErrFor map[string]error) func(args ...string) (string, error) {
	t.Helper()
	byEvent := parseSeededTableByEvent(seededTable)
	return func(args ...string) (string, error) {
		if len(args) >= 3 && args[0] == "show-hooks" && args[1] == "-g" {
			if readErrFor != nil {
				if err, ok := readErrFor[args[2]]; ok {
					return "", err
				}
			}
			return byEvent[args[2]], nil
		}
		if len(args) >= 2 && args[0] == "show-hooks" && args[1] == "-g" {
			t.Fatalf("convergence engine must read per-event, not the no-arg global show-hooks -g: %v", args)
			return "", nil
		}
		if len(args) >= 4 && args[0] == "set-hook" && args[1] == "-ga" {
			if setHookErrFor != nil {
				if err, ok := setHookErrFor[args[2]]; ok {
					return "", err
				}
			}
			return "", nil
		}
		if len(args) >= 3 && args[0] == "set-hook" && args[1] == "-gu" {
			if unsetErrFor != nil {
				if err, ok := unsetErrFor[args[2]]; ok {
					return "", err
				}
			}
			return "", nil
		}
		t.Fatalf("unexpected command: %v", args)
		return "", nil
	}
}

func readErrForAllManagedEvents(err error) map[string]error {
	m := map[string]error{}
	for _, ev := range tmux.ManagedEventNames() {
		m[ev] = err
	}
	return m
}

func assertNoSetHookCalls(t *testing.T, calls [][]string) {
	t.Helper()
	for _, c := range calls {
		if len(c) >= 2 && c[0] == "set-hook" {
			t.Errorf("set-hook must not be called when show-hooks fails: %v", c)
		}
	}
}

func parseSeededTableByEvent(table string) map[string]string {
	byEvent := map[string]string{}
	for line := range strings.SplitSeq(table, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		open := strings.IndexByte(line, '[')
		if open <= 0 {
			continue
		}
		ev := line[:open]
		byEvent[ev] += line + "\n"
	}
	return byEvent
}

func setHookCalls(calls [][]string) [][2]string {
	var out [][2]string
	for _, c := range calls {
		if len(c) >= 4 && c[0] == "set-hook" && c[1] == "-ga" {
			out = append(out, [2]string{c[2], c[3]})
		}
	}
	return out
}

type setHookEvent struct {
	Verb   string
	Target string
}

func setHookEvents(calls [][]string) []setHookEvent {
	var out []setHookEvent
	for _, c := range calls {
		switch {
		case len(c) >= 4 && c[0] == "set-hook" && c[1] == "-ga":
			out = append(out, setHookEvent{Verb: "-ga", Target: c[2]})
		case len(c) >= 3 && c[0] == "set-hook" && c[1] == "-gu":
			out = append(out, setHookEvent{Verb: "-gu", Target: c[2]})
		}
	}
	return out
}

func eventOfUnsetTarget(target string) string {
	if i := strings.IndexByte(target, '['); i > 0 {
		return target[:i]
	}
	return target
}

// Lines are single-outer-quoted to mirror tmux's show-hooks output (Portal
// bodies contain literal double quotes), which ParseShowHooks strips back off.
func convergedTable() string {
	var b strings.Builder
	for _, e := range expectedSaveTriggerEvents {
		cmd := expectedNotifyCommand
		if e == "session-closed" {
			cmd = expectedCommitNowCommand
		}
		fmt.Fprintf(&b, "%s[0] => '%s'\n", e, cmd)
	}
	for _, e := range tmux.HydrationTriggerEvents {
		fmt.Fprintf(&b, "%s[0] => '%s'\n", e, expectedSignalHydrateCommand)
	}
	return b.String()
}

func TestRegisterPortalHooks_FreshTable(t *testing.T) {
	mock := &MockCommander{RunFunc: perEventDispatch(t, "", nil)}
	client := tmux.NewClient(mock)

	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := setHookCalls(mock.Calls)
	if len(got) != expectedManagedEventCount {
		t.Fatalf("set-hook -ga call count = %d, want %d: %v", len(got), expectedManagedEventCount, got)
	}

	wantBody := map[string]string{}
	for _, ev := range nonSessionClosedSaveTriggerEvents {
		wantBody[ev] = expectedNotifyCommand
	}
	wantBody["session-closed"] = expectedCommitNowCommand
	for _, ev := range tmux.HydrationTriggerEvents {
		wantBody[ev] = expectedSignalHydrateCommand
	}

	seen := map[string]int{}
	for _, c := range got {
		seen[c[0]]++
		if want, ok := wantBody[c[0]]; !ok {
			t.Errorf("unexpected event appended: %q", c[0])
		} else if c[1] != want {
			t.Errorf("event %q body = %q, want %q", c[0], c[1], want)
		}
		if strings.Contains(c[1], "portal state migrate-rename") {
			t.Errorf("event %q registered migrate-rename: %q", c[0], c[1])
		}
	}
	for ev := range wantBody {
		if seen[ev] != 1 {
			t.Errorf("event %q appended %d times, want exactly 1", ev, seen[ev])
		}
	}

	if unsets := unsetHookCalls(mock.Calls); len(unsets) != 0 {
		t.Errorf("expected 0 set-hook -gu on empty table, got %d: %v", len(unsets), unsets)
	}
}

func TestRegisterPortalHooks_IdempotentFastPath(t *testing.T) {
	mock := &MockCommander{RunFunc: perEventDispatch(t, convergedTable(), nil)}
	client := tmux.NewClient(mock)

	logger := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, logger.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if appends := setHookCalls(mock.Calls); len(appends) != 0 {
		t.Errorf("expected 0 set-hook -ga on converged table, got %d: %v", len(appends), appends)
	}
	if unsets := unsetHookCalls(mock.Calls); len(unsets) != 0 {
		t.Errorf("expected 0 set-hook -gu on converged table, got %d: %v", len(unsets), unsets)
	}
	for _, line := range logger.infos() {
		if strings.Contains(line, "reaped") || strings.Contains(line, "collapsed") {
			t.Errorf("unexpected eviction INFO on idempotent fast path: %q", line)
		}
	}
}

func TestRegisterPortalHooks_KDeepStackCollapse(t *testing.T) {
	const k = 5
	var b strings.Builder
	for i := range k {
		fmt.Fprintf(&b, "pane-focus-out[%d] => '%s'\n", i, expectedNotifyCommand)
	}
	mock := &MockCommander{RunFunc: perEventDispatch(t, b.String(), nil)}
	client := tmux.NewClient(mock)

	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var paneFocusUnsets []string
	for _, u := range unsetHookCalls(mock.Calls) {
		if strings.HasPrefix(u, "pane-focus-out[") {
			paneFocusUnsets = append(paneFocusUnsets, u)
		}
	}
	if len(paneFocusUnsets) != k {
		t.Fatalf("pane-focus-out unset count = %d, want %d: %v", len(paneFocusUnsets), k, paneFocusUnsets)
	}
	for i, u := range paneFocusUnsets {
		want := fmt.Sprintf("pane-focus-out[%d]", k-1-i)
		if u != want {
			t.Errorf("unset[%d] = %q, want %q (descending-index order required)", i, u, want)
		}
	}

	appendIdx, lastUnsetIdx := -1, -1
	var appendBody string
	var paneFocusAppends int
	for i, e := range setHookEvents(mock.Calls) {
		if e.Verb == "-ga" && e.Target == "pane-focus-out" {
			paneFocusAppends++
			appendIdx = i
		}
		if e.Verb == "-gu" && strings.HasPrefix(e.Target, "pane-focus-out[") {
			lastUnsetIdx = i
		}
	}
	for _, c := range setHookCalls(mock.Calls) {
		if c[0] == "pane-focus-out" {
			appendBody = c[1]
		}
	}
	if paneFocusAppends != 1 {
		t.Fatalf("pane-focus-out append count = %d, want 1", paneFocusAppends)
	}
	if appendBody != expectedNotifyCommand {
		t.Errorf("pane-focus-out append body = %q, want %q", appendBody, expectedNotifyCommand)
	}
	if appendIdx <= lastUnsetIdx {
		t.Errorf("append (event[%d]) must follow the unsets (last at event[%d])", appendIdx, lastUnsetIdx)
	}
}

func TestRegisterPortalHooks_StaleSignalHydrateMigratesInPlace(t *testing.T) {
	raw := fmt.Sprintf("client-attached[0] => '%s'\n", staleSignalHydrateCommand)
	mock := &MockCommander{RunFunc: perEventDispatch(t, raw, nil)}
	client := tmux.NewClient(mock)

	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var attachedUnsets []string
	for _, u := range unsetHookCalls(mock.Calls) {
		if strings.HasPrefix(u, "client-attached[") {
			attachedUnsets = append(attachedUnsets, u)
		}
	}
	if len(attachedUnsets) != 1 || attachedUnsets[0] != "client-attached[0]" {
		t.Fatalf("client-attached unsets = %v, want [client-attached[0]]", attachedUnsets)
	}

	var appendCount int
	var appendBody string
	for _, c := range setHookCalls(mock.Calls) {
		if c[0] == "client-attached" {
			appendCount++
			appendBody = c[1]
		}
	}
	if appendCount != 1 {
		t.Fatalf("client-attached append count = %d, want 1", appendCount)
	}
	if appendBody != expectedSignalHydrateCommand {
		t.Errorf("client-attached append body = %q, want %q (the -- form)", appendBody, expectedSignalHydrateCommand)
	}
}

func TestRegisterPortalHooks_StaleNotifyOnSessionClosedMigratesToCommitNow(t *testing.T) {
	raw := fmt.Sprintf("session-closed[0] => '%s'\n", expectedNotifyCommand)
	mock := &MockCommander{RunFunc: perEventDispatch(t, raw, nil)}
	client := tmux.NewClient(mock)

	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var closedUnsets []string
	for _, u := range unsetHookCalls(mock.Calls) {
		if strings.HasPrefix(u, "session-closed[") {
			closedUnsets = append(closedUnsets, u)
		}
	}
	if len(closedUnsets) != 1 || closedUnsets[0] != "session-closed[0]" {
		t.Fatalf("session-closed unsets = %v, want [session-closed[0]]", closedUnsets)
	}

	unsetIdx, appendIdx := -1, -1
	var appendBody string
	var closedAppends int
	for i, e := range setHookEvents(mock.Calls) {
		if e.Verb == "-gu" && e.Target == "session-closed[0]" {
			unsetIdx = i
		}
		if e.Verb == "-ga" && e.Target == "session-closed" {
			closedAppends++
			appendIdx = i
		}
	}
	for _, c := range setHookCalls(mock.Calls) {
		if c[0] == "session-closed" {
			appendBody = c[1]
		}
	}
	if closedAppends != 1 {
		t.Fatalf("session-closed append count = %d, want 1", closedAppends)
	}
	if appendBody != expectedCommitNowCommand {
		t.Errorf("session-closed append body = %q, want %q", appendBody, expectedCommitNowCommand)
	}
	if unsetIdx < 0 || appendIdx < 0 || unsetIdx >= appendIdx {
		t.Errorf("unset (event[%d]) must precede append (event[%d])", unsetIdx, appendIdx)
	}
}

func TestRegisterPortalHooks_SessionClosedUnionFastPath(t *testing.T) {
	raw := fmt.Sprintf("session-closed[0] => '%s'\n", expectedCommitNowCommand)
	mock := &MockCommander{RunFunc: perEventDispatch(t, raw, nil)}
	client := tmux.NewClient(mock)

	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, c := range setHookCalls(mock.Calls) {
		if c[0] == "session-closed" {
			t.Errorf("unexpected set-hook -ga on already-converged session-closed: %q", c[1])
		}
	}
	for _, u := range unsetHookCalls(mock.Calls) {
		if strings.HasPrefix(u, "session-closed[") {
			t.Errorf("unexpected set-hook -gu on already-converged session-closed: %q", u)
		}
	}
}

func TestRegisterPortalHooks_SessionClosedSubstringEvictsPortalStateNotifyBody(t *testing.T) {
	const notifyDebugBody = `run-shell "command -v portal >/dev/null 2>&1 && portal state notify --debug"`
	if !strings.Contains(notifyDebugBody, "portal state notify") {
		t.Fatalf("test fixture %q does not contain the substring fingerprint", notifyDebugBody)
	}

	raw := fmt.Sprintf("session-closed[0] => '%s'\n", notifyDebugBody)
	mock := &MockCommander{RunFunc: perEventDispatch(t, raw, nil)}
	client := tmux.NewClient(mock)

	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var closedUnsets []string
	for _, u := range unsetHookCalls(mock.Calls) {
		if strings.HasPrefix(u, "session-closed[") {
			closedUnsets = append(closedUnsets, u)
		}
	}
	if len(closedUnsets) != 1 || closedUnsets[0] != "session-closed[0]" {
		t.Fatalf("session-closed unsets = %v, want [session-closed[0]] (substring predicate must evict the notify body)", closedUnsets)
	}

	var appends int
	var body string
	for _, c := range setHookCalls(mock.Calls) {
		if c[0] == "session-closed" {
			appends++
			body = c[1]
		}
	}
	if appends != 1 {
		t.Fatalf("session-closed append count = %d, want 1", appends)
	}
	if body != expectedCommitNowCommand {
		t.Errorf("session-closed append body = %q, want %q", body, expectedCommitNowCommand)
	}
}

func TestRegisterPortalHooks_SessionClosedNonMatchingUserHookSurvives(t *testing.T) {
	const userHook = `run-shell "tmux-resurrect save"`
	if strings.Contains(userHook, "portal state notify") || strings.Contains(userHook, "portal state commit-now") {
		t.Fatalf("test fixture %q unexpectedly contains a Portal fingerprint", userHook)
	}

	raw := fmt.Sprintf("session-closed[0] => '%s'\n", userHook)
	mock := &MockCommander{RunFunc: perEventDispatch(t, raw, nil)}
	client := tmux.NewClient(mock)

	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, u := range unsetHookCalls(mock.Calls) {
		if strings.HasPrefix(u, "session-closed[") {
			t.Errorf("unexpected unset on non-matching user hook: %q", u)
		}
	}

	var appends int
	var body string
	for _, c := range setHookCalls(mock.Calls) {
		if c[0] == "session-closed" {
			appends++
			body = c[1]
		}
	}
	if appends != 1 {
		t.Fatalf("session-closed append count = %d, want 1 (alongside the user hook)", appends)
	}
	if body != expectedCommitNowCommand {
		t.Errorf("session-closed append body = %q, want %q", body, expectedCommitNowCommand)
	}
}

func TestRegisterPortalHooks_UserHookUntouched(t *testing.T) {
	const userHook = `run-shell "tmux-resurrect save"`
	raw := fmt.Sprintf("pane-focus-out[0] => '%s'\n", userHook)
	mock := &MockCommander{RunFunc: perEventDispatch(t, raw, nil)}
	client := tmux.NewClient(mock)

	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, u := range unsetHookCalls(mock.Calls) {
		if strings.HasPrefix(u, "pane-focus-out[") {
			t.Errorf("unexpected unset on user hook: %q", u)
		}
	}

	var appends int
	var body string
	for _, c := range setHookCalls(mock.Calls) {
		if c[0] == "pane-focus-out" {
			appends++
			body = c[1]
		}
	}
	if appends != 1 {
		t.Fatalf("pane-focus-out append count = %d, want 1 (alongside the user hook)", appends)
	}
	if body != expectedNotifyCommand {
		t.Errorf("pane-focus-out append body = %q, want %q", body, expectedNotifyCommand)
	}
}

func TestRegisterPortalHooks_PerEventReadFailureFolds(t *testing.T) {
	sentinel := errors.New("tmux show-hooks failure on session-renamed")
	mock := &MockCommander{RunFunc: perEventDispatchWithFaults(t, "", nil,
		map[string]error{"session-renamed": sentinel}, nil)}
	client := tmux.NewClient(mock)

	logger := &migrationLog{}
	err := tmux.RegisterPortalHooks(client, logger.Logger().With("component", "bootstrap"))

	if err == nil {
		t.Fatal("expected aggregate error wrapping the sentinel, got nil")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
	}
	if !strings.Contains(err.Error(), "session-renamed") {
		t.Errorf("error %q does not name the failed event session-renamed", err.Error())
	}
	if !strings.Contains(err.Error(), "show-hooks failed") {
		t.Errorf("error %q missing the show-hooks-failed wrap", err.Error())
	}

	if len(logger.warns()) == 0 {
		t.Errorf("expected at least one WARN for the show-hooks failure, got none")
	}

	for _, c := range setHookCalls(mock.Calls) {
		if c[0] == "session-renamed" {
			t.Errorf("session-renamed must not be appended when its read fails: %v", c)
		}
	}

	got := map[string]int{}
	for _, c := range setHookCalls(mock.Calls) {
		got[c[0]]++
	}
	for _, ev := range expectedSaveTriggerEvents {
		if ev == "session-renamed" {
			continue
		}
		if got[ev] != 1 {
			t.Errorf("event %q append count = %d, want 1 (must still converge)", ev, got[ev])
		}
	}
	for _, ev := range tmux.HydrationTriggerEvents {
		if got[ev] != 1 {
			t.Errorf("event %q append count = %d, want 1 (must still converge)", ev, got[ev])
		}
	}
}

func TestRegisterPortalHooks_PerIndexUnsetFailureWarnsAndContinues(t *testing.T) {
	raw := fmt.Sprintf("pane-focus-out[0] => '%s'\npane-focus-out[1] => '%s'\n",
		expectedNotifyCommand, expectedNotifyCommand)
	sentinel := errors.New("tmux unset failed at index 1")
	mock := &MockCommander{RunFunc: perEventDispatchWithFaults(t, raw, nil, nil,
		map[string]error{"pane-focus-out[1]": sentinel})}
	client := tmux.NewClient(mock)

	logger := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, logger.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("unexpected error from RegisterPortalHooks: %v", err)
	}

	var unsets []string
	for _, u := range unsetHookCalls(mock.Calls) {
		if strings.HasPrefix(u, "pane-focus-out[") {
			unsets = append(unsets, u)
		}
	}
	if len(unsets) != 2 || unsets[0] != "pane-focus-out[1]" || unsets[1] != "pane-focus-out[0]" {
		t.Fatalf("pane-focus-out unsets = %v, want [pane-focus-out[1] pane-focus-out[0]]", unsets)
	}

	if len(logger.warns()) == 0 {
		t.Errorf("expected at least one WARN for the per-index unset failure, got none")
	}

	var appended bool
	for _, c := range setHookCalls(mock.Calls) {
		if c[0] == "pane-focus-out" && c[1] == expectedNotifyCommand {
			appended = true
			break
		}
	}
	if !appended {
		t.Errorf("expected notifyCommand appended after partial eviction, none recorded")
	}
}

func TestRegisterPortalHooks_SingleReapedInfoOnEviction(t *testing.T) {
	const staleNotify = `run-shell "portal state notify"`
	var b strings.Builder
	fmt.Fprintf(&b, "window-linked[0] => '%s'\n", expectedNotifyCommand)
	fmt.Fprintf(&b, "window-linked[1] => '%s'\n", expectedNotifyCommand)
	fmt.Fprintf(&b, "session-created[0] => '%s'\n", staleNotify)
	mock := &MockCommander{RunFunc: perEventDispatch(t, b.String(), nil)}
	client := tmux.NewClient(mock)

	logger := &migrationLog{}
	if err := tmux.RegisterPortalHooks(client, logger.Logger().With("component", "bootstrap")); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	infos := logger.infos()
	if len(infos) != 1 {
		t.Fatalf("INFO line count = %d, want 1; infos=%v", len(infos), infos)
	}
	if !strings.HasPrefix(infos[0], "[bootstrap] ") {
		t.Errorf("INFO line %q not bound to the bootstrap component", infos[0])
	}
	if logger.infoReaped()[0] != 3 {
		t.Errorf("reaped attr = %d, want 3 (2 window-linked + 1 session-created)", logger.infoReaped()[0])
	}
}

func TestRegisterPortalHooks_NoReapedInfoOnZeroEviction(t *testing.T) {
	t.Run("fresh table (all appends, no evictions)", func(t *testing.T) {
		mock := &MockCommander{RunFunc: perEventDispatch(t, "", nil)}
		client := tmux.NewClient(mock)

		logger := &migrationLog{}
		if err := tmux.RegisterPortalHooks(client, logger.Logger().With("component", "bootstrap")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logger.infos()) != 0 {
			t.Errorf("expected 0 INFO lines on zero-eviction fresh table, got %d: %v", len(logger.infos()), logger.infos())
		}
	})

	t.Run("converged table (all fast-path, no evictions)", func(t *testing.T) {
		mock := &MockCommander{RunFunc: perEventDispatch(t, convergedTable(), nil)}
		client := tmux.NewClient(mock)

		logger := &migrationLog{}
		if err := tmux.RegisterPortalHooks(client, logger.Logger().With("component", "bootstrap")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(logger.infos()) != 0 {
			t.Errorf("expected 0 INFO lines on all-fast-path converged table, got %d: %v", len(logger.infos()), logger.infos())
		}
	})
}

func TestSignalHydrateCommand_HasEndOfFlagsSeparator(t *testing.T) {
	t.Run("signalHydrateCommand resolves with -- before #{session_name}", func(t *testing.T) {
		want := `run-shell "command -v portal >/dev/null 2>&1 && portal state signal-hydrate -- #{session_name}"`
		if expectedSignalHydrateCommand != want {
			t.Errorf("expectedSignalHydrateCommand = %q, want %q", expectedSignalHydrateCommand, want)
		}
		if !strings.Contains(expectedSignalHydrateCommand, " -- #{session_name}") {
			t.Errorf("expectedSignalHydrateCommand %q missing ` -- #{session_name}` separator", expectedSignalHydrateCommand)
		}
	})

	t.Run("RegisterPortalHooks emits the -- separator on every hydration event", func(t *testing.T) {
		mock := &MockCommander{RunFunc: perEventDispatch(t, "", nil)}
		client := tmux.NewClient(mock)

		if err := tmux.RegisterPortalHooks(client, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := map[string]string{}
		for _, c := range setHookCalls(mock.Calls) {
			got[c[0]] = c[1]
		}
		for _, ev := range tmux.HydrationTriggerEvents {
			cmd := got[ev]
			if !strings.Contains(cmd, "portal state signal-hydrate -- #{session_name}") {
				t.Errorf("event %q command = %q, missing `signal-hydrate -- #{session_name}`", ev, cmd)
			}
		}
	})
}

func TestRegisterPortalHooks_NoMigrateRename(t *testing.T) {
	mock := &MockCommander{RunFunc: perEventDispatch(t, "", nil)}
	client := tmux.NewClient(mock)

	if err := tmux.RegisterPortalHooks(client, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, c := range setHookCalls(mock.Calls) {
		if strings.Contains(c[1], "portal state migrate-rename") {
			t.Errorf("unexpected migrate-rename registration on event %q: %q", c[0], c[1])
		}
		if c[1] != expectedNotifyCommand && c[1] != expectedCommitNowCommand && c[1] != expectedSignalHydrateCommand {
			t.Errorf("unexpected command body on event %q: %q", c[0], c[1])
		}
	}
}

// migrationLog captures the hook-convergence lines, rendered as
// "[<component>] <message>" so a caller asserts on the component binding and
// the message together.
type migrationLog struct {
	sink logtest.Sink
}

func (m *migrationLog) Logger() *slog.Logger { return slog.New(&m.sink) }

func (m *migrationLog) infos() []string { return migrationLines(&m.sink, slog.LevelInfo) }

func (m *migrationLog) warns() []string { return migrationLines(&m.sink, slog.LevelWarn) }

func (m *migrationLog) infoReaped() []int64 {
	var out []int64
	for _, rec := range m.sink.RecordsAtExactLevel(slog.LevelInfo) {
		out = append(out, migrationReaped(rec))
	}
	return out
}

func migrationLines(sink *logtest.Sink, level slog.Level) []string {
	var out []string
	for _, rec := range sink.RecordsAtExactLevel(level) {
		out = append(out, migrationLine(rec))
	}
	return out
}

func migrationLine(rec logtest.Record) string {
	return "[" + attrOrEmpty(rec, "component") + "] " + rec.Msg
}

// migrationReaped stands in -1 for a line carrying no reaped attr, so a caller
// asserting on the count sees the absence rather than a zero.
func migrationReaped(rec logtest.Record) int64 {
	v, ok := rec.Attrs["reaped"]
	if !ok {
		return -1
	}
	return v.Int64()
}
