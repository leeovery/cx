package cmd

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/xdg"
)

func TestThemesDirPath_EnvVarWins(t *testing.T) {
	t.Setenv("PORTAL_THEMES_DIR", "/tmp/x")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/cfg")
	t.Setenv("HOME", "/tmp/h")

	got, err := themesDirPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := "/tmp/x"; got != want {
		t.Errorf("themesDirPath() = %q, want %q", got, want)
	}
}

func TestThemesDirPath_EmptyEnvFallsThrough(t *testing.T) {
	t.Setenv("PORTAL_THEMES_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/cfg")
	t.Setenv("HOME", "/tmp/h")

	got, err := themesDirPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := filepath.Join("/tmp/cfg", "portal", "themes"); got != want {
		t.Errorf("themesDirPath() = %q, want %q", got, want)
	}
}

func TestThemesDirPath_XDGConfigHome(t *testing.T) {
	t.Setenv("PORTAL_THEMES_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	t.Setenv("HOME", "/tmp/h")

	got, err := themesDirPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := filepath.Join("/tmp/xdg-config", "portal", "themes"); got != want {
		t.Errorf("themesDirPath() = %q, want %q", got, want)
	}
}

func TestThemesDirPath_HomeFallback(t *testing.T) {
	t.Setenv("PORTAL_THEMES_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "/tmp/h")

	got, err := themesDirPath()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if want := filepath.Join("/tmp/h", ".config", "portal", "themes"); got != want {
		t.Errorf("themesDirPath() = %q, want %q", got, want)
	}
}

func TestThemesDirPath_HomeResolutionFailurePropagates(t *testing.T) {
	t.Setenv("PORTAL_THEMES_DIR", "")
	t.Setenv("XDG_CONFIG_HOME", "")
	t.Setenv("HOME", "")

	got, err := themesDirPath()
	if err == nil {
		t.Fatalf("themesDirPath() = %q, want an error when the home directory cannot be resolved", got)
	}
	if got != "" {
		t.Errorf("themesDirPath() = %q on error, want the empty path", got)
	}
}

func TestThemesDirPath_NeverCreatesDirectory(t *testing.T) {
	t.Run("resolved under XDG_CONFIG_HOME", func(t *testing.T) {
		base := t.TempDir()
		t.Setenv("PORTAL_THEMES_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", base)
		t.Setenv("HOME", base)

		var got string
		for range 3 {
			p, err := themesDirPath()
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got = p
		}

		assertAbsent(t, got)
		assertAbsent(t, filepath.Dir(got))
	})

	t.Run("resolved from PORTAL_THEMES_DIR", func(t *testing.T) {
		dir := filepath.Join(t.TempDir(), "themes")
		t.Setenv("PORTAL_THEMES_DIR", dir)

		for range 3 {
			if _, err := themesDirPath(); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		}

		assertAbsent(t, dir)
	})
}

func assertAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("themesDirPath() must create nothing, but %q exists (stat err = %v)", path, err)
	}
}

func TestThemesDirPath_IsNotAConfigFileMember(t *testing.T) {
	t.Run("it resolves the themes dir with no migration", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("PORTAL_THEMES_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
		t.Setenv("HOME", tmpDir)

		// Seeded so a config-file-shaped implementation would visibly migrate it.
		oldThemes := filepath.Join(tmpDir, "Library", "Application Support", "portal", "themes")
		if err := os.MkdirAll(oldThemes, 0o755); err != nil {
			t.Fatalf("failed to seed old macOS themes dir: %v", err)
		}

		sink := logtest.Install(t)

		got, err := themesDirPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		shared, err := xdg.ConfigDirPath(xdg.OSEnv, xdg.ThemesDir)
		if err != nil {
			t.Fatalf("shared rule: %v", err)
		}
		if got != shared {
			t.Errorf("themesDirPath() = %q, want %q — the directory rule has one declaration", got, shared)
		}

		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("expected no log records from themesDirPath, got %d: %+v", len(recs), recs)
		}
		if _, err := os.Stat(oldThemes); err != nil {
			t.Errorf("old macOS themes dir must be left untouched: %v", err)
		}
		assertAbsent(t, got)
	})

	t.Run("the themes directory is named as a directory, not a config file", func(t *testing.T) {
		if got, want := xdg.ThemesDir.EnvVar, "PORTAL_THEMES_DIR"; got != want {
			t.Errorf("ThemesDir.EnvVar = %q, want %q", got, want)
		}
		if got, want := xdg.ThemesDir.Dirname, "themes"; got != want {
			t.Errorf("ThemesDir.Dirname = %q, want %q", got, want)
		}
	})
}
