// White-box (package prefs) because the write path's contract is the unexported
// mutate seam: §8.9 deliberately refuses to export a whole-record Load/Save API,
// so "the mutator declines and nothing is written" is only reachable from inside
// the package. The Save-level tests live here alongside it rather than in the
// external prefs_test package so the whole write-path story — strict decode,
// merge, abort — reads as one file.
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

// seedPrefsFile writes content to a prefs.json inside a fresh temp dir and
// returns its path. Deliberately a second declaration of the helper shape in
// theme_keys_test.go: that file is package prefs_test and cannot share with this
// one.
func seedPrefsFile(t *testing.T, content string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "prefs.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write test prefs file: %v", err)
	}
	return path
}

// readRaw returns the file's exact bytes.
func readRaw(t *testing.T, path string) []byte {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read prefs file: %v", err)
	}
	return data
}

// decodeWritten reads the on-disk prefs.json into a generic map so key
// presence/absence is asserted structurally rather than by substring.
func decodeWritten(t *testing.T, path string) map[string]any {
	t.Helper()

	data := readRaw(t, path)
	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("written prefs file is not valid JSON (%q): %v", data, err)
	}
	return decoded
}

// assertWrittenValue asserts an on-disk key holds exactly want.
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

// assertNoTempFiles asserts AtomicWrite left no temp file behind in dir — an
// abort must not even reach the write, so there is nothing to clean up.
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

// assertUntouched asserts the file's bytes are identical to before and that no
// temp file survives.
func assertUntouched(t *testing.T, path string, before []byte) {
	t.Helper()

	if after := readRaw(t, path); !bytes.Equal(after, before) {
		t.Errorf("prefs file was rewritten: got %q, want %q byte-identical", after, before)
	}
	assertNoTempFiles(t, filepath.Dir(path))
}

// TestSave_CreatesAbsentFile pins the create-on-absent half of §8.9: an absent
// prefs.json has nothing to merge and nothing to lose, so the write proceeds.
// This is the most common write in the product — a brand-new user's first
// keypress — and an abort here would be permanent, since nothing else creates
// the file.
func TestSave_CreatesAbsentFile(t *testing.T) {
	cases := []struct {
		name string
		rel  []string
	}{
		{name: "the file is absent", rel: []string{"prefs.json"}},
		{name: "the parent directory is absent too", rel: []string{"sub", "nested", "prefs.json"}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := filepath.Join(append([]string{t.TempDir()}, c.rel...)...)
			store := NewStore(path)

			if err := store.Save(ModeByTag); err != nil {
				t.Fatalf("unexpected Save error: %v", err)
			}

			assertWrittenValue(t, decodeWritten(t, path), "session_list_mode", "by-tag")
			assertNoTempFiles(t, filepath.Dir(path))
		})
	}
}

// TestSave_AbortsOnMalformedJSON pins the abort-on-undecodable half of §8.9.
// Under the tolerant load decode a stray comma collapses to a zero-valued record
// with no error, so the writer merges into it and one keypress erases every
// other key — the exact silent, permanent destruction this split exists to
// prevent.
func TestSave_AbortsOnMalformedJSON(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "a truncated object", content: `{`},
		{name: "a trailing comma", content: `{"session_list_mode":"flat",}`},
		{name: "junk that is not JSON at all", content: `not json`},
		{name: "an unterminated object carrying real values", content: `{"session_list_mode":"flat","theme":"nord"`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, c.content)
			before := readRaw(t, path)
			store := NewStore(path)

			err := store.Save(ModeByTag)
			if err == nil {
				t.Fatalf("Save returned nil, want an abort error")
			}

			// Returned verbatim: the decoder's own syntax error, not a
			// re-classified one — prefs is a leaf and reports by returning.
			var syntaxErr *json.SyntaxError
			if !errors.As(err, &syntaxErr) {
				t.Errorf("error = %v (%T), want the decoder's *json.SyntaxError", err, err)
			}

			assertUntouched(t, path, before)
		})
	}
}

// TestSave_AbortsOnEmptyFile pins the zero-byte case deliberately rather than
// letting it fall out: the file is present and is not valid JSON, and the rule
// is syntax-driven, so it aborts.
func TestSave_AbortsOnEmptyFile(t *testing.T) {
	path := seedPrefsFile(t, "")
	before := readRaw(t, path)
	store := NewStore(path)

	if err := store.Save(ModeByTag); err == nil {
		t.Fatalf("Save returned nil for a zero-byte file, want an abort error")
	}

	assertUntouched(t, path, before)
}

