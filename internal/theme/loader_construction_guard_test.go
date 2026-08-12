package theme_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/sourceguardtest"
)

var (
	loaderConstructionFile = filepath.Join("internal", "theme", "load.go")
	loaderConstructionFunc = "NewLoader"
)

func TestLoader_HasNoProductionCompositeLiteral(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	paths, err := sourceguardtest.GoSourceFiles(root)
	if err != nil {
		t.Fatalf("enumerate .go files: %v", err)
	}

	scanned, exempted := 0, 0
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			t.Fatalf("relativise %s: %v", path, relErr)
		}

		fset := token.NewFileSet()
		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", rel, parseErr)
		}
		scanned++

		packageLocal := filepath.Dir(rel) == filepath.Dir(loaderConstructionFile)
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			exempt := isFunc && rel == loaderConstructionFile && fn.Name.Name == loaderConstructionFunc

			ast.Inspect(decl, func(n ast.Node) bool {
				lit, ok := n.(*ast.CompositeLit)
				if !ok || !isLoaderTypeExpr(lit.Type, packageLocal) {
					return true
				}
				if exempt {
					exempted++
					return true
				}
				t.Errorf("%s:%d assembles a theme.Loader literal — a hand-assembled Loader reserves no built-in slugs, so a drop-in taking a built-in's slug is judged valid instead of `reserved name`: it lists as a second selectable row for that slug and diagnoses as loadable, while resolution still applies the built-in; production callers take theme.NewLoader or theme.NewSilentLoader", rel, fset.Position(lit.Pos()).Line)
				return true
			})
		}
	}

	if scanned == 0 {
		t.Fatal("the scan parsed no production .go file — the assertion above passed having looked at nothing")
	}
	if exempted != 1 {
		t.Fatalf("found %d Loader literals in %s's %s, want exactly 1 — the exemption names a construction that no longer exists, so the scan is not covering what it claims", exempted, loaderConstructionFile, loaderConstructionFunc)
	}
}

func isLoaderTypeExpr(expr ast.Expr, packageLocal bool) bool {
	switch typ := expr.(type) {
	case *ast.Ident:
		return packageLocal && typ.Name == "Loader"
	case *ast.SelectorExpr:
		pkg, ok := typ.X.(*ast.Ident)
		return ok && pkg.Name == "theme" && typ.Sel.Name == "Loader"
	}
	return false
}
