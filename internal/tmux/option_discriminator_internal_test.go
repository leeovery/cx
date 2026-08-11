package tmux

import (
	"errors"
	"slices"
	"testing"
)

type internalMockCommander struct {
	Output string
	Err    error
}

func (m *internalMockCommander) Run(args ...string) (string, error) {
	return m.Output, m.Err
}

func (m *internalMockCommander) RunRaw(args ...string) (string, error) {
	return m.Output, m.Err
}

func TestGetServerOption_DiscriminatorSet(t *testing.T) {
	for _, pat := range optionAbsentStderrPatterns {
		t.Run(pat, func(t *testing.T) {
			stderr := pat + " @foo"
			mock := &internalMockCommander{Err: &CommandError{
				Stderr: stderr,
				Err:    errors.New("exit status 1"),
			}}
			client := NewClient(mock)

			got, err := client.GetServerOption("@foo")

			if got != "" {
				t.Errorf("GetServerOption() = %q, want empty string", got)
			}
			if !errors.Is(err, ErrOptionNotFound) {
				t.Errorf("GetServerOption() error = %v, want ErrOptionNotFound", err)
			}
		})
	}

	t.Run("unrelated_stderr_does_not_match", func(t *testing.T) {
		stderr := "some unrelated error: connection refused"
		cmdErr := &CommandError{Stderr: stderr, Err: errors.New("exit status 1")}
		mock := &internalMockCommander{Err: cmdErr}
		client := NewClient(mock)

		got, err := client.GetServerOption("@foo")

		if got != "" {
			t.Errorf("GetServerOption() = %q, want empty string", got)
		}
		if err == nil {
			t.Fatal("GetServerOption() error = nil, want non-nil")
		}
		if errors.Is(err, ErrOptionNotFound) {
			t.Errorf("GetServerOption() error = %v, must not be ErrOptionNotFound", err)
		}
		var recovered *CommandError
		if !errors.As(err, &recovered) {
			t.Fatalf("errors.As did not recover *CommandError from %v (%T)", err, err)
		}
		if recovered.Stderr != stderr {
			t.Errorf("recovered Stderr = %q, want %q", recovered.Stderr, stderr)
		}
	})

	t.Run("slice_contents_pinned", func(t *testing.T) {
		want := []string{"invalid option:", "unknown option:", "ambiguous option:"}
		if len(optionAbsentStderrPatterns) != len(want) {
			t.Fatalf("optionAbsentStderrPatterns has %d entries, want %d (got %v)",
				len(optionAbsentStderrPatterns), len(want), optionAbsentStderrPatterns)
		}
		for _, w := range want {
			found := slices.Contains(optionAbsentStderrPatterns, w)
			if !found {
				t.Errorf("optionAbsentStderrPatterns missing pattern %q (got %v)",
					w, optionAbsentStderrPatterns)
			}
		}
	})
}
