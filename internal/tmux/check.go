// Package tmux provides tmux integration for Portal.
package tmux

import (
	"errors"
	"os/exec"
)

// CheckTmuxAvailable reports whether tmux is on PATH, returning an error
// carrying install instructions when it is not.
func CheckTmuxAvailable() error {
	_, err := exec.LookPath("tmux")
	if err != nil {
		return errors.New("Portal requires tmux. Install with: brew install tmux") //nolint:staticcheck // user-facing message requires capitalization per spec
	}
	return nil
}
