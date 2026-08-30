package tmux

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestWrapCommandError(t *testing.T) {
	t.Run("nil_input_returns_nil", func(t *testing.T) {
		if got := WrapCommandError(nil); got != nil {
			t.Errorf("WrapCommandError(nil) = %v, want nil", got)
		}
	})

	t.Run("exec_exit_error_populates_stderr", func(t *testing.T) {
		if _, err := exec.LookPath("sh"); err != nil {
			t.Skipf("sh not available on PATH: %v", err)
		}
		const marker = "wrap-helper stderr marker"
		cmd := exec.Command("sh", "-c", `echo "`+marker+`" 1>&2; exit 1`)
		// cmd.Stderr is deliberately left nil: only then does exec populate
		// ExitError.Stderr with the child's bytes.
		_, runErr := cmd.Output()
		if runErr == nil {
			t.Fatal("expected non-nil error from sh exit 1")
		}

		wrapped := WrapCommandError(runErr)
		if wrapped == nil {
			t.Fatal("WrapCommandError returned nil for non-nil exec error")
		}
		var cmdErr *CommandError
		if !errors.As(wrapped, &cmdErr) {
			t.Fatalf("errors.As did not extract *CommandError from %v (%T)", wrapped, wrapped)
		}
		if cmdErr.Stderr == "" {
			t.Errorf("CommandError.Stderr is empty; want bytes containing %q", marker)
		}
		if _, ok := errors.AsType[*exec.ExitError](cmdErr.Err); !ok {
			t.Errorf("cmdErr.Err = %v (%T); expected to unwrap to *exec.ExitError", cmdErr.Err, cmdErr.Err)
		}
	})

	t.Run("non_exec_error_empty_stderr", func(t *testing.T) {
		sentinel := errors.New("plain non-exec error")

		wrapped := WrapCommandError(sentinel)
		if wrapped == nil {
			t.Fatal("WrapCommandError returned nil for non-nil non-exec error")
		}
		var cmdErr *CommandError
		if !errors.As(wrapped, &cmdErr) {
			t.Fatalf("errors.As did not extract *CommandError from %v (%T)", wrapped, wrapped)
		}
		if cmdErr.Stderr != "" {
			t.Errorf("CommandError.Stderr = %q, want empty string for non-exec error", cmdErr.Stderr)
		}
		if !errors.Is(cmdErr.Err, sentinel) {
			t.Errorf("cmdErr.Err = %v; want original sentinel preserved", cmdErr.Err)
		}
	})

	t.Run("variadic_args_populate_CommandError_Args", func(t *testing.T) {
		sentinel := errors.New("boom")
		wrapped := WrapCommandError(sentinel, "list-panes", "-t", "=missing")

		var cmdErr *CommandError
		if !errors.As(wrapped, &cmdErr) {
			t.Fatalf("errors.As did not extract *CommandError from %v (%T)", wrapped, wrapped)
		}
		want := []string{"list-panes", "-t", "=missing"}
		if len(cmdErr.Args) != len(want) {
			t.Fatalf("Args = %v, want %v", cmdErr.Args, want)
		}
		for i := range want {
			if cmdErr.Args[i] != want[i] {
				t.Fatalf("Args = %v, want %v", cmdErr.Args, want)
			}
		}
	})

	t.Run("nil_input_returns_nil_even_with_args", func(t *testing.T) {
		if got := WrapCommandError(nil, "list-panes", "-t", "x"); got != nil {
			t.Errorf("WrapCommandError(nil, args...) = %v, want nil", got)
		}
	})

	t.Run("no_args_leaves_Args_nil_for_legacy_literal_parity", func(t *testing.T) {
		wrapped := WrapCommandError(errors.New("boom"))
		var cmdErr *CommandError
		if !errors.As(wrapped, &cmdErr) {
			t.Fatalf("errors.As did not extract *CommandError")
		}
		if cmdErr.Args != nil {
			t.Errorf("Args = %v, want nil for argv-less wrap", cmdErr.Args)
		}
	})
}

func TestCommandError_ErrorRendering_WithArgs(t *testing.T) {
	t.Run("exit_error_renders_argv_exit_code_and_trimmed_stderr", func(t *testing.T) {
		if _, err := exec.LookPath("sh"); err != nil {
			t.Skipf("sh not available on PATH: %v", err)
		}
		cmd := exec.Command("sh", "-c", `echo "  nope  " 1>&2; exit 2`)
		// cmd.Stderr is deliberately left nil: only then does exec populate
		// ExitError.Stderr with the child's bytes.
		_, runErr := cmd.Output()
		if runErr == nil {
			t.Fatal("expected non-nil error from sh exit 2")
		}
		wrapped := WrapCommandError(runErr, "kill-session", "-t", "=foo")

		got := wrapped.Error()
		for _, want := range []string{"tmux", "kill-session", "-t", "=foo", "exit 2", "nope"} {
			if !strings.Contains(got, want) {
				t.Errorf("Error() = %q, want it to contain %q", got, want)
			}
		}
		if strings.Contains(got, "  nope  ") {
			t.Errorf("Error() = %q, expected trimmed stderr (no surrounding spaces)", got)
		}
	})

	t.Run("path_lookup_error_renders_argv_with_no_exit_fragment_and_empty_stderr", func(t *testing.T) {
		cmd := exec.Command("__portal_test_nonexistent_binary__", "arg")
		_, runErr := cmd.Output()
		if runErr == nil {
			t.Fatal("expected non-nil error invoking nonexistent binary")
		}
		if _, ok := errors.AsType[*exec.ExitError](runErr); ok {
			t.Fatalf("expected non-ExitError failure, got *exec.ExitError")
		}
		wrapped := WrapCommandError(runErr, "list-sessions")

		got := wrapped.Error()
		if !strings.Contains(got, "list-sessions") {
			t.Errorf("Error() = %q, want it to contain argv 'list-sessions'", got)
		}
		if strings.Contains(got, "exit ") {
			t.Errorf("Error() = %q, want no 'exit N' fragment for *exec.Error", got)
		}
	})

	t.Run("argv_with_spaces_and_quotes_renders_intact", func(t *testing.T) {
		ce := &CommandError{
			Args:   []string{"send-keys", `echo "hello world"`, ";"},
			Err:    errors.New("exit status 1"),
			Stderr: "boom",
		}
		got := ce.Error()
		if !strings.Contains(got, `echo "hello world"`) {
			t.Errorf("Error() = %q, want it to contain the quoted/spaced token intact", got)
		}
		if !strings.Contains(got, ";") {
			t.Errorf("Error() = %q, want it to contain the ';' metacharacter token", got)
		}
	})

	t.Run("empty_args_falls_back_to_legacy_rendering", func(t *testing.T) {
		ce := &CommandError{Err: errors.New("exit status 1"), Stderr: "  boom  "}
		if got, want := ce.Error(), "exit status 1: boom"; got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	})

	t.Run("args_present_but_empty_stderr_omits_trailing_stderr_fragment", func(t *testing.T) {
		ce := &CommandError{Args: []string{"list-panes"}, Err: errors.New("exit status 1")}
		got := ce.Error()
		if !strings.Contains(got, "tmux list-panes") {
			t.Errorf("Error() = %q, want it to contain 'tmux list-panes'", got)
		}
		if strings.HasSuffix(got, ": ") || strings.HasSuffix(got, ":") {
			t.Errorf("Error() = %q, want no dangling trailing separator", got)
		}
	})
}
