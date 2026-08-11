package theme

// Nomination is the loaded theme setting: the palette (constant) or palettes
// (adaptive pair) the resolved setting nominates, every file already read.
// Construct only via ConstantNomination or AdaptivePair; the zero value is
// neither state and answers every accessor with a zero Theme, so it cannot pass
// for a constant nomination of an empty palette.
type Nomination struct {
	state nominationState

	// Separate fields: folding the constant into dark would make Constant()
	// on an adaptive pair return the dark member.
	constant Theme
	light    Theme
	dark     Theme
}

type nominationState int

const (
	nominationUnset nominationState = iota
	nominationConstant
	nominationAdaptive
)

// ConstantNomination returns the nomination for a constant theme setting: one
// loaded palette, active from frame one, detection never consulted.
func ConstantNomination(t Theme) Nomination {
	return Nomination{state: nominationConstant, constant: t}
}

// AdaptivePair returns the nomination for an adaptive theme setting from its two
// palettes, light first; the gate's answer arrives through Select.
func AdaptivePair(light, dark Theme) Nomination {
	return Nomination{state: nominationAdaptive, light: light, dark: dark}
}

// IsConstant reports whether the light/dark gate is needed at all. A zero
// Nomination reports false.
func (n Nomination) IsConstant() bool {
	return n.state == nominationConstant
}

// Constant returns the constant setting's palette, and the zero Theme for any
// other state — a mis-ordered call degrades to a colourless render rather than
// taking the process down at construction.
func (n Nomination) Constant() Theme {
	return n.constant
}

// Select returns the palette the light/dark answer names, and is the route to an
// adaptive pair's active member. Under a constant it returns the constant for
// either answer, so a stray gate resolution cannot change what a constant
// paints; a zero Nomination returns the zero Theme.
func (n Nomination) Select(m Member) Theme {
	switch {
	case n.state == nominationUnset:
		return Theme{}
	case n.state == nominationConstant:
		return n.constant
	case m == MemberDark:
		return n.dark
	default:
		return n.light
	}
}
