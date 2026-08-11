package tmux_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

func dispatchUnregisterHooks(t *testing.T, showOutput string, unsetErrFor map[string]error) func(args ...string) (string, error) {
	t.Helper()
	return perEventDispatchWithFaults(t, showOutput, nil, nil, unsetErrFor)
}

func linesForEvent(showOutput, event string) string {
	return parseSeededTableByEvent(showOutput)[event]
}

func unsetHookCalls(calls [][]string) []string {
	var out []string
	for _, c := range calls {
		if len(c) >= 3 && c[0] == "set-hook" && c[1] == "-gu" {
			out = append(out, c[2])
		}
	}
	return out
}

func TestUnregisterPortalHooks(t *testing.T) {
	t.Run("removes a single Portal entry from an otherwise-empty array", func(t *testing.T) {
		raw := `session-created[0] => "run-shell \"command -v portal >/dev/null 2>&1 && portal state notify\""` + "\n"
		mock := &MockCommander{RunFunc: dispatchUnregisterHooks(t, raw, nil)}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := unsetHookCalls(mock.Calls)
		want := []string{"session-created[0]"}
		if len(got) != len(want) {
			t.Fatalf("set-hook -gu calls = %v, want %v", got, want)
		}
		for i, g := range got {
			if g != want[i] {
				t.Errorf("call[%d] = %q, want %q", i, g, want[i])
			}
		}
	})

	t.Run("removes interleaved Portal entries and leaves user entries in place", func(t *testing.T) {
		raw := "session-created[0] run-shell 'tmux-resurrect save'\n" +
			"session-created[1] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"session-created[2] run-shell 'user custom hook'\n" +
			"session-created[3] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n"
		mock := &MockCommander{RunFunc: dispatchUnregisterHooks(t, raw, nil)}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := unsetHookCalls(mock.Calls)
		want := []string{"session-created[3]", "session-created[1]"}
		if len(got) != len(want) {
			t.Fatalf("set-hook -gu calls = %v, want %v", got, want)
		}
		for i, g := range got {
			if g != want[i] {
				t.Errorf("call[%d] = %q, want %q", i, g, want[i])
			}
		}
		for _, g := range got {
			if g == "session-created[0]" || g == "session-created[2]" {
				t.Errorf("user entry incorrectly removed: %q", g)
			}
		}
	})

	t.Run("removes entries in reverse index order", func(t *testing.T) {
		raw := "session-closed[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"session-closed[2] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"session-closed[4] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n"
		mock := &MockCommander{RunFunc: dispatchUnregisterHooks(t, raw, nil)}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := unsetHookCalls(mock.Calls)
		want := []string{"session-closed[4]", "session-closed[2]", "session-closed[0]"}
		if len(got) != len(want) {
			t.Fatalf("set-hook -gu calls = %v, want %v", got, want)
		}
		for i, g := range got {
			if g != want[i] {
				t.Errorf("call[%d] = %q, want %q", i, g, want[i])
			}
		}
	})

	t.Run("removes both Portal entries on session-renamed (notify and migrate-rename)", func(t *testing.T) {
		raw := "session-renamed[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"session-renamed[1] run-shell 'command -v portal >/dev/null 2>&1 && portal state migrate-rename old new'\n"
		mock := &MockCommander{RunFunc: dispatchUnregisterHooks(t, raw, nil)}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		got := unsetHookCalls(mock.Calls)
		want := []string{"session-renamed[1]", "session-renamed[0]"}
		if len(got) != len(want) {
			t.Fatalf("set-hook -gu calls = %v, want %v", got, want)
		}
		for i, g := range got {
			if g != want[i] {
				t.Errorf("call[%d] = %q, want %q", i, g, want[i])
			}
		}
	})

	t.Run("is a no-op when no Portal entries are present", func(t *testing.T) {
		raw := "session-created[0] run-shell 'tmux-resurrect save'\n" +
			"session-closed[0] run-shell 'user-defined notify'\n"
		mock := &MockCommander{RunFunc: dispatchUnregisterHooks(t, raw, nil)}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := unsetHookCalls(mock.Calls); len(got) != 0 {
			t.Errorf("expected 0 set-hook -gu calls, got %d: %v", len(got), got)
		}
	})

	t.Run("ignores matching substrings on events outside Portal's event list", func(t *testing.T) {
		raw := "window-renamed[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"after-select-pane[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state migrate-rename a b'\n"
		mock := &MockCommander{RunFunc: dispatchUnregisterHooks(t, raw, nil)}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := unsetHookCalls(mock.Calls); len(got) != 0 {
			t.Errorf("expected 0 set-hook -gu calls (events outside portalEvents), got %d: %v", len(got), got)
		}
	})

	t.Run("returns the aggregate error and removes nothing when every per-event read fails", func(t *testing.T) {
		sentinel := errors.New("tmux exec failed")
		mock := &MockCommander{
			RunFunc: perEventDispatchWithFaults(t, "", nil, readErrForAllManagedEvents(sentinel), nil),
		}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
		}
		if !strings.Contains(err.Error(), "show-hooks failed") {
			t.Errorf("error %q does not contain expected wrap message %q", err.Error(), "show-hooks failed")
		}
		if got := unsetHookCalls(mock.Calls); len(got) != 0 {
			t.Errorf("expected 0 removals when every read fails, got %d: %v", len(got), got)
		}
	})

	t.Run("folds a single-event read failure into the aggregate and still reaps Portal entries on other events", func(t *testing.T) {
		sentinel := errors.New("transient show-hooks failure")
		raw := "session-created[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"pane-focus-out[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n"
		mock := &MockCommander{
			RunFunc: perEventDispatchWithFaults(t, raw, nil, map[string]error{"pane-focus-out": sentinel}, nil),
		}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err == nil {
			t.Fatal("expected aggregate error from the failing event, got nil")
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("error %v does not wrap sentinel %v", err, sentinel)
		}
		if !strings.Contains(err.Error(), "show-hooks failed") {
			t.Errorf("error %q does not contain expected wrap message %q", err.Error(), "show-hooks failed")
		}
		got := unsetHookCalls(mock.Calls)
		want := []string{"session-created[0]"}
		if len(got) != len(want) {
			t.Fatalf("set-hook -gu calls = %v, want %v (readable event still torn down)", got, want)
		}
		for i, g := range got {
			if g != want[i] {
				t.Errorf("call[%d] = %q, want %q", i, g, want[i])
			}
		}
	})

	t.Run("attempts every removal even when one set-hook -gu call fails", func(t *testing.T) {
		raw := "session-created[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"session-created[1] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"session-created[2] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n"
		failures := map[string]error{
			"session-created[1]": errors.New("transient tmux failure"),
		}
		mock := &MockCommander{RunFunc: dispatchUnregisterHooks(t, raw, failures)}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err == nil {
			t.Fatal("expected aggregate error, got nil")
		}
		got := unsetHookCalls(mock.Calls)
		want := []string{"session-created[2]", "session-created[1]", "session-created[0]"}
		if len(got) != len(want) {
			t.Fatalf("set-hook -gu calls = %v, want %v (every removal attempted)", got, want)
		}
		for i, g := range got {
			if g != want[i] {
				t.Errorf("call[%d] = %q, want %q", i, g, want[i])
			}
		}
	})

	t.Run("returns a joined error naming every failed index", func(t *testing.T) {
		sentinelA := errors.New("tmux failure A")
		sentinelB := errors.New("tmux failure B")
		raw := "session-created[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"client-attached[2] run-shell 'command -v portal >/dev/null 2>&1 && portal state signal-hydrate #{session_name}'\n"
		failures := map[string]error{
			"session-created[0]": sentinelA,
			"client-attached[2]": sentinelB,
		}
		mock := &MockCommander{RunFunc: dispatchUnregisterHooks(t, raw, failures)}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !errors.Is(err, sentinelA) {
			t.Errorf("error %v does not wrap sentinelA %v", err, sentinelA)
		}
		if !errors.Is(err, sentinelB) {
			t.Errorf("error %v does not wrap sentinelB %v", err, sentinelB)
		}
		if !strings.Contains(err.Error(), "session-created") {
			t.Errorf("error %q does not name failed event session-created", err.Error())
		}
		if !strings.Contains(err.Error(), "client-attached") {
			t.Errorf("error %q does not name failed event client-attached", err.Error())
		}
		if !strings.Contains(err.Error(), "[0]") || !strings.Contains(err.Error(), "[2]") {
			t.Errorf("error %q does not name both indices", err.Error())
		}
	})

	t.Run("idempotent: a second run after a successful removal does nothing", func(t *testing.T) {
		var removed bool
		runFunc := func(args ...string) (string, error) {
			if len(args) >= 3 && args[0] == "show-hooks" && args[1] == "-g" {
				if removed {
					return "", nil
				}
				return linesForEvent(
					"session-created[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n",
					args[2],
				), nil
			}
			if len(args) >= 3 && args[0] == "set-hook" && args[1] == "-gu" {
				removed = true
				return "", nil
			}
			t.Fatalf("unexpected command: %v", args)
			return "", nil
		}
		mock := &MockCommander{RunFunc: runFunc}
		client := tmux.NewClient(mock)

		if err := tmux.UnregisterPortalHooks(client); err != nil {
			t.Fatalf("first run: unexpected error: %v", err)
		}
		firstRunRemovals := len(unsetHookCalls(mock.Calls))
		if firstRunRemovals != 1 {
			t.Fatalf("first run set-hook -gu count = %d, want 1", firstRunRemovals)
		}

		mock.Calls = nil
		if err := tmux.UnregisterPortalHooks(client); err != nil {
			t.Fatalf("second run: unexpected error: %v", err)
		}
		if got := unsetHookCalls(mock.Calls); len(got) != 0 {
			t.Errorf("second run produced %d removals, want 0 (idempotent): %v", len(got), got)
		}
	})

	t.Run("does not match user entries that mention 'portal' but not Portal commands", func(t *testing.T) {
		raw := "session-created[0] run-shell 'echo my portal is open'\n" +
			"session-closed[0] run-shell 'echo migrating my portal'\n"
		mock := &MockCommander{RunFunc: dispatchUnregisterHooks(t, raw, nil)}
		client := tmux.NewClient(mock)

		err := tmux.UnregisterPortalHooks(client)

		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got := unsetHookCalls(mock.Calls); len(got) != 0 {
			t.Errorf("expected 0 set-hook -gu calls (no Portal command substrings present), got %d: %v",
				len(got), got)
		}
	})

	t.Run("integration smoke: removes Portal entries across multiple Portal events", func(t *testing.T) {
		raw := "session-created[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"client-attached[1] run-shell 'command -v portal >/dev/null 2>&1 && portal state signal-hydrate #{session_name}'\n" +
			"session-renamed[0] run-shell 'command -v portal >/dev/null 2>&1 && portal state notify'\n" +
			"session-renamed[1] run-shell 'command -v portal >/dev/null 2>&1 && portal state migrate-rename a b'\n"
		mock := &MockCommander{RunFunc: dispatchUnregisterHooks(t, raw, nil)}
		client := tmux.NewClient(mock)

		if err := tmux.UnregisterPortalHooks(client); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		got := unsetHookCalls(mock.Calls)
		want := []string{
			"session-created[0]",
			"session-renamed[1]",
			"session-renamed[0]",
			"client-attached[1]",
		}
		if len(got) != len(want) {
			t.Fatalf("set-hook -gu calls = %v, want %v", got, want)
		}
		for i, g := range got {
			if g != want[i] {
				t.Errorf("call[%d] = %q, want %q", i, g, want[i])
			}
		}
	})
}
