package theme_test

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

var (
	nominationLight = theme.Theme{Canvas: theme.Token{Value: "#E1E2E7"}}
	nominationDark  = theme.Theme{Canvas: theme.Token{Value: "#0B0C14"}}
)

var bothMembers = []theme.Member{theme.MemberLight, theme.MemberDark}

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

func TestAdaptivePair_ArgumentOrderIsLightThenDark(t *testing.T) {
	inOrder := theme.AdaptivePair(nominationLight, nominationDark)
	swapped := theme.AdaptivePair(nominationDark, nominationLight)

	if inOrder == swapped {
		t.Errorf("a swapped construction built the same nomination; the light/dark order would be unobservable")
	}
	if got := inOrder.Select(theme.MemberLight); got != nominationLight {
		t.Errorf("Select(MemberLight) = %s, want %s", label(got), label(nominationLight))
	}
	if got := inOrder.Select(theme.MemberDark); got != nominationDark {
		t.Errorf("Select(MemberDark) = %s, want %s", label(got), label(nominationDark))
	}
	if got := swapped.Select(theme.MemberLight); got != nominationDark {
		t.Errorf("swapped Select(MemberLight) = %s, want the first argument %s", label(got), label(nominationDark))
	}
	if got := swapped.Select(theme.MemberDark); got != nominationLight {
		t.Errorf("swapped Select(MemberDark) = %s, want the second argument %s", label(got), label(nominationLight))
	}
}

func TestAdaptivePair_FillsBothMembers(t *testing.T) {
	n := theme.AdaptivePair(nominationLight, nominationDark)

	for _, member := range bothMembers {
		if got := n.Select(member); got == (theme.Theme{}) {
			t.Errorf("Select(%s) = the zero Theme; a constructed pair fills both members", memberLabel(member))
		}
	}
}

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

func TestMember_ZeroValueIsDark(t *testing.T) {
	var zero theme.Member

	if zero != theme.MemberDark {
		t.Errorf("the zero Member is %v, want MemberDark — the no-answer fallback", zero)
	}
}

func adaptivePairForTest() theme.Nomination {
	return theme.AdaptivePair(nominationLight, nominationDark)
}

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

func memberLabel(m theme.Member) string {
	name, _ := m.Slot().AttrName()
	return name
}

func label(th theme.Theme) string {
	if th.Canvas.Value == "" {
		return "zero-theme"
	}
	return "theme(canvas " + th.Canvas.Value + ")"
}

func TestNomination_ZeroValueIsDistinguishableFromBothStates(t *testing.T) {
	var zero theme.Nomination

	if theme.ConstantNomination(theme.Theme{}) == zero {
		t.Errorf("a constant nomination of the zero Theme equals the zero Nomination; 'nothing was injected' becomes undecidable")
	}
	pairOfZeroes := theme.AdaptivePair(theme.Theme{}, theme.Theme{})
	if pairOfZeroes == zero {
		t.Errorf("an adaptive pair of zero Themes equals the zero Nomination; 'nothing was injected' becomes undecidable")
	}
}
