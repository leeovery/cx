package cmd

import (
	"go/ast"
	"go/parser"
	"go/token"
	"image/color"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// TestAppearanceMapping_PinToConstant pins §10.2's exact mapping of the surviving
// legacy `appearance` pref onto a theme nomination, and follows it through to the
// canvas the model actually paints.
//
// The mapping is exact by design, and that is what makes it cheap: `dark` meant
// "always dark regardless of terminal", whose new equivalent is a CONSTANT theme —
// so a user who pinned a mode keeps a pinned theme, and detection stays off for
// them just as it was. `auto` maps to the shipped adaptive pair, which is what
// `auto` already meant.
//
// The painted half is asserted because the mapping alone proves only half a
// claim: a nomination holding the right palette that the model then ignores would
// pass a value check and still render the wrong screen. The observable is
// View().BackgroundColor — the owned canvas Portal sets via OSC 11 — which is nil
// under the pair precisely because an adaptive gate paints NOTHING until it
// resolves.
func TestAppearanceMapping_PinToConstant(t *testing.T) {
	loader := newThemeLoader()
	dark := builtinThemeForTest(t, theme.DefaultDarkSlug)
	light := builtinThemeForTest(t, theme.DefaultLightSlug)

	for _, tc := range []struct {
		name       string
		appearance prefs.Appearance
		assert     func(*testing.T, theme.Nomination)
		wantCanvas color.Color
	}{
		{
			name:       "dark pins the constant tokyo-night",
			appearance: prefs.AppearanceDark,
			assert:     func(t *testing.T, n theme.Nomination) { assertConstant(t, n, dark) },
			wantCanvas: dark.Canvas.Color(),
		},
		{
			name:       "light pins the constant tokyo-night-day",
			appearance: prefs.AppearanceLight,
			assert:     func(t *testing.T, n theme.Nomination) { assertConstant(t, n, light) },
			wantCanvas: light.Canvas.Color(),
		},
		{
			name:       "auto is the shipped adaptive pair",
			appearance: prefs.AppearanceAuto,
			assert: func(t *testing.T, n theme.Nomination) {
				if n.IsConstant() {
					t.Fatalf("auto produced a constant nomination; want the adaptive pair (ignoring `appearance: auto` lands exactly on the shipped default)")
				}
				if got := n.Select(true); got != dark {
					t.Errorf("auto pair's dark member = %s, want %s", canvasOf(got), canvasOf(dark))
				}
				if got := n.Select(false); got != light {
					t.Errorf("auto pair's light member = %s, want %s", canvasOf(got), canvasOf(light))
				}
			},
			// No member is active until the gate resolves, so nothing is painted.
			wantCanvas: nil,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nomination := nominationForAppearance(loader, tc.appearance)
			tc.assert(t, nomination)

			cfg := defaultTestTUIConfig()
			cfg.theme = nomination
			if got := buildTUIModel(cfg, "", nil).View().BackgroundColor; got != tc.wantCanvas {
				t.Errorf("painted canvas = %v, want %v", got, tc.wantCanvas)
			}
		})
	}
}

// TestAppearanceMapping_IsInMemoryOnly pins §10.2's other half: the translation
// is computed and used IN MEMORY, and this task writes nothing.
//
// Both file states are covered because they fail differently: an absent
// prefs.json must not be CREATED (§8.1 leaves a fresh install without one, and
// nothing here may add a side effect to that path), and an existing one must come
// back byte-for-byte — a whole-file rewrite that merely round-tripped would still
// be a write on a path specified to have none.
func TestAppearanceMapping_IsInMemoryOnly(t *testing.T) {
	t.Run("an absent prefs.json stays absent", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "prefs.json")
		t.Setenv("PORTAL_PREFS_FILE", path)

		if got := nominationFromPrefsForTest(t); got.IsConstant() {
			t.Errorf("a first-ever launch produced a constant nomination; want the shipped adaptive pair")
		}

		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("prefs.json exists after the mapping (stat err = %v); want it still absent", err)
		}
	})

	t.Run("an existing prefs.json is unchanged, byte for byte", func(t *testing.T) {
		const content = `{"session_list_mode":"by-tag","appearance":"dark"}`
		path := filepath.Join(t.TempDir(), "prefs.json")
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatalf("write prefs file: %v", err)
		}
		t.Setenv("PORTAL_PREFS_FILE", path)

		assertConstant(t, nominationFromPrefsForTest(t), builtinThemeForTest(t, theme.DefaultDarkSlug))

		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read prefs file back: %v", err)
		}
		if string(after) != content {
			t.Errorf("prefs.json changed:\n got %s\nwant %s\n(the translation is in-memory only — nothing is persisted)", after, content)
		}
	})
}

// TestOpenExecPath_DoesNoThemeWork pins §12.3's recorded win: on the path Portal
// is most careful to keep free of cost — `portal open <target>`, which execs
// without painting — this feature adds nothing at all.
//
// A source guard, because "did no work" has no runtime trace: the exec path
// constructs no TUI, so the assertion is that every theme call site in the package
// sits inside the TUI-construction path (openTUI) or inside the mapping itself. A
// call added to the resolution/exec branch — or to a shared pre-run step that
// every verb passes through — would be caught here and nowhere else.
//
// cmd/theme.go is exempt in full: that file IS `portal theme export`, a separate
// verb whose whole job is theme work, which no `portal open` invocation reaches.
func TestOpenExecPath_DoesNoThemeWork(t *testing.T) {
	allowed := map[string]bool{
		// The TUI-construction path — the only place a theme is used, and the only
		// place §10.5 puts the translation.
		"openTUI": true,
		// The mapping, the loader constructor and the by-slug read they share.
		"nominationForAppearance": true,
		"newThemeLoader":          true,
		"loadBuiltinOrZero":       true,
	}

	for file, callers := range themeCallSites(t) {
		if file == "theme.go" {
			continue
		}
		for fn, call := range callers {
			if !allowed[fn] {
				t.Errorf("%s: %s calls %s — theme work belongs to TUI construction; the exec path constructs no TUI and must stay free of it", file, fn, call)
			}
		}
	}
}

// TestThemeComponent_BoundOnceInCmd pins CLAUDE.md's bind-once-per-package rule
// for the `theme` component (§12.3): one package-level logger, bound once.
//
// The component is legitimately emitted from more than one PACKAGE (§8.9 — the
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

// nominationFromPrefsForTest mirrors openTUI's read: load the store, read the
// legacy appearance tolerantly, map it. Both steps are the production functions.
func nominationFromPrefsForTest(t *testing.T) theme.Nomination {
	t.Helper()
	store, err := loadPrefsStore()
	if err != nil {
		t.Fatalf("load prefs store: %v", err)
	}
	appearance, _ := store.LoadAppearance()
	return nominationForAppearance(newThemeLoader(), appearance)
}

// builtinThemeForTest loads one embedded built-in, failing on anything but a
// clean parse (§7.6 makes a rejection here a build-time impossibility).
func builtinThemeForTest(t *testing.T, slug string) theme.Theme {
	t.Helper()
	loaded, rejection, found := theme.NewLoader(nil).LoadBuiltin(slug)
	if !found {
		t.Fatalf("built-in %q not found in the embedded set", slug)
	}
	if rejection != nil {
		t.Fatalf("built-in %q was rejected: %s", slug, rejection.Reason)
	}
	return loaded.Theme
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
	local := map[string]bool{"nominationForAppearance": true, "newThemeLoader": true}
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
