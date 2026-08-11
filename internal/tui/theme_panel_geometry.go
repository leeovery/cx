package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// The theme slide-over's layout arithmetic: the width ladder, the measured header
// rows, the render floor and the split of a render height into chrome and list
// body. Every value here is a function of the terminal and the panel's state, and
// none of it draws anything.

const (
	// themePanelPreferredWidth / themePanelMinWidth are the two ends of the column
	// ladder (name, markers, slot indicators, border, gutter, padding). A fixed
	// width is predictable to lay out against; a content-driven one would make the
	// panel jump around as the theme library changes.
	//
	// themePanelWidthFor chooses between them for a given terminal, and
	// themePanelFloor refuses below the minimum; renderThemePanel renders at
	// whatever width it is handed.
	themePanelPreferredWidth = 30
	themePanelMinWidth       = 24

	// themePanelPreferredAffordance is the content width from which the panel takes
	// its preferred width: twice that width, so at least half the previewed page
	// stays visible wherever the panel is at its widest. Below it the ladder steps
	// straight to the minimum — two stages and no intermediate, so the panel holds
	// its width across a resize instead of re-laying out on every terminal column.
	themePanelPreferredAffordance = 2 * themePanelPreferredWidth

	// themePanelBorderWidth is the one column the left border occupies. It is not
	// list space, which is why the inner content width subtracts it — there is no
	// top, bottom or right edge to charge for.
	themePanelBorderWidth = 1

	// themePanelGutterWidth is the panel's inner gutter: the blank canvas column
	// between the left border and everything the panel draws, so its content sits
	// two cells in from that border. That matches the page's own Hinset, which is
	// what stops the slide-over reading as cramped beside the page it covers.
	//
	// It applies to every surface below the header rule uniformly, because it is
	// charged once in themePanelBlock rather than at each renderer: the label, the
	// list rows (cursor column included, so the `▌` moves with them), the pinned
	// directory row, the message slot and the vertical key list.
	themePanelGutterWidth = 1

	// themePanelMinBodyRows is the one list row the height floor guarantees. Below
	// it the panel refuses to open at all (themePanelFloor), so the clamp is a
	// floor rather than a degradation step.
	themePanelMinBodyRows = 1

	// themePanelFloorMessageRows is the one message row the floor counts, even
	// though the slot is not reserved when empty.
	//
	// Neither of the slot's contenders can be suppressed: the confirm gates a write
	// that must not happen silently, and the failed-commit line persists until the
	// next keypress. A floor computed without this row puts the panel one row short
	// at the moment a message appears — asking "clear constant <slug>?" about a row
	// that has just been pushed off screen.
	themePanelFloorMessageRows = 1
)

// themePanelHeaderShape is where the panel's header draws its two surfaces and
// what the whole region costs: the index of the rule, the index of the `Themes`
// label, and the rows reserved for both.
//
// The reserved rows are by construction the rows that render, because the renderer
// indexes into a block of this many rows with these two indices — the same
// discipline themePanelDirRowHeight and themePanelFooterHeight apply by measuring
// their own output.
type themePanelHeaderShape struct {
	ruleRow  int
	labelRow int
	rows     int
}

// borderFrom is the first row carrying the panel's left `│` — below the rule in
// either shape, because the rule runs THROUGH the border's column (see
// themePanelHeaderBlock).
func (s themePanelHeaderShape) borderFrom() int {
	return s.ruleRow + 1
}

// themePanelCompactHeaderRows is the compact header's cost: the rule row and the
// label row, and nothing else. It is the header the geometry rule's floor resolves
// against — the panel's header carries exactly those two things.
const themePanelCompactHeaderRows = 2

// themePanelCompactHeaderShape is the header with the two surfaces closed up: the
// rule at the top of the block and the label directly beneath it.
func themePanelCompactHeaderShape() themePanelHeaderShape {
	return themePanelHeaderShape{ruleRow: 0, labelRow: 1, rows: themePanelCompactHeaderRows}
}

