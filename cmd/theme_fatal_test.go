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

// TestThemeFatal_TravelsExecuteUnaltered pins the propagation contract the build-time
// guarantee depends on: a fallback that cannot resolve is an ORDINARY ERROR, so it reaches
// the user as one printed line through the path every ordinary error takes.
//
// Three shapes in this package would each swallow that line, and all three are
// live enough to be reached for by mistake:
//
//   - a *bootstrap.FatalError — Execute prints its UserMessage and main
//     deliberately does NOT reprint, so dressing the theme fatal as one would
//     surface the bootstrap's wording in place of the pinned copy's;
//   - a silent-exit sentinel (IsSilentExitError) — main suppresses stderr
//     entirely, and the user would get a bare non-zero exit with no explanation
//     of a binary they cannot fix;
//   - a *UsageError — exit code 2 and a usage framing, for something the user
//     did not do wrong.
//
// The error is injected at the orchestrator seam rather than raised from its
// production site: ResolveNomination has no caller in cmd until the construction
// wiring lands, so what is asserted here is the error's TRAVEL and not its
// origin — Execute hands it back untouched, and writes nothing to the fatal
// stream that would compete with main's single line.
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
		t.Error("the theme fatal classifies as a *bootstrap.FatalError — main suppresses stderr for those, so §14A's line would never be printed")
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

// TestThemeCallSites_TerminateNoProcess asserts no function in cmd that touches
// internal/theme terminates the process.
//
// The build-time guarantee's escalation is a RETURNED error all the way to main.go's single
// os.Exit owner — no bare exit, no log.Fatal, no panic. The prohibition on a
// bare os.Exit outside main is a standing rule of this codebase with exactly one
// sanctioned exception (the daemon self-eject, which pairs itself with a
// terminal marker), and this scope is where the theme work would otherwise be
// tempted to take a second: a fatal discovered at TUI construction is a genuinely
// unrecoverable state, and "just exit here" is the shortest route.
//
// The scan is scoped to theme-touching functions rather than the whole package
// deliberately — cmd legitimately routes through the osExit SEAM elsewhere, and
// a blanket ban would be a guard nobody could keep.
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

// touchesTheme reports whether the function references internal/theme anywhere
// — in its signature or its body. Signature references count deliberately: a
// helper taking a theme.Loader is part of the theme path whether or not it names
// the package again inside.
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

// requireNoProcessTermination fails for any panic, os.Exit, osExit seam call or
// log.Fatal* within the function.
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
