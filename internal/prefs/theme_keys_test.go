package prefs_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/prefs"
)

// writePrefsFile writes content to a prefs.json inside a fresh temp dir and
// returns its path.
func writePrefsFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test prefs file: %v", err)
	}
	return path
}

// decodePrefsFile reads the on-disk prefs.json into a generic map so key
// presence/absence is asserted structurally rather than by substring — a
// `"theme"` substring also matches `"theme_light"`, which would make an
// omitempty assertion silently vacuous.
func decodePrefsFile(t *testing.T, path string) map[string]any {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read prefs file: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("prefs file is not valid JSON (%q): %v", data, err)
	}
	return decoded
}

// assertPrefsValue asserts an on-disk key holds exactly want.
func assertPrefsValue(t *testing.T, decoded map[string]any, key, want string) {
	t.Helper()

	got, ok := decoded[key]
	if !ok {
		t.Errorf("written JSON has no %q key, want %q", key, want)
		return
	}
	if got != want {
		t.Errorf("written JSON %q = %v, want %q", key, got, want)
	}
}

func TestLoadThemeKeys_DecodesAllThree(t *testing.T) {
	cases := []struct {
		name    string
		content string
		want    prefs.ThemeKeys
	}{
		{
			name:    "a constant theme alone",
			content: `{"theme":"nord"}`,
			want:    prefs.ThemeKeys{Theme: "nord"},
		},
		{
			name:    "the adaptive slots alone",
			content: `{"theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`,
			want:    prefs.ThemeKeys{Light: "tokyo-night-day", Dark: "tokyo-night"},
		},
		{
			name:    "all three alongside the pre-existing keys",
			content: `{"session_list_mode":"by-tag","appearance":"dark","theme":"nord","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`,
			want:    prefs.ThemeKeys{Theme: "nord", Light: "tokyo-night-day", Dark: "tokyo-night"},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			store := prefs.NewStore(writePrefsFile(t, c.content))

			got, err := store.LoadThemeKeys()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("theme keys = %+v, want %+v", got, c.want)
			}
		})
	}
}

