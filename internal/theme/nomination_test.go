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

// TestConstantNomination_HoldsOneTheme pins §8.2's constant state: one loaded
// Theme, active from frame one, with detection NEVER consulted — so Select
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
	for _, dark := range []bool{true, false} {
		if got := n.Select(dark); got != nominationDark {
			t.Errorf("Select(%v) = %s, want the constant %s (detection is never consulted)", dark, label(got), label(nominationDark))
		}
	}
}

// TestAdaptivePair_HoldsBothWithNoActiveMember pins §8.4's adaptive state: both
// themes are loaded and held, and the value carries NO provisional active member
// — the only way to a member is Select, i.e. the gate's answer.
func TestAdaptivePair_HoldsBothWithNoActiveMember(t *testing.T) {
	n := theme.AdaptivePair(nominationLight, nominationDark)

	if n.IsConstant() {
		t.Errorf("IsConstant() = true, want false for an adaptive pair")
	}
	if got := n.Constant(); got != (theme.Theme{}) {
		t.Errorf("Constant() = %s, want the zero Theme (an adaptive pair has no constant member)", label(got))
	}
	if got := n.Select(true); got != nominationDark {
		t.Errorf("Select(true) = %s, want the dark member %s", label(got), label(nominationDark))
	}
	if got := n.Select(false); got != nominationLight {
		t.Errorf("Select(false) = %s, want the light member %s", label(got), label(nominationLight))
	}
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
	for _, dark := range []bool{true, false} {
		if got := zero.Select(dark); got != (theme.Theme{}) {
			t.Errorf("zero Nomination Select(%v) = %s, want the zero Theme", dark, label(got))
		}
	}
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
	if theme.AdaptivePair(theme.Theme{}, theme.Theme{}) == zero {
		t.Errorf("an adaptive pair of zero Themes equals the zero Nomination; 'nothing was injected' becomes undecidable")
	}
}
