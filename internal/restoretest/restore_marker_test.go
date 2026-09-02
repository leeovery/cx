package restoretest

import (
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/commandertest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// serverOptionRecorder stands in for the server's option table over the shared
// commander fake: it answers show-option from what set-option wrote, so the
// bracket's two halves are observed through the same read production takes.
type serverOptionRecorder struct {
	*commandertest.Scripted
	options map[string]string
}

func newServerOptionRecorder() *serverOptionRecorder {
	r := &serverOptionRecorder{options: map[string]string{}}
	r.Scripted = commandertest.Quiet(
		commandertest.Answering(optionArgv(4, "set-option", "-s"), r.setOption),
		commandertest.Answering(optionArgv(3, "set-option", "-su"), r.unsetOption),
		commandertest.Answering(optionArgv(3, "show-option", "-sv"), r.showOption),
	)
	return r
}

// optionArgv matches an option argv of exactly arity arguments beginning with
// prefix, so the neighbouring forms (-s, -su, -sv) cannot answer for each other.
func optionArgv(arity int, prefix ...string) commandertest.Matcher {
	byPrefix := commandertest.ArgvPrefix(prefix...)
	return func(args []string) bool {
		return len(args) == arity && byPrefix(args)
	}
}

func (r *serverOptionRecorder) setOption(args ...string) (string, error) {
	r.options[args[2]] = args[3]
	return "", nil
}

func (r *serverOptionRecorder) unsetOption(args ...string) (string, error) {
	delete(r.options, args[2])
	return "", nil
}

func (r *serverOptionRecorder) showOption(args ...string) (string, error) {
	return r.options[args[2]], nil
}

// index reports where a call appears in the recording, or -1.
func (r *serverOptionRecorder) index(call string) int {
	return slices.Index(r.renderedCalls(), call)
}

// renderedCalls is the recorded argv, one space-joined line per call, which is
// the form the bracket's assertions name.
func (r *serverOptionRecorder) renderedCalls() []string {
	rendered := make([]string, 0, len(r.Calls()))
	for _, args := range r.Calls() {
		rendered = append(rendered, strings.Join(args, " "))
	}
	return rendered
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
			t.Fatalf("%s was never set; calls=%v", state.RestoringMarkerName, rec.renderedCalls())
		}
		if set > unset {
			t.Errorf("%s was set after it was unset; calls=%v", state.RestoringMarkerName, rec.renderedCalls())
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
			t.Fatalf("%s was never unset; calls=%v", state.RestoringMarkerName, rec.renderedCalls())
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