// themePanelPageAlignedHeaderShape is the header padded out to the PAGE's own
// rhythm, measured off the page's renderers and never restated with a literal.
//
// The panel is composited over the page at the content region's Y=0, so its rows
// and the page's are the same terminal rows, and the slide-over only reads as a
// surface inside the content region if the two run one rhythm: the rule lands in
// the page header block's rule lane, the label on the page's section-header row,
// and the region ends where the page's first session row begins. Deriving all
// three from the page's own renderers is what moves the panel when the header
// block or the section header changes.
//
// The rows the two surfaces do not occupy are blank — an ALIGNMENT LUXURY the
// panel spends when the height affords it, never a cost the render floor carries
// (themePanelHeaderShapeFor).
//
// The measurements are taken at zero width on the colourless path, as
// themePanelFooterHeight measures its own block: a row count is a function of the
// content, not of the width or the palette, so the layout resolves before either
// is in hand.
func themePanelPageAlignedHeaderShape() themePanelHeaderShape {
	label := lipgloss.Height(renderHeaderBlock(0, theme.Theme{}, true))
	return themePanelHeaderShape{
		ruleRow:  lipgloss.Height(headerBand(0, theme.Theme{}, true)),
		labelRow: label,
		rows:     label + sectionHeaderBlockRows(),
	}
}

// themePanelHeaderShapeFor is the SINGLE decision between the panel's two header
// shapes, taken from the height the panel is rendered at: the page-aligned header
// while the height can afford the rows it pads with, the compact one below that.
//
// No renderer and no arithmetic may re-decide it. The blank alignment rows carry
// nothing, so charging them to the render floor would refuse the panel across a
// band of terminals on which it can still render a rule, a label, a list row, the
// message slot and the whole footer — the refusal the geometry rule's
// degrade-don't-refuse doctrine reserves for a panel that genuinely cannot render.
//
// The affordance is the panel's own floor computed with the page-aligned header, so
// the two questions are one arithmetic asked with two header costs.
func themePanelHeaderShapeFor(height int, dirUnusable bool) themePanelHeaderShape {
	pageAligned := themePanelPageAlignedHeaderShape()
	if height >= themePanelFloorFor(pageAligned.rows, themePanelKeymap(), dirUnusable) {
		return pageAligned
	}
	return themePanelCompactHeaderShape()
}

// themePanelHeaderRows is the panel's header cost at a given render height — the
// rows themePanelChromeRows charges and the renderer fills.
func themePanelHeaderRows(height int, dirUnusable bool) int {
	return themePanelHeaderShapeFor(height, dirUnusable).rows
}

// themePanelDim names the dimension the render floor refused on, so the entry gate
// and the resize path select the per-dimension copy from one answer rather than
// each re-deciding which dimension was at fault.
type themePanelDim int

const (
	// dimNone is no failure: both dimensions cleared the floor.
	dimNone themePanelDim = iota
	// dimWidth is the width floor — the content region cannot hold even the
	// minimum panel.
	dimWidth
	// dimHeight is the height floor — the content region cannot hold header +
	// footer + one list row + one message row (+ the directory row when unusable).
	dimHeight
)

// themePanelWidthFor is the width ladder: the panel's outer width for a given
// content-region width, and whether the region can hold a panel at all.
//
// The panel renders at its preferred width while the content region affords it
// (themePanelPreferredAffordance), steps down to the minimum below that, and
// refuses below the minimum.
//
// The width is clamped on the refusing path too. Callers take w and ignore ok
// because themePanelFloor has already refused by the time either runs; returning
// an unclamped w would make an impossible state render a sub-minimum panel
// instead of degrading to the minimum.
func themePanelWidthFor(contentW int) (w int, ok bool) {
	if contentW >= themePanelPreferredAffordance {
		return themePanelPreferredWidth, true
	}
	return themePanelMinWidth, contentW >= themePanelMinWidth
}

// themePanelMinHeight is the height floor: the header's two content rows + footer +
// one list row + one message row, plus the pinned directory row when the themes
// directory is unusable.
//
// It is the COMPACT header's cost, never the page-aligned one. The rows the
// page-aligned header pads with are blank, so a floor carrying them would refuse a
// panel that has every row it needs to render (themePanelHeaderShapeFor).
//
// The footer is measured (themePanelFooterHeight), so the floor follows a change to
// it with no second edit.
//
// The message row is unconditional because neither contender can be suppressed
// (see themePanelFloorMessageRows). The directory row is counted only when it
// renders: counting it always would refuse terminals that render a perfectly good
// panel, while counting it never would let the warning consume the single list
// row at exactly the moment built-in and persisted rows must render beneath it.
//
// Callers must pass the standing scope, never whichever footer scope happens to
// be live. The confirm's footer is strictly shorter than the standing one, so a
// terminal that clears the floor with the standing footer has rows to spare while
// the confirm is up, and the saving lands in the list body
// (themePanelListSize). Computing the floor from the transient scope would admit
// terminals that could not render the panel once the confirm resolved.
func themePanelMinHeight(entries []keymapEntry, dirUnusable bool) int {
	return themePanelFloorFor(themePanelCompactHeaderRows, entries, dirUnusable)
}

