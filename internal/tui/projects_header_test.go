package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

const projectsHeaderWidth = 90

func TestProjectsHeader_LabelGreenCountDetail(t *testing.T) {
	forEachBuiltinTheme(t, func(t *testing.T, th theme.Theme) {
		header := renderProjectsSectionHeader(14, projectsHeaderWidth, th, false)

		if !strings.Contains(ansi.Strip(header), "Projects") {
			t.Errorf("Projects header missing the %q label:\n%s", "Projects", header)
		}
		if !strings.Contains(ansi.Strip(header), "14") {
			t.Errorf("Projects header missing the count %q:\n%s", "14", header)
		}
		if seq := tokenFgSeq(t, th.StatePositive); !strings.Contains(header, seq) {
			t.Errorf("Projects header missing the state.positive label role sequence %q", seq)
		}
		countRun := headerStyle(th.TextMuted, th, false).Render("14")
		if !strings.Contains(header, countRun) {
			t.Errorf("Projects header missing the exact count 14 in a text.muted run:\n%s", header)
		}
	})
}

func TestProjectsHeader_RightAlignedFilterHint(t *testing.T) {
	header := renderProjectsSectionHeader(8, projectsHeaderWidth, testDarkTheme(t), false)
	if !strings.Contains(header, sectionFilterHint) {
		t.Errorf("Projects header missing the %q hint:\n%s", sectionFilterHint, header)
	}
	labelIdx := strings.Index(ansi.Strip(header), "Projects")
	hintIdx := strings.LastIndex(ansi.Strip(header), sectionFilterHint)
	if hintIdx < labelIdx {
		t.Errorf("Projects header: hint (idx %d) appears before the label (idx %d); must be right-aligned", hintIdx, labelIdx)
	}
	if got := lipgloss.Width(header); got != projectsHeaderWidth {
		t.Errorf("Projects header width = %d, want exactly %d (flex spacer to content width)", got, projectsHeaderWidth)
	}
}

func TestProjectsHeader_AlignsWithWordmark(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		const w = projectsHeaderWidth
		wordmarkLine := strings.SplitN(renderHeaderBlock(w, th, false), "\n", 2)[0]
		wordmarkCol := leadingPrintableCol(wordmarkLine)
		if wordmarkCol != 0 {
			t.Fatalf("[%v] PORTAL wordmark leading column = %d, want 0", themeLabel(th), wordmarkCol)
		}
		header := renderProjectsSectionHeader(3, w, th, false)
		if got := leadingPrintableCol(header); got != 0 {
			t.Errorf("[%v] Projects header leading column = %d, want 0 (must align with the PORTAL wordmark at the content edge)", themeLabel(th), got)
		}
	}
}

func TestProjectsHeader_NarrowDegradeDropsHint(t *testing.T) {
	wide := renderProjectsSectionHeader(5, projectsHeaderWidth, testDarkTheme(t), false)
	if !strings.Contains(wide, sectionFilterHint) {
		t.Fatalf("wide Projects header missing the hint:\n%s", wide)
	}
	const narrow = 14
	narrowHeader := renderProjectsSectionHeader(5, narrow, testDarkTheme(t), false)
	if strings.Contains(narrowHeader, sectionFilterHint) {
		t.Errorf("narrow Projects header at width %d still shows the %q hint (degrade failed):\n%s", narrow, sectionFilterHint, narrowHeader)
	}
	for i, line := range strings.Split(narrowHeader, "\n") {
		if lw := lipgloss.Width(line); lw > narrow {
			t.Errorf("narrow Projects header line %d width = %d (overflow, want <= %d)", i, lw, narrow)
		}
	}
}

func TestProjectsHeader_ColourlessDropsHueAndCanvas(t *testing.T) {
	header := renderProjectsSectionHeader(6, projectsHeaderWidth, testDarkTheme(t), true)
	if !strings.Contains(ansi.Strip(header), "Projects") || !strings.Contains(ansi.Strip(header), "6") || !strings.Contains(header, sectionFilterHint) {
		t.Errorf("colourless Projects header dropped structure:\n%s", header)
	}
	if seq := canvasSeq(t, testDarkTheme(t)); strings.Contains(header, seq) {
		t.Errorf("colourless Projects header still paints the canvas background sequence %q", seq)
	}
	for _, tok := range []theme.Token{testDarkTheme(t).StatePositive, testDarkTheme(t).TextMuted} {
		if seq := tokenFgSeq(t, tok); strings.Contains(header, seq) {
			t.Errorf("colourless Projects header still emits a foreground role sequence %q", seq)
		}
	}
}
