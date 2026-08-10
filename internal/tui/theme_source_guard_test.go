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

// oldThemeSubpackage is the import path of the retired internal/tui/theme package.
// The token layer moved out to the internal/theme leaf, so nothing in the tree
// may import this path and the directory itself must be gone.
const oldThemeSubpackage = "github.com/leeovery/portal/internal/tui" + "/theme"

// TestOldThemeSubpackageIsGone walks the whole module and fails if any file still
// imports the retired internal/tui/theme package, or if the package directory
// survives. It is the cheap source guard that relocation needs: a leftover
// importer would keep the old paired-token vocabulary compiling alongside the new
// single-palette one, which is precisely the drift the move exists to end.
func TestOldThemeSubpackageIsGone(t *testing.T) {
	root := repoRoot(t)

	if info, err := os.Stat(filepath.Join(root, "internal", "tui", "theme")); err == nil && info.IsDir() {
		t.Errorf("internal/tui/theme still exists; §3.2 relocates the token layer to internal/theme")
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

// TestNoPackageLevelThemeVar fails if any production file in internal/tui declares
// a PACKAGE-SCOPE var holding theme data — a Theme, a Token, or anything derived
// from the theme package at init time.
//
// The completeness risk names `pagepreview.go`'s package-init Token copy as one of two
// offenders "fixed outright, not guarded around", and is explicit that "the guard is what
// stops them returning": a value captured at package init can never see a theme
// swap, so the element it paints silently keeps the previous theme's colours. The active-theme
// plumbing forbids the same shape for a different reason — package-level mutable theme
// state on the render path, in a suite that already forbids t.Parallel().
//
// It scans production files only, matching the colour-literal guard's scope: a
// test-file copy cannot reach the render path.
//
// # Reach (probed, not assumed)
//
// A `theme.`-qualified rule alone is NOT enough: since the retired package-level
// theme.MV is gone, an init-time capture can now only be written by calling an
// IN-PACKAGE theme source, which carries no `theme.` selector at the call site.
// Each shape below was probed against this guard by dropping it into a temporary
// production file in this package and running the test:
//
//	CAUGHT  var x theme.Token = defaultDarkTheme().AccentMode   (declared type)
//	CAUGHT  var x = theme.Token{Name: ..., Value: ...}          (initialiser selector)
//	CAUGHT  var x = defaultDarkTheme().AccentMode               (in-package source call)
//	CAUGHT  var x = defaultDarkTheme()                          (in-package source call)
//	CAUGHT  var x Model                                         (declared local theme-bearing type)
//	CAUGHT  var x = lipgloss.NewStyle().Foreground(defaultDarkTheme().AccentMode.Color())
//	QUIET   var x = 4                                           (negative control)
//	QUIET   var x = someNonThemeFunc()                          (negative control)
//
// The in-package arm is structural, not name-based: themeSources derives the
// source set from the declarations themselves, so a theme accessor named without
// the word "theme" is still caught.
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

// parseProductionFiles parses each named file in the package directory, keyed by
// name, sharing one FileSet so reported positions are comparable across files.
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

// themeSources is the in-package vocabulary that yields theme data without ever
// writing `theme.` at the call site: the local struct types carrying theme fields
// (Model, which holds both the active Theme and the injected Nomination) and the
// functions whose results carry theme data either directly (defaultDarkTheme,
// loadBuiltinTheme) or through one of those types (New, NewModelWithSessions).
type themeSources struct {
	types map[string]bool
	funcs map[string]bool
}

// collectThemeSources derives the source vocabulary from the package's own
// declarations, in two ordered passes: every theme-bearing struct type first
// (a function may return a type declared in another file), then every function
// whose results carry theme data directly or through one of those types — the
// single transitive step that reaches the model constructors.
//
// Methods are collected alongside plain functions; a method name is inert here,
// because the matching arm only fires on a bare-identifier call. Its one known
// blind spot is therefore a var initialised by a METHOD call
// (`someNomination.Select(...)`), which needs a package-scope receiver that these
// same arms already reject.
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

// typeSpecs returns every top-level type declaration in a file.
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

// themeReference reports whether a package-scope var spec holds theme data — by
// naming the theme package or a local theme-bearing type in its declared type, or
// by naming the theme package or calling an in-package theme source in any
// initialiser — and where.
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

// namesThemePackage reports whether an expression mentions the theme package
// anywhere within it (theme.Theme, theme.Token, []theme.Token, a call taking a
// theme constant, ...).
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

// namesLocalThemeType reports whether a type expression names one of the local
// theme-bearing struct types, bare or under a pointer/slice/map.
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

// callsThemeSource reports whether an initialiser calls an in-package theme
// source at ANY depth, so a var built through a chain — lipgloss.NewStyle().
// Foreground(defaultDarkTheme().AccentMode.Color()) — is caught alongside the
// bare `= defaultDarkTheme()`.
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

// repoRoot resolves the module root the repo-wide guards walk.
func repoRoot(t *testing.T) string {
	t.Helper()
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	return root
}

// forEachGoFile parses every .go file under root and hands it to fn.
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
