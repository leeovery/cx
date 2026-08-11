// Package prefs persists UI preferences to prefs.json: the last-used session
// list grouping mode and the theme setting. Deliberately a leaf — stdlib plus
// internal/fileutil only — so internal/tui can import it without a cycle.
package prefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/leeovery/portal/internal/fileutil"
)

// SessionListMode is the grouping mode for the TUI session list.
type SessionListMode int

const (
	// ModeFlat is the ungrouped session list and the tolerant-decode default.
	ModeFlat SessionListMode = iota
	// ModeByProject groups sessions by their resolved project directory.
	ModeByProject
	// ModeByTag groups sessions by their project tags.
	ModeByTag
)

// Strings rather than ints keep prefs.json human-readable.
const (
	modeFlatString      = "flat"
	modeByProjectString = "by-project"
	modeByTagString     = "by-tag"
)

// String returns the canonical on-disk string for the mode; out-of-range
// values map to the flat default.
func (m SessionListMode) String() string {
	switch m {
	case ModeByProject:
		return modeByProjectString
	case ModeByTag:
		return modeByTagString
	default:
		return modeFlatString
	}
}

func parseMode(s string) SessionListMode {
	switch s {
	case modeByProjectString:
		return ModeByProject
	case modeByTagString:
		return ModeByTag
	default:
		return ModeFlat
	}
}

// Decodes any JSON value without error: a field-level error would zero the
// tolerant load's whole record and abort every write's strict re-read.
type migrationMarker bool

func (m *migrationMarker) UnmarshalJSON(data []byte) error {
	*m = migrationMarker(bytes.Equal(bytes.TrimSpace(data), []byte("true")))
	return nil
}

func (m migrationMarker) MarshalJSON() ([]byte, error) {
	return strconv.AppendBool(nil, bool(m)), nil
}

type prefsFile struct {
	SessionListMode string `json:"session_list_mode,omitempty"`
	// Preserved verbatim, never parsed. Do not delete the field: an undeclared
	// key is dropped on re-encode, erasing a downgraded binary's pin.
	Appearance string `json:"appearance,omitempty"`
	Theme      string `json:"theme,omitempty"`
	ThemeLight string `json:"theme_light,omitempty"`
	ThemeDark  string `json:"theme_dark,omitempty"`
	// Must stay declared for the same re-encode reason as Appearance.
	ThemeMigrated migrationMarker `json:"theme_migrated,omitempty"`
}

// ThemeKeys carries the three raw theme slugs from prefs.json, verbatim —
// validation, defaulting and the theme-wins tiebreak belong to the resolver.
type ThemeKeys struct {
	Theme string
	Light string
	Dark  string
}

// MigrationState is the appearance-translation gate's input: the on-disk
// appearance value, verbatim and unparsed, and whether the translation ran.
type MigrationState struct {
	Appearance string
	Migrated   bool
}

// ThemeSlot names one half of the adaptive pair; the zero value is deliberately
// invalid so a forgotten argument cannot silently write the light slot.
type ThemeSlot int

const (
	// SlotLight is the theme_light half of the adaptive pair.
	SlotLight ThemeSlot = iota + 1
	// SlotDark is the theme_dark half of the adaptive pair.
	SlotDark
)

// Store manages persistence of UI preferences to a JSON file.
type Store struct {
	path string
}

// NewStore creates a Store that reads and writes to the given file path.
func NewStore(path string) *Store {
	return &Store{path: path}
}

func (s *Store) readBytes() (data []byte, present bool, err error) {
	data, err = os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, false, nil
		}
		return nil, false, err
	}

	return data, true, nil
}

// Tolerant load-path decode. Never route a writer through it: corrupt content
// collapses to a zero record with no error, which a merge would then commit.
func (s *Store) readFile() (prefsFile, bool, error) {
	data, present, err := s.readBytes()
	if err != nil {
		return prefsFile{}, false, err
	}
	if !present {
		return prefsFile{}, false, nil
	}

	var f prefsFile
	if err := json.Unmarshal(data, &f); err != nil {
		return prefsFile{}, false, nil
	}

	return f, true, nil
}

// Strict write-path decode: malformed content aborts rather than merges. The
// Field != "" carve-out separates a wrong-typed declared field (rest of the
// record still populated) from a top-level mismatch (wholly zero record).
func (s *Store) readFileStrict() (prefsFile, bool, error) {
	data, present, err := s.readBytes()
	if err != nil {
		return prefsFile{}, false, err
	}
	if !present {
		return prefsFile{}, false, nil
	}

	var f prefsFile
	if err := json.Unmarshal(data, &f); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) && typeErr.Field != "" {
			return f, true, nil
		}
		return prefsFile{}, false, err
	}

	return f, true, nil
}

