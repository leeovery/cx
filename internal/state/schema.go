package state

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// SchemaVersion is the current sessions.json schema version, bumped on any
// schema-breaking change.
const SchemaVersion = 1

// Index is the root document persisted to sessions.json.
type Index struct {
	Version  int       `json:"version"`
	SavedAt  time.Time `json:"saved_at"`
	Sessions []Session `json:"sessions"`
}

// Session captures a single tmux session.
//
// PortalID persists the session's immutable @portal-id so a renamed session's
// hook key survives a reboot — tmux user-options do not outlive the server. It
// decodes to "" for a legacy un-stamped session, which restore keys by name.
type Session struct {
	Name        string            `json:"name"`
	PortalID    string            `json:"portal_id"`
	Environment map[string]string `json:"environment"`
	Windows     []Window          `json:"windows"`
}

// Window captures a single tmux window.
type Window struct {
	Index  int    `json:"index"`
	Name   string `json:"name"`
	Layout string `json:"layout"`
	Zoomed bool   `json:"zoomed"`
	Active bool   `json:"active"`
	Panes  []Pane `json:"panes"`
}

// Pane captures a single tmux pane. ScrollbackFile is relative to the state
// directory.
type Pane struct {
	Index          int    `json:"index"`
	CWD            string `json:"cwd"`
	Active         bool   `json:"active"`
	CurrentCommand string `json:"current_command"`
	ScrollbackFile string `json:"scrollback_file"`
}

// Canonicalize normalises the index for stable on-disk encoding, replacing nil
// slices and maps so they encode as [] and {} rather than null. It mutates the
// receiver.
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

// DecodeIndex parses a sessions.json payload. Malformed JSON, a missing version
// field and a version other than SchemaVersion are all errors; unknown fields
// are ignored for forward compatibility.
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
