package tmux_test

import (
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmuxtest"
)

// The enumeration is the one read every hook-key consumer shares, so what it
// answers for a mixed population is the property worth pinning against a real
// server: tmux's own per-pane option resolution, not a fake's.
func TestPaneTokenEnumeration_PerPaneTokensAreDistinct(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-xsite-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const sessionName = "xsite-multi"
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

	stampPaneToken(t, ts, sessionName+":0.0", "tokA")
	stampPaneToken(t, ts, sessionName+":0.1", "tokB")

	rows, err := client.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}

	for location, wantToken := range map[string]string{
		sessionName + ":0.0": "tokA",
		sessionName + ":0.1": "tokB",
		sessionName + ":1.0": "",
	} {
		if got := findPaneHookRow(t, rows, location).Token; got != wantToken {
			t.Errorf("pane %s token = %q, want %q", location, got, wantToken)
		}
	}
}
