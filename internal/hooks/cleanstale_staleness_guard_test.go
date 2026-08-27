package hooks_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The staleness rule has one implementation and both readers reach it directly.
// CleanStale must not be layered on the exported StaleKeys: it judges the key
// set it has already loaded, and the exported query is free to grow
// caller-facing behaviour that must not run inside CleanStale's own pass.
func TestCleanStaleDoesNotCallStaleKeys(t *testing.T) {
	paths, err := sourceguardtest.PackageGoFiles(".", false)
	if err != nil {
		t.Fatalf("enumerate package sources: %v", err)
	}

	scanned := 0
	for _, path := range paths {
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		scanned++
		sourceguardtest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
			if funcName == "CleanStale" && calleeName(call) == "StaleKeys" {
				t.Errorf("%s: CleanStale calls StaleKeys — both must reach the staleness rule through the unexported implementation", path)
			}
			return true
		})
	}

	if scanned == 0 {
		t.Fatal("guard scanned no files")
	}
}

func calleeName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}
