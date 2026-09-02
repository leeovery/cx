package bootstrap

import (
	"errors"
	"log/slog"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
)

// massUnsetDeferralWarnMessage is the message of the WARN emitted when the
// guard defers a sweep that would otherwise unset every marker.
const massUnsetDeferralWarnMessage = "stale-marker cleanup: zero live panes parsed with markers present; skipping to avoid mass-unset hazard (next bootstrap retries)"

type fakeMarkerLister struct {
	markers map[string]struct{}
	err     error
	calls   int
}

func (f *fakeMarkerLister) ShowAllServerOptions() (string, error) {
	f.calls++
	if f.err != nil {
		return "", f.err
	}
	if len(f.markers) == 0 {
		return "", nil
	}
	var b strings.Builder
	for k := range f.markers {
		b.WriteString(state.SkeletonMarkerPrefix)
		b.WriteString(k)
		b.WriteString(" \"1\"\n")
	}
	return b.String(), nil
}

type fakeLivePaneLister struct {
	output      string
	err         error
	gotFormat   string
	formatCalls int
}

func (f *fakeLivePaneLister) ListAllPanesWithFormat(format string) (string, error) {
	f.formatCalls++
	f.gotFormat = format
	if f.err != nil {
		return "", f.err
	}
	return f.output, nil
}

type fakeMarkerUnsetter struct {
	calls []string
	err   error
	errs  map[int]error
	errOn map[string]error
}

func (f *fakeMarkerUnsetter) UnsetServerOption(name string) error {
	f.calls = append(f.calls, name)
	if f.err != nil {
		return f.err
	}
	if e, ok := f.errs[len(f.calls)]; ok {
		return e
	}
	if e, ok := f.errOn[name]; ok {
		return e
	}
	return nil
}

func TestCleanStaleMarkers_unsetsMarkerWhosePaneKeyIsNotInLiveSet(t *testing.T) {
	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"stale__0.0": {},
		"live__0.0":  {},
	}}
	live := &fakeLivePaneLister{output: "live:0.0\n"}
	unsetter := &fakeMarkerUnsetter{}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
	}
	if err := c.CleanStaleMarkers(); err != nil {
		t.Fatalf("CleanStaleMarkers returned error: %v", err)
	}

	if len(unsetter.calls) != 1 {
		t.Fatalf("expected exactly 1 unset call, got %d (%v)", len(unsetter.calls), unsetter.calls)
	}
	want := state.SkeletonMarkerPrefix + "stale__0.0"
	if unsetter.calls[0] != want {
		t.Errorf("unset name = %q, want %q", unsetter.calls[0], want)
	}
}

func TestCleanStaleMarkers_leavesLiveMarkerAlone(t *testing.T) {
	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"live__0.0": {},
	}}
	live := &fakeLivePaneLister{output: "live:0.0\n"}
	unsetter := &fakeMarkerUnsetter{}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
	}
	if err := c.CleanStaleMarkers(); err != nil {
		t.Fatalf("CleanStaleMarkers returned error: %v", err)
	}

	if len(unsetter.calls) != 0 {
		t.Errorf("expected zero unset calls, got %d (%v)", len(unsetter.calls), unsetter.calls)
	}
}

func TestCleanStaleMarkers_requestsLivePanesWithCanonicalFormat(t *testing.T) {
	lister := &fakeMarkerLister{markers: map[string]struct{}{}}
	live := &fakeLivePaneLister{output: "live:0.0\n"}
	unsetter := &fakeMarkerUnsetter{}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
	}
	if err := c.CleanStaleMarkers(); err != nil {
		t.Fatalf("CleanStaleMarkers returned error: %v", err)
	}

	if live.gotFormat != tmux.StructuralKeyFormat {
		t.Errorf("ListAllPanesWithFormat format = %q, want %q", live.gotFormat, tmux.StructuralKeyFormat)
	}
}

