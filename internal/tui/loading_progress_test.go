package tui_test

import (
	"math"
	"reflect"
	"testing"

	"github.com/leeovery/portal/internal/tui"
)

const floatEps = 1e-9

func feed(acc tui.LoadingProgress, events ...tui.BootstrapProgressMsg) tui.LoadingProgress {
	for _, e := range events {
		acc = acc.Apply(e)
	}
	return acc
}

func activeLabelText(v tui.LoadingProgressView) string {
	for _, l := range v.Labels {
		if l.State == tui.LabelActive {
			return l.Text
		}
	}
	return ""
}

func TestStepMapsToFriendlyLabel(t *testing.T) {
	cases := []struct {
		name      string
		event     tui.BootstrapProgressMsg
		wantLabel string
	}{
		{"step 1 EnsureServer", tui.BootstrapProgressMsg{Index: 1}, tui.LabelStartedTmuxServer},
		{"step 2 RegisterPortalHooks", tui.BootstrapProgressMsg{Index: 2}, tui.LabelRegisteredHooks},
		{"step 3 SetRestoring", tui.BootstrapProgressMsg{Index: 3}, tui.LabelRegisteredHooks},
		{"step 4 SweepOrphanDaemons", tui.BootstrapProgressMsg{Index: 4}, tui.LabelRegisteredHooks},
		{"step 5 EnsureSaver", tui.BootstrapProgressMsg{Index: 5}, tui.LabelRegisteredHooks},
		{"step 6 Restore skeleton (M>0)", tui.BootstrapProgressMsg{Index: 6, RestoreN: 1, RestoreM: 3}, tui.LabelRestoringSessions},
		{"step 6 Restore complete (M==0)", tui.BootstrapProgressMsg{Index: 6}, tui.LabelReplayingScrollback},
		{"step 7 EagerSignalHydrate", tui.BootstrapProgressMsg{Index: 7}, tui.LabelReplayingScrollback},
		{"step 8 ClearRestoring", tui.BootstrapProgressMsg{Index: 8}, tui.LabelRunningResumeCommands},
		{"step 9 CleanStaleMarkers", tui.BootstrapProgressMsg{Index: 9}, tui.LabelRunningResumeCommands},
		{"step 10 SweepOrphanFIFOs", tui.BootstrapProgressMsg{Index: 10}, tui.LabelRunningResumeCommands},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tui.LabelForStep(tc.event); got != tc.wantLabel {
				t.Errorf("LabelForStep(step %d) = %q; want %q", tc.event.Index, got, tc.wantLabel)
			}
		})
	}
}

func TestBarAdvancesEveryStep(t *testing.T) {
	acc := tui.LoadingProgress{}
	if f := acc.View().BarFraction; f != 0 {
		t.Fatalf("initial bar fraction = %v; want 0", f)
	}
	for step := 1; step <= 10; step++ {
		acc = acc.Apply(tui.BootstrapProgressMsg{Index: step})
		want := float64(step) / 10.0
		got := acc.View().BarFraction
		if math.Abs(got-want) > floatEps {
			t.Errorf("after step %d bar fraction = %v; want %v", step, got, want)
		}
		if step < 10 && got >= 1.0 {
			t.Errorf("bar reached 100%% at step %d; must reach 100%% only after step 10", step)
		}
	}
	if got := acc.View().BarFraction; math.Abs(got-1.0) > floatEps {
		t.Errorf("after step 10 bar fraction = %v; want 1.0", got)
	}
}

