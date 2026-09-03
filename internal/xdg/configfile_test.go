package xdg_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/xdg"
)

func TestEnvSlice(t *testing.T) {
	t.Run("it reads a variable out of an exec.Cmd-style slice", func(t *testing.T) {
		lookup := xdg.EnvSlice([]string{"HOME=/home/x", "XDG_CONFIG_HOME=/cfg"})

		if got := lookup("XDG_CONFIG_HOME"); got != "/cfg" {
			t.Errorf("lookup(XDG_CONFIG_HOME) = %q, want %q", got, "/cfg")
		}
	})

	t.Run("it reports the empty string for an absent variable", func(t *testing.T) {
		lookup := xdg.EnvSlice([]string{"HOME=/home/x"})

		if got := lookup("XDG_CONFIG_HOME"); got != "" {
			t.Errorf("lookup(XDG_CONFIG_HOME) = %q, want the empty string", got)
		}
	})

	t.Run("a later entry wins, as it does for the subprocess the slice launches", func(t *testing.T) {
		lookup := xdg.EnvSlice([]string{"XDG_CONFIG_HOME=/first", "XDG_CONFIG_HOME=/last"})

		if got := lookup("XDG_CONFIG_HOME"); got != "/last" {
			t.Errorf("lookup(XDG_CONFIG_HOME) = %q, want %q — exec.Cmd dedupes last-wins", got, "/last")
		}
	})

	t.Run("it matches on the whole name, not a prefix of it", func(t *testing.T) {
		lookup := xdg.EnvSlice([]string{"XDG_CONFIG_HOME_EXTRA=/nope"})

		if got := lookup("XDG_CONFIG_HOME"); got != "" {
			t.Errorf("lookup(XDG_CONFIG_HOME) = %q, want the empty string", got)
		}
	})
}

func TestConfigFilePath(t *testing.T) {
	t.Run("the per-file variable wins ahead of XDG_CONFIG_HOME", func(t *testing.T) {
		lookup := xdg.EnvSlice([]string{"XDG_CONFIG_HOME=/cfg", "PORTAL_HOOKS_FILE=/explicit/hooks.json"})

		got, err := xdg.ConfigFilePath(lookup, xdg.HooksFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if got.Path != "/explicit/hooks.json" {
			t.Errorf("Path = %q, want %q", got.Path, "/explicit/hooks.json")
		}
		if !got.Overridden {
			t.Error("Overridden = false, want true — the path came from the per-file variable")
		}
	})

	t.Run("it resolves under XDG_CONFIG_HOME when no file variable is set", func(t *testing.T) {
		lookup := xdg.EnvSlice([]string{"XDG_CONFIG_HOME=/cfg"})

		got, err := xdg.ConfigFilePath(lookup, xdg.HooksFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		want := filepath.Join("/cfg", "portal", "hooks.json")
		if got.Path != want {
			t.Errorf("Path = %q, want %q", got.Path, want)
		}
		if got.Overridden {
			t.Error("Overridden = true, want false — the path came from the config base")
		}
	})

	t.Run("an empty per-file variable is treated as unset", func(t *testing.T) {
		lookup := xdg.EnvSlice([]string{"XDG_CONFIG_HOME=/cfg", "PORTAL_HOOKS_FILE="})

		got, err := xdg.ConfigFilePath(lookup, xdg.HooksFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if want := filepath.Join("/cfg", "portal", "hooks.json"); got.Path != want {
			t.Errorf("Path = %q, want %q", got.Path, want)
		}
	})

	t.Run("it falls back to the home config base when the lookup carries neither", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)

		got, err := xdg.ConfigFilePath(xdg.EnvSlice(nil), xdg.HooksFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if want := filepath.Join(home, ".config", "portal", "hooks.json"); got.Path != want {
			t.Errorf("Path = %q, want %q", got.Path, want)
		}
	})

	t.Run("it resolves against the process environment through OSEnv", func(t *testing.T) {
		t.Setenv("XDG_CONFIG_HOME", "/cfg-from-process")

		got, err := xdg.ConfigFilePath(xdg.OSEnv, xdg.ConfigFileID{EnvVar: "PORTAL_HOOKS_FILE_UNSET", Filename: "hooks.json"})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if want := filepath.Join("/cfg-from-process", "portal", "hooks.json"); got.Path != want {
			t.Errorf("Path = %q, want %q", got.Path, want)
		}
	})

	t.Run("it resolves a path and creates nothing", func(t *testing.T) {
		base := t.TempDir()
		lookup := xdg.EnvSlice([]string{"XDG_CONFIG_HOME=" + base})

		got, err := xdg.ConfigFilePath(lookup, xdg.HooksFile)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if _, err := os.Stat(filepath.Dir(got.Path)); !os.IsNotExist(err) {
			t.Errorf("resolving created %s (stat err = %v), want a resolver that touches the filesystem not at all", filepath.Dir(got.Path), err)
		}
	})
}
