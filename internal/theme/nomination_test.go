package theme_test

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// nominationLight and nominationDark are two distinguishable palettes. Only the
// canvas is seeded: it is the one token the two states must never confuse, and a
// full 19-token pair would bury the assertion in setup.
var (
	nominationLight = theme.Theme{Canvas: theme.Token{Value: "#E1E2E7"}}
	nominationDark  = theme.Theme{Canvas: theme.Token{Value: "#0B0C14"}}
)

// bothMembers is the whole of the light/dark answer domain, for the assertions
// that must hold whatever the gate answers.
var bothMembers = []theme.Member{theme.MemberLight, theme.MemberDark}

// TestConstantNomination_HoldsOneTheme pins the constant-or-pair rule's constant state: one
// loaded Theme, active from frame one, with detection NEVER consulted — so Select
// returns the constant for either answer rather than having an answer-dependent
// member at all.
func TestConstantNomination_HoldsOneTheme(t *testing.T) {
	n := theme.ConstantNomination(nominationDark)

	if !n.IsConstant() {
		t.Errorf("IsConstant() = false, want true for a constant nomination")
	}
	if got := n.Constant(); got != nominationDark {
		t.Errorf("Constant() = %s, want %s", label(got), label(nominationDark))
	}
	for _, member := range bothMembers {
		if got := n.Select(member); got != nominationDark {
			t.Errorf("Select(%s) = %s, want the constant %s (detection is never consulted)", memberLabel(member), label(got), label(nominationDark))
		}
	}
}

// TestAdaptivePair_HoldsBothWithNoActiveMember pins the construction-time load rule's adaptive
// state: both themes are loaded and held, and the value carries NO provisional active member
// — the only way to a member is Select, i.e. the gate's answer.
func TestAdaptivePair_HoldsBothWithNoActiveMember(t *testing.T) {
	n := adaptivePairForTest()

	if n.IsConstant() {
		t.Errorf("IsConstant() = true, want false for an adaptive pair")
	}
	if got := n.Constant(); got != (theme.Theme{}) {
		t.Errorf("Constant() = %s, want the zero Theme (an adaptive pair has no constant member)", label(got))
	}
	if got := n.Select(theme.MemberDark); got != nominationDark {
		t.Errorf("Select(MemberDark) = %s, want the dark member %s", label(got), label(nominationDark))
	}
	if got := n.Select(theme.MemberLight); got != nominationLight {
		t.Errorf("Select(MemberLight) = %s, want the light member %s", label(got), label(nominationLight))
	}
}

// TestAdaptivePair_WhichMemberIsNamedCarriesTheHalves pins what replaces the
// light-then-dark POSITIONAL contract: the named palette lands in the member it
// names and the other argument lands in the opposite half, so naming either
// member builds the same pair.
func TestAdaptivePair_WhichMemberIsNamedCarriesTheHalves(t *testing.T) {
	fromLight := theme.AdaptivePair(theme.MemberLight.Palette(nominationLight), nominationDark)
	fromDark := theme.AdaptivePair(theme.MemberDark.Palette(nominationDark), nominationLight)

	if fromLight != fromDark {
		t.Errorf("naming the light half and naming the dark half built different pairs; the member names which half, so either naming is the same nomination")
	}
	if theme.MemberLight.Palette(nominationLight) == theme.MemberDark.Palette(nominationLight) {
		t.Errorf("one palette tagged light equals itself tagged dark; the member does not travel with the palette, so the pair is positional after all")
	}
	if got := fromDark.Select(theme.MemberLight); got != nominationLight {
		t.Errorf("Select(MemberLight) = %s, want %s", label(got), label(nominationLight))
	}
	if got := fromDark.Select(theme.MemberDark); got != nominationDark {
		t.Errorf("Select(MemberDark) = %s, want %s", label(got), label(nominationDark))
	}
}

// TestAdaptivePair_FillsBothMembers pins the property that makes a half-empty pair
// unconstructible: whichever member is named, BOTH halves are filled from the
// arguments, so no call can leave a member unfilled — an unfilled member is the
// zero Theme, which every token of resolves through lipgloss.Color("") into a
// silently colourless render.
//
// A same-member pair cannot be expressed at all: the second argument is a bare
// palette, so it can only land in the half the first one does not name.
func TestAdaptivePair_FillsBothMembers(t *testing.T) {
	for _, named := range bothMembers {
		n := theme.AdaptivePair(named.Palette(nominationLight), nominationDark)

		if got := n.Select(named); got != nominationLight {
			t.Errorf("Select(%s) = %s, want the named palette %s", memberLabel(named), label(got), label(nominationLight))
		}
		if got := n.Select(named.Opposite()); got != nominationDark {
			t.Errorf("Select(%s) = %s, want the opposite palette %s", memberLabel(named.Opposite()), label(got), label(nominationDark))
		}
		for _, member := range bothMembers {
			if got := n.Select(member); got == (theme.Theme{}) {
				t.Errorf("Select(%s) = the zero Theme after naming %s; a constructed pair fills both members", memberLabel(member), memberLabel(named))
			}
		}
	}
}

