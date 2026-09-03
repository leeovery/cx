package commandertest_test

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/commandertest"
)

// recordingTB captures the failures the fake reports, so its own loud defaults
// can be asserted without failing the enclosing test.
type recordingTB struct {
	errors []string
	fatals []string
}

func (r *recordingTB) Helper() {}

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, fmt.Sprintf(format, args...))
}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.fatals = append(r.fatals, fmt.Sprintf(format, args...))
}

func TestScripted(t *testing.T) {
	t.Run("it returns the scripted result for a matching argv", func(t *testing.T) {
		c := commandertest.New(t,
			commandertest.Returns("first", "show-option", "-gqv", "@a"),
			commandertest.Returns("second", "show-option"),
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
		c := commandertest.Quiet()

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
		if got := c.CallsMatching("capture-pane"); len(got) != 1 {
			t.Errorf("CallsMatching(capture-pane) = %v, want exactly one", got)
		}
	})

	t.Run("it pairs each matched call with its index in the whole log", func(t *testing.T) {
		c := commandertest.Quiet()

		_, _ = c.Run("has-session", "-t", "=work")
		_, _ = c.Run("kill-session", "-t", "=work")

		want := commandertest.MatchedCalls{
			{Index: 1, Argv: []string{"kill-session", "-t", "=work"}},
		}
		if got := c.CallsMatching("kill-session"); !reflect.DeepEqual(got, want) {
			t.Errorf("CallsMatching(kill-session) = %v, want %v", got, want)
		}
		if got := c.CallsMatching("kill-session").FirstIndex(); got != 1 {
			t.Errorf("FirstIndex() = %d, want 1", got)
		}
	})

	t.Run("it narrows a query to the calls carrying every stated substring", func(t *testing.T) {
		c := commandertest.Quiet()

		_, _ = c.Run("set-hook", "-g", "session-created[0]", "run-shell")
		_, _ = c.Run("set-hook", "-gu", "session-created[0]")

		if got := c.CallsMatching("set-hook", "-gu").FirstIndex(); got != 1 {
			t.Errorf("CallsMatching(set-hook, -gu).FirstIndex() = %d, want 1", got)
		}
		if got := c.CallsMatching("set-hook", "-gu", "window-layout-changed"); len(got) != 0 {
			t.Errorf("CallsMatching with an unmet substring = %v, want none", got)
		}
	})

	t.Run("it answers -1 for the first index of a query that matched nothing", func(t *testing.T) {
		c := commandertest.Quiet()

		_, _ = c.Run("has-session", "-t", "=work")

		if got := c.CallsMatching("kill-session").FirstIndex(); got != -1 {
			t.Errorf("FirstIndex() of an unmatched query = %d, want -1", got)
		}
	})

	t.Run("it forgets the calls recorded so far when reset", func(t *testing.T) {
		c := commandertest.Quiet()

		_, _ = c.Run("has-session", "-t", "=work")
		c.ResetCalls()
		_, _ = c.Run("kill-session", "-t", "=work")

		want := [][]string{{"kill-session", "-t", "=work"}}
		if got := c.Calls(); !reflect.DeepEqual(got, want) {
			t.Errorf("Calls() after ResetCalls = %v, want %v", got, want)
		}
	})

	t.Run("it takes the stated default for an unmatched argv", func(t *testing.T) {
		t.Run("reporting the argv when no default is stated", func(t *testing.T) {
			rec := &recordingTB{}
			c := commandertest.New(rec, commandertest.Returns("v", "show-option"))

			out, err := c.Run("list-panes", "-a")
			if err == nil {
				t.Error("Run returned a nil error for an unscripted argv; want the loud default")
			}
			if out != "" {
				t.Errorf("Run = %q, want empty on the unmatched default", out)
			}
			if len(rec.errors) != 1 {
				t.Fatalf("unscripted argv reported %d failures through the TB, want 1: %v", len(rec.errors), rec.errors)
			}
			if !strings.Contains(rec.errors[0], "list-panes") {
				t.Errorf("TB report %q does not name the offending argv", rec.errors[0])
			}
		})

		t.Run("answering with the stated pair", func(t *testing.T) {
			c := commandertest.New(t, commandertest.Returns("v", "show-option")).
				AllowingUnmatched("fallback", nil)

			out, err := c.Run("list-panes", "-a")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if out != "fallback" {
				t.Errorf("Run = %q, want the stated unmatched output", out)
			}
		})

		t.Run("delegating to the inner commander", func(t *testing.T) {
			inner := commandertest.New(t, commandertest.Returns("inner-answer", "list-panes"))
			c := commandertest.New(t, commandertest.Returns("v", "show-option")).DelegatingTo(inner)

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

	t.Run("it trims Run output", func(t *testing.T) {
		c := commandertest.New(t, commandertest.Returns("  body\n\n", "capture-pane"))

		if got, _ := c.Run("capture-pane"); got != "body" {
			t.Errorf("Run = %q, want the trimmed %q", got, "body")
		}
	})

	t.Run("it returns RunRaw output verbatim", func(t *testing.T) {
		c := commandertest.New(t, commandertest.Returns("  body\n\n", "capture-pane"))

		if got, _ := c.RunRaw("capture-pane"); got != "  body\n\n" {
			t.Errorf("RunRaw = %q, want the output verbatim", got)
		}
	})

	t.Run("it fatals on RunRaw in strict mode", func(t *testing.T) {
		rec := &recordingTB{}
		c := commandertest.New(rec, commandertest.Returns("body", "list-panes")).Strict()

		if got, _ := c.Run("list-panes"); got != "body" {
			t.Errorf("Run = %q, want strict mode to leave Run alone", got)
		}
		if len(rec.fatals) != 0 {
			t.Fatalf("Run reported %v through the TB; strict mode governs RunRaw alone", rec.fatals)
		}

		out, err := c.RunRaw("list-panes")
		if len(rec.fatals) != 1 {
			t.Fatalf("RunRaw reported %d fatals, want 1: %v", len(rec.fatals), rec.fatals)
		}
		if !strings.Contains(rec.fatals[0], "list-panes") {
			t.Errorf("fatal %q does not name the offending argv", rec.fatals[0])
		}
		if out != "" || err != nil {
			t.Errorf("RunRaw = (%q, %v), want the empty pair after the fatal", out, err)
		}
	})

	t.Run("it returns the scripted error for a failing argv", func(t *testing.T) {
		boom := errors.New("exit status 1")
		c := commandertest.New(t, commandertest.Fails(boom, "has-session"))

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
		c := commandertest.New(t,
			commandertest.Returns("", "set-option").Doing(func(args []string) {
				seen = append(seen, args)
			}),
		).AllowingUnmatched("", nil)

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
		c := commandertest.New(t, commandertest.When(func(args []string) bool {
			return slices.Contains(args, "-a")
		}, "all-panes", nil))

		if got, _ := c.Run("list-panes", "-a", "-F", "#{pane_id}"); got != "all-panes" {
			t.Errorf("Run = %q, want the predicate's answer", got)
		}
	})

	t.Run("it answers a matching argv from a function", func(t *testing.T) {
		c := commandertest.New(t, commandertest.Answering(commandertest.Any, func(args ...string) (string, error) {
			return strings.Join(args, "+"), nil
		}))

		if got, _ := c.Run("list-panes", "-a"); got != "list-panes+-a" {
			t.Errorf("Run = %q, want the function's answer", got)
		}
	})

	t.Run("it answers every argv from a function", func(t *testing.T) {
		boom := errors.New("exit status 1")
		c := commandertest.FromFunc(func(args ...string) (string, error) {
			if args[0] == "has-session" {
				return "", boom
			}
			return " out \n", nil
		})

		if got, _ := c.Run("list-panes"); got != "out" {
			t.Errorf("Run = %q, want the trimmed function answer", got)
		}
		if got, _ := c.RunRaw("list-panes"); got != " out \n" {
			t.Errorf("RunRaw = %q, want the verbatim function answer", got)
		}
		if _, err := c.Run("has-session"); !errors.Is(err, boom) {
			t.Errorf("Run error = %v, want %v", err, boom)
		}
	})
}
