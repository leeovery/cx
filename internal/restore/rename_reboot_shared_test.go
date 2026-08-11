//go:build integration

package restore_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/state"
)

const (
	renamePortalID = "tok123"
	renameOldName  = "renamesrc"
	renameNewName  = "renamedst"
)

func findCapturedSession(t *testing.T, idx state.Index, name string) state.Session {
	t.Helper()
	var names []string
	for _, s := range idx.Sessions {
		if s.Name == name {
			return s
		}
		names = append(names, s.Name)
	}
	t.Fatalf("captured index has no session %q; captured names=%v", name, names)
	return state.Session{}
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

func persistIndex(t *testing.T, idx state.Index, stateDir string) {
	t.Helper()
	data, err := state.EncodeIndex(idx)
	if err != nil {
		t.Fatalf("EncodeIndex: %v", err)
	}
	if err := os.WriteFile(state.SessionsJSON(stateDir), data, 0o600); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}
}

func seedScrollback(t *testing.T, stateDir, name string) {
	t.Helper()
	scrollbackKey := state.SanitizePaneKey(name, 0, 0)
	scrollbackPath := state.ScrollbackFile(stateDir, scrollbackKey)
	if err := os.MkdirAll(filepath.Dir(scrollbackPath), 0o700); err != nil {
		t.Fatalf("mkdir scrollback dir: %v", err)
	}
	if err := os.WriteFile(scrollbackPath, []byte("\x1b[31mred\x1b[0m\nbefore reboot\n"), 0o600); err != nil {
		t.Fatalf("write fixture scrollback: %v", err)
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
