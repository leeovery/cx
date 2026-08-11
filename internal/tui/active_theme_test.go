package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

const (
	testDarkThemeCanvas  = "#0B0C14"
	testLightThemeCanvas = "#E1E2E7"
)

func TestBuiltinCanvasValuesPinned(t *testing.T) {
	if got := testDarkTheme(t).Canvas.Value; got != testDarkThemeCanvas {
		t.Errorf("dark built-in canvas = %q, want %q", got, testDarkThemeCanvas)
	}
	if got := testLightTheme(t).Canvas.Value; got != testLightThemeCanvas {
		t.Errorf("light built-in canvas = %q, want %q", got, testLightThemeCanvas)
	}
}

func assertActiveTheme(t *testing.T, m Model, wantCanvas string) {
	t.Helper()
	if got := m.themeState.active.Canvas.Value; got != wantCanvas {
		t.Errorf("activeTheme.Canvas.Value = %q, want %q", got, wantCanvas)
	}
	seq := "\x1b[" + sgrParams(t, lipgloss.NewStyle().Background(lipgloss.Color(wantCanvas))) + "m"
	if view := m.View().Content; !strings.Contains(view, seq) {
		t.Errorf("rendered frame does not paint the %s canvas (SGR %q)", wantCanvas, seq)
	}
}

func TestFooterTopRule_UsesBorderToken(t *testing.T) {
	dark := testDarkTheme(t)
	rule := footerTopRule(referenceFooterWidth, dark, false)

	if want := tokenFgSeq(t, dark.Border); !strings.Contains(rule, want) {
		t.Errorf("footer rule does not carry the border token %q (SGR %q)", dark.Border.Value, want)
	}
	if got := dark.Border.Value; !strings.EqualFold(got, "#292E42") {
		t.Errorf("dark border token = %q, want #292E42 (the consolidated title/footer rule)", got)
	}

	retired := sgrParams(t, lipgloss.NewStyle().Foreground(lipgloss.Color("#20232E")))
	frame := renderSessionsFooter(sessionsKeymap(), referenceFooterWidth, dark, false)
	if strings.Contains(frame, retired) {
		t.Errorf("the retired border.footer shade #20232E (SGR %q) still renders in the footer", retired)
	}
	if strings.Contains(rule, retired) {
		t.Errorf("the retired border.footer shade #20232E (SGR %q) still renders in the footer rule", retired)
	}
}
