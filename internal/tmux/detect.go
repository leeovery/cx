package tmux

import "os"

func InsideTmux() bool {
	return os.Getenv("TMUX") != ""
}
