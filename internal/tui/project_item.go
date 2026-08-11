package tui

import (
	"fmt"
	"io"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
)

// Non-colour attributes only — rowToken layers the mode-matched colour pair.
var projectNameBase = lipgloss.NewStyle().Bold(true)

type ProjectItem struct {
	Project project.Project
}

func (i ProjectItem) FilterValue() string {
	return i.Project.Name
}

// Theme is re-pointed by the model on every restyle; a zero value renders
// silently colourless.
type ProjectDelegate struct {
	Theme theme.Theme
	// Colourless is the NO_COLOR carve-out: no backgrounds and no hue, with
	// structure and the ▌ glyph unchanged so selection stays glyph-distinct.
	Colourless bool
}

// A uniform height keeps bubbles/list pagination exact.
func (d ProjectDelegate) Height() int { return 2 }

func (d ProjectDelegate) Spacing() int { return 0 }

func (d ProjectDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d ProjectDelegate) rowBg(selected bool) lipgloss.Style {
	return rowBgStyle(d.Theme, selected, d.Colourless)
}

func (d ProjectDelegate) rowToken(base lipgloss.Style, fg theme.Token, selected bool) lipgloss.Style {
	return rowTokenStyle(base, fg, d.Theme, selected, d.Colourless)
}

func (d ProjectDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	pi, ok := item.(ProjectItem)
	if !ok {
		return
	}

	selected := index == m.Index()
	// While the filter input is being edited no row renders selected — the
	// cursor lives in the filter input.
	if m.FilterState() == list.Filtering {
		selected = false
	}

	nameTok := d.Theme.TextPrimary
	pathTok := d.Theme.TextMuted
	if selected {
		nameTok = d.Theme.TextOnSelection
		pathTok = d.Theme.TextTertiary
	}

	line1 := d.renderRowLine(m, selected, d.rowToken(projectNameBase, nameTok, selected), pi.Project.Name)
	line2 := d.renderRowLine(m, selected, d.rowToken(lipgloss.Style{}, pathTok, selected), pi.Project.Path)

	_, _ = fmt.Fprintf(w, "%s\n%s", line1, line2)
}

func (d ProjectDelegate) renderRowLine(m list.Model, selected bool, textStyle lipgloss.Style, text string) string {
	bg := d.rowBg(selected)

	bar := renderLeftBarColumn(bg, d.rowToken(lipgloss.Style{}, d.Theme.AccentPrimary, true), selected)

	// An unsized list (before the first WindowSizeMsg) has no width to flex
	// against — render the full text untruncated and unpadded.
	total := m.Width()
	if total <= 0 {
		return bar + textStyle.Render(text)
	}

	textWidth := max(total-leftBarColumnWidth, 1)
	visible := ansi.Truncate(text, textWidth, "…")
	body := textStyle.Render(visible)
	pad := bg.Render(padTo("", textWidth-lipgloss.Width(visible)))

	line := bar + body + pad

	// At pathological narrow widths the bar column plus the 1-cell floored
	// text can exceed total; a no-op on the happy path.
	return ansi.Truncate(line, total, "…")
}

func ProjectsToListItems(projects []project.Project) []list.Item {
	items := make([]list.Item, len(projects))
	for i, p := range projects {
		items[i] = ProjectItem{Project: p}
	}
	return items
}
