package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

// TestStartupCanvasHex_CapturedAtGateResolution pins §8.4's timing rule for the
// §11.4 retained hex: it is captured from the theme the gate SELECTED, at the
// single moment the gate resolves — which is also the moment the first frame is
// composed, so it is defined for every frame that exists.
//
// The pre-resolution assertion is the other half of that rule and is not
// incidental: while the detect-or-timeout window is open View paints the neutral
// blank frame and sets no OSC 11 background, so nothing has been painted and
// there is nothing to restore. An empty hex is the honest value there —
// sameHexColour returns false for it, so a Portal that dies mid-window emits the
// set-back to the terminal's own original, a harmless no-op write.
func TestStartupCanvasHex_CapturedAtGateResolution(t *testing.T) {
	for _, tc := range []struct {
		name string
		msg  tea.Msg
		want string
	}{
		{"a dark OSC 11 reply captures the dark member's canvas", darkBg, testDarkThemeCanvas},
		{"a light OSC 11 reply captures the light member's canvas", lightBg, testLightThemeCanvas},
		{"the no-answer timeout captures the dark member's canvas", appearanceTimeoutMsg{}, testDarkThemeCanvas},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := detectModel(t, testBuiltinPair(t))
			if got := m.startupCanvasHex; got != "" {
				t.Fatalf("startupCanvasHex = %q while the gate is still open, want empty (nothing is painted yet, so there is nothing to restore)", got)
			}

			updated, _ := m.Update(tc.msg)
			after := updated.(Model)

			if got := after.startupCanvasHex; got != tc.want {
				t.Errorf("startupCanvasHex = %q, want %q (the canvas of the member the gate selected)", got, tc.want)
			}
			if got, want := after.startupCanvasHex, after.activeTheme.Canvas.Value; got != want {
				t.Errorf("startupCanvasHex = %q but the selected member's canvas is %q; the retained hex must come from the SELECTED theme, not from the nomination", got, want)
			}
		})
	}
}

// TestStartupCanvasHex_ConstantCapturedAtConstruction pins the constant half of
// §8.4: a constant nomination is active from frame one, so its gate is built
// already resolved and the hex is captured at construction — before any frame is
// composed. No View is rendered here, deliberately: the value must be in hand
// without one.
func TestStartupCanvasHex_ConstantCapturedAtConstruction(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   func(*testing.T) theme.Theme
		want string
	}{
		{"a dark constant", testDarkTheme, testDarkThemeCanvas},
		{"a light constant", testLightTheme, testLightThemeCanvas},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := Build(Deps{Lister: fakeLister{}, Theme: theme.ConstantNomination(tc.th(t))})

			if !m.modeResolved() {
				t.Fatalf("a constant nomination left the first-paint gate open; the hex is captured when the gate resolves")
			}
			if got := m.startupCanvasHex; got != tc.want {
				t.Errorf("startupCanvasHex = %q at construction, want %q", got, tc.want)
			}
		})
	}
}
