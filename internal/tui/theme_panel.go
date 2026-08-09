package tui

import (
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// The slide-over theme panel: a full-height, right-edge, left-border-only block
// composited over the already-composed page view, with the page beneath
// deliberately not re-laid-out.
//
// The shape follows from live preview. Portal's modals blank the page to the
// canvas before drawing, so a modal theme picker would render the canvas plus its
// own frame and preview nothing. Every downstream constraint inherits from that:
// the ~27–34 column budget, the row-composition priority, the message slot's
// truncation rule, and the accepted cost of covering three footer entries and
// cutting a label mid-word.
//
// Every panel surface resolves to an existing token: the body is `canvas`, the
// left border and the header rule are `border`, the header label is
// `accent.mode`, the pinned directory row is `accent.attention`. That is what
// keeps the colour-literal guard and the swap-and-diff guard satisfied with no
// carve-out.
//
// The panel does not animate. "Slide-over" names the shape, not a motion idiom.
// Opening and closing are one frame each: an animated open would re-emit OSC 11
// repeatedly through a canvas-bearing slide, render intermediate widths no
// fixture covers, and make `t` immediately followed by `Esc` a race.

const (
	// themePanelPreferredWidth / themePanelMinWidth are the two ends of the column
	// ladder (name, markers, slot indicators, border, gutter, padding). A fixed
	// width is predictable to lay out against; a content-driven one would make the
	// panel jump around as the theme library changes.
	//
	// themePanelWidthFor chooses between them for a given terminal, and
	// themePanelFloor refuses below the minimum; renderThemePanel renders at
	// whatever width it is handed.
	themePanelPreferredWidth = 34
	themePanelMinWidth       = 27

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

	// themePanelHeaderLabel is the panel's header copy. There is deliberately no
	// theme count beside it — noise at this list size.
	themePanelHeaderLabel = "Themes"

	// themePanelDirUnreadable is the pinned chrome copy for an unusable themes
	// directory. It is deliberately short — 16 columns — so it fits the panel's
	// minimum width without truncation: none of the row-composition priorities
	// apply to it, and it is what stands between the user and a themes directory
	// silently yielding nothing. The header's `Themes` label supplies the context
	// the copy drops.
	themePanelDirUnreadable = flashWarningGlyph + " dir unreadable"

	// The four badge texts, pinned user-facing copy. Which badge a row carries is a
	// fact about the setting and is derived in internal/theme (theme.Badges); the
	// words it is drawn with are the panel's.
	//
	// `● both` is deliberately no wider than `● light`, so the collapsed form
	// cannot move the row-composition truncation budget and steal columns from the
	// label.
	themePanelBadgeConstant = "●"
	themePanelBadgeLight    = "● light"
	themePanelBadgeDark     = "● dark"
	themePanelBadgeBoth     = "● both"

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

const (
	// themePanelNarrowClosedFlash / themePanelShortClosedFlash are the pinned
	// forced-close copy — the resize half of the pair the entry strings below
	// belong to, kept side by side so neither can be re-worded at a call site.
	themePanelNarrowClosedFlash = "terminal too narrow — theme picker closed"
	themePanelShortClosedFlash  = "terminal too short — theme picker closed"

	// themePanelNoColorFlash / themePanelNarrowEntryFlash /
	// themePanelShortEntryFlash are the pinned blocked-entry copy — the three
	// strings the entry gate can answer with. They read differently from their
	// forced-close siblings on purpose: one pair reports why nothing opened, the
	// other why something the user was looking at went away.
	themePanelNoColorFlash     = "theme picker needs colour — NO_COLOR is set"
	themePanelNarrowEntryFlash = "terminal too narrow for the theme picker"
	themePanelShortEntryFlash  = "terminal too short for the theme picker"

	// themeNotSavedFlash is the close-report copy: what the user is left reading on
	// the main screen when the panel closes with a failed commit outstanding.
	//
	// It carries no `⚠` of its own. It is raised into the notice band, whose
	// warning role prepends the status glyph — as it does for the five siblings
	// above — so a glyph in the copy would render two. Contrast the panel's own
	// failed-commit line (themePanelCommitFailedMessage), which keeps its glyph
	// because the panel's message slot adds none.
	//
	// It names the log rather than a retry, because the panel is gone by the time
	// it is read and `theme: commit failed` is already written there.
	themeNotSavedFlash = "theme not saved — see portal.log"
)

// themePanelWidthFor is the width ladder: the panel's outer width for a given
// content-region width, and whether the region can hold a panel at all.
//
// The panel takes half the content region, clamped to the [minimum, preferred]
// ends declared above. The half-width cap keeps the previewed page visible while
// the terminal is wide enough to afford it; the clamp turns that into a staged
// shrink rather than a proportional one that would shrink forever.
//
// The width is clamped on the refusing path too. Callers take w and ignore ok
// because themePanelFloor has already refused by the time either runs; returning
// an unclamped w would make an impossible state render a sub-minimum panel
// instead of degrading to the minimum.
func themePanelWidthFor(contentW int) (w int, ok bool) {
	return min(max(contentW/2, themePanelMinWidth), themePanelPreferredWidth), contentW >= themePanelMinWidth
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

// themePanelForcedCloseFlash is the forced-close copy for the dimension the
// floor refused on.
//
// dimNone is impossible here — the caller reaches this only on a refusal — and it
// degrades to the width copy rather than branching on an unreachable state, the
// same way themePanelWidthFor clamps a refusing width.
func themePanelForcedCloseFlash(dim themePanelDim) string {
	if dim == dimHeight {
		return themePanelShortClosedFlash
	}
	return themePanelNarrowClosedFlash
}

// themePanelEntryFlash is the blocked-entry copy for the dimension the floor
// refused on — the entry-side mirror of themePanelForcedCloseFlash, degrading to
// the width copy on the unreachable dimNone for the same reason.
func themePanelEntryFlash(dim themePanelDim) string {
	if dim == dimHeight {
		return themePanelShortEntryFlash
	}
	return themePanelNarrowEntryFlash
}

// themePanel is the slide-over's state.
//
// Its list is the worst case of the cached-style class: its
// `bubbles/list`-owned styles (pagination dots, help/title styles) are assigned
// once at construction, while it is the one surface whose theme changes on every
// arrow keypress. Those styles are re-pointed by the restyle path — the same path
// the main list uses — never rebuilt.
//
// The panel's delegate is replaced rather than re-derived: themeRowDelegate
// carries its theme, colourless flag and width as fields, so it is a value the
// model assembles at the single construction point Model.themeRowDelegate, handed
// to the list at open and re-pointed from that same site by the restyle path and
// by resizeThemePanel.
//
// renderThemePanel takes no Model and never touches the delegate, so the rows are
// drawn with whatever the model last assigned. Keeping that in step with the
// theme the panel is rendered at is therefore the model's job, which is why the
// restyle re-points the delegate on the same keypress that moves the preview.
type themePanel struct {
	// open gates the whole surface, and it means "the panel is live" rather than
	// "an open was requested": armThemePanel sets it only once the list below
	// exists, because the restyle path's panel arm keys off it and would otherwise
	// run against a zero list.Model mid-arm. See armThemePanel's ordering note.
	open bool

	// list is the panel's own bubbles/list instance (see the type comment).
	list list.Model

	// enumeration is the one directory read, retained for the panel's lifetime so
	// arrowing previews from values already in hand and the post-commit recompute
	// re-derives with no fresh I/O. openThemePanel fills it; the open-time
	// re-resolution, the commit recompute and `Esc`'s final re-resolution read it.
	enumeration theme.Enumeration

	// union is the finished row set, already ordered and already carrying each
	// row's single rejection reason.
	union theme.Union

	// badges is the `●` table, keyed by theme.Row.BadgeKey — a fact about the
	// whole setting rather than about one row, which is why it is held here and
	// looked up per row rather than derived at the delegate. It is derived at open
	// from the seam's re-resolution against the retained enumeration — the panel's
	// parse, never construction's — and rowItems assembles the list through it.
	//
	// The post-commit recompute re-derives it against that same enumeration
	// (applyCommittedSetting), which is what makes the `●` mean "what is
	// persisted" after a write as well as before one. A failed commit reaches
	// neither writer, so the marker cannot move on a write that did not land.
	badges map[string]theme.Badge

	// message is the panel's message slot: a single-slot arbiter holding at most
	// one of its two contenders — the slot-from-constant confirm and the
	// failed-commit line. The two can never be live at once, because a confirm
	// resolves before any write happens; theme_panel_message.go holds the value,
	// its pinned copy, its renderer and the writers that install it.
	//
	// `Enter` raises no confirm even over a pair: it visibly does what it says, and
	// the theme is already previewing behind the panel. The asymmetry with `d`/`l`
	// is the point — the confirm guards the case where the resolved theme changes
	// as a side effect of a write the user was told is inert.
	message themePanelMessage

	// pending is the assignment a live confirm will apply on `y` — the slug the
	// cursor was on when the question was asked, and the slot the keypress named.
	pending themeSlotConfirm

	// width is the panel's outer width, border column included — the value
	// themePanelWidthFor chooses between the minimum and preferred widths.
	width int
}

// newThemePanelList constructs the panel's `bubbles/list` instance.
//
// Every piece of chrome the list can draw for itself is disabled, because the
// panel supplies all of it: the header (title), the pinned `⚠ dir unreadable` row
// and the message slot (status bar), and the vertical keymap footer (help).
// Filtering is off, so themeRowItem.FilterValue is never consulted.
//
// Pagination is deliberately left on: overflow scrolls through the
// `bubbles/list` machinery, and a paginating fixture exercises the dots so the
// swap-and-diff guard is not blind at that site.
//
// The nav keymap is pinned through the shared pinArrowOnlyNav, as the Sessions
// and Projects lists pin theirs: navigation is arrows only, and the v2
// DefaultKeyMap re-introduces the vim aliases plus PgUp/PgDn/Home/End/b/u/f/d.
// Two of those collide with the panel's own commit keys — the default binds `l`
// and `d` to NextPage — so pinning here keeps a banned key from ever reaching the
// list's own Update.
//
// The list is created at zero size and sized at open by applyThemePanelListStyles,
// which the centred dot row's explicit width depends on. renderThemePanel
// re-applies the same size per frame on its own copy, from the height it is
// actually rendered at, so the model's page is the drawn page and `Ctrl+↑`/`Ctrl+↓`
// move a page rather than a row.
func newThemePanelList(items []list.Item, delegate list.ItemDelegate) list.Model {
	l := list.New(items, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	pinArrowOnlyNav(&l.KeyMap)
	return l
}

// themePanelEntry is the panel's entry gate: the pre-read decision to open the
// panel, answering either "open" or the pinned copy the refusal flashes.
//
// `NO_COLOR` and the render floor are decided here. The other things that block
// `t` — a modal, a pending burst, and the pages that bind no `t` — are decided by
// where the key is dispatched from and produce no flash. That split is the
// feedback rule: flash where the key is bound and the user could reasonably
// expect it to work, silent where it is not bound at all, as `s` already behaves.
//
// The two refusals below are opposite calls:
//
//   - `NO_COLOR` is a capability absence — the panel previews nothing and a
//     commit would persist a choice with zero visible feedback — so it is blocked
//     proactively, keeping the user out of a walkable dead end.
//   - A narrow or short terminal is a space shortage, where the rule is degrade:
//     the panel shrinks through the width ladder and refuses only once even the
//     minimum cannot render.
//
// It reads themePanelFloor — the same predicate the resize condition reads — with
// dirUnusable false, because that flag is a product of the enumeration and the
// read happens on this keypress, after this decision. openThemePanel re-applies
// the same predicate with the real flag.
func (m Model) themePanelEntry() (blockedFlash string, ok bool) {
	if m.colourless {
		return themePanelNoColorFlash, false
	}
	if dim, fits := themePanelFloor(m.contentWidth(), m.contentHeight(), false); !fits {
		return themePanelEntryFlash(dim), false
	}
	return "", true
}

// handleThemePanelKey is the `t` handler both pages' dispatch arms call: the gate
// above decides, and this either opens or raises the refusal's copy.
//
// Sessions and Projects must answer `t` identically — theme is a global setting —
// and a page consulting the gate for itself is how one page comes to flash where
// the other opens. Both pages raise the flash through their own band with no
// branch here, because the band is arbitrated per page from the same m.flashText.
func (m Model) handleThemePanelKey() (tea.Model, tea.Cmd) {
	flash, ok := m.themePanelEntry()
	if !ok {
		return m.blockThemePanel(flash)
	}
	return m.openThemePanel()
}

// blockThemePanel raises a blocked-`t` flash and schedules its auto-clear — the
// site both refusals (the pre-read gate, the post-read floor re-evaluation) report
// through.
//
// The flash is an ordinary transient one with no bespoke lifecycle: the post-bump
// generation feeds flashTickCmd so the block inherits the standard timer, as the
// proactive multi-select block does, with the next-actionable-key clear at the top
// of each page's update remaining the authoritative route.
//
// It is raised through setThemeFlash, which gives it precedence over the filter
// line. That matters for the `NO_COLOR` refusal: it is a proactive block, so a
// flash that could not claim the slot would produce nothing at all — the walkable
// dead end the block exists to prevent, reached by another route.
func (m Model) blockThemePanel(flash string) (tea.Model, tea.Cmd) {
	(&m).setThemeFlash(flash)
	return m, flashTickCmd(m.flashGen)
}

// openThemePanel is what `t` runs: the directory read, the parse results retained
// for the panel's lifetime, the list assembled from the union they produced, and
// the opening state resolved against them (armThemePanel).
//
// The read happens on the keypress rather than at construction, so a
// latency-engineered cold path does not become an N-file scan-parse-validate
// sweep, and it re-runs on every open rather than caching, which is what makes
// the drop-in loop work: copy a built-in, edit it, see it, without relaunching
// Portal. The keys it is handed are the construction-time snapshot and are
// deliberately not re-read — see themeState.keys. A nil seam is a silent no-op.
//
// The floor is re-evaluated here against the same predicate the entry gate read,
// now with the real directory verdict: the pinned `⚠ dir unreadable` row raises
// the height floor by one, and DirUnusable does not exist until the enumeration
// runs. Neither shortcut is available — assuming the row present at the gate
// would refuse terminals that render a perfectly good panel, and assuming it
// absent would open a panel whose list body is zero rows.
//
// A refusal on this path has already read the directory and emitted
// `theme: enumerated`, which is honest because the enumeration happened. The
// enumeration is discarded and the same pinned copy the gate would have raised is
// flashed, selected through the shared themePanelEntryFlash so the two
// evaluations' copy is identical.
func (m Model) openThemePanel() (tea.Model, tea.Cmd) {
	if m.themeState.enumerator == nil {
		return m, nil
	}

	enumeration, union := m.themeState.enumerator.Open(m.themeState.keys)
	if dim, fits := themePanelFloor(m.contentWidth(), m.contentHeight(), union.DirUnusable); !fits {
		return m.blockThemePanel(themePanelEntryFlash(dim))
	}
	(&m).armThemePanel(enumeration, union)
	return m, nil
}

// armThemePanel installs one enumeration's results as the live panel state, and
// the opening state with them: the badges, the theme actually rendering, and the
// row the cursor lands on.
//
// The width comes from the ladder, not from the preferred width, so the width a
// panel opens at and the width it degrades to on a resize are the same function
// of the same input. The `ok` return is unreachable here — the entry gate already
// refused below the floor — and themePanelWidthFor clamps to the minimum on that
// impossible path.
//
// The order is load-bearing at five points. The width is set before the list is
// built, because Model.themeRowDelegate composes the row budget from it. The
// resolution runs before the list is built, because it decides both the badges
// every row item carries and the palette that delegate is constructed from — a
// list built first would render the previous theme's rows behind the new canvas.
// The list is built before the styles are applied, because those re-point the
// list's own chrome. And the cursor is anchored last, because applying the styles
// re-sizes the list, which re-derives its page from the index it finds.
//
// The fifth is `open`, set only once the list exists. The resolution above
// applies the in-force theme through Model.ApplyTheme, whose restyle path
// re-points the panel list's own styles and delegate while the panel is open
// (applyThemePanelCanvasMode) — so a panel marked open before its list is built
// would have that path run against the zero list.Model. Skipping that re-point
// during the arm is also redundant: the list is constructed with a delegate taken
// from the palette the resolution just applied, and applyThemePanelListStyles
// re-points its chrome on the next line.
//
// The capture-only cursor seed runs after the resolution's anchor and before the
// message seed, and it moves the cursor and nothing else. It re-anchors by row
// identity through the same anchor the resolution's answer went through, so a
// fixture cannot seed a cursor onto a row the union does not hold, and it applies
// no theme.
func (m *Model) armThemePanel(enumeration theme.Enumeration, union theme.Union) {
	width, _ := themePanelWidthFor(m.contentWidth())
	m.themePanel = themePanel{
		enumeration: enumeration,
		union:       union,
		width:       width,
	}
	cursor := m.applyThemePanelResolution(enumeration)
	m.themePanel.list = newThemePanelList(m.themePanel.rowItems(), m.themeRowDelegate())
	m.themePanel.open = true
	m.applyThemePanelListStyles()
	m.anchorThemePanelCursor(cursor)
	m.anchorThemePanelCursor(m.themeState.initialCursor)
	m.seedThemePanelMessage()
}

// seedThemePanelMessage installs the capture-only MESSAGE STATE on a panel that
// has just opened — the offline harness's only route to the message slot, which is
// otherwise reached by a keypress that a one-shot render never makes.
//
// It must run after the cursor seed. The confirm records the slug under the
// cursor as the assignment it would apply, so a seed that ran first would name
// whichever row the open happened to anchor; and raising a message re-derives the
// panel's vertical budget, which re-sizes the list from the index it finds.
//
// Each seed routes through the production writer, so the copy is composed from
// the message slot's own pinned constants and a fixture cannot ship a paraphrase.
// The two are alternatives rather than a pair: the slot is a single-slot arbiter.
//
// The confirm's slot is light, which is immaterial to the frame: the question
// names the constant being cleared rather than the half being written.
func (m *Model) seedThemePanelMessage() {
	switch {
	case m.themeState.initialConfirm:
		if slug, ok := committableThemeSlug(m.themePanel.list); ok {
			m.raiseSlotConfirm(slug, theme.MemberLight)
		}
	case m.themeState.initialCommitFailed:
		m.reportCommitFailure()
	}
}

// applyInForceTheme is the panel's re-resolution, and the whole of what the
// panel's open and its close have in common: the persisted setting taken from the
// model's raw keys, resolved against the retained enumeration, the one member in
// force selected from the answer, and that member applied through
// Model.ApplyTheme.
//
// The degrade policy below governs every panel call site of Resolve — this open,
// `Esc`'s close and the commit recompute — so it is stated here rather than at
// each site. Only the open and the close share the body: a commit recomputes rows
// and badges and never the rendered theme, so routing it through here would swap
// the screen off the preview the user is still looking at.
//
// It resolves against the retained parse and never the filesystem: a read here
// would produce a third parse of the same slug, free to disagree with the rows
// the user is looking at. The light/dark answer is likewise read off the model —
// the gate resolves exactly once.
//
// The only error Resolve can return is the broken-builtin fatal, unreachable in a
// correctly built binary. The panel degrades rather than escalating: nothing is
// applied and nothing is written, because a settings surface must not become the
// route by which a broken binary quits Portal mid-session. A resolution naming no
// slot at all takes the same path, which keeps a fixture that returns one from
// painting a colourless picker.
//
// The seam is always live here: openThemePanel's nil guard is what makes the
// panel openable at all.
//
// A theme already painting the screen is skipped — explicitness rather than
// necessity, since ApplyTheme is idempotent per swap, and it makes the common
// close cost no restyle.
func (m *Model) applyInForceTheme(e theme.Enumeration) (theme.Resolution, theme.SlotResolution, bool) {
	resolution, err := m.themeState.enumerator.Resolve(e, m.themeSetting())
	if err != nil {
		return theme.Resolution{}, theme.SlotResolution{}, false
	}
	inForce, ok := inForceSlot(resolution, m.themeState.canvasMode)
	if !ok {
		return theme.Resolution{}, theme.SlotResolution{}, false
	}

	if inForce.Theme != m.themeState.active {
		m.ApplyTheme(inForce.Theme)
	}
	return resolution, inForce, true
}

// applyThemePanelResolution is what makes "the cursor lands on the theme that is
// actually rendering": the re-resolution above, plus the two things only the OPEN
// needs from it — the badge table refreshed from the answer, and the row identity
// the cursor belongs on.
//
// Opening is not a passive read: the panel's parse supersedes the
// construction-time one, so this is where a mid-session edit lands. An
// edited-but-still-valid active theme re-renders with its new values, and one
// that has been invalidated flips to the mode-matched fallback here rather than
// at `Esc` — deferring would leave the panel listing a theme as invalid while the
// screen still renders it. The mirror case falls out of the same call: a theme
// that was broken at construction becomes loadable the moment the user fixes the
// file.
//
// The empty string is the degrade path's identity, which anchorThemePanelCursor
// reads as "leave the cursor where it is". It is unambiguous: the anchored slug
// charset makes an empty slug illegal, so a resolved slot can never carry one.
// Badges are left untouched on that path for the same reason the theme is.
func (m *Model) applyThemePanelResolution(e theme.Enumeration) string {
	resolution, inForce, ok := m.applyInForceTheme(e)
	if !ok {
		return ""
	}

	m.themePanel.badges = theme.Badges(resolution.Slots)
	return inForce.Resolved
}

// themeSetting collapses the model's construction-time raw keys onto the
// two-state setting.
//
// It routes through ResolveSetting rather than restating the tiebreak — the same
// site the union assembly and doctor's line resolve through — so what the panel
// lists, what it marks and what it resolves cannot disagree about which slug is
// live. Re-running it on already-stripped keys is safe: stripping is idempotent
// and the resolution is pure and total.
func (m Model) themeSetting() theme.Setting {
	setting, _ := theme.ResolveSetting(m.themeState.keys.Theme, m.themeState.keys.Light, m.themeState.keys.Dark)
	return setting
}

// inForceSlot is the slot painting the screen: the constant under a constant, and
// under a pair the member the light/dark answer names — light in a light
// terminal, dark otherwise.
//
// The answer is read off the model rather than asked for again: a constant never
// consulted detection at all and a pair resolved exactly once before first paint,
// so a query here would reopen the race the resolve-once rule closes.
//
// On a gate that was never armed the value starts as the standing dark no-answer
// fallback — until the mid-session constant → adaptive conversion, which records
// Model.retainedCanvasAnswer into themeState.canvasMode while the pinned gate
// keeps that fallback for good. From that commit on the mode read here must not
// be gate.appearance: substituting one for the other closes a converted light
// terminal onto the dark slot. See themeState.canvasMode and loadNewlyLiveSlot.
//
// The answer reaches the slot vocabulary through the gate's own conversion rather
// than a light/dark rule restated here, so the slot this matches on and the
// member the active palette is selected by can never be two different readings of
// one answer.
//
// The slot is matched on its Slot rather than taken by position, and one record
// answers for both the palette and the cursor's target, so the theme applied and
// the row anchored cannot be two lookups that disagree.
//
// The false return is a resolution naming no slot at all — not a state the
// resolver produces, but a shape a fixture can hand back, and the caller degrades
// on it rather than selecting a zero Theme.
func inForceSlot(r theme.Resolution, mode canvasAppearance) (theme.SlotResolution, bool) {
	want := mode.member().Slot()
	for _, slot := range r.Slots {
		if slot.Slot == theme.SlotConstant || slot.Slot == want {
			return slot, true
		}
	}
	return theme.SlotResolution{}, false
}

// anchorThemePanelCursor puts the cursor on the row whose IDENTITY is slug.
//
// Anchoring is by identity and never by index. An index silently breaks the
// invariant the moment a row is inserted above the cursor — the screen keeps
// previewing one theme while the cursor sits on another — which is what the
// commit recompute does, since clearing a slot can remove a row and assigning one
// can mint a new row above the cursor.
//
// The target is the resolved slug, never the requested one: under a fallback
// those differ, and the fallback's row is the one that is painted. The persisted
// row keeps its `●` and is unselectable, so parking the cursor there would put it
// somewhere the arrows cannot return to.
//
// An empty slug is the degrade path's no-op.
func (m *Model) anchorThemePanelCursor(slug string) {
	if slug == "" {
		return
	}
	m.themePanel.list.Select(themePanelRowIndex(m.themePanel.union.Rows, slug))
}

// themePanelRowIndex is the index of the selectable row identified by slug,
// degrading to the first selectable row and then to zero.
//
// The identity is Row.SortKey — the slug wherever one exists, else the filename,
// else the raw persisted string — the same value the ordering and the badge
// lookup key on, so a row can be found by exactly what it is listed under. A
// `reserved name` row shares its key with the built-in it collides with by
// definition, and the built-in sorts first, so the first match is always the
// selectable one the slug actually resolved to.
//
// The Selectable filter is not redundant with that order. The capture seed
// (themeState.initialCursor) re-anchors by identity from a string a fixture
// declares, and fixtures are built from an invalid drop-in and an unreadable
// themes directory — whose rows are exactly the unselectable ones. A cursor
// parked there would sit somewhere the arrows, which skip unselectable rows,
// cannot return to; falling through is the same degrade a seed naming no row at
// all takes.
//
// The final clamp is a structural guard rather than a live path — built-ins are
// always valid, so a union with no selectable row is unreachable — but the
// alternative to degrading is indexing out of range inside a list the user is
// looking at.
func themePanelRowIndex(rows []theme.Row, slug string) int {
	identified := func(row theme.Row) bool { return row.SortKey() == slug && row.Selectable() }
	if at := slices.IndexFunc(rows, identified); at >= 0 {
		return at
	}
	return max(slices.IndexFunc(rows, theme.Row.Selectable), 0)
}

// closeThemePanel is `Esc`: it discards an uncommitted preview, renders the
// resolved persisted state, and then drops everything the panel retained so the
// next open re-reads rather than replaying a stale parse.
//
// It deliberately does not restore a theme snapshotted at open, and must not be
// "simplified" into one — that is wrong in both directions. Backwards: a user who
// broke their active theme's file mid-session would be handed back a palette the
// config no longer yields. Forwards: a commit writes prefs and leaves the panel
// open, so an `Esc` after one must resolve to the newly persisted state.
//
// Order matters: the resolution reads the retained enumeration, so the discard is
// last. Zeroing the whole struct is the point — the enumeration, the union, the
// badge table, the message and the list are one lifetime, and clearing a subset
// is how a panel comes to show rows from one read and badges from another.
//
// The page beneath needs no re-layout because it was never reduced on open.
// Adding a reclaim step here is one step from adding the open-time reduction that
// would justify it, which reflows the surface being previewed.
//
// Nothing is written — no prefs write, no tmux option, no file — which eliminates
// the "applied but not persisted" state persist-on-close would reach.
func (m *Model) closeThemePanel() tea.Cmd {
	m.applyInForceTheme(m.themePanel.enumeration)
	m.themePanel = themePanel{}
	return m.reportOutstandingCommitFailure()
}

// reportOutstandingCommitFailure is the close report: with a failed commit
// outstanding, the close raises the main-screen flash and clears the state in the
// same act.
//
// The report must survive the close. `Esc` re-resolves persisted state, so
// without this step the next keypress both takes the panel's message down and
// drops the theme the user chose — with no `●` movement to signal it, since the
// marker must not move on a write that did not land.
//
// Raising it discharges the state, which is the report the state exists to
// produce: without the discharge, every subsequent close would re-fire a flash
// about a failure already reported.
//
// It goes through setThemeFlash for precedence over a filter line, which would
// otherwise keep this report off the band entirely — destroying it rather than
// deferring it, since the discharge happens whether or not a band rendered.
func (m *Model) reportOutstandingCommitFailure() tea.Cmd {
	if !m.themeState.commitFailed {
		return nil
	}
	m.setThemeFlash(themeNotSavedFlash)
	m.themeState.commitFailed = false
	return flashTickCmd(m.flashGen)
}

// resizeThemePanel is the resize condition, the second of themePanelFloor's
// callers: while the terminal stays above the render floor the panel degrades in
// place, and below either dimension's floor it force-closes with the pinned
// per-dimension copy.
//
// Degrading in place is three things, and the third is easy to miss: the ladder
// re-runs, the body arithmetic re-runs (a stale PerPage makes `Ctrl+↑`/`Ctrl+↓`
// move a different distance than the screen scrolls, which no rendered frame
// reveals), and the panel's delegate is re-pointed. The delegate matters because
// it holds its composition budget as a field, unlike SessionDelegate, which reads
// the width off the list it renders into; a resize with no arrow after it would
// otherwise leave every row composing against the pre-resize budget. All three
// come from re-invoking applyThemePanelListStyles, the function the open runs.
//
// The main screen is deliberately not re-laid-out to the reduced width, so a
// panel width change never reflows the surface being previewed.
//
// The forced close takes the `Esc` path exactly — closeThemePanel, never a second
// teardown — and then raises its geometry flash if that close has not already
// spoken. Any other behaviour strands the user rendering a theme they never chose
// with the surface that could change it gone.
//
// With a commit failure outstanding both flashes are due at once and the report
// wins: losing the geometry flash costs nothing, while losing the report on the
// one path where the user cannot reopen the panel to retry is the failure the
// report closes. Order matters — the flag is read before the close, because the
// close's own report step discharges it as part of raising the flash, so a
// post-close read always sees false and would overwrite the report just placed in
// the single-slot band.
//
// A live slot-from-constant confirm is silently cancelled here — its one exit
// other than a keypress. Nothing has been written at that point, and the cancel
// is structural: the question and the pending assignment both live on the panel
// struct, which closeThemePanel discards whole.
//
// The geometry flash carries no auto-clear tick and clears on the next actionable
// key; the failed-commit report brings its own tick, the command returned here.
func (m *Model) resizeThemePanel() tea.Cmd {
	if !m.themePanel.open {
		return nil
	}
	if dim, ok := themePanelFloor(m.contentWidth(), m.contentHeight(), m.themePanel.union.DirUnusable); !ok {
		// Order matters: the read precedes the close, which discharges the flag as it
		// reports. Reading it afterwards always yields false.
		willReport := m.themeState.commitFailed
		cmd := m.closeThemePanel()
		if !willReport {
			m.setThemeFlash(themePanelForcedCloseFlash(dim))
		}
		return cmd
	}
	m.themePanel.width, _ = themePanelWidthFor(m.contentWidth())
	m.applyThemePanelListStyles()
	return nil
}

// rowItems pairs each union row with the badge it carries — the item assembly
// site, which the restyle path re-invokes rather than re-deriving.
//
// The badge is looked up through Row.BadgeKey and never through Slug: a
// `reserved name` row's slug is identical to the built-in's it collides with by
// definition, so a bare Slug lookup would paint `●` on both rows on precisely the
// install that has a drop-in shadowing a built-in.
func (p themePanel) rowItems() []list.Item {
	items := make([]list.Item, 0, len(p.union.Rows))
	for _, row := range p.union.Rows {
		items = append(items, themeRowItem{Row: row, Badge: p.badges[row.BadgeKey()]})
	}
	return items
}

// applyThemePanelListStyles sizes the panel's list and re-points the chrome it
// draws FOR ITSELF onto the active theme.
//
// The list is sized first because the centred dot row pins an explicit width off
// it, from the same function and height renderThemePanel sizes its own per-frame
// copy with, so the model's page and the drawn page are the same page.
//
// Sizing to themePanelMinBodyRows would be a defect: `bubbles/list` derives
// Paginator.PerPage from the height it is given, so a list left at the floor has
// a one-row page and `Ctrl+↑`/`Ctrl+↓` would route perfectly while doing nothing
// `↑`/`↓` does not.
//
// The two sizings converge, which is not obvious: `bubbles/list` charges its own
// pagination block against the height before dividing, and that block is two rows
// while the list paginates and none once it does not, so one SetSize derives its
// page from the page count it is replacing. The derivation settles on the second
// pass, and applyThemePanelCanvasMode's SetDelegate below re-runs it immediately.
//
// The re-point itself is applyThemePanelCanvasMode — the same function the
// restyle path calls — so what the open assigns and what an arrow re-points
// cannot diverge.
func (m *Model) applyThemePanelListStyles() {
	width, rows := themePanelListSize(m.themePanel, m.contentHeight())
	m.themePanel.list.SetSize(width, rows)
	m.applyThemePanelCanvasMode()
}

// applyThemePanelCanvasMode is the restyle path's THIRD arm, beside the Sessions
// and Projects ones: it re-points the panel list's `bubbles/list`-owned styles and
// its row delegate onto the model's active palette.
//
// The panel's instance is the worst case of the cached-style class — its styles
// are assigned once at open while its theme changes on every arrow keypress — so
// it takes the same restyle path the main list does. It is not a rebuild: no item
// is re-derived and no content is touched.
//
// The dots are why it cannot be skipped: `bubbles/list` reads its dot strings out
// of the styles once at construction, so restyling without re-feeding the
// paginator leaves the library's hardcoded greys rendering under every theme —
// identical before and after a swap, which the swap-and-diff guard structurally
// cannot see, precisely because nothing changed.
//
// The help, title and no-items styles come with the shared sequence although the
// panel disables the first two and the built-in rows make the third unreachable.
// Taking them costs one assignment each and keeps a future SetShowTitle(true), or
// a union that could genuinely empty, from shipping a stale palette.
//
// It is the bare sequence and not applyPageListCanvasMode: the title-bar geometry
// that wrapper adds serves the section-header surgery, which the panel has no
// equivalent of.
//
// The delegate goes through Model.themeRowDelegate so the previewed theme, the
// colourless flag and the panel's inner width are assembled in one place.
//
// The `open` guard stays here rather than inside the shared sequence — it is a
// fact about this panel, not about restyling a list — and it is what makes
// armThemePanel's mid-arm ApplyTheme safe: the panel struct is installed before
// its list is built, so this path would otherwise run against the zero list.Model.
func (m *Model) applyThemePanelCanvasMode() {
	if !m.themePanel.open {
		return
	}
	applyListCanvasMode(&m.themePanel.list, m.themeRowDelegate(), m.themeState.active, m.colourless)
}

// updateThemePanel is the panel's key-exclusive input routing: the panel owns the
// keyboard while it is open.
//
// Pass-through is genuinely bad — `k` would kill the highlighted session while
// you pick a theme, `x` would swap to Projects with the panel open, `m` would
// start a multi-select behind it. None of that reaches the global quit, and
// swallowing that would take away the user's exit key inside a settings surface,
// so `Ctrl-C` stays live as it does under the burst input-lock.
//
// Non-blanking and key-exclusive are not in tension: the page stays fully
// rendered because it is what the theme is being judged against, not because it
// is still interactive.
//
// `d` and `l` are panel-owned (they take the dark and light slots) rather than
// swallowed page keys, and leave the panel exactly as they found it bar the
// write, whether the keypress writes or — over a constant — raises the confirm
// instead (theme_panel_confirm.go).
//
// The navigation arm is matched against the panel list's own KeyMap, the way the
// Sessions page matches CursorUp / CursorDown / PrevPage / NextPage, so the
// arrow-only rule is stated once at newThemePanelList's pinArrowOnlyNav rather
// than as a second literal key list here.
//
// `Esc` is the only way out and is consumed here, never reaching the page
// beneath, where it is the progressive-back key: closing must not clear an
// applied filter, must not exit multi-select, and must not quit — the innermost
// surface resolves first. `Enter` commits and stays, and is deliberately not an
// arm of the `Esc` case: a dual-purpose exit would let a user who had just set
// both slots wipe the pair on their way out.
//
// The failed-commit line is cleared ahead of the dispatch, and the fall-through
// is the point: the message persists until the next keypress, and that keypress
// still performs its own action. The keypress that raises the line is unaffected,
// because the raise happens inside the dispatch below.
//
// It clears the message and not the state. The outstanding failure
// (themeState.commitFailed) runs until a commit succeeds, which is what stops the
// next `Esc` — a close re-resolves persisted state — from reinstating the silent
// revert the report exists to close.
func (m Model) updateThemePanel(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	(&m).clearThemePanelCommitFailed()

	switch {
	case keyIsCtrlC(msg):
		// With a commit failure outstanding this exit raises nothing and discharges
		// nothing: the main screen is going away, so there is nowhere to put a
		// flash, and `theme: commit failed` is already written as the record. A
		// post-TUI stderr warning is refused — it would put a message about a colour
		// preference on the channel Portal reserves for bootstrap failures.
		return m, tea.Quit
	case m.themePanel.confirming():
		// The slot-from-constant confirm is key-exclusive within the panel, so its
		// arm sits ahead of every other one below — the arrows (a cursor move
		// mid-question would re-theme the screen behind the answer), the commit
		// keys, and the `Esc` close, which the confirm takes as a cancel because the
		// innermost thing resolves first. Only `Ctrl-C` above it survives: it is the
		// global quit, not a third answer.
		return m.updateSlotConfirm(msg)
	case keyIsCode(msg, tea.KeyEscape):
		// The command is the close report riding out of the panel: nil unless a
		// commit failure was outstanding, in which case the close raised the flash the
		// user is left reading and the tick that auto-clears it.
		cmd := (&m).closeThemePanel()
		return m, cmd
	case keyIsCode(msg, tea.KeyEnter):
		// Commit-a-constant, and it deliberately does not close: the panel stays
		// open, the cursor stays where it is, and nothing is re-themed. The error is
		// discarded because this arm has nothing left to do with it — the
		// message-slot line and its outstanding-failure state are raised inside the
		// commit — and on failure the raw keys are untouched, so the `●` cannot move.
		_ = (&m).commitSelectedConstant()
		return m, nil
	case isRuneKey(msg, "d"):
		return m.handleSlotCommitKey(theme.MemberDark)
	case isRuneKey(msg, "l"):
		return m.handleSlotCommitKey(theme.MemberLight)
	case themePanelNavKey(m.themePanel.list.KeyMap, msg):
		return m, (&m).moveThemePanelCursor(msg)
	default:
		return m, nil
	}
}

// themePanelNavKey reports whether msg is one of the four keys the panel gives its
// cursor: `↑`/`↓` to step and `Ctrl+↑`/`Ctrl+↓` to page.
//
// It matches against the live KeyMap rather than against key literals, so the
// panel's routing and the list's own dispatch are driven by one binding set.
// GoToStart / GoToEnd are deliberately absent: pinArrowOnlyNav empties them
// (`g`/`G` and `Home`/`End` are not bound), so there is nothing to route.
func themePanelNavKey(km list.KeyMap, msg tea.KeyPressMsg) bool {
	return key.Matches(msg, km.CursorUp, km.CursorDown, km.PrevPage, km.NextPage)
}

// moveThemePanelCursor is the arrow-preview in its three steps: move, skip,
// preview.
//
// The preview takes the restyle and nothing else: Model.ApplyTheme against a
// palette already in hand. No file is read, no union is reassembled, no directory
// is touched and the session list is not rebuilt.
//
// OSC 11 needs nothing here. View assigns v.BackgroundColor declaratively from
// the active theme's canvas and Bubble Tea diffs it, so hovering N themes emits
// exactly once per distinct canvas landed on. The query is issued only from Init,
// so a later switch opens no new race and the canvas-echo guard needs no new
// handling — do not add suppression or a debounce here.
//
// The mixed-mode flash is likewise left alone: arrowing past a light theme in a
// dark terminal flips the whole canvas near-white and back. That is the feature —
// the previewed theme is what the frame shows.
func (m *Model) moveThemePanelCursor(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
	m.themePanel.list, cmd = m.themePanel.list.Update(msg)
	m.skipUnselectableThemeRow(msg)
	m.previewSelectedThemeRow()
	return cmd
}

// skipUnselectableThemeRow keeps the panel cursor off the unselectable rows after
// the list has processed a navigation key, reusing the mechanism that skips
// group-header rows on the Sessions list: model.go's skipHeaderRow applied to
// Row.Selectable. The step is taken on the list, so it crosses a page boundary
// exactly as an ordinary move would.
//
// Two deliberate differences from skipHeaderRow:
//
//   - It loops. No two group headers are ever adjacent, so one step always clears
//     one; several broken drop-ins in one directory are adjacent by nature.
//   - It reverses rather than falling off. Either end can be reached with no
//     selectable row in the direction of travel, so the loop turns round at
//     whichever boundary it hits and settles on the nearest selectable row back
//     along the way it came. After a page move that need not be the row the cursor
//     started from, because a page jumps over rows without checking them; the
//     nearest selectable row is the right answer either way.
//
// The bound is twice the row count because the reversal can retrace the whole
// span the walk just covered. Built-ins are always valid, so the loop terminates
// on the check; the bound exists so a future all-invalid union degrades instead
// of spinning on a keypress.
func (m *Model) skipUnselectableThemeRow(msg tea.KeyPressMsg) {
	l := &m.themePanel.list
	rows := len(l.Items())
	upward := key.Matches(msg, l.KeyMap.CursorUp, l.KeyMap.PrevPage)

	for range 2 * rows {
		if row, ok := selectedThemeRow(*l); ok && row.Selectable() {
			return
		}
		switch index := l.Index(); {
		case upward && index == 0:
			upward = false
		case !upward && index == rows-1:
			upward = true
		}
		if upward {
			l.CursorUp()
			continue
		}
		l.CursorDown()
	}
}

// previewSelectedThemeRow applies the cursor's row to the whole frame — the app
// re-themes live behind the panel — and is where the panel writes to the rendered
// palette.
//
// The palette comes off the row, which carries it because the enumeration parsed
// it at open and the panel retains the results for its lifetime. That retention
// keeps the swap an O(1) restyle: there is nothing here to read.
//
// A row already painting the screen is skipped, so an arrow that could not move —
// or a reversal that turned back to the row it started on — costs no restyle.
// Swapping to the active theme is a legal no-op in any case (ApplyTheme is
// idempotent per swap); skipping it makes that explicit.
//
// An unselectable row is never previewed: it carries no palette, and the skip
// above makes the case unreachable in production. Checking anyway keeps a union
// with no selectable row at all — the shape the skip's bound also degrades on —
// from painting a zero Theme, which renders silently colourless.
func (m *Model) previewSelectedThemeRow() {
	row, ok := selectedThemeRow(m.themePanel.list)
	if !ok || !row.Selectable() || row.Theme == m.themeState.active {
		return
	}
	m.ApplyTheme(row.Theme)
}

// selectedThemeRow is the union row under the panel's cursor. The false return
// covers an empty list and a cursor on nothing, neither of which is a row to skip
// to or preview from.
func selectedThemeRow(l list.Model) (theme.Row, bool) {
	item, ok := l.SelectedItem().(themeRowItem)
	if !ok {
		return theme.Row{}, false
	}
	return item.Row, true
}

// themeRowDelegate is the single construction point for the panel's row delegate,
// assembling its three inputs from the model: the previewed theme
// (m.themeState.active, not the persisted one — arrowing re-themes the panel
// behind the cursor), the NO_COLOR carve-out flag, and the panel's current inner
// content width.
//
// Two construction sites can disagree about width or colourlessness, and that
// disagreement is invisible until a resize during a live preview. The restyle
// path re-invokes this method rather than rebuilding a delegate of its own.
func (m Model) themeRowDelegate() themeRowDelegate {
	return themeRowDelegate{
		Theme:      m.themeState.active,
		Colourless: m.colourless,
		Width:      themePanelInnerWidth(m.themePanel.width),
	}
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
