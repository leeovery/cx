package theme

// Member names one half of an adaptive pair: the palette a light terminal is
// painted with, or the one a dark terminal is. It is the light/dark ANSWER as a
// type.
//
// It is two-valued where Slot is three-valued, and the difference is not a
// narrowing for its own sake: a slot is a POSITION IN THE SETTING, and the
// setting has a constant position; an answer to "light or dark?" has no third
// value to give, so a selector typed as a Slot would carry one that means
// nothing to it.
//
// Its ZERO VALUE IS DARK, deliberately and load-bearingly: dark is the no-answer
// fallback everywhere else, so a member nobody set already names the palette
// Portal falls back to. Ordering the constants the other way round inverts that
// silently.
type Member int

const (
	// MemberDark is the dark half of the pair, and MUST stay first so it is the
	// zero value.
	MemberDark Member = iota
	// MemberLight is the light half of the pair.
	MemberLight
)

// Opposite is the other half of the pair.
func (m Member) Opposite() Member {
	if m == MemberLight {
		return MemberDark
	}
	return MemberLight
}

// Slot is the setting slot this member's palette is nominated in: the light
// member's is the light slug, the dark member's the dark one.
//
// It is THE correspondence between the answer and the position, so a caller
// holding one and needing the other calls it rather than restating the rule.
func (m Member) Slot() Slot {
	if m == MemberLight {
		return SlotLight
	}
	return SlotDark
}

// Palette tags a loaded palette as the one this member serves, which is how
// AdaptivePair is told which half each of its arguments is.
//
// It is the only constructor for a MemberPalette. The fields are unexported so a
// palette cannot reach the pair untagged, which — dark being the zero value —
// would quietly make it the dark one.
func (m Member) Palette(t Theme) MemberPalette {
	return MemberPalette{member: m, theme: t}
}

// MemberPalette is one loaded palette of an adaptive pair together with the
// member it serves. CONSTRUCTOR-ONLY: see Member.Palette.
type MemberPalette struct {
	member Member
	theme  Theme
}
