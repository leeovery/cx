package restoretest

import (
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// serverOptionRecorder is a tmux.Commander standing in for the server's option
// table: it answers show-option from what set-option wrote, so the bracket's two
// halves are observed through the same read production takes.
type serverOptionRecorder struct {
	options map[string]string
	calls   []string
}

func newServerOptionRecorder() *serverOptionRecorder {
	return &serverOptionRecorder{options: map[string]string{}}
}

func (r *serverOptionRecorder) Run(args ...string) (string, error) {
	r.calls = append(r.calls, strings.Join(args, " "))
	switch {
	case len(args) == 4 && args[0] == "set-option" && args[1] == "-s":
		r.options[args[2]] = args[3]
	case len(args) == 3 && args[0] == "set-option" && args[1] == "-su":
		delete(r.options, args[2])
	case len(args) == 3 && args[0] == "show-option" && args[1] == "-sv":
		return r.options[args[2]], nil
	}
	return "", nil
}

func (r *serverOptionRecorder) RunRaw(args ...string) (string, error) { return r.Run(args...) }

// index reports where a call appears in the recording, or -1.
func (r *serverOptionRecorder) index(call string) int {
	return slices.Index(r.calls, call)
}

// The whole point of the fixture is the stand-down window it opens around a
// restore, and callers observe only its closing half. Both halves are pinned
// here, so removing either one fails rather than leaving every restore running
// with the daemon's capture loop still live.
func TestRestoreWithMarker_BracketsTheRestore(t *testing.T) {
	const (
		setCall   = "set-option -s " + state.RestoringMarkerName + " 1"
		unsetCall = "set-option -su " + state.RestoringMarkerName
	)

	t.Run("it sets @portal-restoring for the duration of a restore", func(t *testing.T) {
		rec := newServerOptionRecorder()
		client := tmux.NewClient(rec)

		// An empty state dir holds no sessions.json, so the restore itself is a
		// no-op and the recording is the bracket alone.
		o := NewFakeExeOrchestrator(t, client, t.TempDir(), nil)
		if err := RestoreWithMarker(t, client, o); err != nil {
			t.Fatalf("RestoreWithMarker: %v", err)
		}

		set, unset := rec.index(setCall), rec.index(unsetCall)
		if set < 0 {
			t.Fatalf("%s was never set; calls=%v", state.RestoringMarkerName, rec.calls)
		}
		if set > unset {
			t.Errorf("%s was set after it was unset; calls=%v", state.RestoringMarkerName, rec.calls)
		}
	})

	t.Run("it clears @portal-restoring when the restore returns", func(t *testing.T) {
		rec := newServerOptionRecorder()
		client := tmux.NewClient(rec)

		o := NewFakeExeOrchestrator(t, client, t.TempDir(), nil)
		if err := RestoreWithMarker(t, client, o); err != nil {
			t.Fatalf("RestoreWithMarker: %v", err)
		}

		if rec.index(unsetCall) < 0 {
			t.Fatalf("%s was never unset; calls=%v", state.RestoringMarkerName, rec.calls)
		}
		if val, held := rec.options[state.RestoringMarkerName]; held {
			t.Errorf("%s = %q after the restore returned; want it gone", state.RestoringMarkerName, val)
		}
	})

	// The guard on the guard: assertRestoringSet is what makes a deleted
	// SetServerOption a failure rather than a silent behaviour change, so its own
	// failure path is exercised rather than assumed.
	t.Run("it fails a restore driven with the marker unset", func(t *testing.T) {
		rec := newServerOptionRecorder()
		fake := &fakeFataller{}

		assertRestoringSet(fake, tmux.NewClient(rec))

		if !fake.fatalCalled {
			t.Fatalf("assertRestoringSet passed with %s unset; deleting the set half would leave every lane green",
				state.RestoringMarkerName)
		}
	})
}
