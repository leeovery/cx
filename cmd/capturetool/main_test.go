package main

// Tests in this file read NO_COLOR and MUST NOT use t.Parallel.

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
	"github.com/leeovery/portal/internal/tui"
)

// TestFlags_AreFixtureAndThemeOnly pins §13.3's flag surface: --appearance is
// REMOVED, not kept alongside --theme, and --theme defaults to the shipped dark
// built-in.
//
// It is a source guard because flag registration happens in main(), which no test
// runs — a surviving --appearance would compile, parse and be honoured forever
// with nothing failing. The registered names are read from the AST rather than
// grepped, so a doc comment explaining what --theme replaced cannot satisfy or
// break the assertion.
func TestFlags_AreFixtureAndThemeOnly(t *testing.T) {
	flags := registeredStringFlags(t)

	names := make([]string, 0, len(flags))
	for name := range flags {
		names = append(names, name)
	}
	slices.Sort(names)

	if want := []string{"fixture", "theme"}; !slices.Equal(names, want) {
		t.Errorf("registered flags = %v, want %v — a theme IS the mode, so there is no appearance left to pin (§13.3)", names, want)
	}
	if got, want := flags["theme"], "defaultThemeSlug"; got != want {
		t.Errorf("--theme default = %s, want the %s constant", got, want)
	}
}

// registeredStringFlags returns every flag.String the harness registers, mapped
// from its name to the source expression of its default value.
func registeredStringFlags(t *testing.T) map[string]string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	flags := map[string]string{}
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok || len(call.Args) < 2 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel.Name != "String" {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || pkg.Name != "flag" {
			return true
		}
		name, ok := call.Args[0].(*ast.BasicLit)
		if !ok || name.Kind != token.STRING {
			return true
		}
		flags[strings.Trim(name.Value, `"`)] = types.ExprString(call.Args[1])
		return true
	})
	return flags
}

// TestResolveTheme_DefaultsToTheShippedDarkBuiltin pins the built-in --theme
// resolves to when the flag is omitted, and pins WHERE that value comes from.
//
// It is the shipped dark default (§13.3) and every capture taken without the flag
// depends on it, so the constant is DERIVED from theme.DefaultDarkSlug rather
// than restating its value: moving the shipped default has to move every
// unflagged capture with it, not leave them rendering a palette the product no
// longer defaults to with nothing failing. The source guard is what makes the
// derivation checkable — a literal spelling out today's default would satisfy the
// equality below forever.
func TestResolveTheme_DefaultsToTheShippedDarkBuiltin(t *testing.T) {
	if defaultThemeSlug != theme.DefaultDarkSlug {
		t.Fatalf("defaultThemeSlug = %q, want the shipped dark default %q", defaultThemeSlug, theme.DefaultDarkSlug)
	}
	if got, want := declaredConstSource(t, "defaultThemeSlug"), "theme.DefaultDarkSlug"; got != want {
		t.Errorf("defaultThemeSlug is declared as %s, want %s — the capture default follows the shipped dark default rather than restating it", got, want)
	}

	got, err := resolveTheme(theme.NewSilentLoader(), defaultThemeSlug, io.Discard)
	if err != nil {
		t.Fatalf("resolveTheme(%s): %v", defaultThemeSlug, err)
	}

	want, rejection, found := theme.NewSilentLoader().LoadBuiltin(theme.DefaultDarkSlug)
	if !found || rejection != nil {
		t.Fatalf("LoadBuiltin(%s) found=%v rejection=%v", theme.DefaultDarkSlug, found, rejection)
	}
	if got != want.Theme {
		t.Errorf("resolveTheme(%s) did not return the embedded %s palette", defaultThemeSlug, theme.DefaultDarkSlug)
	}
}

// declaredConstSource returns the source expression the named package-level
// constant is declared from, so a test can assert a value is DERIVED rather than
// restated — which its runtime value alone can never show.
func declaredConstSource(t *testing.T, name string) string {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "main.go", nil, parser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parse main.go: %v", err)
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.CONST {
			continue
		}
		for _, spec := range gen.Specs {
			value, ok := spec.(*ast.ValueSpec)
			if !ok || len(value.Names) != 1 || len(value.Values) != 1 || value.Names[0].Name != name {
				continue
			}
			return types.ExprString(value.Values[0])
		}
	}

	t.Fatalf("main.go declares no const %s", name)
	return ""
}

