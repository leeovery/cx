//go:build integration

package restore_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// recordingReporter stands in for *testing.T so a test can observe what the
// marker assertion reports without failing itself.
type recordingReporter struct {
	errors []string
	fatals []string
}

type fatalSentinel struct{}

func (r *recordingReporter) Helper() {}

func (r *recordingReporter) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *recordingReporter) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, fmt.Sprintf(format, args...))
	panic(fatalSentinel{})
}

// run drives fn, absorbing the abort a Fatalf stands for.
func (r *recordingReporter) run(fn func()) {
	defer func() {
		if rec := recover(); rec != nil {
			if _, ok := rec.(fatalSentinel); !ok {
				panic(rec)
			}
		}
	}()
	fn()
}

func (r *recordingReporter) failed() bool { return len(r.errors)+len(r.fatals) > 0 }

func (r *recordingReporter) report() string {
	return fmt.Sprintf("errors=%v fatals=%v", r.errors, r.fatals)
}

func TestAssertMarkerCount_LateMarkerStillSatisfies(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hook-fired.txt")

	go func() {
		time.Sleep(300 * time.Millisecond)
		if err := os.WriteFile(path, []byte("HOOK_FIRED\n"), 0o644); err != nil {
			panic(err)
		}
	}()

	rep := &recordingReporter{}
	rep.run(func() { assertMarkerCountOn(rep, path, "HOOK_FIRED", 1) })

	if rep.failed() {
		t.Fatalf("a marker written after the assertion started should still satisfy it; got %s", rep.report())
	}
}

func TestAssertMarkerCount_WantZeroFailsWhenMarkerAppears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hook-fired.txt")
	if err := os.WriteFile(path, []byte("PANE1_HOOK_FIRED\n"), 0o644); err != nil {
		t.Fatalf("seed marker file: %v", err)
	}

	rep := &recordingReporter{}
	rep.run(func() { assertMarkerCountOn(rep, path, "PANE1_HOOK_FIRED", 0) })

	if !rep.failed() {
		t.Fatal("a want of 0 must fail when the marker is present, or the absence assertion proves nothing")
	}
}

func TestAssertMarkerCount_WantZeroWaitsOutTheBudget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hook-fired.txt")

	rep := &recordingReporter{}
	start := time.Now()
	rep.run(func() { assertMarkerCountOn(rep, path, "PANE1_HOOK_FIRED", 0) })
	elapsed := time.Since(start)

	if rep.failed() {
		t.Fatalf("a marker that never appears satisfies a want of 0; got %s", rep.report())
	}
	if elapsed < hydrateBudget {
		t.Fatalf("want-0 returned after %v, before the %v budget elapsed; an absence seen immediately is the "+
			"absence of a hook that has not fired yet", elapsed, hydrateBudget)
	}
}
