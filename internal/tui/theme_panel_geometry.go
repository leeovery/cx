package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

const (
	// Fixed rather than content-driven, so the panel does not resize as the
	// theme library changes.
	themePanelPreferredWidth = 30
	themePanelMinWidth       = 24

	// Twice the preferred width, so at least half the previewed page stays
	// visible; two stages only, so a resize does not re-lay-out per column.
	themePanelPreferredAffordance = 2 * themePanelPreferredWidth

	themePanelBorderWidth = 1

	// Matches the page's Hinset; charged once in themePanelBlock.
	themePanelGutterWidth = 1

	themePanelMinBodyRows = 1

	// Counted even though the slot is unreserved when empty: the floor would be
	// one row short the moment a message appears.
	themePanelFloorMessageRows = 1
)

type themePanelHeaderShape struct {
	ruleRow  int
	labelRow int
	rows     int
}

// Below the rule in either shape — the rule runs through the border's column.
func (s themePanelHeaderShape) borderFrom() int {
	return s.ruleRow + 1
}

const themePanelCompactHeaderRows = 2

func themePanelCompactHeaderShape() themePanelHeaderShape {
	return themePanelHeaderShape{ruleRow: 0, labelRow: 1, rows: themePanelCompactHeaderRows}
}

// Measured off the page's renderers, never literals, so the panel tracks page
// chrome changes. Zero-width colourless measurement is safe: row counts are
// functions of content only.
func themePanelPageAlignedHeaderShape() themePanelHeaderShape {
	label := lipgloss.Height(renderHeaderBlock(0, theme.Theme{}, true))
	return themePanelHeaderShape{
		ruleRow:  lipgloss.Height(headerBand(0, theme.Theme{}, true)),
		labelRow: label,
		rows:     label + sectionHeaderBlockRows(),
	}
}

// Decides the header shape; renderers must not re-decide it. Charging the blank
// alignment rows to the floor would refuse terminals that can render the panel.
func themePanelHeaderShapeFor(height int, dirUnusable bool) themePanelHeaderShape {
	pageAligned := themePanelPageAlignedHeaderShape()
	if height >= themePanelFloorFor(pageAligned.rows, themePanelKeymap(), dirUnusable) {
		return pageAligned
	}
	return themePanelCompactHeaderShape()
}

func themePanelHeaderRows(height int, dirUnusable bool) int {
	return themePanelHeaderShapeFor(height, dirUnusable).rows
}

type themePanelDim int

const (
	dimNone themePanelDim = iota
	dimWidth
	dimHeight
)

// w is clamped on the refusing path too: callers ignore ok (the floor already
// refused), and an unclamped w would render a sub-minimum panel.
func themePanelWidthFor(contentW int) (w int, ok bool) {
	if contentW >= themePanelPreferredAffordance {
		return themePanelPreferredWidth, true
	}
	return themePanelMinWidth, contentW >= themePanelMinWidth
}

// Compact header cost, never page-aligned — its blank padding rows would refuse a
// renderable panel. Pass the standing keymap scope, never the live one: the
// confirm's shorter footer would admit terminals that cannot render the panel.
func themePanelMinHeight(entries []keymapEntry, dirUnusable bool) int {
	return themePanelFloorFor(themePanelCompactHeaderRows, entries, dirUnusable)
}

func themePanelFloorFor(headerRows int, entries []keymapEntry, dirUnusable bool) int {
	return themePanelChromeRows(headerRows, dirUnusable, themePanelFloorMessageRows, entries) + themePanelMinBodyRows
}

// The floor and the body budget share this chrome set, so a new component cannot
// reach one arithmetic and miss the other.
func themePanelChromeRows(headerRows int, dirUnusable bool, messageRows int, footer []keymapEntry) int {
	return headerRows +
		themePanelDirRowHeight(dirUnusable) +
		messageRows +
		themePanelFooterHeight(footer)
}

// Entry and resize must consume this answer, not derive their own arithmetic —
// passing one and failing the other opens a broken frame or refuses a fitting
// panel. Width first, so failing both reports narrow.
func themePanelFloor(contentW, contentH int, dirUnusable bool) (dim themePanelDim, ok bool) {
	if _, wide := themePanelWidthFor(contentW); !wide {
		return dimWidth, false
	}
	if contentH < themePanelMinHeight(themePanelKeymap(), dirUnusable) {
		return dimHeight, false
	}
	return dimNone, true
}

// Border and gutter are charged exactly once, here.
func themePanelInnerWidth(width int) int {
	return max(width-themePanelBorderWidth-themePanelGutterWidth, 0)
}

// The remainder is not trusted: `bubbles/list` renders a hard minimum of three
// rows however few it is given, so renderThemePanel clamps the body. The footer
// entries are the live scope — reserving the standing footer's rows while the
// confirm's shorter one renders would leave rows unaccounted for.
func themePanelListSize(p themePanel, height int) (width, rows int) {
	inner := themePanelInnerWidth(p.width)
	reserved := themePanelChromeRows(
		themePanelHeaderRows(height, p.union.DirUnusable),
		p.union.DirUnusable,
		themePanelMessageHeight(p.message, inner, themePanelMessageWraps(p, height)),
		themePanelFooterScope(p.message),
	)
	return inner, max(height-reserved, themePanelMinBodyRows)
}