func TestCleanStaleMarkers_composesOptionNameFromSkeletonMarkerPrefix(t *testing.T) {
	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"bar__1.2": {},
	}}
	live := &fakeLivePaneLister{output: "foo:0.0\n"}
	unsetter := &fakeMarkerUnsetter{}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
	}
	if err := c.CleanStaleMarkers(); err != nil {
		t.Fatalf("CleanStaleMarkers returned error: %v", err)
	}

	if len(unsetter.calls) != 1 {
		t.Fatalf("expected 1 unset call, got %v", unsetter.calls)
	}
	want := state.SkeletonMarkerPrefix + "bar__1.2"
	if unsetter.calls[0] != want {
		t.Errorf("unset name = %q, want %q (must be SkeletonMarkerPrefix + paneKey)", unsetter.calls[0], want)
	}
}

func TestCleanStaleMarkers_emptyMarkerSet(t *testing.T) {
	lister := &fakeMarkerLister{markers: map[string]struct{}{}}
	live := &fakeLivePaneLister{output: "foo:0.0\nbar:1.2\n"}
	unsetter := &fakeMarkerUnsetter{}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
	}
	if err := c.CleanStaleMarkers(); err != nil {
		t.Fatalf("CleanStaleMarkers returned error: %v", err)
	}

	if len(unsetter.calls) != 0 {
		t.Errorf("expected zero unset calls for empty marker set, got %v", unsetter.calls)
	}
}

func TestCleanStaleMarkers_fullOverlapNoUnsetCalls(t *testing.T) {
	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"foo__0.0": {},
		"bar__1.2": {},
	}}
	live := &fakeLivePaneLister{output: "foo:0.0\nbar:1.2\n"}
	unsetter := &fakeMarkerUnsetter{}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
	}
	if err := c.CleanStaleMarkers(); err != nil {
		t.Fatalf("CleanStaleMarkers returned error: %v", err)
	}

	if len(unsetter.calls) != 0 {
		t.Errorf("expected zero unset calls when all markers are live, got %v", unsetter.calls)
	}
}

func TestStaleMarkerCleanup_PaneKeyNormalisation(t *testing.T) {
	t.Run("it recognises a marker in canonical form against a live pane in tmux session:win.pane form", func(t *testing.T) {
		canonical := state.SanitizePaneKey("my-session", 0, 1)
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			canonical: {},
		}}
		live := &fakeLivePaneLister{output: "my-session:0.1\n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers returned error: %v", err)
		}

		if len(unsetter.calls) != 0 {
			t.Errorf("expected zero unset calls (canonical marker should match live pane after sanitisation), got %v", unsetter.calls)
		}
	})

	t.Run("it does not treat raw session:win.pane and canonical session__win.pane as equivalent", func(t *testing.T) {
		raw := "my-session:0.1"
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			raw: {},
		}}
		live := &fakeLivePaneLister{output: "my-session:0.1\n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers returned error: %v", err)
		}

		if len(unsetter.calls) != 1 {
			t.Fatalf("expected exactly 1 unset call (raw marker form must NOT match canonical live set), got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
		want := state.SkeletonMarkerPrefix + raw
		if unsetter.calls[0] != want {
			t.Errorf("unset name = %q, want %q", unsetter.calls[0], want)
		}
	})

	t.Run("it recognises a canonical marker against a live pane whose session name contains a colon", func(t *testing.T) {
		canonical := state.SanitizePaneKey("host:1234", 0, 0)
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			canonical: {},
		}}
		live := &fakeLivePaneLister{output: "host:1234:0.0\n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers returned error: %v", err)
		}

		if len(unsetter.calls) != 0 {
			t.Errorf("expected zero unset calls (rightmost-colon split must recover session name with colon), got %v", unsetter.calls)
		}
	})
}

func TestCleanStaleMarkers_noOverlapUnsetsEveryMarker(t *testing.T) {
	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"stale1__0.0": {},
		"stale2__1.2": {},
	}}
	live := &fakeLivePaneLister{output: "alive:9.9\n"}
	unsetter := &fakeMarkerUnsetter{}

	c := &MarkerCleanupCore{
		Markers:  lister,
		Panes:    live,
		Unsetter: unsetter,
	}
	if err := c.CleanStaleMarkers(); err != nil {
		t.Fatalf("CleanStaleMarkers returned error: %v", err)
	}

	if len(unsetter.calls) != 2 {
		t.Fatalf("expected 2 unset calls, got %d (%v)", len(unsetter.calls), unsetter.calls)
	}
	gotSet := map[string]struct{}{
		unsetter.calls[0]: {},
		unsetter.calls[1]: {},
	}
	wantSet := map[string]struct{}{
		state.SkeletonMarkerPrefix + "stale1__0.0": {},
		state.SkeletonMarkerPrefix + "stale2__1.2": {},
	}
	for k := range wantSet {
		if _, ok := gotSet[k]; !ok {
			t.Errorf("missing expected unset for %q; got %v", k, unsetter.calls)
		}
	}
}