// TestSave_AbortsOnReadError pins the I/O half of "present but unusable": a
// non-ErrNotExist read failure aborts exactly like malformed JSON, and the OS
// error is returned verbatim.
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

// TestSave_WrongTypedFieldDoesNotAbort pins the discriminator: a wrong TYPE on a
// declared field is a value problem, not a syntax one. encoding/json still
// populates every other field, so the merge is safe and the write proceeds.
//
// The string fields are what exercise this rung. theme_migrated — the field a
// user is most likely to hand-edit wrongly, and the reason the carve-out was
// specified — never reaches it: its own total decode (see migrationMarker)
// absorbs a wrong type one layer earlier, so no UnmarshalTypeError is ever
// raised for it. That is deliberate. The carve-out here still leaves the
// tolerant LOAD path zeroing the record, which for the marker would lose the
// mode, the theme keys and the retained appearance on every read.
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
		// The accepted consequence of §8.1's tolerant absorption: the
		// wrong-typed field decodes to its zero value and is re-encoded as such
		// (omitted, by omitempty). That is absorption, not the loss the abort
		// rule exists to catch.
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

// TestSave_TopLevelTypeMismatchAborts is why the discriminator is `Field != ""`
// and not the error type alone: a top-level mismatch also yields an
// UnmarshalTypeError, but with an empty Field and a wholly zero-valued struct.
// Absorbing that would merge into an empty record and commit it.
func TestSave_TopLevelTypeMismatchAborts(t *testing.T) {
	cases := []struct {
		name    string
		content string
	}{
		{name: "an array", content: `[1,2]`},
		{name: "a bare string", content: `"x"`},
		{name: "a bare number", content: `3`},
		{name: "a bare boolean", content: `true`},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			path := seedPrefsFile(t, c.content)
			before := readRaw(t, path)
			store := NewStore(path)

			err := store.Save(ModeByTag)
			if err == nil {
				t.Fatalf("Save returned nil, want an abort error")
			}

			var typeErr *json.UnmarshalTypeError
			if !errors.As(err, &typeErr) {
				t.Fatalf("error = %v (%T), want a *json.UnmarshalTypeError", err, err)
			}
			if typeErr.Field != "" {
				t.Errorf("UnmarshalTypeError.Field = %q, want empty — the top-level discriminator", typeErr.Field)
			}

			assertUntouched(t, path, before)
		})
	}
}

// TestSave_UnrecognisedValueIsNotUnusable pins that the strict decode judges
// SYNTAX only. Treating an unrecognised value as fatal would make hand-editing
// prefs.json a way to lock yourself out of every write.
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

// TestWrite_OmitsAnUnsetSessionListMode pins that session_list_mode follows the
// same omission rule as every other key: a field-specific saver firing on a file
// whose mode was never set leaves the key ABSENT rather than writing it as an
// empty string.
//
// The case is reachable, not theoretical. §8.1 leaves a fresh install with no
// prefs.json at all, so the first theme commit both creates the file and encodes
// a mode nobody has ever chosen. `""` and absent are indistinguishable to every
// reader — parseMode collapses both to ModeFlat, neither decode carries
// key-presence logic, and a downgraded binary reads them the same — so this is
// about keeping the hand-editable file clean, which is the same reason §8.1
// gives for the theme keys and the retained appearance.
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

	// A mode that WAS chosen still round-trips: omitempty must not swallow a
	// real choice, and ModeFlat marshals as "flat" rather than the empty string,
	// so the default is persistable like any other.
	if err := NewStore(path).Save(ModeFlat); err != nil {
		t.Fatalf("unexpected Save error: %v", err)
	}
	assertWrittenValue(t, decodeWritten(t, path), "session_list_mode", "flat")
}

// TestMutate_DecliningMutatorWritesNothing pins the skip contract every later
// saver in this phase relies on: returning false touches no bytes and returns no
// error.
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

// TestMutate_ReportsFileExistence pins the second half of the mutator's
// contract: fn is told whether the file existed, which is what lets task 6-3's
// marker be written only when prefs.json already exists (§8.1).
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
