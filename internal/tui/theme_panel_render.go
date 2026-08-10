package tui

import (
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// The theme slide-over's render and composite stack: the top-to-bottom assembly of
// the panel's own blocks, the canvas backfill they are laid down on, and the
// z-layer composite that puts the finished block over the page it previews.

// renderThemePanel renders the slide-over as a block of exactly height rows, each
// exactly p.width cells, laid out top to bottom as:
//
//	header (measured) · directory row (0 or 1) · list body · message slot (0 or 1) · footer
//
// th is the previewed theme — every chrome surface this function renders (the
// border, the header, the pinned directory row, the message slot, the footer and
// the canvas backfill) is painted from it per frame, with nothing cached. It does
// not reach the list's rows: those are drawn by the delegate the model assigned,
// so a th that disagrees with that delegate's own theme renders themed chrome
// over stale rows — the caller keeps the two in step, this function cannot.
//
// The list is sized HERE, from the height the panel is actually rendered at, so the
// block's total is exact whatever the model's list was last sized to. p is a value,
// so the SetSize lands on this frame's copy and never mutates the model.
func renderThemePanel(p themePanel, height int, th theme.Theme, colourless bool) string {
	inner, bodyRows := themePanelListSize(p, height)
	p.list.SetSize(inner, bodyRows)

	rows := themePanelHeaderBlock(p.width, th, colourless)
	rows = appendBlock(rows, themePanelDirRow(p.union.DirUnusable, th, colourless))
	rows = appendBlock(rows, clampBlockHeight(p.list.View(), bodyRows))
	rows = appendBlock(rows, renderThemePanelMessage(p.message, inner, themePanelMessageWraps(p, height), th, colourless))
	rows = appendBlock(rows, renderThemePanelFooter(themePanelFooterScope(p.message), inner, th, colourless))

	return themePanelBlock(rows, height, p.width, th, colourless)
}

// themePanelHeaderBlock is the panel's header region, cut to the PAGE's rhythm: a rule
// in the page's own rule lane, the label `Themes` in accent.mode (bold) on the
// page's section-header row, and blank rows everywhere else — one per row the page
// spends on its header block and its section header (see the measurement note above
// themePanelHeaderRuleRow).
//
// It carries no theme count, deliberately: noise at this list size.
//
// The region above the rule carries nothing by decision rather than by omission.
// The panel's body and the page's are painted the same `canvas` token, so those
// blank rows are indistinguishable from the page's own canvas, which lets the
// page's header band read as uninterrupted across the full width.
//
// The rule spans the panel's whole width, border column included, which is why
// the left border starts below it (themePanelBorderFromRow). The panel is an
// opaque layer, so it covers the right end of the page's rule; drawing its own
// across every one of its columns continues that rule to the frame edge. A `│` in
// the rule's lane would notch it, and a border running the full height would cut
// the page's header band in two — which is what makes a slide-over read as a
// second column rather than as a surface inside the content region.
func themePanelHeaderBlock(width int, th theme.Theme, colourless bool) []string {
	rows := make([]string, themePanelHeaderRows())
	rows[themePanelHeaderRuleRow()] = headerStyle(th.Border, th, colourless).
		Render(strings.Repeat(headerRuleGlyph, max(width, 0)))
	rows[themePanelHeaderLabelRow()] = headerStyle(th.AccentMode, th, colourless).Bold(true).
		Render(themePanelHeaderLabel)
	return rows
}

// themePanelDirRow renders the `⚠ dir unreadable` warning, or "" when the themes
// directory is usable.
//
// It is chrome pinned to the viewport rather than a list delegate: a list row
// participates in pagination, so the warning would vanish the moment the user
// paged down. As chrome it is always visible and needs no arrow-skip rule.
// Built-in rows and persisted-slug rows still render beneath it, the persisted
// rows especially, or a user with an unreadable directory loses the `●` entirely.
//
// The glyph and the text share one accent.attention run, and the copy is never
// truncated (see themePanelDirUnreadable).
func themePanelDirRow(unusable bool, th theme.Theme, colourless bool) string {
	if !unusable {
		return ""
	}
	return headerStyle(th.AccentAttention, th, colourless).Render(themePanelDirUnreadable)
}

// themePanelDirRowHeight is the directory row's measured contribution to the
// vertical budget — one row while the directory is unusable, zero otherwise.
//
// It is measured off themePanelDirRow itself (with a zero theme and the
// colourless path, as themePanelFooterHeight measures its own block) so the
// reserved row is by construction the row that renders.
func themePanelDirRowHeight(unusable bool) int {
	return blockHeight(themePanelDirRow(unusable, theme.Theme{}, true))
}

// themePanelBlock assembles the panel's rows into the finished block: each row
// below the header rule prefixed with the one `border`-coloured `│` cell and the
// inner gutter, each row above it laid down bare, every row padded out to exactly
// width cells with the owned canvas, and the whole clamped and padded to exactly
// height rows.
//
// Left border only — no top, bottom or right edge. That is what makes the panel
// read as a slide-over rather than as an inset bordered dialog like the modals,
// and it is the only thing distinguishing the panel from the list behind it.
//
// The border starts below the header rule rather than at the top of the frame,
// which is what makes the slide-over read as a surface inside the content region:
// above the rule the panel contributes nothing but canvas, so the page's header
// band and the rule beneath it run unbroken to the frame edge. A `│` running from
// row 0 cuts that band in two, and the panel then reads as a second column beside
// the page rather than as a layer over it.
//
// The gutter is charged here and at no other renderer, which is what makes it
// uniform: every surface below the rule — the label, the list rows with their
// cursor column, the pinned directory row, the message slot and the vertical key
// list — is composed against themePanelInnerWidth and laid down after the same
// two columns.
//
// Rows are padded but never truncated: every row this file composes is authored
// to fit the minimum inner width (the header label is 6 cells, the pinned warning
// 16, the widest footer row 15, and a list row is exactly inner cells by
// construction), and below the minimum width the panel refuses to open at all.
func themePanelBlock(rows []string, height, width int, th theme.Theme, colourless bool) string {
	inner := themePanelInnerWidth(width)
	prefix := headerStyle(th.Border, th, colourless).Render(panelFrameSide) +
		headerCanvasBg(th, colourless).Render(spaces(themePanelGutterWidth))
	blank := blankCanvasRow(max(inner, 0), th, colourless)
	painter := newThemePanelPainter(th, colourless)
	borderFrom := themePanelBorderFromRow()

	out := make([]string, 0, max(height, 0))
	for _, row := range rows {
		if len(out) == height {
			break
		}
		if len(out) < borderFrom {
			out = append(out, painter.paint(themePanelPadRow(row, width, th, colourless)))
			continue
		}
		out = append(out, prefix+painter.paint(themePanelPadRow(row, inner, th, colourless)))
	}
	for len(out) < height {
		out = append(out, prefix+blank)
	}
	return strings.Join(out, "\n")
}

// themePanelPadRow pads one composed row out to w cells with the owned canvas,
// with an empty row rendered as a whole canvas blank rather than joined against
// one — the header region's unbordered rows carry no content at all, and
// lipgloss's horizontal join has no defined geometry for a zero-width segment.
func themePanelPadRow(row string, w int, th theme.Theme, colourless bool) string {
	if row == "" {
		return blankCanvasRow(max(w, 0), th, colourless)
	}
	return headerPadRight(row, lipgloss.Width(row), w, th, colourless)
}

// themePanelPainter re-establishes the owned canvas across a panel row's bare
// cells, reusing one parser for the whole block exactly as fillCanvas does.
//
// The panel needs its own backfill because it is composited after the outer
// full-terminal fill (see Model.overlayThemePanelOnContent): the fill's per-line
// backfill has already run by then and can never reach a panel cell. The bare
// cells are real — `bubbles/list` pads its short lines with unstyled spaces — and
// left bare they would be terminal-bg islands inside the panel, and would
// additionally be dropped as trailing whitespace by the compositor's cell
// renderer, shortening the block.
//
// Under the NO_COLOR carve-out the zero value is used: canvasBgParams supplies
// nothing to re-establish and backfillCanvasBackground returns the row untouched.
type themePanelPainter struct {
	canvasBg string
	parser   *ansi.Parser
}

func newThemePanelPainter(th theme.Theme, colourless bool) themePanelPainter {
	if colourless {
		return themePanelPainter{}
	}
	return themePanelPainter{
		canvasBg: canvasBgParams(th.Canvas.Color()),
		parser:   ansi.NewParser(),
	}
}

func (p themePanelPainter) paint(row string) string {
	return backfillCanvasBackground(row, p.canvasBg, p.parser)
}

// overlayThemePanel composites the rendered panel over the already-composed page
// view at the content region's right edge, mirroring overlayHelpOnPreview exactly:
// the page is the Z=0 background layer at (0,0) and the panel is the Z=1 foreground
// layer at (contentW − panelWidth, 0), through the lipgloss Compositor's real
// z-layers.
//
// Composite, do not re-lay-out: base is composed at the unreduced content width
// and the main screen is deliberately not re-laid-out while the panel is open.
// That keeps the swap an O(1) restyle and keeps the surface being previewed from
// reflowing under the user.
//
// The consequence is accepted: the overlay cuts wherever its left border falls,
// mid-label included — `x proje▏`. That is not a violation of the "never truncate
// a label" rule, which governs how the footer lays itself out as the terminal
// narrows; the panel is an opaque layer over a footer that laid out at full
// width.
//
// What it covers is the right-hand column — the right-side header hint,
// session-row meta, and the right end of the footer — which is the least
// theme-informative part of the screen.
func overlayThemePanel(base, panel string, contentW int) string {
	background := lipgloss.NewLayer(base).X(0).Y(0).Z(0)
	foreground := lipgloss.NewLayer(panel).X(max(contentW-lipgloss.Width(panel), 0)).Y(0).Z(1)
	return lipgloss.NewCompositor(background, foreground).Render()
}

// appendBlock appends a rendered block's lines to rows, contributing nothing for
// an empty block. It makes "not reserved when empty" a property of the assembly
// rather than a branch at each optional row: lipgloss.Height("") is 1, so a naive
// split would give an empty slot a row it was never budgeted.
func appendBlock(rows []string, block string) []string {
	if block == "" {
		return rows
	}
	return append(rows, strings.Split(block, "\n")...)
}

// clampBlockHeight cuts a rendered block down to at most rows lines, returning it
// untouched when it already fits.
//
// It exists for the list body, the one block that can exceed the height it was
// sized to: `bubbles/list` renders a hard minimum of three rows — one item, a
// blank, the paginator — however few it is given, while the render floor budgets
// the body one row. Cutting the overflow here takes it off the body, where the
// rows lost are the paginator and its blank. Left uncut it comes off the bottom of
// the assembled block instead, where themePanelBlock takes it out of the footer —
// `esc close` first, the one key that closes a panel the user can no longer read
// the way out of.
//
// Raising themePanelMinBodyRows to the list's own minimum would silently redefine
// the render floor from one row to three and refuse the panel on terminals where
// it can still degrade; degrading the paginator spends chrome rather than the
// keymap.
func clampBlockHeight(block string, rows int) string {
	if blockHeight(block) <= rows {
		return block
	}
	return strings.Join(strings.Split(block, "\n")[:max(rows, 0)], "\n")
}

// blockHeight is a rendered block's row count, with the empty block contributing
// zero rather than lipgloss.Height's 1. It is the measurement half of
// appendBlock's rule, so a budget and the assembly it budgets for always agree.
func blockHeight(block string) int {
	if block == "" {
		return 0
	}
	return lipgloss.Height(block)
}