// The strict re-read happens at write time, not load: a stale snapshot would
// silently revert another instance's commit.
func (s *Store) mutate(fn func(f *prefsFile, existed bool) bool) error {
	f, existed, err := s.readFileStrict()
	if err != nil {
		return err
	}
	if !fn(&f, existed) {
		return nil
	}
	return s.write(f)
}

// Load reads the persisted session list mode; every degenerate file yields
// ModeFlat with no error, and only a non-ErrNotExist read error propagates.
func (s *Store) Load() (SessionListMode, error) {
	f, _, err := s.readFile()
	if err != nil {
		return ModeFlat, err
	}
	return parseMode(f.SessionListMode), nil
}

// LoadThemeKeys reads the three raw theme slugs verbatim, under the same
// tolerant policy as Load.
func (s *Store) LoadThemeKeys() (ThemeKeys, error) {
	f, _, err := s.readFile()
	if err != nil {
		return ThemeKeys{}, err
	}
	return ThemeKeys{Theme: f.Theme, Light: f.ThemeLight, Dark: f.ThemeDark}, nil
}

// LoadMigrationState reads the raw appearance value and the migration marker,
// under the same tolerant policy as Load.
func (s *Store) LoadMigrationState() (MigrationState, error) {
	f, _, err := s.readFile()
	if err != nil {
		return MigrationState{}, err
	}
	return MigrationState{Appearance: f.Appearance, Migrated: bool(f.ThemeMigrated)}, nil
}

// Save persists the grouping mode, leaving every other key untouched; a
// malformed prefs.json aborts the write rather than overwriting it.
func (s *Store) Save(mode SessionListMode) error {
	return s.mutate(func(f *prefsFile, _ bool) bool {
		f.SessionListMode = mode.String()
		return true
	})
}

// SaveTheme persists slug verbatim as the constant theme, clearing both
// adaptive slots in the same atomic write.
func (s *Store) SaveTheme(slug string) error {
	return s.mutate(func(f *prefsFile, _ bool) bool {
		f.Theme = slug
		f.ThemeLight = ""
		f.ThemeDark = ""
		return true
	})
}

// SaveThemeSlot persists slug into one adaptive slot, clearing the constant in
// the same write and leaving the other slot untouched; an invalid slot writes
// nothing and returns an error naming it.
func (s *Store) SaveThemeSlot(slug string, slot ThemeSlot) error {
	if slot != SlotLight && slot != SlotDark {
		return fmt.Errorf("prefs: invalid theme slot %d", slot)
	}

	return s.mutate(func(f *prefsFile, _ bool) bool {
		switch slot {
		case SlotLight:
			f.ThemeLight = slug
		case SlotDark:
			f.ThemeDark = slug
		}
		f.Theme = ""
		return true
	})
}

// SaveMigrationMarker records that the appearance translation has run, writing
// nothing else. It never creates an absent prefs.json — a fresh install has
// nothing to translate — and declining returns nil rather than an error.
func (s *Store) SaveMigrationMarker() error {
	return s.mutate(func(f *prefsFile, existed bool) bool {
		if !existed {
			return false
		}
		f.ThemeMigrated = true
		return true
	})
}

// SaveTranslation records the one-shot appearance translation — the translated
// theme key and the migration marker — in a single atomic write, reporting
// whether a theme key was actually persisted.
//
// One write, not two: a failure between separate writes would persist the key
// with the marker unset. The decision is made against the write-time re-read,
// so a concurrently committed theme or marker wins; the file is never created,
// and an aborted write leaves the marker unset so the next launch retries.
func (s *Store) SaveTranslation(slug string) (persisted bool, err error) {
	err = s.mutate(func(f *prefsFile, existed bool) bool {
		if !existed {
			return false
		}
		if bool(f.ThemeMigrated) {
			return false
		}

		f.ThemeMigrated = true

		if slug == "" || f.Theme != "" || f.ThemeLight != "" || f.ThemeDark != "" {
			return true
		}

		f.Theme = slug
		// Already empty by construction; kept to state the mutual-exclusion
		// rule.
		f.ThemeLight = ""
		f.ThemeDark = ""
		persisted = true
		return true
	})
	if err != nil {
		// A failed write persisted nothing, even if the mutator set persisted.
		return false, err
	}

	return persisted, nil
}

// Indirection so commits can be counted; production never reassigns it.
var atomicWrite = fileutil.AtomicWrite

func (s *Store) write(f prefsFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal prefs: %w", err)
	}

	return atomicWrite(s.path, data)
}
