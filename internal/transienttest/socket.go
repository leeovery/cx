package transienttest

import (
	"os/exec"
	"strings"

	"github.com/leeovery/portal/internal/tmux"
)

// SocketCommander targets one tmux socket with -f /dev/null, so the developer's
// ~/.tmux.conf cannot couple a test to their environment. Errors are wrapped as
// the production commander wraps them, so *tmux.CommandError matching still works.
type SocketCommander struct {
	SocketPath string
}

func (s *SocketCommander) runArgs(args []string) []string {
	return append([]string{"-S", s.SocketPath, "-f", "/dev/null"}, args...)
}

func (s *SocketCommander) Run(args ...string) (string, error) {
	out, err := exec.Command("tmux", s.runArgs(args)...).Output()
	if err != nil {
		return "", tmux.WrapCommandError(err, args...)
	}
	return strings.TrimSpace(string(out)), nil
}

func (s *SocketCommander) RunRaw(args ...string) (string, error) {
	out, err := exec.Command("tmux", s.runArgs(args)...).Output()
	if err != nil {
		return "", tmux.WrapCommandError(err, args...)
	}
	return string(out), nil
}

var _ tmux.Commander = (*SocketCommander)(nil)
