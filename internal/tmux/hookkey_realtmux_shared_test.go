package tmux_test

import (
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func seedThreePaneStampedSession(t *testing.T, ts *tmuxtest.Socket, client *tmux.Client, sessionName, portalID string) []string {
	t.Helper()
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)
	if err := client.SetSessionOption(sessionName, portalIDLiteral, portalID); err != nil {
		t.Fatalf("SetSessionOption(%q, %q, %q): %v", sessionName, portalIDLiteral, portalID, err)
	}
	if err := client.SplitWindow(sessionName+":0", "", ""); err != nil {
		t.Fatalf("SplitWindow(%q): %v", sessionName+":0", err)
	}
	if err := client.NewWindow(sessionName, "", "", ""); err != nil {
		t.Fatalf("NewWindow(%q): %v", sessionName, err)
	}
	return sessionPaneIDs(t, ts, sessionName)
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
