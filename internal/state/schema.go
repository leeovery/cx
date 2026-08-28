package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// SchemaVersion is bumped on any schema-breaking change.
const SchemaVersion = 1

// Index is the root document persisted to sessions.json.
type Index struct {
	Version  int       `json:"version"`
	SavedAt  time.Time `json:"saved_at"`
	Sessions []Session `json:"sessions"`
}

type Session struct {
	Name        string            `json:"name"`
	Environment map[string]string `json:"environment"`
	Windows     []Window          `json:"windows"`
}

type Window struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Layout string `json:"layout"`
	Zoomed bool   `json:"zoomed"`
	Active bool   `json:"active"`
	Panes  []Pane `json:"panes"`
}

// Pane's ScrollbackFile is relative to the state directory. PortalPaneID
// persists the pane's durable identity token across a reboot — the tmux pane
// user-option holding it does not outlive the server. It decodes to "" for a
// pane that has never been stamped, which is the ordinary case.
type Pane struct {
	Index          int    `json:"index"`
	CWD            string `json:"cwd"`
	Active         bool   `json:"active"`
	CurrentCommand string `json:"current_command"`
	ScrollbackFile string `json:"scrollback_file"`
	PortalPaneID   string `json:"portal_pane_id"`
}

// Canonicalize replaces nil slices and maps so they encode as [] and {} rather
// than null. It mutates the receiver.
func (idx *Index) Canonicalize() {
	idx.Version = SchemaVersion

	if idx.Sessions == nil {
		idx.Sessions = []Session{}
	}
	for i := range idx.Sessions {
		s := &idx.Sessions[i]
		if s.Environment == nil {
			s.Environment = map[string]string{}
		}
		if s.Windows == nil {
			s.Windows = []Window{}
		}
		for j := range s.Windows {
			w := &s.Windows[j]
			if w.Panes == nil {
				w.Panes = []Pane{}
			}
		}
	}
}

// EncodeIndex serialises idx to canonical indented JSON without mutating it.
func EncodeIndex(idx Index) ([]byte, error) {
	local := idx
	local.Canonicalize()
	return json.MarshalIndent(local, "", "  ")
}

// DecodeIndex errors on malformed JSON, a missing version field, and a version
// other than SchemaVersion; unknown fields are ignored for forward compat.
func DecodeIndex(data []byte) (Index, error) {
	var idx Index
	if err := json.Unmarshal(data, &idx); err != nil {
		return idx, fmt.Errorf("decode sessions.json: %w", err)
	}
	if idx.Version == 0 {
		return idx, errors.New("sessions.json missing version field")
	}
	if idx.Version != SchemaVersion {
		return idx, fmt.Errorf("unsupported sessions.json version: %d (current: %d)", idx.Version, SchemaVersion)
	}
	return idx, nil
}
