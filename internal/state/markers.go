package state

import (
	"strings"

	"github.com/leeovery/portal/internal/tmuxout"
)

// SkeletonMarkerPrefix names the tmux server option marking a pane as
// skeleton-restored and awaiting hydration. While set, the save loop skips the
// pane so its pre-boot scrollback file stays authoritative.
const SkeletonMarkerPrefix = "@portal-skeleton-"

// RestoringMarkerName is the tmux server option bootstrap sets while it builds
// the restore skeleton. While set, the daemon skips captures entirely so
// half-built session structure is never recorded.
const RestoringMarkerName = "@portal-restoring"

// BootstrappedMarkerName is the tmux server option holding the version-stamped
// bootstrap latch. Its value is load-bearing rather than its presence:
// satisfaction is equality against the running binary version, so an upgraded
// binary re-bootstraps on its first command.
const BootstrappedMarkerName = "@portal-bootstrapped"

// ServerOptionLister is the seam used by ListSkeletonMarkers, satisfied by
// *tmux.Client. It is declared here so internal/state need not import
// internal/tmux, which imports internal/state and would close a cycle.
type ServerOptionLister interface {
	ShowAllServerOptions() (string, error)
}

// RestoringChecker is the seam used by IsRestoringSet, satisfied by
// *tmux.Client.
type RestoringChecker interface {
	TryGetServerOption(name string) (string, bool, error)
}

// ServerOptionWriter is the marker-writing seam, satisfied by *tmux.Client.
type ServerOptionWriter interface {
	SetServerOption(name, value string) error
	UnsetServerOption(name string) error
}

// ListSkeletonMarkers returns the set of paneKeys whose skeleton markers are
// set as tmux server options. A read failure returns (nil, err), never a
// partial set; a marker with no value or an empty value counts as absent.
func ListSkeletonMarkers(c ServerOptionLister) (map[string]struct{}, error) {
	out, err := c.ShowAllServerOptions()
	if err != nil {
		return nil, err
	}
	set := map[string]struct{}{}
	if out == "" {
		return set, nil
	}
	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		idx := strings.IndexAny(line, " \t")
		if idx < 0 {
			continue
		}
		name := line[:idx]
		if !strings.HasPrefix(name, SkeletonMarkerPrefix) {
			continue
		}
		value := tmuxout.StripMatchedOuterQuotes(strings.TrimSpace(line[idx+1:]))
		if value == "" {
			continue
		}
		paneKey := strings.TrimPrefix(name, SkeletonMarkerPrefix)
		set[paneKey] = struct{}{}
	}
	return set, nil
}

// SetSkeletonMarker marks paneKey as skeleton-restored and awaiting hydration.
func SetSkeletonMarker(w ServerOptionWriter, paneKey string) error {
	return w.SetServerOption(SkeletonMarkerPrefix+paneKey, "1")
}

// UnsetSkeletonMarker clears paneKey's marker, so the save loop resumes
// capturing that pane's scrollback.
func UnsetSkeletonMarker(w ServerOptionWriter, paneKey string) error {
	return w.UnsetServerOption(SkeletonMarkerPrefix + paneKey)
}

// UnsetSkeletonMarkerForFIFO clears the skeleton marker for the pane whose
// hydration FIFO is fifoPath, recovering the paneKey from the basename.
func UnsetSkeletonMarkerForFIFO(w ServerOptionWriter, fifoPath string) error {
	return UnsetSkeletonMarker(w, PaneKeyFromFIFOPath(fifoPath))
}

// IsRestoringSet reports whether the @portal-restoring marker is set to a
// non-empty value; absent and empty both report false. A tmux error propagates
// so a real failure cannot masquerade as "not restoring".
func IsRestoringSet(c RestoringChecker) (bool, error) {
	val, found, err := c.TryGetServerOption(RestoringMarkerName)
	if err != nil {
		return false, err
	}
	if !found {
		return false, nil
	}
	return val != "", nil
}

// BootstrappedLatchSatisfied reports whether the @portal-bootstrapped latch is
// present and its value exactly equals runningVersion. Absence, mismatch and
// read failure all report false, so a cold, upgraded or unreachable server
// takes the full-bootstrap path.
//
// Swallowing the read error into a bare bool is deliberate — do not "fix" this
// into a (bool, error) signature. runningVersion is a parameter rather than a
// read of cmd.version so internal/state stays a leaf.
func BootstrappedLatchSatisfied(c RestoringChecker, runningVersion string) bool {
	val, found, err := c.TryGetServerOption(BootstrappedMarkerName)
	if err != nil {
		return false
	}
	if !found {
		return false
	}
	return val == runningVersion
}
