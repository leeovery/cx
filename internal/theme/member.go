package theme

type Member int

const (
	// MemberDark must stay first, so it is the zero value: dark is the
	// no-answer fallback everywhere else, and reordering inverts that silently.
	MemberDark Member = iota
	MemberLight
)

func (m Member) Opposite() Member {
	if m == MemberLight {
		return MemberDark
	}
	return MemberLight
}

func (m Member) Slot() Slot {
	if m == MemberLight {
		return SlotLight
	}
	return SlotDark
}

// Palette tags a loaded palette as the one this member serves. The fields are
// unexported so a palette cannot reach AdaptivePair untagged, which — dark being
// the zero value — would quietly make it the dark one.
func (m Member) Palette(t Theme) MemberPalette {
	return MemberPalette{member: m, theme: t}
}

// MemberPalette is constructed only via Member.Palette.
type MemberPalette struct {
	member Member
	theme  Theme
}
