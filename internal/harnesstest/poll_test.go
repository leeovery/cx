package harnesstest_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/harnesstest"
)

func TestPollUntil(t *testing.T) {
	t.Run("it returns true when cond becomes true before the timeout", func(t *testing.T) {
		var calls atomic.Int32
		cond := func() bool {
			// Flip on the third call so a helper that short-circuits on the
			// first iteration cannot pass.
			return calls.Add(1) >= 3
		}
		start := time.Now()
		got := harnesstest.PollUntil(t, 500*time.Millisecond, 10*time.Millisecond, cond)
		elapsed := time.Since(start)
		if !got {
			t.Fatalf("PollUntil returned false; want true (calls=%d, elapsed=%s)",
				calls.Load(), elapsed)
		}
		if elapsed >= 500*time.Millisecond {
			t.Fatalf("PollUntil took %s; want < timeout (500ms)", elapsed)
		}
	})

	t.Run("it returns false when the timeout elapses with cond never true", func(t *testing.T) {
		cond := func() bool { return false }
		start := time.Now()
		got := harnesstest.PollUntil(t, 80*time.Millisecond, 10*time.Millisecond, cond)
		elapsed := time.Since(start)
		if got {
			t.Fatalf("PollUntil returned true; want false")
		}
		if elapsed < 80*time.Millisecond {
			t.Fatalf("PollUntil returned after %s; want >= timeout (80ms)", elapsed)
		}
	})
}
