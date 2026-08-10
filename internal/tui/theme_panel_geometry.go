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

// The panel's header region is measured off the page's, never restated with a
// literal.
//
// The panel is composited over the page at the content region's Y=0, so its rows
// and the page's are the same terminal rows, and the slide-over only reads as a
// surface inside the content region if the two run one rhythm. Deriving these
// four from the page's own renderers is what moves the panel when the header
// block or section header changes.
//
//   - themePanelHeaderRuleRow — the rows above the rule, which is the page header
//     block's band. Nothing is drawn in them (see themePanelHeaderBlock).
//   - themePanelHeaderLabelRow — the page header block's whole height, which is
//     therefore the index of the section-header row beneath it.
//   - themePanelHeaderRows — that plus the section-header block, so the row after
//     the panel's header is the row after the page's.
//   - themePanelBorderFromRow — where the left border starts; see
//     themePanelHeaderBlock for why it is below the rule rather than at it.
//
// The measurements are taken at zero width on the colourless path, as
// themePanelFooterHeight measures its own block: a row count is a function of the
// content, not of the width or the palette, so the layout resolves before either
// is in hand.

// themePanelHeaderRuleRow is the index of the panel's header rule — the rows the
// page's header BAND occupies above its own rule.
func themePanelHeaderRuleRow() int {
	return lipgloss.Height(headerBand(0, theme.Theme{}, true))
}

// themePanelHeaderLabelRow is the index of the panel's `Themes` label: the page's
// header block ends there, so that is the row its section header sits on.
func themePanelHeaderLabelRow() int {
	return lipgloss.Height(renderHeaderBlock(0, theme.Theme{}, true))
}

// themePanelHeaderRows is the panel's whole header cost — the page's header block
// plus its section-header block — so the panel's first list row is the page's first
// session row.
func themePanelHeaderRows() int {
	return themePanelHeaderLabelRow() + sectionHeaderBlockRows()
}

// themePanelBorderFromRow is the first row carrying the panel's left `│`.
func themePanelBorderFromRow() int {
	return themePanelHeaderRuleRow() + 1
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

// themePanelMinHeight is the height floor: header + footer + one list row + one
// message row, plus the pinned directory row when the themes directory is
// unusable.
//
// Nothing here is a literal. The footer and the header are both measured
// (themePanelFooterHeight, themePanelHeaderRows), so the floor follows a change to
// either with no second edit.
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
	return themePanelChromeRows(dirUnusable, themePanelFloorMessageRows, entries) + themePanelMinBodyRows
}

// themePanelChromeRows is the panel's whole chrome cost at a given state — header +
// directory row + message rows + footer — i.e. every row that is NOT the list body.
//
// It single-sources the SET of components the floor and the body budget share
// (themePanelMinHeight, themePanelListSize), so a component added to the chrome
// cannot reach one arithmetic and miss the other.
//
// Callers differ in the arguments they pass, not in the sum: the floor passes its
// fixed message row and the standing footer scope, while the body passes the
// slot's measured height and the live scope.
func themePanelChromeRows(dirUnusable bool, messageRows int, footer []keymapEntry) int {
	return themePanelHeaderRows() +
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
// floored at one row. All four subtrahends are measured off the renderer that
// produces them (themePanelHeaderRows, themePanelDirRowHeight,
// themePanelMessageHeight, themePanelFooterHeight), so those reserved rows are by
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
		p.union.DirUnusable,
		themePanelMessageHeight(p.message, inner, themePanelMessageWraps(p, height)),
		themePanelFooterScope(p.message),
	)
	return inner, max(height-reserved, themePanelMinBodyRows)
}
