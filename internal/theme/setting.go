package theme

import "cmp"

// RawKeys are prefs.json's theme keys as read: no defaults substituted, no
// charset check, no state collapsed. They travel alongside the resolved Setting
// because that collapse hides what surfaces still have to report — an unset
// slot, and a slug the tiebreak left unread, are invisible in a Setting.
type RawKeys struct {
	Theme string
	Light string
	Dark  string
}

// NewRawKeys builds the raw keys from prefs.json's values, control-stripped. A
// value that is only control characters strips to empty and so counts as unset
// rather than as an illegal slug; nothing else is normalised, so a merely wrong
// value reaches the charset check as the user typed it.
func NewRawKeys(theme, light, dark string) RawKeys {
	return RawKeys{Theme: theme, Light: light, Dark: dark}.stripped()
}

func (k RawKeys) stripped() RawKeys {
	return RawKeys{
		Theme: StripControl(k.Theme),
		Light: StripControl(k.Light),
		Dark:  StripControl(k.Dark),
	}
}

// WithConstant is these keys with slug as the constant and both slots cleared:
// nothing of the receiver survives, so the mutual exclusion cannot half-apply.
func (k RawKeys) WithConstant(slug string) RawKeys {
	return RawKeys{Theme: slug}
}

// WithMember is these keys with slug in the named half of the adaptive pair, the
// other half carried across verbatim and the constant cleared. The half is a
// Member, not a Slot, which is what makes this total: a Slot's constant position
// is no half of the pair and names nothing to put a slug in.
func (k RawKeys) WithMember(m Member, slug string) RawKeys {
	if m == MemberLight {
		return RawKeys{Light: slug, Dark: k.Dark}
	}
	return RawKeys{Light: k.Light, Dark: slug}
}

// Setting is the theme setting collapsed to its two states: constant (one slug,
// detection never consulted) or adaptive (a light and a dark slug). The slot
// classifies the theme, so "light" says when a theme is used, not what is in it.
type Setting struct {
	// Constant is non-empty iff IsConstant, and Light and Dark are both
	// non-empty iff not.
	IsConstant bool

	Constant string

	Light string
	Dark  string
}

// ResolveSetting collapses prefs.json's raw theme keys onto the two-state
// setting and returns the stripped raw keys alongside it. A non-empty `theme`
// wins the tiebreak and the slots are not read at all — a hand-edited file may
// legally carry all three, and the stale slots are left untouched on disk.
// Otherwise the pair, with a shipped default per unset slot.
func ResolveSetting(keys RawKeys) (Setting, RawKeys) {
	// Re-stripped, since a caller may have built its keys as a plain literal
	// rather than through NewRawKeys.
	raw := keys.stripped()

	if raw.Theme != "" {
		return Setting{IsConstant: true, Constant: raw.Theme}, raw
	}

	return Setting{
		Light: cmp.Or(raw.Light, defaultSlugFor(SlotLight)),
		Dark:  cmp.Or(raw.Dark, defaultSlugFor(SlotDark)),
	}, raw
}

// Slug is the slug one slot nominates, with the shipped default substituted for
// an unset slot — the per-slot half of ResolveSetting's substitution, so a
// single-slot caller cannot report a fallback of a slug nobody set. An unset
// constant answers the empty string; there is no constant default.
func (s Setting) Slug(slot Slot) string {
	switch slot {
	case SlotLight:
		return cmp.Or(s.Light, defaultSlugFor(SlotLight))
	case SlotDark:
		return cmp.Or(s.Dark, defaultSlugFor(SlotDark))
	default:
		return s.Constant
	}
}

// SlugForSlot is the collapse from the persisted raw keys to the slug one slot
// nominates: the tiebreak a constant wins, then the shipped default substituted
// for an unset slot. Every surface that needs a single slot's slug goes through
// it, so the two halves cannot be paired anywhere else and drift.
func SlugForSlot(keys RawKeys, slot Slot) string {
	setting, _ := ResolveSetting(keys)
	return setting.Slug(slot)
}

// InForceKey is one persisted value Portal is actually reading, and where in the
// setting it sits. The value is as persisted, never validated, resolved or
// defaulted: a value the charset check rejects is still in force and still has
// to be reportable.
type InForceKey struct {
	Value string

	// Slot is SlotLight where Both is set, since a collapsed key occupies that
	// slot as well as the other.
	Slot Slot

	Both bool
}

// InForceKeys selects which of prefs.json's keys a surface reports on — those in
// force, never every key present, since reporting a value Portal is not reading
// would put the user to work fixing something with no effect. Under a pair only
// slots with a non-empty raw value qualify: the raw value is what distinguishes
// "the user chose this" from "Portal substituted the default". Two slots naming
// the same value collapse to one entry. The order is light then dark.
func InForceKeys(keys RawKeys) []InForceKey {
	setting, raw := ResolveSetting(keys)
	if setting.IsConstant {
		return []InForceKey{{Value: setting.Constant, Slot: SlotConstant}}
	}

	if raw.Light != "" && raw.Light == raw.Dark {
		return []InForceKey{{Value: raw.Light, Slot: SlotLight, Both: true}}
	}

	inForce := make([]InForceKey, 0, 2)
	if raw.Light != "" {
		inForce = append(inForce, InForceKey{Value: raw.Light, Slot: SlotLight})
	}
	if raw.Dark != "" {
		inForce = append(inForce, InForceKey{Value: raw.Dark, Slot: SlotDark})
	}
	return inForce
}
