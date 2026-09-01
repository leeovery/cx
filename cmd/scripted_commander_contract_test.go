package cmd

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
)

// recordingTB captures the failure an unmatched argv reports, so the fake's own
// loud default can be asserted without failing the enclosing test.
type recordingTB struct {
	testing.TB
	reported []string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.reported = append(r.reported, fmt.Sprintf(format, args...))
}

func TestScriptedCommander(t *testing.T) {
	t.Run("it returns the scripted result for a matching argv", func(t *testing.T) {
		c := newScriptedCommander(t,
			returns("first", "show-option", "-gqv", "@a"),
			returns("second", "show-option"),
		)

		got, err := c.Run("show-option", "-gqv", "@a")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "first" {
			t.Errorf("Run = %q, want %q (first matching entry wins)", got, "first")
		}

		got, err = c.Run("show-option", "-gqv", "@b")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if got != "second" {
			t.Errorf("Run = %q, want %q", got, "second")
		}
	})

	t.Run("it records every call in order", func(t *testing.T) {
		c := quietCommander()

		_, _ = c.Run("has-session", "-t", "=work")
		_, _ = c.RunRaw("capture-pane", "-p")
		_, _ = c.Run("kill-session", "-t", "=work")

		want := [][]string{
			{"has-session", "-t", "=work"},
			{"capture-pane", "-p"},
			{"kill-session", "-t", "=work"},
		}
		if got := c.Calls(); !reflect.DeepEqual(got, want) {
			t.Errorf("Calls() = %v, want %v", got, want)
		}
		if got := c.callsMatching("capture-pane"); len(got) != 1 {
			t.Errorf("callsMatching(capture-pane) = %v, want exactly one", got)
		}
	})

	t.Run("it takes the stated default for an unmatched argv", func(t *testing.T) {
		t.Run("reporting the argv when no default is stated", func(t *testing.T) {
			rec := &recordingTB{}
			c := newScriptedCommander(rec, returns("v", "show-option"))

			out, err := c.Run("list-panes", "-a")
			if err == nil {
				t.Error("Run returned a nil error for an unscripted argv; want the loud default")
			}
			if out != "" {
				t.Errorf("Run = %q, want empty on the unmatched default", out)
			}
			if len(rec.reported) != 1 {
				t.Fatalf("unscripted argv reported %d failures through the TB, want 1: %v", len(rec.reported), rec.reported)
			}
			if !strings.Contains(rec.reported[0], "list-panes") {
				t.Errorf("TB report %q does not name the offending argv", rec.reported[0])
			}
		})

		t.Run("answering with the stated pair", func(t *testing.T) {
			c := newScriptedCommander(t, returns("v", "show-option")).
				allowingUnmatched("fallback", nil)

			out, err := c.Run("list-panes", "-a")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != "fallback" {
				t.Errorf("Run = %q, want the stated unmatched output", out)
			}
		})

		t.Run("delegating to the inner commander", func(t *testing.T) {
			inner := newScriptedCommander(t, returns("inner-answer", "list-panes"))
			c := newScriptedCommander(t, returns("v", "show-option")).delegatingTo(inner)

			out, err := c.Run("list-panes", "-a")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != "inner-answer" {
				t.Errorf("Run = %q, want the delegate's answer", out)
			}
			if len(inner.Calls()) != 1 {
				t.Errorf("delegate recorded %d calls, want 1", len(inner.Calls()))
			}
		})
	})

	t.Run("it trims Run output and leaves RunRaw verbatim", func(t *testing.T) {
		c := newScriptedCommander(t, returns("  body\n\n", "capture-pane"))

		if got, _ := c.Run("capture-pane"); got != "body" {
			t.Errorf("Run = %q, want the trimmed %q", got, "body")
		}
		if got, _ := c.RunRaw("capture-pane"); got != "  body\n\n" {
			t.Errorf("RunRaw = %q, want the output verbatim", got)
		}
	})

	t.Run("it returns the scripted error for a failing argv", func(t *testing.T) {
		boom := errors.New("exit status 1")
		c := newScriptedCommander(t, fails(boom, "has-session"))

		out, err := c.Run("has-session", "-t", "=gone")
		if !errors.Is(err, boom) {
			t.Errorf("Run error = %v, want %v", err, boom)
		}
		if out != "" {
			t.Errorf("Run = %q, want empty alongside an error", out)
		}
		if _, err := c.RunRaw("has-session", "-t", "=gone"); !errors.Is(err, boom) {
			t.Errorf("RunRaw error = %v, want %v", err, boom)
		}
	})

	t.Run("it runs an entry's side effect when that entry matches", func(t *testing.T) {
		var seen [][]string
		c := newScriptedCommander(t,
			returns("", "set-option").doing(func(args []string) {
				seen = append(seen, args)
			}),
		).allowingUnmatched("", nil)

		_, _ = c.Run("set-option", "-su", "@portal-skeleton-x")
		_, _ = c.Run("list-panes")

		if len(seen) != 1 {
			t.Fatalf("side effect ran %d times, want once", len(seen))
		}
		if !reflect.DeepEqual(seen[0], []string{"set-option", "-su", "@portal-skeleton-x"}) {
			t.Errorf("side effect saw %v", seen[0])
		}
	})

	t.Run("it matches on a predicate that is not an argv prefix", func(t *testing.T) {
		c := newScriptedCommander(t, when(func(args []string) bool {
			return slices.Contains(args, "-a")
		}, "all-panes", nil))

		if got, _ := c.Run("list-panes", "-a", "-F", "#{pane_id}"); got != "all-panes" {
			t.Errorf("Run = %q, want the predicate's answer", got)
		}
	})
}
