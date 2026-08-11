package restoretest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/state"
)

// SeedSessionsJSON writes a minimal sessions.json holding one
// single-window/single-pane session per supplied name, with a zero SavedAt. The
// pane's ScrollbackFile is a placeholder path that need not exist: Restore reads
// only the index, and the in-pane hydrate helper reads the file.
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
		sessions = append(sessions, state.Session{
			Name: name,
			Windows: []state.Window{{
				Index:  0,
				Layout: "tiled",
				Active: true,
				Panes: []state.Pane{{
					Index:          0,
					Active:         true,
					ScrollbackFile: filepath.Join("scrollback", name+"-w0-p0.bin"),
				}},
			}},
		})
	}
	idx := state.Index{SavedAt: savedAt, Sessions: sessions}
	data, err := state.EncodeIndex(idx)
	if err != nil {
		t.Fatalf("EncodeIndex: %v", err)
	}
	if err := os.WriteFile(state.SessionsJSON(stateDir), data, 0o600); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}
}
