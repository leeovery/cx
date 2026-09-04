package tmux

import (
	"fmt"
	"strconv"
	"strings"
)

type ClientInfo struct {
	PID int
	// Activity is tmux's #{client_activity}, in epoch seconds.
	Activity int64
}

// ListClients enumerates the tmux clients attached to the named session. A
// failing invocation yields an empty slice and a nil error — it is the no-server
// / no-clients signal — and only a malformed line returns an error.
func (c *Client) ListClients(session string) ([]ClientInfo, error) {
	output, err := c.cmd.Run("list-clients", "-t", SessionTargetExact(session), "-F", "#{client_pid} #{client_activity}")
	if err != nil {
		// Swallowed deliberately: the error is the zero-clients signal.
		return []ClientInfo{}, nil
	}

	if output == "" {
		return []ClientInfo{}, nil
	}

	lines := strings.Split(output, "\n")
	clients := make([]ClientInfo, 0, len(lines))

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) != 2 {
			return nil, fmt.Errorf("unexpected client format: %q", line)
		}

		pid, err := strconv.Atoi(fields[0])
		if err != nil {
			return nil, fmt.Errorf("invalid client pid %q: %w", fields[0], err)
		}

		activity, err := strconv.ParseInt(fields[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid client activity %q: %w", fields[1], err)
		}

		clients = append(clients, ClientInfo{PID: pid, Activity: activity})
	}

	return clients, nil
}
