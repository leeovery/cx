package portalbintest_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

// TestForEachFuncCall_VisitsEveryCallUnderItsFunction asserts the walk reaches
// calls at any depth — nested arguments, closures and inner function literals —
// and reports each under the top-level declaration holding it, methods included.
func TestForEachFuncCall_VisitsEveryCallUnderItsFunction(t *testing.T) {
	file := parseSource(t, `package a

func outer() {
	first(second())
	go func() { inClosure() }()
}

func (r receiver) method() {
	onMethod()
}

var atPackageLevel = notAFunctionDecl()
`)

	var got []string
	portalbintest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
		got = append(got, funcName+":"+callName(t, call))
		return true
	})

	want := []string{
		"outer:first",
		"outer:second",
		"outer:<func literal>",
		"outer:inClosure",
		"method:onMethod",
	}
	if !slices.Equal(got, want) {
		t.Errorf("ForEachFuncCall visited %v, want %v", got, want)
	}
}

// TestForEachFuncCall_BodylessDeclarationYieldsNothing pins the nil-body case:
// an externally-implemented declaration carries no call and must not panic the
// walk that steps over it.
func TestForEachFuncCall_BodylessDeclarationYieldsNothing(t *testing.T) {
	file := parseSource(t, `package a

func external()

func local() { after() }
`)

	var got []string
	portalbintest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
		got = append(got, funcName+":"+callName(t, call))
		return true
	})

	want := []string{"local:after"}
	if !slices.Equal(got, want) {
		t.Errorf("ForEachFuncCall visited %v, want %v", got, want)
	}
}

// TestForEachFuncCall_FalseStopsTheWalk asserts a false return ends the whole
// walk rather than the current declaration, so a guard that has found what it
// came for stops looking everywhere.
func TestForEachFuncCall_FalseStopsTheWalk(t *testing.T) {
	file := parseSource(t, `package a

func first() {
	stopHere()
	laterInSameFunc()
}

func second() { inLaterFunc() }
`)

	var got []string
	portalbintest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
		got = append(got, funcName+":"+callName(t, call))
		return false
	})

	want := []string{"first:stopHere"}
	if !slices.Equal(got, want) {
		t.Errorf("ForEachFuncCall visited %v, want %v", got, want)
	}
}

func parseSource(t *testing.T, src string) *ast.File {
	t.Helper()
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", src, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse fixture source: %v", err)
	}
	return file
}

func callName(t *testing.T, call *ast.CallExpr) string {
	t.Helper()
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.FuncLit:
		return "<func literal>"
	}
	t.Fatalf("fixture call %T is neither a bare identifier nor an immediately-invoked literal", call.Fun)
	return ""
}
