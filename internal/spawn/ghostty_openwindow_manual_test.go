//go:build manual

package spawn

import "testing"

// Manual live-terminal check: run it on a Mac inside Ghostty and visually
// confirm a new window opened running the marker command. The assertion only
// checks that OpenWindow reported success.
func TestManual_OpenWindow_OpensRealGhosttyWindow(t *testing.T) {
	// A visible marker command rather than a real `portal open`, so the manual
	// gate needs no live session.
	argv := []string{
		"/usr/bin/env", "-u", "TMUX", "-u", "TMUX_PANE",
		"echo", "portal", "spawn", "manual", "verification",
	}

	result := newGhosttyAdapter().OpenWindow(argv)

	if !result.OK() {
		t.Fatalf("OpenWindow did not succeed: outcome=%v detail=%q", result.Outcome, result.Detail)
	}
	t.Logf("OpenWindow reported success (detail=%q) — visually confirm a new Ghostty window opened", result.Detail)
}
