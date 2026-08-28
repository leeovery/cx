package tmux_test

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func readHookKey(t *testing.T, ts *tmuxtest.Socket, target string) string {
	t.Helper()
	out := ts.Run(t, "display-message", "-p", "-t", target, tmux.HookKeyFormat)
	return strings.TrimRight(out, "\n")
}

func TestHookKeyFormat_StampedPane(t *testing.T) {
	const sessionName = "hk-stamped"
	ts, _ := seedHookKeyServer(t, sessionName, nil)

	ts.StampPaneToken(t, sessionName+":0.0", "tok123")

	// The harness runs tmux -f /dev/null, so base-index and pane-base-index
	// stay at 0 — hence the ":0.0" targets throughout this file.
	if got := readHookKey(t, ts, sessionName+":0.0"); got != "tok123" {
		t.Errorf("stamped pane hook key = %q, want %q", got, "tok123")
	}
}

func TestHookKeyFormat_UnstampedPane(t *testing.T) {
	const sessionName = "hk-unstamped"
	ts, _ := seedHookKeyServer(t, sessionName, nil)

	if got := readHookKey(t, ts, sessionName+":0.0"); got != "" {
		t.Errorf("un-stamped pane hook key = %q, want an empty key", got)
	}
}

func TestHookKeyFormat_StampIsPerPaneNotPerSession(t *testing.T) {
	const sessionName = "hk-multi"
	ts, _, _ := seedThreePaneStampedSession(t, sessionName, "tokOne")

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
