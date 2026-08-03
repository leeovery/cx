// White-box (package prefs) for the same reasons store_write_path_test.go and
// theme_savers_test.go are: the marker's tolerant decode is an unexported type
// whose omission-on-false is asserted against the encoded struct directly, and
// the write-path properties it inherits are only reachable through the
// unexported mutate seam. The helpers declared in store_write_path_test.go
// (seedPrefsFile, readRaw, decodeWritten, assertWrittenValue, assertNoTempFiles,
// assertUntouched) and theme_savers_test.go (assertKeysAbsent, themeSaverCases)
// are reused rather than redeclared: this file is the same package.
package prefs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// assertMarkerOnDisk asserts the on-disk marker.
func assertMarkerOnDisk(t *testing.T, path string, want bool) {
	t.Helper()

	assertMarkerValue(t, decodeWritten(t, path), want)
}

// assertMarkerValue asserts the marker in an already-decoded record — the shape
// the translation's tests need, since they assert against the bytes captured at
// the atomicWrite seam as well as against the file. A false marker is ABSENT
// rather than present-and-false (§8.1's omit-empty-values rule), so the two
// cases are asserted structurally on the decoded map — a substring check for
// `"theme_migrated"` could not tell `false` from omitted.
func assertMarkerValue(t *testing.T, decoded map[string]any, want bool) {
	t.Helper()

	got, ok := decoded["theme_migrated"]
	if !want {
		if ok {
			t.Errorf("written JSON carries theme_migrated = %#v, want the key absent — a false marker is omitted, never written", got)
		}
		return
	}
	if !ok {
		t.Errorf("written JSON has no theme_migrated key, want true — the marker was dropped on re-encode")
		return
	}
	if got != true {
		t.Errorf("written JSON theme_migrated = %#v, want true", got)
	}
}

// markerWriterCase drives every writer in the store over the marker's
// round-trip. The point is coverage of ALL of them: §8.8's drop-on-re-encode
// hazard is a property of the struct, so one unfixed writer is enough to erase
// an on-disk marker.
//
// setsMarker names the one writer whose job IS the marker, so the tests that
// assert nobody invents the key can exclude it by property rather than by a
// second, driftable table.
type markerWriterCase struct {
	name       string
	setsMarker bool
	write      func(store *Store) error
}

func markerWriterCases() []markerWriterCase {
	return []markerWriterCase{
		{name: "Save", write: func(s *Store) error { return s.Save(ModeByProject) }},
		{name: "SaveTheme", write: func(s *Store) error { return s.SaveTheme("nord") }},
		{name: "SaveThemeSlot light", write: func(s *Store) error { return s.SaveThemeSlot("nord", SlotLight) }},
		{name: "SaveThemeSlot dark", write: func(s *Store) error { return s.SaveThemeSlot("nord", SlotDark) }},
		{name: "SaveMigrationMarker", setsMarker: true, write: func(s *Store) error { return s.SaveMigrationMarker() }},
	}
}

// TestMigrationMarker_RoundTrips is why the field is declared in this task,
// before its first writer exists (task 6-4). prefs.json decodes into a plain Go
// struct, so any key NOT declared as a field is dropped on re-encode (§8.8), and
// §8.9 makes every writer re-encode the whole file. Leave it undeclared and a
// marker written by a newer instance is erased by the next `s` keypress or theme
// commit from an older code path in the same release — silently re-arming a
// translation §10.3 guarantees fires exactly once ever.
func TestMigrationMarker_RoundTrips(t *testing.T) {
	const seeded = `{"session_list_mode":"flat","appearance":"dark","theme_dark":"tokyo-night","theme_migrated":true}`

	for _, c := range markerWriterCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, seeded)

			if err := c.write(NewStore(path)); err != nil {
				t.Fatalf("unexpected write error: %v", err)
			}

			assertMarkerOnDisk(t, path, true)
		})
	}
}

