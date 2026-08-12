package tui

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
)

func renderProjectRow(d ProjectDelegate, width int, items []list.Item, index, selIndex int) string {
	m := list.New(items, d, width, 10)
	m.Select(selIndex)
	var buf bytes.Buffer
	d.Render(&buf, m, index, items[index])
	return buf.String()
}

func projectItems(specs ...project.Project) []list.Item {
	items := make([]list.Item, len(specs))
	for i, p := range specs {
		items[i] = ProjectItem{Project: p}
	}
	return items
}

func TestProjectRow_TwoLinesNamePrimaryPathDetail(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := ProjectDelegate{Theme: th}
		items := projectItems(
			project.Project{Name: "row-zero", Path: "/home/user/code/row-zero"},
			project.Project{Name: "portal", Path: "/home/user/code/portal"},
		)
		out := renderProjectRow(d, 80, items, 1, 0)

		lines := strings.Split(out, "\n")
		if len(lines) != 2 {
			t.Fatalf("[%v] project row must render exactly 2 lines, got %d:\n%q", themeLabel(th), len(lines), out)
		}
		if !strings.Contains(ansi.Strip(lines[0]), "portal") {
			t.Errorf("[%v] line 1 missing the project name 'portal': %q", themeLabel(th), ansi.Strip(lines[0]))
		}
		if !strings.Contains(ansi.Strip(lines[1]), "/home/user/code/portal") {
			t.Errorf("[%v] line 2 missing the project path: %q", themeLabel(th), ansi.Strip(lines[1]))
		}
		if seq := tokenFgSeq(t, th.TextPrimary); !strings.Contains(lines[0], seq) {
			t.Errorf("[%v] name line missing text.primary fg %q", themeLabel(th), seq)
		}
		if seq := tokenFgSeq(t, th.TextMuted); !strings.Contains(lines[1], seq) {
			t.Errorf("[%v] path line missing text.muted fg %q", themeLabel(th), seq)
		}
		boldName := lipgloss.NewStyle().Bold(true).Foreground(th.TextPrimary.Color()).Render("portal")
		if !strings.Contains(lines[0], "1;") && !strings.Contains(boldName, "1;") {
			t.Errorf("[%v] precondition: bold name run should carry SGR bold", themeLabel(th))
		}
		if !strings.Contains(lines[0], "1;38;2") {
			t.Errorf("[%v] name line missing the heavy/bold name run (SGR 1 + 24-bit fg): %q", themeLabel(th), escSeq(lines[0]))
		}
	}
}

func TestProjectRow_SelectedFullHeightBarTintAcrossBothLines(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := ProjectDelegate{Theme: th}
		items := projectItems(project.Project{Name: "selected", Path: "/home/user/code/selected"})
		out := renderProjectRow(d, 80, items, 0, 0)
		lines := strings.Split(out, "\n")
		if len(lines) != 2 {
			t.Fatalf("[%v] selected row must render 2 lines, got %d:\n%q", themeLabel(th), len(lines), out)
		}

		violet := tokenFgSeq(t, th.AccentPrimary)
		bgParams := selectionBgParams(t, th)
		for i, line := range lines {
			if !strings.Contains(ansi.Strip(line), "▌") {
				t.Errorf("[%v] selected line %d missing the ▌ full-height bar: %q", themeLabel(th), i, ansi.Strip(line))
			}
			if !strings.Contains(line, violet) {
				t.Errorf("[%v] selected line %d bar missing accent.primary fg %q", themeLabel(th), i, violet)
			}
			if !lineHasBgParams(line, bgParams) {
				t.Errorf("[%v] selected line %d missing the bg.selection tint %q: %q", themeLabel(th), i, bgParams, escSeq(line))
			}
		}
	}
}

func TestProjectRow_SelectedNameOnSelectionPathMutedBright(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := ProjectDelegate{Theme: th}
		items := projectItems(project.Project{Name: "selected", Path: "/home/user/code/selected"})
		out := renderProjectRow(d, 80, items, 0, 0)
		lines := strings.Split(out, "\n")

		if seq := tokenFgSeq(t, th.TextOnSelection); !strings.Contains(lines[0], seq) {
			t.Errorf("[%v] selected name missing text.on-selection fg %q", themeLabel(th), seq)
		}
		if seq := tokenFgSeq(t, th.TextTertiary); !strings.Contains(lines[1], seq) {
			t.Errorf("[%v] selected path missing text.tertiary fg %q", themeLabel(th), seq)
		}
	}
}

func TestProjectRow_UnselectedHasNoBarOrTint(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := ProjectDelegate{Theme: th}
		items := projectItems(
			project.Project{Name: "row-zero", Path: "/home/user/code/row-zero"},
			project.Project{Name: "row-one", Path: "/home/user/code/row-one"},
		)
		out := renderProjectRow(d, 80, items, 1, 0)
		bgParams := selectionBgParams(t, th)
		canvasParams := wantCanvasBgParams(t, th)
		for i, line := range strings.Split(out, "\n") {
			if strings.Contains(ansi.Strip(line), "▌") {
				t.Errorf("[%v] unselected line %d must not carry the ▌ bar: %q", themeLabel(th), i, ansi.Strip(line))
			}
			if lineHasBgParams(line, bgParams) {
				t.Errorf("[%v] unselected line %d must not carry the bg.selection tint %q: %q", themeLabel(th), i, bgParams, escSeq(line))
			}
			if !lineHasBgParams(line, canvasParams) {
				t.Errorf("[%v] unselected line %d missing the canvas paint %q: %q", themeLabel(th), i, canvasParams, escSeq(line))
			}
		}
	}
}

