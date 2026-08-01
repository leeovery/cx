package theme_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// themePkg is the import path of the package under test.
const themePkg = "github.com/leeovery/portal/internal/theme"

// xdgPkg is the config-path-resolution package internal/theme must never reach.
const xdgPkg = "github.com/leeovery/portal/internal/xdg"

// themesDirEnvVar is the themes-directory env var. cmd/config.go's
// themesDirPath owns it; internal/theme must not know the name exists.
const themesDirEnvVar = "PORTAL_THEMES_DIR"

// TestThemePackage_ResolvesNoPaths asserts internal/theme contains no
// path-resolution code: it neither depends on internal/xdg nor reads the
// themes-directory env var or the home directory itself.
//
// The loader takes its directory as an INJECTED value. That is what keeps the
// embedded built-in set reachable with no path at all — internal/capture uses
// only the built-in lookup, and its no-real-config import guard forbids
// reaching config — and what keeps portal doctor and portal theme export free
// of config discovery they must not perform. A loader that grew its own lookup
// would break all three silently, so the invariant is guarded rather than
// documented.
//
// Modelled on internal/prefs' leaf guard (a go list -deps walk), with a source
// scan added for the two lookups that need no import to write.
func TestThemePackage_ResolvesNoPaths(t *testing.T) {
	t.Run("does not depend on internal/xdg", func(t *testing.T) {
		// Anchored at the import path (not a relative dir) so it resolves
		// regardless of the test binary's runtime CWD.
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
		for _, path := range themeSourceFiles(t) {
			fset := token.NewFileSet()
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				t.Fatalf("parse %s: %v", path, err)
			}

			name := filepath.Base(path)
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

// themeSourceFiles globs every non-test production .go file in the
// internal/theme package directory. It is a glob rather than a hand-maintained
// list so a file added by a later phase is covered automatically, and it fails
// on an empty match so the scan can never pass vacuously.
func themeSourceFiles(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(".", "*.go"))
	if err != nil {
		t.Fatalf("glob internal/theme package files: %v", err)
	}
	files := make([]string, 0, len(matches))
	for _, m := range matches {
		if strings.HasSuffix(filepath.Base(m), "_test.go") {
			continue
		}
		files = append(files, m)
	}
	if len(files) == 0 {
		t.Fatal("themeSourceFiles glob matched no production .go files in internal/theme")
	}
	return files
}
