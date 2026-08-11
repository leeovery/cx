package tmux

import "os"

// InsideTmux reports whether Portal is running inside an existing tmux session.
func InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}
