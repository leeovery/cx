package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/sourceguard"
)

func prePlaceModalOnClearedCanvas(panel string, width, height int) string {
	return lipgloss.Place(width, height, lipgloss.Center, lipgloss.Center, panel)
}

func TestPlaceModalOnClearedCanvas_ByteIdenticalToInline(t *testing.T) {
	panels := []string{
		"",
		"x",
		"single line panel",
		"line one\nline two\nline three",
		"╭───────╮\n│ panel │\n╰───────╯",
	}
	dims := []struct{ w, h int }{
		{80, 24},
		{120, 40},
		{40, 12},
		{1, 1},
		{0, 0},
	}
	for _, panel := range panels {
		for _, d := range dims {
			want := prePlaceModalOnClearedCanvas(panel, d.w, d.h)
			if got := placeModalOnClearedCanvas(panel, d.w, d.h); got != want {
				t.Errorf("placeModalOnClearedCanvas(%q, %d, %d) drift\n got: %q\nwant: %q",
					panel, d.w, d.h, got, want)
			}
		}
	}
}

func TestModalCentringAppearsInExactlyOnePlace(t *testing.T) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "modal.go", nil, 0)
	if err != nil {
		t.Fatalf("parse modal.go: %v", err)
	}

	var hosts []string
	sourceguard.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
		if isClearedCanvasPlaceCall(call) {
			hosts = append(hosts, funcName)
		}
		return true
	})

	if len(hosts) != 1 {
		t.Fatalf("cleared-canvas centring lipgloss.Place(width, height, Center, Center, panel) must appear in exactly one function; found in %v", hosts)
	}
	if hosts[0] != "placeModalOnClearedCanvas" {
		t.Errorf("cleared-canvas centring lives in %q, want placeModalOnClearedCanvas", hosts[0])
	}
}

func isClearedCanvasPlaceCall(call *ast.CallExpr) bool {
	if !isSelector(call.Fun, "lipgloss", "Place") {
		return false
	}
	if len(call.Args) != 5 {
		return false
	}
	return isIdent(call.Args[0], "width") &&
		isIdent(call.Args[1], "height") &&
		isSelector(call.Args[2], "lipgloss", "Center") &&
		isSelector(call.Args[3], "lipgloss", "Center")
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	return isIdent(sel.X, pkg)
}

func isIdent(expr ast.Expr, name string) bool {
	id, ok := expr.(*ast.Ident)
	return ok && id.Name == name
}