func TestStaleMarkerCleanup_MassUnsetHazardGuard(t *testing.T) {
	t.Run("it skips unset and emits a warning when ListAllPanesWithFormat returns an error", func(t *testing.T) {
		sentinel := errors.New("tmux: connection refused")
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"protected__0.0": {},
		}}
		live := &fakeLivePaneLister{err: sentinel}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		err := c.CleanStaleMarkers()
		if err == nil {
			t.Fatalf("expected non-nil error from CleanStaleMarkers; got nil")
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("expected returned error to wrap sentinel %v, got %v", sentinel, err)
		}
		if len(unsetter.calls) != 0 {
			t.Errorf("expected zero unset calls when ListAllPanesWithFormat fails, got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
	})

	t.Run("it returns nil and emits a Warn log entry when zero live panes are returned with markers present", func(t *testing.T) {
		sink := &logtest.Sink{}
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"protected__0.0": {},
			"another__1.2":   {},
		}}
		live := &fakeLivePaneLister{output: ""}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
			Logger:   slog.New(sink).With("component", "bootstrap"),
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers must return nil for zero-panes-with-markers deferral; got %v", err)
		}
		if len(unsetter.calls) != 0 {
			t.Errorf("expected zero unset calls under zero-panes guard, got %d (%v)", len(unsetter.calls), unsetter.calls)
		}

		deferral := sink.RecordsAtExactLevelWithMessage(slog.LevelWarn, massUnsetDeferralWarnMessage).
			Only(t, "stale-marker cleanup mass-unset-hazard deferral WARN")
		if comp := deferral.AttrString(t, "component"); comp != "bootstrap" {
			t.Errorf("deferral Warn component = %q, want %q", comp, "bootstrap")
		}
	})

	t.Run("it is a clean no-op when zero live panes are returned with zero markers", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{}}
		live := &fakeLivePaneLister{output: ""}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("expected nil error for zero markers + zero live panes; got %v", err)
		}
		if len(unsetter.calls) != 0 {
			t.Errorf("expected zero unset calls for empty markers + empty live, got %v", unsetter.calls)
		}
	})

	t.Run("the zero-panes guard runs before any unset", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"a__0.0": {},
			"b__0.1": {},
			"c__1.0": {},
		}}
		live := &fakeLivePaneLister{output: "   \n  \n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers must return nil for whitespace-only zero-panes deferral; got %v", err)
		}
		if len(unsetter.calls) != 0 {
			t.Errorf("expected zero unset calls (guard must run before any unset), got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
	})

	t.Run("it never mass-unsets when ListAllPanesWithFormat fails", func(t *testing.T) {
		sentinel := errors.New("tmux gone")
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"m1__0.0": {},
			"m2__0.1": {},
			"m3__1.0": {},
			"m4__1.1": {},
			"m5__2.0": {},
		}}
		live := &fakeLivePaneLister{err: sentinel}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		err := c.CleanStaleMarkers()
		if err == nil {
			t.Fatalf("expected non-nil error from ListAllPanesWithFormat failure; got nil")
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("expected returned error to wrap sentinel %v, got %v", sentinel, err)
		}
		if len(unsetter.calls) != 0 {
			t.Errorf("expected zero unset calls when ListAllPanesWithFormat fails (mass-unset hazard), got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
	})
}

