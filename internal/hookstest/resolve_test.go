package hookstest_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/hookstest"
)

func TestResolveHooksFilePathFromEnv(t *testing.T) {
	t.Run("it resolves PORTAL_HOOKS_FILE ahead of XDG_CONFIG_HOME", func(t *testing.T) {
		explicit := filepath.Join(t.TempDir(), "explicit-hooks.json")
		env := []string{"XDG_CONFIG_HOME=" + t.TempDir(), "PORTAL_HOOKS_FILE=" + explicit}

		if got := hookstest.ResolveHooksFilePathFromEnv(t, env); got != explicit {
			t.Errorf("resolved %q, want %q", got, explicit)
		}
	})

	t.Run("it resolves under XDG_CONFIG_HOME when no file variable is set", func(t *testing.T) {
		base := t.TempDir()

		got := hookstest.ResolveHooksFilePathFromEnv(t, []string{"XDG_CONFIG_HOME=" + base})

		if want := filepath.Join(base, "portal", "hooks.json"); got != want {
			t.Errorf("resolved %q, want %q", got, want)
		}
	})

	t.Run("it reads the last of a repeated variable, as the subprocess would", func(t *testing.T) {
		last := filepath.Join(t.TempDir(), "last.json")
		env := []string{"PORTAL_HOOKS_FILE=" + filepath.Join(t.TempDir(), "first.json"), "PORTAL_HOOKS_FILE=" + last}

		if got := hookstest.ResolveHooksFilePathFromEnv(t, env); got != last {
			t.Errorf("resolved %q, want %q — exec.Cmd dedupes last-wins", got, last)
		}
	})

	t.Run("it triggers no config migration on the seeder path", func(t *testing.T) {
		home := t.TempDir()
		t.Setenv("HOME", home)
		legacyDir := filepath.Join(home, "Library", "Application Support", "portal")
		if err := os.MkdirAll(legacyDir, 0o700); err != nil {
			t.Fatalf("stage the legacy config dir: %v", err)
		}
		legacy := filepath.Join(legacyDir, "hooks.json")
		if err := os.WriteFile(legacy, []byte(`{"legacy":{"on-resume":"untouched"}}`), 0o600); err != nil {
			t.Fatalf("stage the legacy hooks.json: %v", err)
		}
		base := t.TempDir()
		env := []string{"HOME=" + home, "XDG_CONFIG_HOME=" + base}

		path := hookstest.ResolveHooksFilePathFromEnv(t, env)

		if _, err := os.Stat(legacy); err != nil {
			t.Errorf("stat the legacy hooks.json after resolving: %v — resolving moved a real config file", err)
		}
		if _, err := os.Stat(filepath.Dir(path)); !os.IsNotExist(err) {
			t.Errorf("resolving created %s (stat err = %v), want a resolver that creates nothing the production read would not", filepath.Dir(path), err)
		}

		hookstest.SeedHooksJSON(t, env, map[string]string{hookstest.ReapableSeedA: "seeded"})

		if _, err := os.Stat(legacy); err != nil {
			t.Errorf("stat the legacy hooks.json after seeding: %v — seeding moved a real config file", err)
		}
		if got := hookstest.HooksJSONBytes(t, env); !strings.Contains(string(got), "seeded") {
			t.Errorf("seeded hooks.json = %s, want the seeded entry at the resolved path", got)
		}
	})
}

// fatalChildEnv names the variable that turns this test binary into the child
// half of the fatal case: the seeder's tripwire is a t.Fatalf, which only a
// separate process can survive observing.
const fatalChildEnv = "HOOKSTEST_RESOLVE_FATAL_CHILD"

func TestResolveHooksFilePathFromEnv_FatalsWithoutEitherVariable(t *testing.T) {
	t.Run("it fatals when the env slice carries neither variable", func(t *testing.T) {
		if os.Getenv(fatalChildEnv) == "1" {
			hookstest.ResolveHooksFilePathFromEnv(t, []string{"HOME=/nonexistent", "PATH=/usr/bin"})
			return
		}

		child := exec.Command(os.Args[0], "-test.run=TestResolveHooksFilePathFromEnv_FatalsWithoutEitherVariable")
		child.Env = append(os.Environ(), fatalChildEnv+"=1")
		out, err := child.CombinedOutput()

		if err == nil {
			t.Fatalf("the child passed, want a fatal:\n%s", out)
		}
		if !strings.Contains(string(out), "neither PORTAL_HOOKS_FILE nor XDG_CONFIG_HOME") {
			t.Errorf("child output names no isolation regression:\n%s", out)
		}
	})
}
