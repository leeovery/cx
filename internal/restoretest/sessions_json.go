package restoretest

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
)

// SeedSessionsJSON writes one single-window/single-pane session per name, with
// a zero SavedAt. The pane's ScrollbackFile is a placeholder that need not
// exist: Restore reads only the index, and the hydrate helper reads the file.
func SeedSessionsJSON(t *testing.T, stateDir string, names ...string) {
	t.Helper()
	SeedSessionsJSONWithSavedAt(t, stateDir, time.Time{}, names...)
}

// SeedSessionsJSONWithSavedAt encodes savedAt verbatim, so a caller can capture
// it before a run and assert nothing in the pipeline advanced it.
func SeedSessionsJSONWithSavedAt(t *testing.T, stateDir string, savedAt time.Time, names ...string) {
	t.Helper()
	sessions := make([]state.Session, 0, len(names))
	for _, name := range names {
		sessions = append(sessions, singlePaneSession(name, ""))
	}
	writeIndex(t, stateDir, savedAt, sessions)
}

// SeedSessionsJSONWithPaneTokens writes one single-window/single-pane session
// per map entry, each pane carrying the entry's durable identity token, so a
// restore bakes that token as the pane's hook key. Sessions are ordered by name.
func SeedSessionsJSONWithPaneTokens(t *testing.T, stateDir string, tokens map[string]string) {
	t.Helper()
	names := make([]string, 0, len(tokens))
	for name := range tokens {
		names = append(names, name)
	}
	slices.Sort(names)

	sessions := make([]state.Session, 0, len(names))
	for _, name := range names {
		sessions = append(sessions, singlePaneSession(name, tokens[name]))
	}
	writeIndex(t, stateDir, time.Time{}, sessions)
}

func singlePaneSession(name, paneToken string) state.Session {
	return state.Session{
		Name: name,
		Windows: []state.Window{{
			Index:  0,
			Layout: "tiled",
			Active: true,
			Panes: []state.Pane{{
				Index:          0,
				Active:         true,
				ScrollbackFile: filepath.Join("scrollback", name+"-w0-p0.bin"),
				PortalPaneID:   paneToken,
			}},
		}},
	}
}

func writeIndex(t *testing.T, stateDir string, savedAt time.Time, sessions []state.Session) {
	t.Helper()
	idx := state.Index{SavedAt: savedAt, Sessions: sessions}
	data, err := state.EncodeIndex(idx)
	if err != nil {
		t.Fatalf("EncodeIndex: %v", err)
	}
	if err := os.WriteFile(state.SessionsJSON(stateDir), data, 0o600); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}
}
