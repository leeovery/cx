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

const tokenCount = 19

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

func TestTokenCount_IsNineteen(t *testing.T) {
	if got := len(wantTokenNames); got != tokenCount {
		t.Fatalf("wantTokenNames holds %d names, want %d — the guard's own table is wrong", got, tokenCount)
	}
	if got := len(theme.Theme{}.All()); got != tokenCount {
		t.Errorf("len(Theme{}.All()) = %d, want %d (closed token vocabulary)", got, tokenCount)
	}
	if got := len(theme.TokenNames()); got != tokenCount {
		t.Errorf("len(TokenNames()) = %d, want %d (closed token vocabulary)", got, tokenCount)
	}
}

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

func TestTokenNames_MatchExactSet(t *testing.T) {
	if got := theme.TokenNames(); !slices.Equal(got, wantTokenNames) {
		t.Errorf("TokenNames() = %v, want %v", got, wantTokenNames)
	}
}

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

func TestTokenColor_ZeroValueDoesNotPanic(t *testing.T) {
	var zero theme.Token

	got := zero.Color()

	if want := lipgloss.Color(""); got != want {
		t.Errorf("Token{}.Color() = %v, want the no-colour sentinel %v", got, want)
	}
}

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
	"DirThemeSource.LoadSlot",
	"DirThemeSource.Open",
	"DirThemeSource.Reassemble",
	"DirThemeSource.Resolve",
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
	"Loader.OpenEnumeration",
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
	"SlugForSlot",
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

func seedValue(row int) string {
	return fmt.Sprintf("#0000%02d", row)
}

func exportedSymbols(t *testing.T) []string {
	t.Helper()

	symbols := []string{}
	for _, source := range parseThemeSources(t) {
		symbols = append(symbols, exportedDecls(source.File)...)
	}

	slices.Sort(symbols)
	return symbols
}

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
