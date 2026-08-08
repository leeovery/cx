// Package theme is the closed single-palette colour-token vocabulary Portal
// renders against, together with the machinery that turns a .theme file into
// one: the parser, the validator and the rejection ladder.
//
// The vocabulary is 19 semantic roles, and it is a public contract — every
// .theme file, built-in or dropped into the user's themes directory, is written
// against these exact names, so a rename breaks every file in that directory.
// One canonical ordered name↔field table backs the Theme struct, All() and
// TokenNames(), so the count, the names and the table order cannot drift apart.
//
// A Theme is ONE palette of 19 values, not a light/dark pair: light and dark are
// two different themes, expressed by the shape of the theme setting rather than
// by anything inside a theme. A Theme is therefore not a package-level built-in
// but the parse result of a theme file — an ordinary struct a test can construct
// directly — and it carries no identity field: the slug is held alongside the
// palette by whatever loaded it.
//
// The package is a leaf under cmd and internal/tui alike, because its consumers
// span two layers: TUI construction, the theme panel, portal doctor and portal
// theme export. Unlike internal/prefs it is not a no-logging leaf — it binds the
// theme log component.
package theme

import (
	"image/color"

	"charm.land/lipgloss/v2"
)

// Token is one semantic role in the closed vocabulary: the token name a theme
// file keys against, and the #RRGGBB value that file gave it.
type Token struct {
	Name, Value string
}

// Color resolves the token's value to a colour.
//
// It is an accessor rather than an inline lipgloss.Color(tok.Value) at each
// render call site so that a widening of the value domain lands in one place.
//
// A zero-value Token resolves through lipgloss.Color(""), which yields the
// no-colour sentinel rather than erroring, so a partly-populated hand-built
// Theme is safe to render.
func (t Token) Color() color.Color {
	return lipgloss.Color(t.Value)
}

// Theme is one palette: the 19 tokens of the closed vocabulary. Every field is a
// role, and the role meanings in the comments below are the public contract
// documented in docs/theming.md.
type Theme struct {
	// Text ramp, bright to faint.
	TextPrimary     Token // names, wordmark, active labels, modal titles, chip text
	TextSecondary   Token // selected-row meta, help actions, banner/signpost
	TextTertiary    Token // done-tick labels, selected-row path
	TextMuted       Token // paths, counts, footer labels, subtitles, group headings
	TextSubtle      Token // de-emphasised but still readable — group ··· N counts, pending loading steps
	TextFaint       Token // decorative only — inactive dots, + add, mode indicator, hints; never content a user must read
	TextOnSelection Token // name on the selected row (pairs against bg.selection)

	// Accents and states.
	AccentPrimary    Token // cursor, selector bar, active dot, ? key, focused field label, mode bar, loading bar
	AccentKey        Token // footer / modal key-hint glyphs
	AccentMode       Token // signals a distinct mode — Sessions header, Preview chrome, active tick
	AccentAttention  Token // filter query and /, edit-mode, warning ⚠
	StatePositive    Token // ● attached, Sessions count, Projects label, ✓ done, success flash
	StateDestructive Token // kill / delete emphasis, ▲

	// Surfaces.
	Canvas          Token // the owned mode-matched canvas, painted on every cell
	BgSelection     Token // selected-row tint
	BgAttention     Token // warning-flash band
	BgSubtle        Token // low neutral fill — loading-bar empty track
	Border          Token // title rule, footer rule, modal panel frames, edit-modal chips
	TextOnAttention Token // warning-flash message (pairs against bg.attention)
}

// fieldRef pairs a token name with the Theme field holding that role's value.
type fieldRef struct {
	Name  string
	Field *Token
}

// fields is the canonical ordered name↔field table: the 19 token roles in their
// documented order. It is the single source All(), TokenNames() and the parser's
// key→field assignment derive from, so there is deliberately no second list of
// token names anywhere in production code and the three cannot drift.
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

// All returns every token in the theme in the canonical table order.
//
// Each token's Name comes from the canonical table rather than from the stored
// field, so a Theme built by hand — one constructed without going through the
// loader — still enumerates under the right names when only its values were set.
func (t Theme) All() []Token {
	refs := t.fields()
	tokens := make([]Token, 0, len(refs))
	for _, ref := range refs {
		tokens = append(tokens, Token{Name: ref.Name, Value: ref.Field.Value})
	}
	return tokens
}

// TokenNames returns the 19 token names in the canonical table order. These are
// the keys a .theme file is written against, and renaming one breaks every
// existing theme file.
func TokenNames() []string {
	var t Theme
	refs := t.fields()
	names := make([]string, 0, len(refs))
	for _, ref := range refs {
		names = append(names, ref.Name)
	}
	return names
}
