package tmux_test

import (
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmuxtest"
)

// Registration reads one pane and the sweep enumerates them all; the two must
// answer with the same token for the same pane, or a hook is written under a key
// the sweep will never see live. Pinned against a real server because what is
// measured is tmux's own per-pane option resolution, not a fake's.
func TestHookKeyCrossSite_ResolveAgreesWithEnumeration(t *testing.T) {
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

	for _, location := range []string{sessionName + ":0.0", sessionName + ":0.1", sessionName + ":1.0"} {
		t.Run(location, func(t *testing.T) {
			resolved, err := client.ResolveHookKey(location)
			if err != nil {
				t.Fatalf("ResolveHookKey(%q): %v", location, err)
			}
			if enumerated := findPaneHookRow(t, rows, location).Token; resolved != enumerated {
				t.Errorf("ResolveHookKey(%q) = %q, enumeration row token = %q", location, resolved, enumerated)
			}
		})
	}
}
