// White-box: "one atomic write per save" is not observable from the
// filesystem, so it is counted at the unexported atomicWrite seam.
package prefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// Asserted structurally: a "theme" substring would also match "theme_light".
func assertKeysAbsent(t *testing.T, decoded map[string]any, keys ...string) {
	t.Helper()

	for _, key := range keys {
		if value, ok := decoded[key]; ok {
			t.Errorf("written JSON carries %q = %#v, want the key absent", key, value)
		}
	}
}

func TestSaveTheme_ClearsBothSlots(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{
			name:    "a file holding the adaptive pair",
			content: `{"session_list_mode":"flat","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`,
		},
		{
			name:    "a hand-edited file holding a constant and both slots",
			content: `{"theme":"gruvbox","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`,
		},
		{
			name:    "a file holding only one slot",
			content: `{"theme_dark":"tokyo-night"}`,
		},
		{
			name:    "a file holding a constant alone",
			content: `{"theme":"gruvbox"}`,
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, c.content)
			store := NewStore(path)

			if err := store.SaveTheme("nord"); err != nil {
				t.Fatalf("unexpected SaveTheme error: %v", err)
			}

			decoded := decodeWritten(t, path)
			assertWrittenValue(t, decoded, "theme", "nord")
			assertKeysAbsent(t, decoded, "theme_light", "theme_dark")
		})
	}
}

func TestSaveThemeSlot_ClearsConstant(t *testing.T) {
	path := seedPrefsFile(t, `{"theme":"gruvbox","theme_light":"tokyo-night-day"}`)
	store := NewStore(path)

	if err := store.SaveThemeSlot("nord", SlotDark); err != nil {
		t.Fatalf("unexpected SaveThemeSlot error: %v", err)
	}

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "theme_dark", "nord")
	assertWrittenValue(t, decoded, "theme_light", "tokyo-night-day")
	assertKeysAbsent(t, decoded, "theme")
}

func TestSaveThemeSlot_OtherSlotUnaffected(t *testing.T) {
	cases := []struct {
		name      string
		slot      ThemeSlot
		wantKey   string
		otherKey  string
		otherWant string
	}{
		{
			name:      "assigning dark leaves light exactly as it was",
			slot:      SlotDark,
			wantKey:   "theme_dark",
			otherKey:  "theme_light",
			otherWant: "tokyo-night-day",
		},
		{
			name:      "assigning light leaves dark exactly as it was",
			slot:      SlotLight,
			wantKey:   "theme_light",
			otherKey:  "theme_dark",
			otherWant: "tokyo-night",
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, `{"theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`)
			store := NewStore(path)

			if err := store.SaveThemeSlot("nord", c.slot); err != nil {
				t.Fatalf("unexpected SaveThemeSlot error: %v", err)
			}

			decoded := decodeWritten(t, path)
			assertWrittenValue(t, decoded, c.wantKey, "nord")
			assertWrittenValue(t, decoded, c.otherKey, c.otherWant)
		})
	}
}

func TestSaveThemeSlot_LightThenDarkYieldsBoth(t *testing.T) {
	path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":"gruvbox"}`)
	store := NewStore(path)

	if err := store.SaveThemeSlot("nord", SlotLight); err != nil {
		t.Fatalf("unexpected SaveThemeSlot(light) error: %v", err)
	}
	if err := store.SaveThemeSlot("nord", SlotDark); err != nil {
		t.Fatalf("unexpected SaveThemeSlot(dark) error: %v", err)
	}

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "theme_light", "nord")
	assertWrittenValue(t, decoded, "theme_dark", "nord")
	assertKeysAbsent(t, decoded, "theme")
}

func recordWrites(t *testing.T) func() [][]byte {
	t.Helper()

	var commits [][]byte
	previous := atomicWrite
	atomicWrite = func(path string, data []byte) error {
		commits = append(commits, bytes.Clone(data))
		return previous(path, data)
	}
	t.Cleanup(func() { atomicWrite = previous })

	return func() [][]byte { return commits }
}

