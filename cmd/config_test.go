package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/xdg"
)

func TestConfigFilePath(t *testing.T) {
	// TestMain poisons HOME package-wide, so a subtest resolving the default
	// location can never reach the developer's files. Pinning a temp HOME of its
	// own buys a subtest a home no other subtest writes into, which is what lets
	// it assert on the exact path and on what the migration moved there.
	t.Run("it resolves projects.json under the temp home when XDG_CONFIG_HOME is empty", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)
		t.Setenv("XDG_CONFIG_HOME", "")

		got, err := configFilePath(xdg.ConfigFileID{EnvVar: "TEST_CONFIG_UNSET", Filename: "projects.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := filepath.Join(homeDir, ".config", "portal", "projects.json")
		if got != want {
			t.Errorf("configFilePath() = %q, want %q", got, want)
		}
	})

	t.Run("returns env var value when per-file env var is set", func(t *testing.T) {
		t.Setenv("TEST_CONFIG_PATH", "/custom/path/file.json")

		got, err := configFilePath(xdg.ConfigFileID{EnvVar: "TEST_CONFIG_PATH", Filename: "file.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := "/custom/path/file.json"
		if got != want {
			t.Errorf("configFilePath() = %q, want %q", got, want)
		}
	})

	t.Run("respects XDG_CONFIG_HOME when set", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")

		got, err := configFilePath(xdg.ConfigFileID{EnvVar: "TEST_CONFIG_UNSET", Filename: "projects.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := filepath.Join("/tmp/xdg-config", "portal", "projects.json")
		if got != want {
			t.Errorf("configFilePath() = %q, want %q", got, want)
		}
	})

	t.Run("treats empty XDG_CONFIG_HOME as unset", func(t *testing.T) {
		homeDir := t.TempDir()
		t.Setenv("HOME", homeDir)
		t.Setenv("XDG_CONFIG_HOME", "")

		got, err := configFilePath(xdg.ConfigFileID{EnvVar: "TEST_CONFIG_UNSET", Filename: "projects.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := filepath.Join(homeDir, ".config", "portal", "projects.json")
		if got != want {
			t.Errorf("configFilePath() = %q, want %q", got, want)
		}
	})

	t.Run("per-file env var takes precedence over XDG_CONFIG_HOME", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
		t.Setenv("TEST_OVERRIDE", "/explicit/override/file.json")

		got, err := configFilePath(xdg.ConfigFileID{EnvVar: "TEST_OVERRIDE", Filename: "file.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := "/explicit/override/file.json"
		if got != want {
			t.Errorf("configFilePath() = %q, want %q", got, want)
		}
	})

	t.Run("XDG_CONFIG_HOME with trailing slash is normalized", func(t *testing.T) {
		t.Setenv("HOME", t.TempDir())
		t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config/")

		got, err := configFilePath(xdg.ConfigFileID{EnvVar: "TEST_CONFIG_UNSET", Filename: "hooks.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := filepath.Join("/tmp/xdg-config", "portal", "hooks.json")
		if got != want {
			t.Errorf("configFilePath() = %q, want %q", got, want)
		}
	})
}

func TestMigrateConfigFile(t *testing.T) {
	t.Run("migration is no-op when old directory does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldPath := filepath.Join(tmpDir, "nonexistent", "portal", "projects.json")
		newPath := filepath.Join(tmpDir, ".config", "portal", "projects.json")

		migrateConfigFile(oldPath, newPath, "projects")

		if _, err := os.Stat(newPath); !os.IsNotExist(err) {
			t.Errorf("new file should not exist when old file does not exist")
		}
	})

	t.Run("migration does not overwrite existing file at new path", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldDir := filepath.Join(tmpDir, "Library", "Application Support", "portal")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("failed to create old dir: %v", err)
		}
		oldPath := filepath.Join(oldDir, "projects.json")
		if err := os.WriteFile(oldPath, []byte("old content"), 0o644); err != nil {
			t.Fatalf("failed to write old file: %v", err)
		}

		newDir := filepath.Join(tmpDir, ".config", "portal")
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			t.Fatalf("failed to create new dir: %v", err)
		}
		newPath := filepath.Join(newDir, "projects.json")
		if err := os.WriteFile(newPath, []byte("new content"), 0o644); err != nil {
			t.Fatalf("failed to write new file: %v", err)
		}

		migrateConfigFile(oldPath, newPath, "projects")

		data, err := os.ReadFile(newPath)
		if err != nil {
			t.Fatalf("failed to read new file: %v", err)
		}
		if string(data) != "new content" {
			t.Errorf("new file content = %q, want %q (should not be overwritten)", string(data), "new content")
		}

		if _, err := os.Stat(oldPath); err != nil {
			t.Errorf("old file should still exist when new path is occupied: %v", err)
		}
	})

	t.Run("migration handles partial state", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldDir := filepath.Join(tmpDir, "Library", "Application Support", "portal")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("failed to create old dir: %v", err)
		}
		oldAliases := filepath.Join(oldDir, "aliases")
		if err := os.WriteFile(oldAliases, []byte("a=/path/a"), 0o644); err != nil {
			t.Fatalf("failed to write old aliases: %v", err)
		}

		newDir := filepath.Join(tmpDir, ".config", "portal")
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			t.Fatalf("failed to create new dir: %v", err)
		}
		newProjects := filepath.Join(newDir, "projects.json")
		if err := os.WriteFile(newProjects, []byte("existing"), 0o644); err != nil {
			t.Fatalf("failed to write new projects: %v", err)
		}

		newAliases := filepath.Join(newDir, "aliases")

		migrateConfigFile(oldAliases, newAliases, "aliases")

		data, err := os.ReadFile(newAliases)
		if err != nil {
			t.Fatalf("failed to read migrated aliases: %v", err)
		}
		if string(data) != "a=/path/a" {
			t.Errorf("aliases content = %q, want %q", string(data), "a=/path/a")
		}

		oldProjects := filepath.Join(oldDir, "projects.json")
		migrateConfigFile(oldProjects, newProjects, "projects")

		projData, err := os.ReadFile(newProjects)
		if err != nil {
			t.Fatalf("failed to read projects: %v", err)
		}
		if string(projData) != "existing" {
			t.Errorf("projects content = %q, want %q", string(projData), "existing")
		}
	})

	t.Run("migration cleans up empty old directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldDir := filepath.Join(tmpDir, "Library", "Application Support", "portal")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("failed to create old dir: %v", err)
		}
		oldPath := filepath.Join(oldDir, "projects.json")
		if err := os.WriteFile(oldPath, []byte("data"), 0o644); err != nil {
			t.Fatalf("failed to write old file: %v", err)
		}

		newDir := filepath.Join(tmpDir, ".config", "portal")
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			t.Fatalf("failed to create new dir: %v", err)
		}
		newPath := filepath.Join(newDir, "projects.json")

		migrateConfigFile(oldPath, newPath, "projects")

		if _, err := os.Stat(oldDir); !os.IsNotExist(err) {
			t.Errorf("old directory should be removed when empty after migration")
		}
	})

	t.Run("migration preserves non-empty old directory", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldDir := filepath.Join(tmpDir, "Library", "Application Support", "portal")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("failed to create old dir: %v", err)
		}
		oldPath := filepath.Join(oldDir, "projects.json")
		if err := os.WriteFile(oldPath, []byte("data"), 0o644); err != nil {
			t.Fatalf("failed to write old file: %v", err)
		}
		otherFile := filepath.Join(oldDir, "hooks.json")
		if err := os.WriteFile(otherFile, []byte("hooks"), 0o644); err != nil {
			t.Fatalf("failed to write other file: %v", err)
		}

		newDir := filepath.Join(tmpDir, ".config", "portal")
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			t.Fatalf("failed to create new dir: %v", err)
		}
		newPath := filepath.Join(newDir, "projects.json")

		migrateConfigFile(oldPath, newPath, "projects")

		if _, err := os.Stat(oldDir); err != nil {
			t.Errorf("old directory should be preserved when non-empty: %v", err)
		}

		if _, err := os.Stat(otherFile); err != nil {
			t.Errorf("other file in old directory should still exist: %v", err)
		}
	})

	t.Run("migration creates target directory if missing", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldDir := filepath.Join(tmpDir, "Library", "Application Support", "portal")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("failed to create old dir: %v", err)
		}
		oldPath := filepath.Join(oldDir, "aliases")
		if err := os.WriteFile(oldPath, []byte("x=/y"), 0o644); err != nil {
			t.Fatalf("failed to write old file: %v", err)
		}

		newPath := filepath.Join(tmpDir, ".config", "portal", "aliases")

		migrateConfigFile(oldPath, newPath, "aliases")

		data, err := os.ReadFile(newPath)
		if err != nil {
			t.Fatalf("failed to read migrated file: %v", err)
		}
		if string(data) != "x=/y" {
			t.Errorf("migrated file content = %q, want %q", string(data), "x=/y")
		}
	})

	t.Run("migration is skipped when stat of new path returns non-not-found error", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldDir := filepath.Join(tmpDir, "Library", "Application Support", "portal")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("failed to create old dir: %v", err)
		}
		oldPath := filepath.Join(oldDir, "projects.json")
		if err := os.WriteFile(oldPath, []byte("old data"), 0o644); err != nil {
			t.Fatalf("failed to write old file: %v", err)
		}

		// Unreadable parent so os.Stat(newPath) returns a permission error rather
		// than not-exist.
		newDir := filepath.Join(tmpDir, ".config", "portal")
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			t.Fatalf("failed to create new dir: %v", err)
		}
		if err := os.Chmod(newDir, 0o000); err != nil {
			t.Fatalf("failed to chmod new dir: %v", err)
		}
		t.Cleanup(func() {
			_ = os.Chmod(newDir, 0o755)
		})

		newPath := filepath.Join(newDir, "projects.json")

		migrateConfigFile(oldPath, newPath, "projects")

		if _, err := os.Stat(oldPath); err != nil {
			t.Errorf("old file should still exist when stat of new path fails with non-not-found error: %v", err)
		}
	})

	t.Run("migrates file from old macOS path to new path", func(t *testing.T) {
		tmpDir := t.TempDir()

		oldDir := filepath.Join(tmpDir, "Library", "Application Support", "portal")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("failed to create old dir: %v", err)
		}
		oldPath := filepath.Join(oldDir, "projects.json")
		if err := os.WriteFile(oldPath, []byte(`{"projects":[]}`), 0o644); err != nil {
			t.Fatalf("failed to write old file: %v", err)
		}

		newDir := filepath.Join(tmpDir, ".config", "portal")
		if err := os.MkdirAll(newDir, 0o755); err != nil {
			t.Fatalf("failed to create new dir: %v", err)
		}
		newPath := filepath.Join(newDir, "projects.json")

		migrateConfigFile(oldPath, newPath, "projects")

		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Errorf("old file should not exist after migration")
		}

		data, err := os.ReadFile(newPath)
		if err != nil {
			t.Fatalf("failed to read new file: %v", err)
		}
		if string(data) != `{"projects":[]}` {
			t.Errorf("migrated file content = %q, want %q", string(data), `{"projects":[]}`)
		}
	})
}