// TestMigrationMarker_FalseIsAbsentOnDisk pins §8.1's omission rule for the
// marker: only a true marker appears on disk. A false one is absent, which is
// the same shape a never-migrated file already has — so the marker adds no key
// to a file that has nothing to record.
func TestMigrationMarker_FalseIsAbsentOnDisk(t *testing.T) {
	t.Run("the encoded struct omits false and names true", func(t *testing.T) {
		cases := []struct {
			name   string
			record prefsFile
			want   bool
		}{
			{name: "false", record: prefsFile{SessionListMode: "flat"}, want: false},
			{name: "true", record: prefsFile{SessionListMode: "flat", ThemeMigrated: true}, want: true},
		}

		for _, c := range cases {
			t.Run(c.name, func(t *testing.T) {
				data, err := json.Marshal(c.record)
				if err != nil {
					t.Fatalf("failed to marshal prefsFile: %v", err)
				}

				var decoded map[string]any
				if err := json.Unmarshal(data, &decoded); err != nil {
					t.Fatalf("encoded prefsFile is not valid JSON (%q): %v", data, err)
				}

				got, ok := decoded["theme_migrated"]
				if !c.want {
					if ok {
						t.Errorf("encoded %q carries theme_migrated = %#v, want the key absent", data, got)
					}
					return
				}
				if got != true {
					t.Errorf("encoded %q theme_migrated = %#v, want true", data, got)
				}
			})
		}
	})

	t.Run("no writer invents the key on a markerless file", func(t *testing.T) {
		for _, c := range markerWriterCases() {
			if c.setsMarker {
				continue
			}
			t.Run(c.name, func(t *testing.T) {
				path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":"nord"}`)

				if err := c.write(NewStore(path)); err != nil {
					t.Fatalf("unexpected write error: %v", err)
				}

				assertMarkerOnDisk(t, path, false)
			})
		}
	})
}

