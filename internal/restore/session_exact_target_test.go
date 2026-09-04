package restore_test

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/commandertest"
	"github.com/leeovery/portal/internal/restore"
	"github.com/leeovery/portal/internal/tmux"
)

// The skeleton addresses the session it has just created by session alone,
// leaving tmux to pick its active window. Unpinned, that target prefix-matches:
// a create that produced no session would put every split and window into a
// live stranger whose name starts the same way.
func TestSessionRestorer_SkeletonTargetsAreExactSessions(t *testing.T) {
	mock := commandertest.FromFunc(restoreRunFunc("0:0\n0:1\n1:0"))
	r := &restore.SessionRestorer{Client: tmux.NewClient(mock), StateDir: t.TempDir()}

	sess := newSession("work", nil,
		newWindow(0, "main",
			newPane(0, "/work", "scrollback/work__0.0.bin"),
			newPane(1, "/work", "scrollback/work__0.1.bin"),
		),
		newWindow(1, "logs",
			newPane(0, "/work", "scrollback/work__1.0.bin"),
		),
	)

	if _, err := r.Restore(sess); err != nil {
		t.Fatalf("Restore: %v", err)
	}

	want := tmux.CoordTargetExact("work")
	for _, cmd := range []string{"split-window", "new-window"} {
		for _, call := range callsNamed(mock.Calls(), cmd) {
			flag := slices.Index(call, "-t")
			if flag < 0 || flag+1 >= len(call) {
				t.Fatalf("%v carries no -t target", call)
			}
			if got := call[flag+1]; got != want {
				t.Errorf("%s target = %q, want %q", cmd, got, want)
			}
		}
	}

	if got := len(callsNamed(mock.Calls(), "split-window")); got != 1 {
		t.Errorf("split-window calls = %d, want 1", got)
	}
	if got := len(callsNamed(mock.Calls(), "new-window")); got != 1 {
		t.Errorf("new-window calls = %d, want 1", got)
	}
}

func callsNamed(calls [][]string, cmd string) [][]string {
	var out [][]string
	for _, call := range calls {
		if len(call) > 0 && call[0] == cmd {
			out = append(out, call)
		}
	}
	return out
}
