package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// The vertical keymap footer of the theme slide-over: one row per Core keymap
// entry, `⏎ set theme` / `d set as dark` / `l set as light` / `esc close`.
//
// It is vertical rather than a fifth flavour of Portal's horizontal footer row
// because a horizontal keymap does not fit a ~34-column panel — that one line of
// copy is ~50 cells. It follows the help modal's key-column body idiom
// (helpModalRow's fixed key column, one row per binding).
//
// Its rows are descriptor-driven so the panel's commit/close keys cannot drift
// from dispatch. The Core filter is the whole of the rule — arrows and paging
// ride the same descriptor as non-core entries so the scope stays complete (see
// themePanelKeymap).
//
// Every cell is canvas-painted (leaf .Background(canvas)), including the key
// column's pad and the pad out to the panel's inner width, so no terminal-bg
// island opens inside the panel body. Under the NO_COLOR carve-out the canvas
// and every hue drop; the panel is blocked under NO_COLOR outright, so that is
// the defence rather than the daily path.

// themePanelFooterKeyColumnWidth is the fixed width of the left key-glyph column,
// so the labels share a left edge regardless of glyph length. It is sized for the
// panel: the widest glyph in the panel scope is `esc`, and the help body's far
// wider column would eat nearly half a 27-column panel.
//
// It is fixed rather than derived from the widest glyph in the entries it is
// handed: the confirm footer substitutes `y`/`n` into the same screen position,
// and a per-slice column would step its labels two cells left as the confirm
// raises and back again as it resolves.
//
// The gap between the column and the label is the horizontal footer's own
// footerKeyLabelGap, so both footers read the same gap from one place.
const themePanelFooterKeyColumnWidth = 3

// renderThemePanelFooter renders the vertical keymap footer for the given
// entries: the Core entries only, one row per entry, each padded to width cells.
//
// It takes its entries as a parameter and never calls themePanelKeymap, because
// the nested confirm scope (themePanelConfirmKeymap) temporarily replaces this
// footer — `y confirm` / `n cancel` while the slot-from-constant confirm is live
// — and the live scope is chosen by themePanelFooterScope, which the panel's
// layout and its render path both read.
//
// width is the panel's inner content width; rows are padded out to it so the
// canvas covers every cell. A row wider than width is returned unpadded rather
// than truncated or wrapped: the widest row (`d set as dark`) is 15 cells against
// a minimum inner width comfortably above it, and below the render floor the
// panel refuses to open at all (themePanelFloor).
func renderThemePanelFooter(entries []keymapEntry, width int, th theme.Theme, colourless bool) string {
	return lipgloss.JoinVertical(lipgloss.Left, themePanelFooterRows(entries, width, th, colourless)...)
}

// themePanelFooterHeight is the rendered height of the vertical footer for the
// given entries — the row budget the panel layout subtracts and the height floor
// adds.
//
// It is measured off the rendered block, as sessionFooterHeight is, so the
// reserved rows are by construction the rows that render. The measurement needs
// neither a width nor a theme: the block is one row per Core entry and never
// wraps, so the row count is a function of the entries alone — which lets the
// panel's layout ask for the height before it has resolved either.
func themePanelFooterHeight(entries []keymapEntry) int {
	return lipgloss.Height(renderThemePanelFooter(entries, 0, theme.Theme{}, true))
}

// themePanelFooterRows renders one row per Core entry, in descriptor order.
// Non-core entries are dropped — the same distinction the main footer applies:
// arrows and paging are a given in a list and ride the descriptor for dispatch
// parity, not for the user's eye.
func themePanelFooterRows(entries []keymapEntry, width int, th theme.Theme, colourless bool) []string {
	rows := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.Core {
			continue
		}
		rows = append(rows, themePanelFooterRow(e, width, th, colourless))
	}
	return rows
}

// themePanelFooterRow renders one entry as `<glyph> <label>`: the key glyph in
// accent.key within the fixed key column, one canvas gap, then the Action label
// in text.muted — the same token split the horizontal footer uses, in the shared
// fixed-width key-column layout.
//
// The glyph resolves through helpKeyGlyph, so a HelpKey override reads exactly as
// it does in the help body. The label is the terse Action, never HelpAction — a
// ~30 column panel has no room for "Assign to the dark slot".
func themePanelFooterRow(e keymapEntry, width int, th theme.Theme, colourless bool) string {
	row := keyColumnRow(
		helpKeyGlyph(e), e.Action,
		headerStyle(th.AccentKey, th, colourless),
		headerStyle(th.TextMuted, th, colourless),
		themePanelFooterKeyColumnWidth, footerKeyLabelGap, th, colourless,
	)
	return headerPadRight(row, lipgloss.Width(row), width, th, colourless)
}
