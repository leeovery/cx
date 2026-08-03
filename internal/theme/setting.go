package theme

import "cmp"

// RawKeys are prefs.json's three theme keys as they were READ (§8.1) — no
// defaults substituted, no charset check applied, no state collapsed.
//
// They exist alongside the resolved Setting because the resolution deliberately
// discards things the surfaces still need: a slot the user never set, a slug the
// tiebreak left unread, and a value no theme file answers to are all invisible
// in a Setting, and all three have to be reportable.
//
// THEY ARE CONTROL-STRIPPED, and stripped AT THE POINT THEY ARE READ rather than
// at the point they are drawn (§9.5). That is what makes the removal a property
// of the VALUE, inherited by every consumer alike — the panel row and doctor's
// advisory line both get a value that renders as one line of ordinary text,
// rather than each remembering to sanitise. A pasted newline, tab or ANSI escape
// would otherwise corrupt whichever of those two surfaces the user is reading in
// order to FIND the problem.
//
// TRUNCATION IS SEPARATE AND STAYS PANEL-LOCAL: it is a geometry concern of one
// surface, and doctor has full width and wants the whole value. Nothing here
// shortens anything.
//
// Nothing else is normalised (§5.2) — no trimming, no lowercasing. A value that
// is merely wrong reaches the charset check as the user typed it, so it is
// reported as `bad name` rather than quietly resolved to a different theme.
type RawKeys struct {
	Theme string
	Light string
	Dark  string
}

// Setting is §8.2's theme setting: EXACTLY TWO STATES, derived from the three
// raw keys.
//
//   - CONSTANT — one slug, detection never consulted.
//   - ADAPTIVE — a light and a dark slug, with detection choosing between them.
//
// It holds SLUGS, never palettes. Nothing here inspects a theme's colours, and
// it could not: under the adaptive form THE SLOT CLASSIFIES THE THEME (§4.7), so
// "light" is a statement about when a theme is used rather than about what is in
// it.
type Setting struct {
	// IsConstant is which of the two states this is: Constant is non-empty iff
	// it is true, and Light and Dark are both non-empty iff it is false.
	IsConstant bool

	// Constant is the constant state's single slug.
	Constant string

	// Light and Dark are the adaptive state's two slugs.
	Light string
	Dark  string
}

// ResolveSetting collapses prefs.json's three raw theme keys onto §8.2's
// two-state setting, and returns the raw keys alongside it.
//
// THE TIEBREAK: a non-empty `theme` wins and THE SLOTS ARE NOT READ AT ALL. A
// hand-edited file may legally carry all three keys — mutual exclusion is
// enforced on write, not on the file — so the read side needs a deterministic
// answer, and §9.5's "the two setting states never coexist on screen" holds
// because THIS RULE makes the pair invisible, not because the file cannot
// contain both. The stale slots are left untouched on disk; nothing prunes them,
// which is why they are still returned in RawKeys.
//
// OTHERWISE THE PAIR, with a shipped default per unset slot. "Nothing set" and
// "pair nominated" are THE SAME STATE (§8.3), so there is no unconfigured branch
// to take — only a default value per slot. Partial pairs therefore do not exist:
// `theme_dark = nord` alone resolves to {DefaultLightSlug, nord}, because light
// was never OVERRIDDEN rather than half-missing, and there is no incomplete-pair
// state for anything downstream to validate, explain or render around.
//
// The two defaults are DefaultLightSlug and DefaultDarkSlug — the same constants
// task 5-4's per-slot fallback resolves to, never literals. That coincidence is
// what §8.3's "the adaptive pair degrades to a constant dark default" argument
// rests on: an unresolvable slot lands on the theme the shipped default already
// nominates, so the pair is a superset of a constant dark default rather than a
// different mechanism.
//
// The empty string is the unambiguous UNSET sentinel, which is what lets cmp.Or
// stand in for the whole rule: §5.2's anchored charset makes an empty slug
// illegal, so an empty value can never be a theme's real name and needs no
// separate "was it set?" flag to disambiguate it.
//
// The Setting and the RawKeys come from ONE evaluation on purpose: what a
// surface later LISTS and what it MARKS both derive from this call, so they
// cannot disagree about which slug a badge sits on.
//
// IT IS PURE AND TOTAL — no file, no environment, no clock, no logging and no
// error return, and the same triple always answers the same way. There is
// nothing to fail: every input is a legal input, including a nonsense one, which
// is why an unrecognised value is a RESOLUTION problem for a later step rather
// than a decode one here. It also deals in SLUGS ONLY and never loads, inspects
// or classifies a palette (§4.7).
func ResolveSetting(theme, light, dark string) (Setting, RawKeys) {
	// Stripped FIRST, before anything is decided, so every rule below reads the
	// stripped form. That ordering resolves an ambiguity the spec leaves open —
	// it pins stripping and it pins "a non-empty `theme` wins", but not which
	// happens first for a value that is ONLY control characters. Such a value
	// strips to empty and is therefore UNSET rather than an illegal slug,
	// because §9.5 makes the stripped form "the value" for every consumer, and
	// because the alternative would mint a panel row labelled with an empty
	// string. A later phase revisiting this should do so deliberately.
	raw := RawKeys{
		Theme: StripControl(theme),
		Light: StripControl(light),
		Dark:  StripControl(dark),
	}

	if raw.Theme != "" {
		return Setting{IsConstant: true, Constant: raw.Theme}, raw
	}

	return Setting{
		Light: cmp.Or(raw.Light, DefaultLightSlug),
		Dark:  cmp.Or(raw.Dark, DefaultDarkSlug),
	}, raw
}
