//go:build integration

package restoretest

import (
	"fmt"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const livePaneCoordFormat = "#{window_index}:#{pane_index}"

// LivePaneCoords returns "<window>:<pane>" for every live pane in the session,
// across all its windows, in tmux's own order. The read fatals the test.
func LivePaneCoords(t *testing.T, ts *tmuxtest.Socket, session string) []string {
	t.Helper()
	coords, err := TryLivePaneCoords(ts, session)
	if err != nil {
		t.Fatalf("%v", err)
	}
	return coords
}

// TryLivePaneCoords is LivePaneCoords for a caller that must handle the read
// failing — a session that may already have gone away — rather than fail on it.
//
// The target is pinned exactly: tmux's fuzzy form lets a live prefix sibling
// answer for a session that is gone, so an assertion about where a restore put
// its panes could be satisfied by a stranger's session.
func TryLivePaneCoords(ts *tmuxtest.Socket, session string) ([]string, error) {
	out, err := ts.TryRun("list-panes", "-s", "-t", tmux.CoordTargetExact(session),
		"-F", livePaneCoordFormat)
	if err != nil {
		return nil, fmt.Errorf("list panes in session %q: %w: %s", session, err, strings.TrimSpace(out))
	}
	return strings.Fields(out), nil
}
