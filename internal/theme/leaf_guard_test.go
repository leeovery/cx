package theme_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguard"
)

const themePkg = "github.com/leeovery/portal/internal/theme"

const xdgPkg = "github.com/leeovery/portal/internal/xdg"

const themesDirEnvVar = "PORTAL_THEMES_DIR"

func TestThemePackage_ResolvesNoPaths(t *testing.T) {
	t.Run("does not depend on internal/xdg", func(t *testing.T) {
		out, err := exec.Command("go", "list", "-deps", themePkg).CombinedOutput()
		if err != nil {
			t.Fatalf("go list -deps %s: %v\n%s", themePkg, err, out)
		}

		for dep := range strings.FieldsSeq(string(out)) {
			if dep == xdgPkg {
				t.Fatalf("internal/theme transitively imports %s — the themes directory is resolved by cmd/config.go's themesDirPath and injected, never looked up here", xdgPkg)
			}
		}
	})

	t.Run("reads neither the themes env var nor the home directory", func(t *testing.T) {
		for _, source := range parseThemeSources(t) {
			fset, file, name := source.Fset, source.File, source.Name

			ast.Inspect(file, func(n ast.Node) bool {
				switch node := n.(type) {
				case *ast.BasicLit:
					if node.Kind != token.STRING {
						return true
					}
					value, uerr := strconv.Unquote(node.Value)
					if uerr != nil {
						return true
					}
					if strings.Contains(value, themesDirEnvVar) {
						t.Errorf("%s:%d carries the %s literal — the env var belongs to cmd/config.go's themesDirPath, and the directory arrives here as an injected value", name, fset.Position(node.Pos()).Line, themesDirEnvVar)
					}
				case *ast.SelectorExpr:
					pkg, ok := node.X.(*ast.Ident)
					if !ok {
						return true
					}
					if pkg.Name == "os" && node.Sel.Name == "UserHomeDir" {
						t.Errorf("%s:%d calls os.UserHomeDir — internal/theme resolves no paths; the themes directory arrives as an injected value", name, fset.Position(node.Pos()).Line)
					}
				}
				return true
			})
		}
	})
}

func TestThemePackage_DeclaresNoHexLiterals(t *testing.T) {
	hexLiteral := regexp.MustCompile(`#[0-9a-fA-F]{6}`)

	for _, source := range parseThemeSources(t) {
		ast.Inspect(source.File, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}

			value, err := strconv.Unquote(lit.Value)
			if err != nil {
				return true
			}
			if found := hexLiteral.FindString(value); found != "" {
				t.Errorf("%s:%d declares the hex literal %s — every colour value lives in a .theme file, never in Go", source.Name, source.Fset.Position(lit.Pos()).Line, found)
			}
			return true
		})
	}
}

func TestThemePackage_HasNoInitFunction(t *testing.T) {
	for _, source := range parseThemeSources(t) {
		for _, decl := range source.File.Decls {
			switch d := decl.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.Name == "init" {
					t.Errorf("%s:%d declares func init() — internal/theme does no work at load time; nothing walks the embedded set at init", source.Name, source.Fset.Position(d.Pos()).Line)
				}
			case *ast.GenDecl:
				if d.Tok == token.VAR {
					requireNoCallingInitialiser(t, source, d)
				}
			}
		}
	}
}

func requireNoCallingInitialiser(t *testing.T, source parsedThemeSource, decl *ast.GenDecl) {
	t.Helper()

	for _, spec := range decl.Specs {
		value, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		for _, expr := range value.Values {
			ast.Inspect(expr, func(n ast.Node) bool {
				call, isCall := n.(*ast.CallExpr)
				if !isCall {
					return true
				}
				t.Errorf("%s:%d initialises a package-level var by calling a function — internal/theme does no work at load time; nothing walks the embedded set at init", source.Name, source.Fset.Position(call.Pos()).Line)
				return false
			})
		}
	}
}

type parsedThemeSource struct {
	Name string
	Fset *token.FileSet
	File *ast.File
}

func parseThemeSources(t *testing.T) []parsedThemeSource {
	t.Helper()

	paths := themeSourceFiles(t)
	sources := make([]parsedThemeSource, 0, len(paths))
	for _, path := range paths {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		sources = append(sources, parsedThemeSource{Name: filepath.Base(path), Fset: fset, File: file})
	}
	return sources
}

func themeSourceFiles(t *testing.T) []string {
	t.Helper()
	files, err := sourceguard.PackageGoFiles(".", false)
	if err != nil {
		t.Fatalf("enumerate the internal/theme package sources: %v", err)
	}
	return files
}
