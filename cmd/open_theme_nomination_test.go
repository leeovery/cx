package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/resolver"
	"github.com/leeovery/portal/internal/theme"
	"github.com/spf13/cobra"
)

// TestOpenExecPath_DoesNoThemeWork pins the `theme` log component's recorded win: on the path
// Portal is most careful to keep free of cost — `portal open <target>`, which execs
// without painting — this feature adds nothing at all.
//
// Both halves are needed, and they fail differently.
//
// The SOURCE guard catches a call site added where no TUI is constructed: a
// theme read in the resolution/exec branch, or in a shared pre-run step every
// verb passes through, would run on every `x <target>` and leave no trace in a
// run that execs before anything is painted. Two files are exempt IN FULL, each
// because it IS a separate verb whose whole job is theme work and which no
// `portal open` invocation reaches: cmd/theme.go is `portal theme export`, and
// cmd/doctor_theme.go is `portal doctor`'s themes-directory scan. Unreachability
// from open — not bootstrap exemption — is the whole basis of both.
//
// An exempt file hides every helper it declares, so this half backstops its own
// exemption by TRACKING that file's entry point as a local helper:
// collectThemeAdvisories sits in the `local` map below, which leaves the scan's
// internals unguarded but fails the moment the scan itself is wired into the open
// path ("open.go: openResolved calls collectThemeAdvisories"). The runtime half
// below does NOT back the exemption up and must not be read as doing so: doctor
// hands its loader log.Discard() by design, so a doctor-side helper
// called from the exec path would read the poisoned directory and still write no
// record.
//
// The RUNTIME half catches the same regression from the other side for theme work
// that DOES log, and makes the claim about the WHOLE program rather than about
// this package's call sites: a real `portal open <session>` runs with the themes
// directory poisoned to a mode-0000 path — any read of it would raise a
// `theme: directory unusable` WARN — and the `theme` component must emit nothing
// at all.
func TestOpenExecPath_DoesNoThemeWork(t *testing.T) {
	t.Run("no theme call site sits outside TUI construction", func(t *testing.T) {
		allowed := map[string]bool{
			// The TUI-construction path — the only place a theme is USED.
			"openTUI": true,
			// The construction-time resolution and the two loader constructors it
			// reaches: prefs' keys → the setting → the per-slot load.
			"themeResolution":  true,
			"buildThemeLoader": true,
			"newThemeLoader":   true,
			// newThemeEnumerator is deliberately ABSENT. It resolves the themes
			// directory and reads nothing, so it encloses no theme call site to
			// permit — and naming it here would licence in advance exactly the
			// construction-time sweep the lazy-discovery rule forbids, since these names are matched in
			// EVERY file. What puts the constructor under this guard is the `local`
			// map below, which tracks it as openTUI's callee.
		}

		exemptFiles := map[string]bool{"theme.go": true, "doctor_theme.go": true}

		for file, callers := range themeCallSites(t) {
			if exemptFiles[file] {
				continue
			}
			for fn, call := range callers {
				if !allowed[fn] {
					t.Errorf("%s: %s calls %s — theme work belongs to TUI construction; the exec path constructs no TUI and must stay free of it", file, fn, call)
				}
			}
		}
	})

	t.Run("an exec-path open emits no theme record", func(t *testing.T) {
		poisonThemesDir(t)
		// A DROP-IN slug, so resolving it must consult the themes directory — a
		// built-in would resolve out of the embedded set and never touch the poison.
		setPrefsFile(t, `{"theme":"a-drop-in"}`)

		// The fixture has to be LOUD or the zero-record assertion below could pass
		// for want of anything observable. Running the construction-time resolution
		// against it proves the records exist to be seen.
		loud := installMigrateCapture(t)
		themeNominationForTest(t)
		if len(themeEvents(t, loud)) == 0 {
			t.Fatal("the construction-time resolution emitted no theme record against the poisoned directory; the zero-record assertion below would be vacuous")
		}

		sink := installMigrateCapture(t)

		if got := execOpenSession(t, "api-x7Kd9a"); got != "api-x7Kd9a" {
			t.Fatalf("open attached %q, want the session it resolved — the exec path must have run", got)
		}

		assertThemeEvents(t, sink)
	})
}

