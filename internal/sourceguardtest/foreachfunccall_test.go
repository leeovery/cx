package sourceguardtest_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

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
	sourceguardtest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
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

func TestForEachFuncCall_BodylessDeclarationYieldsNothing(t *testing.T) {
	file := parseSource(t, `package a

func external()

func local() { after() }
`)

	var got []string
	sourceguardtest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
		got = append(got, funcName+":"+callName(t, call))
		return true
	})

	want := []string{"local:after"}
	if !slices.Equal(got, want) {
		t.Errorf("ForEachFuncCall visited %v, want %v", got, want)
	}
}

func TestForEachFuncCall_FalseStopsTheWalk(t *testing.T) {
	file := parseSource(t, `package a

func first() {
	stopHere()
	laterInSameFunc()
}

func second() { inLaterFunc() }
`)

	var got []string
	sourceguardtest.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
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

// callName labels a visited call for the order assertions: the callee's own
// name where it has one, and an explicit marker for the immediately-invoked
// literal the fixtures use, which names none.
func callName(t *testing.T, call *ast.CallExpr) string {
	t.Helper()
	if name := sourceguardtest.CalleeName(call); name != "" {
		return name
	}
	if _, isLiteral := call.Fun.(*ast.FuncLit); isLiteral {
		return "<func literal>"
	}
	t.Fatalf("fixture call %T is neither a bare identifier nor an immediately-invoked literal", call.Fun)
	return ""
}
