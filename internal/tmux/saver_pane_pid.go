package tmux

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Unexported deliberately: SaverPanePIDOrAbsent's "any error means absent"
// collapse must stay the only path out of the package.
func saverPanePID(c *Client, sessionName string) (int, error) {
	out, err := c.cmd.Run("list-panes", "-t", exactTarget(sessionName), "-F", "#{pane_pid}")
	if err != nil {
		wrapped := wrapNoSuchSession(err)
		return 0, fmt.Errorf("list-panes -t %s: %w", sessionName, wrapped)
	}

	for line := range strings.SplitSeq(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		pid, parseErr := strconv.Atoi(line)
		if parseErr != nil {
			return 0, fmt.Errorf("list-panes -t %s: parse pane_pid %q: %w: %w",
				sessionName, line, ErrPanePIDParse, parseErr)
		}
		return pid, nil
	}
	return 0, fmt.Errorf("list-panes -t %s: %w", sessionName, ErrEmptyPaneList)
}

// SaverPaneID returns the tmux pane id (e.g. "%42") of the first pane in the
// named session, or ErrEmptyPaneList when the session lists no pane.
func (c *Client) SaverPaneID(sessionName string) (string, error) {
	out, err := c.cmd.Run("list-panes", "-t", exactTarget(sessionName), "-F", "#{pane_id}")
	if err != nil {
		return "", fmt.Errorf("list-panes -t %s -F #{pane_id}: %w", sessionName, err)
	}
	for line := range strings.SplitSeq(out, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return line, nil
		}
	}
	return "", fmt.Errorf("list-panes -t %s -F #{pane_id}: %w", sessionName, ErrEmptyPaneList)
}

// SaverPanePIDOrAbsent returns the pid of the named session's first pane.
// A missing session and an empty pane list are both legitimate absences and
// collapse to (0, false, nil); any other failure returns (0, false, err) for
// the caller to apply its own policy to.
func SaverPanePIDOrAbsent(c *Client, sessionName string) (pid int, present bool, err error) {
	pid, err = saverPanePID(c, sessionName)
	if err != nil {
		if errors.Is(err, ErrNoSuchSession) || errors.Is(err, ErrEmptyPaneList) {
			return 0, false, nil
		}
		return 0, false, err
	}
	return pid, true, nil
}
