package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// TestCanvasAppearance_ZeroValueIsDark pins the gate's light/dark answer as an
// unexported internal/tui concept whose ZERO VALUE is dark — the §8.8 no-answer
// fallback. A bare bool would invert this silently (false meaning "light"), which
// is exactly why the answer is a named enum with dark declared first.
func TestCanvasAppearance_ZeroValueIsDark(t *testing.T) {
	var zero canvasAppearance
	if zero != appearanceDarkCanvas {
		t.Errorf("zero canvasAppearance = %v, want appearanceDarkCanvas (the no-answer fallback)", zero)
	}
	if appearanceDarkCanvas == appearanceLightCanvas {
		t.Fatal("appearanceDarkCanvas and appearanceLightCanvas are the same value; the two canvases are indistinguishable")
	}
	// A zero-value gate carries the dark answer, so a directly constructed model
	// paints the canvas the palette was tuned for.
	var g appearanceGate
	if g.appearance != appearanceDarkCanvas {
		t.Errorf("zero appearanceGate.appearance = %v, want appearanceDarkCanvas", g.appearance)
	}
}

// testDarkThemeCanvas / testLightThemeCanvas are the two built-ins' canvas values,
// restated here so the table above can name an expectation without a *testing.T.
// They are asserted against the loaded built-ins by TestBuiltinCanvasValuesPinned.
const (
	testDarkThemeCanvas  = "#0B0C14"
	testLightThemeCanvas = "#E1E2E7"
)

// TestBuiltinCanvasValuesPinned keeps the two constants above honest against the
// embedded files they restate.
func TestBuiltinCanvasValuesPinned(t *testing.T) {
	if got := testDarkTheme(t).Canvas.Value; got != testDarkThemeCanvas {
		t.Errorf("dark built-in canvas = %q, want %q", got, testDarkThemeCanvas)
	}
	if got := testLightTheme(t).Canvas.Value; got != testLightThemeCanvas {
		t.Errorf("light built-in canvas = %q, want %q", got, testLightThemeCanvas)
	}
}

// assertActiveTheme asserts the model's active palette is the built-in carrying
// wantCanvas, and that the rendered frame actually paints that canvas — a model
// holding the right Theme but rendering from somewhere else would otherwise pass.
func assertActiveTheme(t *testing.T, m Model, wantCanvas string) {
	t.Helper()
	if got := m.activeTheme.Canvas.Value; got != wantCanvas {
		t.Errorf("activeTheme.Canvas.Value = %q, want %q", got, wantCanvas)
	}
	probe := lipgloss.NewStyle().Background(lipgloss.Color(wantCanvas)).Render(" ")
	seq := probe[:strings.IndexByte(probe, ' ')]
	if view := m.View().Content; !strings.Contains(view, seq) {
		t.Errorf("rendered frame does not paint the %s canvas (SGR %q)", wantCanvas, seq)
	}
}

// TestFooterTopRule_UsesBorderToken pins §2.2's border consolidation: `border.footer`
// is dropped, so the footer rule renders with the SAME token as the title rule. The
// accepted visual change is that the dark footer rule becomes marginally more
// prominent — #292E42 rather than #20232E — and the retired shade must appear
// nowhere in the frame.
func TestFooterTopRule_UsesBorderToken(t *testing.T) {
	dark := testDarkTheme(t)
	rule := footerTopRule(referenceFooterWidth, dark, false)

	if want := tokenFgSeq(t, dark.Border); !strings.Contains(rule, want) {
		t.Errorf("footer rule does not carry the border token %q (SGR %q)", dark.Border.Value, want)
	}
	if got := dark.Border.Value; !strings.EqualFold(got, "#292E42") {
		t.Errorf("dark border token = %q, want #292E42 (the consolidated title/footer rule)", got)
	}

	// The retired border.footer shade must not survive anywhere in the frame.
	retired := sgrForegroundCore(t, "#20232E")
	frame := renderSessionsFooter(referenceFooterWidth, dark, false)
	if strings.Contains(frame, retired) {
		t.Errorf("the retired border.footer shade #20232E (SGR %q) still renders in the footer", retired)
	}
	if strings.Contains(rule, retired) {
		t.Errorf("the retired border.footer shade #20232E (SGR %q) still renders in the footer rule", retired)
	}
}

// sgrForegroundCore renders a probe in the given raw hex and returns the bare SGR
// parameter core, so a test can assert a specific colour is ABSENT from a frame.
func sgrForegroundCore(t *testing.T, hex string) string {
	t.Helper()
	probe := lipgloss.NewStyle().Foreground(lipgloss.Color(hex)).Render("x")
	start := strings.IndexByte(probe, '[')
	end := strings.IndexByte(probe, 'm')
	if start < 0 || end <= start {
		t.Fatalf("could not derive foreground SGR core from %q", probe)
	}
	return probe[start+1 : end]
}
