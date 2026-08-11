package tmuxtest

import (
	"fmt"
	"testing"
)

// ApplyBaseIndices sets base-index and pane-base-index on ts at both server and
// global scope: -g controls what new sessions inherit, -s what show-option
// reports, and both affect the coordinates tmux assigns to fresh panes.
func ApplyBaseIndices(t *testing.T, ts *Socket, base, paneBase int) {
	t.Helper()
	ts.Run(t, "set-option", "-g", "base-index", fmt.Sprintf("%d", base))
	ts.Run(t, "set-option", "-g", "pane-base-index", fmt.Sprintf("%d", paneBase))
	ts.Run(t, "set-option", "-s", "base-index", fmt.Sprintf("%d", base))
	ts.Run(t, "set-option", "-s", "pane-base-index", fmt.Sprintf("%d", paneBase))
}
