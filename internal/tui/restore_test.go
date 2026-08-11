package tui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

func TestRestoreTerminalBackground_CanvasEchoGuard(t *testing.T) {
	dark := testDarkTheme(t).Canvas.Value
	light := testLightTheme(t).Canvas.Value
	lowerDark := strings.ToLower(dark)

	cases := []struct {
		name       string
		startup    string
		originalBg string
	}{
		{"exact", dark, dark},
		{"lower case", dark, lowerDark},
		{"trailing alpha", dark, lowerDark + "ff"},
		{"no leading hash", dark, strings.TrimPrefix(lowerDark, "#")},
		{"light exact", light, light},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder

			RestoreTerminalBackground(&b, Model{originalBg: tc.originalBg, themeState: themeState{startupCanvasHex: tc.startup}})

			if got := b.String(); got != "" {
				t.Errorf("canvas-echo original %q (startup canvas %s) must be skipped, but wrote %q",
					tc.originalBg, tc.startup, got)
			}
		})
	}
}

func TestRestoreTerminalBackground_AnchoredToStartupHex(t *testing.T) {
	resolved, _ := detectModel(t, testBuiltinPair(t)).Update(darkBg)
	startup := resolved.(Model)
	if got := startup.themeState.startupCanvasHex; got != testDarkThemeCanvas {
		t.Fatalf("startupCanvasHex = %q after a dark reply, want %q", got, testDarkThemeCanvas)
	}

	startup.themeState.active = testLightTheme(t)

	t.Run("an echo of the startup canvas is still skipped", func(t *testing.T) {
		m := startup
		m.originalBg = testDarkThemeCanvas

		var b strings.Builder
		RestoreTerminalBackground(&b, m)

		if got := b.String(); got != "" {
			t.Errorf("wrote %q for an echo of the startup canvas %s; the guard must still skip it after activeTheme moved",
				got, testDarkThemeCanvas)
		}
	})

	t.Run("an echo of the now-active canvas is not skipped", func(t *testing.T) {
		m := startup
		m.originalBg = testLightThemeCanvas

		var b strings.Builder
		RestoreTerminalBackground(&b, m)

		want := ansi.SetBackgroundColor(testLightThemeCanvas)
		if got := b.String(); got != want {
			t.Errorf("wrote %q, want %q — %s is the ACTIVE theme's canvas, not the startup one, so it is a genuine original and must be set back",
				got, want, testLightThemeCanvas)
		}
	})
}

func TestRestoreTerminalBackground_NonHexReplyStillSetsBack(t *testing.T) {
	const rgbReply = "rgb:0b0b/0c0c/1414"

	var b strings.Builder
	RestoreTerminalBackground(&b, Model{originalBg: rgbReply, themeState: themeState{startupCanvasHex: testDarkThemeCanvas}})

	want := ansi.SetBackgroundColor(rgbReply)
	if got := b.String(); got != want {
		t.Errorf("RestoreTerminalBackground wrote %q for a non-hex reply, want %q", got, want)
	}
}

func TestNoColor_HexCapturedAndSetBackIsANoOp(t *testing.T) {
	m := Build(Deps{Lister: fakeLister{}, Theme: testBuiltinPair(t), NoColor: true})
	if got := m.themeState.startupCanvasHex; got != testDarkThemeCanvas {
		t.Errorf("startupCanvasHex = %q under NO_COLOR, want %q (captured as normal from the selected member)", got, testDarkThemeCanvas)
	}

	updated, _ := m.Update(tea.BackgroundColorMsg{Color: color.RGBA{R: 0x1e, G: 0x1e, B: 0x2e, A: 0xff}})
	final := updated.(Model)
	if got := final.OriginalBackground(); got != "#1e1e2e" {
		t.Fatalf("OriginalBackground() = %q, want %q — the capture must still happen, or this test would pass vacuously", got, "#1e1e2e")
	}

	var b strings.Builder
	RestoreTerminalBackground(&b, final)

	if b.Len() != 0 {
		t.Errorf("RestoreTerminalBackground wrote %d bytes (%q) under NO_COLOR; Portal set no background, so it must set none back", b.Len(), b.String())
	}
}

func TestSameHexColour(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"#0b0c14", "#0b0c14", true},
		{"#0B0C14", "#0b0c14", true},
		{" #0b0c14 ", "#0b0c14", true},
		{"0b0c14", "#0b0c14", true},
		{"#0b0c14ff", "#0b0c14", true},
		{"#0b0c14", "#e1e2e7", false},
		{"rgb:0b0b/0c0c/1414", "#0b0c14", false},
		{"", "#0b0c14", false},
		{"#0b0c", "#0b0c14", false},
	}
	for _, tc := range cases {
		if got := sameHexColour(tc.a, tc.b); got != tc.want {
			t.Errorf("sameHexColour(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
