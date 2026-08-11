package bootstrap

import (
	"context"
	"testing"
)

func TestRun_EmitsProgressEventPerStepInOrder(t *testing.T) {
	r := &stepRecorder{}
	o := newOrchestrator(r, nil)

	var got []StepEvent
	ctx := WithProgressEmitter(context.Background(), func(ev StepEvent) {
		got = append(got, ev)
	})

	if _, _, err := o.Run(ctx); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	want := []StepEvent{
		{Index: 1, Name: stepEnsureServer},
		{Index: 2, Name: stepRegisterHooks},
		{Index: 3, Name: stepSetRestoring},
		{Index: 4, Name: stepSweepOrphanDaemons},
		{Index: 5, Name: stepEnsureSaver},
		{Index: 6, Name: stepRestore},
		{Index: 7, Name: stepEagerSignalHydrate},
		{Index: 8, Name: stepClearRestoring},
		{Index: 9, Name: stepCleanStaleMarkers},
		{Index: 10, Name: stepSweepOrphanFIFOs},
	}
	if len(got) != len(want) {
		t.Fatalf("emitted %d events, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("event[%d] = %+v, want %+v", i, got[i], want[i])
		}
	}
}

func TestRun_NoEmitterIsNoOp(t *testing.T) {
	r := &stepRecorder{}
	o := newOrchestrator(r, nil)

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run returned error: %v", err)
	}

	want := []string{
		"EnsureServer", "RegisterPortalHooks", "Set", "SweepOrphanDaemons",
		"EnsureSaver", "Restore", "EagerSignalHydrate", "Clear",
		"CleanStaleMarkers", "Sweep",
	}
	if !equalCalls(r.calls, want) {
		t.Errorf("call order = %v, want %v", r.calls, want)
	}
}

func TestRun_FatalStepStopsEmitting(t *testing.T) {
	r := &stepRecorder{EnsureServerErr: context.Canceled}
	o := newOrchestrator(r, nil)

	var got []StepEvent
	ctx := WithProgressEmitter(context.Background(), func(ev StepEvent) {
		got = append(got, ev)
	})

	if _, _, err := o.Run(ctx); err == nil {
		t.Fatal("Run returned nil error, want fatal from EnsureServer")
	}
	if len(got) != 0 {
		t.Errorf("emitted %d events on fatal step 1, want 0: %+v", len(got), got)
	}
}
