package bootstrap

import (
	"errors"
	"log/slog"
	"sort"
	"testing"

	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/statetest"
)

func TestEagerHydrateSignalerInterfaceContract(t *testing.T) {
	var _ EagerHydrateSignaler = NoOpEagerHydrateSignaler{}
	var _ EagerHydrateSignaler = (*EagerSignalCore)(nil)
}

func TestNoOpEagerHydrateSignaler_ReturnsNil(t *testing.T) {
	if err := (NoOpEagerHydrateSignaler{}).EagerSignalHydrate(); err != nil {
		t.Errorf("NoOpEagerHydrateSignaler.EagerSignalHydrate = %v; want nil", err)
	}
}

func TestEagerSignalHydrate_WritesSignalToEveryMarkerFIFO(t *testing.T) {
	stateDir := "/var/state"
	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"alpha__0.0": {},
		"beta__1.2":  {},
		"gamma__2.0": {},
	}}
	signaler := &statetest.RecordingFIFOSignaler{}

	c := &EagerSignalCore{
		Markers:  lister,
		StateDir: stateDir,
		Signaler: signaler,
	}

	if err := c.EagerSignalHydrate(); err != nil {
		t.Fatalf("EagerSignalHydrate returned error: %v", err)
	}

	want := []string{
		state.FIFOPath(stateDir, "alpha__0.0"),
		state.FIFOPath(stateDir, "beta__1.2"),
		state.FIFOPath(stateDir, "gamma__2.0"),
	}
	got := append([]string{}, signaler.Calls...)
	sort.Strings(want)
	sort.Strings(got)
	if len(got) != len(want) {
		t.Fatalf("Signaler.SendSignal call count = %d (%v); want %d (%v)", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("Signaler.SendSignal call[%d] = %q; want %q", i, got[i], want[i])
		}
	}
}

func TestEagerSignalHydrate_ZeroMarkersIsNoOp(t *testing.T) {
	lister := &fakeMarkerLister{markers: map[string]struct{}{}}
	signaler := &statetest.RecordingFIFOSignaler{}

	c := &EagerSignalCore{
		Markers:  lister,
		StateDir: "/var/state",
		Signaler: signaler,
	}

	if err := c.EagerSignalHydrate(); err != nil {
		t.Fatalf("EagerSignalHydrate returned error: %v", err)
	}
	if len(signaler.Calls) != 0 {
		t.Errorf("Signaler.SendSignal called %d times under zero-marker no-op; want 0 (calls=%v)", len(signaler.Calls), signaler.Calls)
	}
}

func TestEagerSignalHydrate_PerFIFOWriteFailureLogsAndContinues(t *testing.T) {
	sink := installCleanSummarySink(t)
	stateDir := "/var/state"
	failPath := state.FIFOPath(stateDir, "broken__0.0")
	sentinel := errors.New("write fifo: i/o error")

	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"broken__0.0":   {},
		"healthy__1.0":  {},
		"healthy2__2.0": {},
	}}
	signaler := &statetest.RecordingFIFOSignaler{
		ErrOn: map[string]error{failPath: sentinel},
	}

	c := &EagerSignalCore{
		Markers:  lister,
		StateDir: stateDir,
		Signaler: signaler,
	}

	if err := c.EagerSignalHydrate(); err != nil {
		t.Fatalf("EagerSignalHydrate must return nil after per-FIFO write failure; got %v", err)
	}
	if len(signaler.Calls) != 3 {
		t.Errorf("Signaler.SendSignal call count = %d; want 3 (loop must continue past the failing write); calls=%v", len(signaler.Calls), signaler.Calls)
	}

	warns := sink.matching(slog.LevelWarn, "signal", "eager-signal write fifo failed")
	if len(warns) != 1 {
		t.Fatalf("expected 1 WARN under component=signal for the failing FIFO, got %d: %+v", len(warns), sink.Records())
	}
	rec := warns[0]
	if p, ok := rec.Attrs["path"]; !ok || p.String() != failPath {
		t.Errorf("WARN path attr = %v; want %q", rec.Attrs["path"], failPath)
	}
	if ec, ok := rec.Attrs["error_class"]; !ok || ec.String() != "unexpected" {
		t.Errorf("WARN error_class attr = %v; want %q", rec.Attrs["error_class"], "unexpected")
	}
	errAttr, ok := rec.Attrs["error"]
	if !ok {
		t.Fatalf("WARN missing error attr: %+v", rec.Attrs)
	}
	if errAttr.Kind() != slog.KindAny {
		t.Errorf("error attr kind = %v; want Any (wrapped err passed directly, not .Error())", errAttr.Kind())
	}
	if gotErr, ok := errAttr.Any().(error); !ok || !errors.Is(gotErr, sentinel) {
		t.Errorf("error attr = %v; want errors.Is(err, sentinel)=true", errAttr.Any())
	}
}

func TestEagerSignalHydrate_ReturnsErrorWhenListSkeletonMarkersFails(t *testing.T) {
	sentinel := errors.New("show-options boom")
	lister := &fakeMarkerLister{err: sentinel}
	signaler := &statetest.RecordingFIFOSignaler{}

	c := &EagerSignalCore{
		Markers:  lister,
		StateDir: "/var/state",
		Signaler: signaler,
	}

	err := c.EagerSignalHydrate()
	if err == nil {
		t.Fatal("EagerSignalHydrate returned nil; want wrapped error")
	}
	if !errors.Is(err, sentinel) {
		t.Errorf("EagerSignalHydrate err = %v; want errors.Is(err, sentinel)=true", err)
	}
	if len(signaler.Calls) != 0 {
		t.Errorf("Signaler.SendSignal called %d times after enumeration failure; want 0", len(signaler.Calls))
	}
}

