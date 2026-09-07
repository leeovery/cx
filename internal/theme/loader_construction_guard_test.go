package theme_test

import (
	"go/ast"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
)

var (
	loaderConstructionFile = filepath.Join("internal", "theme", "load.go")
	loaderConstructionFunc = "NewLoader"
)

func TestLoader_HasNoProductionCompositeLiteral(t *testing.T) {
	_, sources := sourceguardtest.RepoSources(t, sourceguardtest.NonTestSources)

	exempted := 0
	for _, source := range sources {
		rel := source.Path

		packageLocal := filepath.Dir(rel) == filepath.Dir(loaderConstructionFile)
		for _, decl := range source.File.Decls {
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
				t.Errorf("%s:%d assembles a theme.Loader literal — a hand-assembled Loader reserves no built-in slugs, so a drop-in taking a built-in's slug is judged valid instead of `reserved name`: it lists as a second selectable row for that slug and diagnoses as loadable, while resolution still applies the built-in; production callers take theme.NewLoader or theme.NewSilentLoader", rel, source.Fset.Position(lit.Pos()).Line)
				return true
			})
		}
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
