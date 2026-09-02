package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/hookstest"
)

// TestSeederResolvesTheProductionRule pins the delegation that makes a seeded
// hooks.json and the hooks.json the binary under test reads the same file. Both
// routes are driven over one env: the production route through the process
// environment, the seeder route through the equivalent slice. A route that
// grows an env layer the other does not — the failure mode that turns a
// destructive integration suite green while it asserts on a file nothing
// touched — shows up here as two different paths.
func TestSeederResolvesTheProductionRule(t *testing.T) {
	t.Run("it resolves the same path as the production rule for the same env", func(t *testing.T) {
		base := t.TempDir()
		explicit := filepath.Join(t.TempDir(), "explicit-hooks.json")

		cases := []struct {
			name string
			env  [][2]string
		}{
			{
				name: "the per-file variable alone",
				env:  [][2]string{{"XDG_CONFIG_HOME", ""}, {"PORTAL_HOOKS_FILE", explicit}},
			},
			{
				name: "XDG_CONFIG_HOME alone",
				env:  [][2]string{{"XDG_CONFIG_HOME", base}, {"PORTAL_HOOKS_FILE", ""}},
			},
			{
				name: "both, the per-file variable winning",
				env:  [][2]string{{"XDG_CONFIG_HOME", base}, {"PORTAL_HOOKS_FILE", explicit}},
			},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				env := applyEnvCase(t, tc.env)

				production, err := configFilePath("PORTAL_HOOKS_FILE", "hooks.json")
				if err != nil {
					t.Fatalf("production route: %v", err)
				}
				seeder := hookstest.ResolveHooksFilePathFromEnv(t, env)

				if production != seeder {
					t.Errorf("the two routes diverged over one env:\nproduction %q\nseeder     %q", production, seeder)
				}
			})
		}
	})

	t.Run("the migration stays on the production route", func(t *testing.T) {
		home := t.TempDir()
		env := applyEnvCase(t, [][2]string{{"HOME", home}, {"XDG_CONFIG_HOME", t.TempDir()}, {"PORTAL_HOOKS_FILE", ""}})
		legacy := stageLegacyHooksFile(t, home)

		if got := hookstest.ResolveHooksFilePathFromEnv(t, env); got == "" {
			t.Fatal("the seeder resolved nothing")
		}
		if _, err := os.Stat(legacy); err != nil {
			t.Errorf("the seeder route moved the legacy file (stat: %v) — the migration must stay on the production path", err)
		}

		if _, err := configFilePath("PORTAL_HOOKS_FILE", "hooks.json"); err != nil {
			t.Fatalf("production route: %v", err)
		}

		if _, err := os.Stat(legacy); !os.IsNotExist(err) {
			t.Errorf("stat the legacy file after the production read = %v, want it moved — the production route still owns the one-shot migration", err)
		}
	})
}

// applyEnvCase puts one env case on both routes at once: onto the process
// environment the production rule reads, and into the slice the seeder reads,
// so a case cannot describe two different environments. HOME is pinned to a
// temp dir unless the case names its own, keeping the home fallback and the
// migration's legacy path off the developer's real files.
func applyEnvCase(t *testing.T, pairs [][2]string) []string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	env := []string{"HOME=" + home}
	for _, pair := range pairs {
		t.Setenv(pair[0], pair[1])
		env = append(env, pair[0]+"="+pair[1])
	}
	return env
}

// stageLegacyHooksFile plants a hooks.json at the old macOS config path under a
// temp home, so a route that migrates is distinguishable from one that does not.
func stageLegacyHooksFile(t *testing.T, home string) string {
	t.Helper()
	dir := filepath.Join(home, "Library", "Application Support", "portal")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("stage the legacy config dir: %v", err)
	}
	path := filepath.Join(dir, "hooks.json")
	body := fmt.Sprintf(`{%q:{"on-resume":"moved"}}`, hookstest.SubjectSeedA)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("stage the legacy hooks.json: %v", err)
	}
	return path
}
