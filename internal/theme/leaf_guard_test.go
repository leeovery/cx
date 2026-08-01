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

// TestThemePackage_DeclaresNoHexLiterals asserts no production file in
// internal/theme declares a `#RRGGBB` value.
//
// Every colour Portal renders lives in a `.theme` file, built-in or drop-in
// (§7.1) — there is no Go-side palette, no compiled-in last-resort values
// beneath the built-in fallback (§7.6's build-time guarantee replaces exactly
// that crutch), and no hex constant a test could quietly diverge from the
// shipped files. This is also what retires the `internal/tui` colour-literal
// guard's exemption for the token layer: with the values out of Go entirely,
// the rule "no raw hex at a call site" holds with no carve-out.
//
// Only string literals are scanned. A `#RRGGBB` in prose — a doc comment naming
// the value domain, or a rejection example — is documentation, not a value.
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

// TestThemePackage_HasNoInitFunction asserts the package does no work when it
// is loaded.
//
// §7.6 is explicit that validation is NOT startup-eager: nothing walks the
// embedded set at init, because the build-time test already proves the set and
// re-proving it on every launch buys nothing on a cold path this feature
// otherwise adds no cost to. Embedding the built-ins is what makes that worth
// guarding — a set of files sitting in a package variable is a standing
// invitation to "just validate them once at init", which would put a parse of
// every built-in on the startup path of `portal open`, `portal doctor` and
// every other verb alike.
//
// Both halves are structural, because both routes into init exist: a declared
// func init(), and a package-level var whose initialiser calls something. The
// second is the subtler one — `var builtins = mustParseAll()` runs before main
// with no init() in sight.
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

// requireNoCallingInitialiser fails the test for any package-level var in the
// declaration whose initialiser calls a function.
//
// A package-level var may hold a value; it may not COMPUTE one. That is the
// checkable form of "nothing walks the embedded set at init" — the embedded
// filesystem itself is a plain declaration with no initialiser at all, and a
// var that called anything would be running that call before main.
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

// parsedThemeSource is one production file of the package, parsed, with the
// file set its positions resolve against.
type parsedThemeSource struct {
	Name string
	Fset *token.FileSet
	File *ast.File
}

// parseThemeSources parses every production file of the package, so the source
// guards above scan a syntax tree rather than text.
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
