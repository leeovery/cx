// Package tmuxtest provides an isolated tmux-server harness. It lives outside
// _test.go so any package's tests can import it; production code must not.
package tmuxtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/harnesstest"
	"github.com/leeovery/portal/internal/tmux"
)

func socketArgs(socketPath string, args ...string) []string {
	return append([]string{"-S", socketPath, "-f", "/dev/null"}, args...)
}

// Socket scopes an isolated tmux server to a single test. The socket is an
// absolute -S path rooted in /tmp: -L under t.TempDir() would compose a path past
// darwin's ~104-byte UNIX-socket cap.
type Socket struct {
	socketPath string
}

// New constructs a Socket and registers a cleanup that kills the server and
// removes its temp dir. prefix names the temp dir; empty defaults to "ptl-".
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

// SocketPath returns the socket file's absolute path; prefer Run, TryRun or
// Client over reaching into it.
func (s *Socket) SocketPath() string { return s.socketPath }

// -f /dev/null runs against vanilla tmux defaults, not the user's ~/.tmux.conf.
func (s *Socket) cmd(args ...string) *exec.Cmd {
	return exec.Command("tmux", socketArgs(s.socketPath, args...)...)
}

// Run executes a tmux command on the isolated socket, fatalling the test on
// failure, and returns combined stdout+stderr.
func (s *Socket) Run(t *testing.T, args ...string) string {
	t.Helper()
	out, err := s.cmd(args...).CombinedOutput()
	if err != nil {
		t.Fatalf("tmux %v: %v\n%s", args, err, out)
	}
	return string(out)
}

func (s *Socket) TryRun(args ...string) (string, error) {
	out, err := s.cmd(args...).CombinedOutput()
	return string(out), err
}

// SendKeys types keys into the pane addressed by target and follows them with
// Enter, so the pane runs them as a command rather than leaving them at its
// prompt. It runs on the fixture's own socket, never the ambient server.
func (s *Socket) SendKeys(t *testing.T, target, keys string) {
	t.Helper()
	s.Run(t, "send-keys", "-t", target, keys, "Enter")
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

func (sc *socketCommander) Run(args ...string) (string, error) {
	out, err := sc.runRaw(args)
	if err != nil {
		return "", tmux.WrapCommandError(err, args...)
	}
	return strings.TrimSpace(string(out)), nil
}

func (sc *socketCommander) RunRaw(args ...string) (string, error) {
	out, err := sc.runRaw(args)
	if err != nil {
		return "", tmux.WrapCommandError(err, args...)
	}
	return string(out), nil
}

func (s *Socket) Client() *tmux.Client {
	return tmux.NewClient(&socketCommander{socketPath: s.socketPath})
}

// WaitForSession polls until the named session is queryable or timeout elapses.
// new-session looks synchronous, but a brief settle window before the session
// answers has been observed. The target is pinned the way the client pins a
// has-session target: tmux's fuzzy form lets a live prefix sibling answer for a
// session that does not exist yet, and a separator-free "=name" is split on a
// period into window and pane, so a period-bearing name would never answer.
func (s *Socket) WaitForSession(t harnesstest.TestingT, name string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		out, err := s.TryRun("has-session", "-t", tmux.CoordTargetExact(name))
		if err == nil {
			return
		}
		// has-session is noisy on stderr during the settle window.
		_ = out
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("session %q did not appear within %s", name, timeout)
}
