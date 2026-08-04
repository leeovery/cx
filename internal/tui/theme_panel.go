package tui

import (
	"strings"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// The §9.1 slide-over theme panel: a full-height, right-edge, LEFT-BORDER-ONLY
// block composited over the already-composed page view, with the page beneath
// deliberately NOT re-laid-out.
//
// THE SHAPE IS NOT A PREFERENCE. Portal's modals blank the page to the canvas
// before drawing (modal.go clears to canvas, then placeModalOnClearedCanvas), so a
// modal theme picker would render the canvas plus its own frame and PREVIEW
// NOTHING — and live preview is the entire feature. Non-blanking is the only shape
// that can do the job, and every downstream constraint inherits from that one fact:
// the ~24–30 column budget (§9.8), the four-element row-composition priority
// (§9.5), the message slot's truncation rule (§9.1), and the accepted cost of
// covering three footer entries and cutting a label mid-word.
//
// EVERY PANEL SURFACE RESOLVES TO AN EXISTING TOKEN (§9.1's table): the body is
// `canvas`, the left border and the header rule are `border`, the header label is
// `accent.mode`, the pinned directory row is `accent.attention`. The reference
// frames' `#0C0C16` body on a `#2B3050` border is deliberately NOT ADOPTED — they
// are per-frame literals, and expressing that distinction would need a 20th token
// whose only role is "the panel's background is a bit different from the canvas".
// Resolving every surface to an existing token is what keeps §2.1's colour-literal
// guard and §13.4's swap-and-diff guard satisfied with no carve-out.
//
// THE PANEL DOES NOT ANIMATE. "Slide-over" names the SHAPE — full-height,
// right-edge, left-border-only, as against a floating dialog — not a motion idiom.
// Opening and closing are one frame each, because an animated open would interact
// badly with three pinned behaviours: §11.3's OSC 11 emission would fire repeatedly
// through a canvas-bearing slide, intermediate panel widths would render frames no
// fixture covers, and `t` followed immediately by `Esc` would need a race resolved.

const (
	// themePanelPreferredWidth / themePanelMinWidth are the two ends of §9.8's
	// ~24–30 column ladder (name, markers, slot indicators, border, padding). A
	// FIXED width is predictable to lay out against; a content-driven one would make
	// the panel jump around as the theme library changes.
	//
	// This file only DECLARES the ends. Choosing between them for a given terminal —
	// and refusing below the minimum — is task 8-11's ladder; renderThemePanel
	// renders at whatever width it is handed.
	themePanelPreferredWidth = 30
	themePanelMinWidth       = 24

	// themePanelBorderWidth is the one column the left border occupies. It is NOT
	// list space, which is why the inner content width subtracts it exactly once —
	// there is no top, bottom or right edge to charge for.
	themePanelBorderWidth = 1

	// themePanelHeaderRows is §9.1's fixed two-row header cost: the label row plus
	// the one-row rule. It is a CONSTANT rather than a measurement because §9.8's
	// minimum-height rule resolves against it, and a header that quietly grew a third
	// row would silently move that floor.
	themePanelHeaderRows = 2

	// themePanelHeaderLabel is §9.1's header copy. There is deliberately NO theme
	// count beside it — noise at this list size.
	themePanelHeaderLabel = "Themes"

	// themePanelDirUnreadable is §9.5's pinned chrome copy for an unusable themes
	// directory, verbatim. It is deliberately SHORT — 16 columns — so it fits the
	// panel's minimum width WITHOUT truncation: it is the one row §9.5's composition
	// rules cannot degrade (no label, no badge, no reason, so none of the four
	// priorities apply) and the one that must not become nonsense, being what stands
	// between the user and the "completely in the dark" state. The header's `Themes`
	// label supplies the context the copy drops.
	themePanelDirUnreadable = flashWarningGlyph + " dir unreadable"

	// themePanelMinBodyRows is the one list row §9.8's floor guarantees. Below it the
	// panel refuses to open at all (task 8-11), so the clamp is a floor rather than a
	// degradation step.
	themePanelMinBodyRows = 1
)

// themePanel is the slide-over's state.
//
// ITS list IS THE THIRD `bubbles/list` INSTANCE, and §11.2 names it the WORST CASE
// of the cached-style class: its `bubbles/list`-owned styles (pagination dots, its
// own help/title styles) are assigned ONCE here at construction, while it is the one
// surface whose theme changes on EVERY arrow keypress (§9.11 requires the panel's
// own chrome to re-theme, no exceptions). Those styles are RE-POINTED by task 8-9's
// restyle path — the same path the main list uses — never rebuilt: a rebuild is
// §11.1's expensive path, and it would be worse here, on a per-keypress surface.
//
// The panel's own delegate is the other half of that rule, and it is REPLACED
// rather than re-derived: themeRowDelegate carries its theme, its colourless flag
// and its width as FIELDS, so the delegate is a VALUE the model assembles at the
// single construction point Model.themeRowDelegate and hands to the list — once
// when the panel opens (task 8-7, through newThemePanelList), then RE-POINTED from
// that same site by task 8-9's restyle and task 8-11's resize (SetDelegate, never a
// second construction site).
//
// renderThemePanel takes no Model and never touches the delegate, so the rows are
// drawn with whatever the model last assigned. Keeping that in step with the theme
// the panel is RENDERED at is therefore the MODEL's job, not the renderer's — which
// is why 8-9's restyle re-points the delegate on the same keypress that moves the
// preview.
type themePanel struct {
	// open gates the whole surface. Nothing in this file sets it — `t` does
	// (task 8-7) — so a test drives the composited frame by setting it directly.
	open bool

	// list is the panel's own bubbles/list instance (see the type comment).
	list list.Model

	// enumeration is the ONE directory read, retained for the panel's lifetime
	// (§5.8) so arrowing previews from values already in hand and §9.2's post-commit
	// recompute re-derives with no fresh I/O. openThemePanel fills it; task 8-8's
	// open-time re-resolution and §9.2's commit recompute are its READERS, which is
	// why nothing consults it yet.
	enumeration theme.Enumeration //nolint:unused // retained for the panel's lifetime (§5.8); read by task 8-8

	// union is §9.4's finished row set, already ordered and already carrying each
	// row's single §6.2 reason.
	union theme.Union

	// badges is §9.5's `●` table, keyed by theme.Row.BadgeKey — a fact about the
	// whole SETTING rather than about one row, which is why it is held here and
	// looked up per row rather than derived at the delegate. openThemePanel derives
	// it from the injected slot record and rowItems assembles the list through it.
	badges map[string]theme.Badge

	// message is §9.1's message slot. IT IS ALWAYS EMPTY IN PHASE 8: both of the
	// slot's contenders — the slot-from-constant confirm (§9.2) and the failed-commit
	// line (§9.13) — are commit-path states owned by Phase 9. Phase 8 leaves the
	// slot's height accounted for and the field unset; there is deliberately no
	// setter and no arbiter here.
	message string

	// width is the panel's OUTER width, border column included — the value task
	// 8-11's ladder chooses between themePanelMinWidth and themePanelPreferredWidth.
	width int
}

// newThemePanelList constructs the panel's `bubbles/list` instance.
//
// Every piece of chrome the list can draw for itself is DISABLED, because the panel
// supplies all of it: the §9.1 header (title), the pinned `⚠ dir unreadable` row and
// the message slot (status bar), and the §9.1 vertical keymap footer (help).
// Filtering is off by decision — panel search is deferred (§1.4), which is also why
// themeRowItem.FilterValue is never consulted.
//
// Pagination is deliberately LEFT ON: §9.8 pins overflow to scroll through the
// `bubbles/list` machinery, and §11.2 requires the dots to be exercised by a
// paginating fixture so the swap-and-diff guard is not blind at the new site.
//
// The list is created at zero size. The panel's fixed inner WIDTH is applied once
// at open (applyThemePanelListStyles, which the centred dot row's explicit width
// depends on); the HEIGHT is applied per frame by renderThemePanel, from the height
// it is actually rendered at (see themePanelListSize).
func newThemePanelList(items []list.Item, delegate list.ItemDelegate) list.Model {
	l := list.New(items, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	return l
}

// openThemePanel is §9.6's `t`: the ONE directory read, the parse results
// retained for the panel's lifetime, and the list assembled from the union they
// produced.
//
// THE READ HAPPENS HERE, ON THE KEYPRESS, AND NOWHERE ELSE. §5.7 keeps
// construction free of it — a cold path that is explicitly latency-engineered
// must not become an N-file scan-parse-validate sweep — and §5.8 makes it happen
// on EVERY open rather than once per process, because caching buys nothing
// measurable while breaking the loop the whole drop-in route exists for: copy a
// built-in, edit it, see it, without relaunching Portal.
//
// The KEYS it is handed are the construction-time snapshot and are deliberately
// NOT re-read (§8.4) — see Model.themeKeys for the asymmetry and its reason.
//
// A nil seam is a silent no-op (the modePersister nil-guard precedent), so a
// fixture or capturetool model that wires none simply has no panel.
func (m Model) openThemePanel() (tea.Model, tea.Cmd) {
	if m.themeEnumerator == nil {
		return m, nil
	}

	enumeration, union := m.themeEnumerator.Open(m.themeKeys)
	(&m).armThemePanel(enumeration, union)
	return m, nil
}

// armThemePanel installs one enumeration's results as the live panel state.
//
// The ORDER is load-bearing: the width is set before the list is built, because
// Model.themeRowDelegate composes the row budget from it, and the list is built
// before the styles are applied, because those re-point the list's own chrome.
func (m *Model) armThemePanel(enumeration theme.Enumeration, union theme.Union) {
	m.themePanel = themePanel{
		open:        true,
		enumeration: enumeration,
		union:       union,
		badges:      theme.Badges(m.themeSlots),
		width:       themePanelPreferredWidth,
	}
	m.themePanel.list = newThemePanelList(m.themePanel.rowItems(), m.themeRowDelegate())
	m.applyThemePanelListStyles()
}

// closeThemePanel drops the panel and everything it retained, so the next open
// RE-READS (§5.8) rather than replaying a stale parse.
//
// Zeroing the whole struct is the point rather than a shortcut: the enumeration,
// the union, the badge table and the list are one lifetime, and clearing a subset
// is how a panel comes to show rows from one read and badges from another.
func (m *Model) closeThemePanel() {
	m.themePanel = themePanel{}
}

// rowItems pairs each union row with the badge it carries — the SINGLE item
// assembly site, which task 8-9's restyle re-invokes rather than re-deriving.
//
// The badge is looked up through Row.BadgeKey and NEVER through Slug: a
// `reserved name` row's slug is identical to the built-in's it collides with by
// definition, so a bare Slug lookup would paint `●` on BOTH rows on precisely the
// install that has a drop-in shadowing a built-in.
func (p themePanel) rowItems() []list.Item {
	items := make([]list.Item, 0, len(p.union.Rows))
	for _, row := range p.union.Rows {
		items = append(items, themeRowItem{Row: row, Badge: p.badges[row.BadgeKey()]})
	}
	return items
}

// applyThemePanelListStyles re-points the chrome the panel's `bubbles/list` draws
// FOR ITSELF onto the active theme — today the §3.5 pagination dots, the one such
// surface the panel does not render itself.
//
// §11.2 assigns the panel's chrome to the same restyle path as the main list, and
// the dots are exactly the cached-style class it names: `bubbles/list` reads its
// dot STRINGS out of the styles once, so restyling without re-feeding the
// paginator leaves the library's hardcoded greys rendering under every theme —
// identical before and after a swap, which is invisible to §13.4's swap-and-diff
// guard precisely because nothing changed. The shared canvas/colourless helpers
// are reused verbatim so the panel's dots cannot drift from the two lists'.
//
// The list is SIZED first because the centred dot row pins an explicit width off
// it. Only the width matters here and it is fixed for the panel's lifetime; the
// height is re-applied per frame by renderThemePanel on its own copy.
func (m *Model) applyThemePanelListStyles() {
	m.themePanel.list.SetSize(themePanelInnerWidth(m.themePanel.width), themePanelMinBodyRows)
	if m.colourless {
		colourlessPaginationDots(&m.themePanel.list)
		return
	}
	canvasPaginationDots(&m.themePanel.list, m.activeTheme)
}

// updateThemePanel is §9.7's key-exclusive input routing: the panel OWNS the
// keyboard while it is open.
//
// Pass-through is genuinely bad — `k` would kill the highlighted session while you
// pick a theme, `x` would swap to Projects with the panel open, `m` would start a
// multi-select behind it. None of that reasoning reaches the global quit, and
// swallowing THAT would take away the user's exit key inside a settings surface,
// so `Ctrl-C` stays live exactly as it does under the burst input-lock.
//
// THE `Esc` BODY HERE IS PROVISIONAL. Task 8-10 replaces it with the
// re-resolution close and is the real close path; a bare state clear is correct
// only at this point in the sequence, where no arrow has previewed anything yet
// and so there is nothing to resolve back to. Task 8-9 adds the arrow handling and
// task 8-13 owns the entry conditions and the blocked-`t` flashes.
func (m Model) updateThemePanel(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyIsCtrlC(msg):
		return m, tea.Quit
	case keyIsCode(msg, tea.KeyEscape):
		(&m).closeThemePanel()
		return m, nil
	default:
		return m, nil
	}
}

// themeRowDelegate is THE SINGLE CONSTRUCTION POINT for the panel's row delegate,
// assembling its three inputs from the model: the PREVIEWED theme (m.activeTheme,
// not the persisted one — arrowing re-themes the panel behind the cursor), the
// NO_COLOR carve-out flag, and the panel's current INNER content width.
//
// There must be exactly one place those three are assembled, and a source guard
// keeps it that way. Two construction sites can disagree about width or
// colourlessness, and that disagreement is invisible until a resize during a live
// preview — on the surface §11.2 calls the worst case of the cached-style class.
// Task 8-9's restyle path re-invokes THIS method rather than rebuilding a delegate
// of its own.
func (m Model) themeRowDelegate() themeRowDelegate {
	return themeRowDelegate{
		Theme:      m.activeTheme,
		Colourless: m.colourless,
		Width:      themePanelInnerWidth(m.themePanel.width),
	}
}

// themePanelInnerWidth is the content width inside the left border — every panel
// row is composed against it and the list is sized to it.
func themePanelInnerWidth(width int) int {
	return max(width-themePanelBorderWidth, 0)
}

// themePanelListSize is the (width, height) the panel's list is sized to at a given
// render height: the inner content width, and §9.1's layout remainder —
//
//	height − header(2) − directory row(0 or 1) − message slot(0 or 1) − footer
//
// floored at one row. Three of the four subtrahends are MEASURED off the renderer
// that produces them (themePanelDirRowHeight, themePanelMessageHeight,
// themePanelFooterHeight), so those reserved rows are by construction the rows that
// render and the two can never drift — the same discipline sessionFooterHeight
// applies on the main screen. The fourth, the header, is a pinned constant by design
// (see themePanelHeaderRows).
//
// THE REMAINDER IS NOT MEASURED, AND IT IS NOT TRUSTED EITHER. The list body is the
// one block that can EXCEED the height it is sized to: `bubbles/list` renders a hard
// minimum of three rows however few it is given, so at the low end of this budget it
// overshoots. renderThemePanel therefore CLAMPS it (clampBlockHeight), which is not
// cosmetic — themePanelBlock pads a short assembly out, but a long one it can only
// CUT, and it cuts from the BOTTOM, so an unclamped overshoot comes off the footer.
//
// It is a PURE function of the panel and the height, taking no theme: the row
// COUNTS a block contributes are a function of its content, not of its palette, so
// the layout can be resolved before a theme is in hand (task 8-11's floor
// arithmetic reads the same function).
//
// The footer's entries are the panel scope's, resolved here and again at render
// time. Phase 9's nested confirm scope (§9.2) TEMPORARILY REPLACES that footer with
// a shorter one, so whatever threads the live scope through must thread it through
// BOTH — a budget reserving four rows while a two-row footer renders would leave two
// rows of the panel unaccounted for.
func themePanelListSize(p themePanel, height int) (width, rows int) {
	inner := themePanelInnerWidth(p.width)
	reserved := themePanelHeaderRows +
		themePanelDirRowHeight(p.union.DirUnusable) +
		themePanelMessageHeight(p.message, inner) +
		themePanelFooterHeight(themePanelKeymap())
	return inner, max(height-reserved, themePanelMinBodyRows)
}

// renderThemePanel renders the §9.1 slide-over as a block of EXACTLY height rows,
// each EXACTLY p.width cells, laid out top to bottom as:
//
//	header (2) · directory row (0 or 1) · list body · message slot (0 or 1) · footer
//
// th is the PREVIEWED theme — every chrome surface THIS FUNCTION renders (the
// border, the header, the pinned directory row, the message slot, the footer and
// the canvas backfill) is painted from it per frame, with nothing cached (§9.11).
// The one surface it does NOT reach is the LIST's rows: those are drawn by the
// delegate the model assigned (see themePanel's type comment), so a th that
// disagrees with that delegate's own theme renders themed chrome over stale rows —
// the caller keeps the two in step, this function cannot.
//
// The list is sized HERE, from the height the panel is actually rendered at, so the
// block's total is exact whatever the model's list was last sized to. p is a value,
// so the SetSize lands on this frame's copy and never mutates the model.
func renderThemePanel(p themePanel, height int, th theme.Theme, colourless bool) string {
	inner, bodyRows := themePanelListSize(p, height)
	p.list.SetSize(inner, bodyRows)

	rows := themePanelHeaderBlock(inner, th, colourless)
	rows = appendBlock(rows, themePanelDirRow(p.union.DirUnusable, th, colourless))
	rows = appendBlock(rows, clampBlockHeight(p.list.View(), bodyRows))
	rows = appendBlock(rows, renderThemePanelMessage(p.message, inner, th, colourless))
	rows = appendBlock(rows, renderThemePanelFooter(themePanelKeymap(), inner, th, colourless))

	return themePanelBlock(rows, height, inner, th, colourless)
}

// themePanelHeaderBlock is §9.1's two-row header: the label `Themes` in accent.mode
// (bold), then a one-row `border` rule spanning the inner width — the Sessions
// section-header idiom minus the count.
//
// It carries NO THEME COUNT, deliberately: noise at this list size, and the
// resulting exact two-row cost is what §9.8's minimum-height rule resolves against.
func themePanelHeaderBlock(inner int, th theme.Theme, colourless bool) []string {
	label := headerStyle(th.AccentMode, th, colourless).Bold(true).Render(themePanelHeaderLabel)
	rule := headerStyle(th.Border, th, colourless).Render(strings.Repeat(headerRuleGlyph, max(inner, 0)))
	return []string{label, rule}
}

// themePanelDirRow renders §9.5's `⚠ dir unreadable` warning, or "" when the themes
// directory is usable.
//
// IT IS CHROME PINNED TO THE VIEWPORT, NOT A LIST DELEGATE. A list row participates
// in pagination, so the warning would vanish the moment the user paged down — and
// §9.5 justifies the row as what stands between the user and the "completely in the
// dark" state, which a page-1-only warning does not do. As chrome it is always
// visible and needs no arrow-skip rule. Built-in rows and persisted-slug rows still
// render BENEATH it, the persisted rows especially, or a user with an unreadable
// directory loses the `●` entirely.
//
// The glyph and the text share one accent.attention run (§9.1's table), and the copy
// is never truncated (see themePanelDirUnreadable).
func themePanelDirRow(unusable bool, th theme.Theme, colourless bool) string {
	if !unusable {
		return ""
	}
	return headerStyle(th.AccentAttention, th, colourless).Render(themePanelDirUnreadable)
}

// themePanelDirRowHeight is the directory row's measured contribution to the
// vertical budget — one row while the directory is unusable, zero otherwise.
//
// It is measured off themePanelDirRow itself (with a zero theme and the colourless
// path, exactly as themePanelFooterHeight measures its own block) so the reserved
// row is by construction the row that renders.
func themePanelDirRowHeight(unusable bool) int {
	return blockHeight(themePanelDirRow(unusable, theme.Theme{}, true))
}

// renderThemePanelMessage renders §9.1's message slot: a single row directly above
// the vertical keymap footer, or "" when there is no message.
//
// IT IS NOT RESERVED WHEN EMPTY — it appears and the list shrinks by one, the same
// way the main screen's notice band recomputes list height.
//
// It renders ONE row, truncated rather than wrapped: §9.1 pins truncation at the
// minimum height (the floor counts exactly one message row, and a two-row message
// there would leave zero list rows or overflow the frame), and a slot that is one
// row at every width is the only shape under which the budget above and the block
// below can never disagree.
//
// The slot is a single-slot ARBITER with two contenders, and BOTH ARE PHASE 9's —
// the slot-from-constant confirm (§9.2, text.secondary, no band) and the failed
// commit write (§9.13, `⚠` and text in accent.attention). Phase 8 ships neither, so
// the text renders in the confirm's role and Phase 9 adds the discrimination along
// with the states that need it. There is deliberately no contender, setter or
// arbiter here.
func renderThemePanelMessage(message string, inner int, th theme.Theme, colourless bool) string {
	if message == "" {
		return ""
	}
	return headerStyle(th.TextSecondary, th, colourless).
		Render(ansi.Truncate(message, max(inner, 0), themeRowEllipsis))
}

// themePanelMessageHeight is the message slot's MEASURED height — the value
// themePanelListSize subtracts, so a message appearing costs the list exactly the
// row the slot renders. It measures the real renderer (with a zero theme and the
// colourless path, as themePanelFooterHeight does), so the two cannot drift.
func themePanelMessageHeight(message string, inner int) int {
	return blockHeight(renderThemePanelMessage(message, inner, theme.Theme{}, true))
}

// themePanelBlock assembles the panel's rows into the finished block: every row
// prefixed with the one `border`-coloured `│` cell and padded out to the inner width
// with the owned canvas, clamped and padded to exactly height rows.
//
// LEFT BORDER ONLY — no top, bottom or right edge. That is what makes the panel read
// as a slide-over rather than as an inset bordered dialog like the modals (§9.1), and
// it is the only thing distinguishing the panel from the list behind it.
//
// Rows are padded but never truncated: every row this file composes is authored to
// fit the minimum inner width (the header label is 6 cells, the pinned warning 16,
// the widest footer row 15, and a list row is exactly inner cells by construction),
// and below §9.8's minimum the panel refuses to open at all (task 8-11) — so there
// is no width at which a row has to degrade.
func themePanelBlock(rows []string, height, inner int, th theme.Theme, colourless bool) string {
	border := headerStyle(th.Border, th, colourless).Render(panelFrameSide)
	blank := blankCanvasRow(max(inner, 0), th, colourless)
	painter := newThemePanelPainter(th, colourless)

	out := make([]string, 0, max(height, 0))
	for _, row := range rows {
		if len(out) == height {
			break
		}
		padded := headerPadRight(row, lipgloss.Width(row), inner, th, colourless)
		out = append(out, border+painter.paint(padded))
	}
	for len(out) < height {
		out = append(out, border+blank)
	}
	return strings.Join(out, "\n")
}

// themePanelPainter re-establishes the owned canvas across a panel row's bare
// cells, reusing one parser for the whole block exactly as fillCanvas does.
//
// The panel NEEDS its own backfill because it is composited AFTER the outer
// full-terminal fill (see Model.overlayThemePanelOnContent): the fill's per-line
// backfill has already run by then and can never reach a panel cell. The bare cells
// are real — `bubbles/list` pads its short lines (the filler rows on a part-full
// page, the pagination row's own padding) with unstyled spaces — and left bare they
// would be terminal-bg islands inside the panel, and would additionally be dropped
// as trailing whitespace by the compositor's cell renderer, shortening the block.
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
// COMPOSITE, DO NOT RE-LAY-OUT. base is composed at the UNREDUCED content width and
// the main screen is deliberately NOT re-laid-out while the panel is open. That is
// what keeps the swap the O(1) restyle of §11.1 and keeps the surface being
// previewed from reflowing under the user — the opposite of what a preview wants.
//
// The consequence is accepted rather than worked around: the overlay CUTS WHEREVER
// ITS LEFT BORDER FALLS, mid-label included — `x proje▏`. That is not a violation of
// §14.4's "never truncate a label", which governs how the footer lays ITSELF out as
// the terminal narrows; the panel is an opaque layer over a footer that laid out at
// full width. Reflowing to the reduced width would produce a cleaner edge and was
// rejected for that cost.
//
// What it covers is the right-hand column — the right-side header hint, session-row
// meta, and the right end of the footer including `t theme` itself — which is the
// LEAST theme-informative part of the screen, and therefore exactly what a preview
// surface wants to give up.
func overlayThemePanel(base, panel string, contentW int) string {
	background := lipgloss.NewLayer(base).X(0).Y(0).Z(0)
	foreground := lipgloss.NewLayer(panel).X(max(contentW-lipgloss.Width(panel), 0)).Y(0).Z(1)
	return lipgloss.NewCompositor(background, foreground).Render()
}

// appendBlock appends a rendered block's lines to rows, contributing NOTHING for an
// empty block. It is what makes "not reserved when empty" a property of the assembly
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
// IT EXISTS FOR THE LIST BODY, the one block that can exceed the height it was
// sized to: `bubbles/list` renders a hard minimum of three rows — one item, a blank,
// the paginator — however few it is given, while §9.8's floor budgets the body ONE
// row. Cutting the overflow HERE cuts it off the body, which is where it belongs:
// the rows lost are the paginator and its blank, chrome the panel can spare. Left
// uncut it comes off the BOTTOM of the assembled block instead, where themePanelBlock
// takes it out of the FOOTER — `esc close` first, the one key that closes a panel the
// user can no longer read the way out of.
//
// Raising themePanelMinBodyRows to the list's own minimum was the alternative and is
// refused: paired with the refuse threshold it would need, it silently redefines
// §9.8's floor from one row to three — a spec-facing change §9.8 does not authorise —
// and refuses the panel on terminals where it can still degrade. Degrading the
// paginator keeps the floor literally true and spends chrome rather than the keymap.
func clampBlockHeight(block string, rows int) string {
	if blockHeight(block) <= rows {
		return block
	}
	return strings.Join(strings.Split(block, "\n")[:max(rows, 0)], "\n")
}

// blockHeight is a rendered block's row count, with the empty block contributing
// ZERO rather than lipgloss.Height's 1. It is the measurement half of appendBlock's
// rule, so a budget and the assembly it budgets for always agree.
func blockHeight(block string) int {
	if block == "" {
		return 0
	}
	return lipgloss.Height(block)
}
