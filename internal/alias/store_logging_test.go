package alias_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/alias"
	"github.com/leeovery/portal/internal/logtest"
)

// readOnlyDirAliasPath returns a path under an existing 0500 directory, so the
// write itself fails rather than the directory creation.
func readOnlyDirAliasPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	roDir := filepath.Join(dir, "ro")
	if err := os.Mkdir(roDir, 0o500); err != nil {
		t.Fatalf("failed to create read-only dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o700) })
	return filepath.Join(roDir, "aliases")
}

func TestSetAndSave(t *testing.T) {
	t.Run("emits INFO op=set with value and via=cli after persisting a new alias", func(t *testing.T) {
		dir := t.TempDir()
		store := alias.NewStore(filepath.Join(dir, "aliases"))
		sink := logtest.Install(t)

		if err := store.SetAndSave("p", "/code/portal", "cli"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rec := sink.Records().Only(t, "log record")
		logtest.AssertRecord(t, rec, logtest.RecordWant{
			Level:     slog.LevelInfo,
			Msg:       "set",
			Component: "aliases",
			Op:        "set",
			Via:       "cli",
		})
		if got := rec.AttrString(t, "alias"); got != "p" {
			t.Errorf("alias = %q, want %q", got, "p")
		}
		if got := rec.AttrString(t, "value"); got != "/code/portal" {
			t.Errorf("value = %q, want %q", got, "/code/portal")
		}

		loaded, err := alias.NewStore(filepath.Join(dir, "aliases")).Load()
		if err != nil {
			t.Fatalf("reload failed: %v", err)
		}
		if loaded["p"] != "/code/portal" {
			t.Errorf("persisted value = %q, want %q", loaded["p"], "/code/portal")
		}
	})

	t.Run("emits INFO op=modify when the alias exists with a different path", func(t *testing.T) {
		dir := t.TempDir()
		store := alias.NewStore(filepath.Join(dir, "aliases"))
		store.Set("p", "/code/old")
		if err := store.Save(); err != nil {
			t.Fatalf("seed save failed: %v", err)
		}
		sink := logtest.Install(t)

		if err := store.SetAndSave("p", "/code/new", "cli"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rec := sink.Records().Only(t, "log record")
		logtest.AssertRecord(t, rec, logtest.RecordWant{
			Level:     slog.LevelInfo,
			Msg:       "modify",
			Component: "aliases",
			Op:        "modify",
			Via:       "cli",
		})
		if got := rec.AttrString(t, "alias"); got != "p" {
			t.Errorf("alias = %q, want %q", got, "p")
		}
		if got := rec.AttrString(t, "value"); got != "/code/new" {
			t.Errorf("value = %q, want %q", got, "/code/new")
		}
	})

	t.Run("emits DEBUG op=set-noop and skips the persist when the alias path is unchanged", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "aliases")
		store := alias.NewStore(path)
		store.Set("p", "/code/portal")
		if err := store.Save(); err != nil {
			t.Fatalf("seed save failed: %v", err)
		}

		info, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat after seed failed: %v", err)
		}
		seedModTime := info.ModTime()

		sink := logtest.Install(t)

		if err := store.SetAndSave("p", "/code/portal", "cli"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rec := sink.Records().Only(t, "log record")
		logtest.AssertRecord(t, rec, logtest.RecordWant{
			Level:     slog.LevelDebug,
			Msg:       "set-noop",
			Component: "aliases",
			Op:        "set-noop",
			Via:       "cli",
		})
		if got := rec.AttrString(t, "alias"); got != "p" {
			t.Errorf("alias = %q, want %q", got, "p")
		}

		after, err := os.Stat(path)
		if err != nil {
			t.Fatalf("stat after set-noop failed: %v", err)
		}
		if !after.ModTime().Equal(seedModTime) {
			t.Errorf("file was rewritten on set-noop: modtime changed %v -> %v", seedModTime, after.ModTime())
		}
	})

	t.Run("emits WARN with write-failed-write error_class when persist fails", func(t *testing.T) {
		path := readOnlyDirAliasPath(t)
		store := alias.NewStore(path)
		sink := logtest.Install(t)

		err := store.SetAndSave("p", "/code/portal", "cli")
		if err == nil {
			t.Fatal("expected error from persist into read-only dir, got nil")
		}

		rec := sink.Records().Only(t, "log record")
		logtest.AssertRecord(t, rec, logtest.RecordWant{
			Level:     slog.LevelWarn,
			Msg:       "set",
			Component: "aliases",
			Op:        "set",
			Via:       "cli",
		})
		if got := rec.AttrString(t, "error_class"); got != "write-failed-write" {
			t.Errorf("error_class = %q, want %q", got, "write-failed-write")
		}
		if !rec.HasAttr("error") {
			t.Errorf("WARN record missing error attr: %+v", rec.Attrs)
		}
	})
}

func TestDeleteAndSave(t *testing.T) {
	t.Run("emits INFO op=rm without a value attr for a successful delete", func(t *testing.T) {
		dir := t.TempDir()
		store := alias.NewStore(filepath.Join(dir, "aliases"))
		store.Set("p", "/code/portal")
		if err := store.Save(); err != nil {
			t.Fatalf("seed save failed: %v", err)
		}
		sink := logtest.Install(t)

		existed, err := store.DeleteAndSave("p", "cli")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !existed {
			t.Fatal("expected existed=true for a present alias")
		}

		rec := sink.Records().Only(t, "log record")
		logtest.AssertRecord(t, rec, logtest.RecordWant{
			Level:     slog.LevelInfo,
			Msg:       "rm",
			Component: "aliases",
			Op:        "rm",
			Via:       "cli",
		})
		if got := rec.AttrString(t, "alias"); got != "p" {
			t.Errorf("alias = %q, want %q", got, "p")
		}
		if rec.HasAttr("value") {
			t.Errorf("rm record must not carry a value attr: %+v", rec.Attrs)
		}
	})

	t.Run("emits nothing and returns existed=false when deleting an absent alias", func(t *testing.T) {
		dir := t.TempDir()
		store := alias.NewStore(filepath.Join(dir, "aliases"))
		sink := logtest.Install(t)

		existed, err := store.DeleteAndSave("missing", "cli")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if existed {
			t.Error("expected existed=false for an absent alias")
		}

		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("expected no log records for absent-delete, got %d: %+v", len(recs), recs)
		}
	})

	t.Run("emits WARN with write-failed-write error_class when persist fails", func(t *testing.T) {
		path := readOnlyDirAliasPath(t)
		store := alias.NewStore(path)
		store.Set("p", "/code/portal")
		sink := logtest.Install(t)

		existed, err := store.DeleteAndSave("p", "cli")
		if err == nil {
			t.Fatal("expected error from persist into read-only dir, got nil")
		}
		if !existed {
			t.Error("expected existed=true (the entry was present in memory)")
		}

		rec := sink.Records().Only(t, "log record")
		if rec.Level != slog.LevelWarn {
			t.Errorf("level = %v, want WARN", rec.Level)
		}
		if rec.Msg != "rm" {
			t.Errorf("msg = %q, want %q", rec.Msg, "rm")
		}
		if got := rec.AttrString(t, "op"); got != "rm" {
			t.Errorf("op = %q, want %q", got, "rm")
		}
		if got := rec.AttrString(t, "error_class"); got != "write-failed-write" {
			t.Errorf("error_class = %q, want %q", got, "write-failed-write")
		}
		if !rec.HasAttr("error") {
			t.Errorf("WARN record missing error attr: %+v", rec.Attrs)
		}
	})
}
