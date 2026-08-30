package hooks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
)

// storeWithContent stages a hooks.json holding content in a fresh temp
// directory and returns a store over it.
func storeWithContent(t *testing.T, content string) *hooks.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("seed hooks.json: %v", err)
	}
	return hooks.NewStore(path)
}

// assertNoHook pins the whole no-hook answer: the lookup degrades rather than
// erroring, reports no hit, and yields no command.
func assertNoHook(t *testing.T, cmd string, ok bool, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ok {
		t.Errorf("got ok=true, want false")
	}
	if cmd != "" {
		t.Errorf("got cmd=%q, want empty", cmd)
	}
}

func TestLookupOnResume(t *testing.T) {
	t.Run("returns no-hook when hooks.json is missing", func(t *testing.T) {
		store := hooks.NewStore(filepath.Join(t.TempDir(), "hooks.json"))

		cmd, ok, err := hooks.LookupOnResume(store, "session:0.0")
		assertNoHook(t, cmd, ok, err)
	})

	t.Run("returns no-hook when hooks.json is malformed JSON", func(t *testing.T) {
		store := storeWithContent(t, "{not json")

		cmd, ok, err := hooks.LookupOnResume(store, "session:0.0")
		assertNoHook(t, cmd, ok, err)
	})

	t.Run("returns no-hook when the hook-key is absent", func(t *testing.T) {
		store := storeWithContent(t, `{"other-session:0.0":{"on-resume":"echo hi"}}`)

		cmd, ok, err := hooks.LookupOnResume(store, "missing-session:0.0")
		assertNoHook(t, cmd, ok, err)
	})

	t.Run("returns no-hook when the key has no on-resume event", func(t *testing.T) {
		store := storeWithContent(t, `{"session:0.0":{"on-attach":"echo attached"}}`)

		cmd, ok, err := hooks.LookupOnResume(store, "session:0.0")
		assertNoHook(t, cmd, ok, err)
	})

	t.Run("returns no-hook when the on-resume command is empty string", func(t *testing.T) {
		store := storeWithContent(t, `{"session:0.0":{"on-resume":""}}`)

		cmd, ok, err := hooks.LookupOnResume(store, "session:0.0")
		assertNoHook(t, cmd, ok, err)
	})

	t.Run("returns the command verbatim when on-resume is registered", func(t *testing.T) {
		const want = "echo hello world; ls -la"
		store := storeWithContent(t, `{"session:0.0":{"on-resume":"echo hello world; ls -la"}}`)

		cmd, ok, err := hooks.LookupOnResume(store, "session:0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Errorf("got ok=false, want true")
		}
		if cmd != want {
			t.Errorf("got cmd=%q, want %q", cmd, want)
		}
	})

	t.Run("round-trips hook keys containing colons in the session name", func(t *testing.T) {
		const want = "ls"
		store := storeWithContent(t, `{"work:foo:0.0":{"on-resume":"ls"}}`)

		cmd, ok, err := hooks.LookupOnResume(store, "work:foo:0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Errorf("got ok=false, want true")
		}
		if cmd != want {
			t.Errorf("got cmd=%q, want %q", cmd, want)
		}
	})

	t.Run("returns no hook for an empty key even when hooks.json holds an empty-key entry", func(t *testing.T) {
		store := storeWithContent(t, `{"":{"on-resume":"rm -rf /"}}`)

		cmd, ok, err := hooks.LookupOnResume(store, "")
		assertNoHook(t, cmd, ok, err)
	})

	t.Run("reads no file for an empty key", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "hooks.json")
		// A directory would surface EISDIR out of the store's read, so a clean
		// miss here proves the file was never consulted.
		if err := os.Mkdir(filePath, 0o700); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		store := hooks.NewStore(filePath)
		cmd, ok, err := hooks.LookupOnResume(store, "")
		assertNoHook(t, cmd, ok, err)
	})

	t.Run("does not trim a whitespace-only key", func(t *testing.T) {
		store := storeWithContent(t, `{"":{"on-resume":"rm -rf /"}}`)

		cmd, ok, err := hooks.LookupOnResume(store, " ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || cmd != "" {
			t.Errorf("got cmd=%q ok=%v, want a miss: a whitespace key must not collapse to the empty key", cmd, ok)
		}

		seeded := storeWithContent(t, `{" ":{"on-resume":"echo spaced"}}`)
		cmd, ok, err = hooks.LookupOnResume(seeded, " ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || cmd != "echo spaced" {
			t.Errorf("got cmd=%q ok=%v, want the literally-seeded whitespace key to hit", cmd, ok)
		}
	})

	t.Run("surfaces a wrapped I/O error distinct from the no-hook case", func(t *testing.T) {
		filePath := filepath.Join(t.TempDir(), "hooks.json")
		// Reading a directory returns EISDIR, not ErrNotExist, so the error
		// propagates rather than degrading to "no hook".
		if err := os.Mkdir(filePath, 0o700); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		store := hooks.NewStore(filePath)
		cmd, ok, err := hooks.LookupOnResume(store, "session:0.0")
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
		if ok {
			t.Errorf("got ok=true, want false")
		}
		if cmd != "" {
			t.Errorf("got cmd=%q, want empty", cmd)
		}
		if !strings.Contains(err.Error(), "load hooks") {
			t.Errorf("error %q does not contain %q", err.Error(), "load hooks")
		}
	})
}
