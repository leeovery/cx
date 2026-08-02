package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// TestJoinedPanel_SingleToneJoinedFrame asserts the shared single-tone joined
// panel: a hand-drawn rounded frame (╭─╮ / │…│ / ╰─╯) whose every glyph — corners,
// sides, dividers — renders in border.separator (single-tone), with the
// compartment dividers joined to the side borders via real ├/┤ junctions. The
// help modal and the kill modal both compose through this helper.
func TestJoinedPanel_SingleToneJoinedFrame(t *testing.T) {
	for _, tc := range []struct {
		name string
		th   theme.Theme
	}{
		{"dark", testDarkTheme(t)},
		{"light", testLightTheme(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			compartments := [][]string{
				{"header"},
				{"body line one", "body line two"},
				{"footer"},
			}
			panel := renderJoinedPanel(compartments, tc.th.Border, tc.th, false)

			// Frame glyphs present.
			if !strings.ContainsAny(panel, "╭╮╰╯") {
				t.Errorf("panel must carry the rounded corner glyphs; got:\n%s", panel)
			}
			// Single-tone: every frame glyph is border.separator.
			sepSeq := tokenFgSeq(t, tc.th.Border)
			if !strings.Contains(panel, sepSeq) {
				t.Errorf("panel frame must be drawn in border.separator SGR core %q; missing in:\n%s", sepSeq, panel)
			}
			// §2.2 consolidated the two border tokens into one, so there is no
			// second border hue a frame glyph could drift onto — the positive
			// assertion above is the whole single-tone contract now.
		})
	}
}

// TestJoinedPanel_BorderTokenParameterised asserts the borderToken parameter is
// threaded through every frame glyph: passing accent.cyan (the §9.1 preview hue)
// paints the corners/sides/dividers in accent.cyan and NOT border.separator, while
// the modal default (border.separator) is unaffected — proving the preview and the
// modals can share the helper with distinct single-tone frames.
func TestJoinedPanel_BorderTokenParameterised(t *testing.T) {
	compartments := [][]string{{"header"}, {"body"}, {"footer"}}
	panel := renderJoinedPanel(compartments, testDarkTheme(t).AccentMode, testDarkTheme(t), false)

	cyanSeq := tokenFgSeq(t, testDarkTheme(t).AccentMode)
	if !strings.Contains(panel, cyanSeq) {
		t.Errorf("panel with accent.cyan border token must paint the frame in accent.cyan SGR core %q; missing in:\n%s", cyanSeq, panel)
	}
	// The grey separator hue must NOT appear when cyan was requested.
	sepSeq := tokenFgSeq(t, testDarkTheme(t).Border)
	if strings.Contains(panel, sepSeq) {
		t.Errorf("cyan-token panel must NOT carry the border.separator hue %q; found in:\n%s", sepSeq, panel)
	}
}

// TestJoinedPanel_DividersJoinSideBorders asserts the helper interleaves a joined
// ├───┤ divider between EACH adjacent pair of compartments (two compartments → one
// divider, three → two), each running flush junction-to-junction (no inset gap), so
// they meet both side borders. There must be exactly len(compartments)-1 dividers.
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

// TestJoinedPanel_UniformWidth asserts every assembled frame line is exactly the
// same width — the frame columns align top to bottom (the pagination/alignment
// invariant the help modal relies on).
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

// TestJoinedPanel_RowsAreInsetDividersAreNot asserts content rows carry the L/R
// inset (panelRowInset) inside the side borders while the dividers run flush — the
// same inset contract the help modal uses.
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

// TestJoinedPanel_Colourless asserts the NO_COLOR carve-out: the frame keeps its
// glyphs but paints no border.separator hue (native fg).
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
