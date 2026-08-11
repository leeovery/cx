package tmux_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestListAllPaneHookKeys_StampedSession(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hookkeys-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const sessionName = "lapk-stamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	if err := client.SetSessionOption(sessionName, portalIDLiteral, "tok123"); err != nil {
		t.Fatalf("SetSessionOption(%q, %q, %q): %v", sessionName, portalIDLiteral, "tok123", err)
	}

	keys, err := client.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}
	if !slices.Contains(keys, "tok123:0.0") {
		t.Errorf("stamped session hook key %q not found in %v (conditional must take the @portal-id branch)", "tok123:0.0", keys)
	}
}

func TestListAllPaneHookKeys_UnstampedSession(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hookkeys-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const sessionName = "lapk-unstamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	want := sessionName + ":0.0"
	keys, err := client.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}
	if !slices.Contains(keys, want) {
		t.Errorf("un-stamped session hook key %q not found in %v (unset @portal-id must take the #{session_name} branch)", want, keys)
	}
}

func TestListAllPaneHookKeys_MultiWindowMultiPane(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hookkeys-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const sessionName = "lapk-multi"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	if err := client.SetSessionOption(sessionName, portalIDLiteral, "tokMulti"); err != nil {
		t.Fatalf("SetSessionOption(%q, %q, %q): %v", sessionName, portalIDLiteral, "tokMulti", err)
	}

	ts.Run(t, "split-window", "-t", sessionName+":0")
	ts.Run(t, "new-window", "-t", sessionName)

	keys, err := client.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}

	want := []string{"tokMulti:0.0", "tokMulti:0.1", "tokMulti:1.0"}
	for _, w := range want {
		if !slices.Contains(keys, w) {
			t.Errorf("expected distinct hook key %q under the shared id, not found in %v", w, keys)
		}
	}
}

func TestListAllPaneHookKeys_MixedStampedAndUnstamped(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hookkeys-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const stamped = "lapk-mix-stamped"
	const unstamped = "lapk-mix-unstamped"
	if err := client.NewSession(stamped, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", stamped, err)
	}
	ts.WaitForSession(t, stamped, 2*time.Second)
	if err := client.NewSession(unstamped, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", unstamped, err)
	}
	ts.WaitForSession(t, unstamped, 2*time.Second)

	if err := client.SetSessionOption(stamped, portalIDLiteral, "tokMix"); err != nil {
		t.Fatalf("SetSessionOption(%q, %q, %q): %v", stamped, portalIDLiteral, "tokMix", err)
	}

	keys, err := client.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}

	if !slices.Contains(keys, "tokMix:0.0") {
		t.Errorf("stamped session key %q not found in %v (must take the @portal-id branch)", "tokMix:0.0", keys)
	}
	unstampedWant := unstamped + ":0.0"
	if !slices.Contains(keys, unstampedWant) {
		t.Errorf("un-stamped session key %q not found in %v (must take the #{session_name} branch)", unstampedWant, keys)
	}
}

func TestListAllPaneHookKeys_EmptyOutputReturnsNonNilEmptySlice(t *testing.T) {
	client := tmux.NewClient(&MockCommander{Output: ""})

	keys, err := client.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys on empty output: unexpected error %v", err)
	}
	if keys == nil {
		t.Fatal("ListAllPaneHookKeys returned a nil slice on empty output, want non-nil []string{}")
	}
	if len(keys) != 0 {
		t.Errorf("ListAllPaneHookKeys on empty output = %v (len %d), want empty slice", keys, len(keys))
	}
}

func TestListAllPaneHookKeys_ListPanesFailurePropagates(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hookkeys-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	// Tear the server down so the subsequent list-panes -a fails with "no
	// server running" — the reliable read-failure path.
	ts.KillServer()

	keys, err := client.ListAllPaneHookKeys()
	if err == nil {
		t.Fatal("expected a wrapped error from a failed list-panes -a read, got nil")
	}
	if keys != nil {
		t.Errorf("hook keys on read failure = %v, want nil (MUST NOT treat a tmux failure as an empty live set)", keys)
	}

	var cmdErr *tmux.CommandError
	if !errors.As(err, &cmdErr) {
		t.Errorf("error %v is not a recoverable *tmux.CommandError (errors.As failed)", err)
	}
}
