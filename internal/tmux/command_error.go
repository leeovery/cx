package tmux

import (
	"errors"
	"os/exec"
	"strconv"
	"strings"
)

// CommandError wraps an error returned by Commander.Run / Commander.RunRaw and
// carries the child process's captured stderr, which is empty when none was
// captured (e.g. a PATH-lookup *exec.Error).
type CommandError struct {
	Stderr string
	Err    error
	// Args is the child argv — the tmux subcommand and its flags, not the
	// binary name. Nil is a supported zero value: Error() then omits the argv.
	Args []string
}

// Error renders the wrapped error in a human-readable form. The rendered format
// is not part of the contract — discriminate on Stderr / Args, or recover the
// typed value with errors.As, rather than parsing this string.
func (e *CommandError) Error() string {
	trimmed := strings.TrimSpace(e.Stderr)
	if len(e.Args) > 0 {
		return e.renderWithArgs(trimmed)
	}
	if e.Err == nil {
		if trimmed == "" {
			return "<no error>"
		}
		return trimmed
	}
	if trimmed == "" {
		return e.Err.Error()
	}
	return e.Err.Error() + ": " + trimmed
}

func (e *CommandError) renderWithArgs(trimmedStderr string) string {
	var b strings.Builder
	b.WriteString("tmux ")
	b.WriteString(strings.Join(e.Args, " "))
	if exitErr, ok := errors.AsType[*exec.ExitError](e.Err); ok {
		b.WriteString(": exit ")
		b.WriteString(strconv.Itoa(exitErr.ExitCode()))
	}
	if trimmedStderr != "" {
		b.WriteString(": ")
		b.WriteString(trimmedStderr)
	}
	return b.String()
}

func (e *CommandError) Unwrap() error {
	return e.Err
}

// WrapCommandError returns nil for a nil error, so it can be applied
// unconditionally to an exec result.
//
// Precondition: the *exec.Cmd must have left cmd.Stderr == nil. exec populates
// (*exec.ExitError).Stderr only under that condition, so assigning cmd.Stderr
// silently zeroes it and defeats the wrap; capture stderr explicitly and build
// the *CommandError directly instead.
func WrapCommandError(err error, args ...string) error {
	if err == nil {
		return nil
	}
	var stderr string
	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok {
		stderr = string(exitErr.Stderr)
	}
	return &CommandError{Stderr: stderr, Err: err, Args: args}
}