// poisonThemesDir points PORTAL_THEMES_DIR at an existing but UNREADABLE
// directory, so any attempt to read it is loud: the directory-resolution rule makes an
// unusable directory the one state that earns a `theme: directory unusable` WARN, where an
// absent one is silent. Absence would make "emitted nothing" vacuous.
func poisonThemesDir(t *testing.T) {
	t.Helper()

	dir := useThemesDir(t)
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("make themes dir unreadable: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
}

// execOpenSession runs a real `portal open <target>` that resolves in the session
// domain, and returns the session it handed to the connector. Every tmux-touching
// seam is injected, so the body runs its production resolution → outcome switch
// without reaching a server.
func execOpenSession(t *testing.T, name string) string {
	t.Helper()

	bootstrapDeps = &BootstrapDeps{Orchestrator: &nopRunner{}}
	t.Cleanup(func() { bootstrapDeps = nil })
	openDeps = &OpenDeps{
		SessionLister: &testSessionLister{names: []string{name}},
		AliasLookup:   &testAliasLookup{aliases: map[string]string{}},
		Zoxide:        &testZoxideQuerier{err: resolver.ErrNoMatch},
		DirValidator:  &testDirValidator{existing: map[string]bool{}},
	}
	t.Cleanup(func() { openDeps = nil })

	var attached string
	previous := openSessionFunc
	openSessionFunc = func(_ *cobra.Command, target string) error {
		attached = target
		return nil
	}
	t.Cleanup(func() { openSessionFunc = previous })

	resetRootCmd()
	rootCmd.SetOut(io.Discard)
	rootCmd.SetErr(io.Discard)
	rootCmd.SetArgs([]string{"open", name})
	if err := rootCmd.Execute(); err != nil {
		t.Fatalf("portal open %s: %v", name, err)
	}
	return attached
}

// TestThemeComponent_BoundOnceInCmd pins CLAUDE.md's bind-once-per-package rule
// for the `theme` component: one package-level logger, bound once.
//
// The component is legitimately emitted from more than one PACKAGE (the
// loader, the translation, the persister), which is exactly why the per-package
// rule needs guarding here: a second binding inside cmd would be invisible at
// review, since every call site would still look correct in isolation.
func TestThemeComponent_BoundOnceInCmd(t *testing.T) {
	bindings := componentBindings(t, "theme")

	if len(bindings) != 1 {
		t.Fatalf(`cmd binds the "theme" log component %d times via %v, want exactly 1 (a single package-level themeLogger)`, len(bindings), bindings)
	}
	if bindings[0] != "themeLogger" {
		t.Errorf(`the "theme" component is bound to %q, want the package-level var "themeLogger"`, bindings[0])
	}
}

// assertConstant asserts the nomination is the constant state holding want.
func assertConstant(t *testing.T, n theme.Nomination, want theme.Theme) {
	t.Helper()
	if !n.IsConstant() {
		t.Fatalf("nomination is not constant; a pinned appearance must become a pinned THEME, with detection still off")
	}
	if got := n.Constant(); got != want {
		t.Errorf("constant = %s, want %s", canvasOf(got), canvasOf(want))
	}
}

// canvasOf names a theme by its canvas for a failure message — a whole Theme
// through %+v is 19 {name value} pairs of noise.
func canvasOf(th theme.Theme) string {
	if th.Canvas.Value == "" {
		return "zero-theme"
	}
	return "theme(canvas " + th.Canvas.Value + ")"
}

// themeCallSites maps each production source file to the enclosing function of
// every call it makes into internal/theme or into the local theme helpers.
func themeCallSites(t *testing.T) map[string]map[string]string {
	t.Helper()
	local := map[string]bool{
		"themeResolution":    true,
		"buildThemeLoader":   true,
		"newThemeLoader":     true,
		"newThemeEnumerator": true,
		// Tracked because its FILE is exempt in full: exempting the file would
		// otherwise make every helper declared in it invisible to this scan, so a
		// doctor-side helper called from the exec path would read as no call at
		// all. Tracking the entry point puts the exempt file's ONE production
		// caller back under the guard — see the exemption note on the test.
		//
		// That one caller is invisible here only because it sits in doctorCmd's
		// composite-literal RunE, which is not an *ast.FuncDecl. Extracting it into
		// a named function in doctor.go trips this guard; keep the call in the
		// literal rather than widening `allowed`, whose names are matched in every
		// file including open.go.
		"collectThemeAdvisories": true,
	}
	sites := map[string]map[string]string{}

	for name, file := range parseCmdFiles(t) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				switch target := call.Fun.(type) {
				case *ast.SelectorExpr:
					pkg, ok := target.X.(*ast.Ident)
					if !ok || pkg.Name != "theme" {
						return true
					}
					record(sites, name, fn.Name.Name, "theme."+target.Sel.Name)
				case *ast.Ident:
					if local[target.Name] {
						record(sites, name, fn.Name.Name, target.Name)
					}
				}
				return true
			})
		}
	}
	return sites
}

// record notes one call site under its file and enclosing function.
func record(sites map[string]map[string]string, file, fn, call string) {
	if sites[file] == nil {
		sites[file] = map[string]string{}
	}
	sites[file][fn] = call
}

// componentBindings returns one entry per log.For(component) call in the
// package's production sources, in either of the two shapes a binding can take:
// the VAR NAME where the call initialises a package-level var (the sanctioned
// shape), and "file:function" where it sits inside a function body (a second
// binding by another name, which must still count as one).
func componentBindings(t *testing.T, component string) []string {
	t.Helper()
	var bound []string
	for _, file := range parseCmdFiles(t) {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				continue
			}
			for _, spec := range gen.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, expr := range value.Values {
					if isLogForCall(expr, component) && i < len(value.Names) {
						bound = append(bound, value.Names[i].Name)
					}
				}
			}
		}
	}
	// Any log.For(component) call outside a package-level var is a second binding
	// by another name, and must count as one.
	return append(bound, logForCalls(t, component)...)
}

// isLogForCall reports whether expr is exactly log.For("<component>").
func isLogForCall(expr ast.Expr, component string) bool {
	call, ok := expr.(*ast.CallExpr)
	if !ok || len(call.Args) != 1 {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "For" {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "log" {
		return false
	}
	lit, ok := call.Args[0].(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && lit.Value == `"`+component+`"`
}

// logForCalls returns a marker per log.For(component) call made INSIDE a function
// body — i.e. every binding that is not the package-level var.
func logForCalls(t *testing.T, component string) []string {
	t.Helper()
	var found []string
	for name, file := range parseCmdFiles(t) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok {
				continue
			}
			ast.Inspect(fn, func(n ast.Node) bool {
				expr, ok := n.(ast.Expr)
				if ok && isLogForCall(expr, component) {
					found = append(found, name+":"+fn.Name.Name)
				}
				return true
			})
		}
	}
	return found
}

// parseCmdFiles parses the cmd package's production sources, keyed by filename.
// go test runs in the package's source directory, so the relative walk resolves
// wherever the suite was invoked from.
func parseCmdFiles(t *testing.T) map[string]*ast.File {
	t.Helper()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}
	fset := token.NewFileSet()
	files := map[string]*ast.File{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		parsed, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		files[name] = parsed
	}
	return files
}
