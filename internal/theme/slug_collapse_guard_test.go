package theme_test

import (
	"go/ast"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

// A source guard, not a compile-time one: a second collapse would build and
// answer correctly on the day it was written, then rot silently the next time
// the substitution rule moves.
func TestSlugForSlot_IsTheOnlyCollapseOutsideThisPackagesTests(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	paths, err := sourceguardtest.GoSourceFiles(root)
	if err != nil {
		t.Fatalf("enumerate .go files: %v", err)
	}

	var scanPaths []string
	for _, path := range paths {
		if !exemptFromCollapseGuard(relToProjectRoot(t, root, path)) {
			scanPaths = append(scanPaths, path)
		}
	}

	var collapsing []string
	for _, source := range sourceguardtest.ParseSources(t, scanPaths) {
		rel := relToProjectRoot(t, root, source.Path)
		for _, name := range collapsingFuncsIn(source.File) {
			collapsing = append(collapsing, rel+":"+name)
		}
	}

	want := []string{filepath.Join("internal", "theme", "setting.go") + ":SlugForSlot"}
	slices.Sort(collapsing)
	if !slices.Equal(collapsing, want) {
		t.Errorf("ResolveSetting followed by Setting.Slug appears in %v, want only %v — one collapse from the raw keys to a slot's nominated slug", collapsing, want)
	}
}

// relToProjectRoot names a scanned file the way the repository does, so a
// guard's complaint reads as a repository path rather than as one machine's
// checkout.
func relToProjectRoot(t *testing.T, root, path string) string {
	t.Helper()
	rel, err := filepath.Rel(root, path)
	if err != nil {
		t.Fatalf("relativise %s: %v", path, err)
	}
	return rel
}

// This package's own tests exercise the two halves separately, which is what the
// guard's own subject is built out of.
func exemptFromCollapseGuard(rel string) bool {
	return strings.HasPrefix(rel, filepath.Join("internal", "theme")+string(filepath.Separator)) &&
		strings.HasSuffix(rel, "_test.go")
}

func collapsingFuncsIn(file *ast.File) []string {
	resolvers := map[string]bool{}
	sluggers := map[string]bool{}
	sourceguardtest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
		switch sourceguardtest.CalleeName(call) {
		case "ResolveSetting":
			resolvers[funcName] = true
		case "Slug":
			sluggers[funcName] = true
		}
		return true
	})

	var both []string
	for name := range resolvers {
		if sluggers[name] {
			both = append(both, name)
		}
	}
	return both
}
