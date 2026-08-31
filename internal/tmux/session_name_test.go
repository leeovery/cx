package tmux_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxerr"
)

const colonSession = "a:b"

func TestRenameSessionRefusesColon(t *testing.T) {
	t.Run("it refuses a rename to a name containing a colon", func(t *testing.T) {
		client := tmux.NewClient(&MockCommander{})

		err := client.RenameSession("old-name", colonSession)

		if err == nil {
			t.Fatal("RenameSession to a colon-bearing name returned nil; want a refusal")
		}
		if !errors.Is(err, tmuxerr.ErrUnaddressableSessionName) {
			t.Errorf("error = %v; want it to wrap tmuxerr.ErrUnaddressableSessionName", err)
		}
	})

	t.Run("it names the offending character in the refusal message", func(t *testing.T) {
		client := tmux.NewClient(&MockCommander{})

		err := client.RenameSession("old-name", colonSession)

		if err == nil {
			t.Fatal("expected a refusal, got nil")
		}
		if !strings.Contains(err.Error(), `":"`) {
			t.Errorf("refusal message %q does not name the offending %q character", err.Error(), ":")
		}
	})

	t.Run("it issues no tmux command for a refused rename", func(t *testing.T) {
		mock := &MockCommander{}
		client := tmux.NewClient(mock)

		_ = client.RenameSession("old-name", colonSession)

		if len(mock.Calls) != 0 {
			t.Errorf("refused rename issued %d tmux calls (%v); want none", len(mock.Calls), mock.Calls)
		}
	})

	t.Run("it renames a colon-free name unchanged", func(t *testing.T) {
		mock := &MockCommander{}
		client := tmux.NewClient(mock)

		if err := client.RenameSession("old-name", "new-name"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(mock.Calls) != 1 {
			t.Fatalf("expected 1 call, got %d: %v", len(mock.Calls), mock.Calls)
		}
		const wantArgs = "rename-session -t =old-name new-name"
		if got := strings.Join(mock.Calls[0], " "); got != wantArgs {
			t.Errorf("called with %q, want %q", got, wantArgs)
		}
	})
}

// noSuchSessionCommander answers every command the way tmux answers a target it
// cannot resolve — the same stderr for a vanished session and for a live session
// whose name the exact-target form cannot express.
func noSuchSessionCommander(target string) *MockCommander {
	return &MockCommander{
		RunFunc: func(...string) (string, error) {
			return "", &tmux.CommandError{
				Stderr: "no such session: " + target,
				Err:    errors.New("exit status 1"),
			}
		},
	}
}

func TestShowEnvironmentClassifiesUnaddressableName(t *testing.T) {
	t.Run("it classifies an unaddressable session name as anomalous rather than natural churn", func(t *testing.T) {
		client := tmux.NewClient(noSuchSessionCommander("=" + colonSession))

		_, err := client.ShowEnvironment(colonSession)

		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if errors.Is(err, tmuxerr.ErrNoSuchSession) {
			t.Errorf("error = %v; a colon-bearing name must NOT be classified as a vanished session", err)
		}
		if !errors.Is(err, tmuxerr.ErrUnaddressableSessionName) {
			t.Errorf("error = %v; want it to wrap tmuxerr.ErrUnaddressableSessionName", err)
		}
	})

	t.Run("it still treats a genuinely vanished session as natural churn", func(t *testing.T) {
		client := tmux.NewClient(noSuchSessionCommander("=gone"))

		_, err := client.ShowEnvironment("gone")

		if err == nil {
			t.Fatal("expected an error, got nil")
		}
		if !errors.Is(err, tmuxerr.ErrNoSuchSession) {
			t.Errorf("error = %v; want it to wrap tmuxerr.ErrNoSuchSession", err)
		}
		if errors.Is(err, tmuxerr.ErrUnaddressableSessionName) {
			t.Errorf("error = %v; a colon-free name is addressable", err)
		}
	})
}