// TestMigrationMarker_TolerantDecode pins §8.1's decode rule for the marker:
// anything that is not literal `true` — absent, empty, corrupt, wrong-typed,
// unrecognised — is false, and NEITHER decode path errors on it.
//
// The two paths are asserted separately because they differ by design (§8.9) and
// the marker has to be total in both. Each is asserted through its "the record
// decoded cleanly" bool as well as the value, because neither reports a decode
// failure as an error: the tolerant path swallows it and returns a ZERO record,
// so a marker that merely happens to be false would look identical to one that
// took the whole record down with it.
//
// The failure direction is deliberate. False means "run the translation again",
// and the translation is idempotent by §10.5, so a corrupt marker costs one
// redundant write rather than a wrong theme.
func TestMigrationMarker_TolerantDecode(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    bool
	}{
		{name: "literal true", content: `{"theme_migrated":true}`, want: true},
		{name: "literal false", content: `{"theme_migrated":false}`},
		{name: "the string yes", content: `{"theme_migrated":"yes"}`},
		{name: "the string true", content: `{"theme_migrated":"true"}`},
		{name: "the number 1", content: `{"theme_migrated":1}`},
		{name: "the number 0", content: `{"theme_migrated":0}`},
		{name: "null", content: `{"theme_migrated":null}`},
		{name: "an empty string", content: `{"theme_migrated":""}`},
		{name: "an array", content: `{"theme_migrated":[]}`},
		{name: "an object", content: `{"theme_migrated":{}}`},
		{name: "a populated object", content: `{"theme_migrated":{"at":"yesterday"}}`},
		{name: "absent", content: `{"session_list_mode":"flat"}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := NewStore(seedPrefsFile(t, c.content))

			tolerant, decoded, err := store.readFile()
			if err != nil {
				t.Fatalf("unexpected readFile error: %v", err)
			}
			if !decoded {
				t.Errorf("readFile reports the record did not decode — the marker must never take the whole record down")
			}
			if bool(tolerant.ThemeMigrated) != c.want {
				t.Errorf("readFile marker = %v, want %v", tolerant.ThemeMigrated, c.want)
			}

			strict, existed, err := store.readFileStrict()
			if err != nil {
				t.Fatalf("unexpected readFileStrict error: %v — a hand-edited marker must never abort a write", err)
			}
			if !existed {
				t.Errorf("readFileStrict reports the file absent, want present")
			}
			if bool(strict.ThemeMigrated) != c.want {
				t.Errorf("readFileStrict marker = %v, want %v", strict.ThemeMigrated, c.want)
			}

			// And the consequence that matters: the value does not abort the
			// next write, which under task 6-1's rule is what a decode error
			// here would do to EVERY write from then on.
			if err := store.Save(ModeByProject); err != nil {
				t.Fatalf("unexpected Save error: %v — a hand-edited marker must not lock the user out of their own prefs", err)
			}
			assertMarkerOnDisk(t, store.path, c.want)
		})
	}
}

// TestMigrationMarker_WrongTypeDoesNotZeroTheRecord is the case the marker's own
// decode exists for. `"theme_migrated": "yes"` is the hand-edit §8.1 anticipates,
// and a field-level decode that ERRORED on it would take the whole struct with
// it: the tolerant load would collapse to a zero record (losing the mode, the
// theme keys and the retained appearance), and under task 6-1's rule the strict
// re-read would abort EVERY subsequent write — locking the user out of their own
// prefs over one hand-typed word.
func TestMigrationMarker_WrongTypeDoesNotZeroTheRecord(t *testing.T) {
	const content = `{"session_list_mode":"by-tag","appearance":"sepia","theme":"nord","theme_light":"tokyo-night-day","theme_dark":"tokyo-night","theme_migrated":"yes"}`

	wantKeys := ThemeKeys{Theme: "nord", Light: "tokyo-night-day", Dark: "tokyo-night"}

	t.Run("the tolerant load path still yields every other field", func(t *testing.T) {
		store := NewStore(seedPrefsFile(t, content))

		mode, err := store.Load()
		if err != nil {
			t.Fatalf("unexpected Load error: %v", err)
		}
		if mode != ModeByTag {
			t.Errorf("mode = %v, want ModeByTag — the record was zeroed by the marker", mode)
		}

		keys, err := store.LoadThemeKeys()
		if err != nil {
			t.Fatalf("unexpected LoadThemeKeys error: %v", err)
		}
		if keys != wantKeys {
			t.Errorf("theme keys = %+v, want %+v — the record was zeroed by the marker", keys, wantKeys)
		}
	})

	t.Run("the strict write path still passes the file as usable", func(t *testing.T) {
		f, existed, err := NewStore(seedPrefsFile(t, content)).readFileStrict()
		if err != nil {
			t.Fatalf("unexpected readFileStrict error: %v — a wrong-typed marker is a value problem, not a syntax one", err)
		}
		if !existed {
			t.Errorf("readFileStrict reports the file absent, want present")
		}
		if f.SessionListMode != modeByTagString {
			t.Errorf("strict record session_list_mode = %q, want %q", f.SessionListMode, modeByTagString)
		}
	})

	t.Run("a subsequent write proceeds and preserves the record", func(t *testing.T) {
		path := seedPrefsFile(t, content)

		if err := NewStore(path).Save(ModeByProject); err != nil {
			t.Fatalf("unexpected Save error: %v — a wrong-typed marker must not abort a write", err)
		}

		decoded := decodeWritten(t, path)
		assertWrittenValue(t, decoded, "session_list_mode", "by-project")
		assertWrittenValue(t, decoded, "appearance", "sepia")
		assertWrittenValue(t, decoded, "theme", "nord")
		assertWrittenValue(t, decoded, "theme_light", "tokyo-night-day")
		assertWrittenValue(t, decoded, "theme_dark", "tokyo-night")
		// The unrecognised value normalises away on re-encode, which is §8.1's
		// tolerant absorption: the next launch simply re-runs an idempotent
		// translation.
		assertMarkerOnDisk(t, path, false)
	})
}

// TestMigrationMarker_NotTouchedByThemeSavers pins §8.1's "never participates in
// mutual exclusion" in BOTH directions, which is the property §10.3 exists to
// guarantee against a re-armable absence gate. A theme saver must not CLEAR a
// true marker (that re-arms the one-shot translation, so a user who hand-deletes
// their theme keys per §9.9's escape hatch is silently re-translated and
// re-pinned), and must not SET a false one (that records a translation which
// never ran, suppressing it forever).
func TestMigrationMarker_NotTouchedByThemeSavers(t *testing.T) {
	t.Run("a true marker survives a theme saver", func(t *testing.T) {
		for _, c := range themeSaverCases() {
			t.Run(c.name, func(t *testing.T) {
				path := seedPrefsFile(t, `{"theme":"gruvbox","theme_light":"tokyo-night-day","theme_dark":"tokyo-night","theme_migrated":true}`)

				if err := c.save(NewStore(path), "nord"); err != nil {
					t.Fatalf("unexpected save error: %v", err)
				}

				// The saver's own key proves it actually ran, so the marker
				// assertion below is not vacuous.
				assertWrittenValue(t, decodeWritten(t, path), c.writtenKey, "nord")
				assertMarkerOnDisk(t, path, true)
			})
		}
	})

	t.Run("a theme saver never sets the marker", func(t *testing.T) {
		for _, c := range themeSaverCases() {
			t.Run(c.name, func(t *testing.T) {
				path := seedPrefsFile(t, `{"session_list_mode":"flat"}`)

				if err := c.save(NewStore(path), "nord"); err != nil {
					t.Fatalf("unexpected save error: %v", err)
				}

				assertMarkerOnDisk(t, path, false)
			})
		}
	})

	t.Run("clearing the theme keys by hand does not clear the marker", func(t *testing.T) {
		// §9.9's documented escape hatch: the user hand-deletes their theme keys
		// to return to the shipped adaptive pair. The marker is orthogonal, so
		// the translation stays spent and Portal does not reinstate what they
		// just undid.
		path := seedPrefsFile(t, `{"session_list_mode":"flat","appearance":"dark","theme_migrated":true}`)

		if err := NewStore(path).Save(ModeByTag); err != nil {
			t.Fatalf("unexpected Save error: %v", err)
		}

		decoded := decodeWritten(t, path)
		assertKeysAbsent(t, decoded, "theme", "theme_light", "theme_dark")
		assertWrittenValue(t, decoded, "appearance", "dark")
		assertMarkerOnDisk(t, path, true)
	})
}

// TestSaveMigrationMarker_PreservesEveryOtherField pins that the marker save
// writes ONLY the marker. It is a field-specific saver like every other in this
// store (§8.9), so all three theme keys, session_list_mode and the retained raw
// appearance round-trip through it untouched.
//
// The seed is deliberately the hand-edited both-present file of §8.2 — a
// constant AND both slots. The marker never participates in mutual exclusion
// (§8.1), so this save prunes nothing: it is not a theme write and has no
// business enforcing a theme rule.
func TestSaveMigrationMarker_PreservesEveryOtherField(t *testing.T) {
	const seeded = `{"session_list_mode":"by-tag","appearance":"sepia","theme":"gruvbox","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`
	path := seedPrefsFile(t, seeded)

	if err := NewStore(path).SaveMigrationMarker(); err != nil {
		t.Fatalf("unexpected SaveMigrationMarker error: %v", err)
	}

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
	assertWrittenValue(t, decoded, "appearance", "sepia")
	assertWrittenValue(t, decoded, "theme", "gruvbox")
	assertWrittenValue(t, decoded, "theme_light", "tokyo-night-day")
	assertWrittenValue(t, decoded, "theme_dark", "tokyo-night")
	assertMarkerOnDisk(t, path, true)

	// The raw appearance is what §10.3's gate reads back, so the round-trip
	// through the accessor is the property the translation actually depends on.
	got, err := NewStore(path).LoadMigrationState()
	if err != nil {
		t.Fatalf("unexpected LoadMigrationState error: %v", err)
	}
	want := MigrationState{Appearance: "sepia", Migrated: true}
	if got != want {
		t.Errorf("migration state = %+v, want %+v", got, want)
	}
}

// TestSaveMigrationMarker_DoesNotCreateAbsentFile pins the half of task 6-1's
// write-path rule this saver deliberately does NOT inherit. Every other saver
// creates an absent prefs.json; this one declines, because a fresh install has
// no appearance to translate and creating the file purely to record a marker
// would add a side effect to a path this feature otherwise leaves free (§8.1,
// and the same restraint §5.5 shows by refusing to create the themes directory).
//
// Re-evaluating next launch costs an absent-field check on a read that is
// already happening, and the file appears the moment the user changes anything.
func TestSaveMigrationMarker_DoesNotCreateAbsentFile(t *testing.T) {
	cases := []struct {
		name string
		rel  []string
	}{
		{name: "the file is absent", rel: []string{"prefs.json"}},
		{name: "the parent directory is absent too", rel: []string{"sub", "nested", "prefs.json"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(append([]string{dir}, c.rel...)...)

			if err := NewStore(path).SaveMigrationMarker(); err != nil {
				t.Fatalf("unexpected SaveMigrationMarker error: %v — declining to write is not a failure", err)
			}

			if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("os.Stat error = %v, want the file still absent", err)
			}
			// Nothing at all is created — not the file, not the tree above it.
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("failed to read temp dir: %v", err)
			}
			if len(entries) != 0 {
				t.Errorf("temp dir contains %d entries, want none — the save must create nothing", len(entries))
			}
			assertNoTempFiles(t, dir)
		})
	}
}

