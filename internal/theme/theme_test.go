package theme_test

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"image/color"
	"os"
	"reflect"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// tokenCount is the size of the closed vocabulary (§2.1). It is stated here as a
// literal rather than derived from anything in the package so growing the
// vocabulary is a deliberate, spec-amending act.
const tokenCount = 19

// wantTokenNames is the §2.4 rename table, rows 1 through 19, in table order.
//
// Production derives every name from internal/theme's single canonical
// name↔field table, so this slice is the ONE deliberate restatement of the
// vocabulary: renaming a token has to be done twice, here and there. That is the
// point — these names are the public contract every .theme file is written
// against (§1.3), and a rename breaks every file in a user's themes directory.
var wantTokenNames = []string{
	"text.primary",
	"text.secondary",
	"text.tertiary",
	"text.muted",
	"text.subtle",
	"text.faint",
	"text.on-selection",
	"accent.primary",
	"accent.key",
	"accent.mode",
	"accent.attention",
	"state.positive",
	"state.destructive",
	"canvas",
	"bg.selection",
	"bg.attention",
	"bg.subtle",
	"border",
	"text.on-attention",
}

// TestTokenCount_IsNineteen pins the closed vocabulary at 19 tokens (§2.1), the
// count the 20-token TestMVTokenCount is retired into (§13.6). Both accessors
// are checked so neither can grow without the other.
func TestTokenCount_IsNineteen(t *testing.T) {
	if got := len(wantTokenNames); got != tokenCount {
		t.Fatalf("wantTokenNames holds %d names, want %d — the guard's own table is wrong", got, tokenCount)
	}
	if got := len(theme.Theme{}.All()); got != tokenCount {
		t.Errorf("len(Theme{}.All()) = %d, want %d (closed §2.4 vocabulary)", got, tokenCount)
	}
	if got := len(theme.TokenNames()); got != tokenCount {
		t.Errorf("len(TokenNames()) = %d, want %d (closed §2.4 vocabulary)", got, tokenCount)
	}
}

// TestAll_ReturnsSpecTableOrder asserts the whole ordered slice, not a set:
// position 1 is text.primary and position 19 is text.on-attention (§3.2 — the
// §2.4 numbering IS the definition of All()'s stable order).
//
// Each field is seeded with a distinct value in §2.4 row order, so the
// assertion also pins the name↔field pairing: an entry in the canonical table
// wired to the wrong field, a duplicated field or a reordered one all surface as
// a value mismatch rather than passing silently.
func TestAll_ReturnsSpecTableOrder(t *testing.T) {
	seeded := theme.Theme{
		TextPrimary:      theme.Token{Value: seedValue(1)},
		TextSecondary:    theme.Token{Value: seedValue(2)},
		TextTertiary:     theme.Token{Value: seedValue(3)},
		TextMuted:        theme.Token{Value: seedValue(4)},
		TextSubtle:       theme.Token{Value: seedValue(5)},
		TextFaint:        theme.Token{Value: seedValue(6)},
		TextOnSelection:  theme.Token{Value: seedValue(7)},
		AccentPrimary:    theme.Token{Value: seedValue(8)},
		AccentKey:        theme.Token{Value: seedValue(9)},
		AccentMode:       theme.Token{Value: seedValue(10)},
		AccentAttention:  theme.Token{Value: seedValue(11)},
		StatePositive:    theme.Token{Value: seedValue(12)},
		StateDestructive: theme.Token{Value: seedValue(13)},
		Canvas:           theme.Token{Value: seedValue(14)},
		BgSelection:      theme.Token{Value: seedValue(15)},
		BgAttention:      theme.Token{Value: seedValue(16)},
		BgSubtle:         theme.Token{Value: seedValue(17)},
		Border:           theme.Token{Value: seedValue(18)},
		TextOnAttention:  theme.Token{Value: seedValue(19)},
	}

	got := seeded.All()
	if len(got) != tokenCount {
		t.Fatalf("len(All()) = %d, want %d", len(got), tokenCount)
	}

	for i, tok := range got {
		want := theme.Token{Name: wantTokenNames[i], Value: seedValue(i + 1)}
		if tok != want {
			t.Errorf("All()[%d] = %+v, want %+v", i, tok, want)
		}
	}
}

