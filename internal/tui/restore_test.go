package tui

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// TestRestoreTerminalBackground_CanvasEchoGuard is the regression for the
// no-tags-signpost / Ghostty leak: when the async OSC 11 query raced the canvas
// set and the reply came back AS the canvas colour, the set-back must be SKIPPED
// (otherwise it re-paints the canvas after Bubble Tea's OSC 111 reset and the
// canvas sticks). Covers both modes and the trailing-alpha / case / missing-#
// reply shapes. The normal set-back and empty-capture paths are covered by
// background_restore_test.go (TestRestoreTerminalBackground_WritesSetBack /
// _EmptyWritesNothing), which still pass since their originals are not the canvas.
func TestRestoreTerminalBackground_CanvasEchoGuard(t *testing.T) {
	cases := []struct {
		name       string
		th         theme.Theme
		originalBg string
	}{
		{"dark exact", testDarkTheme(t), testDarkTheme(t).Canvas.Value},
		{"dark uppercase", testDarkTheme(t), strings.ToUpper(testDarkTheme(t).Canvas.Value)},
		{"dark trailing alpha", testDarkTheme(t), testDarkTheme(t).Canvas.Value + "ff"},
		{"dark no hash", testDarkTheme(t), strings.TrimPrefix(testDarkTheme(t).Canvas.Value, "#")},
		{"light exact", testLightTheme(t), testLightTheme(t).Canvas.Value},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var b strings.Builder
			RestoreTerminalBackground(&b, Model{originalBg: tc.originalBg, activeTheme: tc.th})
			if got := b.String(); got != "" {
				t.Errorf("canvas-echo original %q (canvas %v) must be skipped, but wrote %q",
					tc.originalBg, tc.th.Canvas.Value, got)
			}
		})
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
