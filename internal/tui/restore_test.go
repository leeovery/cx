package tui

import (
	"image/color"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
)

// TestRestoreTerminalBackground_CanvasEchoGuard is the regression for the
// no-tags-signpost / Ghostty leak: when the async OSC 11 query raced the canvas
// set and the reply came back AS the canvas colour, the set-back must be SKIPPED
// (otherwise it re-paints the canvas after Bubble Tea's OSC 111 reset and the
// canvas sticks). Covers the trailing-alpha / case / missing-# reply shapes
// against a CANONICAL #RRGGBB canvas — §4.3 canonicalises at parse, so the
// retained hex is upper-case while a terminal echoes whatever case it likes.
//
// The normal set-back and empty-capture paths are covered by
// background_restore_test.go (TestRestoreTerminalBackground_WritesSetBack /
// _EmptyWritesNothing), which still pass since their originals are not the canvas.
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

			RestoreTerminalBackground(&b, Model{originalBg: tc.originalBg, startupCanvasHex: tc.startup})

			if got := b.String(); got != "" {
				t.Errorf("canvas-echo original %q (startup canvas %s) must be skipped, but wrote %q",
					tc.originalBg, tc.startup, got)
			}
		})
	}
}

// TestRestoreTerminalBackground_AnchoredToStartupHex is §11.4's named
// verification of the anchor: the guard compares against the canvas in force
// during the STARTUP window, never against whatever theme is active at exit.
//
// The startup hex is captured through the production path (an adaptive pair
// resolving on a dark OSC 11 reply), then activeTheme is moved to the other
// built-in — the shape a mid-session commit and a quit-with-uncommitted-preview
// will both take in Phases 8-9. This phase proves the ANCHOR; Phase 4 owns those
// two divergence cases, which need the panel to exist.
//
// Both directions are asserted, and the second is what makes the test bite: an
// implementation that re-derived the comparison from activeTheme would skip the
// set-back for the now-active canvas, leaving a colour the user never chose stuck
// in their terminal after Portal exits.
func TestRestoreTerminalBackground_AnchoredToStartupHex(t *testing.T) {
	resolved, _ := detectModel(t, testBuiltinPair(t)).Update(darkBg)
	startup := resolved.(Model)
	if got := startup.startupCanvasHex; got != testDarkThemeCanvas {
		t.Fatalf("startupCanvasHex = %q after a dark reply, want %q", got, testDarkThemeCanvas)
	}

	// The active theme moves; the retained hex must not move with it.
	startup.activeTheme = testLightTheme(t)

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

// TestRestoreTerminalBackground_NonHexReplyStillSetsBack pins the fall-through:
// a terminal that answers OSC 11 in the `rgb:` form cannot be compared against
// the retained hex, so the guard declines to match and the set-back is emitted
// unchanged — never worse than before the guard existed.
func TestRestoreTerminalBackground_NonHexReplyStillSetsBack(t *testing.T) {
	const rgbReply = "rgb:0b0b/0c0c/1414" // the dark canvas, in the shape sameHexColour cannot read

	var b strings.Builder
	RestoreTerminalBackground(&b, Model{originalBg: rgbReply, startupCanvasHex: testDarkThemeCanvas})

	want := ansi.SetBackgroundColor(rgbReply)
	if got := b.String(); got != want {
		t.Errorf("RestoreTerminalBackground wrote %q for a non-hex reply, want %q", got, want)
	}
}

// TestNoColor_HexCapturedAndSetBackIsANoOp pins §9.10's two halves for the
// colourless carve-out.
//
// The hex is captured as normal from the selected member — the theme machinery
// below the render layer does not branch on NO_COLOR — so the comparison value is
// defined. But NO canvas is painted and no OSC 11 background is set, so there is
// nothing to restore and the set-back must be a genuine no-op: not "idempotent in
// RGB terms", but zero bytes. Explicitly setting a terminal's reported background
// is not free — on a transparent or blurred background it makes it opaque.
//
// The captured original is deliberately DISTINCT from the canvas, so the echo
// guard cannot be what suppresses the write: only the colourless carve-out can.
func TestNoColor_HexCapturedAndSetBackIsANoOp(t *testing.T) {
	m := Build(Deps{Lister: fakeLister{}, Theme: testBuiltinPair(t), NoColor: true})
	if got := m.startupCanvasHex; got != testDarkThemeCanvas {
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
		{"rgb:0b0b/0c0c/1414", "#0b0c14", false}, // non-hex → false → caller still sets back
		{"", "#0b0c14", false},
		{"#0b0c", "#0b0c14", false},
	}
	for _, tc := range cases {
		if got := sameHexColour(tc.a, tc.b); got != tc.want {
			t.Errorf("sameHexColour(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
