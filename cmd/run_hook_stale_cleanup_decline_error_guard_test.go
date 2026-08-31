package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// A declinedError is how a stand-down reaches the caller that renders it, so one
// built from the zero value would report a decline with no reason — the silent
// empty-reason line the type exists to prevent. Go cannot forbid the zero
// composite literal inside its own package, so the rule is read off the source.
func TestDeclinedErrorLiteralAlwaysNamesItsStandDown(t *testing.T) {
	paths, err := sourceguardtest.PackageGoFiles(".", false)
	if err != nil {
		t.Fatalf("enumerate cmd package sources: %v", err)
	}

	fset := token.NewFileSet()
	literals := 0
	for _, path := range paths {
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if ident, ok := lit.Type.(*ast.Ident); !ok || ident.Name != "declinedError" {
				return true
			}
			literals++
			if len(lit.Elts) == 0 {
				t.Errorf("%s: declinedError literal names no stand-down; a decline must carry the reason it reports",
					fset.Position(lit.Pos()))
			}
			return true
		})
	}

	if literals == 0 {
		t.Fatal("no declinedError literal found in the cmd package; the guard is scanning nothing")
	}
}
