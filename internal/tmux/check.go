package tmux

import (
	"errors"
	"os/exec"
)

func CheckTmuxAvailable() error {
	_, err := exec.LookPath("tmux")
	if err != nil {
		return errors.New("Portal requires tmux. Install with: brew install tmux") //nolint:staticcheck // user-facing message requires capitalization per spec
	}
	return nil
}
