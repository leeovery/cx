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
	// themeRowGap is the single canvas-painted space separating one row element
	// from the next. Every trailing element budgets its own width PLUS this gap,
	// so no two elements can ever abut.
	themeRowGap = 1
	// themeRowEllipsis is the truncation glyph, the same one the session and
	// project rows truncate with.
	themeRowEllipsis = "…"
	// themeRowLabelFloor is the truncation floor: three visible characters plus
	// the ellipsis. ansi.Truncate counts the tail INSIDE the width it is given, so
	// four cells is exactly "three visible characters plus the ellipsis".
	//
	// Below it the label simply stops shrinking, so it is the label's guaranteed
	// share of the row rather than a limit on it: whatever the trailing elements
	// leave, these cells are spent. Every element charged after the label must
	// therefore charge the label at least this much (see themeRowReason), or the
	// composed row renders wider than the width it was handed.
	themeRowLabelFloor = 4
)

// themeRowItem is one row of the union as the panel's list holds it: the row
// itself plus the `●` badge it carries.
//
// The badge is a field rather than something derived here because the derivation
// is a fact about the whole setting, not about one row (theme.Badges): whoever
// assembles these items looks each row's badge up through theme.Row.BadgeKey and
// never through Slug, which is what keeps a `reserved name` row from painting a
// second `●` on the slug it collides with.
type themeRowItem struct {
	Row   theme.Row
	Badge theme.Badge
}

// FilterValue returns the row's display label, satisfying list.Item. The panel's
// list is constructed with filtering disabled; the label is returned so a filter
// would match what the user can read on the row.
func (i themeRowItem) FilterValue() string { return i.Row.Label() }

// themeRowDelegate renders one panel row on exactly one line.
//
// Theme is the previewed palette, re-derived per frame rather than cached: the
// delegate's styles are assigned once at open while its theme changes on every
// arrow keypress, so it holds no derived style and the panel re-points the whole
// delegate on each restyle and each resize.
//
// Colourless is the NO_COLOR carve-out, honoured as SessionDelegate honours it:
// no canvas background and no foreground hue, with the row's glyphs (`⚠`, `●`,
// `▌`) carrying its state instead. The panel is blocked under NO_COLOR outright,
// so this is the defence rather than the daily path.
//
// Width is the panel's inner content width — a field, not a read off the list's
// own width, because the row's composition budget must follow a resize even when
// no arrow keypress follows it.
type themeRowDelegate struct {
	Theme      theme.Theme
	Colourless bool
	Width      int
}

// Height returns 1 — the one-delegate-line invariant, which `bubbles/list`
// pagination, the invalid-row skip and the paging all rest on.
func (d themeRowDelegate) Height() int { return 1 }

// Spacing returns 0, no gap between rows — the same snug band the Sessions list
// uses (a terminal has no half-row, so the only alternative is a full blank line).
func (d themeRowDelegate) Spacing() int { return 0 }

// Update returns nil; no item-level keybinding handling is needed. The panel is
// key-exclusive and routes its own keys.
func (d themeRowDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }

// Render renders one panel row. A non-themeRowItem renders nothing — the panel's
// list holds no other item type.
func (d themeRowDelegate) Render(w io.Writer, m list.Model, index int, item list.Item) {
	it, ok := item.(themeRowItem)
	if !ok {
		return
	}
	_, _ = fmt.Fprint(w, d.renderRow(it, index == m.Index()))
}

// themeRowSegment is one right-aligned trailing element: the text and the
// role token it renders in. The segments are laid out left to right after the
// label's flex column, each preceded by one background-painted gap cell.
type themeRowSegment struct {
	text  string
	token theme.Token
}

// renderRow composes the row on one line:
//
//	[2-cell cursor column][label, truncated …][pad][trailing segments]
//
// The label flexes to whatever the fixed columns leave, so the row is exactly
// d.Width cells and every cell carries a background (canvas, or the bg.selection
// tint on the cursor row) with no terminal-bg island opening inside it.
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

// compose applies the fixed element priority against d.Width, returning the
// cells the label may occupy and the trailing segments to its right.
//
// The order is fixed because the elements compete for ~24–30 columns and would
// otherwise collide non-deterministically as the panel narrows:
//
//  1. The 2-cell cursor column, which every row pays for so they share a left
//     edge (charged here, rendered by cursorColumn).
//  2. The `⚠`, always on an invalid row — the invalidity signal, reserved before
//     anything that could crowd it out.
//  3. The `●` badge, right-aligned. It outranks the reason, which competes for
//     the same right edge, so a badged row has no reason slot to fill.
//  4. The terse reason, which rides with the `⚠` as one `⚠ <reason>` phrase and
//     is charged last — see themeRowReason.
//  5. The label, truncated to what is left and floored at themeRowLabelFloor.
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

