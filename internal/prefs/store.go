// Package prefs provides persistence for UI preferences that do not belong in a
// domain store like projects.json. It owns the last-used session list grouping
// mode, persisted to prefs.json.
//
// The package is a pure leaf — it imports only the standard library and
// internal/fileutil — so it is safe to import from internal/tui without an
// import cycle. It deliberately does NOT emit audit/breadcrumb logging:
// prefs.json is not part of the closed state-mutation audit-trail set
// (hooks/aliases/projects), so it must not import internal/log or
// internal/storelog.
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

// SessionListMode is the grouping mode for the TUI session list. It is the
// single source of truth for the three modes; the TUI reuses this type.
type SessionListMode int

const (
	// ModeFlat is the ungrouped session list and the first-run / tolerant-decode
	// default.
	ModeFlat SessionListMode = iota
	// ModeByProject groups sessions by their resolved project directory.
	ModeByProject
	// ModeByTag groups sessions by their project tags.
	ModeByTag
)

// Canonical on-disk strings for each mode. String enum (not int) so prefs.json
// stays human-readable and stable.
const (
	modeFlatString      = "flat"
	modeByProjectString = "by-project"
	modeByTagString     = "by-tag"
)

// String returns the canonical on-disk string for the mode. An out-of-range
// value maps to the flat default so the marshalled form is always one of the
// three canonical tokens.
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

// parseMode maps a canonical on-disk string to its mode. Any unrecognised value
// collapses to ModeFlat (tolerant decode).
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

// migrationMarker is the tolerant decode for the one-shot appearance-translation
// gate: anything that is not literal `true` — absent, empty, corrupt,
// wrong-typed, unrecognised — is false.
//
// It never returns an error, and that totality is load-bearing: prefs.json has
// two decodes and a field-level error breaks both — the tolerant load path treats
// any error as corruption and collapses to a zero record, while the write path's
// strict re-read aborts every subsequent write. This is the field a user is most
// likely to hand-edit wrongly (`"theme_migrated": "yes"`), so without the
// absorption one hand-typed word would lock them out of their own prefs.
//
// The failure direction is deliberate: false means "run the translation again",
// and the translation is idempotent, so a corrupt marker costs one redundant
// write rather than a wrong theme.
type migrationMarker bool

// UnmarshalJSON accepts every JSON value — including null, an array and an
// object — and yields true only for the literal `true`.
func (m *migrationMarker) UnmarshalJSON(data []byte) error {
	*m = migrationMarker(bytes.Equal(bytes.TrimSpace(data), []byte("true")))
	return nil
}

// MarshalJSON writes the marker as a JSON boolean. On the prefsFile path
// omitempty means only a true marker ever reaches this: a false one is absent
// from the file rather than present-and-false.
func (m migrationMarker) MarshalJSON() ([]byte, error) {
	return strconv.AppendBool(nil, bool(m)), nil
}

// prefsFile is the on-disk JSON structure for prefs.json. Each preference is an
// independent field; a missing field decodes to its zero value, which the
// per-field parsers collapse to their default (tolerant decode).
//
// Empty values are omitted on write across every field, so a key the user has
// never set is absent from the file rather than present-and-empty — which keeps
// a hand-edited file clean and lets an older binary read an absent appearance as
// absent rather than as an empty string. session_list_mode is under the same rule
// even though Save always marshals a canonical non-empty token, because the
// field-specific savers write records whose mode was never set.
type prefsFile struct {
	SessionListMode string `json:"session_list_mode,omitempty"`
	// Appearance is a plain string that is read and preserved, never parsed — the
	// enum, its tolerant decode and its accessors are gone, but this slot in the
	// file stays so a downgraded binary still honours the user's pin. Do not
	// delete the field: prefs.json decodes into this plain struct, so any key not
	// declared here is dropped on re-encode, and every writer re-encodes the whole
	// file — the first `s` keypress after upgrade would silently erase the pin,
	// invisible until the user downgrades.
	Appearance string `json:"appearance,omitempty"`
	Theme      string `json:"theme,omitempty"`
	ThemeLight string `json:"theme_light,omitempty"`
	ThemeDark  string `json:"theme_dark,omitempty"`
	// ThemeMigrated is the one-shot gate for the appearance translation, declared
	// here beside the keys the translation writes for the same reason Appearance
	// is: an undeclared key is dropped on re-encode, so a marker written by a
	// newer instance would be silently erased by an older code path, re-arming a
	// translation that must fire exactly once ever.
	//
	// omitempty makes a false marker absent rather than present-and-false, the
	// same on-disk shape a never-migrated file has. Its own total decode (see
	// migrationMarker) keeps a hand-edited value from erroring either decode path.
	ThemeMigrated migrationMarker `json:"theme_migrated,omitempty"`
}

