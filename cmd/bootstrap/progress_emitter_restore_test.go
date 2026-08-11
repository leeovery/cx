package bootstrap

import (
	"context"
	"testing"
)

type progressRecorder struct {
	stepRecorder
	m          int
	progressFn func(n, m int)
}

func (r *progressRecorder) SetProgress(fn func(n, m int)) { r.progressFn = fn }

func (r *progressRecorder) Restore() (bool, error) {
	r.calls = append(r.calls, "Restore")
	if r.progressFn != nil {
		for n := 1; n <= r.m; n++ {
			r.progressFn(n, r.m)
		}
	}
	return r.RestoreCorrupt, r.RestoreErr
}

func newProgressOrchestrator(r *progressRecorder) *Orchestrator {
	o := newOrchestrator(&r.stepRecorder, nil)
	o.Restore = r
	return o
}

func TestRun_InstallsRestoreProgressAndForwardsNMOntoEmitter(t *testing.T) {
	r := &progressRecorder{m: 3}
	o := newProgressOrchestrator(r)

	var got []StepEvent
	ctx := WithProgressEmitter(context.Background(), func(ev StepEvent) {
		got = append(got, ev)
	})
	if _, _, err := o.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}

	var nm [][2]int
	for _, ev := range got {
		if ev.RestoreM > 0 {
			if ev.Index != 6 || ev.Name != stepRestore {
				t.Errorf("restore-progress event = %+v, want Index 6 / %q", ev, stepRestore)
			}
			nm = append(nm, [2]int{ev.RestoreN, ev.RestoreM})
		}
	}
	want := [][2]int{{1, 3}, {2, 3}, {3, 3}}
	if len(nm) != len(want) {
		t.Fatalf("restore-progress N/M = %v, want %v", nm, want)
	}
	for i := range want {
		if nm[i] != want[i] {
			t.Errorf("restore-progress[%d] = %v, want %v", i, nm[i], want[i])
		}
	}

	var sawStepTick bool
	for _, ev := range got {
		if ev.Index == 6 && ev.RestoreM == 0 {
			sawStepTick = true
		}
	}
	if !sawStepTick {
		t.Error("restore step-complete tick (Index 6, zero N/M) never fired")
	}
}

func TestRun_NoEmitterDoesNotInstallRestoreProgress(t *testing.T) {
	r := &progressRecorder{m: 3}
	o := newProgressOrchestrator(r)

	if _, _, err := o.Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if r.progressFn != nil {
		t.Error("SetProgress was called on the synchronous route — restore progress must only install when an emitter is wired")
	}
}
