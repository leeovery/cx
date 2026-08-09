package theme

// Nomination is the LOADED theme setting: the palette (or palettes) the
// resolved setting nominates, with every file already read.
//
// It has exactly two states, mirroring the two-state theme setting:
//
//   - CONSTANT — one Theme, active from frame one. Detection is never consulted,
//     so a constant's first paint waits for nothing.
//   - ADAPTIVE — the light and dark Themes of a nominated pair, both loaded, and
//     NO ACTIVE MEMBER. The light/dark gate selects one when the OSC 11 reply or
//     the timeout lands, which is before anything is painted — so there is no
//     frame to render in the interval and nothing a provisional member would be
//     for. Handing one out would also invite a second resolution to reconcile
//     with the gate's resolve-once rule.
//
// It is what the TUI constructor takes where it once took a light/dark
// appearance: a theme IS the mode, so there is no mode left to pin, and the
// gate's job narrows from "choose a canvas" to "choose between values already in
// hand".
//
// CONSTRUCTOR-ONLY. The fields are unexported so the two states can only be
// reached through ConstantNomination and AdaptivePair; a zero value is NEITHER
// state and answers every accessor with a zero Theme rather than panicking or
// silently impersonating a constant. That also makes the zero value a reliable
// "nothing was injected" sentinel for a caller holding one by value.
type Nomination struct {
	// state is which of the two states this nomination is in. Its zero value is
	// deliberately neither, so an accidentally zero Nomination cannot pass for a
	// constant nomination of an empty palette.
	state nominationState

	// constant is the CONSTANT state's single palette; light and dark are the
	// ADAPTIVE state's two. They are separate fields rather than one reused slot
	// because the states are answered by different accessors: folding the constant
	// into dark would make Constant() on an adaptive pair return the dark member,
	// which is precisely the "provisional active member" this type refuses to have.
	constant Theme
	light    Theme
	dark     Theme
}

// nominationState distinguishes the two states of a Nomination, with an
// unset zero value that is neither.
type nominationState int

const (
	// nominationUnset is the zero value: a Nomination nobody constructed.
	nominationUnset nominationState = iota
	// nominationConstant is the constant state — one theme, no detection.
	nominationConstant
	// nominationAdaptive is the adaptive state — a light and a dark theme, with
	// the gate choosing between them.
	nominationAdaptive
)

// ConstantNomination returns the nomination for a constant theme setting: the
// one loaded palette, active from frame one, with detection never consulted.
func ConstantNomination(t Theme) Nomination {
	return Nomination{state: nominationConstant, constant: t}
}

// AdaptivePair returns the nomination for an adaptive theme setting: both
// loaded palettes and no active member. Which one becomes active is the gate's
// answer, delivered through Select.
//
// EACH PALETTE NAMES THE MEMBER IT SERVES, so the arguments carry nothing
// positional and transposing them yields the same nomination. A light-then-dark
// parameter order is a contract only a comment can state, and reading it the
// wrong way round at one call site inverts light and dark for the whole product
// — with every palette still loading, every token still resolving and nothing
// failing anywhere.
//
// BOTH MEMBERS MUST BE NAMED. A pair naming one member twice fills that member
// and leaves the other the zero Theme, for the reason the zero Nomination
// answers with one: a caller error degrades to a palette that renders rather
// than taking the process down at construction.
func AdaptivePair(a, b MemberPalette) Nomination {
	n := Nomination{state: nominationAdaptive}
	n.hold(a)
	n.hold(b)
	return n
}

// hold puts one palette in the member it names.
func (n *Nomination) hold(p MemberPalette) {
	if p.member == MemberLight {
		n.light = p.theme
		return
	}
	n.dark = p.theme
}

// IsConstant reports whether the setting is constant — the question the caller
// asks to decide whether the light/dark gate is needed at all.
//
// A zero Nomination reports false: it names no constant, so claiming one would
// send the caller to Constant() for a palette that is not there.
func (n Nomination) IsConstant() bool {
	return n.state == nominationConstant
}

// Constant returns the constant setting's single palette, and the zero Theme for
// any other state.
//
// A zero Theme rather than a panic, deliberately: this is the accessor a caller
// reaches after IsConstant, and a mis-ordered call should degrade to a palette
// that renders (lipgloss.Color("") is the no-colour sentinel) rather than take
// the process down at construction.
func (n Nomination) Constant() Theme {
	return n.constant
}

// Select returns the member the light/dark answer names — the ONLY route to an
// adaptive pair's active member.
//
// Under a CONSTANT it returns the constant for either answer. That is not a
// convenience: detection is never consulted for a constant, so there is no
// answer to honour, and having the accessor honour one anyway would make a
// stray gate resolution able to change what a constant paints.
//
// A zero Nomination returns the zero Theme for either answer, matching Constant.
func (n Nomination) Select(m Member) Theme {
	switch {
	case n.state == nominationUnset:
		// Redundant against the zero fields, and deliberately kept: this is the one
		// place all three states meet, so the contract is stated rather than left to
		// be inferred from a struct that happens to be empty.
		return Theme{}
	case n.state == nominationConstant:
		return n.constant
	case m == MemberDark:
		return n.dark
	default:
		return n.light
	}
}
