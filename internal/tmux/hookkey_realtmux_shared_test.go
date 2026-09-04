package tmux_test

import (
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

// hookKeyFixtureSocketPrefix names the temp dir of every hook-key fixture's
// socket, so a stray server left behind by any of these suites is recognisable.
const hookKeyFixtureSocketPrefix = "ptl-hookkey-"

// sessionSettleTimeout bounds the window a freshly created session has been
// observed to need before it answers.
const sessionSettleTimeout = 2 * time.Second

// realTmuxFixture describes the isolated server a real-tmux suite works
// against: the temp-dir prefix its socket is named with, the sessions to
// create, and the topology to shape them into once they answer.
type realTmuxFixture struct {
	socketPrefix string
	sessions     []string
	topology     func(t *testing.T, ts *tmuxtest.Socket, client *tmux.Client, dir string)
}

// seedRealTmuxServer starts the fixture's server, creates its sessions in one
// temp dir, and returns the socket, its client, and that dir as tmux reports it
// (macOS resolves the temp dir through /private, so an unresolved path would
// not compare equal to tmux's own answer).
func seedRealTmuxServer(t *testing.T, fixture realTmuxFixture) (*tmuxtest.Socket, *tmux.Client, string) {
	t.Helper()
	tmuxtest.SkipIfNoTmux(t)

	ts := tmuxtest.New(t, fixture.socketPrefix)
	client := ts.Client()
	if _, err := client.EnsureServer(); err != nil {
		t.Fatalf("EnsureServer: %v", err)
	}
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("EvalSymlinks: %v", err)
	}
	for _, name := range fixture.sessions {
		if err := client.NewSession(name, dir, ""); err != nil {
			t.Fatalf("NewSession(%q): %v", name, err)
		}
		ts.WaitForSession(t, name, sessionSettleTimeout)
	}
	if fixture.topology != nil {
		fixture.topology(t, ts, client, dir)
	}
	return ts, client, dir
}

// hookKeyFixture describes an isolated server holding one hook-key session,
// shaped by topology once it answers.
func hookKeyFixture(sessionName string, topology func(t *testing.T, ts *tmuxtest.Socket, client *tmux.Client, dir string)) realTmuxFixture {
	return realTmuxFixture{
		socketPrefix: hookKeyFixtureSocketPrefix,
		sessions:     []string{sessionName},
		topology:     topology,
	}
}

// seedThreePaneStampedSession builds a three-pane session and stamps the token
// onto the first pane only, so callers have a stamped pane and two unstamped
// siblings to tell apart.
func seedThreePaneStampedSession(t *testing.T, sessionName, token string) (*tmuxtest.Socket, *tmux.Client, []string) {
	t.Helper()
	ts, client, _ := seedRealTmuxServer(t, hookKeyFixture(sessionName,
		func(t *testing.T, _ *tmuxtest.Socket, client *tmux.Client, _ string) {
			if err := client.SplitWindow(sessionName+":0", "", ""); err != nil {
				t.Fatalf("SplitWindow(%q): %v", sessionName+":0", err)
			}
			if err := client.NewWindow(sessionName, "", "", ""); err != nil {
				t.Fatalf("NewWindow(%q): %v", sessionName, err)
			}
		}))
	paneIDs := sessionPaneIDs(t, ts, sessionName)
	if len(paneIDs) == 0 {
		t.Fatalf("session %q reported no panes", sessionName)
	}
	ts.StampPaneToken(t, paneIDs[0], token)
	return ts, client, paneIDs
}

func sessionPaneIDs(t *testing.T, ts *tmuxtest.Socket, session string) []string {
	t.Helper()
	out := ts.Run(t, "list-panes", "-s", "-t", session, "-F", "#{pane_id}")
	var ids []string
	for line := range strings.SplitSeq(strings.TrimSpace(out), "\n") {
		if id := strings.TrimSpace(line); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// livePanePID reads the session's own pane pid straight from tmux, so an
// assertion compares against the wire truth rather than a second Portal read.
func livePanePID(t *testing.T, ts *tmuxtest.Socket, session string) int {
	t.Helper()
	out := strings.TrimSpace(ts.Run(t, "list-panes", "-t", tmux.CoordTargetExact(session), "-F", "#{pane_pid}"))
	pid, err := strconv.Atoi(strings.SplitN(out, "\n", 2)[0])
	if err != nil {
		t.Fatalf("parse pane pid %q: %v", out, err)
	}
	return pid
}