func TestLoadThemeKeys_TolerantDecode(t *testing.T) {
	cases := []struct {
		name    string
		missing bool
		content string
	}{
		{name: "a missing file", missing: true},
		{name: "an empty file", content: ""},
		{name: "corrupt unparseable JSON", content: "{invalid json!!!"},
		{name: "valid JSON carrying none of the three keys", content: `{"session_list_mode":"by-tag"}`},
		{name: "an empty object", content: `{}`},
		{name: "the keys present but empty-string", content: `{"theme":"","theme_light":"","theme_dark":""}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "nonexistent", "prefs.json")
			if !c.missing {
				path = writePrefsFile(t, c.content)
			}
			store := prefs.NewStore(path)

			got, err := store.LoadThemeKeys()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != (prefs.ThemeKeys{}) {
				t.Errorf("theme keys = %+v, want a zero ThemeKeys", got)
			}
		})
	}
}

func TestLoadThemeKeys_PropagatesReadError(t *testing.T) {
	// A directory at the prefs path makes os.ReadFile fail with a non-ErrNotExist
	// error (EISDIR), exercising the propagated-error branch.
	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := os.Mkdir(path, 0o755); err != nil {
		t.Fatalf("failed to create dir at prefs path: %v", err)
	}
	store := prefs.NewStore(path)

	got, err := store.LoadThemeKeys()
	if err == nil {
		t.Fatalf("expected a read error, got nil")
	}
	if got != (prefs.ThemeKeys{}) {
		t.Errorf("theme keys = %+v, want a zero ThemeKeys alongside the error", got)
	}
}

func TestLoadThemeKeys_NoValidationOrNormalisation(t *testing.T) {
	// Decode never validates, trims or lowercases: an unrecognised value is a
	// resolution problem, not a decode one, and trimming would turn a stray-space
	// value into a silently-different slug instead of an honest `bad name`.
	cases := []struct {
		name string
		raw  string
	}{
		{name: "uppercase", raw: "Nord"},
		{name: "path traversal", raw: "../evil"},
		{name: "leading and trailing space", raw: "  nord "},
		{name: "embedded tab", raw: "no\trd"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			content, err := json.Marshal(map[string]string{
				"theme":       c.raw,
				"theme_light": c.raw,
				"theme_dark":  c.raw,
			})
			if err != nil {
				t.Fatalf("failed to marshal test content: %v", err)
			}
			store := prefs.NewStore(writePrefsFile(t, string(content)))

			got, err := store.LoadThemeKeys()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			want := prefs.ThemeKeys{Theme: c.raw, Light: c.raw, Dark: c.raw}
			if got != want {
				t.Errorf("theme keys = %+v, want %+v verbatim", got, want)
			}
		})
	}
}

func TestSave_PreservesThemeKeys(t *testing.T) {
	const seeded = `{"session_list_mode":"flat","appearance":"dark","theme":"nord","theme_light":"tokyo-night-day","theme_dark":"tokyo-night"}`
	path := writePrefsFile(t, seeded)
	store := prefs.NewStore(path)

	if err := store.Save(prefs.ModeByTag); err != nil {
		t.Fatalf("unexpected Save error: %v", err)
	}

	got, err := store.LoadThemeKeys()
	if err != nil {
		t.Fatalf("unexpected LoadThemeKeys error: %v", err)
	}
	want := prefs.ThemeKeys{Theme: "nord", Light: "tokyo-night-day", Dark: "tokyo-night"}
	if got != want {
		t.Errorf("theme keys = %+v, want %+v", got, want)
	}

	decoded := decodePrefsFile(t, path)
	assertPrefsValue(t, decoded, "theme", "nord")
	assertPrefsValue(t, decoded, "theme_light", "tokyo-night-day")
	assertPrefsValue(t, decoded, "theme_dark", "tokyo-night")
	assertPrefsValue(t, decoded, "appearance", "dark")
	assertPrefsValue(t, decoded, "session_list_mode", "by-tag")
}

func TestSave_PreservesRawAppearance(t *testing.T) {
	t.Run("an appearance pin survives ten successive saves byte-identically", func(t *testing.T) {
		path := writePrefsFile(t, `{"session_list_mode":"flat","appearance":"dark"}`)
		store := prefs.NewStore(path)

		var first []byte
		for i := range 10 {
			if err := store.Save(prefs.ModeByTag); err != nil {
				t.Fatalf("unexpected Save error on save %d: %v", i+1, err)
			}
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("failed to read prefs file after save %d: %v", i+1, err)
			}
			if i == 0 {
				first = data
				continue
			}
			if !bytes.Equal(data, first) {
				t.Fatalf("save %d rewrote the file: got %q, want %q", i+1, data, first)
			}
		}

		assertPrefsValue(t, decodePrefsFile(t, path), "appearance", "dark")
	})

	t.Run("an unrecognised appearance value is preserved verbatim, never parsed", func(t *testing.T) {
		path := writePrefsFile(t, `{"appearance":"sepia"}`)
		store := prefs.NewStore(path)

		if err := store.Save(prefs.ModeByProject); err != nil {
			t.Fatalf("unexpected Save error: %v", err)
		}

		assertPrefsValue(t, decodePrefsFile(t, path), "appearance", "sepia")
	})
}

func TestSave_OmitsEmptyThemeKeysAndAppearance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")
	store := prefs.NewStore(path)

	if err := store.Save(prefs.ModeByTag); err != nil {
		t.Fatalf("unexpected Save error: %v", err)
	}

	decoded := decodePrefsFile(t, path)
	for _, key := range []string{"theme", "theme_light", "theme_dark", "appearance"} {
		if value, ok := decoded[key]; ok {
			t.Errorf("written JSON carries %q = %v, want the key absent", key, value)
		}
	}
	assertPrefsValue(t, decoded, "session_list_mode", "by-tag")
}

func TestPrefsFile_DeclaresNoMigrationMarker(t *testing.T) {
	// Nothing writes theme_migrated until Phase 6, which declares the field before
	// its first writer exists — so no on-disk marker can be dropped in the interim,
	// and no writer here may emit the key.
	path := writePrefsFile(t, `{"session_list_mode":"flat","theme":"nord"}`)
	store := prefs.NewStore(path)

	if err := store.Save(prefs.ModeByTag); err != nil {
		t.Fatalf("unexpected Save error: %v", err)
	}
	if value, ok := decodePrefsFile(t, path)["theme_migrated"]; ok {
		t.Errorf("Save wrote theme_migrated = %v, want the key absent", value)
	}

	if err := store.SaveAppearance(prefs.AppearanceDark); err != nil {
		t.Fatalf("unexpected SaveAppearance error: %v", err)
	}
	if value, ok := decodePrefsFile(t, path)["theme_migrated"]; ok {
		t.Errorf("SaveAppearance wrote theme_migrated = %v, want the key absent", value)
	}
}
