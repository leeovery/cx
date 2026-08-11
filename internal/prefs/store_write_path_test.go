// White-box: the write path's contract is the unexported mutate seam.
package prefs

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Deliberately duplicates theme_keys_test.go's helper: that file is package
// prefs_test and cannot share with this one.
func seedPrefsFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test prefs file: %v", err)
	}
	return path
}

func readRaw(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read prefs file: %v", err)
	}
	return data
}

func decodeWritten(t *testing.T, path string) map[string]any {
	t.Helper()

	data := readRaw(t, path)
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("written prefs file is not valid JSON (%q): %v", data, err)
	}
	return decoded
}

func assertWrittenValue(t *testing.T, decoded map[string]any, key, want string) {
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

func assertNoTempFiles(t *testing.T, dir string) {
	t.Helper()

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".atomic-") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

func assertUntouched(t *testing.T, path string, before []byte) {
	t.Helper()

	if after := readRaw(t, path); !bytes.Equal(after, before) {
		t.Errorf("prefs file was rewritten: got %q, want %q byte-identical", after, before)
	}
	assertNoTempFiles(t, filepath.Dir(path))
}

func assertNothingCreated(t *testing.T, dir, path string) {
	t.Helper()

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("os.Stat(%s) error = %v, want the file still absent", path, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("failed to read temp dir: %v", err)
	}
	if len(entries) != 0 {
		names := make([]string, 0, len(entries))
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("temp dir %s contains %v, want none — the save must create nothing", dir, names)
	}
	assertNoTempFiles(t, dir)
}

type undecodableErrClass int

const (
	classSyntax undecodableErrClass = iota
	// A *json.UnmarshalTypeError with empty Field: the whole document is the
	// wrong JSON type, decoding to a wholly zero record.
	classTopLevelTypeMismatch
)

type undecodableCase struct {
	name     string
	content  string
	errClass undecodableErrClass
}

func (c undecodableCase) assertErr(t *testing.T, err error) {
	t.Helper()

	switch c.errClass {
	case classSyntax:
		var syntaxErr *json.SyntaxError
		if !errors.As(err, &syntaxErr) {
			t.Errorf("error = %v (%T), want the decoder's *json.SyntaxError", err, err)
		}
	case classTopLevelTypeMismatch:
		var typeErr *json.UnmarshalTypeError
		if !errors.As(err, &typeErr) {
			t.Fatalf("error = %v (%T), want a *json.UnmarshalTypeError", err, err)
		}
		if typeErr.Field != "" {
			t.Errorf("UnmarshalTypeError.Field = %q, want empty — the top-level discriminator", typeErr.Field)
		}
	}
}

func undecodablePrefsCases() []undecodableCase {
	return []undecodableCase{
		{name: "a truncated object", content: `{`, errClass: classSyntax},
		{name: "a trailing comma", content: `{"session_list_mode":"flat",}`, errClass: classSyntax},
		{name: "junk that is not JSON at all", content: `not json`, errClass: classSyntax},
		{name: "an unterminated object carrying real values", content: `{"session_list_mode":"flat","theme":"nord"`, errClass: classSyntax},
		{name: "a zero-byte file", content: ``, errClass: classSyntax},
		{name: "a top-level array", content: `[1,2]`, errClass: classTopLevelTypeMismatch},
		{name: "a top-level bare string", content: `"x"`, errClass: classTopLevelTypeMismatch},
		{name: "a top-level bare number", content: `3`, errClass: classTopLevelTypeMismatch},
		{name: "a top-level bare boolean", content: `true`, errClass: classTopLevelTypeMismatch},
	}
}

type absentPathCase struct {
	name string
	rel  []string
}

func (c absentPathCase) path(dir string) string {
	return filepath.Join(append([]string{dir}, c.rel...)...)
}

func absentPathCases() []absentPathCase {
	return []absentPathCase{
		{name: "the file is absent", rel: []string{"prefs.json"}},
		{name: "the parent directory is absent too", rel: []string{"sub", "nested", "prefs.json"}},
	}
}

func TestSave_CreatesAbsentFile(t *testing.T) {
	for _, c := range absentPathCases() {
		t.Run(c.name, func(t *testing.T) {
			path := c.path(t.TempDir())
			store := NewStore(path)

			if err := store.Save(ModeByTag); err != nil {
				t.Fatalf("unexpected Save error: %v", err)
			}

			assertWrittenValue(t, decodeWritten(t, path), "session_list_mode", "by-tag")
			assertNoTempFiles(t, filepath.Dir(path))
		})
	}
}

func TestSave_AbortsOnUndecodable(t *testing.T) {
	for _, c := range undecodablePrefsCases() {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, c.content)
			before := readRaw(t, path)
			store := NewStore(path)

			err := store.Save(ModeByTag)
			if err == nil {
				t.Fatalf("Save returned nil, want an abort error")
			}
			c.assertErr(t, err)

			assertUntouched(t, path, before)
		})
	}
}

func TestSave_AbortsOnReadError(t *testing.T) {
	path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":"nord"}`)
	before := readRaw(t, path)

	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("failed to chmod prefs file unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if _, err := os.ReadFile(path); err == nil {
		t.Skip("this user can read a 0000-mode file; the unreadable-file branch is unreachable here")
	}

	store := NewStore(path)
	err := store.Save(ModeByTag)
	if err == nil {
		t.Fatalf("Save returned nil for an unreadable file, want an abort error")
	}
	if !errors.Is(err, os.ErrPermission) {
		t.Errorf("error = %v, want the OS permission error returned verbatim", err)
	}

	if err := os.Chmod(path, 0o644); err != nil {
		t.Fatalf("failed to restore prefs file mode: %v", err)
	}
	assertUntouched(t, path, before)
}