// TestMember_NamesItsSlotAndItsOpposite pins the two-valued answer's whole
// vocabulary: the setting slot a member's palette is nominated in — the ONE
// mapping between the two light/dark types — and the other half of the pair.
func TestMember_NamesItsSlotAndItsOpposite(t *testing.T) {
	for _, tc := range []struct {
		member   theme.Member
		slot     theme.Slot
		opposite theme.Member
	}{
		{theme.MemberLight, theme.SlotLight, theme.MemberDark},
		{theme.MemberDark, theme.SlotDark, theme.MemberLight},
	} {
		if got := tc.member.Slot(); got != tc.slot {
			t.Errorf("%v.Slot() = %v, want %v", tc.member, got, tc.slot)
		}
		if got := tc.member.Opposite(); got != tc.opposite {
			t.Errorf("%v.Opposite() = %v, want %v", tc.member, got, tc.opposite)
		}
	}
}

// TestMember_ZeroValueIsDark pins the ordering as load-bearing rather than
// incidental: dark is the standing no-answer fallback everywhere else, so a
// member nobody set must select the palette Portal falls back to.
func TestMember_ZeroValueIsDark(t *testing.T) {
	var zero theme.Member

	if zero != theme.MemberDark {
		t.Errorf("the zero Member is %v, want MemberDark — the no-answer fallback", zero)
	}
}

// adaptivePairForTest is the two distinguishable palettes as an adaptive pair,
// each named by the member it serves.
func adaptivePairForTest() theme.Nomination {
	return theme.AdaptivePair(theme.MemberLight.Palette(nominationLight), nominationDark)
}

// TestNomination_ZeroValueIsNeitherState pins the constructor-only contract: the
// fields are unexported, so a zero value cannot be assembled by accident — and
// when one is reached anyway it is NEITHER state, answering every accessor with a
// zero Theme rather than panicking or impersonating a constant.
func TestNomination_ZeroValueIsNeitherState(t *testing.T) {
	var zero theme.Nomination

	if zero.IsConstant() {
		t.Errorf("zero Nomination IsConstant() = true, want false (the zero value is neither state)")
	}
	if got := zero.Constant(); got != (theme.Theme{}) {
		t.Errorf("zero Nomination Constant() = %s, want the zero Theme", label(got))
	}
	for _, member := range bothMembers {
		if got := zero.Select(member); got != (theme.Theme{}) {
			t.Errorf("zero Nomination Select(%s) = %s, want the zero Theme", memberLabel(member), label(got))
		}
	}
}

// memberLabel names a member in the light/dark vocabulary the package already
// defines, so a failure says which answer was asked rather than printing an int.
func memberLabel(m theme.Member) string {
	name, _ := m.Slot().AttrName()
	return name
}

// label names a theme by its canvas for a failure message. A whole Theme through
// %+v is 19 {name value} pairs — ~500 characters of noise on a line whose only
// job is to say WHICH palette came back.
func label(th theme.Theme) string {
	if th.Canvas.Value == "" {
		return "zero-theme"
	}
	return "theme(canvas " + th.Canvas.Value + ")"
}

// TestNomination_ZeroValueIsDistinguishableFromBothStates pins the property the
// TUI depends on to keep its dark-built-in seed: an injected nomination is never
// equal to the zero value, so "nothing was injected" is decidable from the value.
func TestNomination_ZeroValueIsDistinguishableFromBothStates(t *testing.T) {
	var zero theme.Nomination

	if theme.ConstantNomination(theme.Theme{}) == zero {
		t.Errorf("a constant nomination of the zero Theme equals the zero Nomination; 'nothing was injected' becomes undecidable")
	}
	pairOfZeroes := theme.AdaptivePair(theme.MemberLight.Palette(theme.Theme{}), theme.Theme{})
	if pairOfZeroes == zero {
		t.Errorf("an adaptive pair of zero Themes equals the zero Nomination; 'nothing was injected' becomes undecidable")
	}
}
