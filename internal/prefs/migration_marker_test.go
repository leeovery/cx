// White-box: the marker's decode and the mutate seam are unexported.
package prefs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func assertMarkerOnDisk(t *testing.T, path string, want bool) {
	t.Helper()

	assertMarkerValue(t, decodeWritten(t, path), want)
}

// A false marker is absent on disk, so presence is asserted structurally — a
// substring check could not tell `false` from omitted.
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

// One writer failing to round-trip the marker is enough to erase it on
// re-encode, so every writer is driven.
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

// False fails safe: the translation is idempotent and simply re-runs.
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

			if err := store.Save(ModeByProject); err != nil {
				t.Fatalf("unexpected Save error: %v — a hand-edited marker must not lock the user out of their own prefs", err)
			}
			assertMarkerOnDisk(t, store.path, c.want)
		})
	}
}

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
		// The unrecognised value normalises away on re-encode.
		assertMarkerOnDisk(t, path, false)
	})
}

func TestMigrationMarker_NotTouchedByThemeSavers(t *testing.T) {
	t.Run("a true marker survives a theme saver", func(t *testing.T) {
		for _, c := range themeSaverCases() {
			t.Run(c.name, func(t *testing.T) {
				path := seedPrefsFile(t, `{"theme":"gruvbox","theme_light":"tokyo-night-day","theme_dark":"tokyo-night","theme_migrated":true}`)

				if err := c.save(NewStore(path), "nord"); err != nil {
					t.Fatalf("unexpected save error: %v", err)
				}

				// The saver's own key proves it ran, so the marker assertion
				// is not vacuous.
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

// The seed deliberately holds both a constant and the pair: the marker save is
// not a theme write, so it prunes nothing.
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

	got, err := NewStore(path).LoadMigrationState()
	if err != nil {
		t.Fatalf("unexpected LoadMigrationState error: %v", err)
	}
	want := MigrationState{Appearance: "sepia", Migrated: true}
	if got != want {
		t.Errorf("migration state = %+v, want %+v", got, want)
	}
}

func TestSaveMigrationMarker_DoesNotCreateAbsentFile(t *testing.T) {
	for _, c := range absentPathCases() {
		t.Run(c.name, func(t *testing.T) {
			dir := t.TempDir()
			path := c.path(dir)

			if err := NewStore(path).SaveMigrationMarker(); err != nil {
				t.Fatalf("unexpected SaveMigrationMarker error: %v — declining to write is not a failure", err)
			}

			assertNothingCreated(t, dir, path)
		})
	}
}

func TestSaveMigrationMarker_AbortsOnUndecodable(t *testing.T) {
	for _, c := range undecodablePrefsCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, c.content)
			before := readRaw(t, path)

			err := NewStore(path).SaveMigrationMarker()
			if err == nil {
				t.Fatalf("SaveMigrationMarker returned nil, want an abort error")
			}
			c.assertErr(t, err)

			assertUntouched(t, path, before)
		})
	}
}

func TestSaveMigrationMarker_AbsenceJudgedAtReRead(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")

	store := NewStore(path)
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("os.Stat error = %v, want the file absent before the other writer commits", err)
	}

	if err := NewStore(path).Save(ModeByTag); err != nil {
		t.Fatalf("unexpected Save error from the other writer: %v", err)
	}

	if err := store.SaveMigrationMarker(); err != nil {
		t.Fatalf("unexpected SaveMigrationMarker error: %v", err)
	}

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
	assertMarkerOnDisk(t, path, true)
}

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
