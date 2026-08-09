// White-box (package prefs) because the step both decodes start from is
// unexported and has no exported surface of its own: the file read and its
// absent-is-not-an-error classification are shared, while the decode policy layered
// on top of them deliberately is not. The seedPrefsFile helper this file uses is
// declared in store_write_path_test.go and reused rather than redeclared: it is
// the same package.
package prefs

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// TestReadBytes_SharedReadPolicy pins the three outcomes of the read the tolerant
// and strict decodes both start from: an absent file is not an error, a present one
// hands back its exact bytes, and any other read failure is returned verbatim
// alongside no bytes.
//
// It is asserted HERE, once, because both decode paths inherit it: a change to the
// absent-file or permission policy must be reachable in one place, and a copy of it
// on one path only is the shape in which a loader and a writer come to disagree
// about what an unreadable prefs.json means.
func TestReadBytes_SharedReadPolicy(t *testing.T) {
	t.Run("an absent file reports absent with no error", func(t *testing.T) {
		store := NewStore(filepath.Join(t.TempDir(), "nested", "prefs.json"))

		data, present, err := store.readBytes()
		if err != nil {
			t.Fatalf("readBytes error for an absent file = %v, want nil — absence is the normal first-run state", err)
		}
		if present {
			t.Error("readBytes reported an absent file as present")
		}
		if data != nil {
			t.Errorf("readBytes returned %q for an absent file, want no bytes", data)
		}
	})

	t.Run("a present file hands back its exact bytes", func(t *testing.T) {
		const content = `{"session_list_mode":"by-tag","theme":"nord"}`
		path := seedPrefsFile(t, content)

		data, present, err := NewStore(path).readBytes()
		if err != nil {
			t.Fatalf("unexpected readBytes error: %v", err)
		}
		if !present {
			t.Error("readBytes reported a present file as absent")
		}
		if got := string(data); got != content {
			t.Errorf("readBytes returned %q, want the file's bytes verbatim %q", got, content)
		}
	})

	t.Run("an unreadable file returns the OS error verbatim", func(t *testing.T) {
		path := seedPrefsFile(t, `{"theme":"nord"}`)
		makeUnreadable(t, path)

		data, present, err := NewStore(path).readBytes()
		if !errors.Is(err, os.ErrPermission) {
			t.Fatalf("readBytes error = %v, want the OS permission error returned verbatim", err)
		}
		if present {
			t.Error("readBytes reported an unreadable file as present")
		}
		if data != nil {
			t.Errorf("readBytes returned %q alongside a read error, want no bytes", data)
		}
	})
}

// TestDecodePaths_ShareTheReadAndSplitOnTheDecode is the two-sided statement of the
// split: the tolerant load decode and the strict write-path decode answer
// IDENTICALLY for every outcome the read decides, and DIFFERENTLY for the one the
// decode decides.
//
// The second half is the load-bearing one. A shared read must not become a shared
// decode: routing a writer through the tolerant policy turns a stray comma into a
// zero-valued record it then commits, erasing the mode, every theme key and the
// retained raw appearance in one keypress.
func TestDecodePaths_ShareTheReadAndSplitOnTheDecode(t *testing.T) {
	t.Run("an absent file is absent to both", func(t *testing.T) {
		store := NewStore(filepath.Join(t.TempDir(), "nested", "prefs.json"))

		tolerant, tolerantOK, tolerantErr := store.readFile()
		strict, strictOK, strictErr := store.readFileStrict()

		if tolerantErr != nil || strictErr != nil {
			t.Fatalf("absent-file errors: tolerant = %v, strict = %v, want nil from both", tolerantErr, strictErr)
		}
		if tolerantOK || strictOK {
			t.Errorf("absent-file presence: tolerant = %v, strict = %v, want false from both", tolerantOK, strictOK)
		}
		if tolerant != (prefsFile{}) || strict != (prefsFile{}) {
			t.Errorf("absent-file records: tolerant = %+v, strict = %+v, want the zero record from both", tolerant, strict)
		}
	})

	t.Run("an unreadable file fails both the same way", func(t *testing.T) {
		path := seedPrefsFile(t, `{"theme":"nord"}`)
		makeUnreadable(t, path)
		store := NewStore(path)

		tolerant, tolerantOK, tolerantErr := store.readFile()
		strict, strictOK, strictErr := store.readFileStrict()

		if !errors.Is(tolerantErr, os.ErrPermission) || !errors.Is(strictErr, os.ErrPermission) {
			t.Fatalf("unreadable-file errors: tolerant = %v, strict = %v, want the OS permission error from both", tolerantErr, strictErr)
		}
		if tolerantOK || strictOK {
			t.Errorf("unreadable-file presence: tolerant = %v, strict = %v, want false from both", tolerantOK, strictOK)
		}
		if tolerant != (prefsFile{}) || strict != (prefsFile{}) {
			t.Errorf("unreadable-file records: tolerant = %+v, strict = %+v, want the zero record from both", tolerant, strict)
		}
	})

	t.Run("a corrupt file separates them", func(t *testing.T) {
		store := NewStore(seedPrefsFile(t, `{"session_list_mode":"flat",}`))

		tolerant, tolerantOK, tolerantErr := store.readFile()
		if tolerantErr != nil {
			t.Errorf("the tolerant decode returned %v for a corrupt file, want it absorbed", tolerantErr)
		}
		if tolerantOK || tolerant != (prefsFile{}) {
			t.Errorf("the tolerant decode returned (%+v, %v), want the zero record reported as not cleanly decoded", tolerant, tolerantOK)
		}

		_, strictOK, strictErr := store.readFileStrict()
		var syntaxErr *json.SyntaxError
		if !errors.As(strictErr, &syntaxErr) {
			t.Errorf("the strict decode returned %v (%T) for a corrupt file, want the decoder's *json.SyntaxError — a writer must abort rather than merge", strictErr, strictErr)
		}
		if strictOK {
			t.Error("the strict decode reported a corrupt file as decoded")
		}
	})
}

// makeUnreadable chmods path to 0000 for the duration of the test, skipping when
// the running user can read it anyway (root), which would make the permission
// branch unreachable. The mode is restored on cleanup so the temp dir can be
// removed.
func makeUnreadable(t *testing.T, path string) {
	t.Helper()

	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("failed to chmod prefs file unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

	if _, err := os.ReadFile(path); err == nil {
		t.Skip("this user can read a 0000-mode file; the unreadable-file branch is unreachable here")
	}
}
