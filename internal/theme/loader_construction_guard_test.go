package theme_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

// loaderConstructionFile is the production file allowed to assemble a Loader,
// and loaderConstructionFunc is the function inside it that may do so. Together
// they are the single exemption below: the constructor that populates the
// reserved set, named rather than matched by pattern, so an assembly appearing
// anywhere else in the same file is caught.
var (
	loaderConstructionFile = filepath.Join("internal", "theme", "load.go")
	loaderConstructionFunc = "NewLoader"
)

// TestLoader_HasNoProductionCompositeLiteral asserts no production file
// anywhere in the repository assembles a theme.Loader literal.
//
// Loader's fields are exported so the rejection ladder can be driven with a
// synthetic reserved set, which makes `theme.Loader{}` a legal expression in
// every package — one that judges files through the whole ladder while
// reserving nothing. Under it a user's `tokyo-night.theme` shadows the built-in
// a slot falls back to, which is the one property the constructors exist to make
// impossible. NewLoader refuses a nil event seam loudly for the same reason; the
// zero value bypasses that check and fails silently instead.
//
// The scan is repo-wide rather than package-local because the hazard is not
// here: a `theme.Loader{}` in cmd, internal/tui, internal/capture or the capture
// harness is invisible from inside this package, and those are where such a
// value would actually be written.
func TestLoader_HasNoProductionCompositeLiteral(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	paths, err := portalbintest.GoSourceFiles(root)
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
				t.Errorf("%s:%d assembles a theme.Loader literal — a hand-assembled Loader reserves no built-in slugs, so a drop-in taking the slug of the built-in a slot falls back to would shadow it; production callers take theme.NewLoader or theme.NewSilentLoader", rel, fset.Position(lit.Pos()).Line)
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

// isLoaderTypeExpr reports whether the composite literal's type is theme.Loader,
// in either spelling: the qualified `theme.Loader{…}` any package can write, and
// the bare `Loader{…}` that means this type only inside internal/theme.
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
