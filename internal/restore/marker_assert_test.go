//go:build integration

package restore_test

import (
	"os"
	"strings"
	"testing"
	"time"
)

// markerReporter is the slice of *testing.T the marker assertion needs, so a
// test can drive it without failing itself.
type markerReporter interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// assertMarkerCount polls a hook's side-effect file until it holds exactly want
// occurrences of marker. A count short of want fails at the hydrate budget; a
// count past it fails at once, since no further waiting can bring it back. The
// poll is what makes the assertion honest: a hook fires after its helper has
// cleared its skeleton marker, so a single read taken when the reboot returns
// races the shell that runs it.
//
// A want of 0 waits out the whole budget before passing — 0 is also what a hook
// that has simply not fired yet reads as, so returning early on 0 would assert
// nothing.
func assertMarkerCount(t *testing.T, path, marker string, want int) {
	t.Helper()
	assertMarkerCountOn(t, path, marker, want)
}

func assertMarkerCountOn(t markerReporter, path, marker string, want int) {
	t.Helper()

	got, contents := waitForMarkerCount(t, path, marker, want)
	switch {
	case got == want:
		return
	case got == 0:
		t.Errorf("marker %q never appeared in %s within %v; want %d (a bare-shell miss leaves the file absent)",
			marker, path, hydrateBudget, want)
	case want == 0:
		t.Errorf("CROSS-FIRE: marker %q leaked into %s %d times (hooks did not route per-pane)\ncontents:\n%s",
			marker, path, got, contents)
	default:
		t.Errorf("marker %q fired %d times cumulatively in %s; want exactly %d\ncontents:\n%s",
			marker, got, path, want, contents)
	}
}

// waitForMarkerCount returns the last count it read, along with the file
// contents behind it.
func waitForMarkerCount(t markerReporter, path, marker string, want int) (int, string) {
	t.Helper()

	deadline := time.Now().Add(hydrateBudget)
	for {
		got, contents := readMarkerCount(t, path, marker)
		// The files are append-only, so a count at or past want can no longer
		// come back down and there is nothing left to wait for — except at a
		// want of 0, which every unfired hook also satisfies.
		if got > want || (got == want && want > 0) || !time.Now().Before(deadline) {
			return got, contents
		}
		time.Sleep(hydrateTick)
	}
}

// readMarkerCount treats an absent file as a count of 0: until the hook fires,
// nothing has created it.
func readMarkerCount(t markerReporter, path, marker string) (int, string) {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, ""
		}
		t.Fatalf("read marker file %s: %v", path, err)
		return 0, ""
	}
	return strings.Count(string(data), marker), string(data)
}