// theme_migrated never reaches the carve-out — its own total decode absorbs a
// wrong type one layer earlier — so string fields exercise it instead.
func TestSave_WrongTypedFieldDoesNotAbort(t *testing.T) {
	t.Run("a wrong-typed session_list_mode still preserves theme", func(t *testing.T) {
		path := seedPrefsFile(t, `{"session_list_mode":5,"theme":"nord"}`)
		store := NewStore(path)

		if err := store.Save(ModeByTag); err != nil {
			t.Fatalf("unexpected Save error: %v", err)
		}

		decoded := decodeWritten(t, path)
		assertWrittenValue(t, decoded, "session_list_mode", "by-tag")
		assertWrittenValue(t, decoded, "theme", "nord")
	})

	t.Run("the offending field is normalised to its zero value, siblings survive", func(t *testing.T) {
		path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":5,"theme_dark":"nord","appearance":"dark"}`)
		store := NewStore(path)

		if err := store.Save(ModeByProject); err != nil {
			t.Fatalf("unexpected Save error: %v", err)
		}

		decoded := decodeWritten(t, path)
		assertWrittenValue(t, decoded, "session_list_mode", "by-project")
		assertWrittenValue(t, decoded, "theme_dark", "nord")
		assertWrittenValue(t, decoded, "appearance", "dark")
		if value, ok := decoded["theme"]; ok {
			t.Errorf("written JSON carries theme = %v, want the wrong-typed value normalised away", value)
		}
	})
}

func TestSave_UnrecognisedValueIsNotUnusable(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "an unrecognised session_list_mode", content: `{"session_list_mode":"sideways"}`},
		{name: "an unrecognised appearance", content: `{"appearance":"sepia"}`},
		{name: "an unrecognised theme slug", content: `{"theme":"../evil"}`},
		{name: "a key the struct does not declare", content: `{"session_list_mode":"flat","made_up_key":"whatever"}`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, c.content)
			store := NewStore(path)

			if err := store.Save(ModeByTag); err != nil {
				t.Fatalf("unexpected Save error: %v", err)
			}

			assertWrittenValue(t, decodeWritten(t, path), "session_list_mode", "by-tag")
		})
	}
}

// Reachable: the first theme commit creates the file and encodes a mode
// nobody has chosen.
func TestWrite_OmitsAnUnsetSessionListMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "prefs.json")

	if err := NewStore(path).SaveTheme("nord"); err != nil {
		t.Fatalf("unexpected SaveTheme error: %v", err)
	}

	decoded := decodeWritten(t, path)
	assertWrittenValue(t, decoded, "theme", "nord")
	if value, ok := decoded["session_list_mode"]; ok {
		t.Errorf("written JSON carries session_list_mode = %#v, want the key absent — it was never set", value)
	}

	// ModeFlat marshals as "flat", not the empty string omitempty swallows.
	if err := NewStore(path).Save(ModeFlat); err != nil {
		t.Fatalf("unexpected Save error: %v", err)
	}
	assertWrittenValue(t, decodeWritten(t, path), "session_list_mode", "flat")
}

func TestMutate_DecliningMutatorWritesNothing(t *testing.T) {
	decline := func(*prefsFile, bool) bool { return false }

	t.Run("a present file is left byte-identical", func(t *testing.T) {
		path := seedPrefsFile(t, `{"session_list_mode":"flat","theme":"nord"}`)
		before := readRaw(t, path)
		store := NewStore(path)

		if err := store.mutate(decline); err != nil {
			t.Fatalf("unexpected mutate error: %v", err)
		}

		assertUntouched(t, path, before)
	})

	t.Run("an absent file is not created", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "prefs.json")
		store := NewStore(path)

		if err := store.mutate(decline); err != nil {
			t.Fatalf("unexpected mutate error: %v", err)
		}

		if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("os.Stat error = %v, want the file still absent", err)
		}
		assertNoTempFiles(t, dir)
	})
}

func TestMutate_ReportsFileExistence(t *testing.T) {
	t.Run("true for a present file", func(t *testing.T) {
		store := NewStore(seedPrefsFile(t, `{"session_list_mode":"flat"}`))

		var got, called = false, false
		if err := store.mutate(func(_ *prefsFile, existed bool) bool {
			got, called = existed, true
			return false
		}); err != nil {
			t.Fatalf("unexpected mutate error: %v", err)
		}

		if !called {
			t.Fatalf("mutate never invoked fn")
		}
		if !got {
			t.Errorf("existed = false, want true for a present file")
		}
	})

	t.Run("false for an absent file", func(t *testing.T) {
		store := NewStore(filepath.Join(t.TempDir(), "prefs.json"))

		var got, called = true, false
		if err := store.mutate(func(_ *prefsFile, existed bool) bool {
			got, called = existed, true
			return false
		}); err != nil {
			t.Fatalf("unexpected mutate error: %v", err)
		}

		if !called {
			t.Fatalf("mutate never invoked fn")
		}
		if got {
			t.Errorf("existed = true, want false for an absent file")
		}
	})
}
