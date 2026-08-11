package tui_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguard"
)

// centralisedColourSites enumerates every NON-test production .go file in the
// internal/tui package (the package directory only, not recursing). It is the
// "closed vocabulary, no literal hex at call sites" rule made executable: none of
// these render files may construct a colour from a raw hex / ANSI-index literal —
// every colour must flow from a token on the model's active theme.
//
// It is the whole package directory (not a hand-maintained list) so no render
// file can be silently omitted from coverage as renderers are added — the guard's
// coverage grows with the package automatically.
func centralisedColourSites(t *testing.T) []string {
	t.Helper()
	paths, err := sourceguard.PackageGoFiles(".", false)
	if err != nil {
		t.Fatalf("enumerate the internal/tui package sources: %v", err)
	}
	files := make([]string, 0, len(paths))
	for _, path := range paths {
		files = append(files, filepath.Base(path))
	}
	return files
}

// TestNoRawColourLiteralAtCentralisedSites parses every production render file in
// internal/tui (via centralisedColourSites) and fails if any
// lipgloss.Color(...) call is passed a raw string/int literal (a hex like "#777777"
// or an ANSI index like "212"/76). Raw colour values live in the embedded .theme
// FILES and nowhere in Go at all — every call site must reference a token.
func TestNoRawColourLiteralAtCentralisedSites(t *testing.T) {
	for _, name := range centralisedColourSites(t) {
		t.Run(name, func(t *testing.T) {
			fset := token.NewFileSet()
			path := filepath.Join(".", name)
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", name, err)
			}

			ast.Inspect(file, func(n ast.Node) bool {
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
					// Non-literal argument (a token reference, a variable) is fine.
					return true
				}
				if lit.Kind == token.STRING || lit.Kind == token.INT {
					raw := lit.Value
					if lit.Kind == token.STRING {
						if unq, uerr := strconv.Unquote(lit.Value); uerr == nil {
							raw = unq
						}
					}
					pos := fset.Position(lit.Pos())
					t.Errorf("%s:%d constructs lipgloss.Color(%q) from a raw colour literal; reference a token on the active theme instead", name, pos.Line, raw)
				}
				return true
			})
		})
	}
}

// isLipglossColorCall reports whether call is `lipgloss.Color(...)`.
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
