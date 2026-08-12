package tui

import (
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// Set-back rather than reset: some terminals (mosh/Blink) ignore OSC 111, leaving
// the owned canvas stuck after Portal quits. Under NO_COLOR it writes nothing —
// no canvas was painted, and setting a reported RGB back would make a transparent
// background opaque.
//
// Canvas-echo guard — do not drop. The OSC 11 query races the canvas set, so a
// terminal can report our own canvas back as the "original"; setting that after
// Bubble Tea's reset re-sticks it. The comparison must stay anchored to the
// retained startup canvas hex, never re-derived from the active theme — a
// mid-session commit would otherwise strand a colour the user never chose.
func RestoreTerminalBackground(w io.Writer, m Model) {
	if m.colourless {
		return
	}
	RestoreBackground(w, m.OriginalBackground(), m.themeState.startupCanvasHex)
}

// RestoreBackground is the set-back itself, for a surface that paints a canvas
// without being the picker — the contrast swatch. An empty original means the
// terminal never answered; an original equal to the painted canvas is the echo
// race, where setting it back would re-stick the canvas.
func RestoreBackground(w io.Writer, original, paintedCanvasHex string) {
	if original == "" || sameHexColour(original, paintedCanvasHex) {
		return
	}
	_, _ = io.WriteString(w, ansi.SetBackgroundColor(original))
}

// Tolerant of case, a leading '#', space, and a trailing alpha pair. A non-hex
// value (e.g. an `rgb:` reply) returns false, so the caller emits the set-back.
func sameHexColour(a, b string) bool {
	na, oka := normaliseHex6(a)
	nb, okb := normaliseHex6(b)
	return oka && okb && na == nb
}

func normaliseHex6(s string) (string, bool) {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "#")
	if len(s) < 6 {
		return "", false
	}
	s = s[:6]
	for i := 0; i < len(s); i++ {
		if !isHexDigit(s[i]) {
			return "", false
		}
	}
	return s, true
}

func isHexDigit(c byte) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')
}