func TestLabelStateTransitions(t *testing.T) {
	acc := tui.LoadingProgress{}

	assertStates(t, acc.View(), map[string]tui.LabelState{
		tui.LabelStartedTmuxServer:     tui.LabelPending,
		tui.LabelRegisteredHooks:       tui.LabelPending,
		tui.LabelRestoringSessions:     tui.LabelPending,
		tui.LabelReplayingScrollback:   tui.LabelPending,
		tui.LabelRunningResumeCommands: tui.LabelPending,
	})

	acc = acc.Apply(tui.BootstrapProgressMsg{Index: 1})
	assertStates(t, acc.View(), map[string]tui.LabelState{
		tui.LabelStartedTmuxServer:     tui.LabelDone,
		tui.LabelRegisteredHooks:       tui.LabelActive,
		tui.LabelRestoringSessions:     tui.LabelPending,
		tui.LabelReplayingScrollback:   tui.LabelPending,
		tui.LabelRunningResumeCommands: tui.LabelPending,
	})

	for step := 2; step <= 5; step++ {
		acc = acc.Apply(tui.BootstrapProgressMsg{Index: step})
	}
	assertStates(t, acc.View(), map[string]tui.LabelState{
		tui.LabelStartedTmuxServer:     tui.LabelDone,
		tui.LabelRegisteredHooks:       tui.LabelDone,
		tui.LabelRestoringSessions:     tui.LabelActive,
		tui.LabelReplayingScrollback:   tui.LabelPending,
		tui.LabelRunningResumeCommands: tui.LabelPending,
	})

	for step := 6; step <= 10; step++ {
		acc = acc.Apply(tui.BootstrapProgressMsg{Index: step})
	}
	assertStates(t, acc.View(), map[string]tui.LabelState{
		tui.LabelStartedTmuxServer:     tui.LabelDone,
		tui.LabelRegisteredHooks:       tui.LabelDone,
		tui.LabelRestoringSessions:     tui.LabelDone,
		tui.LabelReplayingScrollback:   tui.LabelDone,
		tui.LabelRunningResumeCommands: tui.LabelDone,
	})
	if got := activeLabelText(acc.View()); got != "" {
		t.Errorf("after all steps active label = %q; want none", got)
	}
}

func TestMultiStepLabelStaysActiveUntilLastStep(t *testing.T) {
	acc := feed(tui.LoadingProgress{},
		tui.BootstrapProgressMsg{Index: 1},
		tui.BootstrapProgressMsg{Index: 2},
		tui.BootstrapProgressMsg{Index: 3},
		tui.BootstrapProgressMsg{Index: 4},
	)
	if got := labelState(acc.View(), tui.LabelRegisteredHooks); got != tui.LabelActive {
		t.Errorf("Registered hooks state after step 4 = %v; want active (last step 5 not done)", got)
	}

	acc = acc.Apply(tui.BootstrapProgressMsg{Index: 5})
	if got := labelState(acc.View(), tui.LabelRegisteredHooks); got != tui.LabelDone {
		t.Errorf("Registered hooks state after step 5 = %v; want done", got)
	}
}

func TestRestoringSessionsCounter(t *testing.T) {
	acc := feed(tui.LoadingProgress{},
		tui.BootstrapProgressMsg{Index: 1},
		tui.BootstrapProgressMsg{Index: 2},
		tui.BootstrapProgressMsg{Index: 3},
		tui.BootstrapProgressMsg{Index: 4},
		tui.BootstrapProgressMsg{Index: 5},
		tui.BootstrapProgressMsg{Index: 6, RestoreN: 1, RestoreM: 3},
	)
	v := acc.View()
	if got := counterText(v, tui.LabelRestoringSessions); got != "1/3" {
		t.Errorf("Restoring sessions counter = %q; want %q", got, "1/3")
	}
	if got := labelState(v, tui.LabelRestoringSessions); got != tui.LabelActive {
		t.Errorf("mid-flight Restoring sessions state = %v; want active (step 6 not yet complete)", got)
	}
	if got := labelState(v, tui.LabelReplayingScrollback); got != tui.LabelPending {
		t.Errorf("mid-flight Replaying scrollback state = %v; want pending", got)
	}
	if want := 5.0 / 10.0; math.Abs(v.BarFraction-want) > floatEps {
		t.Errorf("mid-flight bar fraction = %v; want %v (skeleton event must not advance step 6)", v.BarFraction, want)
	}
	for _, l := range v.Labels {
		if l.Text == tui.LabelRestoringSessions {
			continue
		}
		if l.Counter != "" {
			t.Errorf("label %q carries counter %q; only Restoring sessions may", l.Text, l.Counter)
		}
	}

	acc = acc.Apply(tui.BootstrapProgressMsg{Index: 6, RestoreN: 3, RestoreM: 3})
	if got := counterText(acc.View(), tui.LabelRestoringSessions); got != "3/3" {
		t.Errorf("Restoring sessions counter after N=3 = %q; want %q", got, "3/3")
	}

	acc = acc.Apply(tui.BootstrapProgressMsg{Index: 6})
	done := acc.View()
	if got := labelState(done, tui.LabelRestoringSessions); got != tui.LabelDone {
		t.Errorf("after completion tick Restoring sessions state = %v; want done", got)
	}
	if want := 6.0 / 10.0; math.Abs(done.BarFraction-want) > floatEps {
		t.Errorf("after completion tick bar fraction = %v; want %v", done.BarFraction, want)
	}
	if got := counterText(done, tui.LabelRestoringSessions); got != "3/3" {
		t.Errorf("after completion tick counter = %q; want %q (sticky last N/M)", got, "3/3")
	}
}