// ThemeKeys carries the three raw theme slugs persisted in prefs.json: a constant
// Theme, or the adaptive Light/Dark pair. The values are exactly what is on disk —
// interpreting them (validation, defaulting, the theme-wins tiebreak) belongs to the
// resolver, not to this store.
type ThemeKeys struct {
	Theme string
	Light string
	Dark  string
}

// MigrationState is the one-shot gate's input, read in one go: the retained raw
// appearance value and whether the translation has already been recorded.
//
// Appearance is the on-disk value verbatim and unparsed. prefs has no Appearance
// enum any more and must not regrow one: the dark/light/auto mapping belongs to
// the translation, and the slot is retained purely as a frozen legacy value a
// downgraded binary can still honour.
type MigrationState struct {
	Appearance string
	Migrated   bool
}

// ThemeSlot names one half of the adaptive pair. Slot assignment takes a typed
// value rather than a caller-supplied key name so no caller can mint a third
// slot, and the zero value is deliberately invalid — and deliberately unnamed —
// so a forgotten argument cannot silently write the light slot.
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

// readBytes reads prefs.json and classifies the read — the half both decodes
// share, held here so the absent-file and permission policies have one home. An
// absent file (the normal first-run state) is reported as not present with no
// error; any other read failure is returned verbatim, with no bytes.
//
// It stops at the bytes. Which record those bytes decode to, and what a decode
// failure means, is the half the two paths must not share — see readFile and
// readFileStrict.
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

// readFile reads and decodes prefs.json with the tolerant policy shared by every
// loader: a missing file (the normal first-run state) and an empty or
// corrupt/unparseable file both yield a zero-valued prefsFile with no error (its
// empty-string fields collapse to each preference's default). Only a non-ErrNotExist
// read error is propagated, alongside the zero prefsFile. The bool reports whether
// the file existed and decoded cleanly — callers that read-modify-write use it to
// avoid clobbering a sibling field's value with a default when the file is present.
//
// This is the load-path decode and it is not usable on the write path. prefs.json
// has two decodes and they must differ: reading is tolerant — missing, empty or
// unrecognised falls to the shipped default per field — while the write-path
// re-read (readFileStrict) judges syntax and aborts. Routing a writer through
// here would remove the abort's only trigger: a stray comma collapses to a
// zero-valued record with no error, so the writer merges into an empty struct and
// commits it, erasing session_list_mode, every theme key and the retained raw
// appearance in one keypress. Neither function may do the other's job.
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
		// Tolerant decode: an empty file unmarshals as a JSON error and lands
		// here, as does any corrupt/unparseable content.
		return prefsFile{}, false, nil
	}

	return f, true, nil
}