// TestThemeArg_SlugVersusPath pins §13.3's discrimination rule: the argument is a
// PATH if it carries a path separator OR ends in `.theme`, and a built-in SLUG
// otherwise.
//
// The separator half is the load-bearing one. Without it `./mytheme.txt` — a real
// file that plainly exists — classifies as a slug and is rejected as an unknown
// built-in, naming the wrong problem entirely.
func TestThemeArg_SlugVersusPath(t *testing.T) {
	tests := []struct {
		name   string
		arg    string
		isPath bool
	}{
		{name: "a bare slug", arg: "nord"},
		{name: "a bare .theme filename", arg: "nord.theme", isPath: true},
		{name: "a relative .theme path", arg: "./nord.theme", isPath: true},
		{name: "an absolute .theme path", arg: "/abs/nord.theme", isPath: true},
		{name: "a nested path with no extension", arg: "sub/dir/x", isPath: true},
		{name: "a relative path with an unexpected extension", arg: "./mytheme.txt", isPath: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isThemePath(tt.arg); got != tt.isPath {
				t.Errorf("isThemePath(%q) = %v, want %v", tt.arg, got, tt.isPath)
			}
		})
	}
}

// TestResolveTheme_UnknownSlugIsAnError asserts a slug naming no built-in is a
// hard error carrying the slug, never a silent fall back to some other palette.
//
// Rendering the wrong theme at a visual gate is precisely the failure this tool
// exists to prevent (§13.3), so an unknown slug must render nothing at all.
func TestResolveTheme_UnknownSlugIsAnError(t *testing.T) {
	got, err := resolveTheme(theme.NewSilentLoader(), "not-a-theme", io.Discard)
	if err == nil {
		t.Fatal("resolveTheme(not-a-theme) returned nil error, want an error")
	}
	if !strings.Contains(err.Error(), "not-a-theme") {
		t.Errorf("error %q does not name the slug that was asked for", err.Error())
	}
	if got != (theme.Theme{}) {
		t.Error("resolveTheme returned a palette alongside its error")
	}
}

// TestResolveTheme_InvalidBuiltinIsAnErrorNotAFallback asserts a slug that DOES
// name a built-in whose file fails validation is a hard error carrying the §6.2
// reason — again never a fallback.
//
// §7.6's build-time guarantee makes this unreachable through the shipped set, so
// the rejection arrives through an injected loader rather than by breaking a
// shipped file.
func TestResolveTheme_InvalidBuiltinIsAnErrorNotAFallback(t *testing.T) {
	rejection := &theme.Rejection{Reason: theme.ReasonMissingTokens, Detail: "missing canvas, border"}

	got, err := resolveTheme(rejectingLoader{rejection: rejection}, "tokyo-night", io.Discard)
	if err == nil {
		t.Fatal("resolveTheme returned nil error for a rejected built-in, want an error rather than a fallback")
	}
	for _, want := range []string{"tokyo-night", string(theme.ReasonMissingTokens), rejection.Detail} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not carry %q", err.Error(), want)
		}
	}
	if got != (theme.Theme{}) {
		t.Error("resolveTheme returned a palette alongside its error")
	}
}

// rejectingLoader is a theme loader that refuses every slug: each one is a known
// built-in whose file the §6.2 ladder refused.
type rejectingLoader struct {
	rejection *theme.Rejection
}

// LoadBuiltin returns the canned rejection, found, and no palette — the shape
// theme.Loader produces for a built-in that does not parse.
func (l rejectingLoader) LoadBuiltin(string) (theme.Result, *theme.Rejection, bool) {
	return theme.Result{}, l.rejection, true
}

