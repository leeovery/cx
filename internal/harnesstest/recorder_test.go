package harnesstest_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/harnesstest"
)

// fatalThenErrorf models a helper that aborts: a real Fatalf never returns, so
// the Errorf after it must not be reached either.
func fatalThenErrorf(t harnesstest.TestingT) {
	t.Helper()
	t.Fatalf("read %s: %v", "hooks.json", "is a directory")
	t.Errorf("unreachable")
}

func TestRecorder(t *testing.T) {
	t.Run("it records the fatal message and stops the helper", func(t *testing.T) {
		rec := &harnesstest.Recorder{}

		rec.Run(func() { fatalThenErrorf(rec) })

		if len(rec.Fatals) != 1 {
			t.Fatalf("got %d fatals, want exactly 1: %v", len(rec.Fatals), rec.Fatals)
		}
		if want := "read hooks.json: is a directory"; rec.Fatals[0] != want {
			t.Errorf("fatal message = %q, want %q", rec.Fatals[0], want)
		}
		if len(rec.Errors) != 0 {
			t.Errorf("the helper carried on past its Fatalf and reported %v", rec.Errors)
		}
		if !rec.Failed() {
			t.Error("Failed() = false after a fatal")
		}
		if rec.HelperCalls != 1 {
			t.Errorf("HelperCalls = %d, want 1", rec.HelperCalls)
		}
	})

	t.Run("it records an Errorf without stopping", func(t *testing.T) {
		rec := &harnesstest.Recorder{}

		rec.Run(func() {
			rec.Errorf("first %d", 1)
			rec.Errorf("second %d", 2)
		})

		if want := []string{"first 1", "second 2"}; len(rec.Errors) != 2 ||
			rec.Errors[0] != want[0] || rec.Errors[1] != want[1] {
			t.Errorf("errors = %v, want %v", rec.Errors, want)
		}
		if len(rec.Fatals) != 0 {
			t.Errorf("an Errorf recorded a fatal: %v", rec.Fatals)
		}
		if !rec.Failed() {
			t.Error("Failed() = false after an Errorf")
		}
	})

	t.Run("it reports no failure for a passing helper", func(t *testing.T) {
		rec := &harnesstest.Recorder{}

		rec.Run(func() { rec.Helper() })

		if rec.Failed() {
			t.Errorf("Failed() = true for a helper that reported nothing: %s", rec.Report())
		}
	})

	t.Run("it re-raises a panic that is not a fatal", func(t *testing.T) {
		rec := &harnesstest.Recorder{}

		defer func() {
			raised := recover()
			if raised == nil {
				t.Fatal("a crashing helper was absorbed and would read as one that did not fail")
			}
			if msg, ok := raised.(string); !ok || msg != "boom" {
				t.Errorf("re-raised %v, want the original panic value", raised)
			}
		}()

		rec.Run(func() { panic("boom") })
	})

	t.Run("it reports what it recorded", func(t *testing.T) {
		rec := &harnesstest.Recorder{}

		rec.Run(func() {
			rec.Errorf("an error")
			rec.Fatalf("a fatal")
		})

		report := rec.Report()
		if !strings.Contains(report, "an error") || !strings.Contains(report, "a fatal") {
			t.Errorf("Report() = %q, want it to carry both what was recorded", report)
		}
	})

	t.Run("it names itself for a helper that puts the test name in a diagnostic", func(t *testing.T) {
		var rec harnesstest.NamingT = &harnesstest.Recorder{}

		if rec.Name() == "" {
			t.Error("Name() = \"\", want a name a diagnostic can carry")
		}
	})

	t.Run("it renders the fatal message when a Fatalf escapes Run", func(t *testing.T) {
		rec := &harnesstest.Recorder{}

		defer func() {
			raised := recover()
			if raised == nil {
				t.Fatal("a Fatalf outside Run did not panic")
			}
			rendered, ok := raised.(fmt.Stringer)
			if !ok {
				t.Fatalf("panic value %#v does not render itself; a suite would see it with no message", raised)
			}
			if !strings.Contains(rendered.String(), "read hooks.json") {
				t.Errorf("panic renders as %q, want it to carry the fatal message", rendered.String())
			}
		}()

		rec.Fatalf("read %s", "hooks.json")
	})
}
