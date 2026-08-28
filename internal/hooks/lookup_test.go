package hooks_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
)

func TestLookupOnResume(t *testing.T) {
	t.Run("returns no-hook when hooks.json is missing", func(t *testing.T) {
		dir := t.TempDir()
		store := hooks.NewStore(filepath.Join(dir, "hooks.json"))

		cmd, ok, err := hooks.LookupOnResume(store, "session:0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Errorf("got ok=true, want false")
		}
		if cmd != "" {
			t.Errorf("got cmd=%q, want empty", cmd)
		}
	})

	t.Run("returns no-hook when hooks.json is malformed JSON", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		if err := os.WriteFile(filePath, []byte("{not json"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
		cmd, ok, err := hooks.LookupOnResume(store, "session:0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Errorf("got ok=true, want false")
		}
		if cmd != "" {
			t.Errorf("got cmd=%q, want empty", cmd)
		}
	})

	t.Run("returns no-hook when the hook-key is absent", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		content := `{"other-session:0.0":{"on-resume":"echo hi"}}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
		cmd, ok, err := hooks.LookupOnResume(store, "missing-session:0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Errorf("got ok=true, want false")
		}
		if cmd != "" {
			t.Errorf("got cmd=%q, want empty", cmd)
		}
	})

	t.Run("returns no-hook when the key has no on-resume event", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		content := `{"session:0.0":{"on-attach":"echo attached"}}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
		cmd, ok, err := hooks.LookupOnResume(store, "session:0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Errorf("got ok=true, want false")
		}
		if cmd != "" {
			t.Errorf("got cmd=%q, want empty", cmd)
		}
	})

	t.Run("returns no-hook when the on-resume command is empty string", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		content := `{"session:0.0":{"on-resume":""}}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
		cmd, ok, err := hooks.LookupOnResume(store, "session:0.0")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Errorf("got ok=true, want false")
		}
		if cmd != "" {
			t.Errorf("got cmd=%q, want empty", cmd)
		}
	})

	t.Run("returns the command verbatim when on-resume is registered", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		const want = "echo hello world; ls -la"
		content := `{"session:0.0":{"on-resume":"echo hello world; ls -la"}}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
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
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		const want = "ls"
		content := `{"work:foo:0.0":{"on-resume":"ls"}}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
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
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		content := `{"":{"on-resume":"rm -rf /"}}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
		cmd, ok, err := hooks.LookupOnResume(store, "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Errorf("got ok=true, want false for an empty key")
		}
		if cmd != "" {
			t.Errorf("got cmd=%q, want empty", cmd)
		}
	})

	t.Run("reads no file for an empty key", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		// A directory would surface EISDIR out of the store's read, so a clean
		// miss here proves the file was never consulted.
		if err := os.Mkdir(filePath, 0o700); err != nil {
			t.Fatalf("failed to create directory: %v", err)
		}

		store := hooks.NewStore(filePath)
		cmd, ok, err := hooks.LookupOnResume(store, "")
		if err != nil {
			t.Fatalf("unexpected error for an empty key over an unreadable store: %v", err)
		}
		if ok {
			t.Errorf("got ok=true, want false")
		}
		if cmd != "" {
			t.Errorf("got cmd=%q, want empty", cmd)
		}
	})

	t.Run("does not trim a whitespace-only key", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		content := `{"":{"on-resume":"rm -rf /"}}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
		cmd, ok, err := hooks.LookupOnResume(store, " ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok || cmd != "" {
			t.Errorf("got cmd=%q ok=%v, want a miss: a whitespace key must not collapse to the empty key", cmd, ok)
		}

		seeded := filepath.Join(dir, "seeded.json")
		if err := os.WriteFile(seeded, []byte(`{" ":{"on-resume":"echo spaced"}}`), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}
		cmd, ok, err = hooks.LookupOnResume(hooks.NewStore(seeded), " ")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok || cmd != "echo spaced" {
			t.Errorf("got cmd=%q ok=%v, want the literally-seeded whitespace key to hit", cmd, ok)
		}
	})

	t.Run("surfaces a wrapped I/O error distinct from the no-hook case", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
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