// TestResolveTheme_PathContentReasonsAreHardErrors pins §13.3's contract for an
// explicit path: ONLY the four content reasons apply, and each is a hard error
// carrying its §6.2 reason rather than a fallback.
//
// The files are staged and loaded for real — this is the one place the harness's
// path branch meets the ladder, and a fake loader would prove only that the fake
// was wired up.
func TestResolveTheme_PathContentReasonsAreHardErrors(t *testing.T) {
	tests := []struct {
		name       string
		setup      func(t *testing.T, dir string) string
		wantReason theme.Reason
	}{
		{
			name:       "a duplicate key",
			wantReason: theme.ReasonBadSyntax,
			setup: func(t *testing.T, dir string) string {
				return themetest.Write(t, dir, "broken.theme", append(themetest.Lines(), "text.primary = #010203"))
			},
		},
		{
			name:       "a bad hex",
			wantReason: theme.ReasonBadColour,
			setup: func(t *testing.T, dir string) string {
				return themetest.Write(t, dir, "broken.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))
			},
		},
		{
			name:       "a missing token",
			wantReason: theme.ReasonMissingTokens,
			setup: func(t *testing.T, dir string) string {
				return themetest.Write(t, dir, "broken.theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))
			},
		},
		{
			name:       "an absent file",
			wantReason: theme.ReasonUnreadable,
			setup: func(t *testing.T, dir string) string {
				return filepath.Join(dir, "absent.theme")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup(t, t.TempDir())

			got, err := resolveTheme(theme.NewSilentLoader(), path, io.Discard)

			if err == nil {
				t.Fatalf("resolveTheme(%q) returned nil error, want the hard error %q", path, tt.wantReason)
			}
			if !strings.Contains(err.Error(), string(tt.wantReason)) {
				t.Errorf("error %q does not carry the §6.2 reason %q", err.Error(), tt.wantReason)
			}
			if got != (theme.Theme{}) {
				t.Error("resolveTheme returned a palette alongside its error")
			}
		})
	}
}

// TestResolveTheme_NoFallbackOnFailure pins the consequence of every failure
// above: NOTHING renders. Not the default theme, not the built-in the slug nearly
// named — no model at all, on either branch of the harness.
//
// This is the failure the tool exists to prevent (§13.3): a visual gate whose
// whole job is judging colours cannot be handed a frame of some other palette,
// and a silent fallback is indistinguishable from a correct capture to a reviewer.
func TestResolveTheme_NoFallbackOnFailure(t *testing.T) {
	dir := t.TempDir()

	args := []struct {
		name string
		arg  string
	}{
		{name: "an unknown slug", arg: "not-a-theme"},
		{name: "a broken file", arg: themetest.Write(t, dir, "broken.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))},
		{name: "an absent file", arg: filepath.Join(dir, "absent.theme")},
	}

	fixtures := []string{"sessions-flat", capture.ContrastValidationFixture}

	for _, ta := range args {
		t.Run(ta.name, func(t *testing.T) {
			got, err := resolveTheme(theme.NewSilentLoader(), ta.arg, io.Discard)
			if err == nil {
				t.Fatalf("resolveTheme(%q) returned nil error", ta.arg)
			}
			if got != (theme.Theme{}) {
				t.Errorf("resolveTheme(%q) fell back to %+v, want the zero palette", ta.arg, got)
			}

			for _, fixture := range fixtures {
				m, err := resolveProgram(fixture, ta.arg, io.Discard)
				if err == nil {
					t.Errorf("resolveProgram(%s, %q) returned nil error", fixture, ta.arg)
				}
				if m != nil {
					t.Errorf("resolveProgram(%s, %q) returned a model alongside its error — nothing must render", fixture, ta.arg)
				}
			}
		})
	}
}

// TestResolveTheme_PathBadNameWarnsWithoutBlocking pins the first of the two
// filename reasons on the path form: a fatal filename WARNS on stderr and renders
// anyway.
//
// Warning rather than blocking is what the flag's stated purpose demands (§13.3):
// this is the only visual-verification route a drop-in author has, so it is the
// one place a fatal name is worth catching before the file reaches the themes
// directory — and refusing to render would leave them with no route at all.
//
// Both §5.2 causes are covered, because both are `bad name` and both warn with the
// one line — capturetool is not doctor, which is where §14A's per-cause lines live.
func TestResolveTheme_PathBadNameWarnsWithoutBlocking(t *testing.T) {
	tests := []struct {
		name string
		base string
	}{
		{name: "an upper-case stem", base: "Nord.THEME"},
		{name: "an unexpected extension", base: "mytheme.txt"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := themetest.Write(t, t.TempDir(), tt.base, themetest.Lines())
			var warnings bytes.Buffer

			got, err := resolveTheme(theme.NewSilentLoader(), path, &warnings)

			if err != nil {
				t.Fatalf("resolveTheme(%q) = %v, want the theme rendered despite its name", path, err)
			}
			if got == (theme.Theme{}) {
				t.Error("resolveTheme returned the zero palette for a valid file")
			}
			want := fmt.Sprintf("capturetool: warning: %s: bad name — a themes-directory file must be named <slug>.theme, lowercase (rendering it anyway)\n", tt.base)
			if warnings.String() != want {
				t.Errorf("warnings = %q, want %q", warnings.String(), want)
			}
		})
	}
}

// TestResolveTheme_PathReservedNameWarnsWithoutBlocking pins the second: a file
// whose basename yields a BUILT-IN's slug warns and renders.
//
// The candidate slug exists solely to produce this line (§13.3) and is never used
// as identity — the theme it names is still the one the file declared, and the
// Result carries no slug at all.
//
// Blocking would break the workflow §12.1 publishes: `portal theme export` writes
// a built-in out under its own name, so an exported theme IS a reserved slug right
// up until the user renames it.
func TestResolveTheme_PathReservedNameWarnsWithoutBlocking(t *testing.T) {
	const base = "nord.theme"
	if !slices.Contains(theme.BuiltinSlugs(), "nord") {
		t.Fatalf("nord is not a built-in (%v) — the fixture cannot collide", theme.BuiltinSlugs())
	}

	path := themetest.Write(t, t.TempDir(), base, themetest.Lines())
	var warnings bytes.Buffer

	got, err := resolveTheme(theme.NewSilentLoader(), path, &warnings)

	if err != nil {
		t.Fatalf("resolveTheme(%q) = %v, want the theme rendered despite its name", path, err)
	}
	if got == (theme.Theme{}) {
		t.Error("resolveTheme returned the zero palette for a valid file")
	}
	// The rendered palette is the FILE's, not the built-in's — the candidate slug
	// produced a warning and nothing else.
	builtin, _, _ := theme.NewSilentLoader().LoadBuiltin("nord")
	if got == builtin.Theme {
		t.Error("resolveTheme rendered the built-in nord palette, want the file's own — the candidate slug is not identity")
	}
	want := "capturetool: warning: nord.theme: reserved name — \"nord\" is a built-in slug, so a file with this name is ignored in the themes directory (rendering it anyway)\n"
	if warnings.String() != want {
		t.Errorf("warnings = %q, want %q", warnings.String(), want)
	}
}

// TestResolveTheme_SlugFormNeverWarns pins the other half of §13.3's "path form
// only": a SLUG argument names a built-in by design, so neither filename reason
// applies to it.
//
// `nord` is the case that makes this non-trivial rather than incidental: it IS a
// built-in slug, so a reserved-name check applied to the slug form would fire on
// the normal, documented invocation and warn every user about the thing they
// asked for.
func TestResolveTheme_SlugFormNeverWarns(t *testing.T) {
	for _, slug := range theme.BuiltinSlugs() {
		t.Run(slug, func(t *testing.T) {
			var warnings bytes.Buffer

			if _, err := resolveTheme(theme.NewSilentLoader(), slug, &warnings); err != nil {
				t.Fatalf("resolveTheme(%s): %v", slug, err)
			}

			if warnings.Len() != 0 {
				t.Errorf("--theme %s warned %q, want silence — a slug names a built-in by design", slug, warnings.String())
			}
		})
	}
}

// TestResolveModel verifies the capture tool resolves a fixture name into a real
// production tui.Model via the shared tui.Build constructor — without opening a
// tmux server or touching config (the fixture is fully in-memory).
func TestResolveModel(t *testing.T) {
	pinned := builtinForTest(t, defaultThemeSlug)

	t.Run("known fixture builds a sessions-page model", func(t *testing.T) {
		m, err := resolveModel("sessions-flat", pinned)
		if err != nil {
			t.Fatalf("resolveModel(sessions-flat): %v", err)
		}
		if m.ActivePage() != tui.PageSessions {
			t.Errorf("ActivePage() = %d, want PageSessions", m.ActivePage())
		}
	})

	t.Run("unknown fixture is an error that lists the available fixtures", func(t *testing.T) {
		_, err := resolveModel("nope", pinned)
		if err == nil {
			t.Fatal("resolveModel(nope) returned nil error, want error")
		}
		if !strings.Contains(err.Error(), "sessions-flat") {
			t.Errorf("error %q does not list the available fixtures", err.Error())
		}
	})

	t.Run("empty fixture name is an error", func(t *testing.T) {
		if _, err := resolveModel("", pinned); err == nil {
			t.Fatal("resolveModel(\"\") returned nil error, want error")
		}
	})
}

// TestResolveModel_PassesConstantNomination pins the shape §13.3 requires of every
// tui.Build capture: the CONSTANT nomination, never an adaptive pair.
//
// A constant is what makes a capture byte-deterministic — no OSC 11 detection, no
// first-paint wait, the palette named on the command line and no other. Both
// halves are observable on the FIRST frame and both are asserted: an adaptive
// nomination would hold the neutral blank frame (BackgroundColor nil) until a
// reply that never arrives outside a running program, and a nomination carrying
// anything but the resolved theme would paint a different canvas.
//
// The theme is loaded from a PATH, so the assertion cannot pass on the built-in
// the model seeds itself with when nothing is injected.
func TestResolveModel_PassesConstantNomination(t *testing.T) {
	pinned := pathThemeForTest(t, "#1A2B3C")

	m, err := resolveModel("sessions-flat", pinned)
	if err != nil {
		t.Fatalf("resolveModel(sessions-flat): %v", err)
	}

	got := m.View().BackgroundColor
	if got == nil {
		t.Fatal("the first frame painted no canvas — the nomination is not constant (an adaptive pair holds the blank frame)")
	}
	if want := pinned.Canvas.Color(); got != want {
		t.Errorf("canvas = %v, want the pinned theme's %v", got, want)
	}
}

// TestResolveModel_NoColorWinsOverTheme pins the §2.5 carve-out's precedence on
// the fixture path: NO_COLOR is read after the theme resolves and WINS over it,
// because a colourless render has no canvas to select.
//
// The pinned theme is a path theme with a canvas hex no built-in carries, so the
// negative assertion is about the resolved palette rather than about colour in
// general — and the positive control proves the same call paints it when NO_COLOR
// is absent.
func TestResolveModel_NoColorWinsOverTheme(t *testing.T) {
	pinned := pathThemeForTest(t, "#1A2B3C")

	t.Run("NO_COLOR renders the colourless native-bg frame", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")

		m, err := resolveModel("sessions-flat", pinned)
		if err != nil {
			t.Fatalf("resolveModel(sessions-flat): %v", err)
		}

		if bg := m.View().BackgroundColor; bg != nil {
			t.Errorf("View().BackgroundColor = %v, want nil — NO_COLOR sets no canvas", bg)
		}
		if content := m.View().Content; strings.Contains(content, "48;2;") {
			t.Errorf("the colourless frame emits a background SGR:\n%q", content)
		}
	})

	t.Run("without NO_COLOR the pinned theme paints", func(t *testing.T) {
		t.Setenv("NO_COLOR", "")
		if err := os.Unsetenv("NO_COLOR"); err != nil {
			t.Fatalf("unset NO_COLOR: %v", err)
		}

		m, err := resolveModel("sessions-flat", pinned)
		if err != nil {
			t.Fatalf("resolveModel(sessions-flat): %v", err)
		}

		if got, want := m.View().BackgroundColor, pinned.Canvas.Color(); got != want {
			t.Errorf("canvas = %v, want the pinned theme's %v", got, want)
		}
	})
}

// TestResolveProgram_ThemeDrivesBothBranches pins §13.3's "one flag, both
// branches": the tui.Build fixture path and the standalone contrast-validation
// swatch render the SAME resolved theme.
//
// They are deliberately different render paths — the swatch does not route
// through tui.Build, which is what lets it take a whole palette before the render
// layer does — so nothing else pins that one input reaches both. A wrong-variable
// slip at either call site is silent: the tool exits 0 having captured a frame of
// some other theme, at a gate whose entire job is judging colours the tests cannot.
func TestResolveProgram_ThemeDrivesBothBranches(t *testing.T) {
	path := themetest.Write(t, t.TempDir(), "mytheme.theme", themetest.WithValue(themetest.Lines(), "canvas", "#1a2b3c"))

	pinned, err := resolveTheme(theme.NewSilentLoader(), path, io.Discard)
	if err != nil {
		t.Fatalf("resolveTheme(%q): %v", path, err)
	}

	fixture, err := resolveProgram("sessions-flat", path, io.Discard)
	if err != nil {
		t.Fatalf("resolveProgram(sessions-flat, %q): %v", path, err)
	}
	if got, want := fixture.(tui.Model).View().BackgroundColor, pinned.Canvas.Color(); got != want {
		t.Errorf("the fixture canvas = %v, want the resolved theme's %v", got, want)
	}

	swatch, err := resolveProgram(capture.ContrastValidationFixture, path, io.Discard)
	if err != nil {
		t.Fatalf("resolveProgram(contrast-validation, %q): %v", path, err)
	}
	title := fmt.Sprintf("CONTRAST VALIDATION — canvas %s", pinned.Canvas.Value)
	if content := swatch.View().Content; !strings.Contains(content, title) {
		t.Errorf("the swatch does not render the resolved theme: no %q in its view\n--- view ---\n%s", title, content)
	}
}

// TestCaptureTool_ThemeResolutionIsSilent asserts resolving a theme writes
// NOTHING to the log — through either branch, and on failure as well as success.
//
// capturetool is §12.3's fifth caller and neither uses nor diagnoses a theme —
// it is an offline renderer whose output is a frame — so the loader is handed
// log.Discard and the `theme` component stays empty on this path. The stderr
// warnings are not log records either: they go to the injected writer. The
// positive control proves the assertion is not vacuous: a component logger
// emitting through the same swapped handler IS captured.
func TestCaptureTool_ThemeResolutionIsSilent(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)

	dir := t.TempDir()
	reserved := themetest.Write(t, dir, "nord.theme", themetest.Lines())
	broken := themetest.Write(t, dir, "broken.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))

	if _, err := resolveProgram(capture.ContrastValidationFixture, defaultThemeSlug, io.Discard); err != nil {
		t.Fatalf("resolveProgram(contrast-validation, %s): %v", defaultThemeSlug, err)
	}
	if _, err := resolveProgram(capture.ContrastValidationFixture, reserved, io.Discard); err != nil {
		t.Fatalf("resolveProgram(contrast-validation, %s): %v", reserved, err)
	}
	if _, err := resolveProgram(capture.ContrastValidationFixture, "not-a-theme", io.Discard); err == nil {
		t.Fatal("resolveProgram with an unknown theme returned nil error")
	}
	if _, err := resolveProgram(capture.ContrastValidationFixture, broken, io.Discard); err == nil {
		t.Fatal("resolveProgram with a broken theme file returned nil error")
	}

	if got := sink.Records(); len(got) != 0 {
		t.Errorf("theme resolution emitted %d log records, want 0: %+v", len(got), got)
	}

	theme.NewEventLogger(log.For("theme")).Rejected("some-theme", "", &theme.Rejection{Reason: theme.ReasonBadSyntax})
	if got := sink.Records(); len(got) != 1 {
		t.Fatalf("the positive control emitted %d records, want 1 — the sink is not wired to the theme component", len(got))
	}
}

// builtinForTest resolves one embedded built-in's palette, failing the test if it
// will not load.
func builtinForTest(t *testing.T, slug string) theme.Theme {
	t.Helper()

	result, rejection, found := theme.NewSilentLoader().LoadBuiltin(slug)
	if !found || rejection != nil {
		t.Fatalf("LoadBuiltin(%s) found=%v rejection=%v", slug, found, rejection)
	}
	return result.Theme
}

// pathThemeForTest stages a valid theme file carrying the given canvas value and
// returns the palette resolved from its PATH.
//
// The canvas is caller-chosen so an assertion against it cannot pass on a
// built-in's palette — including the dark built-in a model seeds itself with when
// no nomination is injected.
func pathThemeForTest(t *testing.T, canvas string) theme.Theme {
	t.Helper()

	path := themetest.Write(t, t.TempDir(), "mytheme.theme", themetest.WithValue(themetest.Lines(), "canvas", canvas))
	pinned, err := resolveTheme(theme.NewSilentLoader(), path, io.Discard)
	if err != nil {
		t.Fatalf("resolveTheme(%q): %v", path, err)
	}
	return pinned
}
