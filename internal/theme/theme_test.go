package theme_test

import (
	"fmt"
	"go/ast"
	"image/color"
	"reflect"
	"slices"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// tokenCount is the size of the closed vocabulary. It is stated here as a
// literal rather than derived from anything in the package so growing the
// vocabulary is a deliberate, spec-amending act.
const tokenCount = 19

// wantTokenNames is the canonical rename table, rows 1 through 19, in table order.
//
// Production derives every name from internal/theme's single canonical
// name↔field table, so this slice is the ONE deliberate restatement of the
// vocabulary: renaming a token has to be done twice, here and there. That is the
// point — these names are the public contract every .theme file is written
// against, and a rename breaks every file in a user's themes directory.
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

// TestTokenCount_IsNineteen pins the closed vocabulary at 19 tokens, the
// count the 20-token TestMVTokenCount is retired into. Both accessors
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
// position 1 is text.primary and position 19 is text.on-attention (the canonical
// table's numbering IS the definition of All()'s stable order).
//
// Each field is seeded with a distinct value in canonical table row order, so the
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

// TestTokenNames_MatchExactSet pins the exact 19 names in the exact canonical table order —
// not alphabetical, not grouped by kind, not whatever the struct's textual order
// happens to be.
func TestTokenNames_MatchExactSet(t *testing.T) {
	if got := theme.TokenNames(); !slices.Equal(got, wantTokenNames) {
		t.Errorf("TokenNames() = %v, want %v", got, wantTokenNames)
	}
}

// TestAll_CoversEveryStructField cross-checks the canonical table against the
// struct by reflection, so a field added to Theme without a table entry fails
// the suite rather than being silently invisible to All() (and to the parser).
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
// hand with only some tokens populated (the completeness guard's synthetic themes).
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
// than anything inside a theme.
//
// Later tasks legitimately extend this list — the lexer added Pair and the
// reason vocabulary, the name rules added ValidSlug, SlugFromFilename and the
// bad-name causes, the reason vocabulary ladder added Loader, Result and LoadFile,
// enumeration added Entry and Enumerate, the `theme` log component added the
// EventLogger seam with the NewLoader that injects it, the embedded built-in
// set added BuiltinBytes, BuiltinSlugs and LoadBuiltin, the build-time guarantee's
// guarantee added DefaultDarkSlug and DefaultLightSlug, the construction-time load rule's
// loaded nomination added Nomination with its two constructors and three accessors, the
// harness contract's explicit-path input added LoadPath alongside LoadFile, the
// constant-or-pair rule's two-state setting added ResolveSetting with the Setting and RawKeys
// it returns, the per-slot fallback rule's per-slot fallback added ResolveNomination with the
// Resolution, Slot and SlotResolution it reports through, the `theme` log component's
// per-theme records added EventLogger.Loaded and EventLogger.FallbackApplied beside the two
// rejection events, the build-time guarantee's runtime escalation added BrokenBuiltinError,
// the panel's union added the Assembler's Open and Reassemble with the Enumeration,
// Union, Row and RowSource they deal in plus the EventLogger.Enumerated they report through,
// and single-sourcing the light/dark vocabulary added Slot.AttrName — the one
// definition of those two words, which the persister and doctor read across the
// package boundary rather than restating, and single-sourcing the silenced
// loader added NewSilentLoader beside NewLoader — the diagnose-shaped
// callers assembled that seam themselves until it had one definition here, and
// taking the raw keys as a value added NewRawKeys — which control-strips the
// three keys as it builds them, so ResolveSetting takes that value rather than
// three interchangeable strings.
// Nothing on the removed list ever returns to it.
//
// InForceKeys and the InForceKey it yields are exported for a REASON RATHER THAN
// FOR A CALLER'S CONVENIENCE: which
// persisted keys Portal is actually reading — the constant-or-pair rule's tiebreak, then the
// slots the user really set, then two slots naming one value collapsed — is one rule
// that the union rule's panel rows and the pinned copy's doctor lines must answer identically,
// and it was authored twice, once on each side of this package boundary, until it was
// exported. The Slot they carry is not a light/dark variant surface returning
// either: it names the POSITION a value occupies in the setting.
//
// RawKeys.WithConstant and RawKeys.WithMember are exported for the same kind of
// reason InForceKeys is: the setting's mutual exclusion — a constant clears both
// slots, a slot clears the constant and carries the other half across — is one
// rule that prefs performs on the FILE and the theme panel must restate on the
// KEYS IT HOLDS, since prefs is a leaf that cannot import this package and so
// cannot share the mutator itself. Exporting the transformation gives the
// in-memory half one home instead of one per commit handler. The half is named by
// a Member rather than a Slot so the pair's two halves are the whole of the
// argument's domain, matching what the write side will accept.
//
// FileExtension is exported for the same kind of reason: the extension is a
// published part of the theme-file contract rather than a private parsing
// detail, and the surfaces that compose or recognise a theme filename outside
// this package restated it verbatim until it had one home.
//
// Badge.Text is a DEPARTURE rather than an addition: the badge derivation stays
// here, because which badge a slug carries is a fact about the setting, but the
// words it is drawn with are the theme panel's copy and live with the panel's
// other pinned strings.
//
// The three-way split of the loader's old surface is pinned here rather than
// merely done: the row-model assembly is Assembler's, the panel seam's four
// methods are DirThemeSource's, and what is left on Loader parses, enumerates and
// resolves. LoadPath is a FUNCTION because neither the reserved set nor the event
// seam bears on a path a caller named itself — a method would take a receiver it
// cannot read.
//
// BrokenBuiltinError is exported for one reason and it is not a caller's: the pinned copy's
// fatal sentence is pinned copy, so the test that pins it must be able to state
// it independently and compare. Nothing constructs one but resolveSlot.
//
// Slot is the second addition that could be mistaken for the removed surface
// returning: it names a light and a dark position. It is not — a slot is a
// position in the SETTING that a whole theme is nominated for, and the no-variant rule
// still holds in full: the slot classifies the theme, and no theme declares a
// variant of itself.
//
// Member is the same kind of addition and takes the same distinction, more
// sharply because it has exactly two values named light and dark: it is the
// light/dark ANSWER, naming which member of an adaptive PAIR the gate picks. No
// Theme declares one, carries one or resolves through one, and there is no
// per-token variant anywhere beneath it.
//
// Nomination's arrival is the one addition that could be mistaken for the
// removed surface returning: it holds a light and a dark Theme. It is not — the
// pairing is the shape of the SETTING, held outside any theme, and neither
// member is a variant of the other. Setting's light and dark fields
// are the same shape one level earlier and are not variants either: they are
// SLUGS, and the slot classifies the theme rather than the theme declaring a
// variant.
//
// The exact-equality check is also what pins the negative half of the build-time guarantee:
// there is no exported eager-validation helper, because validation is not
// startup-eager and no caller is offered a "check the built-ins now" entry
// point to put one on a cold path.
var wantExports = []string{
	"AdaptivePair",
	"Assembler",
	"Assembler.Open",
	"Assembler.Reassemble",
	"BadNameCause",
	"BadNameExtension",
	"BadNameNone",
	"BadNameSlug",
	"Badge",
	"BadgeBoth",
	"BadgeConstant",
	"BadgeDark",
	"BadgeLight",
	"BadgeNone",
	"Badges",
	"BrokenBuiltinError",
	"BuiltinBytes",
	"BuiltinSlugs",
	"ConstantNomination",
	"DefaultDarkSlug",
	"DefaultLightSlug",
	"DirThemeSource",
	"DirThemeSource.Open",
	"DirThemeSource.Reassemble",
	"DirThemeSource.Resolve",
	"DirThemeSource.ResolveSlot",
	"Entry",
	"Enumeration",
	"EventLogger",
	"EventLogger.DirectoryUnusable",
	"EventLogger.Enumerated",
	"EventLogger.FallbackApplied",
	"EventLogger.Loaded",
	"EventLogger.Rejected",
	"FileExtension",
	"InForceKey",
	"InForceKeys",
	"LoadPath",
	"Loader",
	"Loader.Enumerate",
	"Loader.LoadBuiltin",
	"Loader.LoadFile",
	"Loader.ResolveByName",
	"Loader.ResolveByNameFrom",
	"Loader.ResolveNomination",
	"Loader.ResolveNominationFrom",
	"Loader.ResolveSlot",
	"Member",
	"Member.Opposite",
	"Member.Palette",
	"Member.Slot",
	"MemberDark",
	"MemberLight",
	"MemberPalette",
	"NewEventLogger",
	"NewLoader",
	"NewRawKeys",
	"NewSilentLoader",
	"Nomination",
	"Nomination.Constant",
	"Nomination.IsConstant",
	"Nomination.Select",
	"Pair",
	"RawKeys",
	"RawKeys.WithConstant",
	"RawKeys.WithMember",
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
	"Resolution",
	"ResolveSetting",
	"Result",
	"Row",
	"Row.BadgeKey",
	"Row.Identity",
	"Row.Label",
	"Row.Selectable",
	"Row.SortKey",
	"RowSource",
	"Setting",
	"Setting.Slug",
	"Slot",
	"Slot.AttrName",
	"SlotConstant",
	"SlotDark",
	"SlotLight",
	"SlotResolution",
	"SlugFromFilename",
	"SourceBuiltin",
	"SourceFile",
	"SourcePersisted",
	"StripControl",
	"Theme",
	"Theme.All",
	"Token",
	"Token.Color",
	"TokenNames",
	"Union",
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

// seedValue renders a distinct, well-formed hex for the given canonical table row number.
func seedValue(row int) string {
	return fmt.Sprintf("#0000%02d", row)
}

// exportedSymbols returns every exported top-level identifier the package's
// non-test sources declare, sorted. Methods are reported as "Receiver.Method".
func exportedSymbols(t *testing.T) []string {
	t.Helper()

	symbols := []string{}
	for _, source := range parseThemeSources(t) {
		symbols = append(symbols, exportedDecls(source.File)...)
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