func TestEmptyRestoreSuppressesCounterAndTicksDone(t *testing.T) {
	acc := feed(tui.LoadingProgress{},
		tui.BootstrapProgressMsg{Index: 1},
		tui.BootstrapProgressMsg{Index: 2},
		tui.BootstrapProgressMsg{Index: 3},
		tui.BootstrapProgressMsg{Index: 4},
		tui.BootstrapProgressMsg{Index: 5},
		tui.BootstrapProgressMsg{Index: 6},
	)
	v := acc.View()

	if got := counterText(v, tui.LabelRestoringSessions); got != "" {
		t.Errorf("M=0: Restoring sessions counter = %q; want empty (suppressed)", got)
	}
	if got := labelState(v, tui.LabelRestoringSessions); got != tui.LabelDone {
		t.Errorf("M=0: Restoring sessions state = %v; want done (not stalled)", got)
	}
	want := 6.0 / 10.0
	if math.Abs(v.BarFraction-want) > floatEps {
		t.Errorf("M=0: bar fraction after step 6 = %v; want %v", v.BarFraction, want)
	}
}

func TestRunningResumeCommandsTicksDoneWithNoItems(t *testing.T) {
	var acc tui.LoadingProgress
	for step := 1; step <= 5; step++ {
		acc = acc.Apply(tui.BootstrapProgressMsg{Index: step})
	}
	if got := labelState(acc.View(), tui.LabelRunningResumeCommands); got != tui.LabelPending {
		t.Fatalf("Running resume commands before its group = %v; want pending", got)
	}
	for step := 6; step <= 9; step++ {
		acc = acc.Apply(tui.BootstrapProgressMsg{Index: step})
	}
	if got := labelState(acc.View(), tui.LabelRunningResumeCommands); got != tui.LabelActive {
		t.Errorf("Running resume commands at step 9 = %v; want active (last step 10 not done)", got)
	}
	acc = acc.Apply(tui.BootstrapProgressMsg{Index: 10})
	v := acc.View()
	if got := labelState(v, tui.LabelRunningResumeCommands); got != tui.LabelDone {
		t.Errorf("Running resume commands at step 10 = %v; want done (no per-item work)", got)
	}
	if got := counterText(v, tui.LabelRunningResumeCommands); got != "" {
		t.Errorf("Running resume commands counter = %q; want empty (no per-item counter)", got)
	}
}

func TestIdempotentPerStepIndex(t *testing.T) {
	acc := feed(tui.LoadingProgress{},
		tui.BootstrapProgressMsg{Index: 1},
		tui.BootstrapProgressMsg{Index: 1},
		tui.BootstrapProgressMsg{Index: 1},
	)
	if got := acc.View().BarFraction; math.Abs(got-1.0/10.0) > floatEps {
		t.Errorf("3× step 1: bar = %v; want %v (no double-advance)", got, 1.0/10.0)
	}

	acc = feed(tui.LoadingProgress{},
		tui.BootstrapProgressMsg{Index: 3},
		tui.BootstrapProgressMsg{Index: 2},
	)
	if got := acc.View().BarFraction; math.Abs(got-2.0/10.0) > floatEps {
		t.Errorf("steps 3,2 out of order: bar = %v; want %v", got, 2.0/10.0)
	}
}

