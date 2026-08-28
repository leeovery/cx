//go:build integration

package restore_test

import (
	"os"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/restoretest"
	"github.com/leeovery/portal/internal/state"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/tmuxtest"
)

const (
	renamePaneToken = "tok123"
	renameOldName   = "renamesrc"
	renameNewName   = "renamedst"

	rebootScrollback = "\x1b[31mred\x1b[0m\nbefore reboot\n"
)

// readPaneToken reports "" when the option is unset: an unset pane user-option
// makes `show-options -p -v` exit non-zero.
func readPaneToken(t *testing.T, ts *tmuxtest.Socket, sessionName string) string {
	t.Helper()
	out, err := ts.TryRun("show-options", "-p", "-t", tmux.PaneTarget(sessionName, 0, 0),
		"-v", state.PortalPaneIDOption)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(out)
}

func capturedPaneToken(t *testing.T, sess state.Session) string {
	t.Helper()
	if len(sess.Windows) == 0 || len(sess.Windows[0].Panes) == 0 {
		t.Fatalf("captured session %q has no pane 0.0: %+v", sess.Name, sess.Windows)
	}
	return sess.Windows[0].Panes[0].PortalPaneID
}

func persistIndex(t *testing.T, idx state.Index, stateDir string) {
	t.Helper()
	restoretest.WriteIndex(t, stateDir, idx)
}

func verifyHookKeyed(t *testing.T, hooksPath, wantKey string) {
	t.Helper()
	events, err := hooks.NewStore(hooksPath).Get(wantKey)
	if err != nil {
		t.Fatalf("hooks.Get(%q): %v", wantKey, err)
	}
	if _, ok := events["on-resume"]; !ok {
		t.Fatalf("hooks.json missing on-resume entry under stable key %q; got events=%v", wantKey, events)
	}
}

func assertHookFireCount(t *testing.T, hookFireFile string, want int) {
	t.Helper()
	data, err := os.ReadFile(hookFireFile)
	if err != nil {
		t.Fatalf("read hook fire file %s (bare-shell miss leaves it absent): %v", hookFireFile, err)
	}
	got := strings.Count(string(data), "HOOK_FIRED")
	if got != want {
		t.Errorf("hook fired %d times cumulatively; want exactly %d\nfile contents:\n%s", got, want, data)
	}
}