// themePanelFloorFor is the shortest height that renders a whole panel for a given
// header cost — that header + the directory row when it is due + one message row +
// the footer + the one list row the floor guarantees.
//
// It is asked with both header costs: with the compact header it is the panel's own
// floor (themePanelMinHeight), and with the page-aligned one it is the height from
// which the panel can afford to keep the page's rhythm (themePanelHeaderShapeFor,
// and the slot's own threshold in themePanelMessageWraps).
func themePanelFloorFor(headerRows int, entries []keymapEntry, dirUnusable bool) int {
	return themePanelChromeRows(headerRows, dirUnusable, themePanelFloorMessageRows, entries) + themePanelMinBodyRows
}

// themePanelChromeRows is the panel's whole chrome cost at a given state — header +
// directory row + message rows + footer — i.e. every row that is NOT the list body.
//
// It single-sources the SET of components the floor and the body budget share
// (themePanelMinHeight, themePanelListSize), so a component added to the chrome
// cannot reach one arithmetic and miss the other.
//
// Callers differ in the arguments they pass, not in the sum: the floor passes the
// compact header, its fixed message row and the standing footer scope, while the
// body passes the header shape its height affords, the slot's measured height and
// the live scope.
func themePanelChromeRows(headerRows int, dirUnusable bool, messageRows int, footer []keymapEntry) int {
	return headerRows +
		themePanelDirRowHeight(dirUnusable) +
		messageRows +
		themePanelFooterHeight(footer)
}

// themePanelFloor is the render-floor predicate. The entry condition and the
// resize condition must consume this answer rather than each deriving their own
// arithmetic: a terminal that passes one check and fails the other is precisely
// the state that opens a broken frame or refuses a panel that fitted.
//
// It reports which dimension failed so callers select the per-dimension copy from
// the same result. Width is checked first, so a terminal failing both reports
// narrow — the dimension the user can act on with the same gesture that broke it,
// and pinning the order keeps the callers' copy identical.
func themePanelFloor(contentW, contentH int, dirUnusable bool) (dim themePanelDim, ok bool) {
	if _, wide := themePanelWidthFor(contentW); !wide {
		return dimWidth, false
	}
	if contentH < themePanelMinHeight(themePanelKeymap(), dirUnusable) {
		return dimHeight, false
	}
	return dimNone, true
}

// themePanelInnerWidth is the content width inside the left border and the inner
// gutter — every panel row is composed against it and the list is sized to it.
//
// Both columns are charged exactly once, here, so no renderer has to know the panel
// has a gutter at all: themePanelBlock lays the two down and every surface composes
// against what is left.
func themePanelInnerWidth(width int) int {
	return max(width-themePanelBorderWidth-themePanelGutterWidth, 0)
}

// themePanelListSize is the (width, height) the panel's list is sized to at a given
// render height: the inner content width, and the layout remainder —
//
//	height − header − directory row(0 or 1) − message slot(0 or 1) − footer
//
// floored at one row. All four subtrahends come from what draws them — the header
// from the shape the height selects (themePanelHeaderRows), the other three
// measured off their own renderers (themePanelDirRowHeight,
// themePanelMessageHeight, themePanelFooterHeight) — so those reserved rows are by
// construction the rows that render.
//
// The remainder is neither measured nor trusted. The list body is the one block
// that can exceed the height it is sized to: `bubbles/list` renders a hard
// minimum of three rows however few it is given. renderThemePanel therefore
// clamps it (clampBlockHeight), which is not cosmetic — themePanelBlock pads a
// short assembly out, but a long one it can only cut, and it cuts from the
// bottom, so an unclamped overshoot comes off the footer.
//
// It is a pure function of the panel and the height, taking no theme: the row
// counts a block contributes are a function of its content, not of its palette,
// so the layout resolves before a theme is in hand.
//
// The footer's entries are the slot's own scope (themePanelFooterScope), resolved
// here and again at render time from the same message: the nested confirm scope
// temporarily replaces the standing footer with a shorter one, and a budget
// reserving four rows while a two-row footer renders would leave two rows of the
// panel unaccounted for. The saving lands in the list body, which is why the
// height floor stays on the standing scope (themePanelMinHeight).
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