// TestSaveMigrationMarker_AbortsOnUndecodable pins the half of task 6-1's rule
// this saver DOES inherit (§8.9): a re-read that does not decode aborts rather
// than overwrites, so the bytes stay byte-identical. Merging into the tolerant
// decode's zero-valued record would erase session_list_mode, every theme key and
// the retained raw appearance to record one boolean.
//
// The saver has no reporting surface — it runs at prefs load, before any panel
// exists — so its failure signal is the absence of the migration event, and it
// simply retries next launch (§10.5). prefs is a leaf, so the abort is reported
// by returning.
func TestSaveMigrationMarker_AbortsOnUndecodable(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "a truncated object", content: `{`},
		{name: "a trailing comma", content: `{"session_list_mode":"flat",}`},
		{name: "junk that is not JSON at all", content: `not json`},
		{name: "a zero-byte file", content: ``},
		{name: "a top-level array", content: `[1,2]`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, c.content)
			before := readRaw(t, path)

			if err := NewStore(path).SaveMigrationMarker(); err == nil {
				t.Fatalf("SaveMigrationMarker returned nil, want an abort error")
			}

			assertUntouched(t, path, before)
		})
	}
}

// TestSaveMigrationMarker_AbsenceJudgedAtReRead pins WHERE absence is judged:
// at the same RMW re-read as everything else, never against a stat taken at
// load. prefs.json can appear in between — another instance's first commit is
// the ordinary case, and Portal's multi-window burst makes concurrent instances
// normal (§8.9) — and a save that trusted a load-time snapshot would silently
// decline forever on a file that now exists.
func TestSaveMigrationMarker_AbsenceJudgedAtReRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")

	// This instance is constructed while the file is genuinely absent...
	store := NewStore(path)
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat error = %v, want the file absent before the other writer commits", err)
	}

	// ...another instance's first commit creates it in between...
	if err := NewStore(path).Save(ModeByTag); err != nil {
		t.Fatalf("unexpected Save error from the other writer: %v", err)
	}

	// ...and the marker save must observe the file that now exists.
	if err := store.SaveMigrationMarker(); err != nil {
		t.Fatalf("unexpected SaveMigrationMarker error: %v", err)
	}

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
	assertMarkerOnDisk(t, path, true)
}

