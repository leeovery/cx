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
	// It is a FLOOR rather than a degradation step: below it the panel is already
	// at the panel's refuse threshold, so no further rule is needed — the label
	// simply stops shrinking, and the row it belongs to is one the panel refuses
	// to open at all.
	themeRowLabelFloor = 4
)

// themeRowItem is one row of the union as the panel's list holds it: the row
// itself plus the `●` badge it carries.
//
// The badge is a FIELD rather than something derived here because the derivation
// is a fact about the whole setting, not about one row (theme.Badges): whoever
// assembles these items looks each row's badge up through theme.Row.BadgeKey and
// NEVER through Slug, which is what keeps a `reserved name` row from painting a
// second `●` on the slug it collides with.
//
// FilterValue is declared FOR THE list.Item INTERFACE ONLY. Panel search /
// filtering is deferred by decision, so the panel's list is constructed
// with SetFilteringEnabled(false) and this value is never consulted — the label is
// returned so that a future filter, if the decision is ever revisited, matches
// what the user can actually read on the row.
type themeRowItem struct {
	Row   theme.Row
	Badge theme.Badge
}

// FilterValue returns the row's display label. See the type comment: the panel
// disables filtering, so nothing consumes it today.
func (i themeRowItem) FilterValue() string { return i.Row.Label() }

// themeRowDelegate renders one panel row on exactly one line.
//
// Theme is the PREVIEWED palette, and it is re-derived per frame rather than
// cached: the panel's list is the worst case of the cached-style class,
// because its styles are assigned once at open while its theme changes on EVERY
// arrow keypress. The delegate therefore holds no derived style — every run is
// painted from this field at render time, and the panel re-points the whole
// delegate through one construction site on each restyle and each resize.
//
// Colourless is the NO_COLOR carve-out, honoured exactly as SessionDelegate
// honours it: no canvas background and no foreground hue, with the row's glyphs
// (`⚠`, `●`, `▌`) carrying its state instead. The panel is blocked under
// NO_COLOR outright, so this is the defence rather than the daily path.
//
// Width is the panel's INNER content width — a field, not a read off the list's
// own width, because the panel sizes its list from the same arithmetic and the
// row's composition budget must follow a resize even when no arrow keypress
// follows it.
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
		d.rowToken(d.labelToken(it, selected), selected).Render(visible),
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
// The priority is fixed BECAUSE the elements compete for ~27–34 columns, and
// without a fixed order they collide non-deterministically as the panel narrows:
//
//  1. The 2-cell cursor column, which every row pays for so they share a left
//     edge (charged here, rendered by cursorColumn).
//  2. The `⚠`, ALWAYS on an invalid row. It is the invalidity signal, so it is
//     reserved before anything that could crowd it out.
//  3. The `●` badge, right-aligned. The union exists so the marker always has a home,
//     so the badge OUTRANKS the reason: the two compete for the same right edge,
//     and a badged row simply has no reason slot to fill.
//  4. The terse reason, which rides WITH the `⚠` as one pinned `⚠ <reason>`
//     phrase and is charged last — see themeRowReason.
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

// themePanelBadgeText is the badge as the row paints it: the panel's pinned copy
// for the badge theme.Badges derived, and the empty string for a row carrying
// none — so an unbadged row renders nothing rather than needing a presence check
// at the call site.
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
// the cells it costs — or "" where the priority drops it. free is what is left after the
// cursor column, the badge and the `⚠` have been charged.
//
// THE REASON IS THE FIRST ELEMENT DROPPED, in two ways.
// It is dropped OUTRIGHT when the row carries a badge, because the two compete
// for the same right edge and the badge outranks it — the union exists so the marker
// always has a home. And it is dropped against the label's NATURAL width, because
// the label outranks it too (priority 4 over 5), so a slug long enough to
// fill the row takes the columns rather than being truncated to make room for a
// reason. `⚠` still says the row is invalid and doctor says why, which is exactly
// the split the loader's reason/detail pair already draws.
//
// The value is the Reason constant's own string, rendered VERBATIM and never
// re-worded; the caller joins it to the glyph as ONE accent.attention run so the
// phrase reads exactly as it is pinned.
func themeRowReason(it themeRowItem, free int, badge string) (string, int) {
	if badge != "" {
		return "", 0
	}

	reason := string(it.Row.Rejection.Reason)
	cost := lipgloss.Width(reason) + themeRowGap
	if free-lipgloss.Width(it.Row.Label()) < cost {
		return "", 0
	}
	return reason, cost
}

// cursorColumn is the row's first element: the shipped selection treatment's 2-cell
// left-bar column — the accent.primary `▌` plus a trailing cell on the cursor row,
// two background-painted cells otherwise — so every row shares one left edge
// whether or not it is the cursor row.
//
// It routes through the SHARED renderLeftBarColumn the Sessions and Projects
// delegates use, which is what makes the panel "read as the same kind of list as
// Sessions" structurally rather than by imitation.
func (d themeRowDelegate) cursorColumn(bg lipgloss.Style, selected bool) string {
	return renderLeftBarColumn(bg, d.rowToken(d.Theme.AccentPrimary, true), selected)
}

// labelToken is the label role: text.on-selection on the cursor row,
// text.subtle on an invalid one, text.primary otherwise.
//
// AN INVALID LABEL IS text.subtle AND NEVER text.faint. text.faint is
// decorative-only and must never reach the UI floor — but this label
// is the filename or slug the user must read to know which of their files is
// broken, which is the whole justification for listing invalid files.
// text.subtle is the de-emphasised-but-readable step, which is exactly the role.
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

// rowToken delegates to the shared rowTokenStyle free function (session_item.go):
// the role token's foreground over the row's own background (bg.selection on the
// cursor row, canvas otherwise), and no colour at all under NO_COLOR. Panel rows
// carry no non-colour attribute of their own, so there is no base style to pass.
func (d themeRowDelegate) rowToken(fg theme.Token, selected bool) lipgloss.Style {
	return rowTokenStyle(lipgloss.Style{}, fg, d.Theme, selected, d.Colourless)
}