// TestTokenNames_MatchExactSet pins the exact 19 names in the exact §2.4 order —
// not alphabetical, not grouped by kind, not whatever the struct's textual order
// happens to be.
func TestTokenNames_MatchExactSet(t *testing.T) {
	if got := theme.TokenNames(); !slices.Equal(got, wantTokenNames) {
		t.Errorf("TokenNames() = %v, want %v", got, wantTokenNames)
	}
}

// TestAll_CoversEveryStructField cross-checks the canonical table against the
// struct by reflection, so a field added to Theme without a table entry fails
// the suite rather than being silently invisible to All() (and, from task 1.3
// on, to the parser).
func TestAll_CoversEveryStructField(t *testing.T) {
	themeType := reflect.TypeFor[theme.Theme]()
	tokenType := reflect.TypeFor[theme.Token]()

	if got, want := themeType.NumField(), len(theme.Theme{}.All()); got != want {
		t.Errorf("Theme has %d fields but All() returns %d tokens — the canonical table and the struct disagree", got, want)
	}

	for field := range themeType.Fields() {
		if field.Type != tokenType {
			t.Errorf("Theme.%s is %s, want %s — every field is a token", field.Name, field.Type, tokenType)
		}
	}
}

// TestTokenColor_ZeroValueDoesNotPanic pins that an unset token is safe to
// render. lipgloss.Color("") yields the no-colour sentinel rather than erroring,
// which matters because a Theme is an ordinary struct a test may construct by
// hand with only some tokens populated (§3.2, §13.4's synthetic guard themes).
func TestTokenColor_ZeroValueDoesNotPanic(t *testing.T) {
	var zero theme.Token

	got := zero.Color()

	if want := lipgloss.Color(""); got != want {
		t.Errorf("Token{}.Color() = %v, want the no-colour sentinel %v", got, want)
	}
}

// TestTokenColor_ResolvesHexThroughLipgloss asserts the accessor resolves a
// #RRGGBB value to the colour that hex names — checked as RGBA components so the
// assertion is not merely a restatement of the implementation.
func TestTokenColor_ResolvesHexThroughLipgloss(t *testing.T) {
	tok := theme.Token{Name: "text.primary", Value: "#C0CAF5"}

	got := tok.Color()

	if want := lipgloss.Color("#C0CAF5"); got != want {
		t.Errorf("Color() = %v, want %v", got, want)
	}

	rgba, ok := color.RGBAModel.Convert(got).(color.RGBA)
	if !ok {
		t.Fatalf("Color() did not convert to color.RGBA, got %T", color.RGBAModel.Convert(got))
	}
	if want := (color.RGBA{R: 0xC0, G: 0xCA, B: 0xF5, A: 0xFF}); rgba != want {
		t.Errorf("Color() as RGBA = %+v, want %+v", rgba, want)
	}
}

