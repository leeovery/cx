package sourceguardtest_test

import (
	"go/ast"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

func TestCalleeName_UnwrapsAnIdentifierCall(t *testing.T) {
	if got := sourceguardtest.CalleeName(onlyCall(t, "package a\nfunc f() { Load() }\n")); got != "Load" {
		t.Errorf("CalleeName(Load()) = %q, want %q", got, "Load")
	}
}

func TestCalleeName_UnwrapsASelectorCall(t *testing.T) {
	if got := sourceguardtest.CalleeName(onlyCall(t, "package a\nfunc f() { s.Load() }\n")); got != "Load" {
		t.Errorf("CalleeName(s.Load()) = %q, want %q", got, "Load")
	}
}

func TestCalleeName_EmptyForNeitherShape(t *testing.T) {
	if got := sourceguardtest.CalleeName(onlyCall(t, "package a\nfunc f() { func() {}() }\n")); got != "" {
		t.Errorf("CalleeName(func(){}()) = %q, want the empty string — the call names no callee", got)
	}
}

// onlyCall returns the outermost call expression of the single function in src,
// so a case reads as the shape it is about rather than as an AST walk.
func onlyCall(t *testing.T, src string) *ast.CallExpr {
	t.Helper()
	var found *ast.CallExpr
	sourceguardtest.ForEachFuncCall(parseSource(t, src), func(_ string, call *ast.CallExpr) bool {
		found = call
		return false
	})
	if found == nil {
		t.Fatalf("fixture source holds no call expression:\n%s", src)
	}
	return found
}
