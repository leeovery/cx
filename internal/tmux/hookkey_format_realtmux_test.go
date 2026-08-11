package tmux_test

import (
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func readHookKey(t *testing.T, ts *tmuxtest.Socket, target string) string {
	t.Helper()
	out := ts.Run(t, "display-message", "-p", "-t", target, tmux.HookKeyFormat)
	return strings.TrimRight(out, "\n")
}

func TestHookKeyFormat_StampedSession(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "hookkey-")
	client := ts.Client()

	const sessionName = "hk-stamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	if err := client.SetSessionOption(sessionName, portalIDLiteral, "tok123"); err != nil {
		t.Fatalf("SetSessionOption(%q, %q, %q): %v", sessionName, portalIDLiteral, "tok123", err)
	}

	// The harness runs tmux -f /dev/null, so base-index and pane-base-index
	// stay at 0 — hence the ":0.0" suffixes throughout this file.
	if got := readHookKey(t, ts, sessionName); got != "tok123:0.0" {
		t.Errorf("stamped session hook key = %q, want %q (conditional must take the @portal-id branch)", got, "tok123:0.0")
	}
}

func TestHookKeyFormat_UnstampedSession(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "hookkey-")
	client := ts.Client()

	const sessionName = "hk-unstamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	want := sessionName + ":0.0"
	if got := readHookKey(t, ts, sessionName); got != want {
		t.Errorf("un-stamped session hook key = %q, want %q (unset @portal-id must take the #{session_name} branch)", got, want)
	}
}

func TestHookKeyFormat_MultiWindowMultiPane(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "hookkey-")
	client := ts.Client()

	const sessionName = "hk-multi"
	seedThreePaneStampedSession(t, ts, client, sessionName, "tokMulti")

	cases := []struct {
		name   string
		target string
		want   string
	}{
		{name: "window 0 pane 0", target: sessionName + ":0.0", want: "tokMulti:0.0"},
		{name: "window 0 pane 1", target: sessionName + ":0.1", want: "tokMulti:0.1"},
		{name: "window 1 pane 0", target: sessionName + ":1.0", want: "tokMulti:1.0"},
	}

	seen := make(map[string]struct{}, len(cases))
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := readHookKey(t, ts, tc.target)
			if got != tc.want {
				t.Errorf("hook key for %q = %q, want %q", tc.target, got, tc.want)
			}
			if !strings.HasPrefix(got, "tokMulti:") {
				t.Errorf("hook key %q does not share the single @portal-id prefix %q", got, "tokMulti:")
			}
			if _, dup := seen[got]; dup {
				t.Errorf("hook key %q is not distinct across panes (duplicate suffix)", got)
			}
			seen[got] = struct{}{}
		})
	}
}
