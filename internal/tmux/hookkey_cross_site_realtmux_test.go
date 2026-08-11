package tmux_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestCrossSiteConsistency_StampedSession(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-xsite-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const sessionName = "xsite-stamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	if err := client.SetSessionOption(sessionName, portalIDLiteral, "tok123"); err != nil {
		t.Fatalf("SetSessionOption(%q, %q, %q): %v", sessionName, portalIDLiteral, "tok123", err)
	}

	reg, err := client.ResolveHookKey(sessionName)
	if err != nil {
		t.Fatalf("ResolveHookKey(%q): %v", sessionName, err)
	}
	if reg != "tok123:0.0" {
		t.Fatalf("registration key = %q, want %q (conditional must take the @portal-id branch)", reg, "tok123:0.0")
	}

	live, err := client.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}

	if !slices.Contains(live, reg) {
		t.Errorf("registration key %q not found byte-identically in cleanup enumeration %v (the two live sites disagree)", reg, live)
	}
}

func TestCrossSiteConsistency_MultiPaneStampedSession(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-xsite-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	paneIDs := seedThreePaneStampedSession(t, ts, client, "xsite-multi", "tok123")
	if len(paneIDs) != 3 {
		t.Fatalf("expected 3 panes (w0.p0, w0.p1, w1.p0), got %d: %v", len(paneIDs), paneIDs)
	}

	live, err := client.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}

	var regKeys []string
	for _, paneID := range paneIDs {
		reg, err := client.ResolveHookKey(paneID)
		if err != nil {
			t.Fatalf("ResolveHookKey(%q): %v", paneID, err)
		}
		regKeys = append(regKeys, reg)

		if !strings.HasPrefix(reg, "tok123:") {
			t.Errorf("per-pane registration key %q for pane %q does not share the tok123 prefix", reg, paneID)
		}
		if !slices.Contains(live, reg) {
			t.Errorf("per-pane registration key %q not found byte-identically in cleanup enumeration %v (the two live sites disagree)", reg, live)
		}
	}

	if distinct := uniqueCount(regKeys); distinct != 3 {
		t.Errorf("expected 3 distinct per-pane registration keys, got %d from %v", distinct, regKeys)
	}
}

func TestCrossSiteConsistency_UnstampedSession(t *testing.T) {
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, "ptl-xsite-")
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}

	const sessionName = "xsite-unstamped"
	if err := client.NewSession(sessionName, t.TempDir(), ""); err != nil {
		t.Fatalf("NewSession(%q): %v", sessionName, err)
	}
	ts.WaitForSession(t, sessionName, 2*time.Second)

	want := sessionName + ":0.0"

	reg, err := client.ResolveHookKey(sessionName)
	if err != nil {
		t.Fatalf("ResolveHookKey(%q): %v", sessionName, err)
	}
	if reg != want {
		t.Fatalf("un-stamped registration key = %q, want %q (unset @portal-id must take the #{session_name} branch)", reg, want)
	}

	live, err := client.ListAllPaneHookKeys()
	if err != nil {
		t.Fatalf("ListAllPaneHookKeys: %v", err)
	}
	if !slices.Contains(live, reg) {
		t.Errorf("un-stamped registration key %q not found byte-identically in cleanup enumeration %v (the two live sites disagree)", reg, live)
	}
}

func uniqueCount(s []string) int {
	seen := make(map[string]struct{}, len(s))
	for _, v := range s {
		seen[v] = struct{}{}
	}
	return len(seen)
}
