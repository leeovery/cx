package sourceguardtest_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

func TestGoSourceFiles_EnumeratesUnderRoot(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"main.go":                    "package main\n",
		"internal/pkg/thing.go":      "package pkg\n",
		"internal/pkg/thing_test.go": "package pkg\n",
		"README.md":                  "not go\n",
		"internal/pkg/data.json":     "{}\n",
	})

	got := relFiles(t, root)
	want := []string{
		"internal/pkg/thing.go",
		"internal/pkg/thing_test.go",
		"main.go",
	}
	if !slices.Equal(got, want) {
		t.Errorf("GoSourceFiles = %v, want %v", got, want)
	}
}

func TestGoSourceFiles_SkipsExcludedDirectories(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"kept.go":                       "package a\n",
		".git/hooks/hook.go":            "package a\n",
		".claude/skills/example.go":     "package a\n",
		".workflows/notes/scratch.go":   "package a\n",
		"vendor/dep/dep.go":             "package a\n",
		"node_modules/mod/mod.go":       "package a\n",
		"internal/vendorish/keeper.go":  "package a\n",
		"internal/node_modulesish/x.go": "package a\n",
	})

	got := relFiles(t, root)
	want := []string{
		"internal/node_modulesish/x.go",
		"internal/vendorish/keeper.go",
		"kept.go",
	}
	if !slices.Equal(got, want) {
		t.Errorf("GoSourceFiles = %v, want %v", got, want)
	}
}

// WalkDir calls back for the root itself, so a root whose own base name is
// dot-prefixed — a worktree checked out under .worktrees/ — must not be
// excluded on the same rule as a dot-directory found inside it.
func TestGoSourceFiles_DotPrefixedRootIsWalked(t *testing.T) {
	root := filepath.Join(t.TempDir(), ".worktrees-checkout")
	writeFiles(t, root, map[string]string{
		"kept.go":            "package a\n",
		".git/hooks/hook.go": "package a\n",
	})

	got := relFiles(t, root)
	if want := []string{"kept.go"}; !slices.Equal(got, want) {
		t.Errorf("GoSourceFiles = %v, want %v — the walk root was excluded on its own base name, so every guard under it scanned nothing", got, want)
	}
}

func TestGoSourceFiles_MissingRootErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	if _, err := sourceguardtest.GoSourceFiles(missing); err == nil {
		t.Fatalf("GoSourceFiles(%q) = nil error, want an error", missing)
	}
}

func relFiles(t *testing.T, root string) []string {
	t.Helper()
	paths, err := sourceguardtest.GoSourceFiles(root)
	if err != nil {
		t.Fatalf("GoSourceFiles(%q): %v", root, err)
	}
	rels := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("relativise %s: %v", path, err)
		}
		rels = append(rels, filepath.ToSlash(rel))
	}
	slices.Sort(rels)
	return rels
}

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir for %s: %v", rel, err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}
}