// TestLoadMigrationState_ReturnsRawAppearanceUnparsed pins the accessor's one
// job: hand back §10.3's gate inputs — the retained raw appearance and whether
// the translation is already recorded — in a single read.
//
// The appearance value is returned VERBATIM and unparsed. §8.8 killed the
// Appearance enum with its last caller and prefs must not regrow one: the
// dark/light/auto mapping belongs to the translation in cmd/config.go, which is
// also the only thing in the new binary that looks at the value at all (§10.4 —
// it is a frozen legacy value kept for a downgraded binary).
func TestLoadMigrationState_ReturnsRawAppearanceUnparsed(t *testing.T) {
	raws := []struct {
		name string
		raw  string
	}{
		{name: "a pinned dark", raw: "dark"},
		{name: "a pinned light", raw: "light"},
		{name: "auto", raw: "auto"},
		{name: "wrong case", raw: "Dark"},
		{name: "leading and trailing space", raw: "  dark "},
		{name: "an unrecognised value", raw: "sepia"},
		{name: "an empty string", raw: ""},
	}

	for _, r := range raws {
		for _, migrated := range []bool{false, true} {
			name := r.name
			if migrated {
				name += " alongside a recorded marker"
			}

			t.Run(name, func(t *testing.T) {
				content, err := json.Marshal(map[string]any{
					"session_list_mode": "by-tag",
					"appearance":        r.raw,
					"theme_migrated":    migrated,
				})
				if err != nil {
					t.Fatalf("failed to marshal test content: %v", err)
				}

				got, err := NewStore(seedPrefsFile(t, string(content))).LoadMigrationState()
				if err != nil {
					t.Fatalf("unexpected LoadMigrationState error: %v", err)
				}

				want := MigrationState{Appearance: r.raw, Migrated: migrated}
				if got != want {
					t.Errorf("migration state = %+v, want %+v verbatim", got, want)
				}
			})
		}
	}
}

