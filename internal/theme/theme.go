// Package theme is the closed colour-token vocabulary Portal renders against,
// plus the machinery that turns a .theme file into one. The token names are a
// public contract — renaming one breaks every existing drop-in file. A Theme is
// one palette and is itself light or dark; light and dark are two themes.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Token is one semantic role: the name a theme file keys against, and the
// #RRGGBB value that file gave it.
type Token struct {
	Name, Value string
}

// Color resolves the token's value. A zero-value Token yields the lipgloss
// no-colour sentinel rather than erroring, so a partly-populated hand-built
// Theme is safe to render.
func (t Token) Color() color.Color {
	return lipgloss.Color(t.Value)
}

type Theme struct {
	// TextFaint is decorative only — never content a user must read.
	TextPrimary     Token
	TextSecondary   Token
	TextTertiary    Token
	TextMuted       Token
	TextSubtle      Token
	TextFaint       Token
	TextOnSelection Token

	AccentPrimary    Token
	AccentKey        Token
	AccentMode       Token
	AccentAttention  Token
	StatePositive    Token
	StateDestructive Token

	Canvas          Token // painted on every cell, so every other token is read against it
	BgSelection     Token
	BgAttention     Token
	BgSubtle        Token
	Border          Token
	TextOnAttention Token
}

type fieldRef struct {
	Name  string
	Field *Token
}

// Names, order and count all derive from this table rather than from the struct,
// so a token added as a field but not as a row here is never parsed.
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

// All returns every token in table order. Names come from the table, not the
// stored fields, so a hand-built Theme with only values set still enumerates
// under the right names.
func (t Theme) All() []Token {
	refs := t.fields()
	tokens := make([]Token, 0, len(refs))
	for _, ref := range refs {
		tokens = append(tokens, Token{Name: ref.Name, Value: ref.Field.Value})
	}
	return tokens
}

// TokenNames returns the token names in table order — the keys a .theme file is
// written against.
func TokenNames() []string {
	var t Theme
	refs := t.fields()
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	return names
}