// readFileStrict reads and decodes prefs.json with the write-path policy: it is
// the read half of every read-modify-write in this store, and it judges syntax so
// a writer never merges into a record the file does not actually contain.
//
// Its three outcomes are:
//
//   - Absent file — returns (zero prefsFile, false, nil). There is nothing to
//     merge and nothing to lose, so the caller proceeds and creates the file. A
//     fresh install has no prefs.json, and aborting here would leave it
//     permanently uncreated.
//   - Present but unusable — malformed JSON, or any non-ErrNotExist read failure —
//     returns the error verbatim so the caller aborts with the on-disk bytes
//     untouched. A write never becomes an overwrite.
//   - Present and decodable — returns (record, true, nil).
//
// It judges syntax only. Unrecognised values in syntactically valid JSON are
// absorbed exactly as the load path absorbs them; treating them as fatal would
// make hand-editing prefs.json a way to lock yourself out of every write.
//
// The *json.UnmarshalTypeError carve-out discriminates on `Field != ""` rather
// than on the error type alone:
//
//   - Field non-empty — a wrong type on a declared field. encoding/json skips just
//     that field and still populates every other one, so the decoded record is
//     complete enough to merge into and the write proceeds; the offending value is
//     normalised away on re-encode.
//   - Field empty — a top-level type mismatch (`[1,2]`, `"x"`, `3` as the whole
//     document). That yields the same error type but a wholly zero-valued struct,
//     so absorbing it would merge into an empty record and commit it. It aborts.
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

// mutate performs the write-path read-modify-write every save method in this
// store routes through: strict re-read immediately before the write, apply fn to
// the decoded record, write the merged result through AtomicWrite.
//
// The re-read happens immediately before the write, not at load. A stale
// in-memory snapshot is what silently reverts another instance's commit, and
// AtomicWrite does not help because that is a lost update, not a partial write.
//
// fn receives whether the file existed — the marker write needs it, since a
// migration marker must not be recorded in a file that does not exist — and
// returns false to skip the write entirely: no bytes touched, no error. A
// readFileStrict error is returned verbatim and nothing is written; prefs is a
// leaf with no logging, so an abort is reported by returning and the caller
// decides non-fatality.
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

// Load reads the persisted session list mode from prefs.json.
//
// Every degenerate input collapses to ModeFlat with no hard error: a missing
// file (the normal first-run state), an empty or corrupt/unparseable file, and
// an unrecognised session_list_mode value all return (ModeFlat, nil). Only a
// non-ErrNotExist read error is propagated, alongside ModeFlat.
//
// This is a load, so it stays on the tolerant readFile and is unaffected by the
// write path's strict decode — see readFile.
func (s *Store) Load() (SessionListMode, error) {
	f, _, err := s.readFile()
	if err != nil {
		return ModeFlat, err
	}
	return parseMode(f.SessionListMode), nil
}

// LoadThemeKeys reads the three raw theme slugs from prefs.json.
//
// It applies the exact same tolerant policy as Load: a missing file, an empty or
// corrupt/unparseable file, and any missing key all yield zero-valued strings with
// no error. Only a non-ErrNotExist read error is propagated, alongside a zero
// ThemeKeys.
//
// It performs no interpretation of the values it returns: no slug-charset check,
// no trimming, no lowercasing, no default substitution and no theme-wins
// tiebreak. An unrecognised value is a resolution problem, not a decode one — and
// trimming would convert a stray-space value into a silently different slug
// instead of the honest `bad name` rejection the charset check owes the user.
//
// This is a load, so it stays on the tolerant readFile and is unaffected by the
// write path's strict decode — see readFile.
func (s *Store) LoadThemeKeys() (ThemeKeys, error) {
	f, _, err := s.readFile()
	if err != nil {
		return ThemeKeys{}, err
	}
	return ThemeKeys{Theme: f.Theme, Light: f.ThemeLight, Dark: f.ThemeDark}, nil
}

// LoadMigrationState reads the translation gate's inputs — the retained raw
// appearance and the migration marker — in one read.
//
// It applies the exact same tolerant policy as Load and LoadThemeKeys: a missing
// file, an empty or corrupt/unparseable file, and either key missing all yield a
// zero-valued MigrationState with no error. Only a non-ErrNotExist read error is
// propagated, alongside a zero value. A zero value is the safe direction — no
// appearance to translate and an unrecorded marker means the gate re-evaluates
// next launch, which is idempotent.
//
// It performs no interpretation of the appearance value: no parsing, no
// trimming, no lowercasing, no default substitution — see MigrationState.
//
// This is a load, so it stays on the tolerant readFile and is unaffected by the
// write path's strict decode — see readFile.
func (s *Store) LoadMigrationState() (MigrationState, error) {
	f, _, err := s.readFile()
	if err != nil {
		return MigrationState{}, err
	}
	return MigrationState{Appearance: f.Appearance, Migrated: bool(f.ThemeMigrated)}, nil
}