// Covers both savers despite the name; a second write leaves no trace on the
// filesystem, so the count is taken at the seam.
func TestSaveTheme_SingleAtomicWrite(t *testing.T) {
	for _, c := range themeSaverCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":"gruvbox","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`)
			commits := recordWrites(t)

			if err := c.save(NewStore(path), "nord"); err != nil {
				t.Fatalf("unexpected save error: %v", err)
			}

			made := commits()
			if len(made) != 1 {
				t.Fatalf("AtomicWrite called %d times, want exactly 1 — the commit and its clear must land together", len(made))
			}

			var committed map[string]any
			if err := json.Unmarshal(made[0], &committed); err != nil {
				t.Fatalf("the committed bytes are not valid JSON (%q): %v", made[0], err)
			}
			assertWrittenValue(t, committed, c.writtenKey, "nord")
			assertKeysAbsent(t, committed, c.clearedKeys...)
		})
	}
}

func TestThemeSavers_PreserveUnrelatedFields(t *testing.T) {
	// The seeded appearance is deliberately unrecognised: never parsed.
	const seeded = `{"session_list_mode":"by-tag","appearance":"sepia","theme_light":"tokyo-night-day"}`

	for _, c := range themeSaverCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, seeded)

			if err := c.save(NewStore(path), "nord"); err != nil {
				t.Fatalf("unexpected save error: %v", err)
			}

			decoded := decodeWritten(t, path)
			assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
			assertWrittenValue(t, decoded, "appearance", "sepia")
			assertKeysAbsent(t, decoded, "theme_migrated")
		})
	}
}

// The rendered content differs from the bare flat seed only in the given stale
// keys, so byte comparisons isolate the clear.
func seedWithStaleKeys(t *testing.T, keys []string) string {
	t.Helper()

	content := map[string]string{"session_list_mode": "flat"}
	for _, key := range keys {
		content[key] = "gruvbox"
	}

	encoded, err := json.Marshal(content)
	if err != nil {
		t.Fatalf("failed to marshal test content: %v", err)
	}
	return string(encoded)
}

func TestThemeSavers_ClearedKeysAreAbsent(t *testing.T) {
	t.Run("a cleared key is omitted, never written as an empty string", func(t *testing.T) {
		for _, c := range themeSaverCases() {
			t.Run(c.name, func(t *testing.T) {
				path := seedPrefsFile(t, `{"theme":"gruvbox","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`)

				if err := c.save(NewStore(path), "nord"); err != nil {
					t.Fatalf("unexpected save error: %v", err)
				}

				assertKeysAbsent(t, decodeWritten(t, path), c.clearedKeys...)

				// The trailing `":` anchors the needle so `"theme"` does not
				// match `"theme_light"`.
				raw := readRaw(t, path)
				for _, key := range c.clearedKeys {
					if needle := []byte(`"` + key + `":`); bytes.Contains(raw, needle) {
						t.Errorf("written JSON %q still names %q, want the cleared key omitted", raw, key)
					}
				}
			})
		}
	})

	t.Run("clearing an already-absent key is not an error and lands the same bytes", func(t *testing.T) {
		for _, c := range themeSaverCases() {
			t.Run(c.name, func(t *testing.T) {
				nothingToClear := seedPrefsFile(t, `{"session_list_mode":"flat"}`)
				somethingToClear := seedPrefsFile(t, seedWithStaleKeys(t, c.clearedKeys))

				for _, path := range []string{nothingToClear, somethingToClear} {
					if err := c.save(NewStore(path), "nord"); err != nil {
						t.Fatalf("unexpected save error: %v", err)
					}
				}

				assertWrittenValue(t, decodeWritten(t, nothingToClear), c.writtenKey, "nord")

				got, want := readRaw(t, nothingToClear), readRaw(t, somethingToClear)
				if !bytes.Equal(got, want) {
					t.Errorf("clearing absent keys wrote %q, want %q — byte-identical to clearing present ones", got, want)
				}
			})
		}
	})
}

func TestThemeSavers_RepeatedCommitIsByteIdentical(t *testing.T) {
	for _, c := range themeSaverCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, `{"session_list_mode":"flat","appearance":"dark","theme":"gruvbox","theme_light":"tokyo-night-day"}`)
			store := NewStore(path)

			var first []byte
			for i := range 3 {
				if err := c.save(store, "nord"); err != nil {
					t.Fatalf("unexpected save error on commit %d: %v", i+1, err)
				}

				data := readRaw(t, path)
				if i == 0 {
					first = data
					continue
				}
				if !bytes.Equal(data, first) {
					t.Fatalf("commit %d rewrote the file: got %q, want %q byte-identical", i+1, data, first)
				}
			}
		})
	}
}

