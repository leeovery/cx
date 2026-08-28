package tmux_test

import (
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func readHookKey(t *testing.T, ts *tmuxtest.Socket, target string) string {
	t.Helper()
	out := ts.Run(t, "display-message", "-p", "-t", target, tmux.HookKeyFormat)
	return strings.TrimRight(out, "\n")
}

func TestHookKeyFormat_StampedPane(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "hookkey-")
	client := ts.Client()

	const sessionName = "hk-stamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	if err := client.SetPaneOption(sessionName+":0.0", state.PortalPaneIDOption, "tok123"); err != nil {
		t.Fatalf("SetPaneOption: %v", err)
	}

	// The harness runs tmux -f /dev/null, so base-index and pane-base-index
	// stay at 0 — hence the ":0.0" targets throughout this file.
	if got := readHookKey(t, ts, sessionName+":0.0"); got != "tok123" {
		t.Errorf("stamped pane hook key = %q, want %q", got, "tok123")
	}
}

func TestHookKeyFormat_UnstampedPane(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "hookkey-")
	client := ts.Client()

	const sessionName = "hk-unstamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	if got := readHookKey(t, ts, sessionName+":0.0"); got != "" {
		t.Errorf("un-stamped pane hook key = %q, want an empty key", got)
	}
}

func TestHookKeyFormat_StampIsPerPaneNotPerSession(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "hookkey-")
	client := ts.Client()

	const sessionName = "hk-multi"
	seedThreePaneStampedSession(t, ts, client, sessionName, "tokOne")

	cases := []struct {
		name   string
		target string
		want   string
	}{
		{name: "the stamped pane", target: sessionName + ":0.0", want: "tokOne"},
		{name: "its split sibling", target: sessionName + ":0.1", want: ""},
		{name: "a pane in another window", target: sessionName + ":1.0", want: ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := readHookKey(t, ts, tc.target); got != tc.want {
				t.Errorf("hook key for %q = %q, want %q", tc.target, got, tc.want)
			}
		})
	}
}
