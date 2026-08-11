package tui

import (
	"fmt"
	"io"
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

const (
	// Every trailing element budgets its own width plus this gap.
	themeRowGap = 1
	// Shared with the session and project rows.
	themeRowEllipsis = "…"
	// ansi.Truncate counts the tail inside its width, so 4 = three visible
	// characters plus the ellipsis. It is the label's guaranteed share: anything
	// charged after the label must charge at least this much, or the row renders
	// wider than its width.
	themeRowLabelFloor = 4
)

// The badge is looked up via Row.BadgeKey, never Slug — a reserved-name row
// would otherwise badge the slug it collides with.
type themeRowItem struct {
	Row   theme.Row
	Badge theme.Badge
}

func (i themeRowItem) FilterValue() string { return i.Row.Label() }

// Holds no derived styles — the theme changes per arrow keypress, so the model
// re-points the whole delegate. Width is a field, not a read off the list: the
// budget must follow a resize even with no arrow after it.
type themeRowDelegate struct {
	Theme      theme.Theme
	Colourless bool
	Width      int
}

// The one-line-per-row invariant pagination rests on.
func (d themeRowDelegate) Height() int { return 1 }

func (d themeRowDelegate) Spacing() int { return 0 }

func (d themeRowDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

func (d themeRowDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(themeRowItem)
	if !ok {
		return
	}
	_, _ = fmt.Fprint(w, d.renderRow(it, index == m.Index()))
}

type themeRowSegment struct {
	text  string
	token theme.Token
}

// Every cell is painted so no terminal-bg island opens inside the row.
func (d themeRowDelegate) renderRow(it themeRowItem, selected bool) string {
	bg := d.rowBg(selected)
	budget, trailing := d.compose(it)

	visible := ansi.Truncate(it.Row.Label(), budget, themeRowEllipsis)
	runs := []string{
		d.cursorColumn(bg, selected),
		d.labelStyle(it, selected).Render(visible),
		bg.Render(padTo("", budget-lipgloss.Width(visible))),
	}
	for _, segment := range trailing {
		runs = append(runs,
			bg.Render(spaces(themeRowGap)),
			d.rowToken(segment.token, selected).Render(segment.text))
	}
	return strings.Join(runs, "")
}

// Fixed priority — cursor column, `⚠`, badge, reason, label — so elements do
// not collide non-deterministically as the panel narrows: the badge outranks
// the reason for the same right edge; the label takes the remainder, floored.
func (d themeRowDelegate) compose(it themeRowItem) (labelBudget int, trailing []themeRowSegment) {
	remaining := d.Width - leftBarColumnWidth

	badge := themePanelBadgeText(it.Badge)
	if badge != "" {
		remaining -= lipgloss.Width(badge) + themeRowGap
	}

	if it.Row.Rejection != nil {
		signal := flashWarningGlyph
		remaining -= lipgloss.Width(signal) + themeRowGap
		if reason, cost := themeRowReason(it, remaining, badge); reason != "" {
			signal += " " + reason
			remaining -= cost
		}
		trailing = append(trailing, themeRowSegment{text: signal, token: d.Theme.AccentAttention})
	}
	if badge != "" {
		trailing = append(trailing, themeRowSegment{text: badge, token: d.Theme.AccentPrimary})
	}

	return max(remaining, themeRowLabelFloor), trailing
}

func themePanelBadgeText(badge theme.Badge) string {
	switch badge {
	case theme.BadgeConstant:
		return themePanelBadgeConstant
	case theme.BadgeLight:
		return themePanelBadgeLight
	case theme.BadgeDark:
		return themePanelBadgeDark
	case theme.BadgeBoth:
		return themePanelBadgeBoth
	default:
		return ""
	}
}

// The label is charged at least its floor: measured against a shorter natural
// width, a reason would fit and then push the composed row past the panel's
// width, spilling the slide-over out of its composite position.
func themeRowReason(it themeRowItem, free int, badge string) (string, int) {
	if badge != "" {
		return "", 0
	}

	reason := string(it.Row.Rejection.Reason)
	cost := lipgloss.Width(reason) + themeRowGap
	if free-max(lipgloss.Width(it.Row.Label()), themeRowLabelFloor) < cost {
		return "", 0
	}
	return reason, cost
}

func (d themeRowDelegate) cursorColumn(bg lipgloss.Style, selected bool) string {
	return renderLeftBarColumn(bg, d.rowToken(d.Theme.AccentPrimary, true), selected)
}

// An invalid label is text.subtle, never text.faint — the user must still read
// which file is broken. The cursor row wins the combination: the tint has its
// own paired foreground.
func (d themeRowDelegate) labelToken(it themeRowItem, selected bool) theme.Token {
	switch {
	case selected:
		return d.Theme.TextOnSelection
	case it.Row.Rejection != nil:
		return d.Theme.TextSubtle
	default:
		return d.Theme.TextPrimary
	}
}

func (d themeRowDelegate) rowBg(selected bool) lipgloss.Style {
	return rowBgStyle(d.Theme, selected, d.Colourless)
}

// Bold is the label's alone and, as a non-colour attribute, survives NO_COLOR.
func (d themeRowDelegate) labelStyle(it themeRowItem, selected bool) lipgloss.Style {
	base := lipgloss.Style{}
	if selected {
		base = nameBase
	}
	return rowTokenStyle(base, d.labelToken(it, selected), d.Theme, selected, d.Colourless)
}

func (d themeRowDelegate) rowToken(fg theme.Token, selected bool) lipgloss.Style {
	return rowTokenStyle(lipgloss.Style{}, fg, d.Theme, selected, d.Colourless)
}