// Save persists the given mode to prefs.json, read-modify-writing through mutate
// so every other key — the retained raw appearance and the three theme slugs —
// survives untouched, and so the whole record lands in one AtomicWrite.
//
// A malformed prefs.json aborts this write rather than overwriting it, the same
// rule the theme savers follow: a tolerant decode would turn a stray comma into a
// zero-valued record and commit that, erasing every key the user had set. This is
// the writer most likely to fire, being one keypress in the picker. The caller
// swallows the error, so the failure is non-fatal and non-destructive: the user
// loses a grouping-mode persist, not their theme.
func (s *Store) Save(mode SessionListMode) error {
	return s.mutate(func(f *prefsFile, _ bool) bool {
		f.SessionListMode = mode.String()
		return true
	})
}

// SaveTheme persists slug as the constant theme, clearing both adaptive slots in
// the same write.
//
// The clear is the setting's mutual exclusion enforced on write, so "both a
// constant and a pair are present" cannot arise from Portal's own writes. The
// commit and the clear ride one mutate — and so one AtomicWrite — because two
// writes would leave a reachable window where the file holds both forms.
//
// Clearing is writing the empty string, which omitempty renders as key-absent —
// matching "an unset slot holds the shipped default" and keeping a hand-edited
// file clean.
//
// The write is unconditional, which makes a commit always re-attemptable:
// committing the same slug again rewrites the same bytes.
//
// The slug is persisted verbatim. prefs has no slug knowledge: no charset check,
// no trimming, no lowercasing, no default substitution and no theme-wins
// tiebreak. Those are read-side resolution rules owned by internal/theme, and a
// "helpful" second implementation here would diverge from the resolver —
// trimming in particular would turn a stray-space value into a silently
// different slug instead of the honest `bad name` rejection the user is owed.
func (s *Store) SaveTheme(slug string) error {
	return s.mutate(func(f *prefsFile, _ bool) bool {
		f.Theme = slug
		f.ThemeLight = ""
		f.ThemeDark = ""
		return true
	})
}

