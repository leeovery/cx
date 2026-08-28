package tmux_test

import (
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// hookKeyFixtureSocketPrefix names the temp dir of every hook-key fixture's
// socket, so a stray server left behind by any of these suites is recognisable.
const hookKeyFixtureSocketPrefix = "ptl-hookkey-"

// seedHookKeyServer starts an isolated tmux server holding one live session and
// returns the socket and its client. topology, when non-nil, runs once the
// session answers and shapes whatever windows and panes the caller needs.
func seedHookKeyServer(t *testing.T, sessionName string, topology func(ts *tmuxtest.Socket, client *tmux.Client)) (*tmuxtest.Socket, *tmux.Client) {
	t.Helper()
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, hookKeyFixtureSocketPrefix)
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)
	if topology != nil {
		topology(ts, client)
	}
	return ts, client
}

// seedThreePaneStampedSession builds a three-pane session and stamps the token
// onto the first pane only, so callers have a stamped pane and two unstamped
// siblings to tell apart.
func seedThreePaneStampedSession(t *testing.T, sessionName, token string) (*tmuxtest.Socket, *tmux.Client, []string) {
	t.Helper()
	ts, client := seedHookKeyServer(t, sessionName, func(ts *tmuxtest.Socket, client *tmux.Client) {
		if err := client.SplitWindow(sessionName+":0", "", ""); err != nil {
			t.Fatalf("SplitWindow(%q): %v", sessionName+":0", err)
		}
		if err := client.NewWindow(sessionName, "", "", ""); err != nil {
			t.Fatalf("NewWindow(%q): %v", sessionName, err)
		}
	})
	paneIDs := sessionPaneIDs(t, ts, sessionName)
	if len(paneIDs) == 0 {
		t.Fatalf("session %q reported no panes", sessionName)
	}
	ts.StampPaneToken(t, paneIDs[0], token)
	return ts, client, paneIDs
}

func sessionPaneIDs(t *testing.T, ts *tmuxtest.Socket, session string) []string {
	t.Helper()
	out := ts.Run(t, "list-panes", "-s", "-t", session, "-F", "#{pane_id}")
	var ids []string
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if id := strings.TrimSpace(line); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}
