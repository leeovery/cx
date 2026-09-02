package tmuxtest

import (
	"fmt"
	"testing"
	"time"
)

// fakeT stands in for *testing.T so WaitForSession's timeout path can be
// exercised without aborting the harness. Fatalf panics, as testing's own does
// not return, and captureWait absorbs it.
type fakeT struct {
	failed bool
	msg    string
}

func (f *fakeT) Helper() {}

func (f *fakeT) Fatalf(format string, args ...any) {
	f.failed = true
	f.msg = fmt.Sprintf(format, args...)
	panic(fakeFatal{})
}

type fakeFatal struct{}

// captureWait runs the wait against the stand-in, reporting whether it failed
// and with what message.
func captureWait(wait func(t TestingT)) (failed bool, msg string) {
	stand := &fakeT{}
	defer func() {
		_ = recover()
		failed, msg = stand.failed, stand.msg
	}()
	wait(stand)
	return stand.failed, stand.msg
}

func TestSocket_WaitForSession(t *testing.T) {
	t.Run("it does not return for a prefix sibling of the named session", func(t *testing.T) {
		SkipIfNoTmux(t)
		s := New(t, "ptl-waitsess-")
		s.Run(t, "new-session", "-d", "-s", "sib-2")

		failed, msg := captureWait(func(stand TestingT) {
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

		failed, msg := captureWait(func(stand TestingT) {
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
		failed, msg := captureWait(func(stand TestingT) {
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
