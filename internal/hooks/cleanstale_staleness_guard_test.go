package hooks_test

import (
	"go/ast"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

// The staleness rule has one implementation under one exported name, and every
// reader of staleness reaches it rather than restating it. An unexported twin
// beside it would be a second name for the same rule, and a caller reaching the
// twin would be a reader the exported name's callers cannot account for.
func TestStalenessRuleHasOneExportedFunction(t *testing.T) {
	t.Run("it applies the staleness rule through the single exported function from both callers", func(t *testing.T) {
		for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
			for _, decl := range source.File.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if ok && fn.Name.Name == "staleKeys" {
					t.Errorf("%s: an unexported staleKeys survives beside the exported StaleKeys", source.Fset.Position(fn.Pos()))
				}
			}
		}

		assertCallsStaleKeys(t, ".", "deleteStale")
		assertCallsStaleKeys(t, "../../cmd", "checkStaleHooks")
	})
}

// assertCallsStaleKeys fails unless the named function in the package at dir
// reaches the staleness rule by calling StaleKeys.
func assertCallsStaleKeys(t *testing.T, dir, funcName string) {
	t.Helper()

	var called bool
	for _, source := range sourceguardtest.ParsePackageSources(t, dir, false) {
		sourceguardtest.ForEachFuncCall(source.File, func(enclosing string, call *ast.CallExpr) bool {
			if enclosing == funcName && sourceguardtest.CalleeName(call) == "StaleKeys" {
				called = true
			}
			return true
		})
	}
	if !called {
		t.Errorf("%s does not call StaleKeys — every reader of staleness must reach the one exported function", funcName)
	}
}

// Every mutation takes the file under one exclusive hold, so each must reach
// the file through the unexported load/save — which do no locking of their own.
// Routing through any of the locking front doors instead nests a second
// acquisition inside the hold the mutation already owns: a deadlock-shaped
// regression that degrades into a silent multi-second stall rather than failing
// a test. The read front doors are named alongside Load/Save because they
// acquire the same sidecar, shared — which an exclusive holder still blocks.
func TestMutationsDoNotCallExportedLoadOrSave(t *testing.T) {
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

	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		sourceguardtest.ForEachFuncCall(source.File, func(funcName string, call *ast.CallExpr) bool {
			callee := sourceguardtest.CalleeName(call)
			if mutations[funcName] && forbidden[callee] && calleeReceiverName(call) == "s" {
				t.Errorf("%s: %s calls s.%s — a mutation must reach the file through the unexported load/save, never re-enter a locking front door", source.Fset.Position(call.Pos()), funcName, callee)
			}
			return true
		})
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
