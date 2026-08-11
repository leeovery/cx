package tmuxtest

import (
	"os/exec"
	"testing"
)

// SkipIfNoTmux skips the test when tmux is not on PATH, so an environment
// without tmux skips cleanly rather than failing.
func SkipIfNoTmux(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not available; skipping integration test")
	}
}
