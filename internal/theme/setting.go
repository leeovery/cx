package theme

import "cmp"

// RawKeys are prefs.json's three theme keys as they were read — no defaults
// substituted, no charset check applied, no state collapsed.
//
// They exist alongside the resolved Setting because the resolution discards
// things the surfaces still need: a slot the user never set, a slug the tiebreak
// left unread, and a value no theme file answers to are all invisible in a
// Setting, and all three have to be reportable.
//
// They are control-stripped at the point they are read rather than at the point
// they are drawn, which makes the removal a property of the value inherited by
// every consumer alike. A pasted newline, tab or ANSI escape would otherwise
// corrupt whichever surface the user is reading in order to find the problem.
//
// Truncation is separate and stays panel-local: it is a geometry concern of one
// surface, and doctor has full width and wants the whole value.
//
// Nothing else is normalised — no trimming, no lowercasing. A value that is
// merely wrong reaches the charset check as the user typed it, so it is reported
// as `bad name` rather than quietly resolved to a different theme.
type RawKeys struct {
	Theme string
	Light string
	Dark  string
}

// WithConstant is these keys with slug as the constant theme and both slots
// cleared — the setting's mutual exclusion in the constant direction, as a value.
//
// Nothing of the receiver survives, and that is the rule rather than an
// oversight: the clear is performed by constructing a value that holds only the
// constant, so no edit can half-apply it by forgetting a line.
//
// It states in memory what a commit performs on the file, so a surface holding
// keys alongside a write does not author the rule a second time.
func (k RawKeys) WithConstant(slug string) RawKeys {
	return RawKeys{Theme: slug}
}

// WithMember is these keys with slug in the named half of the adaptive pair, the
// other half carried across verbatim, and the constant gone — the same mutual
// exclusion in the slot direction.
//
// The clear is structural exactly as WithConstant's is. The untouched other half
// is what makes one slug reachable in both slots by two separate assignments.
//
// The half is named by a Member rather than a Slot, and the narrowing is what
// makes this total: a Slot carries a constant position, which is no half of the
// pair and names nothing to put a slug in. Member has two values and no third to
// answer for.
func (k RawKeys) WithMember(m Member, slug string) RawKeys {
	if m == MemberLight {
		return RawKeys{Light: slug, Dark: k.Dark}
	}
	return RawKeys{Light: k.Light, Dark: slug}
}

// Setting is the theme setting: exactly two states, derived from the three raw
// keys.
//
//   - Constant — one slug, detection never consulted.
//   - Adaptive — a light and a dark slug, with detection choosing between them.
//
// It holds slugs, never palettes. Nothing here inspects a theme's colours, and it
// could not: under the adaptive form the slot classifies the theme, so "light" is
// a statement about when a theme is used rather than about what is in it.
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
// The tiebreak: a non-empty `theme` wins and the slots are not read at all. A
// hand-edited file may legally carry all three keys — mutual exclusion is
// enforced on write, not on the file — so the read side needs a deterministic
// answer, and the panel's guarantee that the two setting states never coexist on
// screen holds because this rule makes the pair invisible. The stale slots are
// left untouched on disk, which is why they are still returned in RawKeys.
//
// Otherwise the pair, with a shipped default per unset slot. "Nothing set" and
// "pair nominated" are the same state, so there is no unconfigured branch — only
// a default value per slot. Partial pairs therefore do not exist: `theme_dark =
// nord` alone resolves to {DefaultLightSlug, nord}, so there is no
// incomplete-pair state for anything downstream to validate, explain or render
// around.
//
// The two defaults are DefaultLightSlug and DefaultDarkSlug — the same constants
// the per-slot fallback resolves to, never literals — so an unresolvable slot
// lands on the theme the shipped default already nominates rather than on a
// different mechanism.
//
// The empty string is the unambiguous unset sentinel, which lets cmp.Or stand in
// for the whole rule: the anchored slug charset makes an empty slug illegal, so
// an empty value can never be a theme's real name.
//
// The Setting and the RawKeys come from one evaluation, so what a surface later
// lists and what it marks cannot disagree about which slug a badge sits on.
//
// Every input is legal, including a nonsense one: an unrecognised value is a
// resolution problem for a later step rather than a decode one here.
func ResolveSetting(theme, light, dark string) (Setting, RawKeys) {
	// Stripped first, before anything is decided, so every rule below reads the
	// stripped form. A value that is only control characters therefore strips to
	// empty and counts as unset rather than as an illegal slug: treating it as set
	// would mint a panel row labelled with an empty string.
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
// It is the value as persisted rather than a slug: nothing here has been
// validated, resolved or defaulted, because a value the charset check rejects is
// still in force — it is what the user set, and therefore what a surface has to
// report back to them.
type InForceKey struct {
	// Value is the persisted value itself, control-stripped. It may be no legal
	// slug at all.
	Value string

	// Slot is the position the value occupies: SlotConstant under a constant, else
	// the pair's light or dark slot — and the light one where Both is set, since a
	// collapsed key occupies that slot as well as the other.
	Slot Slot

	// Both reports that this one value occupies both slots of the pair.
	//
	// It is a flag rather than a third Slot value because the setting has exactly
	// two slots: a third would name a position that does not exist, and every
	// surface reading a slot's own name would then have to special-case it.
	Both bool
}

// InForceKeys selects which of prefs.json's three keys a surface reports on: the
// keys in force, never every key present.
//
// The tiebreak is ResolveSetting's, applied here rather than restated: a
// non-empty `theme` wins and the slots are not read at all, and reporting two
// values Portal is not reading would put the user to work fixing something with
// no effect. The raw keys are resolved here rather than taken pre-resolved so a
// caller cannot skip that step; handing back keys ResolveSetting already produced
// is safe, because stripping is idempotent and the resolution is pure and total.
//
// The Setting and the raw keys are both read, and neither substitutes for the
// other: the Setting says which state the tiebreak settled on, the raw keys say
// which values are actually persisted. Under a pair only the slots with a
// non-empty raw value are in force — an unset slot arrives in the Setting as the
// shipped default, and the raw value is what distinguishes "the user chose this"
// from "Portal substituted it".
//
// Two slots naming the same value collapse to one entry, keyed on the persisted
// value rather than on a derived slug, so a value yielding no slug at all
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