func TestStaleMarkerCleanup_SoftWarningPosture(t *testing.T) {
	t.Run("it continues attempting unsets when one fails mid-loop", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"a__0.0": {},
			"b__0.0": {},
			"c__0.0": {},
		}}
		live := &fakeLivePaneLister{output: "alive:9.9\n"}
		sentinel := errors.New("tmux: option boom")
		unsetter := &fakeMarkerUnsetter{
			errs: map[int]error{2: sentinel},
		}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		err := c.CleanStaleMarkers()
		if err == nil {
			t.Fatalf("expected non-nil error when one unset fails mid-loop; got nil")
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("expected returned error to wrap sentinel %v, got %v", sentinel, err)
		}
		if len(unsetter.calls) != 3 {
			t.Errorf("expected all 3 unset calls attempted despite mid-loop failure, got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
	})

	t.Run("it attempts every unset when all fail", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"a__0.0": {},
			"b__0.0": {},
		}}
		live := &fakeLivePaneLister{output: "alive:9.9\n"}
		sentinel := errors.New("every unset boom")
		unsetter := &fakeMarkerUnsetter{err: sentinel}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		err := c.CleanStaleMarkers()
		if err == nil {
			t.Fatalf("expected non-nil error when every unset fails; got nil")
		}
		if !errors.Is(err, sentinel) {
			t.Errorf("expected returned error to wrap sentinel %v, got %v", sentinel, err)
		}
		if len(unsetter.calls) != 2 {
			t.Errorf("expected both unset calls attempted, got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
		if fatal, ok := errors.AsType[*FatalError](err); ok {
			t.Errorf("expected non-fatal error; got *FatalError = %v", fatal)
		}
	})

	t.Run("it skips malformed live-pane lines without aborting cleanup", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			state.SanitizePaneKey("good", 0, 0):  {},
			state.SanitizePaneKey("good2", 1, 0): {},
			"stale__9.9":                         {},
		}}
		live := &fakeLivePaneLister{output: "good:0.0\nmalformed-no-colon\ngood2:1.0\n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers returned error: %v", err)
		}
		if len(unsetter.calls) != 1 {
			t.Fatalf("expected exactly 1 unset call (stale marker), got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
		want := state.SkeletonMarkerPrefix + "stale__9.9"
		if unsetter.calls[0] != want {
			t.Errorf("unset name = %q, want %q", unsetter.calls[0], want)
		}
	})

	t.Run("it skips a line whose window index is not an integer", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"good__0.0": {},
		}}
		live := &fakeLivePaneLister{output: "good:abc.0\nalive:9.9\n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers returned error: %v", err)
		}
		if len(unsetter.calls) != 1 {
			t.Fatalf("expected 1 unset call (malformed window must NOT enter live set), got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
		want := state.SkeletonMarkerPrefix + "good__0.0"
		if unsetter.calls[0] != want {
			t.Errorf("unset name = %q, want %q", unsetter.calls[0], want)
		}
	})

	t.Run("it skips a line whose pane index is not an integer", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"good__0.0": {},
		}}
		live := &fakeLivePaneLister{output: "good:0.xyz\nalive:9.9\n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers returned error: %v", err)
		}
		if len(unsetter.calls) != 1 {
			t.Fatalf("expected 1 unset call (malformed pane must NOT enter live set), got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
		want := state.SkeletonMarkerPrefix + "good__0.0"
		if unsetter.calls[0] != want {
			t.Errorf("unset name = %q, want %q", unsetter.calls[0], want)
		}
	})

	t.Run("it skips a line missing the dot separator", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"good__0.0": {},
		}}
		live := &fakeLivePaneLister{output: "good:01\nalive:9.9\n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers returned error: %v", err)
		}
		if len(unsetter.calls) != 1 {
			t.Fatalf("expected 1 unset call (missing-dot line must NOT enter live set), got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
	})

	t.Run("it skips a line missing the colon separator", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"good__0.0": {},
		}}
		live := &fakeLivePaneLister{output: "goodonly\nalive:9.9\n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers returned error: %v", err)
		}
		if len(unsetter.calls) != 1 {
			t.Fatalf("expected 1 unset call (missing-colon line must NOT enter live set), got %d (%v)", len(unsetter.calls), unsetter.calls)
		}
	})

	t.Run("the cleanup never returns a fatal error", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"a__0.0": {},
			"b__0.0": {},
		}}
		live := &fakeLivePaneLister{output: "malformed-no-colon\nalive:9.9\n"}
		sentinel := errors.New("unset boom")
		unsetter := &fakeMarkerUnsetter{err: sentinel}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
		}
		err := c.CleanStaleMarkers()
		if err == nil {
			t.Fatalf("expected non-nil error from per-unset failures; got nil")
		}
		if fatal, ok := errors.AsType[*FatalError](err); ok {
			t.Errorf("CleanStaleMarkers returned *FatalError = %v; soft-warning posture forbids fatal escalation", fatal)
		}
	})

	t.Run("zero-panes guard fires when all lines are malformed and markers exist", func(t *testing.T) {
		sink := &logtest.Sink{}
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"a__0.0": {},
			"b__0.0": {},
			"c__0.0": {},
		}}
		live := &fakeLivePaneLister{output: "malformed1\nmalformed2:nope\nmalformed3:0.zzz\n"}
		unsetter := &fakeMarkerUnsetter{}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
			Logger:   slog.New(sink).With("component", "bootstrap"),
		}
		if err := c.CleanStaleMarkers(); err != nil {
			t.Fatalf("CleanStaleMarkers must return nil under all-malformed + markers-exist deferral; got %v", err)
		}
		if len(unsetter.calls) != 0 {
			t.Errorf("expected zero unset calls under all-malformed + zero-panes guard, got %d (%v)", len(unsetter.calls), unsetter.calls)
		}

		deferral := sink.RecordsAtExactLevelWithMessage(slog.LevelWarn, massUnsetDeferralWarnMessage).
			Only(t, "all-malformed + zero-panes mass-unset-hazard deferral WARN")
		if comp := deferral.AttrString(t, "component"); comp != "bootstrap" {
			t.Errorf("deferral Warn component = %q, want %q", comp, "bootstrap")
		}
	})

	t.Run("logger is nil-safe under per-unset failure and malformed lines", func(t *testing.T) {
		lister := &fakeMarkerLister{markers: map[string]struct{}{
			"a__0.0": {},
		}}
		live := &fakeLivePaneLister{output: "malformed\nalive:9.9\n"}
		unsetter := &fakeMarkerUnsetter{err: errors.New("boom")}

		c := &MarkerCleanupCore{
			Markers:  lister,
			Panes:    live,
			Unsetter: unsetter,
			Logger:   nil,
		}
		if err := c.CleanStaleMarkers(); err == nil {
			t.Fatalf("expected non-nil error; got nil")
		}
	})
}

