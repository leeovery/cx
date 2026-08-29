package hooks_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/fileutil"
	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/transienttest"
)

func installCapture(t *testing.T) *logtest.Sink {
	t.Helper()
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	return sink
}

// readOnlyDirPath returns a path under a 0500 directory, so a write to it fails
// at the temp-create phase. The sidecar lock file is created while the
// directory is still writable: a 0500 directory still permits opening an
// existing file, so without it the mutation would fail at the sidecar instead
// of at the write the fixture exists to fail.
func readOnlyDirPath(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	roDir := filepath.Join(dir, "ro")
	if err := os.Mkdir(roDir, 0o700); err != nil {
		t.Fatalf("failed to create read-only dir: %v", err)
	}
	path := filepath.Join(roDir, "hooks.json")
	transienttest.CreateHooksSidecar(t, path)
	if err := os.Chmod(roDir, 0o500); err != nil {
		t.Fatalf("failed to deny writes to dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(roDir, 0o700) })
	return path
}

// readFileBytes returns the file's exact bytes, so a test can assert a no-op
// left the file untouched rather than rewritten to equivalent content.
func readFileBytes(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read %s: %v", path, err)
	}
	return data
}

// modTime is the file's mtime, so a test can prove a no-op skipped Save even
// when a rewrite would have produced byte-identical content.
func modTime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("failed to stat %s: %v", path, err)
	}
	return info.ModTime()
}

// seedThenDenyWrites writes body to a hooks.json and only then locks its parent
// directory to 0500, so a save fails at the temp-create phase over a file that
// already holds content. The seed write must succeed before the directory is
// locked, so this cannot use readOnlyDirPath.
func seedThenDenyWrites(t *testing.T, body []byte) (*hooks.Store, string) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "hooks.json")
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	transienttest.CreateHooksSidecar(t, path)
	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatalf("chmod parent dir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	return hooks.NewStore(path), path
}

