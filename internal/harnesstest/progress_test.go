package harnesstest_test

import (
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/harnesstest"
)

func TestAwaitProgress(t *testing.T) {
	t.Run("it reaches the target once the observation converges", func(t *testing.T) {
		wait := harnesstest.ProgressWait{Stall: time.Second, Ceiling: 5 * time.Second, Tick: 5 * time.Millisecond}
		n := 0
		observe := func() int {
			n++
			return n
		}

		start := time.Now()
		got := harnesstest.AwaitProgress(t, wait, observe, func(v int) bool { return v >= 3 })
		elapsed := time.Since(start)

		if !got.Reached {
			t.Fatalf("AwaitProgress returned %s; want Reached", got)
		}
		if got.Last != 3 {
			t.Fatalf("AwaitProgress Last = %d; want 3 (%s)", got.Last, got)
		}
		if elapsed >= wait.Stall {
			t.Fatalf("AwaitProgress took %s; want well under the stall budget %s", elapsed, wait.Stall)
		}
	})

	t.Run("it tolerates scheduler noise up to twice the ceiling", func(t *testing.T) {
		wait := harnesstest.ProgressWait{Stall: 100 * time.Millisecond, Ceiling: 900 * time.Millisecond, Tick: 5 * time.Millisecond}
		n := 0
		observe := func() int {
			n++
			return n
		}

		got := harnesstest.AwaitProgress(t, wait, observe, func(int) bool { return false })

		if got.Reached {
			t.Fatalf("AwaitProgress returned %s; want not Reached (target is unreachable)", got)
		}
		if got.Stalled {
			t.Fatalf("AwaitProgress returned %s; want Stalled false — the observation changed on every poll", got)
		}
		if got.Changes == 0 {
			t.Fatalf("AwaitProgress returned %s; want Changes counted — the observation changed on every poll", got)
		}
		if got.Elapsed < 3*wait.Stall {
			t.Fatalf("AwaitProgress gave up after %s; want it to outlive several stall budgets (%s) "+
				"because the observation kept changing (%s)", got.Elapsed, wait.Stall, got)
		}
		// Twice the ceiling, not the ceiling plus a fixed slack: this suite runs
		// on a machine with no CI, where a descheduled poll loop overshoots by a
		// margin that scales with the wait rather than by a constant. A ceiling
		// derived from the wrong budget overshoots by orders of magnitude and is
		// still caught.
		if got.Elapsed > 2*wait.Ceiling {
			t.Fatalf("AwaitProgress ran for %s, more than twice its ceiling %s; the ceiling is "+
				"not bounding the wait (%s)", got.Elapsed, wait.Ceiling, got)
		}
	})

	t.Run("it gives up inside the ceiling when the observation stops changing", func(t *testing.T) {
		wait := harnesstest.ProgressWait{Stall: 100 * time.Millisecond, Ceiling: 3 * time.Second, Tick: 5 * time.Millisecond}

		got := harnesstest.AwaitProgress(t, wait, func() int { return 7 }, func(int) bool { return false })

		if got.Reached {
			t.Fatalf("AwaitProgress returned %s; want not Reached", got)
		}
		if !got.Stalled {
			t.Fatalf("AwaitProgress returned %s; want Stalled — the observation never changed", got)
		}
		if got.Changes != 0 {
			t.Fatalf("AwaitProgress returned %s; want Changes 0 — the observation never changed", got)
		}
		if got.Elapsed < wait.Stall {
			t.Fatalf("AwaitProgress gave up after %s; want at least the stall budget %s (%s)",
				got.Elapsed, wait.Stall, got)
		}
		if got.Elapsed >= wait.Ceiling {
			t.Fatalf("AwaitProgress ran for %s; want it to fail inside the ceiling %s (%s)",
				got.Elapsed, wait.Ceiling, got)
		}
	})

	t.Run("it reports the last observation when the wait stalls", func(t *testing.T) {
		wait := harnesstest.ProgressWait{Stall: 80 * time.Millisecond, Ceiling: 2 * time.Second, Tick: 5 * time.Millisecond}
		observations := []string{"three", "two", "two"}
		i := 0
		observe := func() string {
			v := observations[min(i, len(observations)-1)]
			i++
			return v
		}

		got := harnesstest.AwaitProgress(t, wait, observe, func(v string) bool { return v == "one" })

		if got.Last != "two" {
			t.Fatalf("AwaitProgress Last = %q; want %q (%s)", got.Last, "two", got)
		}
		rendered := got.String()
		for _, want := range []string{"last=two", "reached=false", "stalled=true", "changes="} {
			if !strings.Contains(rendered, want) {
				t.Fatalf("AwaitProgress String() = %q; want it to contain %q", rendered, want)
			}
		}
	})

	t.Run("it applies defaults to a zero-value wait", func(t *testing.T) {
		got := harnesstest.AwaitProgress(t, harnesstest.ProgressWait{}, func() int { return 1 }, func(v int) bool { return v == 1 })

		if !got.Reached {
			t.Fatalf("AwaitProgress with a zero-value ProgressWait returned %s; want Reached", got)
		}
	})
}