func TestMappingCoversAllTenStepsNoGaps(t *testing.T) {
	valid := map[string]bool{
		tui.LabelStartedTmuxServer:     true,
		tui.LabelRegisteredHooks:       true,
		tui.LabelRestoringSessions:     true,
		tui.LabelReplayingScrollback:   true,
		tui.LabelRunningResumeCommands: true,
	}
	for step := 1; step <= 10; step++ {
		got := tui.LabelForStep(tui.BootstrapProgressMsg{Index: step})
		if got == "" {
			t.Errorf("step %d resolved to no label (gap in the step→label mapping)", step)
			continue
		}
		if !valid[got] {
			t.Errorf("step %d resolved to unknown label %q", step, got)
		}
	}
	for _, bad := range []int{0, 11, 12, 99} {
		if got := tui.LabelForStep(tui.BootstrapProgressMsg{Index: bad}); got != "" {
			t.Errorf("out-of-range step %d mapped to label %q; want none", bad, got)
		}
		v := feed(tui.LoadingProgress{}, tui.BootstrapProgressMsg{Index: bad}).View()
		if f := v.BarFraction; f != 0 {
			t.Errorf("out-of-range step %d advanced the bar to %v; want 0", bad, f)
		}
	}

	v := tui.LoadingProgress{}.View()
	if len(v.Labels) != 5 {
		t.Fatalf("View().Labels length = %d; want 5", len(v.Labels))
	}
	wantOrder := []string{
		tui.LabelStartedTmuxServer,
		tui.LabelRegisteredHooks,
		tui.LabelRestoringSessions,
		tui.LabelReplayingScrollback,
		tui.LabelRunningResumeCommands,
	}
	for i, want := range wantOrder {
		if v.Labels[i].Text != want {
			t.Errorf("label[%d] = %q; want %q", i, v.Labels[i].Text, want)
		}
	}
}

func TestRemovedStep11IsUnmapped(t *testing.T) {
	if got := tui.LabelForStep(tui.BootstrapProgressMsg{Index: 11}); got != "" {
		t.Errorf("LabelForStep(step 11) = %q; want empty (step 11 removed)", got)
	}
	v := feed(tui.LoadingProgress{}, tui.BootstrapProgressMsg{Index: 11}).View()
	if f := v.BarFraction; f != 0 {
		t.Errorf("removed step 11 advanced the bar to %v; want 0", f)
	}
}

func TestBootstrapProgressMsgCarriesOnlyConsumedFields(t *testing.T) {
	want := map[string]bool{"Index": true, "RestoreN": true, "RestoreM": true}
	tp := reflect.TypeFor[tui.BootstrapProgressMsg]()
	got := map[string]bool{}
	for field := range tp.Fields() {
		got[field.Name] = true
	}
	for name := range got {
		if !want[name] {
			t.Errorf("BootstrapProgressMsg carries field %q — only Index/RestoreN/RestoreM may ride the wire (the label mapping is loading_progress.go's sole authority)", name)
		}
	}
	for name := range want {
		if !got[name] {
			t.Errorf("BootstrapProgressMsg is missing the consumed field %q", name)
		}
	}
}

func assertStates(t *testing.T, v tui.LoadingProgressView, want map[string]tui.LabelState) {
	t.Helper()
	for text, wantState := range want {
		if got := labelState(v, text); got != wantState {
			t.Errorf("label %q state = %v; want %v", text, got, wantState)
		}
	}
}

func labelState(v tui.LoadingProgressView, text string) tui.LabelState {
	for _, l := range v.Labels {
		if l.Text == text {
			return l.State
		}
	}
	return tui.LabelState(-1)
}

func counterText(v tui.LoadingProgressView, text string) string {
	for _, l := range v.Labels {
		if l.Text == text {
			return l.Counter
		}
	}
	return ""
}
