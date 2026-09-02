package tmuxtest

import (
	"testing"
	"time"

	"github.com/leeovery/portal/internal/harnesstest"
)

// captureWait runs the wait against the shared stand-in, which absorbs the
// abort a Fatalf stands for, and reports whether it failed and with what
// message.
func captureWait(wait func(t harnesstest.TestingT)) (failed bool, msg string) {
	rec := &harnesstest.Recorder{}
	rec.Run(func() { wait(rec) })
	if len(rec.Fatals) == 0 {
		return false, ""
	}
	return true, rec.Fatals[0]
}

func TestSocket_WaitForSession(t *testing.T) {
	t.Run("it does not return for a prefix sibling of the named session", func(t *testing.T) {
		SkipIfNoTmux(t)
		s := New(t, "ptl-waitsess-")
		s.Run(t, "new-session", "-d", "-s", "sib-2")

		failed, msg := captureWait(func(stand harnesstest.TestingT) {
			s.WaitForSession(stand, "sib", 300*time.Millisecond)
		})

		if !failed {
			t.Fatal("WaitForSession returned for a live prefix sibling; want a timeout failure")
		}
		want := `session "sib" did not appear within 300ms`
		if msg != want {
			t.Fatalf("failure message = %q, want %q", msg, want)
		}
	})

	t.Run("it returns once the exact session exists", func(t *testing.T) {
		SkipIfNoTmux(t)
		s := New(t, "ptl-waitsess-")
		s.Run(t, "new-session", "-d", "-s", "sib-2")
		s.Run(t, "new-session", "-d", "-s", "sib")

		failed, msg := captureWait(func(stand harnesstest.TestingT) {
			s.WaitForSession(stand, "sib", 2*time.Second)
		})

		if failed {
			t.Fatalf("WaitForSession failed for a live session: %s", msg)
		}
	})

	t.Run("it fails at its timeout when the session never appears", func(t *testing.T) {
		SkipIfNoTmux(t)
		s := New(t, "ptl-waitsess-")
		s.Run(t, "new-session", "-d", "-s", "other")

		start := time.Now()
		failed, msg := captureWait(func(stand harnesstest.TestingT) {
			s.WaitForSession(stand, "absent", 300*time.Millisecond)
		})
		elapsed := time.Since(start)

		if !failed {
			t.Fatal("WaitForSession returned for a session that never appeared; want a timeout failure")
		}
		want := `session "absent" did not appear within 300ms`
		if msg != want {
			t.Fatalf("failure message = %q, want %q", msg, want)
		}
		if elapsed < 300*time.Millisecond {
			t.Fatalf("WaitForSession failed after %s; want it to poll for the full 300ms timeout", elapsed)
		}
	})
}
