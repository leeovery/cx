package cmd

import (
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/xdg"
)

// TestConfigFileIdentity pins what a single declaration of a config file's
// identity buys: the production route, the shared rule and the test seeder all
// name the same file because they all read the same pair, so renaming either
// half cannot move one route without moving the others.
func TestConfigFileIdentity(t *testing.T) {
	t.Run("it resolves hooks.json from the shared file identity", func(t *testing.T) {
		base := t.TempDir()
		home := t.TempDir()
		t.Setenv("HOME", home)
		t.Setenv("XDG_CONFIG_HOME", base)
		t.Setenv("PORTAL_HOOKS_FILE", "")
		env := []string{"HOME=" + home, "XDG_CONFIG_HOME=" + base}

		shared, err := xdg.ConfigFilePath(xdg.EnvSlice(env), xdg.HooksFile)
		if err != nil {
			t.Fatalf("shared rule: %v", err)
		}

		production, err := hooksFilePath()
		if err != nil {
			t.Fatalf("production route: %v", err)
		}
		seeder := hookstest.ResolveHooksFilePathFromEnv(t, env)

		want := filepath.Join(base, "portal", "hooks.json")
		for _, got := range []struct {
			route string
			path  string
		}{
			{"the shared identity", shared.Path},
			{"the production route", production},
			{"the seeder", seeder},
		} {
			if got.path != want {
				t.Errorf("%s resolved %q, want %q", got.route, got.path, want)
			}
		}
	})

	t.Run("the identity declares the env var and filename together", func(t *testing.T) {
		if got, want := xdg.HooksFile.EnvVar, "PORTAL_HOOKS_FILE"; got != want {
			t.Errorf("HooksFile.EnvVar = %q, want %q", got, want)
		}
		if got, want := xdg.HooksFile.Filename, "hooks.json"; got != want {
			t.Errorf("HooksFile.Filename = %q, want %q", got, want)
		}
		if got, want := xdg.HooksFile.LogComponent, "hooks"; got != want {
			t.Errorf("HooksFile.LogComponent = %q, want %q", got, want)
		}
		if got := xdg.PrefsFile.LogComponent; got != "" {
			t.Errorf("PrefsFile.LogComponent = %q, want the empty component — prefs.json is outside the log vocabulary", got)
		}
	})
}
