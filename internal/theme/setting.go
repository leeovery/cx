package theme

import "cmp"

// RawKeys are prefs.json's three theme keys as read — no defaults
// substituted, no charset check, no state collapsed. They travel alongside
// the resolved Setting because resolution discards what the surfaces still
// have to report: an unset slot, a slug the tiebreak left unread, and a value
// no theme file answers to are all invisible in a Setting.
type RawKeys struct {
	Theme string
	Light string
	Dark  string
}

// NewRawKeys builds the raw keys from prefs.json's three values,
// control-stripped and idempotently. A value that is only control characters
// strips to empty and so counts as unset, not as an illegal slug. Nothing
// else is normalised: a merely wrong value reaches the charset check as the
// user typed it.
func NewRawKeys(theme, light, dark string) RawKeys {
	return RawKeys{Theme: theme, Light: light, Dark: dark}.stripped()
}

// Keys normalise through this rather than a positional triple, where light
// and dark could be handed over the wrong way round.
func (k RawKeys) stripped() RawKeys {
	return RawKeys{
		Theme: StripControl(k.Theme),
		Light: StripControl(k.Light),
		Dark:  StripControl(k.Dark),
	}
}

// WithConstant is these keys with slug as the constant theme and both slots
// cleared. Nothing of the receiver survives — the mutual exclusion is
// performed by constructing a value that holds only the constant, so no edit
// can half-apply it.
func (k RawKeys) WithConstant(slug string) RawKeys {
	return RawKeys{Theme: slug}
}

// WithMember is these keys with slug in the named half of the adaptive pair,
// the other half carried across verbatim and the constant cleared. The half
// is a Member, not a Slot, which is what makes this total: a Slot's constant
// position is no half of the pair and names nothing to put a slug in.
func (k RawKeys) WithMember(m Member, slug string) RawKeys {
	if m == MemberLight {
		return RawKeys{Light: slug, Dark: k.Dark}
	}
	return RawKeys{Light: k.Light, Dark: slug}
}

// Setting is the theme setting collapsed to its two states: constant (one
// slug, detection never consulted) or adaptive (a light and a dark slug). It
// holds slugs, never palettes — the slot classifies the theme, so "light" says
// when a theme is used, not what is in it.
type Setting struct {
	// Constant is non-empty iff IsConstant, and Light and Dark are both
	// non-empty iff not.
	IsConstant bool

	Constant string

	Light string
	Dark  string
}

// ResolveSetting collapses prefs.json's three raw theme keys onto the
// two-state setting and returns the stripped raw keys alongside it. A
// non-empty `theme` wins the tiebreak and the slots are not read at all — a
// hand-edited file may legally carry all three, and the stale slots are left
// untouched on disk. Otherwise the pair, with a shipped default per unset
// slot, so a partial pair is not a state anything downstream must handle.
// Every input is legal: an unrecognised value is a resolution problem for a
// later step, not a decode error here.
func ResolveSetting(keys RawKeys) (Setting, RawKeys) {
	// Stripped before anything is decided, so every rule below reads the
	// stripped form even when the caller built its keys as a plain literal.
	raw := keys.stripped()

	if raw.Theme != "" {
		return Setting{IsConstant: true, Constant: raw.Theme}, raw
	}

	return Setting{
		Light: cmp.Or(raw.Light, DefaultLightSlug),
		Dark:  cmp.Or(raw.Dark, DefaultDarkSlug),
	}, raw
}

// Slug is the slug one slot nominates, with the shipped default substituted
// for an unset slot — the per-slot half of ResolveSetting's substitution, so a
// single-slot caller does not author a second rule and report a fallback of a
// slug nobody set. An unset constant answers the empty string; there is no
// constant default to substitute.
func (s Setting) Slug(slot Slot) string {
	switch slot {
	case SlotLight:
		return cmp.Or(s.Light, DefaultLightSlug)
	case SlotDark:
		return cmp.Or(s.Dark, DefaultDarkSlug)
	default:
		return s.Constant
	}
}

// InForceKey is one persisted value Portal is actually reading, and where in
// the setting it sits. The value is as persisted, never validated, resolved
// or defaulted: a value the charset check rejects is still in force and still
// has to be reportable.
type InForceKey struct {
	// Value may be no legal slug at all.
	Value string

	// Slot is SlotLight where Both is set, since a collapsed key occupies
	// that slot as well as the other.
	Slot Slot

	// Both is a flag rather than a third Slot value: the setting has exactly
	// two slots, and a third would name a position that does not exist.
	Both bool
}

// InForceKeys selects which of prefs.json's three keys a surface reports on —
// the keys in force, never every key present, since reporting a value Portal
// is not reading would put the user to work fixing something with no effect.
// Under a pair only slots with a non-empty raw value are in force: the raw
// value is what distinguishes "the user chose this" from "Portal substituted
// the default". Two slots naming the same value collapse to one entry, keyed
// on the persisted value so a value yielding no slug collapses too. The order
// is light then dark; a constant is the single entry.
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
