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
