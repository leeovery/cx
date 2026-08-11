package tui

import (
	"io"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// RestoreTerminalBackground restores the terminal's original background by
// explicitly setting the captured original back via OSC 11. It is best-effort:
// with no captured original it writes nothing and lets Bubble Tea's OSC 111
// reset stand, and write errors are ignored.
//
// Set-back rather than reset, because some terminals (mosh/Blink) ignore
// OSC 111, leaving the owned canvas stuck after Portal quits; those same
// terminals do honour the set. Under NO_COLOR it writes nothing at all: no
// canvas was painted, and setting a reported RGB back is not free — it would
// make a transparent or blurred background opaque.
//
// Canvas-echo guard — do not drop. The OSC 11 query is async and races the
// canvas set, so a terminal can report our own canvas back as the "original";
// setting that after Bubble Tea's reset re-sticks the canvas. The comparison
// must stay anchored to the retained startup canvas hex and never be
// re-derived from the active theme: a theme committed mid-session, or a quit
// with an uncommitted preview, would then skip or emit the wrong set-back and
// strand a colour the user never chose.
func RestoreTerminalBackground(w io.Writer, m Model) {
	if m.colourless {
		return
	}
	original := m.OriginalBackground()
	if original == "" {
		return
	}
	if sameHexColour(original, m.themeState.startupCanvasHex) {
		return
	}
	_, _ = io.WriteString(w, ansi.SetBackgroundColor(original))
}

// Tolerant of case, a leading '#', surrounding space, and a trailing alpha
// pair. A non-hex value (e.g. an `rgb:` reply) returns false, so the caller
// emits the set-back unchanged.
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