// TestLoadMigrationState_TolerantDecode pins that the accessor inherits today's
// LOAD policy verbatim rather than inventing one: it reads through the same
// tolerant decode as Load and LoadThemeKeys, so every degenerate file yields a
// zero value with no error and only a non-ErrNotExist read error propagates.
//
// A zero value is the safe direction here — no appearance to translate and an
// unrecorded marker means the translation re-evaluates next launch, which is
// idempotent.
func TestLoadMigrationState_TolerantDecode(t *testing.T) {
	cases := []struct {
		name    string
		missing bool
		content string
	}{
		{name: "a missing file", missing: true},
		{name: "an empty file", content: ""},
		{name: "corrupt unparseable JSON", content: "{invalid json!!!"},
		{name: "an empty object", content: `{}`},
		{name: "valid JSON carrying neither key", content: `{"session_list_mode":"by-tag","theme":"nord"}`},
		{name: "the keys present but empty", content: `{"appearance":"","theme_migrated":false}`},
		{name: "a top-level array", content: `[1,2]`},
		{name: "a top-level string", content: `"x"`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nonexistent", "prefs.json")
			if !c.missing {
				path = seedPrefsFile(t, c.content)
			}

			got, err := NewStore(path).LoadMigrationState()
			if err != nil {
				t.Fatalf("unexpected LoadMigrationState error: %v", err)
			}
			if got != (MigrationState{}) {
				t.Errorf("migration state = %+v, want a zero MigrationState", got)
			}
		})
	}

	t.Run("a non-ErrNotExist read error propagates", func(t *testing.T) {
		// A directory at the prefs path makes os.ReadFile fail with EISDIR.
		path := filepath.Join(t.TempDir(), "prefs.json")
		if err := os.Mkdir(path, 0o755); err != nil {
			t.Fatalf("failed to create dir at prefs path: %v", err)
		}

		got, err := NewStore(path).LoadMigrationState()
		if err == nil {
			t.Fatalf("expected a read error, got nil")
		}
		if got != (MigrationState{}) {
			t.Errorf("migration state = %+v, want a zero MigrationState alongside the error", got)
		}
	})
}
