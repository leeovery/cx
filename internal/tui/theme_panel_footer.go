package tui

import (
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
)

// The §9.1 vertical keymap footer of the theme slide-over: one row per Core
// keymap entry, `⏎ set theme` / `d set as dark` / `l set as light` / `esc close`
// (§14A's pinned copy, carried by the descriptor's Action strings).
//
// It is VERTICAL rather than a fifth flavour of Portal's horizontal footer row
// because a horizontal keymap does not fit a ~30-column panel — that one line of
// copy is ~50 cells — and because the vertical form already exists in this
// codebase as the help modal's key-column body, which is the idiom it follows
// (helpModalRow's fixed key column, one row per binding).
//
// Its rows are DESCRIPTOR-DRIVEN (§9.12): the panel introduces four commit/close
// keys, and authoring them as a bespoke second binding list inside this file is
// the exact drift class keymap_dispatch_guard_test exists to guard and the one
// this codebase has already paid for once. The Core filter is the whole of the
// rule — arrows and paging ride the same descriptor as non-core entries so the
// scope stays complete for Phase 9's dispatch guard (see themePanelKeymap).
//
// Every cell is canvas-painted (leaf .Background(canvas), §1), including the key
// column's pad and the pad out to the panel's inner width, so no terminal-bg
// island opens inside the panel body. Under the NO_COLOR carve-out (§2.5) the
// canvas and every hue drop and the rows render on the terminal's native fg/bg —
// §9.10 blocks the panel under NO_COLOR outright, so that is the defence rather
// than the daily path.

// themePanelFooterKeyColumnWidth is the fixed width of the left key-glyph column,
// so the labels share a left edge regardless of glyph length — the same
// two-column idiom as helpModalRow, sized for the panel rather than the help body
// (the widest glyph in the panel scope is `esc`; helpKeyColumnWidth's 10 cells is
// nearly half a 24-column panel).
//
// It is a FIXED constant rather than the widest glyph in the entries it is handed,
// deliberately: Phase 9's confirm footer substitutes `y`/`n` into the SAME screen
// position, and a per-slice column would step its labels two cells left as the
// confirm raises and back again as it resolves.
//
// The gap between the column and the label is the horizontal footer's own
// footerKeyLabelGap rather than a second one-space constant — §9.1 puts the two
// footers on one convention, so they read the same gap from one place.
const themePanelFooterKeyColumnWidth = 3

// renderThemePanelFooter renders the §9.1 vertical keymap footer for the given
// entries: the Core entries only, one row per entry, each padded to width cells.
//
// IT TAKES ITS ENTRIES AS A PARAMETER AND NEVER CALLS themePanelKeymap. Phase 9
// adds a nested confirm scope beneath the panel scope (§9.2) whose footer
// TEMPORARILY REPLACES this one — `y confirm` / `n cancel` while the
// slot-from-constant confirm is live, switching back when it resolves — so the
// shape must admit substitution rather than assume one footer per panel. A second
// renderer for the confirm would be the same drift this footer's descriptor
// discipline exists to prevent, one layer down.
//
// width is the panel's INNER content width; rows are padded out to it so the
// canvas covers every cell. A row wider than width is returned unpadded rather
// than truncated or wrapped: the widest row (`d set as dark`) is 15 cells against
// a minimum inner width comfortably above it, and below §9.8's floor the panel
// refuses to open at all (themePanelFloor) — so there is no width at which a footer row
// has to degrade.
func renderThemePanelFooter(entries []keymapEntry, width int, th theme.Theme, colourless bool) string {
	return lipgloss.JoinVertical(lipgloss.Left, themePanelFooterRows(entries, width, th, colourless)...)
}

// themePanelFooterHeight is the rendered height of the vertical footer for the
// given entries — the row budget task 8-6's panel layout subtracts and task 8-11's
// height floor adds, both reading this ONE source rather than a literal four.
//
// It is MEASURED off the rendered block, exactly as sessionFooterHeight is, so the
// reserved rows are by construction the rows that render and the two can never
// drift. The measurement needs neither a width nor a theme: the block is one row
// per Core entry and never wraps (see renderThemePanelFooter), so the row count is
// a function of the entries alone — which is what lets the panel's layout ask for
// the height before it has resolved either.
func themePanelFooterHeight(entries []keymapEntry) int {
	return lipgloss.Height(renderThemePanelFooter(entries, 0, theme.Theme{}, true))
}

// themePanelFooterRows renders one row per Core entry, in descriptor order.
// Non-core entries are dropped — §14.1's distinction applied to the panel: arrows
// and paging are a given in a list and are present in the descriptor for the
// dispatch guard, not for the user's eye.
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
// accent.key within the fixed key column, one canvas gap, then the Action label in
// text.muted — the SAME token split the horizontal footer uses (§9.1's table),
// laid out in the help body's two-column geometry.
//
// The glyph resolves through helpKeyGlyph, so a HelpKey override reads exactly as
// it does in the help body and the panel scope needs no glyph rule of its own.
// The label is the TERSE Action (§14A's pinned copy), never HelpAction — a ~30
// column panel has no room for "Assign to the dark slot".
func themePanelFooterRow(e keymapEntry, width int, th theme.Theme, colourless bool) string {
	key := headerStyle(th.AccentKey, th, colourless).Render(helpKeyGlyph(e))
	keyWidth := lipgloss.Width(key)
	// Pad the key column to its fixed width so the labels share a left edge. The
	// pad is canvas-painted so the column gap is not a terminal-bg island.
	pad := ""
	if keyWidth < themePanelFooterKeyColumnWidth {
		pad = headerCanvasBg(th, colourless).Render(spaces(themePanelFooterKeyColumnWidth - keyWidth))
	}
	gap := headerCanvasBg(th, colourless).Render(footerKeyLabelGap)
	label := headerStyle(th.TextMuted, th, colourless).Render(e.Action)

	row := lipgloss.JoinHorizontal(lipgloss.Top, key, pad, gap, label)
	return headerPadRight(row, lipgloss.Width(row), width, th, colourless)
}