func TestLoad(t *testing.T) {
	t.Run("returns empty map when file does not exist", func(t *testing.T) {
		dir := t.TempDir()
		store := hooks.NewStore(filepath.Join(dir, "nonexistent", "hooks.json"))

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(h) != 0 {
			t.Errorf("got %d entries, want 0", len(h))
		}
	})

	t.Run("returns empty map when file contains malformed JSON", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")

		if err := os.WriteFile(filePath, []byte("{invalid json!!!"), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(h) != 0 {
			t.Errorf("got %d entries, want 0", len(h))
		}
	})

	t.Run("returns hooks from valid JSON file", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")

		content := `{"my-session:0.0":{"on-resume":"claude --resume abc123"},"my-session:0.1":{"on-resume":"claude --resume def456"}}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(h) != 2 {
			t.Fatalf("got %d keys, want 2", len(h))
		}

		if h["my-session:0.0"]["on-resume"] != "claude --resume abc123" {
			t.Errorf("my-session:0.0 on-resume = %q, want %q", h["my-session:0.0"]["on-resume"], "claude --resume abc123")
		}
		if h["my-session:0.1"]["on-resume"] != "claude --resume def456" {
			t.Errorf("my-session:0.1 on-resume = %q, want %q", h["my-session:0.1"]["on-resume"], "claude --resume def456")
		}
	})
}

func TestPersistence(t *testing.T) {
	t.Run("creates parent directory if missing", func(t *testing.T) {
		dir := t.TempDir()
		nested := filepath.Join(dir, "portal", "sub")
		filePath := filepath.Join(nested, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		info, err := os.Stat(nested)
		if err != nil {
			t.Fatalf("directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Errorf("expected directory, got file")
		}
	})

	t.Run("writes valid JSON that can be loaded back", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if err := store.Set("my-session:0.1", "on-resume", "claude --resume def456", "cli"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		loaded, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load saved file: %v", err)
		}

		if len(loaded) != 2 {
			t.Fatalf("got %d keys, want 2", len(loaded))
		}
		if loaded["my-session:0.0"]["on-resume"] != "claude --resume abc123" {
			t.Errorf("my-session:0.0 on-resume = %q, want %q", loaded["my-session:0.0"]["on-resume"], "claude --resume abc123")
		}
		if loaded["my-session:0.1"]["on-resume"] != "claude --resume def456" {
			t.Errorf("my-session:0.1 on-resume = %q, want %q", loaded["my-session:0.1"]["on-resume"], "claude --resume def456")
		}
	})

	t.Run("uses atomic write (no temp file survives beside hooks.json and its lock)", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("failed to read dir: %v", err)
		}

		for _, entry := range entries {
			if entry.Name() != "hooks.json" && entry.Name() != "hooks.json.lock" {
				t.Errorf("unexpected file in directory: %s", entry.Name())
			}
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read file: %v", err)
		}
		if len(data) == 0 {
			t.Error("file is empty")
		}
	})
}

func TestSet(t *testing.T) {
	t.Run("adds a new hook for a new key", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(h) != 1 {
			t.Fatalf("got %d keys, want 1", len(h))
		}
		if h["my-session:0.0"]["on-resume"] != "claude --resume abc123" {
			t.Errorf("my-session:0.0 on-resume = %q, want %q", h["my-session:0.0"]["on-resume"], "claude --resume abc123")
		}
	})

	t.Run("adds a second event to an existing key", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on first set: %v", err)
		}
		if err := store.Set("my-session:0.0", "on-start", "echo hello", "cli"); err != nil {
			t.Fatalf("unexpected error on second set: %v", err)
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(h) != 1 {
			t.Fatalf("got %d keys, want 1", len(h))
		}
		if len(h["my-session:0.0"]) != 2 {
			t.Fatalf("got %d events for my-session:0.0, want 2", len(h["my-session:0.0"]))
		}
		if h["my-session:0.0"]["on-resume"] != "claude --resume abc123" {
			t.Errorf("my-session:0.0 on-resume = %q, want %q", h["my-session:0.0"]["on-resume"], "claude --resume abc123")
		}
		if h["my-session:0.0"]["on-start"] != "echo hello" {
			t.Errorf("my-session:0.0 on-start = %q, want %q", h["my-session:0.0"]["on-start"], "echo hello")
		}
	})

	t.Run("overwrites existing entry for same key and event", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on first set: %v", err)
		}
		if err := store.Set("my-session:0.0", "on-resume", "claude --resume xyz789", "cli"); err != nil {
			t.Fatalf("unexpected error on second set: %v", err)
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(h) != 1 {
			t.Fatalf("got %d keys, want 1", len(h))
		}
		if len(h["my-session:0.0"]) != 1 {
			t.Fatalf("got %d events for my-session:0.0, want 1", len(h["my-session:0.0"]))
		}
		if h["my-session:0.0"]["on-resume"] != "claude --resume xyz789" {
			t.Errorf("my-session:0.0 on-resume = %q, want %q", h["my-session:0.0"]["on-resume"], "claude --resume xyz789")
		}
	})
}

func TestRemove(t *testing.T) {
	t.Run("it reports a removal when the named event was deleted", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set("my-session:0.1", "on-resume", "claude --resume def456", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		removed, err := store.Remove("my-session:0.0", "on-resume", "cli")
		if err != nil {
			t.Fatalf("unexpected error on remove: %v", err)
		}
		if !removed {
			t.Error("removed = false, want true for a seeded entry")
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(h) != 1 {
			t.Fatalf("got %d keys, want 1", len(h))
		}
		if _, ok := h["my-session:0.0"]; ok {
			t.Error("key my-session:0.0 should have been removed")
		}
		if h["my-session:0.1"]["on-resume"] != "claude --resume def456" {
			t.Errorf("my-session:0.1 on-resume = %q, want %q", h["my-session:0.1"]["on-resume"], "claude --resume def456")
		}
	})

	t.Run("it drops the outer key when its last event goes", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		removed, err := store.Remove("my-session:0.0", "on-resume", "cli")
		if err != nil {
			t.Fatalf("unexpected error on remove: %v", err)
		}
		if !removed {
			t.Error("removed = false, want true for a seeded entry")
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		if len(h) != 0 {
			t.Fatalf("got %d keys, want 0 (outer key should be cleaned up)", len(h))
		}
		if _, ok := h["my-session:0.0"]; ok {
			t.Error("key my-session:0.0 should have been removed from outer map")
		}
	})

	t.Run("it keeps the other events of a key and reports a removal", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set("my-session:0.0", "on-start", "echo hello", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		removed, err := store.Remove("my-session:0.0", "on-resume", "cli")
		if err != nil {
			t.Fatalf("unexpected error on remove: %v", err)
		}
		if !removed {
			t.Error("removed = false, want true for a seeded entry")
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}

		events, ok := h["my-session:0.0"]
		if !ok {
			t.Fatalf("outer key my-session:0.0 should be retained, got %+v", h)
		}
		if _, ok := events["on-resume"]; ok {
			t.Error("on-resume should have been removed")
		}
		if events["on-start"] != "echo hello" {
			t.Errorf("on-start = %q, want %q", events["on-start"], "echo hello")
		}
	})

	t.Run("it reports no removal when the key is absent", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		before := readFileBytes(t, filePath)
		beforeMod := modTime(t, filePath)

		removed, err := store.Remove("nonexistent:9.9", "on-resume", "cli")
		if err != nil {
			t.Fatalf("unexpected error on remove: %v", err)
		}
		if removed {
			t.Error("removed = true, want false for an absent key")
		}

		if after := readFileBytes(t, filePath); !bytes.Equal(before, after) {
			t.Errorf("file rewritten on a no-op removal:\nbefore %s\nafter  %s", before, after)
		}
		if after := modTime(t, filePath); !after.Equal(beforeMod) {
			t.Error("file was written on a no-op removal (Save should be skipped)")
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(h) != 1 {
			t.Fatalf("got %d keys, want 1 (original should remain)", len(h))
		}
	})

	t.Run("it reports no removal when the key is present but the event is not", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		before := readFileBytes(t, filePath)
		beforeMod := modTime(t, filePath)

		removed, err := store.Remove("my-session:0.0", "on-start", "cli")
		if err != nil {
			t.Fatalf("unexpected error on remove: %v", err)
		}
		if removed {
			t.Error("removed = true, want false when the named event is absent")
		}

		if after := readFileBytes(t, filePath); !bytes.Equal(before, after) {
			t.Errorf("file rewritten on a no-op removal:\nbefore %s\nafter  %s", before, after)
		}
		if after := modTime(t, filePath); !after.Equal(beforeMod) {
			t.Error("file was written on a no-op removal (Save should be skipped)")
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(h) != 1 {
			t.Fatalf("got %d keys, want 1", len(h))
		}
		if h["my-session:0.0"]["on-resume"] != "claude --resume abc123" {
			t.Errorf("my-session:0.0 on-resume = %q, want %q", h["my-session:0.0"]["on-resume"], "claude --resume abc123")
		}
	})

	t.Run("it leaves an absent hooks.json absent when it removes nothing", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		removed, err := store.Remove("my-session:0.0", "on-resume", "cli")
		if err != nil {
			t.Fatalf("unexpected error on remove: %v", err)
		}
		if removed {
			t.Error("removed = true, want false with no hooks file at all")
		}

		if _, err := os.Stat(filePath); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("hooks.json should still not exist, stat err = %v", err)
		}
	})

	t.Run("it leaves a malformed hooks.json byte-identical when it removes nothing", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		before := []byte("not json")
		if err := os.WriteFile(filePath, before, 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		store := hooks.NewStore(filePath)

		removed, err := store.Remove("my-session:0.0", "on-resume", "cli")
		if err != nil {
			t.Fatalf("unexpected error on remove: %v", err)
		}
		if removed {
			t.Error("removed = true, want false for a malformed file")
		}

		if after := readFileBytes(t, filePath); !bytes.Equal(before, after) {
			t.Errorf("malformed file rewritten:\nbefore %s\nafter  %s", before, after)
		}
	})

	t.Run("it leaves a key mapped to an empty event map in place", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		before := []byte(`{"abc123": {}}`)
		if err := os.WriteFile(filePath, before, 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		store := hooks.NewStore(filePath)

		removed, err := store.Remove("abc123", "on-resume", "cli")
		if err != nil {
			t.Fatalf("unexpected error on remove: %v", err)
		}
		if removed {
			t.Error("removed = true, want false for a key with no events")
		}

		if after := readFileBytes(t, filePath); !bytes.Equal(before, after) {
			t.Errorf("file rewritten on a no-op removal:\nbefore %s\nafter  %s", before, after)
		}
	})

	t.Run("it reports no removal when the save fails", func(t *testing.T) {
		store, _ := seedThenDenyWrites(t, []byte(`{"my-session:0.0":{"on-resume":"claude --resume abc123"}}`))

		removed, err := store.Remove("my-session:0.0", "on-resume", "cli")
		if err == nil {
			t.Fatal("expected error from Remove on read-only dir, got nil")
		}
		if removed {
			t.Error("removed = true, want false when the save failed")
		}
		if !errors.Is(err, fileutil.ErrWriteTempCreate) {
			t.Errorf("returned error not classified as temp-create: %v", err)
		}
	})
}

func TestList(t *testing.T) {
	t.Run("returns empty slice when no hooks", func(t *testing.T) {
		dir := t.TempDir()
		store := hooks.NewStore(filepath.Join(dir, "hooks.json"))

		list, err := store.List("cli")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(list) != 0 {
			t.Errorf("got %d hooks, want 0", len(list))
		}
	})

	t.Run("returns hooks sorted by key then event", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")

		content := `{"my-session:0.1":{"on-resume":"cmd1"},"my-session:0.0":{"on-start":"cmd0s","on-resume":"cmd0r"}}`
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)
		list, err := store.List("cli")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(list) != 3 {
			t.Fatalf("got %d hooks, want 3", len(list))
		}

		wantKeys := []string{"my-session:0.0", "my-session:0.0", "my-session:0.1"}
		wantEvents := []string{"on-resume", "on-start", "on-resume"}
		wantCmds := []string{"cmd0r", "cmd0s", "cmd1"}

		for i, hook := range list {
			if hook.Key != wantKeys[i] {
				t.Errorf("list[%d].Key = %q, want %q", i, hook.Key, wantKeys[i])
			}
			if hook.Event != wantEvents[i] {
				t.Errorf("list[%d].Event = %q, want %q", i, hook.Event, wantEvents[i])
			}
			if hook.Command != wantCmds[i] {
				t.Errorf("list[%d].Command = %q, want %q", i, hook.Command, wantCmds[i])
			}
		}
	})
}

func TestGet(t *testing.T) {
	t.Run("returns event map for registered key", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		events, err := store.Get("my-session:0.0", "internal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 1 {
			t.Fatalf("got %d events, want 1", len(events))
		}
		if events["on-resume"] != "claude --resume abc123" {
			t.Errorf("on-resume = %q, want %q", events["on-resume"], "claude --resume abc123")
		}
	})

	t.Run("returns empty map for unregistered key", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		events, err := store.Get("nonexistent:9.9", "internal")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(events) != 0 {
			t.Errorf("got %d events, want 0", len(events))
		}
	})
}

// The hook-key seed vocabulary: a reapable key is one the staleness rule can
// judge, so it is swept once absent from the live set.
var (
	reapableSeedA = transienttest.ReapableHookKey(0)
	reapableSeedB = transienttest.ReapableHookKey(1)
	reapableSeedC = transienttest.ReapableHookKey(2)
	reapableSeedD = transienttest.ReapableHookKey(3)
)

func TestCleanStale(t *testing.T) {
	t.Run("removes entries for keys not in live set", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set(reapableSeedA, "on-resume", "claude --resume def456", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		removed, err := store.CleanStale([]string{"my-session:0.0"}, snapshotAll(t, filePath))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(removed) != 1 {
			t.Fatalf("got %d removed, want 1", len(removed))
		}
		if removed[0] != reapableSeedA {
			t.Errorf("removed[0] = %q, want %q", removed[0], reapableSeedA)
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(h) != 1 {
			t.Fatalf("got %d keys, want 1", len(h))
		}
		if _, ok := h["my-session:0.0"]; !ok {
			t.Error("key my-session:0.0 should have been kept")
		}
		if _, ok := h[reapableSeedA]; ok {
			t.Errorf("key %s should have been removed", reapableSeedA)
		}
	})

	t.Run("returns empty slice when store is empty", func(t *testing.T) {
		dir := t.TempDir()
		store := hooks.NewStore(filepath.Join(dir, "hooks.json"))

		removed, err := store.CleanStale([]string{"my-session:0.0", "my-session:0.1"}, snapshotAll(t, filepath.Join(dir, "hooks.json")))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(removed) != 0 {
			t.Errorf("got %d removed, want 0", len(removed))
		}
	})

	t.Run("returns empty slice when all keys are live", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set("my-session:0.1", "on-resume", "claude --resume def456", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		removed, err := store.CleanStale([]string{"my-session:0.0", "my-session:0.1"}, snapshotAll(t, filePath))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(removed) != 0 {
			t.Errorf("got %d removed, want 0", len(removed))
		}
	})

	t.Run("removes all entries when live set is empty", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set(reapableSeedA, "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set(reapableSeedB, "on-resume", "claude --resume def456", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		removed, err := store.CleanStale([]string{}, snapshotAll(t, filePath))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(removed) != 2 {
			t.Fatalf("got %d removed, want 2", len(removed))
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(h) != 0 {
			t.Errorf("got %d keys, want 0", len(h))
		}
	})

	t.Run("only saves file when entries were removed", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		infoBefore, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}

		removed, err := store.CleanStale([]string{"my-session:0.0"}, snapshotAll(t, filePath))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(removed) != 0 {
			t.Errorf("got %d removed, want 0", len(removed))
		}

		infoAfter, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
			t.Error("file was modified when no entries were removed")
		}
	})

	t.Run("cleans stale entries seeded straight into the file", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")

		content := fmt.Sprintf(`{%q:{"on-resume":"claude --resume old1"},%q:{"on-resume":"claude --resume old2"},"my-session:0.0":{"on-resume":"claude --resume new1"}}`, reapableSeedA, reapableSeedB)
		if err := os.WriteFile(filePath, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write test file: %v", err)
		}

		store := hooks.NewStore(filePath)

		removed, err := store.CleanStale([]string{"my-session:0.0"}, snapshotAll(t, filePath))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(removed) != 2 {
			t.Fatalf("got %d removed, want 2", len(removed))
		}

		sort.Strings(removed)
		if removed[0] != reapableSeedA {
			t.Errorf("removed[0] = %q, want %q", removed[0], reapableSeedA)
		}
		if removed[1] != reapableSeedB {
			t.Errorf("removed[1] = %q, want %q", removed[1], reapableSeedB)
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(h) != 1 {
			t.Fatalf("got %d keys, want 1", len(h))
		}
		if _, ok := h["my-session:0.0"]; !ok {
			t.Error("key my-session:0.0 should have been kept")
		}
		if _, ok := h[reapableSeedA]; ok {
			t.Errorf("key %s should have been removed", reapableSeedA)
		}
		if _, ok := h[reapableSeedB]; ok {
			t.Errorf("key %s should have been removed", reapableSeedB)
		}
	})

	t.Run("handles mix of live and stale keys", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "cmd0", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set(reapableSeedB, "on-resume", "cmd-other0", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set(reapableSeedA, "on-resume", "cmd1", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set("other-session:0.1", "on-resume", "cmd-other1", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		removed, err := store.CleanStale([]string{"my-session:0.0", "other-session:0.1"}, snapshotAll(t, filePath))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if len(removed) != 2 {
			t.Fatalf("got %d removed, want 2", len(removed))
		}

		sort.Strings(removed)
		if removed[0] != reapableSeedA {
			t.Errorf("removed[0] = %q, want %q", removed[0], reapableSeedA)
		}
		if removed[1] != reapableSeedB {
			t.Errorf("removed[1] = %q, want %q", removed[1], reapableSeedB)
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("failed to load: %v", err)
		}
		if len(h) != 2 {
			t.Fatalf("got %d keys, want 2", len(h))
		}
		if _, ok := h["my-session:0.0"]; !ok {
			t.Error("key my-session:0.0 should have been kept")
		}
		if _, ok := h["other-session:0.1"]; !ok {
			t.Error("key other-session:0.1 should have been kept")
		}
	})
}

func TestStaleKeys(t *testing.T) {
	t.Run("returns persisted keys absent from the live set", func(t *testing.T) {
		persisted := map[string]map[string]string{
			"live-a:0.0":  {"on-resume": "x"},
			reapableSeedB: {"on-resume": "y"},
			"live-c:0.0":  {"on-resume": "z"},
			reapableSeedD: {"on-resume": "w"},
		}
		live := []string{"live-a:0.0", "live-c:0.0", "extra-e:0.0"}

		got := hooks.StaleKeys(persisted, live)
		sort.Strings(got)
		want := []string{reapableSeedB, reapableSeedD}
		sort.Strings(want)
		if len(got) != len(want) {
			t.Fatalf("StaleKeys = %v, want %v", got, want)
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("StaleKeys[%d] = %q, want %q", i, got[i], want[i])
			}
		}
	})

	t.Run("returns empty when every persisted key is live", func(t *testing.T) {
		persisted := map[string]map[string]string{
			"a:0.0": {"on-resume": "x"},
			"b:0.0": {"on-resume": "y"},
		}
		got := hooks.StaleKeys(persisted, []string{"a:0.0", "b:0.0", "c:0.0"})
		if len(got) != 0 {
			t.Errorf("StaleKeys = %v, want empty", got)
		}
	})

	t.Run("returns every judgeable persisted key when the live set is empty", func(t *testing.T) {
		persisted := map[string]map[string]string{
			reapableSeedA: {"on-resume": "x"},
			reapableSeedB: {"on-resume": "y"},
		}
		got := hooks.StaleKeys(persisted, []string{})
		if len(got) != 2 {
			t.Fatalf("StaleKeys = %v, want both keys", got)
		}
	})

	t.Run("returns empty for an empty persisted map", func(t *testing.T) {
		got := hooks.StaleKeys(map[string]map[string]string{}, []string{"a:0.0"})
		if len(got) != 0 {
			t.Errorf("StaleKeys = %v, want empty", got)
		}
	})
}

func TestCleanStaleRemovesExactlyStaleKeys(t *testing.T) {
	dir := t.TempDir()
	store := hooks.NewStore(filepath.Join(dir, "hooks.json"))
	for _, k := range []string{reapableSeedA, reapableSeedB, reapableSeedC, reapableSeedD} {
		if err := store.Set(k, "on-resume", "cmd", "cli"); err != nil {
			t.Fatalf("seed set %q: %v", k, err)
		}
	}

	live := []string{reapableSeedA, reapableSeedC}

	persisted, err := store.Load("internal")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	predicted := hooks.StaleKeys(persisted, live)
	sort.Strings(predicted)
	if len(predicted) == 0 {
		t.Fatal("StaleKeys predicted nothing — an equality between two empty sets proves nothing here")
	}

	removed, err := store.CleanStale(live, keysOf(persisted))
	if err != nil {
		t.Fatalf("CleanStale: %v", err)
	}
	sort.Strings(removed)

	if len(removed) != len(predicted) {
		t.Fatalf("CleanStale removed %v, StaleKeys predicted %v", removed, predicted)
	}
	for i := range predicted {
		if removed[i] != predicted[i] {
			t.Errorf("removed[%d] = %q, StaleKeys[%d] = %q", i, removed[i], i, predicted[i])
		}
	}
}

// partitionCleanStaleRecords splits the captured clean-stale records into the
// per-key lines and the batch summaries, so an assertion compares sets rather
// than the map-iteration order the per-key lines are emitted in.
func partitionCleanStaleRecords(t *testing.T, recs []logtest.Record) (perKey, summaries []logtest.Record) {
	t.Helper()
	for _, r := range recs {
		if r.Msg != "clean-stale" {
			t.Errorf("unexpected msg %q in %+v", r.Msg, r)
			continue
		}
		if got := r.AttrString(t, "op"); got != "clean-stale" {
			t.Errorf("op = %q, want %q", got, "clean-stale")
		}
		if got := r.AttrString(t, "component"); got != "hooks" {
			t.Errorf("component = %q, want %q", got, "hooks")
		}
		switch {
		case r.HasAttr("hook_key"):
			perKey = append(perKey, r)
		case r.HasAttr("entries"):
			summaries = append(summaries, r)
		default:
			t.Errorf("clean-stale record is neither per-key nor summary: %+v", r)
		}
	}
	return perKey, summaries
}

func TestCleanStaleLogging(t *testing.T) {
	t.Run("it logs one INFO per removed key carrying the key and the removed command", func(t *testing.T) {
		dir := t.TempDir()
		store := hooks.NewStore(filepath.Join(dir, "hooks.json"))

		if err := store.Set("my-session:0.0", "on-resume", "cmd0", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set(reapableSeedA, "on-resume", "cmd1", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set(reapableSeedB, "on-resume", "cmd2", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		sink := installCapture(t)
		removed, err := store.CleanStale([]string{"my-session:0.0"}, snapshotAll(t, filepath.Join(dir, "hooks.json")))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(removed) != 2 {
			t.Fatalf("got %d removed, want 2", len(removed))
		}

		perKey, summaries := partitionCleanStaleRecords(t, sink.Records())

		if len(perKey) != 2 {
			t.Fatalf("got %d per-key records, want 2: %+v", len(perKey), perKey)
		}
		want := map[string]string{reapableSeedA: "cmd1", reapableSeedB: "cmd2"}
		got := map[string]string{}
		for _, r := range perKey {
			if r.Level != slog.LevelInfo {
				t.Errorf("per-key level = %v, want INFO: %+v", r.Level, r)
			}
			if via := r.AttrString(t, "via"); via != "internal" {
				t.Errorf("per-key via = %q, want %q", via, "internal")
			}
			got[r.AttrString(t, "hook_key")] = r.AttrString(t, "value")
		}
		if !maps.Equal(got, want) {
			t.Errorf("per-key records = %v, want %v", got, want)
		}

		if len(summaries) != 1 {
			t.Fatalf("got %d INFO summary records, want 1: %+v", len(summaries), summaries)
		}
		summary := summaries[0]
		if summary.Level != slog.LevelInfo {
			t.Errorf("summary level = %v, want INFO", summary.Level)
		}
		if got := summary.AttrString(t, "entries"); got != "2" {
			t.Errorf("summary entries = %q, want %q", got, "2")
		}
		if got := summary.AttrString(t, "via"); got != "internal" {
			t.Errorf("summary via = %q, want %q", got, "internal")
		}
		if _, ok := summary.Attrs["took"]; !ok {
			t.Errorf("summary missing took attr: %+v", summary.Attrs)
		}
	})

	t.Run("it emits an empty value for a removed entry with no on-resume event", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		if err := os.WriteFile(path, fmt.Appendf(nil, `{%q:{"on-exit":"x"}}`, reapableSeedA), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		store := hooks.NewStore(path)

		sink := installCapture(t)
		if _, err := store.CleanStale([]string{"my-session:0.0"}, snapshotAll(t, path)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		perKey, _ := partitionCleanStaleRecords(t, sink.Records())
		if len(perKey) != 1 {
			t.Fatalf("got %d per-key records, want 1: %+v", len(perKey), perKey)
		}
		if got := perKey[0].AttrString(t, "hook_key"); got != reapableSeedA {
			t.Errorf("hook_key = %q, want %q", got, reapableSeedA)
		}
		if got := perKey[0].AttrString(t, "value"); got != "" {
			t.Errorf("value = %q, want empty", got)
		}
	})

	t.Run("it emits one line for a key holding several events", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "hooks.json")
		if err := os.WriteFile(path, fmt.Appendf(nil, `{%q:{"on-resume":"cmd1","on-exit":"x"}}`, reapableSeedA), 0o600); err != nil {
			t.Fatalf("seed: %v", err)
		}
		store := hooks.NewStore(path)

		sink := installCapture(t)
		if _, err := store.CleanStale([]string{"my-session:0.0"}, snapshotAll(t, path)); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		perKey, _ := partitionCleanStaleRecords(t, sink.Records())
		if len(perKey) != 1 {
			t.Fatalf("got %d per-key records, want 1: %+v", len(perKey), perKey)
		}
		if got := perKey[0].AttrString(t, "value"); got != "cmd1" {
			t.Errorf("value = %q, want %q", got, "cmd1")
		}
	})

	t.Run("it keeps the per-key lines and warns in the summary when the save fails", func(t *testing.T) {
		body := fmt.Appendf(nil, `{%q:{"on-resume":"cmdA"}}`, reapableSeedA)
		store, seeded := seedThenDenyWrites(t, body)
		sink := installCapture(t)

		if _, err := store.CleanStale([]string{"my-session:0.0"}, snapshotAll(t, seeded)); err == nil {
			t.Fatal("expected error from CleanStale on read-only dir, got nil")
		}

		perKey, summaries := partitionCleanStaleRecords(t, sink.Records())
		if len(perKey) != 1 {
			t.Fatalf("got %d per-key records, want 1: %+v", len(perKey), perKey)
		}
		if perKey[0].Level != slog.LevelInfo {
			t.Errorf("per-key level = %v, want INFO", perKey[0].Level)
		}
		if got := perKey[0].AttrString(t, "hook_key"); got != reapableSeedA {
			t.Errorf("hook_key = %q, want %q", got, reapableSeedA)
		}
		if got := perKey[0].AttrString(t, "value"); got != "cmdA" {
			t.Errorf("value = %q, want %q", got, "cmdA")
		}

		if len(summaries) != 1 {
			t.Fatalf("got %d summary records, want 1: %+v", len(summaries), summaries)
		}
		if summaries[0].Level != slog.LevelWarn {
			t.Errorf("summary level = %v, want WARN", summaries[0].Level)
		}
		if _, ok := summaries[0].Attrs["error"]; !ok {
			t.Errorf("summary missing error attr: %+v", summaries[0].Attrs)
		}
		if got := summaries[0].AttrString(t, "error_class"); got != "write-failed-temp-create" {
			t.Errorf("error_class = %q, want %q", got, "write-failed-temp-create")
		}

		after := readFileBytes(t, seeded)
		if !bytes.Equal(after, body) {
			t.Errorf("hooks.json changed on a failed save\nbefore: %s\nafter:  %s", body, after)
		}
	})

	t.Run("omits entries_failed from the summary when no per-entry failures occur", func(t *testing.T) {
		dir := t.TempDir()
		store := hooks.NewStore(filepath.Join(dir, "hooks.json"))

		if err := store.Set("my-session:0.0", "on-resume", "cmd0", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}
		if err := store.Set(reapableSeedA, "on-resume", "cmd1", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		sink := installCapture(t)
		if _, err := store.CleanStale([]string{"my-session:0.0"}, snapshotAll(t, filepath.Join(dir, "hooks.json"))); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		var summary logtest.Record
		var found bool
		for _, r := range sink.Records() {
			if r.Level == slog.LevelInfo && r.Msg == "clean-stale" {
				summary = r
				found = true
			}
		}
		if !found {
			t.Fatalf("no INFO clean-stale summary captured: %+v", sink.Records())
		}
		if _, ok := summary.Attrs["entries_failed"]; ok {
			t.Errorf("summary must omit entries_failed when no failures: %+v", summary.Attrs)
		}
	})

	t.Run("emits WARN with write-failed-* error_class (not unexpected) when the batched Save fails", func(t *testing.T) {
		store, seeded := seedThenDenyWrites(t, fmt.Appendf(nil, `{%q:{"on-resume":"x"},%q:{"on-resume":"y"}}`, reapableSeedA, reapableSeedB))
		sink := installCapture(t)

		_, err := store.CleanStale([]string{}, snapshotAll(t, seeded))
		if err == nil {
			t.Fatal("expected error from CleanStale on read-only dir, got nil")
		}
		if !errors.Is(err, fileutil.ErrWriteTempCreate) {
			t.Errorf("returned error not classified as temp-create: %v", err)
		}

		var warn logtest.Record
		var found bool
		for _, r := range sink.Records() {
			if r.Level == slog.LevelWarn && r.Msg == "clean-stale" {
				warn = r
				found = true
			}
		}
		if !found {
			t.Fatalf("no WARN clean-stale record captured: %+v", sink.Records())
		}
		if got := warn.AttrString(t, "op"); got != "clean-stale" {
			t.Errorf("op = %q, want %q", got, "clean-stale")
		}
		if got := warn.AttrString(t, "component"); got != "hooks" {
			t.Errorf("component = %q, want %q", got, "hooks")
		}
		if got := warn.AttrString(t, "via"); got != "internal" {
			t.Errorf("via = %q, want %q", got, "internal")
		}
		if got := warn.AttrString(t, "entries"); got != "2" {
			t.Errorf("entries = %q, want %q", got, "2")
		}
		if got := warn.AttrString(t, "error_class"); got != "write-failed-temp-create" {
			t.Errorf("error_class = %q, want %q (must be write-failed-*, not unexpected)", got, "write-failed-temp-create")
		}
		if _, ok := warn.Attrs["took"]; !ok {
			t.Errorf("WARN missing took attr: %+v", warn.Attrs)
		}
		errVal, ok := warn.Attrs["error"]
		if !ok {
			t.Fatalf("WARN record missing error attr: %+v", warn.Attrs)
		}
		loggedErr, ok := errVal.Any().(error)
		if !ok {
			t.Fatalf("error attr is not an error value: %T", errVal.Any())
		}
		if !errors.Is(loggedErr, fileutil.ErrWriteTempCreate) {
			t.Errorf("logged error attr does not wrap the temp-create sentinel: %v", loggedErr)
		}
	})

	t.Run("emits no summary and skips Save when zero entries are removed", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "cmd0", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		infoBefore, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}

		sink := installCapture(t)
		removed, err := store.CleanStale([]string{"my-session:0.0"}, snapshotAll(t, filePath))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(removed) != 0 {
			t.Fatalf("got %d removed, want 0", len(removed))
		}

		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("zero-removal CleanStale emitted %d records, want 0: %+v", len(recs), recs)
		}

		infoAfter, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
			t.Error("file was modified on a zero-removal CleanStale (Save should be skipped)")
		}
	})
}

func TestSetLogging(t *testing.T) {
	t.Run("emits INFO op=set with value and via=cli for a new hook key", func(t *testing.T) {
		dir := t.TempDir()
		store := hooks.NewStore(filepath.Join(dir, "hooks.json"))
		sink := installCapture(t)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		rec := sink.OnlyRecord(t)
		if rec.Level != slog.LevelInfo {
			t.Errorf("level = %v, want INFO", rec.Level)
		}
		if rec.Msg != "set" {
			t.Errorf("msg = %q, want %q", rec.Msg, "set")
		}
		if got := rec.AttrString(t, "op"); got != "set" {
			t.Errorf("op = %q, want %q", got, "set")
		}
		if got := rec.AttrString(t, "component"); got != "hooks" {
			t.Errorf("component = %q, want %q", got, "hooks")
		}
		if got := rec.AttrString(t, "hook_key"); got != "my-session:0.0" {
			t.Errorf("hook_key = %q, want %q", got, "my-session:0.0")
		}
		if got := rec.AttrString(t, "value"); got != "claude --resume abc123" {
			t.Errorf("value = %q, want %q", got, "claude --resume abc123")
		}
		if got := rec.AttrString(t, "via"); got != "cli" {
			t.Errorf("via = %q, want %q", got, "cli")
		}
	})

	t.Run("emits INFO op=modify when the key exists with a different value", func(t *testing.T) {
		dir := t.TempDir()
		store := hooks.NewStore(filepath.Join(dir, "hooks.json"))

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on first set: %v", err)
		}

		sink := installCapture(t)
		if err := store.Set("my-session:0.0", "on-resume", "claude --resume xyz789", "cli"); err != nil {
			t.Fatalf("unexpected error on second set: %v", err)
		}

		rec := sink.OnlyRecord(t)
		if rec.Level != slog.LevelInfo {
			t.Errorf("level = %v, want INFO", rec.Level)
		}
		if rec.Msg != "modify" {
			t.Errorf("msg = %q, want %q", rec.Msg, "modify")
		}
		if got := rec.AttrString(t, "op"); got != "modify" {
			t.Errorf("op = %q, want %q", got, "modify")
		}
		if got := rec.AttrString(t, "component"); got != "hooks" {
			t.Errorf("component = %q, want %q", got, "hooks")
		}
		if got := rec.AttrString(t, "hook_key"); got != "my-session:0.0" {
			t.Errorf("hook_key = %q, want %q", got, "my-session:0.0")
		}
		if got := rec.AttrString(t, "value"); got != "claude --resume xyz789" {
			t.Errorf("value = %q, want %q", got, "claude --resume xyz789")
		}
		if got := rec.AttrString(t, "via"); got != "cli" {
			t.Errorf("via = %q, want %q", got, "cli")
		}
	})

	t.Run("emits DEBUG op=set-noop and skips Save when key+value already match", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "hooks.json")
		store := hooks.NewStore(filePath)

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on first set: %v", err)
		}

		infoBefore, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}

		sink := installCapture(t)
		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on noop set: %v", err)
		}

		rec := sink.OnlyRecord(t)
		if rec.Level != slog.LevelDebug {
			t.Errorf("level = %v, want DEBUG", rec.Level)
		}
		if rec.Msg != "set-noop" {
			t.Errorf("msg = %q, want %q", rec.Msg, "set-noop")
		}
		if got := rec.AttrString(t, "op"); got != "set-noop" {
			t.Errorf("op = %q, want %q", got, "set-noop")
		}
		if got := rec.AttrString(t, "component"); got != "hooks" {
			t.Errorf("component = %q, want %q", got, "hooks")
		}
		if got := rec.AttrString(t, "hook_key"); got != "my-session:0.0" {
			t.Errorf("hook_key = %q, want %q", got, "my-session:0.0")
		}
		if got := rec.AttrString(t, "via"); got != "cli" {
			t.Errorf("via = %q, want %q", got, "cli")
		}
		if _, ok := rec.Attrs["value"]; ok {
			t.Errorf("set-noop record should not carry a value attr: %+v", rec.Attrs)
		}

		infoAfter, err := os.Stat(filePath)
		if err != nil {
			t.Fatalf("failed to stat file: %v", err)
		}
		if !infoBefore.ModTime().Equal(infoAfter.ModTime()) {
			t.Error("file was modified on a set-noop (Save should be skipped)")
		}
	})

	t.Run("emits WARN with error_class=write-failed-temp-create when AtomicWrite fails on Set", func(t *testing.T) {
		path := readOnlyDirPath(t)
		store := hooks.NewStore(path)
		sink := installCapture(t)

		err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli")
		if err == nil {
			t.Fatal("expected error from Set on read-only dir, got nil")
		}
		if !errors.Is(err, fileutil.ErrWriteTempCreate) {
			t.Errorf("returned error not classified as temp-create: %v", err)
		}

		rec := sink.OnlyRecord(t)
		if rec.Level != slog.LevelWarn {
			t.Errorf("level = %v, want WARN", rec.Level)
		}
		if rec.Msg != "set" {
			t.Errorf("msg = %q, want %q", rec.Msg, "set")
		}
		if got := rec.AttrString(t, "op"); got != "set" {
			t.Errorf("op = %q, want %q", got, "set")
		}
		if got := rec.AttrString(t, "component"); got != "hooks" {
			t.Errorf("component = %q, want %q", got, "hooks")
		}
		if got := rec.AttrString(t, "error_class"); got != "write-failed-temp-create" {
			t.Errorf("error_class = %q, want %q", got, "write-failed-temp-create")
		}
		errVal, ok := rec.Attrs["error"]
		if !ok {
			t.Fatalf("WARN record missing error attr: %+v", rec.Attrs)
		}
		loggedErr, ok := errVal.Any().(error)
		if !ok {
			t.Fatalf("error attr is not an error value: %T", errVal.Any())
		}
		if !errors.Is(loggedErr, fileutil.ErrWriteTempCreate) {
			t.Errorf("logged error attr does not wrap the temp-create sentinel: %v", loggedErr)
		}
	})
}

func TestSetEmitsOpAsJSONField(t *testing.T) {
	var buf bytes.Buffer
	log.SetTestHandler(t, slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	dir := t.TempDir()
	store := hooks.NewStore(filepath.Join(dir, "hooks.json"))
	if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var rec map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
		t.Fatalf("failed to parse JSON log line %q: %v", buf.String(), err)
	}
	if got := rec["op"]; got != "set" {
		t.Errorf(`JSON "op" field = %v, want "set" (line: %s)`, got, buf.String())
	}
	if got := rec["component"]; got != "hooks" {
		t.Errorf(`JSON "component" field = %v, want "hooks"`, got)
	}
}

func TestRemoveLogging(t *testing.T) {
	t.Run("it still emits INFO op=rm for a real removal", func(t *testing.T) {
		dir := t.TempDir()
		store := hooks.NewStore(filepath.Join(dir, "hooks.json"))

		if err := store.Set("my-session:0.0", "on-resume", "claude --resume abc123", "cli"); err != nil {
			t.Fatalf("unexpected error on set: %v", err)
		}

		sink := installCapture(t)
		removed, err := store.Remove("my-session:0.0", "on-resume", "cli")
		if err != nil {
			t.Fatalf("unexpected error on remove: %v", err)
		}
		if !removed {
			t.Error("removed = false, want true for a seeded entry")
		}

		rec := sink.OnlyRecord(t)
		if rec.Level != slog.LevelInfo {
			t.Errorf("level = %v, want INFO", rec.Level)
		}
		if rec.Msg != "rm" {
			t.Errorf("msg = %q, want %q", rec.Msg, "rm")
		}
		if got := rec.AttrString(t, "op"); got != "rm" {
			t.Errorf("op = %q, want %q", got, "rm")
		}
		if got := rec.AttrString(t, "component"); got != "hooks" {
			t.Errorf("component = %q, want %q", got, "hooks")
		}
		if got := rec.AttrString(t, "hook_key"); got != "my-session:0.0" {
			t.Errorf("hook_key = %q, want %q", got, "my-session:0.0")
		}
		if got := rec.AttrString(t, "via"); got != "cli" {
			t.Errorf("via = %q, want %q", got, "cli")
		}
		if _, ok := rec.Attrs["value"]; ok {
			t.Errorf("rm record should not carry a value attr: %+v", rec.Attrs)
		}
	})

	t.Run("it emits no record at all when it removes nothing", func(t *testing.T) {
		cases := []struct {
			name string
			seed map[string]map[string]string
			key  string
			ev   string
		}{
			{
				name: "absent key",
				seed: map[string]map[string]string{"my-session:0.0": {"on-resume": "claude --resume abc123"}},
				key:  "nonexistent:9.9",
				ev:   "on-resume",
			},
			{
				name: "absent event",
				seed: map[string]map[string]string{"my-session:0.0": {"on-resume": "claude --resume abc123"}},
				key:  "my-session:0.0",
				ev:   "on-start",
			},
			{
				name: "absent file",
				seed: nil,
				key:  "my-session:0.0",
				ev:   "on-resume",
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				dir := t.TempDir()
				store := hooks.NewStore(filepath.Join(dir, "hooks.json"))
				for key, events := range tc.seed {
					for event, command := range events {
						if err := store.Set(key, event, command, "cli"); err != nil {
							t.Fatalf("unexpected error on set: %v", err)
						}
					}
				}

				sink := installCapture(t)
				removed, err := store.Remove(tc.key, tc.ev, "cli")
				if err != nil {
					t.Fatalf("unexpected error on remove: %v", err)
				}
				if removed {
					t.Error("removed = true, want false")
				}

				if recs := sink.Records(); len(recs) != 0 {
					t.Errorf("a removal that removed nothing emitted %d log records, want 0: %+v", len(recs), recs)
				}
			})
		}
	})

	t.Run("emits WARN with error_class=write-failed-temp-create when AtomicWrite fails on Remove", func(t *testing.T) {
		store, _ := seedThenDenyWrites(t, []byte(`{"my-session:0.0":{"on-resume":"claude --resume abc123"}}`))
		sink := installCapture(t)

		removed, err := store.Remove("my-session:0.0", "on-resume", "cli")
		if err == nil {
			t.Fatal("expected error from Remove on read-only dir, got nil")
		}
		if removed {
			t.Error("removed = true, want false when the save failed")
		}
		if !errors.Is(err, fileutil.ErrWriteTempCreate) {
			t.Errorf("returned error not classified as temp-create: %v", err)
		}

		rec := sink.OnlyRecord(t)
		if rec.Level != slog.LevelWarn {
			t.Errorf("level = %v, want WARN", rec.Level)
		}
		if rec.Msg != "rm" {
			t.Errorf("msg = %q, want %q", rec.Msg, "rm")
		}
		if got := rec.AttrString(t, "op"); got != "rm" {
			t.Errorf("op = %q, want %q", got, "rm")
		}
		if got := rec.AttrString(t, "component"); got != "hooks" {
			t.Errorf("component = %q, want %q", got, "hooks")
		}
		if got := rec.AttrString(t, "error_class"); got != "write-failed-temp-create" {
			t.Errorf("error_class = %q, want %q", got, "write-failed-temp-create")
		}
		errVal, ok := rec.Attrs["error"]
		if !ok {
			t.Fatalf("WARN record missing error attr: %+v", rec.Attrs)
		}
		loggedErr, ok := errVal.Any().(error)
		if !ok {
			t.Fatalf("error attr is not an error value: %T", errVal.Any())
		}
		if !errors.Is(loggedErr, fileutil.ErrWriteTempCreate) {
			t.Errorf("logged error attr does not wrap the temp-create sentinel: %v", loggedErr)
		}
	})
}