func TestProjectRow_UniformTwoLineHeightForPagination(t *testing.T) {
	d := ProjectDelegate{}
	if d.Height() != 2 {
		t.Fatalf("Height() = %d, want 2", d.Height())
	}
	if d.Spacing() != 0 {
		t.Fatalf("Spacing() = %d, want 0", d.Spacing())
	}
	items := projectItems(
		project.Project{Name: "a", Path: "/home/user/a"},
		project.Project{Name: "a-much-longer-project-name", Path: "/home/user/code/some/much/longer/path/here"},
	)
	for _, sel := range []int{0, 1} {
		for idx := range 2 {
			out := renderProjectRow(d, 80, items, idx, sel)
			if got := strings.Count(out, "\n"); got != 1 {
				t.Errorf("project row [idx=%d sel=%d] has %d newlines, want exactly 1 (two uniform lines): %q", idx, sel, got, out)
			}
		}
	}
}

func TestProjectRow_OverLongNameAndPathTruncate(t *testing.T) {
	const w = 40
	longName := "this-is-a-really-very-long-project-name-that-overflows-the-row"
	longPath := "/home/user/very/deeply/nested/directory/structure/that/goes/on/and/on/project"
	d := ProjectDelegate{}
	items := projectItems(project.Project{Name: longName, Path: longPath})
	out := renderProjectRow(d, w, items, 0, 0)
	lines := strings.Split(out, "\n")
	if len(lines) != 2 {
		t.Fatalf("over-long row must still render 2 lines, got %d:\n%q", len(lines), out)
	}
	for i, line := range lines {
		if lw := lipgloss.Width(line); lw > w {
			t.Errorf("line %d width = %d, overflows the list width %d: %q", i, lw, w, ansi.Strip(line))
		}
	}
	if !strings.Contains(ansi.Strip(lines[0]), "…") {
		t.Errorf("over-long name line should carry the ellipsis glyph: %q", ansi.Strip(lines[0]))
	}
	if !strings.Contains(ansi.Strip(lines[1]), "…") {
		t.Errorf("over-long path line should carry the ellipsis glyph: %q", ansi.Strip(lines[1]))
	}
}

func TestProjectRow_NeverOverflowsAtNarrowWidths(t *testing.T) {
	for _, w := range []int{1, 4, 8, 20, 40, 80} {
		items := projectItems(project.Project{
			Name: "agentic-workflows-codify",
			Path: "/home/user/leeovery/Code/agentic-workflows",
		})
		for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
			out := renderProjectRow(ProjectDelegate{Theme: th}, w, items, 0, 0)
			for i, line := range strings.Split(out, "\n") {
				if lw := lipgloss.Width(line); lw > w {
					t.Errorf("[w=%d %v] line %d width = %d, overflows the list width %d", w, themeLabel(th), i, lw, w)
				}
			}
		}
	}
}

func TestProjectRow_NoLegacyCursorOrColourLiterals(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := ProjectDelegate{Theme: th}
		items := projectItems(project.Project{Name: "portal", Path: "/home/user/code/portal"})
		out := renderProjectRow(d, 80, items, 0, 0)

		for _, banned := range []string{"38;5;212", "48;5;212"} {
			if strings.Contains(out, banned) {
				t.Errorf("[%v] delegate emitted a legacy ANSI-256 colour sequence %q: %q", themeLabel(th), banned, escSeq(out))
			}
		}
		if strings.Contains(out, "38;2;119;119;119") {
			t.Errorf("[%v] delegate emitted the legacy #777777 grey: %q", themeLabel(th), escSeq(out))
		}
		if strings.Contains(ansi.Strip(out), "> ") {
			t.Errorf("[%v] selected row should use the ▌ bar, not a '> ' cursor: %q", themeLabel(th), ansi.Strip(out))
		}
	}
}

func TestProjectRow_ColourlessDropsHueAndCanvas(t *testing.T) {
	d := ProjectDelegate{Theme: testDarkTheme(t), Colourless: true}
	items := projectItems(project.Project{Name: "portal", Path: "/home/user/code/portal"})
	out := renderProjectRow(d, 80, items, 0, 0)

	if !strings.Contains(ansi.Strip(out), "portal") || !strings.Contains(ansi.Strip(out), "/home/user/code/portal") {
		t.Errorf("colourless row dropped structure: %q", ansi.Strip(out))
	}
	if seq := canvasSeq(t, testDarkTheme(t)); strings.Contains(out, seq) {
		t.Errorf("colourless row still paints the canvas background sequence %q", seq)
	}
	if bgParams := selectionBgParams(t, testDarkTheme(t)); lineHasBgParams(out, bgParams) {
		t.Errorf("colourless selected row still carries the bg.selection tint %q", bgParams)
	}
	for _, tok := range []theme.Token{testDarkTheme(t).TextPrimary, testDarkTheme(t).TextOnSelection, testDarkTheme(t).AccentPrimary} {
		if seq := tokenFgSeq(t, tok); strings.Contains(out, seq) {
			t.Errorf("colourless row still emits a foreground role sequence %q", seq)
		}
	}
}
