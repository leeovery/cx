// Package theme is the closed colour-token vocabulary Portal renders against,
// plus the machinery that turns a .theme file into one. The 19 token names are
// a public contract — renaming one breaks every drop-in theme file. A Theme is
// one palette, not a light/dark pair (light and dark are two different
// themes), and is the parse result of a file, carrying no identity field.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Token is one semantic role in the closed vocabulary: the name a theme file
// keys against, and the #RRGGBB value that file gave it.
type Token struct {
	Name, Value string
}

// Color resolves the token's value to a colour. A zero-value Token yields the
// lipgloss no-colour sentinel rather than erroring, so a partly-populated
// hand-built Theme is safe to render.
func (t Token) Color() color.Color {
	return lipgloss.Color(t.Value)
}

// Theme is one palette: the 19 tokens of the closed vocabulary.
type Theme struct {
	// Text ramp, brightest to faintest. TextFaint is decorative only —
	// never content a user must read.
	TextPrimary     Token
	TextSecondary   Token
	TextTertiary    Token
	TextMuted       Token
	TextSubtle      Token
	TextFaint       Token
	TextOnSelection Token // pairs against BgSelection

	// Accents and states.
	AccentPrimary    Token
	AccentKey        Token
	AccentMode       Token
	AccentAttention  Token
	StatePositive    Token
	StateDestructive Token

	// Surfaces.
	Canvas          Token // the owned canvas, painted on every cell
	BgSelection     Token
	BgAttention     Token
	BgSubtle        Token
	Border          Token
	TextOnAttention Token // pairs against BgAttention
}

type fieldRef struct {
	Name  string
	Field *Token
}

// fields is the canonical name↔field table — the single source All(),
// TokenNames() and the parser derive from, so count, names and order cannot
// drift.
func (t *Theme) fields() []fieldRef {
	return []fieldRef{
		{Name: "text.primary", Field: &t.TextPrimary},
		{Name: "text.secondary", Field: &t.TextSecondary},
		{Name: "text.tertiary", Field: &t.TextTertiary},
		{Name: "text.muted", Field: &t.TextMuted},
		{Name: "text.subtle", Field: &t.TextSubtle},
		{Name: "text.faint", Field: &t.TextFaint},
		{Name: "text.on-selection", Field: &t.TextOnSelection},
		{Name: "accent.primary", Field: &t.AccentPrimary},
		{Name: "accent.key", Field: &t.AccentKey},
		{Name: "accent.mode", Field: &t.AccentMode},
		{Name: "accent.attention", Field: &t.AccentAttention},
		{Name: "state.positive", Field: &t.StatePositive},
		{Name: "state.destructive", Field: &t.StateDestructive},
		{Name: "canvas", Field: &t.Canvas},
		{Name: "bg.selection", Field: &t.BgSelection},
		{Name: "bg.attention", Field: &t.BgAttention},
		{Name: "bg.subtle", Field: &t.BgSubtle},
		{Name: "border", Field: &t.Border},
		{Name: "text.on-attention", Field: &t.TextOnAttention},
	}
}

// All returns every token in canonical table order. Names come from the
// table, not the stored fields, so a hand-built Theme with only values set
// still enumerates under the right names.
func (t Theme) All() []Token {
	refs := t.fields()
	tokens := make([]Token, 0, len(refs))
	for _, ref := range refs {
		tokens = append(tokens, Token{Name: ref.Name, Value: ref.Field.Value})
	}
	return tokens
}

// TokenNames returns the 19 token names in canonical table order — the keys a
// .theme file is written against; renaming one breaks every existing file.
func TokenNames() []string {
	var t Theme
	refs := t.fields()
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	return names
}
