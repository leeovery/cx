package tui

import (
	"go/ast"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/sourceguard"
	"github.com/leeovery/portal/internal/theme"
)

func panelRuleRow(width int, th theme.Theme, colourless bool) string {
	shape := themePanelCompactHeaderShape()
	return themePanelHeaderBlock(shape, width, th, colourless)[shape.ruleRow]
}

func TestPanelRule_RendersThroughThePageRuleRenderer(t *testing.T) {
	th := testDarkTheme(t)
	for _, width := range []int{themePanelMinWidth, themePanelPreferredWidth, 80} {
		for _, colourless := range []bool{false, true} {
			got := panelRuleRow(width, th, colourless)
			want := headerSeparatorRule(width, th, colourless)
			if got != want {
				t.Errorf("panel rule at width %d (colourless=%v) = %q, want the page's rule %q — a glyph or token change must move both",
					width, colourless, got, want)
			}
		}
	}
}

func TestPanelRule_TakesNoPageFallbackWidth(t *testing.T) {
	th := testDarkTheme(t)
	for _, width := range []int{0, 1, 7} {
		row := panelRuleRow(width, th, false)
		if got := lipgloss.Width(row); got != width {
			t.Errorf("panel rule at width %d is %d cells wide; the panel must render at its own width, never the page's fallback", width, got)
		}
		if got, want := ansi.Strip(row), strings.Repeat(headerRuleGlyph, width); got != want {
			t.Errorf("panel rule at width %d = %q, want %q", width, got, want)
		}
	}
}

// The panel's rule exists to sit in the page's rule lane, so exactly one
// function may build the glyph run.
func TestRuleGlyphRun_HasASingleRenderer(t *testing.T) {
	var renderers []string
	for _, file := range parsePackageFilesByName(t) {
		sourceguard.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool {
			if repeatsRuleGlyph(call) {
				renderers = append(renderers, funcName)
			}
			return true
		})
	}

	if len(renderers) != 1 {
		t.Errorf("%d functions build the rule glyph run (%s); want exactly 1 so the page and the panel cannot drift",
			len(renderers), strings.Join(renderers, ", "))
	}
}

func repeatsRuleGlyph(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "Repeat" || len(call.Args) == 0 {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "strings" {
		return false
	}
	glyph, ok := call.Args[0].(*ast.Ident)
	return ok && glyph.Name == headerRuleGlyphIdent
}

const headerRuleGlyphIdent = "headerRuleGlyph"