// SaveThemeSlot persists slug into one half of the adaptive pair, clearing the
// constant in the same write and leaving the other slot exactly as it was — the
// property that makes a `● both` badge reachable in two keypresses.
//
// It is the mirror of SaveTheme and carries the same four rules: mutual exclusion
// enforced on write, both mutations in one AtomicWrite, a cleared key written as
// the empty string so omitempty omits it, and an unconditional write. It performs
// no slug validation either — see SaveTheme.
//
// An out-of-range slot writes nothing at all: the guard runs before the mutator,
// so the file is neither read nor written, and the returned error names the
// invalid slot. That is the structural half of "no caller can mint a third slot";
// the typed constants are the other half.
func (s *Store) SaveThemeSlot(slug string, slot ThemeSlot) error {
	if slot != SlotLight && slot != SlotDark {
		return fmt.Errorf("prefs: invalid theme slot %d", slot)
	}

	return s.mutate(func(f *prefsFile, _ bool) bool {
		// No default arm: the guard above has already rejected every value that
		// is not one half of the pair.
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

// SaveMigrationMarker records the one-shot gate — the marker that says the
// appearance translation has run — and writes nothing else. All three theme keys,
// session_list_mode and the retained raw appearance round-trip through it
// untouched, and it neither reads nor enforces the theme keys' mutual exclusion:
// the marker is orthogonal to which theme keys are set, which is what makes the
// gate a recorded fact rather than a re-armable absence.
//
// It takes the abort-on-undecodable rule and deliberately not a create-on-absent
// half: an absent prefs.json means a fresh install, which has no appearance to
// translate, so creating the file purely to record a marker would add a side
// effect to a path that otherwise leaves the filesystem alone. Declining is not a
// failure — nil is returned, the condition is re-evaluated next launch, and the
// file appears the moment the user changes anything.
//
// Absence is judged at the same RMW re-read as everything else — mutate's
// `existed` — never against a stat taken at load. prefs.json can appear in
// between (another instance's first commit; the multi-window burst makes
// concurrent instances normal), and a save trusting a load-time snapshot would
// decline forever on a file that now exists.
//
// It has no reporting surface and needs none: it runs at prefs load, before any
// panel exists. Its failure signal is the absence of the migration event, and it
// retries next launch. prefs is a leaf, so an abort is reported by returning and
// the caller decides non-fatality.
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
// theme key and the migration marker — in one atomic write, reporting whether a
// theme key was actually persisted.
//
// One write, not two: the field-specific savers each perform their own
// read-modify-write, and the translation's write is best-effort and non-blocking,
// so a failure landing between a SaveTheme and a SaveMigrationMarker would
// persist the theme key with the marker unset. The next launch would then find
// the marker false, see a theme key already set, write only the marker, and never
// emit `theme: appearance migrated` — the translation succeeding while the log
// says it did not.
//
// The whole decision is made inside the mutator, against the re-read bytes about
// to be merged, never against a load-time snapshot. Because the write is
// non-blocking a user can commit a theme between compute and persist, and against
// a stale snapshot the pending translation would write its own key over the one
// they just committed and clear the slots. The same re-read is what lets this
// instance observe that another instance already set the marker.
//
// The four branches, in order:
//
//   - The file does not exist — nothing is written and nothing is created: a
//     fresh install has no appearance to translate. It is a silent no-op
//     returning nil, because an error would invite a caller to treat an ordinary
//     fresh install as a failure.
//   - The marker is already true — nothing is written. Another instance recorded
//     the translation in between; the trigger fires exactly once ever.
//   - Any theme key is already set, or the slug is empty — the marker alone is
//     recorded, which is what stops the translation staying pending forever. A
//     set key is a choice the user has already made; an empty slug means there
//     was nothing to translate. Absorbing both here is also why the writing
//     branch below satisfies mutual exclusion trivially: it only ever runs with
//     all three keys empty.
//   - Otherwise — the theme key is written, both slots are cleared and the marker
//     is recorded in the one write, and persisted is true.
//
// persisted is a first-class result rather than an inference from the error:
// `theme: appearance migrated` fires only when a theme key was actually
// persisted, and a marker-only run translated nothing.
//
// It emits nothing — prefs is a leaf — so an abort is reported by returning and
// the caller decides non-fatality. An aborted or failed write leaves the marker
// unset, so the condition still holds and the next launch retries, which is what
// makes a best-effort write safe here.
func (s *Store) SaveTranslation(slug string) (persisted bool, err error) {
	err = s.mutate(func(f *prefsFile, existed bool) bool {
		if !existed {
			return false
		}
		if bool(f.ThemeMigrated) {
			return false
		}

		// Every remaining branch records the marker; only the last one also
		// writes a theme key.
		f.ThemeMigrated = true

		if slug == "" || f.Theme != "" || f.ThemeLight != "" || f.ThemeDark != "" {
			return true
		}

		f.Theme = slug
		// Already empty by construction — see the branch above — so this states
		// the mutual-exclusion rule rather than doing work.
		f.ThemeLight = ""
		f.ThemeDark = ""
		persisted = true
		return true
	})
	if err != nil {
		// Nothing was persisted: an undecodable re-read never reaches the
		// mutator, and a failed write leaves the on-disk bytes as they were.
		return false, err
	}

	return persisted, nil
}

// atomicWrite is fileutil.AtomicWrite behind a package-level indirection so
// commits can be counted. "One atomic write per save" is a contract — the commit
// and its mutual-exclusion clear land together, so no partial state is reachable
// — and it is not observable from the filesystem afterwards, since a second write
// leaves no trace. Production never reassigns it.
var atomicWrite = fileutil.AtomicWrite

// write marshals the prefsFile and commits it via AtomicWrite (temp file + rename).
func (s *Store) write(f prefsFile) error {
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal prefs: %w", err)
	}

	return atomicWrite(s.path, data)
}
