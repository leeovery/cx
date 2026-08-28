package tmux_test

import (
	"errors"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestResolveHookKey_StampedPane(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hookkey-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const sessionName = "rhk-stamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	const paneTarget = sessionName + ":0.0"
	if err := client.SetPaneOption(paneTarget, state.PortalPaneIDOption, "tok123"); err != nil {
		t.Fatalf("SetPaneOption(%q, %q, tok123): %v", paneTarget, state.PortalPaneIDOption, err)
	}

	got, err := client.ResolveHookKey(paneTarget)
	if err != nil {
		t.Fatalf("ResolveHookKey(%q): %v", paneTarget, err)
	}
	if got != "tok123" {
		t.Errorf("stamped pane hook key = %q, want %q", got, "tok123")
	}
}

func TestResolveHookKey_UnstampedPane(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hookkey-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const sessionName = "rhk-unstamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	got, err := client.ResolveHookKey(sessionName + ":0.0")
	if err != nil {
		t.Fatalf("ResolveHookKey on a live un-stamped pane: %v", err)
	}
	if got != "" {
		t.Errorf("un-stamped pane hook key = %q, want an empty token", got)
	}
}

func TestResolveHookKey_ReadFailureWrapsError(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hookkey-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	// tmux 3.7's display-message tolerates a bogus -t target (it returns ":."
	// with exit 0), so killing the server first is the only reliable way to
	// drive the read-failure path.
	ts.KillServer()

	got, err := client.ResolveHookKey("%nonexistent")
	if err == nil {
		t.Fatal("expected a wrapped error from a failed display-message read, got nil")
	}
	if got != "" {
		t.Errorf("hook key on read failure = %q, want \"\" (a failed read MUST NOT be reported as a resolved key)", got)
	}

	var cmdErr *tmux.CommandError
	if !errors.As(err, &cmdErr) {
		t.Errorf("error %v is not a recoverable *tmux.CommandError (errors.As failed)", err)
	}
}
