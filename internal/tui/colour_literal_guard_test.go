package tui_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// centralisedColourSites enumerates every NON-test production .go file in the
// internal/tui package (the package directory only, not recursing). It is the
// "closed vocabulary, no literal hex at call sites" rule made executable: none of
// these render files may construct a colour from a raw hex / ANSI-index literal —
// every colour must flow from a token on the model's active theme.
//
// It is a GLOB (not a hand-maintained list) so no render file can be silently
// omitted from coverage as later phases add renderers — the guard's coverage grows
// with the package automatically, closing the Phase 3-5 blind spot the former 11-file
// allowlist left open.
func centralisedColourSites(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(".", "*.go"))
	if err != nil {
		t.Fatalf("glob internal/tui package files: %v", err)
	}
	files := make([]string, 0, len(matches))
	for _, m := range matches {
		name := filepath.Base(m)
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		files = append(files, name)
	}
	if len(files) == 0 {
		t.Fatal("centralisedColourSites glob matched no production .go files in internal/tui")
	}
	return files
}

// TestNoRawColourLiteralAtCentralisedSites parses every production render file in
// internal/tui (via the centralisedColourSites glob) and fails if any
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
