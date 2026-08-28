package tmux_test

import (
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// seedThreePaneStampedSession builds a three-pane session and stamps the token
// onto the first pane only, so callers have a stamped pane and two unstamped
// siblings to tell apart.
func seedThreePaneStampedSession(t *testing.T, ts *tmuxtest.Socket, client *tmux.Client, sessionName, token string) []string {
	t.Helper()
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)
	if err := client.SplitWindow(sessionName+":0", "", ""); err != nil {
		t.Fatalf("SplitWindow(%q): %v", sessionName+":0", err)
	}
	if err := client.NewWindow(sessionName, "", "", ""); err != nil {
		t.Fatalf("NewWindow(%q): %v", sessionName, err)
	}
	paneIDs := sessionPaneIDs(t, ts, sessionName)
	if len(paneIDs) == 0 {
		t.Fatalf("session %q reported no panes", sessionName)
	}
	if err := client.SetPaneOption(paneIDs[0], state.PortalPaneIDOption, token); err != nil {
		t.Fatalf("SetPaneOption(%q, %q, %q): %v", paneIDs[0], state.PortalPaneIDOption, token, err)
	}
	return paneIDs
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