func TestEagerSignalHydrate_NilLoggerTolerated(t *testing.T) {
	stateDir := "/var/state"
	failPath := state.FIFOPath(stateDir, "broken__0.0")

	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"broken__0.0":  {},
		"healthy__1.0": {},
	}}
	signaler := &statetest.RecordingFIFOSignaler{
		ErrOn: map[string]error{failPath: errors.New("boom")},
	}

	c := &EagerSignalCore{
		Markers:  lister,
		StateDir: stateDir,
		Signaler: signaler,
		Logger:   nil,
	}

	if err := c.EagerSignalHydrate(); err != nil {
		t.Fatalf("EagerSignalHydrate with nil Logger returned error: %v", err)
	}
	if len(signaler.Calls) != 2 {
		t.Errorf("Signaler.SendSignal call count = %d; want 2 (loop must continue past failing write under nil Logger); calls=%v", len(signaler.Calls), signaler.Calls)
	}
}

func TestEagerSignalHydrate_SuccessEmitsSignalledDebugBreadcrumb(t *testing.T) {
	sink := installCleanSummarySink(t)
	stateDir := "/var/state"

	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"alpha__0.0": {},
		"beta__1.2":  {},
	}}
	signaler := &statetest.RecordingFIFOSignaler{}

	c := &EagerSignalCore{
		Markers:  lister,
		StateDir: stateDir,
		Signaler: signaler,
	}

	if err := c.EagerSignalHydrate(); err != nil {
		t.Fatalf("EagerSignalHydrate returned error: %v", err)
	}

	dbg := sink.matching(slog.LevelDebug, "signal", "fifo signalled")
	if len(dbg) != 2 {
		t.Fatalf("expected 2 DEBUG 'fifo signalled' under component=signal, got %d: %+v", len(dbg), sink.Records())
	}
	gotPaths := map[string]bool{}
	for _, r := range dbg {
		p, ok := r.Attrs["path"]
		if !ok {
			t.Fatalf("DEBUG 'fifo signalled' missing path attr: %+v", r.Attrs)
		}
		gotPaths[p.String()] = true
	}
	for _, key := range []string{"alpha__0.0", "beta__1.2"} {
		wantPath := state.FIFOPath(stateDir, key)
		if !gotPaths[wantPath] {
			t.Errorf("missing DEBUG 'fifo signalled' for path %q; got %v", wantPath, gotPaths)
		}
	}

	if got := sink.matching(slog.LevelInfo, "signal", "fifo signalled"); len(got) != 0 {
		t.Errorf("'fifo signalled' must be DEBUG, not INFO; got %d INFO entries: %+v", len(got), got)
	}
}

func TestEagerSignalHydrate_NoSignalingLineUnderHydrateOrBootstrap(t *testing.T) {
	sink := installCleanSummarySink(t)
	stateDir := "/var/state"
	failPath := state.FIFOPath(stateDir, "broken__0.0")

	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"broken__0.0":  {},
		"healthy__1.0": {},
	}}
	signaler := &statetest.RecordingFIFOSignaler{
		ErrOn: map[string]error{failPath: errors.New("write fifo: i/o error")},
	}

	c := &EagerSignalCore{
		Markers:  lister,
		StateDir: stateDir,
		Signaler: signaler,
	}

	if err := c.EagerSignalHydrate(); err != nil {
		t.Fatalf("EagerSignalHydrate returned error: %v", err)
	}

	for _, r := range sink.Records() {
		comp, ok := r.Attrs["component"]
		if !ok {
			continue
		}
		if comp.String() == "hydrate" || comp.String() == "bootstrap" {
			t.Errorf("no signaling-mechanism line may render under %q; got %+v", comp.String(), r)
		}
	}
}

func TestEagerSignalHydrate_NoCycleSummaryNorNewAttrKeys(t *testing.T) {
	sink := installCleanSummarySink(t)
	stateDir := "/var/state"
	failPath := state.FIFOPath(stateDir, "broken__0.0")

	lister := &fakeMarkerLister{markers: map[string]struct{}{
		"broken__0.0":  {},
		"healthy__1.0": {},
	}}
	signaler := &statetest.RecordingFIFOSignaler{
		ErrOn: map[string]error{failPath: errors.New("write fifo: i/o error")},
	}

	c := &EagerSignalCore{
		Markers:  lister,
		StateDir: stateDir,
		Signaler: signaler,
	}

	if err := c.EagerSignalHydrate(); err != nil {
		t.Fatalf("EagerSignalHydrate returned error: %v", err)
	}

	for _, r := range sink.Records() {
		if r.Level == slog.LevelInfo {
			t.Errorf("EagerSignalHydrate must not emit an INFO cycle summary; got %+v", r)
		}
	}

	allowed := map[string]bool{"component": true, "path": true, "error": true, "error_class": true}
	for _, r := range sink.Records() {
		for key := range r.Attrs {
			if !allowed[key] {
				t.Errorf("unexpected attr key %q on signal line %q; closed set is path/error/error_class", key, r.Msg)
			}
		}
	}
}

func TestOrchestrator_HasEagerSignalerField(t *testing.T) {
	o := &Orchestrator{
		EagerSignaler: NoOpEagerHydrateSignaler{},
	}
	if o.EagerSignaler == nil {
		t.Fatal("Orchestrator.EagerSignaler unexpectedly nil after explicit assignment")
	}
	if err := o.EagerSignaler.EagerSignalHydrate(); err != nil {
		t.Errorf("NoOp injected via Orchestrator.EagerSignaler returned %v; want nil", err)
	}
}
