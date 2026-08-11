package cmd

import (
	"bytes"
	"errors"
	"go/ast"
	"strings"
	"testing"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/theme"
)

func TestThemeFatal_TravelsExecuteUnaltered(t *testing.T) {
	resetBootstrapOnce(t)

	fatal := theme.BrokenBuiltinError(theme.DefaultDarkSlug)
	bootstrapDeps = &BootstrapDeps{Orchestrator: &errRunner{err: fatal}}
	t.Cleanup(func() { bootstrapDeps = nil })

	var fatalStream bytes.Buffer
	originalWriter := fatalErrorStderr
	fatalErrorStderr = &fatalStream
	t.Cleanup(func() { fatalErrorStderr = originalWriter })

	resetRootCmd()
	rootCmd.SetArgs([]string{"list"})
	err := Execute()

	if err == nil {
		t.Fatal("Execute() = nil, want the theme fatal returned unaltered")
	}
	if !errors.Is(err, fatal) {
		t.Errorf("Execute() = %v, want the injected error itself — the fatal travels the ordinary path and is never re-wrapped", err)
	}

	var asFatal *bootstrap.FatalError
	if errors.As(err, &asFatal) {
		t.Error("the theme fatal classifies as a *bootstrap.FatalError — main suppresses stderr for those, so the failure line would never be printed")
	}
	if IsSilentExitError(err) {
		t.Error("the theme fatal classifies as a silent-exit error — the user would get a bare non-zero exit with nothing to read")
	}
	var asUsage *UsageError
	if errors.As(err, &asUsage) {
		t.Error("the theme fatal classifies as a *UsageError — exit code 2 and a usage framing, for something the user did not do")
	}
	if fatalStream.Len() != 0 {
		t.Errorf("fatalErrorStderr = %q, want empty — main prints the single line, and a second writer here would double it", fatalStream.String())
	}
}

// Scoped to theme-touching functions deliberately: cmd legitimately routes
// through the osExit seam elsewhere, so a blanket ban would be a guard nobody
// could keep.
func TestThemeCallSites_TerminateNoProcess(t *testing.T) {
	scanned := 0

	for name, file := range parsePackageFilesByName(t) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !touchesTheme(fn) {
				continue
			}
			scanned++
			requireNoProcessTermination(t, name, fn)
		}
	}

	if scanned == 0 {
		t.Fatal("no theme-touching function found in cmd — the guard passed without looking at anything")
	}
}

// Signature references count deliberately: a helper taking a theme.Loader is
// part of the theme path whether or not it names the package again inside.
func touchesTheme(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if ok && pkg.Name == "theme" {
			found = true
			return false
		}
		return true
	})
	return found
}

func requireNoProcessTermination(t *testing.T, file string, fn *ast.FuncDecl) {
	t.Helper()

	ast.Inspect(fn, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		switch target := call.Fun.(type) {
		case *ast.Ident:
			if target.Name == "panic" {
				t.Errorf("%s: %s panics — a theme fatal is an ordinary error returned to main, and main's recover is the backstop for a programming fault, not the route", file, fn.Name.Name)
			}
			if target.Name == "osExit" {
				t.Errorf("%s: %s calls the osExit seam — the theme fatal is returned, not exited on; main.go owns the single exit", file, fn.Name.Name)
			}
		case *ast.SelectorExpr:
			pkg, isIdent := target.X.(*ast.Ident)
			if !isIdent {
				return true
			}
			if pkg.Name == "os" && target.Sel.Name == "Exit" {
				t.Errorf("%s: %s calls os.Exit — a bare exit outside main is prohibited; the theme fatal is returned", file, fn.Name.Name)
			}
			if pkg.Name == "log" && strings.HasPrefix(target.Sel.Name, "Fatal") {
				t.Errorf("%s: %s calls log.%s — the theme fatal is returned to main, which owns termination", file, fn.Name.Name, target.Sel.Name)
			}
		}
		return true
	})
}
