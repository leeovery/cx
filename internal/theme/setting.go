package theme

import "cmp"

// RawKeys are prefs.json's three theme keys as they were READ — no defaults
// substituted, no charset check applied, no state collapsed.
//
// They exist alongside the resolved Setting because the resolution deliberately
// discards things the surfaces still need: a slot the user never set, a slug the
// tiebreak left unread, and a value no theme file answers to are all invisible
// in a Setting, and all three have to be reportable.
//
// THEY ARE CONTROL-STRIPPED, and stripped AT THE POINT THEY ARE READ rather than
// at the point they are drawn. That is what makes the removal a property
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
// Nothing else is normalised — no trimming, no lowercasing. A value that is
// merely wrong reaches the charset check as the user typed it, so it is reported
// as `bad name` rather than quietly resolved to a different theme.
type RawKeys struct {
	Theme string
	Light string
	Dark  string
}

// Setting is the theme setting: EXACTLY TWO STATES, derived from the three raw
// keys.
//
//   - CONSTANT — one slug, detection never consulted.
//   - ADAPTIVE — a light and a dark slug, with detection choosing between them.
//
// It holds SLUGS, never palettes. Nothing here inspects a theme's colours, and
// it could not: under the adaptive form THE SLOT CLASSIFIES THE THEME, so
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

// ResolveSetting collapses prefs.json's three raw theme keys onto the two-state
// setting, and returns the raw keys alongside it.
//
// THE TIEBREAK: a non-empty `theme` wins and THE SLOTS ARE NOT READ AT ALL. A
// hand-edited file may legally carry all three keys — mutual exclusion is
// enforced on write, not on the file — so the read side needs a deterministic
// answer, and the panel's guarantee that the two setting states never coexist on
// screen holds because THIS RULE makes the pair invisible, not because the file
// cannot contain both. The stale slots are left untouched on disk; nothing prunes
// them, which is why they are still returned in RawKeys.
//
// OTHERWISE THE PAIR, with a shipped default per unset slot. "Nothing set" and
// "pair nominated" are THE SAME STATE, so there is no unconfigured branch to take
// — only a default value per slot. Partial pairs therefore do not exist:
// `theme_dark = nord` alone resolves to {DefaultLightSlug, nord}, because light
// was never OVERRIDDEN rather than half-missing, and there is no incomplete-pair
// state for anything downstream to validate, explain or render around.
//
// The two defaults are DefaultLightSlug and DefaultDarkSlug — the same constants
// the per-slot fallback resolves to, never literals. That is what makes an
// unresolvable slot land on the theme the shipped default already nominates, so
// the pair degrades to the shipped defaults rather than to a different mechanism.
//
// The empty string is the unambiguous UNSET sentinel, which is what lets cmp.Or
// stand in for the whole rule: the anchored slug charset makes an empty slug
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
// or classifies a palette.
func ResolveSetting(theme, light, dark string) (Setting, RawKeys) {
	// Stripped FIRST, before anything is decided, so every rule below reads the
	// stripped form. A value that is ONLY control characters therefore strips to
	// empty and counts as UNSET rather than as an illegal slug: the stripped form
	// is "the value" for every consumer, and treating it as set would mint a
	// panel row labelled with an empty string.
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

// InForceKey is one persisted value Portal is actually reading, and where in the
// setting it sits.
//
// It is the value AS PERSISTED rather than a slug: nothing here has been
// validated, resolved or defaulted, because a value the charset check rejects is
// still in force — it is what the user set, and therefore what a surface has to
// report back to them.
type InForceKey struct {
	// Value is the persisted value itself, control-stripped. It may be no legal
	// slug at all.
	Value string

	// Slot is the position the value occupies: SlotConstant under a constant, else
	// the pair's light or dark slot — and the LIGHT one where Both is set, since a
	// collapsed key occupies that slot as well as the other.
	Slot Slot

	// Both reports that this ONE value occupies BOTH slots of the pair.
	//
	// It is a flag rather than a third Slot value because the setting has exactly
	// two slots: a third would name a position that does not exist, and every
	// surface reading a slot's own name would then have to special-case it.
	Both bool
}

// InForceKeys selects which of prefs.json's three keys a surface reports on: THE
// KEYS IN FORCE, never every key present.
//
// THE TIEBREAK IS ResolveSetting's, applied here rather than restated: a
// non-empty `theme` wins and the slots are not read at all. A hand-edited file
// may legally carry all three keys, and reporting two values Portal is not
// reading would put the user to work fixing something with no effect. The raw
// keys are resolved HERE rather than taken pre-resolved so a caller cannot skip
// that step; handing back keys ResolveSetting already produced is safe, because
// stripping is idempotent and the resolution is pure and total.
//
// THE SETTING AND THE RAW KEYS ARE BOTH READ, AND NEITHER SUBSTITUTES FOR THE
// OTHER. The Setting says which state the tiebreak settled on; the raw keys say
// which values are actually PERSISTED. So under a pair only the slots with a
// NON-EMPTY RAW VALUE are in force: an unset slot arrives in the Setting as the
// shipped default, which is a built-in that always resolves, and the raw value is
// what distinguishes "the user chose this" from "Portal substituted it".
//
// TWO SLOTS NAMING THE SAME VALUE COLLAPSE TO ONE ENTRY, keyed on the persisted
// VALUE rather than on a derived slug, so a value yielding no slug at all
// collapses by the same rule. One value the user set is one problem, however many
// slots it sits in.
//
// The order is light then dark, and a constant is the single entry.
func InForceKeys(keys RawKeys) []InForceKey {
	setting, raw := ResolveSetting(keys.Theme, keys.Light, keys.Dark)
	if setting.IsConstant {
		return []InForceKey{{Value: setting.Constant, Slot: SlotConstant}}
	}

	if raw.Light != "" && raw.Light == raw.Dark {
		return []InForceKey{{Value: raw.Light, Slot: SlotLight, Both: true}}
	}

	var inForce []InForceKey
	if raw.Light != "" {
		inForce = append(inForce, InForceKey{Value: raw.Light, Slot: SlotLight})
	}
	if raw.Dark != "" {
		inForce = append(inForce, InForceKey{Value: raw.Dark, Slot: SlotDark})
	}
	return inForce
}
