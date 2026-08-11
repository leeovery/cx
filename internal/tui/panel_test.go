package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

func TestJoinedPanel_SingleToneJoinedFrame(t *testing.T) {
	forEachBuiltinTheme(t, func(t *testing.T, th theme.Theme) {
		compartments := [][]string{
			{"header"},
			{"body line one", "body line two"},
			{"footer"},
		}
		panel := renderJoinedPanel(compartments, th.Border, th, false)

		if !strings.ContainsAny(panel, "╭╮╰╯") {
			t.Errorf("panel must carry the rounded corner glyphs; got:\n%s", panel)
		}
		sepSeq := tokenFgSeq(t, th.Border)
		if !strings.Contains(panel, sepSeq) {
			t.Errorf("panel frame must be drawn in border.separator SGR core %q; missing in:\n%s", sepSeq, panel)
		}
	})
}

func TestJoinedPanel_BorderTokenParameterised(t *testing.T) {
	compartments := [][]string{{"header"}, {"body"}, {"footer"}}
	panel := renderJoinedPanel(compartments, testDarkTheme(t).AccentMode, testDarkTheme(t), false)

	cyanSeq := tokenFgSeq(t, testDarkTheme(t).AccentMode)
	if !strings.Contains(panel, cyanSeq) {
		t.Errorf("panel with accent.cyan border token must paint the frame in accent.cyan SGR core %q; missing in:\n%s", cyanSeq, panel)
	}
	sepSeq := tokenFgSeq(t, testDarkTheme(t).Border)
	if strings.Contains(panel, sepSeq) {
		t.Errorf("cyan-token panel must NOT carry the border.separator hue %q; found in:\n%s", sepSeq, panel)
	}
}

func TestJoinedPanel_DividersJoinSideBorders(t *testing.T) {
	compartments := [][]string{
		{"header"},
		{"body"},
		{"footer"},
	}
	panel := renderJoinedPanel(compartments, testDarkTheme(t).Border, testDarkTheme(t), false)

	dividerCount := 0
	for raw := range strings.SplitSeq(panel, "\n") {
		line := strings.TrimSpace(ansi.Strip(raw))
		if strings.HasPrefix(line, panelFrameTeeLeft) && strings.HasSuffix(line, panelFrameTeeRight) {
			dividerCount++
			interior := strings.TrimSuffix(strings.TrimPrefix(line, panelFrameTeeLeft), panelFrameTeeRight)
			if interior == "" || strings.Trim(interior, panelRuleGlyph) != "" {
				t.Errorf("divider interior between ├ and ┤ must be all rule glyphs; got %q", interior)
			}
			if strings.HasPrefix(interior, " ") || strings.HasSuffix(interior, " ") {
				t.Errorf("divider must run flush junction-to-junction (no inset gap); interior = %q", interior)
			}
		}
	}
	if dividerCount != len(compartments)-1 {
		t.Errorf("panel must carry exactly %d joined dividers (between adjacent compartments); got %d", len(compartments)-1, dividerCount)
	}
}

func TestJoinedPanel_UniformWidth(t *testing.T) {
	compartments := [][]string{
		{"a short row"},
		{"a much longer body row that sets the width"},
		{"foot"},
	}
	panel := renderJoinedPanel(compartments, testDarkTheme(t).Border, testDarkTheme(t), false)
	lines := strings.Split(panel, "\n")
	want := ansi.StringWidth(lines[0])
	for i, line := range lines {
		if got := ansi.StringWidth(line); got != want {
			t.Errorf("frame line %d width = %d, want %d (uniform):\n%s", i, got, want, panel)
		}
	}
}

func TestJoinedPanel_RowsAreInsetDividersAreNot(t *testing.T) {
	compartments := [][]string{
		{"title row"},
		{"content row"},
	}
	panel := renderJoinedPanel(compartments, testDarkTheme(t).Border, testDarkTheme(t), false)

	var contentRow string
	for raw := range strings.SplitSeq(panel, "\n") {
		line := ansi.Strip(raw)
		if strings.Contains(line, "content row") {
			contentRow = strings.TrimRight(line, " ")
			break
		}
	}
	if contentRow == "" {
		t.Fatalf("content row not found; panel:\n%s", panel)
	}
	l := strings.IndexRune(contentRow, '│')
	interior := contentRow[l+len("│"):]
	if !strings.HasPrefix(interior, strings.Repeat(" ", panelRowInset)) {
		t.Errorf("content row must carry the panelRowInset L inset inside the border; interior = %q", interior)
	}
}

func TestJoinedPanel_Colourless(t *testing.T) {
	compartments := [][]string{{"header"}, {"body"}, {"footer"}}
	panel := renderJoinedPanel(compartments, testDarkTheme(t).Border, testDarkTheme(t), true)
	if !strings.ContainsAny(panel, "╭╮╰╯├┤") {
		t.Errorf("colourless panel must keep the frame glyphs; got:\n%s", panel)
	}
	if seq := tokenFgSeq(t, testDarkTheme(t).Border); strings.Contains(panel, seq) {
		t.Errorf("colourless panel must NOT paint the border.separator hue %q", seq)
	}
}
