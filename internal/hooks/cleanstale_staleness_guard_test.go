package hooks_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The staleness rule has one implementation and both readers reach it directly.
// The clean must not be layered on the exported StaleKeys: it judges the key
// set it has already loaded, and the exported query is free to grow
// caller-facing behaviour that must not run inside the clean's own pass.
func TestCleanStaleDoesNotCallStaleKeys(t *testing.T) {
	paths, err := sourceguardtest.PackageGoFiles(".", false)
	if err != nil {
		t.Fatalf("enumerate package sources: %v", err)
	}

	cleanPath := map[string]bool{"CleanStale": true, "deleteStale": true}

	scanned := 0
	for _, path := range paths {
		file, parseErr := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		scanned++
		sourceguardtest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
			if cleanPath[funcName] && calleeName(call) == "StaleKeys" {
				t.Errorf("%s: the clean calls StaleKeys — both must reach the staleness rule through the unexported implementation", path)
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

// Every mutation takes the file under one exclusive hold, so each must reach
// the file through the unexported load/save — which do no locking of their own.
// Routing through any of the locking front doors instead nests a second
// acquisition inside the hold the mutation already owns: a deadlock-shaped
// regression that degrades into a silent multi-second stall rather than failing
// a test. The read front doors are named alongside Load/Save because they
// acquire the same sidecar, shared — which an exclusive holder still blocks.
func TestMutationsDoNotCallExportedLoadOrSave(t *testing.T) {
	paths, err := sourceguardtest.PackageGoFiles(".", false)
	if err != nil {
		t.Fatalf("enumerate package sources: %v", err)
	}

	mutations := map[string]bool{"Set": true, "Remove": true, "deleteStale": true}
	forbidden := map[string]bool{
		"Load":              true,
		"Save":              true,
		"loadSnapshot":      true,
		"List":              true,
		"Get":               true,
		"loadShared":        true,
		"loadSharedBounded": true,
	}

	scanned := 0
	for _, path := range paths {
		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", path, parseErr)
		}
		scanned++
		sourceguardtest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
			callee := calleeName(call)
			if mutations[funcName] && forbidden[callee] && calleeReceiverName(call) == "s" {
				t.Errorf("%s: %s calls s.%s — a mutation must reach the file through the unexported load/save, never re-enter a locking front door", fset.Position(call.Pos()), funcName, callee)
			}
			return true
		})
	}

	if scanned == 0 {
		t.Fatal("guard scanned no files")
	}
}

// calleeReceiverName reports the identifier a method call is made on
// ("s" for s.Load()), or "" when the call has no plain identifier receiver.
func calleeReceiverName(call *ast.CallExpr) string {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	ident, ok := sel.X.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}
