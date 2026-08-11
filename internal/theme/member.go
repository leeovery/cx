package theme

// Member names one half of an adaptive pair — the light/dark answer as a
// type. Its zero value is dark, deliberately: dark is the no-answer fallback
// everywhere else, and reordering the constants inverts that silently.
type Member int

const (
	// MemberDark must stay first so it is the zero value.
	MemberDark Member = iota
	MemberLight
)

// Opposite is the other half of the pair.
func (m Member) Opposite() Member {
	if m == MemberLight {
		return MemberDark
	}
	return MemberLight
}

// Slot is the setting slot this member's palette is nominated in.
func (m Member) Slot() Slot {
	if m == MemberLight {
		return SlotLight
	}
	return SlotDark
}

// Palette tags a loaded palette as the one this member serves. The fields are
// unexported so a palette cannot reach AdaptivePair untagged, which (dark
// being the zero value) would quietly make it the dark one.
func (m Member) Palette(t Theme) MemberPalette {
	return MemberPalette{member: m, theme: t}
}

// MemberPalette is constructed only via Member.Palette.
type MemberPalette struct {
	member Member
	theme  Theme
}
