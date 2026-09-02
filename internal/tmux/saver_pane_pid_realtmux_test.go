package tmux_test

import (
	"testing"

	"github.com/leeovery/portal/internal/tmux"
)

// saverPrefixSibling is the shape a renamed-away or version-upgraded saver
// leaves behind: a live session whose name extends PortalSaverName, with
// PortalSaverName itself absent.
const saverPrefixSibling = tmux.PortalSaverName + "-old"

// saverFixtureSocketPrefix names the temp dir of this suite's socket, so a
// stray server left behind by it is recognisable.
const saverFixtureSocketPrefix = "ptl-saverpid-"

// saverFixture describes an isolated server holding the named sessions and
// nothing else.
func saverFixture(sessions ...string) realTmuxFixture {
	return realTmuxFixture{socketPrefix: saverFixtureSocketPrefix, sessions: sessions}
}

func TestSaverPanePID_RealTmux(t *testing.T) {
	t.Run("it does not resolve a prefix-sibling saver session", func(t *testing.T) {
		ts, client, _ := seedRealTmuxServer(t, saverFixture(saverPrefixSibling))
		siblingPID := livePanePID(t, ts, saverPrefixSibling)

		pid, err := tmux.SaverPanePID(client, tmux.PortalSaverName)
		if pid == siblingPID {
			t.Fatalf("SaverPanePID(%q) returned %d — the live %q session's pane pid",
				tmux.PortalSaverName, pid, saverPrefixSibling)
		}
		if err == nil {
			t.Fatalf("SaverPanePID(%q) = %d, nil; want a tmux failure", tmux.PortalSaverName, pid)
		}
	})

	t.Run("it reports absence when only a prefix sibling is live", func(t *testing.T) {
		ts, client, _ := seedRealTmuxServer(t, saverFixture(saverPrefixSibling))
		siblingPID := livePanePID(t, ts, saverPrefixSibling)

		pid, present, err := tmux.SaverPanePIDOrAbsent(client, tmux.PortalSaverName)
		if pid == siblingPID {
			t.Fatalf("SaverPanePIDOrAbsent(%q) returned %d — the live %q session's pane pid",
				tmux.PortalSaverName, pid, saverPrefixSibling)
		}
		if pid != 0 || present || err != nil {
			t.Errorf("SaverPanePIDOrAbsent(%q) = %d, %t, %v; want 0, false, nil",
				tmux.PortalSaverName, pid, present, err)
		}
	})

	t.Run("it returns the pane pid of a live _portal-saver", func(t *testing.T) {
		ts, client, _ := seedRealTmuxServer(t, saverFixture(saverPrefixSibling, tmux.PortalSaverName))
		want := livePanePID(t, ts, tmux.PortalSaverName)

		pid, present, err := tmux.SaverPanePIDOrAbsent(client, tmux.PortalSaverName)
		if err != nil {
			t.Fatalf("SaverPanePIDOrAbsent(%q): %v", tmux.PortalSaverName, err)
		}
		if pid != want || !present {
			t.Errorf("SaverPanePIDOrAbsent(%q) = %d, %t; want %d, true",
				tmux.PortalSaverName, pid, present, want)
		}
	})

	t.Run("it collapses a missing session to present=false", func(t *testing.T) {
		_, client, _ := seedRealTmuxServer(t, saverFixture("unrelated"))

		pid, present, err := tmux.SaverPanePIDOrAbsent(client, tmux.PortalSaverName)
		if pid != 0 || present || err != nil {
			t.Errorf("SaverPanePIDOrAbsent(%q) = %d, %t, %v; want 0, false, nil",
				tmux.PortalSaverName, pid, present, err)
		}
	})
}