// themePanelBadgeText is the badge as the row paints it, or the empty string for
// a row carrying none — so an unbadged row renders nothing rather than needing a
// presence check at the call site.
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

// themeRowReason is the terse reason an invalid row renders beside its `⚠`, and
// the cells it costs — or "" where the priority drops it. free is what is left
// after the cursor column, the badge and the `⚠` have been charged.
//
// The reason is the first element dropped, in two ways: outright when the row
// carries a badge (the badge outranks it for the same right edge), and against
// the label's natural width, so a slug long enough to fill the row keeps the
// columns rather than being truncated to make room. `⚠` still says the row is
// invalid and doctor says why.
//
// The label is charged AT LEAST its truncation floor, which matters only for a
// label shorter than the floor: those cells are guaranteed to the label whether it
// wants them or not, so a reason measured against the shorter natural width would
// fit here and then push the composed row past the panel's declared width —
// widening the whole list body and spilling the slide-over out of its composite
// position.
//
// The value is the Reason constant's own string, rendered verbatim; the caller
// joins it to the glyph as one accent.attention run.
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

// cursorColumn is the row's first element: the 2-cell left-bar column — the
// accent.primary `▌` plus a trailing cell on the cursor row, two
// background-painted cells otherwise — so every row shares one left edge.
//
// It routes through the shared renderLeftBarColumn the Sessions and Projects
// delegates use, so the panel matches those lists structurally rather than by
// imitation.
func (d themeRowDelegate) cursorColumn(bg lipgloss.Style, selected bool) string {
	return renderLeftBarColumn(bg, d.rowToken(d.Theme.AccentPrimary, true), selected)
}

// labelToken is the label role: text.on-selection on the cursor row,
// text.subtle on an invalid one, text.primary otherwise.
//
// An invalid label is text.subtle and never text.faint: the label is the
// filename or slug the user must read to know which of their files is broken, so
// it must stay readable, and text.faint is decorative-only.
//
// The cursor row wins over the dimming because the tint has its own paired
// foreground: the arrow skip keeps the cursor off invalid rows in production,
// but a delegate must still answer for the combination rather than paint a dimmed
// label onto the selection band.
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

// rowBg delegates to the shared rowBgStyle free function (session_item.go),
// binding this delegate's theme and colourless flag: the bg.selection tint on the
// cursor row, the owned canvas otherwise, a bare style under the NO_COLOR
// carve-out. Shared with both shipped delegates so the selection-vs-canvas colour
// role lives in exactly one place.
func (d themeRowDelegate) rowBg(selected bool) lipgloss.Style {
	return rowBgStyle(d.Theme, selected, d.Colourless)
}

// labelStyle paints the row's label: the label role over the row's background,
// carrying the shared bold base (session_item.go) on the cursor row. The cursor
// row reproduces all three elements of the shipped selection treatment — `▌`,
// tint, bold label — so the panel's list reads as the same kind of list as
// Sessions. The bold is the LABEL's alone; the trailing segments keep the weight
// they carry on every other row.
//
// Bold is a non-colour attribute, so it survives the NO_COLOR carve-out (which
// drops only hue and background), exactly as the Sessions delegate's selected
// name does.
func (d themeRowDelegate) labelStyle(it themeRowItem, selected bool) lipgloss.Style {
	base := lipgloss.Style{}
	if selected {
		base = nameBase
	}
	return rowTokenStyle(base, d.labelToken(it, selected), d.Theme, selected, d.Colourless)
}

// rowToken delegates to the shared rowTokenStyle free function (session_item.go):
// the role token's foreground over the row's own background (bg.selection on the
// cursor row, canvas otherwise), and no colour at all under NO_COLOR. It renders
// the cursor column and the trailing segments, which carry no non-colour
// attribute of their own; the label goes through labelStyle, which owns the
// cursor row's bold.
func (d themeRowDelegate) rowToken(fg theme.Token, selected bool) lipgloss.Style {
	return rowTokenStyle(lipgloss.Style{}, fg, d.Theme, selected, d.Colourless)
}
