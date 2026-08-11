package tmux_test

import (
	"errors"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestResolveHookKey_StampedSession(t *testing.T) {
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

	if err := client.SetSessionOption(sessionName, portalIDLiteral, "tok123"); err != nil {
		t.Fatalf("SetSessionOption(%q, %q, tok123): %v", sessionName, portalIDLiteral, err)
	}

	got, err := client.ResolveHookKey(sessionName)
	if err != nil {
		t.Fatalf("ResolveHookKey(%q): %v", sessionName, err)
	}
	if got != "tok123:0.0" {
		t.Errorf("stamped hook key = %q, want %q (conditional must take the @portal-id branch)", got, "tok123:0.0")
	}
}

func TestResolveHookKey_UnstampedSession(t *testing.T) {
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

	want := sessionName + ":0.0"
	got, err := client.ResolveHookKey(sessionName)
	if err != nil {
		t.Fatalf("ResolveHookKey(%q): %v", sessionName, err)
	}
	if got != want {
		t.Errorf("un-stamped hook key = %q, want %q (unset @portal-id must take the #{session_name} branch)", got, want)
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
		t.Errorf("hook key on read failure = %q, want \"\" (MUST NOT synthesize a name-based key)", got)
	}

	var cmdErr *tmux.CommandError
	if !errors.As(err, &cmdErr) {
		t.Errorf("error %v is not a recoverable *tmux.CommandError (errors.As failed)", err)
	}
}
