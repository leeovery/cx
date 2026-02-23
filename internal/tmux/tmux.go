package tmux

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// Session represents a running tmux session.
type Session struct {
	Name     string
	Windows  int
	Attached bool
}

// Commander defines the interface for executing tmux commands.
type Commander interface {
	Run(args ...string) (string, error)
}

// RealCommander executes tmux commands via os/exec.
type RealCommander struct{}

// Run executes a tmux command with the given arguments and returns its output.
func (r *RealCommander) Run(args ...string) (string, error) {
	cmd := exec.Command("tmux", args...)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Client provides tmux operations using a Commander.
type Client struct {
	cmd Commander
}

// NewClient creates a new Client with the given Commander.
func NewClient(cmd Commander) *Client {
	return &Client{cmd: cmd}
}

// ListSessions queries tmux for running sessions and returns them as structured data.
// Returns an empty slice and nil error when no tmux server is running.
func (c *Client) ListSessions() ([]Session, error) {
	output, err := c.cmd.Run("list-sessions", "-F", "#{session_name}|#{session_windows}|#{session_attached}")
	if err != nil {
		return []Session{}, nil
	}

	if output == "" {
		return []Session{}, nil
	}

	lines := strings.Split(output, "\n")
	sessions := make([]Session, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.SplitN(line, "|", 3)
		if len(parts) != 3 {
			return nil, fmt.Errorf("unexpected session format: %q", line)
		}

		windows, err := strconv.Atoi(parts[1])
		if err != nil {
			return nil, fmt.Errorf("invalid window count %q: %w", parts[1], err)
		}

		attachedCount, err := strconv.Atoi(parts[2])
		if err != nil {
			return nil, fmt.Errorf("invalid attached count %q: %w", parts[2], err)
		}

		sessions = append(sessions, Session{
			Name:     parts[0],
			Windows:  windows,
			Attached: attachedCount > 0,
		})
	}

	return sessions, nil
}
