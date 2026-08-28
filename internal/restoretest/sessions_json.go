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
	WriteIndex(t, stateDir, state.Index{SavedAt: savedAt, Sessions: sessions})
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
	WriteIndex(t, stateDir, state.Index{Sessions: sessions})
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

// WriteIndex persists idx as sessions.json. It takes the whole index rather
// than its fields so a caller round-tripping a captured one cannot drop a field
// on the way through; EncodeIndex canonicalizes, so a zero Version is filled in.
func WriteIndex(t *testing.T, stateDir string, idx state.Index) {
	t.Helper()
	data, err := state.EncodeIndex(idx)
	if err != nil {
		t.Fatalf("EncodeIndex: %v", err)
	}
	if err := os.WriteFile(state.SessionsJSON(stateDir), data, 0o600); err != nil {
		t.Fatalf("write sessions.json: %v", err)
	}
}

// FindCapturedSession fails the test naming every session the index does hold,
// so a miss reads as "captured the wrong thing" rather than a bare index error.
func FindCapturedSession(t *testing.T, idx state.Index, name string) state.Session {
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
