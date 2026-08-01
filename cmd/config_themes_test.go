package cmd

import (
	"maps"
	"os"
	"path/filepath"
	"testing"
)

// TestThemesDirPath_EnvVarWins: it returns the env var when set.
//
// PORTAL_THEMES_DIR is the first rung of the chain, so it must win over both
// XDG_CONFIG_HOME and HOME — the same precedence configFilePath gives its
// per-file env vars.
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

// TestThemesDirPath_EmptyEnvFallsThrough: it falls through when the env var is
// empty.
//
// An empty PORTAL_THEMES_DIR must be treated as unset. A naive os.LookupEnv
// would return ("", true) and point the loader at the process working
// directory.
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

// TestThemesDirPath_XDGConfigHome: it resolves under XDG_CONFIG_HOME.
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

// TestThemesDirPath_HomeFallback: it falls back to ~/.config when
// XDG_CONFIG_HOME is unset.
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

// TestThemesDirPath_HomeResolutionFailurePropagates: it propagates a
// home-directory resolution failure.
//
// With no env var and no XDG_CONFIG_HOME, xdg.ConfigBase's home-dir read is the
// only remaining source. Its failure must surface as an error rather than an
// empty path (which a caller would happily read the CWD through) or a panic.
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

// TestThemesDirPath_NeverCreatesDirectory: it never creates or seeds the
// directory.
//
// An absent themes directory is the common, silent case, which is why the
// published drop-in workflow starts with `mkdir -p`. themesDirPath returns a
// path and makes no judgement about what is there — no MkdirAll, no Stat, no
// seeding — however many times it is called.
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
		// The intermediate portal/ directory must not appear either.
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

// assertAbsent fails the test if path exists on disk.
func assertAbsent(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("themesDirPath() must create nothing, but %q exists (stat err = %v)", path, err)
	}
}

// TestThemesDirPath_IsNotAConfigFilePathMember: it is not a configFilePath
// member and emits no migrate breadcrumb.
//
// The themes directory resolves a *directory* where configFilePath resolves
// *files*, and it has no old-macOS predecessor to migrate from — so it takes no
// configFileComponents entry and must not run the one-shot move. Routing it
// through configFilePath would migrate (and therefore MOVE) anything sitting at
// ~/Library/Application Support/portal/themes and emit a breadcrumb for it.
func TestThemesDirPath_IsNotAConfigFilePathMember(t *testing.T) {
	t.Run("configFileComponents is unchanged", func(t *testing.T) {
		want := map[string]string{
			"hooks.json":     "hooks",
			"aliases":        "aliases",
			"projects.json":  "projects",
			"prefs.json":     "",
			"terminals.json": "",
		}
		if !maps.Equal(configFileComponents, want) {
			t.Errorf("configFileComponents = %v, want %v — the themes directory takes no entry", configFileComponents, want)
		}
	})

	t.Run("emits no migrate breadcrumb and moves nothing", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Setenv("PORTAL_THEMES_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(tmpDir, "xdg"))
		t.Setenv("HOME", tmpDir)

		// Seed the old macOS path so a configFilePath-shaped implementation
		// would visibly migrate it.
		oldThemes := filepath.Join(tmpDir, "Library", "Application Support", "portal", "themes")
		if err := os.MkdirAll(oldThemes, 0o755); err != nil {
			t.Fatalf("failed to seed old macOS themes dir: %v", err)
		}

		sink := installMigrateCapture(t)

		got, err := themesDirPath()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("expected no log records from themesDirPath, got %d: %+v", len(recs), recs)
		}
		if _, err := os.Stat(oldThemes); err != nil {
			t.Errorf("old macOS themes dir must be left untouched: %v", err)
		}
		assertAbsent(t, got)
	})
}
