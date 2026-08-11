// Package tmuxtest provides an isolated tmux-server harness shared by tests
// across the codebase. It lives outside `_test.go` so any package's tests can
// import it; production code must not.
package tmuxtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmux"
)

func socketArgs(socketPath string, args ...string) []string {
	return append([]string{"-S", socketPath, "-f", "/dev/null"}, args...)
}

// Socket scopes an isolated tmux server to a single test, never touching the
// user's server. The socket is an absolute -S path rooted in /tmp: -L under
// t.TempDir() would compose a path past the platform's ~104-byte UNIX-socket cap
// on darwin.
type Socket struct {
	socketPath string
}

// New constructs a Socket and registers a cleanup that kills the server and
// removes its temp dir, so a stray tmux server is never left behind. prefix
// names the temp dir; an empty string defaults to "ptl-".
func New(t *testing.T, prefix string) *Socket {
	t.Helper()
	if prefix == "" {
		prefix = "ptl-"
	}
	dir, err := os.MkdirTemp("", prefix)
	if err != nil {
		t.Fatalf("mkdir temp socket dir: %v", err)
	}
	socketPath := filepath.Join(dir, "s")
	s := &Socket{socketPath: socketPath}
	t.Cleanup(func() {
		s.KillServer()
		_ = os.RemoveAll(dir)
	})
	return s
}

// SocketPath returns the absolute path of the socket file. Prefer Run, TryRun or
// Client over reaching into it.
func (s *Socket) SocketPath() string { return s.socketPath }

// cmd targets the isolated socket, with -f /dev/null so tests run against
// vanilla tmux defaults rather than the user's ~/.tmux.conf.
func (s *Socket) cmd(args ...string) *exec.Cmd {
	return exec.Command("tmux", socketArgs(s.socketPath, args...)...)
}

// Run executes a tmux command on the isolated socket, fatalling the test on
// failure, and returns combined stdout+stderr verbatim.
func (s *Socket) Run(t *testing.T, args ...string) string {
	t.Helper()
	out, err := s.cmd(args...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %v: %v\n%s", args, err, out)
	}
	return string(out)
}

// TryRun executes a tmux command on the isolated socket, returning the combined
// output and any error for the caller to handle.
func (s *Socket) TryRun(args ...string) (string, error) {
	out, err := s.cmd(args...).CombinedOutput()
	return string(out), err
}

// KillServer tears down the isolated tmux server. Errors are ignored: an
// already-dead server is the common case.
func (s *Socket) KillServer() {
	_, _ = s.cmd("kill-server").CombinedOutput()
}

type socketCommander struct {
	socketPath string
}

// runRaw returns the exec error unwrapped; the Run and RunRaw shims wrap it as
// production does, so callers that recover *CommandError stderr can still tell
// absence from transport failure.
func (sc *socketCommander) runRaw(args []string) ([]byte, error) {
	return exec.Command("tmux", socketArgs(sc.socketPath, args...)...).Output()
}

// Run executes tmux on the isolated socket and trims surrounding whitespace.
func (sc *socketCommander) Run(args ...string) (string, error) {
	out, err := sc.runRaw(args)
	if err != nil {
		return "", tmux.WrapCommandError(err, args...)
	}
	return strings.TrimSpace(string(out)), nil
}

// RunRaw executes tmux on the isolated socket and returns its output verbatim.
func (sc *socketCommander) RunRaw(args ...string) (string, error) {
	out, err := sc.runRaw(args)
	if err != nil {
		return "", tmux.WrapCommandError(err, args...)
	}
	return string(out), nil
}

// Client returns a *tmux.Client wired to the isolated socket commander.
func (s *Socket) Client() *tmux.Client {
	return tmux.NewClient(&socketCommander{socketPath: s.socketPath})
}

// WaitForSession polls until the named session is queryable or timeout elapses.
// new-session looks synchronous but a brief settle window has been observed
// before the session answers.
func (s *Socket) WaitForSession(t *testing.T, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := s.TryRun("has-session", "-t", name)
		if err == nil {
			return
		}
		// has-session is noisy on stderr during the settle window.
		_ = out
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("session %q did not appear within %s", name, timeout)
}
