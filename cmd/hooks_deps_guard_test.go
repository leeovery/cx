package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The package-level hooks seam outlives the test that installs it, so an
// install without its paired restore leaks into every later test in the
// package. The staging helpers are the only place the two are written
// together; a bare assignment anywhere else can drop the restore.
const hooksDepsHelperFile = "testhelpers_test.go"

func TestHooksDepsInstalledOnlyThroughTheStagingHelpers(t *testing.T) {
	paths, err := sourceguardtest.PackageGoFiles(".", true)
	if err != nil {
		t.Fatalf("enumerate the cmd package sources: %v", err)
	}

	fset := token.NewFileSet()
	scanned := 0
	for _, path := range paths {
		base := filepath.Base(path)
		if !strings.HasSuffix(base, "_test.go") || base == hooksDepsHelperFile {
			continue
		}
		file, perr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if perr != nil {
			t.Fatalf("parse %s: %v", path, perr)
		}
		scanned++
		for _, pos := range hooksDepsAssignments(file) {
			t.Errorf("%s:%d assigns the package-level hooks seam directly; install it through the staging helpers in %s so the restore cannot be dropped",
				base, fset.Position(pos).Line, hooksDepsHelperFile)
		}
	}

	if scanned == 0 {
		t.Fatal("no cmd test sources scanned; the guard would pass by having stopped looking")
	}
}

// hooksDepsAssignments returns the position of every assignment whose target is
// the hooks seam identifier.
func hooksDepsAssignments(file *ast.File) []token.Pos {
	var found []token.Pos
	ast.Inspect(file, func(n ast.Node) bool {
		assign, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for _, target := range assign.Lhs {
			if ident, isIdent := target.(*ast.Ident); isIdent && ident.Name == "hooksDeps" {
				found = append(found, ident.Pos())
			}
		}
		return true
	})
	return found
}
