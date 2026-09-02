package restoretest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/harnesstest"
)

// probeAssertion is a fast-polling variant, so a case exercising the poll
// need not pay the promoted budget a real reboot needs.
func probeAssertion(path, marker string, want int) markerAssertion {
	return markerAssertion{
		path:   path,
		marker: marker,
		want:   want,
		budget: 400 * time.Millisecond,
		tick:   10 * time.Millisecond,
	}
}

// writeMarkerAfter appends marker to path once delay has elapsed, reporting a
// write failure on the returned channel rather than aborting the process: a
// panic in this goroutine would take the whole test binary down with it.
func writeMarkerAfter(path, marker string, delay time.Duration) <-chan error {
	done := make(chan error, 1)
	go func() {
		time.Sleep(delay)
		f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err != nil {
			done <- err
			return
		}
		if _, err := f.WriteString(marker + "\n"); err != nil {
			done <- err
			_ = f.Close()
			return
		}
		done <- f.Close()
	}()
	return done
}

func TestAssertMarkerCount_MarkerArrivingAfterTheAssertionStarts(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hook-fired.txt")
	written := writeMarkerAfter(path, "HOOK_FIRED", 150*time.Millisecond)

	rep := &harnesstest.Recorder{}
	rep.Run(func() { probeAssertion(path, "HOOK_FIRED", 1).run(rep) })

	if err := <-written; err != nil {
		t.Fatalf("marker writer: %v", err)
	}
	if rep.Failed() {
		t.Fatalf("a marker written after the assertion started should still satisfy it; got %s", rep.Report())
	}
}

func TestAssertMarkerCount_MarkerNeverAppears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hook-fired.txt")

	rep := &harnesstest.Recorder{}
	start := time.Now()
	rep.Run(func() { probeAssertion(path, "HOOK_FIRED", 1).run(rep) })
	elapsed := time.Since(start)

	if !rep.Failed() {
		t.Fatal("a marker that never appears must fail a want of 1")
	}
	if elapsed < 400*time.Millisecond {
		t.Errorf("failed after %v, before the budget elapsed; the assertion gave up early", elapsed)
	}
}

func TestAssertMarkerCount_MarkerFiresMoreOftenThanWanted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hook-fired.txt")
	if err := os.WriteFile(path, []byte("HOOK_FIRED\nHOOK_FIRED\n"), 0o644); err != nil {
		t.Fatalf("seed marker file: %v", err)
	}

	rep := &harnesstest.Recorder{}
	start := time.Now()
	rep.Run(func() { probeAssertion(path, "HOOK_FIRED", 1).run(rep) })
	elapsed := time.Since(start)

	if !rep.Failed() {
		t.Fatal("a marker past the wanted count must fail")
	}
	if elapsed >= 400*time.Millisecond {
		t.Errorf("waited %v for a count that can only grow; an overshoot is final", elapsed)
	}
}

func TestAssertMarkerCount_WantZeroFailsWhenMarkerAppears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hook-fired.txt")
	if err := os.WriteFile(path, []byte("PANE1_HOOK_FIRED\n"), 0o644); err != nil {
		t.Fatalf("seed marker file: %v", err)
	}

	rep := &harnesstest.Recorder{}
	rep.Run(func() { probeAssertion(path, "PANE1_HOOK_FIRED", 0).run(rep) })

	if !rep.Failed() {
		t.Fatal("a want of 0 must fail when the marker is present, or the absence assertion proves nothing")
	}
}

func TestAssertMarkerCount_WantZeroWaitsOutTheBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hook-fired.txt")

	rep := &harnesstest.Recorder{}
	start := time.Now()
	rep.Run(func() { probeAssertion(path, "PANE1_HOOK_FIRED", 0).run(rep) })
	elapsed := time.Since(start)

	if rep.Failed() {
		t.Fatalf("a marker that never appears satisfies a want of 0; got %s", rep.Report())
	}
	if elapsed < 400*time.Millisecond {
		t.Fatalf("want-0 returned after %v, before the budget elapsed; an absence seen immediately is the "+
			"absence of a hook that has not fired yet", elapsed)
	}
}

func TestAssertMarkerCount_AbsentFileCountsAsZero(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never-created.txt")

	rep := &harnesstest.Recorder{}
	rep.Run(func() { probeAssertion(path, "HOOK_FIRED", 0).run(rep) })

	if len(rep.Fatals) != 0 {
		t.Fatalf("an absent marker file is a count of 0, not a read failure; got fatals=%v", rep.Fatals)
	}
	if rep.Failed() {
		t.Fatalf("an absent marker file satisfies a want of 0; got %s", rep.Report())
	}
}

func TestAssertMarkerCount_WriterFailureIsReportedNotPanicked(t *testing.T) {
	unwritable := filepath.Join(t.TempDir(), "no-such-dir", "hook-fired.txt")
	written := writeMarkerAfter(unwritable, "HOOK_FIRED", 10*time.Millisecond)

	if err := <-written; err == nil {
		t.Fatal("writing under a missing directory must fail, or this test proves nothing")
	}
}

func TestAssertMarkerCount_ExportedEntryPointUsesTheSharedBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hook-fired.txt")
	written := writeMarkerAfter(path, "HOOK_FIRED", 100*time.Millisecond)

	AssertMarkerCount(t, path, "HOOK_FIRED", 1)

	if err := <-written; err != nil {
		t.Fatalf("marker writer: %v", err)
	}
}