func TestStaleMarkerCleanup_GenuineFailurePropagation(t *testing.T) {
	listSentinel := errors.New("list-panes: socket gone")
	markersSentinel := errors.New("show-options: tmux dead")

	cases := []struct {
		name     string
		markers  *fakeMarkerLister
		panes    *fakeLivePaneLister
		wantWrap error
	}{
		{
			name:     "ListAllPanesWithFormat error propagates with no unset calls",
			markers:  &fakeMarkerLister{markers: map[string]struct{}{"m__0.0": {}}},
			panes:    &fakeLivePaneLister{err: listSentinel},
			wantWrap: listSentinel,
		},
		{
			name:     "ListSkeletonMarkers error propagates with no unset calls",
			markers:  &fakeMarkerLister{err: markersSentinel},
			panes:    &fakeLivePaneLister{output: "live:0.0\n"},
			wantWrap: markersSentinel,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			unsetter := &fakeMarkerUnsetter{}
			c := &MarkerCleanupCore{
				Markers:  tc.markers,
				Panes:    tc.panes,
				Unsetter: unsetter,
			}
			err := c.CleanStaleMarkers()
			if err == nil {
				t.Fatalf("expected non-nil error to propagate genuine failure; got nil")
			}
			if !errors.Is(err, tc.wantWrap) {
				t.Errorf("expected returned error to wrap %v, got %v", tc.wantWrap, err)
			}
			if len(unsetter.calls) != 0 {
				t.Errorf("expected zero unset calls when a dependency fails, got %d (%v)", len(unsetter.calls), unsetter.calls)
			}
		})
	}
}
