package tui_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

// Split so this declaration is not itself an importer of the retired path.
const oldThemeSubpackage = "github.com/leeovery/portal/internal/tui" + "/theme"

func TestOldThemeSubpackageIsGone(t *testing.T) {
	root := repoRoot(t)

	if info, err := os.Stat(filepath.Join(root, "internal", "tui", "theme")); err == nil && info.IsDir() {
		t.Errorf("internal/tui/theme still exists; the token layer lives in internal/theme")
	}

	forEachGoFile(t, root, func(path string, file *ast.File) {
		for _, imp := range file.Imports {
			if strings.Trim(imp.Path.Value, `"`) == oldThemeSubpackage {
				rel, _ := filepath.Rel(root, path)
				t.Errorf("%s imports the retired %s", rel, oldThemeSubpackage)
			}
		}
	})
}

// A theme captured at package init can never see a swap. Production files only:
// a test-file copy cannot reach the render path.
func TestNoPackageLevelThemeVar(t *testing.T) {
	names := centralisedColourSites(t)
	fset := token.NewFileSet()
	files := parseProductionFiles(t, fset, names)
	sources := collectThemeSources(files)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			for _, decl := range files[name].Decls {
				gen, ok := decl.(*ast.GenDecl)
				if !ok || gen.Tok != token.VAR {
					continue
				}
				for _, spec := range gen.Specs {
					value, ok := spec.(*ast.ValueSpec)
					if !ok {
						continue
					}
					if pos, found := themeReference(fset, value, sources); found {
						t.Errorf("%s:%d declares a package-scope var holding theme data; a theme captured at package init can never see a swap — take it from the model's active Theme at the call site", name, pos.Line)
					}
				}
			}
		})
	}
}

func parseProductionFiles(t *testing.T, fset *token.FileSet, names []string) map[string]*ast.File {
	t.Helper()
	files := make(map[string]*ast.File, len(names))
	for _, name := range names {
		file, err := parser.ParseFile(fset, filepath.Join(".", name), nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files[name] = file
	}
	return files
}

// The in-package vocabulary that yields theme data without writing `theme.` at
// the call site — a `theme.`-qualified rule alone would miss it.
type themeSources struct {
	types map[string]bool
	funcs map[string]bool
}

// Types first, because a function may return a type declared in another file.
// Blind spot: a var initialised by a method call, which needs a package-scope
// receiver these same arms already reject.
func collectThemeSources(files map[string]*ast.File) themeSources {
	sources := themeSources{types: map[string]bool{}, funcs: map[string]bool{}}
	for _, file := range files {
		for _, spec := range typeSpecs(file) {
			structType, ok := spec.Type.(*ast.StructType)
			if !ok {
				continue
			}
			for _, field := range structType.Fields.List {
				if namesThemePackage(field.Type) {
					sources.types[spec.Name.Name] = true
					break
				}
			}
		}
	}
	for _, file := range files {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Type.Results == nil {
				continue
			}
			for _, result := range fn.Type.Results.List {
				if namesThemePackage(result.Type) || sources.namesLocalThemeType(result.Type) {
					sources.funcs[fn.Name.Name] = true
					break
				}
			}
		}
	}
	return sources
}

func typeSpecs(file *ast.File) []*ast.TypeSpec {
	specs := []*ast.TypeSpec{}
	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.TYPE {
			continue
		}
		for _, spec := range gen.Specs {
			if typeSpec, ok := spec.(*ast.TypeSpec); ok {
				specs = append(specs, typeSpec)
			}
		}
	}
	return specs
}

func themeReference(fset *token.FileSet, value *ast.ValueSpec, sources themeSources) (token.Position, bool) {
	if value.Type != nil && (namesThemePackage(value.Type) || sources.namesLocalThemeType(value.Type)) {
		return fset.Position(value.Pos()), true
	}
	for _, v := range value.Values {
		if namesThemePackage(v) || sources.callsThemeSource(v) {
			return fset.Position(value.Pos()), true
		}
	}
	return token.Position{}, false
}

func namesThemePackage(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "theme" {
			found = true
			return false
		}
		return true
	})
	return found
}

func (s themeSources) namesLocalThemeType(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		if ident, ok := n.(*ast.Ident); ok && s.types[ident.Name] {
			found = true
			return false
		}
		return true
	})
	return found
}

// Matches at any depth, so a var built through a style chain is caught
// alongside a bare call.
func (s themeSources) callsThemeSource(expr ast.Expr) bool {
	found := false
	ast.Inspect(expr, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if ident, ok := call.Fun.(*ast.Ident); ok && s.funcs[ident.Name] {
			found = true
			return false
		}
		return true
	})
	return found
}

func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	return root
}

func forEachGoFile(t *testing.T, root string, fn func(path string, file *ast.File)) {
	t.Helper()
	fset := token.NewFileSet()
	for _, path := range allGoFiles(t, root) {
		file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		fn(path, file)
	}
}
