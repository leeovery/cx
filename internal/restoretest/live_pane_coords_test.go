//go:build integration

package restoretest_test

import (
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/tmuxtest"
)

func TestLivePaneCoords(t *testing.T) {
	t.Run("it reads the coords of a session's live panes", func(t *testing.T) {
		tmuxtest.SkipIfNoTmux(t)
		ts := tmuxtest.New(t, "ptl-panecoords-")
		ts.Run(t, "new-session", "-d", "-s", "alpha", "sleep", "infinity")
		ts.WaitForSession(t, "alpha", 2*time.Second)
		ts.Run(t, "split-window", "-t", "alpha:0.0", "sleep", "infinity")
		ts.Run(t, "new-window", "-t", "alpha:", "sleep", "infinity")

		got := strings.Join(restoretest.LivePaneCoords(t, ts, "alpha"), " ")

		if got != "0:0 0:1 1:0" {
			t.Fatalf("LivePaneCoords(alpha) = %q, want %q", got, "0:0 0:1 1:0")
		}
	})

	t.Run("it does not read a prefix sibling's panes", func(t *testing.T) {
		tmuxtest.SkipIfNoTmux(t)
		ts := tmuxtest.New(t, "ptl-panecoords-")
		ts.Run(t, "new-session", "-d", "-s", "sib-2", "sleep", "infinity")
		ts.WaitForSession(t, "sib-2", 2*time.Second)

		coords, err := restoretest.TryLivePaneCoords(ts, "sib")

		if err == nil {
			t.Fatalf("TryLivePaneCoords(sib) = %v, nil; want an error — only the prefix sibling sib-2 is live", coords)
		}
		if coords != nil {
			t.Fatalf("TryLivePaneCoords(sib) returned coords %v alongside its error; want none", coords)
		}
	})
}
