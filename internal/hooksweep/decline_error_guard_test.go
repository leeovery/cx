package hooksweep

import (
	"go/ast"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// A declinedError is how a stand-down reaches the caller that renders it, so one
// built from the zero value would report a decline with no reason — the silent
// empty-reason line the type exists to prevent. Go cannot forbid the zero
// composite literal inside its own package, so the rule is read off the source.
func TestDeclinedErrorLiteralAlwaysNamesItsStandDown(t *testing.T) {
	literals := 0
	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		ast.Inspect(source.File, func(n ast.Node) bool {
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
					source.Fset.Position(lit.Pos()))
			}
			return true
		})
	}

	if literals == 0 {
		t.Fatal("no declinedError literal found in this package; the guard is scanning nothing")
	}
}
