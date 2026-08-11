package tui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

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
			if got := m.themeState.startupCanvasHex; got != "" {
				t.Fatalf("startupCanvasHex = %q while the gate is still open, want empty (nothing is painted yet, so there is nothing to restore)", got)
			}

			updated, _ := m.Update(tc.msg)
			after := updated.(Model)

			if got := after.themeState.startupCanvasHex; got != tc.want {
				t.Errorf("startupCanvasHex = %q, want %q (the canvas of the member the gate selected)", got, tc.want)
			}
			if got, want := after.themeState.startupCanvasHex, after.themeState.active.Canvas.Value; got != want {
				t.Errorf("startupCanvasHex = %q but the selected member's canvas is %q; the retained hex must come from the SELECTED theme, not from the nomination", got, want)
			}
		})
	}
}

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
			if got := m.themeState.startupCanvasHex; got != tc.want {
				t.Errorf("startupCanvasHex = %q at construction, want %q", got, tc.want)
			}
		})
	}
}
