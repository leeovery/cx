package tui_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
)

const (
	// restoreFileName is the file declaring the exit-time restore helper, parsed
	// directly: the external test package shares the package DIRECTORY, so a
	// relative path resolves wherever the suite was invoked from.
	restoreFileName = "restore.go"
	// restoreHelperName is the exit-time restore helper §11.4 re-anchors.
	restoreHelperName = "RestoreTerminalBackground"
)

// retiredCanvasHexHelper is the helper §11.4 deletes outright — the one that
// re-derived the echo guard's comparison value from a theme at exit.
//
// It is spelled in two halves so this guard file does not itself contain the
// string it forbids anywhere in the tree (the same trick oldThemeSubpackage uses
// for the retired import path).
var retiredCanvasHexHelper = "canvasHex" + "For"

// restoreComparisonReads is the closed set of selector paths the restore helper
// may read off its Model, relative to the parameter it binds. It is small and
// closed on purpose: the ONLY comparison input is the retained startup canvas
// hex, and everything else is either the captured original or the NO_COLOR
// carve-out.
//
// Anything else — the active theme, the nomination, the canvas mode — would
// re-derive the comparison from a theme at exit, which is the exact regression
// §11.4 exists to prevent: a theme committed mid-session, or a quit with an
// uncommitted preview active, makes the active theme diverge from the canvas the
// startup window actually painted.
//
// The paths are WHOLE PATHS rather than leaf field names, which is what keeps the
// guard honest now that the theme state is one grouped struct: permitting the
// prefix `themeState` alone would admit every theme field it holds, which is the
// entire set this guard exists to keep out.
var restoreComparisonReads = []string{
	"OriginalBackground",          // the captured original background
	"colourless",                  // the §2.5 NO_COLOR carve-out
	"themeState.startupCanvasHex", // the retained startup canvas hex — the §11.4 anchor
}

// restoreAnchorRead is the read the comparison MUST make, without which the
// closed set above would pass vacuously.
const restoreAnchorRead = "themeState.startupCanvasHex"

// restoreLaunchSites are the two program-launch sites that call the shared helper
// after p.Run() returns. Both must pass the program's output writer, so they
// restore identically.
var restoreLaunchSites = []string{
	filepath.Join("cmd", "open.go"),
	filepath.Join("cmd", "capturetool", "main.go"),
}

// TestRestorePath_ReadsNoTheme is the source half of §11.4's named verification.
//
// The swap-and-diff guard (§13.4) structurally cannot cover this path — it scans
// rendered fixture output, and this is an OSC 11 write that happens after the last
// render — so the exit path gets its own guards. The behavioural anchor is
// TestRestoreTerminalBackground_AnchoredToStartupHex; this proves the same thing
// structurally, which is what stops a future edit reaching for a theme again.
func TestRestorePath_ReadsNoTheme(t *testing.T) {
	fset := token.NewFileSet()
	path := filepath.Join(".", restoreFileName)
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse %s: %v", restoreFileName, err)
	}

	t.Run("the exit path imports no theme package", func(t *testing.T) {
		for _, imp := range file.Imports {
			if strings.Contains(strings.Trim(imp.Path.Value, `"`), "internal/theme") {
				t.Errorf("%s imports %s; the exit-time restore compares against the RETAINED startup hex, so it needs no theme at all",
					restoreFileName, imp.Path.Value)
			}
		}
	})

	t.Run("the comparison reads only the retained startup hex", func(t *testing.T) {
		fn := restoreHelperDecl(t, file)
		model := modelParamName(t, fn)
		anchored := false

		ast.Inspect(fn.Body, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			path, dotted := selectorPath(sel)
			if !dotted {
				return true
			}
			root, rest, _ := strings.Cut(path, ".")
			switch root {
			case "theme":
				t.Errorf("%s reads %s; the comparison must never be re-derived from a theme at exit (§11.4)",
					restoreHelperName, path)
			case model:
				if !slices.Contains(restoreComparisonReads, rest) {
					t.Errorf("%s reads %s; the only permitted reads are %s.{%s}",
						restoreHelperName, path, model, strings.Join(restoreComparisonReads, ", "))
				}
				if rest == restoreAnchorRead {
					anchored = true
				}
			}
			// The WHOLE path has been judged, so the walk stops rather than
			// re-judging its own prefix: descending would report the permitted
			// nested read as its bare `themeState` prefix.
			return false
		})

		if !anchored {
			t.Errorf("%s never reads %s.%s; the guard above would pass vacuously for a helper that compares against nothing at all",
				restoreHelperName, model, restoreAnchorRead)
		}
	})

	t.Run("the retired canvas-hex helper is gone from the tree", func(t *testing.T) {
		root := repoRoot(t)
		for _, goFile := range allGoFiles(t, root) {
			source, err := os.ReadFile(goFile)
			if err != nil {
				t.Fatalf("read %s: %v", goFile, err)
			}
			if strings.Contains(string(source), retiredCanvasHexHelper) {
				rel, _ := filepath.Rel(root, goFile)
				t.Errorf("%s still mentions %s; §11.4 deletes it outright so nothing can re-derive the comparison from a theme",
					rel, retiredCanvasHexHelper)
			}
		}
	})
}

