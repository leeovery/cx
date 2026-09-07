package tui_test

import (
	"go/ast"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

func TestNoRawColourLiteralAtCentralisedSites(t *testing.T) {
	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		name := filepath.Base(source.Path)
		t.Run(name, func(t *testing.T) {
			ast.Inspect(source.File, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if !isLipglossColorCall(call) {
					return true
				}
				if len(call.Args) != 1 {
					return true
				}
				lit, ok := call.Args[0].(*ast.BasicLit)
				if !ok {
					return true
				}
				if lit.Kind == token.STRING || lit.Kind == token.INT {
					raw := lit.Value
					if lit.Kind == token.STRING {
						if unq, uerr := strconv.Unquote(lit.Value); uerr == nil {
							raw = unq
						}
					}
					pos := source.Position(lit.Pos())
					t.Errorf("%s:%d constructs lipgloss.Color(%q) from a raw colour literal; reference a token on the active theme instead", name, pos.Line, raw)
				}
				return true
			})
		})
	}
}

func isLipglossColorCall(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok {
		return false
	}
	return pkg.Name == "lipgloss" && sel.Sel.Name == "Color" && !strings.Contains(pkg.Name, "_")
}