func TestThemeSavers_InheritWritePathRules(t *testing.T) {
	t.Run("create-on-absent", func(t *testing.T) {
		for _, c := range themeSaverCases() {
			t.Run(c.name, func(t *testing.T) {
				path := filepath.Join(t.TempDir(), "sub", "prefs.json")

				if err := c.save(NewStore(path), "nord"); err != nil {
					t.Fatalf("unexpected save error: %v", err)
				}

				assertWrittenValue(t, decodeWritten(t, path), c.writtenKey, "nord")
				assertNoTempFiles(t, filepath.Dir(path))
			})
		}
	})

	t.Run("abort-on-undecodable", func(t *testing.T) {
		for _, saver := range themeSaverCases() {
			t.Run(saver.name, func(t *testing.T) {
				for _, c := range undecodablePrefsCases() {
					t.Run(c.name, func(t *testing.T) {
						path := seedPrefsFile(t, c.content)
						before := readRaw(t, path)

						err := saver.save(NewStore(path), "nord")
						if err == nil {
							t.Fatalf("save returned nil, want an abort error")
						}
						c.assertErr(t, err)

						assertUntouched(t, path, before)
					})
				}
			})
		}
	})
}

func TestThemeSavers_RMWDoesNotLoseAnotherWritersField(t *testing.T) {
	for _, c := range themeSaverCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, `{"session_list_mode":"flat"}`)

			instanceA := NewStore(path)
			if err := NewStore(path).Save(ModeByTag); err != nil {
				t.Fatalf("unexpected Save error from the other writer: %v", err)
			}
			if err := c.save(instanceA, "nord"); err != nil {
				t.Fatalf("unexpected save error: %v", err)
			}

			decoded := decodeWritten(t, path)
			assertWrittenValue(t, decoded, c.writtenKey, "nord")
			assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
		})
	}
}

func TestSaveThemeSlot_InvalidSlotWritesNothing(t *testing.T) {
	cases := []struct {
		name    string
		slot    ThemeSlot
		wantErr string
	}{
		{name: "the zero value", slot: ThemeSlot(0), wantErr: "prefs: invalid theme slot 0"},
		{name: "one past the pair", slot: ThemeSlot(3), wantErr: "prefs: invalid theme slot 3"},
		{name: "a negative value", slot: ThemeSlot(-1), wantErr: "prefs: invalid theme slot -1"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Run("a present file is left byte-identical", func(t *testing.T) {
				path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":"gruvbox"}`)
				before := readRaw(t, path)

				err := NewStore(path).SaveThemeSlot("nord", c.slot)
				if err == nil {
					t.Fatalf("SaveThemeSlot returned nil, want an error naming the invalid slot")
				}
				if err.Error() != c.wantErr {
					t.Errorf("error = %q, want %q", err, c.wantErr)
				}

				assertUntouched(t, path, before)
			})

			t.Run("an absent file is not created", func(t *testing.T) {
				dir := t.TempDir()
				path := filepath.Join(dir, "prefs.json")

				if err := NewStore(path).SaveThemeSlot("nord", c.slot); err == nil {
					t.Fatalf("SaveThemeSlot returned nil, want an error naming the invalid slot")
				}

				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Errorf("os.Stat error = %v, want the file still absent", err)
				}
				assertNoTempFiles(t, dir)
			})
		})
	}
}

func TestThemeSavers_NoSlugKnowledge(t *testing.T) {
	slugs := []struct {
		name string
		raw  string
	}{
		{name: "path traversal", raw: "../evil"},
		{name: "uppercase", raw: "Nord"},
		{name: "leading space", raw: "  nord"},
		{name: "embedded tab", raw: "no\trd"},
	}

	for _, c := range themeSaverCases() {
		for _, slug := range slugs {
			t.Run(c.name+" "+slug.name, func(t *testing.T) {
				path := seedPrefsFile(t, `{"session_list_mode":"flat"}`)

				if err := c.save(NewStore(path), slug.raw); err != nil {
					t.Fatalf("unexpected save error: %v", err)
				}

				assertWrittenValue(t, decodeWritten(t, path), c.writtenKey, slug.raw)
			})
		}
	}
}

type themeSaverCase struct {
	name        string
	writtenKey  string
	clearedKeys []string
	save        func(store *Store, slug string) error
}

func themeSaverCases() []themeSaverCase {
	return []themeSaverCase{
		{
			name:        "SaveTheme",
			writtenKey:  "theme",
			clearedKeys: []string{"theme_light", "theme_dark"},
			save:        func(store *Store, slug string) error { return store.SaveTheme(slug) },
		},
		{
			name:        "SaveThemeSlot light",
			writtenKey:  "theme_light",
			clearedKeys: []string{"theme"},
			save:        func(store *Store, slug string) error { return store.SaveThemeSlot(slug, SlotLight) },
		},
		{
			name:        "SaveThemeSlot dark",
			writtenKey:  "theme_dark",
			clearedKeys: []string{"theme"},
			save:        func(store *Store, slug string) error { return store.SaveThemeSlot(slug, SlotDark) },
		},
	}
}
