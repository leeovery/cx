package tmux

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestRealCommander_RunWrapsExitError(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skipf("sh not available on PATH: %v", err)
	}

	const marker = "synthetic stderr marker"
	script := `echo "` + marker + `" 1>&2; exit 1`

	cases := []struct {
		name string
		run  func() (string, error)
	}{
		{
			name: "run",
			run: func() (string, error) {
				return runCommand("sh", true, "-c", script)
			},
		},
		{
			name: "runs_raw_variant",
			run: func() (string, error) {
				return runCommand("sh", false, "-c", script)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.run()
			if err == nil {
				t.Fatalf("expected non-nil error, got out=%q err=nil", out)
			}
			var cmdErr *CommandError
			if !errors.As(err, &cmdErr) {
				t.Fatalf("errors.As did not extract *CommandError from %v (%T)", err, err)
			}
			if !strings.Contains(cmdErr.Stderr, marker) {
				t.Errorf("CommandError.Stderr = %q, want it to contain %q", cmdErr.Stderr, marker)
			}
			if len(cmdErr.Args) != 2 || cmdErr.Args[0] != "-c" || cmdErr.Args[1] != script {
				t.Errorf("CommandError.Args = %v, want [-c %q]", cmdErr.Args, script)
			}
			rendered := err.Error()
			for _, want := range []string{"-c", "exit 1", marker} {
				if !strings.Contains(rendered, want) {
					t.Errorf("Error() = %q, want it to contain %q", rendered, want)
				}
			}
			if _, ok := errors.AsType[*exec.ExitError](cmdErr.Err); !ok {
				t.Errorf("cmdErr.Err = %v (%T); expected to unwrap to *exec.ExitError", cmdErr.Err, cmdErr.Err)
			}
		})
	}
}

func TestWrapNoSuchSession_ArgvChainRemainsRecoverable(t *testing.T) {
	cmdErr := &CommandError{
		Stderr: "no such session: missing",
		Err:    errors.New("exit status 1"),
		Args:   []string{"show-environment", "-t", "=missing"},
	}
	chain := wrapNoSuchSession(cmdErr)

	if !errors.Is(chain, ErrNoSuchSession) {
		t.Errorf("errors.Is(chain, ErrNoSuchSession) = false, want true; chain = %v", chain)
	}
	var recovered *CommandError
	if !errors.As(chain, &recovered) {
		t.Fatalf("errors.As did not recover *CommandError from %v (%T)", chain, chain)
	}
	if recovered.Stderr != "no such session: missing" {
		t.Errorf("recovered Stderr = %q, want %q", recovered.Stderr, "no such session: missing")
	}
	wantArgs := []string{"show-environment", "-t", "=missing"}
	if len(recovered.Args) != len(wantArgs) {
		t.Fatalf("recovered Args = %v, want %v", recovered.Args, wantArgs)
	}
	for i := range wantArgs {
		if recovered.Args[i] != wantArgs[i] {
			t.Fatalf("recovered Args = %v, want %v", recovered.Args, wantArgs)
		}
	}
}

func TestRunCommand_RunRawVerbatimOnSuccess(t *testing.T) {
	if _, err := exec.LookPath("printf"); err != nil {
		t.Skipf("printf not available on PATH: %v", err)
	}
	const want = "  line1\nline2  \n"
	out, err := runCommand("printf", false, "%s", want)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != want {
		t.Errorf("RunRaw output = %q, want verbatim %q", out, want)
	}
	trimmed, err := runCommand("printf", true, "%s", want)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trimmed != strings.TrimSpace(want) {
		t.Errorf("Run output = %q, want trimmed %q", trimmed, strings.TrimSpace(want))
	}
}

func TestRealCommander_RunWrapsNonExitError(t *testing.T) {
	const missing = "__portal_test_nonexistent_binary__"

	cases := []struct {
		name string
		run  func() (string, error)
	}{
		{
			name: "run",
			run: func() (string, error) {
				return runCommand(missing, true, "arg")
			},
		},
		{
			name: "runs_raw_variant",
			run: func() (string, error) {
				return runCommand(missing, false, "arg")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out, err := tc.run()
			if err == nil {
				t.Fatalf("expected non-nil error invoking %q, got out=%q err=nil", missing, out)
			}
			var cmdErr *CommandError
			if !errors.As(err, &cmdErr) {
				t.Fatalf("errors.As did not extract *CommandError from %v (%T)", err, err)
			}
			if cmdErr.Stderr != "" {
				t.Errorf("CommandError.Stderr = %q, want empty string for non-ExitError failure", cmdErr.Stderr)
			}
			if cmdErr.Err == nil {
				t.Fatal("CommandError.Err is nil; want underlying error preserved")
			}
			if _, ok := errors.AsType[*exec.ExitError](cmdErr.Err); ok {
				t.Errorf("cmdErr.Err unexpectedly unwraps to *exec.ExitError; want non-exit error type")
			}
		})
	}
}