// wantExports is the package's entire exported surface. Absent from it, and
// deliberately so: Mode, ColorFor, MV, and any Light/Dark variant surface — a
// theme is one palette, and light/dark is the shape of the theme setting rather
// than anything inside a theme (§1.2, §3.1, §3.2).
//
// Later tasks legitimately extend this list — the lexer added Pair and the
// reason vocabulary, the name rules added ValidSlug, SlugFromFilename and the
// bad-name causes, the §6.2 ladder added Loader, Result and LoadFile,
// enumeration added Entry and Enumerate, the `theme` log component added the
// EventLogger seam with the NewLoader that injects it, the embedded built-in
// set added BuiltinBytes, BuiltinSlugs and LoadBuiltin, and §7.6's build-time
// guarantee added DefaultDarkSlug and DefaultLightSlug. Nothing on the removed
// list ever returns to it.
//
// The exact-equality check is also what pins the negative half of §7.6: there
// is no exported eager-validation helper, because validation is not
// startup-eager and no caller is offered a "check the built-ins now" entry
// point to put one on a cold path.
var wantExports = []string{
	"BadNameCause",
	"BadNameExtension",
	"BadNameNone",
	"BadNameSlug",
	"BuiltinBytes",
	"BuiltinSlugs",
	"DefaultDarkSlug",
	"DefaultLightSlug",
	"Entry",
	"EventLogger",
	"EventLogger.DirectoryUnusable",
	"EventLogger.Rejected",
	"Loader",
	"Loader.Enumerate",
	"Loader.LoadBuiltin",
	"Loader.LoadFile",
	"NewEventLogger",
	"NewLoader",
	"Pair",
	"Reason",
	"ReasonBadColour",
	"ReasonBadName",
	"ReasonBadSyntax",
	"ReasonMissingTokens",
	"ReasonNotFound",
	"ReasonReservedName",
	"ReasonUnreadable",
	"Rejection",
	"Rejection.Error",
	"Result",
	"SlugFromFilename",
	"Theme",
	"Theme.All",
	"Token",
	"Token.Color",
	"TokenNames",
	"ValidSlug",
}

// TestVocabulary_HasNoModeSurface asserts the package exports exactly
// wantExports and that a Token carries exactly {Name, Value} — the two places a
// resurrected light/dark mode surface could hide.
func TestVocabulary_HasNoModeSurface(t *testing.T) {
	if got := exportedSymbols(t); !slices.Equal(got, wantExports) {
		t.Errorf("exported surface = %v, want %v", got, wantExports)
	}

	tokenType := reflect.TypeFor[theme.Token]()
	gotFields := make([]string, 0, tokenType.NumField())
	for field := range tokenType.Fields() {
		gotFields = append(gotFields, field.Name)
	}
	if want := []string{"Name", "Value"}; !slices.Equal(gotFields, want) {
		t.Errorf("Token fields = %v, want %v", gotFields, want)
	}
}

// seedValue renders a distinct, well-formed hex for the given §2.4 row number.
func seedValue(row int) string {
	return fmt.Sprintf("#0000%02d", row)
}

// exportedSymbols parses the package's non-test sources and returns every
// exported top-level identifier, sorted. Methods are reported as
// "Receiver.Method". go test runs the binary in the package's source directory,
// so the relative walk resolves regardless of where the suite was invoked from.
func exportedSymbols(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read package dir: %v", err)
	}

	fset := token.NewFileSet()
	symbols := []string{}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}

		file, err := parser.ParseFile(fset, name, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}
		symbols = append(symbols, exportedDecls(file)...)
	}

	slices.Sort(symbols)
	return symbols
}

// exportedDecls collects the exported top-level identifiers declared in one file.
func exportedDecls(file *ast.File) []string {
	symbols := []string{}
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if !d.Name.IsExported() {
				continue
			}
			if d.Recv == nil {
				symbols = append(symbols, d.Name.Name)
				continue
			}
			symbols = append(symbols, receiverName(d.Recv)+"."+d.Name.Name)
		case *ast.GenDecl:
			symbols = append(symbols, exportedSpecs(d.Specs)...)
		}
	}
	return symbols
}

// exportedSpecs collects the exported type, var and const names in a declaration
// group.
func exportedSpecs(specs []ast.Spec) []string {
	symbols := []string{}
	for _, spec := range specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			if s.Name.IsExported() {
				symbols = append(symbols, s.Name.Name)
			}
		case *ast.ValueSpec:
			for _, ident := range s.Names {
				if ident.IsExported() {
					symbols = append(symbols, ident.Name)
				}
			}
		}
	}
	return symbols
}

// receiverName returns the bare type name of a method receiver, with any pointer
// star stripped.
func receiverName(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}

	expr := recv.List[0].Type
	if star, ok := expr.(*ast.StarExpr); ok {
		expr = star.X
	}
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}
