package portalbintest_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

// TestPackageGoFiles_ProductionSources asserts the production enumeration takes
// the directory's own .go files and nothing else: no subdirectory is descended
// into, non-.go files are skipped, and _test.go sources are left out.
func TestPackageGoFiles_ProductionSources(t *testing.T) {
	dir := packageFixture(t)

	got := packageRelFiles(t, dir, false)
	want := []string{"alpha.go", "beta.go"}
	if !slices.Equal(got, want) {
		t.Errorf("PackageGoFiles(includeTests=false) = %v, want %v", got, want)
	}
}

// TestPackageGoFiles_WithTestSources asserts the same enumeration widened by the
// flag picks up the package's _test.go sources alongside its production ones.
func TestPackageGoFiles_WithTestSources(t *testing.T) {
	dir := packageFixture(t)

	got := packageRelFiles(t, dir, true)
	want := []string{"alpha.go", "alpha_test.go", "beta.go"}
	if !slices.Equal(got, want) {
		t.Errorf("PackageGoFiles(includeTests=true) = %v, want %v", got, want)
	}
}

// TestPackageGoFiles_MissingDirErrors asserts an unreadable directory surfaces as
// an error rather than an empty, silently-passing enumeration.
func TestPackageGoFiles_MissingDirErrors(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "absent")
	if _, err := portalbintest.PackageGoFiles(missing, false); err == nil {
		t.Fatalf("PackageGoFiles(%q) = nil error, want an error", missing)
	}
}

// TestPackageGoFiles_EmptyMatchErrors is the anti-vacuity rule: a directory that
// yields no matching source is an error, so a guard driven by this enumeration
// cannot pass by having stopped looking.
func TestPackageGoFiles_EmptyMatchErrors(t *testing.T) {
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"only_test.go": "package a\n",
		"README.md":    "not go\n",
	})

	if _, err := portalbintest.PackageGoFiles(dir, false); err == nil {
		t.Fatalf("PackageGoFiles(%q, false) = nil error, want an error", dir)
	}
}

func packageFixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeFiles(t, dir, map[string]string{
		"alpha.go":       "package a\n",
		"beta.go":        "package a\n",
		"alpha_test.go":  "package a\n",
		"notes.md":       "not go\n",
		"nested/deep.go": "package nested\n",
	})
	return dir
}

func packageRelFiles(t *testing.T, dir string, includeTests bool) []string {
	t.Helper()
	paths, err := portalbintest.PackageGoFiles(dir, includeTests)
	if err != nil {
		t.Fatalf("PackageGoFiles(%q, %v): %v", dir, includeTests, err)
	}
	rels := make([]string, 0, len(paths))
	for _, path := range paths {
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			t.Fatalf("relativise %s: %v", path, relErr)
		}
		rels = append(rels, filepath.ToSlash(rel))
	}
	slices.Sort(rels)
	return rels
}
