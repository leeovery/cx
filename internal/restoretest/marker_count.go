package restoretest

import (
	"os"
	"strings"
	"time"

	"github.com/leeovery/portal/internal/harnesstest"
)

// The budgets are declared together because what separates them is how much of
// the chain is still ahead of the wait when it starts, and that is only legible
// side by side.
const (
	// HydrateBudget bounds a wait for a restored pane's helper to clear its
	// skeleton marker, and for the hook it then fires to land. It begins before
	// the helper does: on a server that has just been rebooted and restored, the
	// pane has still to be spawned, respawned into the hydrate exe and
	// scheduled, so the wait spans a whole cold chain.
	HydrateBudget = 10 * time.Second
	HydrateTick   = 50 * time.Millisecond

	// PaneReactionBudget bounds a wait for a live pane to act on input already
	// delivered to it — a hydrate signal written before the call that wrote it
	// returned, keys already sent, or a helper that has already cleared its
	// skeleton marker. Nothing but the pane's own shell is left to run, so this
	// is a shorter wait on a shorter chain rather than a tighter guess at
	// HydrateBudget's.
	PaneReactionBudget = 2 * time.Second
	PaneReactionTick   = 50 * time.Millisecond
)

// AssertMarkerCount polls a hook's side-effect file until it holds exactly want
// occurrences of marker. A count short of want fails at the hydrate budget; a
// count past it fails at once, since no further waiting can bring it back. The
// poll is what makes the assertion honest: a pane's helper clears its skeleton
// marker before it execs the hook, so a single read taken when the markers
// clear races the shell that runs it.
//
// A want of 0 waits out the whole budget before passing — 0 is also what a hook
// that has simply not fired yet reads as, so returning early on 0 would assert
// nothing.
func AssertMarkerCount(t harnesstest.TestingT, path, marker string, want int) {
	t.Helper()
	markerAssertion{
		path:   path,
		marker: marker,
		want:   want,
		budget: HydrateBudget,
		tick:   HydrateTick,
	}.run(t)
}

type markerAssertion struct {
	path   string
	marker string
	want   int
	budget time.Duration
	tick   time.Duration
}

func (a markerAssertion) run(t harnesstest.TestingT) {
	t.Helper()

	got, contents := a.wait(t)
	switch {
	case got == a.want:
		return
	case got == 0:
		t.Errorf("marker %q never appeared in %s within %v; want %d (a bare-shell miss leaves the file absent)",
			a.marker, a.path, a.budget, a.want)
	case a.want == 0:
		t.Errorf("CROSS-FIRE: marker %q leaked into %s %d times (hooks did not route per-pane)\ncontents:\n%s",
			a.marker, a.path, got, contents)
	default:
		t.Errorf("marker %q fired %d times cumulatively in %s; want exactly %d\ncontents:\n%s",
			a.marker, got, a.path, a.want, contents)
	}
}

// wait returns the last count it read, along with the file contents behind it.
func (a markerAssertion) wait(t harnesstest.TestingT) (int, string) {
	t.Helper()

	deadline := time.Now().Add(a.budget)
	for {
		got, contents := a.read(t)
		// The files are append-only, so a count at or past want can no longer
		// come back down and there is nothing left to wait for — except at a
		// want of 0, which every unfired hook also satisfies.
		if got > a.want || (got == a.want && a.want > 0) || !time.Now().Before(deadline) {
			return got, contents
		}
		time.Sleep(a.tick)
	}
}

// read treats an absent file as a count of 0: until the hook fires, nothing has
// created it.
func (a markerAssertion) read(t harnesstest.TestingT) (int, string) {
	t.Helper()

	data, err := os.ReadFile(a.path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ""
		}
		t.Fatalf("read marker file %s: %v", a.path, err)
		return 0, ""
	}
	return strings.Count(string(data), a.marker), string(data)
}