func TestConfigFilePathMigration(t *testing.T) {
	t.Run("migration does not run when per-file env var is set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)
		t.Setenv("XDG_CONFIG_HOME", "")

		oldDir := filepath.Join(tmpDir, "Library", "Application Support", "portal")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("failed to create old dir: %v", err)
		}
		oldPath := filepath.Join(oldDir, "projects.json")
		if err := os.WriteFile(oldPath, []byte("old data"), 0o644); err != nil {
			t.Fatalf("failed to write old file: %v", err)
		}

		overridePath := filepath.Join(tmpDir, "custom", "projects.json")
		t.Setenv("TEST_MIGRATE_ENVVAR", overridePath)

		got, err := configFilePath(xdg.ConfigFileID{EnvVar: "TEST_MIGRATE_ENVVAR", Filename: "projects.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got != overridePath {
			t.Errorf("configFilePath() = %q, want %q", got, overridePath)
		}

		if _, err := os.Stat(oldPath); err != nil {
			t.Errorf("old file should still exist when env var override is active: %v", err)
		}
	})

	t.Run("it migrates only the temp home's legacy config directory", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", "")

		oldDir := filepath.Join(home, "Library", "Application Support", "portal")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("failed to create old dir: %v", err)
		}
		oldPath := filepath.Join(oldDir, "projects.json")
		if err := os.WriteFile(oldPath, []byte(`{"projects":[]}`), 0o644); err != nil {
			t.Fatalf("failed to write old file: %v", err)
		}

		got, err := configFilePath(xdg.ConfigFileID{EnvVar: "TEST_MIGRATE_HOME_UNSET", Filename: "projects.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := filepath.Join(home, ".config", "portal", "projects.json")
		if got != want {
			t.Errorf("configFilePath() = %q, want %q", got, want)
		}

		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Errorf("stat the legacy file = %v, want it moved out of the temp home", err)
		}

		data, err := os.ReadFile(want)
		if err != nil {
			t.Fatalf("failed to read the migrated file: %v", err)
		}
		if string(data) != `{"projects":[]}` {
			t.Errorf("migrated file content = %q, want %q", string(data), `{"projects":[]}`)
		}
	})

	t.Run("migration runs when XDG_CONFIG_HOME is set", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("HOME", tmpDir)

		xdgDir := filepath.Join(tmpDir, "custom-xdg")
		t.Setenv("XDG_CONFIG_HOME", xdgDir)

		oldDir := filepath.Join(tmpDir, "Library", "Application Support", "portal")
		if err := os.MkdirAll(oldDir, 0o755); err != nil {
			t.Fatalf("failed to create old dir: %v", err)
		}
		oldPath := filepath.Join(oldDir, "aliases")
		if err := os.WriteFile(oldPath, []byte("a=/x"), 0o644); err != nil {
			t.Fatalf("failed to write old file: %v", err)
		}

		got, err := configFilePath(xdg.ConfigFileID{EnvVar: "TEST_MIGRATE_XDG_UNSET", Filename: "aliases"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := filepath.Join(xdgDir, "portal", "aliases")
		if got != want {
			t.Errorf("configFilePath() = %q, want %q", got, want)
		}

		if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
			t.Errorf("old file should not exist after migration")
		}

		data, err := os.ReadFile(want)
		if err != nil {
			t.Fatalf("failed to read migrated file: %v", err)
		}
		if string(data) != "a=/x" {
			t.Errorf("migrated file content = %q, want %q", string(data), "a=/x")
		}
	})
}
