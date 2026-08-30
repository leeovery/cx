package restoretest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/state"
)

// ANSIScrollback is the standard saved-scrollback payload: a colour escape
// around one word, followed by a plain line — both halves separately
// addressable by a caller asserting on a replay.
const ANSIScrollback = "\x1b[31mred\x1b[0m\nbefore reboot\n"

// SeedScrollback writes payload as one pane's saved scrollback, at the path a
// restore of that session will replay from.
func SeedScrollback(t *testing.T, stateDir, session string, window, pane int, payload []byte) {
	t.Helper()
	path := state.ScrollbackFile(stateDir, state.SanitizePaneKey(session, window, pane))
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir scrollback dir: %v", err)
	}
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatalf("write fixture scrollback %s: %v", path, err)
	}
}