// TestLaunchSites_RestoreIdentically pins that the two program-launch sites
// restore the same way: the same shared helper, handed the program's output
// writer, once each — and that there is no third site to diverge from them.
func TestLaunchSites_RestoreIdentically(t *testing.T) {
	root := repoRoot(t)
	sites := restoreCallSites(t, root)

	for _, rel := range restoreLaunchSites {
		t.Run(rel, func(t *testing.T) {
			args, called := sites[rel]
			if !called {
				t.Fatalf("%s no longer calls %s after its program returns", rel, restoreHelperName)
			}
			if len(args) != 1 {
				t.Fatalf("%s calls %s %d times, want exactly 1", rel, restoreHelperName, len(args))
			}
			if args[0] != "os.Stdout" {
				t.Errorf("%s calls %s(%s, ...); want os.Stdout — both sites must write to the program's output",
					rel, restoreHelperName, args[0])
			}
		})
	}

	t.Run("no third site", func(t *testing.T) {
		known := map[string]bool{}
		for _, rel := range restoreLaunchSites {
			known[rel] = true
		}
		for rel := range sites {
			if !known[rel] {
				t.Errorf("%s also calls %s; the restore is a two-site contract, and a third site is a place the behaviour can diverge",
					rel, restoreHelperName)
			}
		}
	})
}

// restoreHelperDecl returns the restore helper's declaration, failing the test if
// the file no longer declares it.
func restoreHelperDecl(t *testing.T, file *ast.File) *ast.FuncDecl {
	t.Helper()
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok && fn.Recv == nil && fn.Name.Name == restoreHelperName {
			return fn
		}
	}
	t.Fatalf("%s declares no %s", restoreFileName, restoreHelperName)
	return nil
}

// modelParamName returns the name the helper binds its Model parameter to, so the
// body scan can attribute selector reads to the model rather than assume an
// identifier.
func modelParamName(t *testing.T, fn *ast.FuncDecl) string {
	t.Helper()
	for _, param := range fn.Type.Params.List {
		ident, ok := param.Type.(*ast.Ident)
		if !ok || ident.Name != "Model" || len(param.Names) != 1 {
			continue
		}
		return param.Names[0].Name
	}
	t.Fatalf("%s takes no Model parameter", restoreHelperName)
	return ""
}

// restoreCallSites maps each production file that calls the restore helper to the
// rendered first argument of every such call, keyed by path relative to root.
//
// It matches both the qualified call an out-of-package caller writes
// (tui.RestoreTerminalBackground) and a bare in-package one, so a third site
// cannot hide by living inside internal/tui.
func restoreCallSites(t *testing.T, root string) map[string][]string {
	t.Helper()
	sites := map[string][]string{}
	fset := token.NewFileSet()
	for _, path := range allGoFiles(t, root) {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", path, err)
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			t.Fatalf("relativise %s: %v", path, err)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || !callsRestoreHelper(call) || len(call.Args) == 0 {
				return true
			}
			sites[rel] = append(sites[rel], exprText(call.Args[0]))
			return true
		})
	}
	return sites
}

// callsRestoreHelper reports whether a call expression invokes the restore helper,
// qualified (pkg.RestoreTerminalBackground) or bare.
func callsRestoreHelper(call *ast.CallExpr) bool {
	switch fn := call.Fun.(type) {
	case *ast.Ident:
		return fn.Name == restoreHelperName
	case *ast.SelectorExpr:
		return fn.Sel.Name == restoreHelperName
	}
	return false
}

// selectorPath renders a selector expression as its dotted identifier path,
// reporting false for anything that is not a pure chain of identifiers (a field
// read off a call result, an index expression) — which names no path to check.
func selectorPath(sel *ast.SelectorExpr) (string, bool) {
	switch x := sel.X.(type) {
	case *ast.Ident:
		return x.Name + "." + sel.Sel.Name, true
	case *ast.SelectorExpr:
		prefix, ok := selectorPath(x)
		if !ok {
			return "", false
		}
		return prefix + "." + sel.Sel.Name, true
	default:
		return "", false
	}
}

// exprText renders the identifier or qualified identifier an expression names, for
// a failure message and for the os.Stdout comparison. Anything else renders as its
// AST type, which reads as "not the writer we asked for" in the failure.
func exprText(expr ast.Expr) string {
	switch e := expr.(type) {
	case *ast.Ident:
		return e.Name
	case *ast.SelectorExpr:
		return exprText(e.X) + "." + e.Sel.Name
	default:
		return fmt.Sprintf("%T", expr)
	}
}

// allGoFiles returns every .go file under root. The guards above need PATHS —
// one reads raw bytes, one parses full ASTs — which forEachGoFile's
// imports-only parse cannot supply.
func allGoFiles(t *testing.T, root string) []string {
	t.Helper()
	paths, err := portalbintest.GoSourceFiles(root)
	if err != nil {
		t.Fatalf("enumerate .go files: %v", err)
	}
	if len(paths) == 0 {
		t.Fatalf("walk %s matched no .go files", root)
	}
	return paths
}
