package tmux_test

import (
	"errors"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestListAllPaneHookKeys_StampedAndUnstampedInOneRead(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-hookkeys-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const sessionName = "lapk-mix"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)
	ts.Run(t, "split-window", "-t", sessionName+":0")

	stampPaneToken(t, ts, sessionName+":0.0", "tokMix")

	rows, err := client.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}

	stamped := findPaneHookRow(t, rows, sessionName+":0.0")
	if stamped.Token != "tokMix" {
		t.Errorf("stamped pane token = %q, want %q", stamped.Token, "tokMix")
	}
	sibling := findPaneHookRow(t, rows, sessionName+":0.1")
	if sibling.Token != "" {
		t.Errorf("unstamped sibling token = %q, want empty (a stamp does not inherit across a split)", sibling.Token)
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

	rows, err := client.ListAllPaneHookKeys()
	if err == nil {
		t.Fatal("expected a wrapped error from a failed list-panes -a read, got nil")
	}
	if rows != nil {
		t.Errorf("rows on read failure = %+v, want nil (MUST NOT treat a tmux failure as an empty live set)", rows)
	}

	var cmdErr *tmux.CommandError
	if !errors.As(err, &cmdErr) {
		t.Errorf("error %v is not a recoverable *tmux.CommandError (errors.As failed)", err)
	}
}

func stampPaneToken(t *testing.T, ts *tmuxtest.Socket, paneTarget, token string) {
	t.Helper()
	ts.Run(t, "set-option", "-p", "-t", paneTarget, state.PortalPaneIDOption, token)
}

func findPaneHookRow(t *testing.T, rows []tmux.PaneHookRow, location string) tmux.PaneHookRow {
	t.Helper()
	for _, row := range rows {
		if row.Location == location {
			return row
		}
	}
	t.Fatalf("no row for pane %q in %+v", location, rows)
	return tmux.PaneHookRow{}
}
