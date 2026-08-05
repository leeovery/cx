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

// The §9.1 slide-over theme panel: a full-height, right-edge, LEFT-BORDER-ONLY
// block composited over the already-composed page view, with the page beneath
// deliberately NOT re-laid-out.
//
// THE SHAPE IS NOT A PREFERENCE. Portal's modals blank the page to the canvas
// before drawing (modal.go clears to canvas, then placeModalOnClearedCanvas), so a
// modal theme picker would render the canvas plus its own frame and PREVIEW
// NOTHING — and live preview is the entire feature. Non-blanking is the only shape
// that can do the job, and every downstream constraint inherits from that one fact:
// the ~27–34 column budget (§9.8), the four-element row-composition priority
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
	// column ladder (name, markers, slot indicators, border, gutter, padding). A
	// FIXED width is predictable to lay out against; a content-driven one would make
	// the panel jump around as the theme library changes.
	//
	// Both ends are ~13% wider than the pair task 8-11 shipped, because at 30 columns
	// the badges crowded the right edge and `set as light` ran to the border. Only the
	// ENDS moved: the `contentW / 2` cap and the clamp shape are untouched, so the
	// ladder is still §2.7's staged shrink. The cost is that the overlay covers four
	// more columns of the main footer, cutting one further entry short — which §9.1
	// already sanctions as the mid-label cut the overlay makes wherever its border
	// falls, and which is the least theme-informative part of the screen.
	//
	// themePanelWidthFor chooses between them for a given terminal, and
	// themePanelFloor refuses below the minimum; renderThemePanel renders at
	// whatever width it is handed.
	themePanelPreferredWidth = 34
	themePanelMinWidth       = 27

	// themePanelBorderWidth is the one column the left border occupies. It is NOT
	// list space, which is why the inner content width subtracts it — there is no
	// top, bottom or right edge to charge for.
	themePanelBorderWidth = 1

	// themePanelGutterWidth is the panel's INNER GUTTER: the blank canvas column
	// between the left border and everything the panel draws, so its content sits TWO
	// cells in from that border rather than hard against it.
	//
	// It is the page's own frame gutter read across. Portal insets every page's
	// content Hinset (2) cells from the terminal's edge; the panel's content sat one
	// cell in from its own edge — half of it — which is what made the slide-over read
	// as cramped beside the page it is composited over. Charging border + gutter puts
	// the two on the same inset.
	//
	// It applies to EVERY surface below the header rule uniformly, because it is
	// charged once in themePanelBlock rather than at each renderer: the label, the
	// list rows (cursor column included, so the `▌` moves with them), the pinned
	// directory row, the message slot and the vertical key list.
	themePanelGutterWidth = 1

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
	// panel refuses to open at all (themePanelFloor), so the clamp is a floor rather
	// than a degradation step.
	themePanelMinBodyRows = 1

	// themePanelFloorMessageRows is the ONE message row §9.8's floor counts, and it
	// is counted EVEN THOUGH §9.1 does not reserve the slot when empty.
	//
	// Both of the slot's contenders are NON-SUPPRESSIBLE: §9.2's confirm gates a
	// write that must not happen silently, and §9.13's failed-commit line is
	// specified to persist until the next keypress. A floor computed without this row
	// therefore puts the panel exactly one row short at the moment a message appears
	// — asking "clear constant <slug>?" about a row that has just been pushed off
	// screen, which is the one question the user cannot answer.
	themePanelFloorMessageRows = 1

	// themePanelMessageWrapRows is the slot's height when it WRAPS — §9.1's "at the
	// minimum panel width the slot may wrap to two rows", read as the slot's maximum
	// rather than as an observation about one string.
	//
	// The cap is what keeps the vertical budget provably bounded. themePanelBlock can
	// only CUT an over-long assembly, and it cuts from the BOTTOM — off the footer,
	// `esc close` first — so an unbounded slot is one long message away from taking
	// the key that closes a panel the user can no longer read the way out of.
	themePanelMessageWrapRows = 2
)

// THE PANEL'S HEADER REGION IS MEASURED OFF THE PAGE'S, NEVER RESTATED.
//
// The panel is composited over the page at the content region's Y=0, so its rows
// and the page's are the SAME terminal rows. §9.1's slide-over is a surface inside
// the content region rather than a second column beside it, and that only reads if
// the two run ONE rhythm: `Themes` on the page's `Sessions N` row, the panel's first
// theme row on the page's first session row, and one rule shared between them.
//
// Task 8-6 pinned the header at a restated two rows, which put `Themes` on the
// WORDMARK row and the panel's first row three rows above the first session row. A
// literal is what drifted, so these four are derived from the page's own renderers:
// a change to the header block or the section header moves the panel with it.
//
//   - themePanelHeaderRuleRow — the rows above the rule, which is the page header
//     block's BAND. Nothing is drawn in them (see themePanelHeaderBlock).
//   - themePanelHeaderLabelRow — the page header block's whole height, which is
//     therefore the index of the section-header row beneath it.
//   - themePanelHeaderRows — that plus the section-header block, so the row after
//     the panel's header is the row after the page's.
//   - themePanelBorderFromRow — where the left border starts; see
//     themePanelHeaderBlock for why it is below the rule rather than at it.
//
// The measurements are taken at zero width on the colourless path, exactly as
// themePanelFooterHeight measures its own block: a row COUNT is a function of the
// content, not of the width or the palette, so the layout resolves before either is
// in hand (themePanelMinHeight's floor arithmetic reads the same measurements).

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

// themePanelDim names the dimension §9.8's floor refused on, so the entry gate and
// the resize path select §14A's per-dimension copy from ONE answer rather than each
// re-deciding which dimension was at fault.
//
// dimNone is the zero value and the "nothing failed" answer, which is what a
// satisfied floor returns alongside ok.
type themePanelDim int

const (
	// dimNone is no failure: both dimensions cleared the floor.
	dimNone themePanelDim = iota
	// dimWidth is §9.8's width floor — the content region cannot hold even the
	// minimum panel.
	dimWidth
	// dimHeight is §9.8's height floor — the content region cannot hold header +
	// footer + one list row + one message row (+ the directory row when unusable).
	dimHeight
)

const (
	// themePanelNarrowClosedFlash / themePanelShortClosedFlash are §14A's pinned
	// forced-close copy, VERBATIM. They are the resize half of the same pair the
	// entry strings below belong to, so the two live side by side and neither can be
	// re-worded at a call site.
	themePanelNarrowClosedFlash = "terminal too narrow — theme picker closed"
	themePanelShortClosedFlash  = "terminal too short — theme picker closed"

	// themePanelNoColorFlash / themePanelNarrowEntryFlash / themePanelShortEntryFlash
	// are §14A's pinned BLOCKED-ENTRY copy, VERBATIM — the three strings §9.7's gate
	// can answer with.
	//
	// They are CONSTANTS for the reason spawn.UnsupportedNoopMessage is: a pinned
	// user-facing string with one home cannot be re-worded at a call site, and these
	// are the whole feedback surface for states the user cannot otherwise diagnose.
	// They read differently from their forced-close siblings above on purpose — one
	// pair reports why nothing opened, the other why something the user was looking at
	// went away.
	themePanelNoColorFlash     = "theme picker needs colour — NO_COLOR is set"
	themePanelNarrowEntryFlash = "terminal too narrow for the theme picker"
	themePanelShortEntryFlash  = "terminal too short for the theme picker"
)

// themePanelWidthFor is §9.8's WIDTH LADDER: the panel's outer width for a given
// content-region width, and whether the region can hold a panel at all.
//
// THE DERIVATION, not the number. The panel takes HALF the content region, clamped
// to the [minimum, preferred] ends declared above. The half-width cap is what keeps
// the previewed page visible while the terminal is wide enough to afford it — a
// preview surface that covers the surface being previewed is no preview — and the
// clamp is what turns that into §2.7's STAGED shrink rather than a proportional one
// that would shrink forever: ≥68 content columns take the preferred 34; 54–67 walk
// down through 33…27; 27–53 sit on the 27-column minimum; below 27 the floor
// refuses. §9.8 leaves the exact thresholds to implementation, exactly as §2.7 does
// for its own degradation steps.
//
// THE WIDTH IS CLAMPED ON THE REFUSING PATH TOO. Both callers — the open sequence
// and the resize — take w and ignore ok, because themePanelFloor has already
// refused by the time either runs; returning an unclamped w would make an
// impossible state render a sub-minimum panel instead of degrading to the minimum.
// That is the same treatment the floor predicate's other caller gives an impossible
// result.
func themePanelWidthFor(contentW int) (w int, ok bool) {
	return min(max(contentW/2, themePanelMinWidth), themePanelPreferredWidth), contentW >= themePanelMinWidth
}

// themePanelMinHeight is §9.8's HEIGHT FLOOR: header + footer + one list row + one
// message row, plus §9.5's pinned directory row when the themes directory is
// unusable.
//
// NOTHING HERE IS A LITERAL. The footer is measured (themePanelFooterHeight), which
// is what makes the floor follow Phase 9's shorter confirm footer without a second
// edit; the HEADER is measured too (themePanelHeaderRows), which is what makes it
// follow the page's own header and section header. A restated header count is
// exactly what task 8-6 shipped and exactly what drifted — so the floor grows by
// precisely the rows the header gained, with no second edit here either.
//
// THE MESSAGE ROW IS UNCONDITIONAL and the DIRECTORY ROW IS CONDITIONAL, and both
// halves are load-bearing. The message row is counted although §9.1 leaves the slot
// unreserved when empty, because neither contender can be suppressed (see
// themePanelFloorMessageRows). The directory row is counted only when it renders,
// because counting it always would refuse terminals that would have rendered a
// perfectly good panel — §9.8's degrade-don't-refuse doctrine — while counting it
// never would let the warning consume the single list row, leaving nothing
// selectable on screen at exactly the moment §9.5 requires built-in and persisted
// rows to render beneath it.
func themePanelMinHeight(entries []keymapEntry, dirUnusable bool) int {
	return themePanelHeaderRows() +
		themePanelDirRowHeight(dirUnusable) +
		themePanelMinBodyRows +
		themePanelFloorMessageRows +
		themePanelFooterHeight(entries)
}

// themePanelFloor is THE render-floor predicate, and it has TWO callers by design:
// §9.7's entry condition and §9.8's resize condition. They must not each derive
// their own arithmetic — a terminal that passes one check and fails the other is
// precisely the state that opens a broken frame or refuses a panel that fitted — so
// the answer is computed here and consumed, never recomputed.
//
// It reports WHICH dimension failed so both callers select §14A's per-dimension
// copy from the same result. WIDTH IS CHECKED FIRST, so a terminal failing both
// reports narrow: that is the dimension the user can act on with the same gesture
// that broke it, and pinning the order is what keeps the two callers' copy
// identical on a terminal that is both.
func themePanelFloor(contentW, contentH int, dirUnusable bool) (dim themePanelDim, ok bool) {
	if _, wide := themePanelWidthFor(contentW); !wide {
		return dimWidth, false
	}
	if contentH < themePanelMinHeight(themePanelKeymap(), dirUnusable) {
		return dimHeight, false
	}
	return dimNone, true
}

// themePanelForcedCloseFlash is §14A's forced-close copy for the dimension the
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

// themePanelEntryFlash is §14A's BLOCKED-ENTRY copy for the dimension the floor
// refused on — the entry-side mirror of themePanelForcedCloseFlash, degrading to the
// width copy on the unreachable dimNone for the same reason.
func themePanelEntryFlash(dim themePanelDim) string {
	if dim == dimHeight {
		return themePanelShortEntryFlash
	}
	return themePanelNarrowEntryFlash
}

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
// that same site by task 8-9's restyle and by resizeThemePanel (SetDelegate, never a
// second construction site).
//
// renderThemePanel takes no Model and never touches the delegate, so the rows are
// drawn with whatever the model last assigned. Keeping that in step with the theme
// the panel is RENDERED at is therefore the MODEL's job, not the renderer's — which
// is why 8-9's restyle re-points the delegate on the same keypress that moves the
// preview.
type themePanel struct {
	// open gates the whole surface, and it means "the panel is LIVE" rather than
	// "an open was requested": armThemePanel sets it only once the list below
	// exists, because the restyle path's panel arm keys off it and would otherwise
	// run against a zero list.Model mid-arm. See armThemePanel's ordering note.
	open bool

	// list is the panel's own bubbles/list instance (see the type comment).
	list list.Model

	// enumeration is the ONE directory read, retained for the panel's lifetime
	// (§5.8) so arrowing previews from values already in hand and §9.2's post-commit
	// recompute re-derives with no fresh I/O. openThemePanel fills it and the
	// open-time re-resolution reads it (applyThemePanelResolution); `Esc`'s close
	// re-resolves against it one last time before dropping it (closeThemePanel),
	// and §9.2's commit recompute is its other reader.
	enumeration theme.Enumeration

	// union is §9.4's finished row set, already ordered and already carrying each
	// row's single §6.2 reason.
	union theme.Union

	// badges is §9.5's `●` table, keyed by theme.Row.BadgeKey — a fact about the
	// whole SETTING rather than about one row, which is why it is held here and
	// looked up per row rather than derived at the delegate. It is derived at open
	// from the seam's own re-resolution against the retained enumeration — the
	// panel's parse, never construction's (§5.8) — and rowItems assembles the list
	// through it.
	//
	// §9.2's post-commit recompute re-derives it against that SAME enumeration
	// (refreshThemePanelBadges), which is what makes the `●` mean "what is
	// persisted" after a write as well as before one. Those two are its only
	// writers: a FAILED commit reaches neither, so the marker cannot move on a
	// write that did not land (§9.13).
	badges map[string]theme.Badge

	// message is §9.1's message slot, and it is STILL ALWAYS EMPTY: both of the
	// slot's contenders — the slot-from-constant confirm (§9.2) and the
	// failed-commit line (§9.13) — belong to later tasks, so the slot's height is
	// accounted for while the field has no setter and no arbiter here.
	//
	// `Enter` RAISES NEITHER, and that is a decision rather than a gap. §9.2 gives
	// the commit-a-constant key no confirm even over a pair — it visibly does what
	// it says, and the theme is already previewing behind the panel — and its
	// failed-write report is task 9-7's. The asymmetry with `d`/`l` is the point:
	// the confirm guards the case where the RESOLVED theme changes as a side effect
	// of a write the user was told is inert.
	message string

	// width is the panel's OUTER width, border column included — the value
	// themePanelWidthFor chooses between themePanelMinWidth and themePanelPreferredWidth.
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
// The nav keymap is PINNED through the shared pinArrowOnlyNav, exactly as the
// Sessions and Projects lists pin theirs: §12.2's revision is arrows only, and the
// v2 DefaultKeyMap re-introduces the vim aliases (h/j/k/l, g/G) plus
// PgUp/PgDn/Home/End/b/u/f/d. On the panel two of those additionally COLLIDE with
// its own commit keys — the default binds `l` to NextPage and `d` to NextPage,
// which §9.2 gives to the light and dark slots — so pinning here is what keeps a
// banned key from ever reaching the list's own Update.
//
// The list is created at zero size and sized at open by applyThemePanelListStyles,
// which the centred dot row's explicit width depends on. renderThemePanel re-applies
// the SAME size per frame on its own copy, from the height it is actually rendered
// at (see themePanelListSize) — the model's list carries the size so its PAGE is the
// drawn page, which is what makes §9.2's `Ctrl+↑`/`Ctrl+↓` move a page rather than a
// row.
func newThemePanelList(items []list.Item, delegate list.ItemDelegate) list.Model {
	l := list.New(items, delegate, 0, 0)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowHelp(false)
	l.SetFilteringEnabled(false)
	pinArrowOnlyNav(&l.KeyMap)
	return l
}

// themePanelEntry is §9.7's ENTRY GATE: the ONE place the pre-read decision to open
// the panel is made, answering either "open" or the pinned copy the refusal flashes.
//
// NOTHING BLOCKS `t` EXCEPT a modal, a pending burst, `NO_COLOR`, a terminal below
// either render-floor dimension, and the pages where it is not bound at all — and
// only the middle two are decided here. The other three are decided by where the key
// is dispatched from at all, which is why they produce NO FLASH:
//
//   - MODALS are key-exclusive by design, so `t` never reaches this gate.
//   - A PENDING BURST swallows it at updateSessionList's input-lock, ahead of every
//     rune arm. That is CONSISTENCY with the lock rather than an exception to it —
//     the model is mid-async-operation and only `Ctrl-C`/`Esc` are live — so there is
//     deliberately no burst branch here to keep the lock decided in one place.
//   - PREVIEW and LOADING bind no `t` at all (§9.6). Preview's body is captured real
//     ANSI scrollback that is deliberately out-of-theme, so a live preview would
//     re-theme the frame chrome alone, and it is ALREADY a full-screen overlay, so the
//     panel would stack an overlay on an overlay. Loading is inert by design (animation
//     only, which is what contains the restore/daemon race surface) and renders no
//     notice band to flash into — and it is not a corner case, since the cold + TUI
//     path holds it for at least LoadingMinDuration with the user watching.
//
// That split IS §9.7's feedback rule: FLASH where the key is bound and the user could
// reasonably expect it to work, SILENT where it is not bound at all — exactly how `s`
// already behaves.
//
// THE TWO REFUSALS BELOW ARE DELIBERATELY OPPOSITE CALLS (§9.10), and conflating them
// gets one of them wrong:
//
//   - `NO_COLOR` is a CAPABILITY ABSENCE — Portal paints no canvas and imposes no
//     hues, so the panel previews nothing, its cursor tint and slot dots have no
//     colour, and a commit would persist a choice with zero visible feedback. It is
//     blocked PROACTIVELY, following the multi-select precedent exactly, which is what
//     keeps the user out of a walkable dead end.
//   - A NARROW OR SHORT terminal is a SPACE SHORTAGE, where §2.7 mandates degrade:
//     the panel shrinks through §9.8's ladder and refuses only once even the minimum
//     cannot render.
//
// It reads themePanelFloor — the SAME predicate §9.8's resize condition reads — with
// dirUnusable FALSE, because that flag is a product of the enumeration and the read
// happens on this keypress, after this decision. openThemePanel re-applies the same
// predicate with the real flag; see its note for why neither shortcut is taken.
func (m Model) themePanelEntry() (blockedFlash string, ok bool) {
	if m.colourless {
		return themePanelNoColorFlash, false
	}
	if dim, fits := themePanelFloor(m.contentWidth(), m.contentHeight(), false); !fits {
		return themePanelEntryFlash(dim), false
	}
	return "", true
}

// handleThemePanelKey is §9.6's `t`, and it is what BOTH pages' dispatch arms call:
// the gate above decides, and this either opens or raises the refusal's copy.
//
// ONE HANDLER, TWO CALL SITES. Sessions and Projects must answer `t` identically —
// theme is a GLOBAL setting (§9.6) — and two arms each consulting the gate for
// themselves is how one page comes to flash where the other opens. Both pages raise
// the flash through their OWN band with no branch here, because the band is arbitrated
// per page from the same m.flashText (§14A gave Projects its own transient-flash slot
// for exactly these six signals).
func (m Model) handleThemePanelKey() (tea.Model, tea.Cmd) {
	flash, ok := m.themePanelEntry()
	if !ok {
		return m.blockThemePanel(flash)
	}
	return m.openThemePanel()
}

// blockThemePanel raises a blocked-`t` flash and schedules its auto-clear — the ONE
// site either refusal (the pre-read gate, the post-read floor re-evaluation) reports
// through.
//
// The flash is an ORDINARY transient one with no bespoke lifecycle: the post-bump
// generation feeds flashTickCmd so the block inherits the standard timer, exactly as
// the proactive multi-select block does, with the next-actionable-key clear at the top
// of each page's update remaining the authoritative route.
func (m Model) blockThemePanel(flash string) (tea.Model, tea.Cmd) {
	(&m).setFlash(flash)
	return m, flashTickCmd(m.flashGen)
}

// openThemePanel is §9.6's `t`: the ONE directory read, the parse results
// retained for the panel's lifetime, the list assembled from the union they
// produced, and §9.2's opening state resolved against them (armThemePanel).
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
//
// THE FLOOR IS RE-EVALUATED HERE, against the SAME predicate the entry gate read and
// with the REAL directory verdict this read has just produced. §9.5's pinned
// `⚠ dir unreadable` row raises the height floor by one, and DirUnusable does not
// exist until the enumeration runs — so the one predicate is evaluated twice rather
// than the arithmetic being re-derived, which is what themePanelFloor's
// "compute it once" forbids. Both shortcuts are refused, in opposite directions:
//
//   - Assuming TRUE at the gate would refuse terminals that render a perfectly good
//     panel, contradicting §9.8's degrade-don't-refuse doctrine outright.
//   - Assuming FALSE and never re-checking would open a panel whose list body is ZERO
//     ROWS — the warning consuming the single row §9.5 requires built-in and persisted
//     rows to render BENEATH it, which is the "completely in the dark" state that row
//     exists to prevent.
//
// THE COST IS ACCEPTED RATHER THAN WORKED AROUND: a refusal on this path has already
// read the directory and already emitted `theme: enumerated` (§12.3). The
// enumeration genuinely happened, so the record is honest; splitting the read from its
// emission would fork task 8-1's seam for one rare edge. The enumeration is simply
// discarded — nothing is armed, so no panel state survives — and the SAME pinned copy
// the gate would have raised is flashed. Only the height can fail here (the width is
// independent of the flag and the gate already cleared it), and selecting through the
// shared themePanelEntryFlash is what keeps the two evaluations' copy identical.
func (m Model) openThemePanel() (tea.Model, tea.Cmd) {
	if m.themeEnumerator == nil {
		return m, nil
	}

	enumeration, union := m.themeEnumerator.Open(m.themeKeys)
	if dim, fits := themePanelFloor(m.contentWidth(), m.contentHeight(), union.DirUnusable); !fits {
		return m.blockThemePanel(themePanelEntryFlash(dim))
	}
	(&m).armThemePanel(enumeration, union)
	return m, nil
}

// armThemePanel installs one enumeration's results as the live panel state, and
// §9.2's opening state with them: the badges, the theme actually rendering, and
// the row the cursor lands on.
//
// THE WIDTH COMES FROM §9.8's LADDER, not from the preferred width, so the width a
// panel OPENS at and the width it degrades to on a resize are the same function of
// the same input. Opening at the preferred width and narrowing only if the user
// happened to resize afterwards would contradict §9.8's staged shrink outright, and
// would leave the degraded band reachable by no fixture at all — a fixture opens
// through captureKeys and never resizes. The `ok` return is DISCARDED because it is
// unreachable here: task 8-13's entry gate already refused below the floor, and
// themePanelWidthFor clamps its width to the minimum on that impossible path rather
// than making this site branch on a state it cannot be in.
//
// The ORDER is load-bearing at five points. The width is set before the list is
// built, because Model.themeRowDelegate composes the row budget from it. The
// RESOLUTION runs before the list is built, because it decides both the badges
// every row item carries and the palette that delegate is constructed from — a
// list built first would render the previous theme's rows behind the new canvas.
// The list is built before the styles are applied, because those re-point the
// list's own chrome. And the cursor is anchored LAST, because applying the styles
// re-sizes the list, which re-derives its page from the index it finds.
//
// THE FIFTH IS `open`, AND IT IS SET ONLY ONCE THE LIST EXISTS. The resolution
// above applies the in-force theme through Model.ApplyTheme, whose restyle path
// re-points the panel list's own styles and delegate WHILE THE PANEL IS OPEN
// (applyThemePanelCanvasMode) — so a panel marked open before its list is built
// would have that path run against the zero list.Model. Marking it open here is
// the honest reading rather than a workaround: `open` means the surface is live,
// and a surface with no list is not. Nothing between the struct install and this
// line reads the flag, and the re-point is not merely safe to skip during the arm
// but redundant — the list is constructed with a delegate taken from the palette
// the resolution just applied, and applyThemePanelListStyles re-points its chrome
// on the very next line.
//
// THE CAPTURE SEED IS LAST, AND IT MOVES THE CURSOR AND NOTHING ELSE (§13.3's
// fourth fixture input, wired only by the offline harness — see
// Model.initialThemeCursor). It re-anchors BY ROW IDENTITY, through the same
// anchor the resolution's answer went through, so a fixture cannot seed a cursor
// onto a row the union does not hold. It applies NO theme: the palette stays the
// one `--theme` pinned, which is what keeps the flag meaningful on precisely the
// frames a drop-in author checks. An empty seed — every production model and
// every non-panel fixture — is the anchor's own no-op.
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
	m.anchorThemePanelCursor(m.initialThemeCursor)
}

// applyInForceTheme is §5.8's RE-RESOLUTION, and it is the whole of what the
// panel's OPEN and its CLOSE have in common: the persisted setting taken from the
// model's raw keys, resolved against the RETAINED enumeration, the one member in
// force selected from the answer, and that member applied through
// Model.ApplyTheme — the same production restyle path every other caller drives.
//
// THE DEGRADE POLICY BELOW IS STATED HERE RATHER THAN RESTATED at each site,
// because it governs EVERY panel call site of Resolve — this open (§9.2),
// `Esc`'s close (§5.8) and Phase 9's commit recompute — so the three cannot each
// invent their own. THE BODY IS SHARED BY THE OPEN AND THE CLOSE ONLY: a commit
// recomputes rows and badges and never the rendered theme (§11.1, §9.2), so it
// takes the policy without taking the apply — routing it through here would swap
// the screen off the preview the user is still looking at. Sharing the body
// between the two that DO apply is also what makes the theme applied and the row
// the open anchors come from ONE evaluation (see inForceSlot).
//
// IT RESOLVES AGAINST THE RETAINED PARSE AND NEVER THE FILESYSTEM (§8.4): a read
// here would produce a THIRD parse of the same slug, neither construction's nor
// the panel's, that can disagree with the rows the user is looking at. The
// light/dark answer is likewise READ OFF THE MODEL rather than asked for again —
// §8.8's gate resolves exactly once.
//
// THE ERROR POLICY. The only error Resolve can return is §7.6's fatal, from a
// binary whose embedded set cannot supply a fallback, which the build-time
// guarantee makes unreachable in a correctly built binary. The panel therefore
// DEGRADES RATHER THAN ESCALATING: nothing is applied, the caller leaves the
// state it holds exactly as it was, and nothing is written. A settings surface
// must not become the route by which a broken binary quits Portal mid-session,
// and §7.6 puts the fatal on the STARTUP path deliberately. A resolution naming
// no slot at all takes the same path, for the same reason: it is a shape nothing
// production can produce, and degrading is what keeps a fixture that returns one
// from painting a colourless picker.
//
// THE SEAM IS ALWAYS LIVE HERE. openThemePanel's nil guard is what makes the
// panel openable at all, so both callers — the open sequence and the close of an
// open panel — run only where a seam that can be called was wired.
//
// A theme already painting the screen is skipped. ApplyTheme is idempotent per
// swap, so that is explicitness rather than necessity — and it is what makes the
// common close, where nothing changed, cost no restyle at all.
func (m *Model) applyInForceTheme(e theme.Enumeration) (theme.Resolution, theme.SlotResolution, bool) {
	resolution, err := m.themeEnumerator.Resolve(e, m.themeSetting())
	if err != nil {
		return theme.Resolution{}, theme.SlotResolution{}, false
	}
	inForce, ok := inForceSlot(resolution, m.canvasMode)
	if !ok {
		return theme.Resolution{}, theme.SlotResolution{}, false
	}

	if inForce.Theme != m.activeTheme {
		m.ApplyTheme(inForce.Theme)
	}
	return resolution, inForce, true
}

// applyThemePanelResolution is §9.2's "the cursor lands on the theme that is
// actually rendering": the re-resolution above, plus the two things only the OPEN
// needs from it — the badge table refreshed from the answer, and the row identity
// the cursor belongs on.
//
// THE RE-RESOLUTION IS THE POINT, and it is why opening is not a passive read.
// §5.8 makes the panel's parse supersede the construction-time one, so this is
// where a mid-session edit lands: an edited-but-still-valid active theme
// re-renders with its new values, and one that has been INVALIDATED flips to
// §8.5's fallback here rather than at `Esc` — deferring would leave the panel
// listing a theme as invalid while the screen still renders it. The mirror case
// falls out of the same call: a theme that was broken at construction becomes
// loadable the moment the user fixes the file, and this open applies THEIRS.
//
// The empty string is the degrade path's identity, which anchorThemePanelCursor
// reads as "leave the cursor where it is". It is unambiguous: §5.2's anchored
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

// themeSetting collapses the model's construction-time raw keys onto §8.2's
// two-state setting.
//
// It routes through ResolveSetting rather than restating §8.2's tiebreak, which
// is the same single site the union assembly and doctor's line resolve through —
// so what the panel LISTS, what it MARKS and what it RESOLVES cannot disagree
// about which slug is live. Re-running it on already-stripped keys is safe:
// stripping is idempotent and the resolution is pure and total.
func (m Model) themeSetting() theme.Setting {
	setting, _ := theme.ResolveSetting(m.themeKeys.Theme, m.themeKeys.Light, m.themeKeys.Dark)
	return setting
}

// inForceSlot is the ONE slot painting the screen: the constant under a constant,
// and under a pair the member the light/dark answer names — light in a light
// terminal, dark otherwise.
//
// The answer is the gate's SINGLE resolution (§8.8), read off the model rather
// than asked for again: a constant never consulted detection at all and a pair
// resolved exactly once before first paint, so a query here would reopen the race
// the resolve-once rule closes. On a gate that was never armed the value is the
// standing dark no-answer fallback, which is the right answer for the same reason
// it is the right canvas.
//
// The slot is matched on its Slot rather than taken by position, and ONE record
// answers for both the palette and the cursor's target — which is what makes
// §9.2's invariant structural: the theme applied and the row anchored come from
// the same slot, so they cannot be two lookups that disagree.
//
// The false return is a resolution naming no slot at all — not a state the
// resolver produces, but a shape a fixture can hand back, and the caller degrades
// on it rather than selecting a zero Theme.
func inForceSlot(r theme.Resolution, mode canvasAppearance) (theme.SlotResolution, bool) {
	want := theme.SlotDark
	if mode == appearanceLightCanvas {
		want = theme.SlotLight
	}
	for _, slot := range r.Slots {
		if slot.Slot == theme.SlotConstant || slot.Slot == want {
			return slot, true
		}
	}
	return theme.SlotResolution{}, false
}

// anchorThemePanelCursor puts the cursor on the row whose IDENTITY is slug.
//
// ANCHORING IS BY IDENTITY AND NEVER BY INDEX (§9.2). An index silently breaks
// the invariant the moment a row is inserted above the cursor — the screen keeps
// previewing one theme while the cursor sits on another — and that is exactly what
// Phase 9's commit recompute does, since clearing a slot can remove the row that
// existed only because the slot named it, and assigning one can mint a new row
// above the cursor.
//
// The target is the RESOLVED slug, never the requested one: under §8.5's fallback
// those differ, and the fallback's row is the one that is painted. The persisted
// row keeps its `●` and stays where it is — `●` is what is SET, the cursor is what
// is PREVIEWED (§9.5) — and it is unselectable, so parking the cursor there would
// put it somewhere the arrows, which skip unselectable rows, cannot return to.
//
// An EMPTY slug is the degrade path's no-op: applyThemePanelResolution's error
// policy leaves the cursor exactly where it was.
func (m *Model) anchorThemePanelCursor(slug string) {
	if slug == "" {
		return
	}
	m.themePanel.list.Select(themePanelRowIndex(m.themePanel.union.Rows, slug))
}

// themePanelRowIndex is the index of the SELECTABLE row identified by slug,
// degrading to the first selectable row and then to zero.
//
// The identity is Row.SortKey — the slug wherever one exists, else the filename,
// else the raw persisted string — which is the same value the ordering and the
// badge lookup key on, so a row can be found by exactly what it is listed under.
// A `reserved name` row shares its key with the built-in it collides with by
// definition (§6.2), and the built-in sorts first (§9.5), so the first match is
// always the selectable one the slug actually resolved to.
//
// THE MATCH IS FILTERED ON Selectable, AND THAT IS NOT REDUNDANT WITH THE ORDER.
// §9.2's invariant is that the cursor is always on a selectable row, and this
// function has a second caller the resolver does not control: §13.3's capture seed
// (Model.initialThemeCursor) re-anchors by identity from a string a FIXTURE declares,
// and §13.3 mandates fixtures built from an invalid drop-in and an unreadable themes
// directory — whose rows are exactly the unselectable ones. A cursor parked there
// would sit somewhere the arrows, which skip unselectable rows (§9.5), cannot return
// to. Falling through is the same degrade a seed naming no row at all already takes.
//
// THE FINAL CLAMP IS A STRUCTURAL GUARD, NOT A LIVE PATH. Built-ins are always valid
// (§7.6's build-time guarantee), so a union with no selectable row is unreachable.
// The guard is here because the alternative to degrading is indexing out of range
// inside a list the user is looking at.
func themePanelRowIndex(rows []theme.Row, slug string) int {
	identified := func(row theme.Row) bool { return row.SortKey() == slug && row.Selectable() }
	if at := slices.IndexFunc(rows, identified); at >= 0 {
		return at
	}
	return max(slices.IndexFunc(rows, theme.Row.Selectable), 0)
}

// closeThemePanel is §9.2's `Esc`: it discards an uncommitted preview, renders
// the RESOLVED PERSISTED STATE, and then drops everything the panel retained so
// the next open RE-READS (§5.8) rather than replaying a stale parse.
//
// IT DOES NOT RESTORE A THEME SNAPSHOTTED AT OPEN. That is the naive
// implementation, it is wrong in BOTH directions, and it must not be "simplified"
// back into one:
//
//   - BACKWARDS — a user who broke their active theme's file mid-session would be
//     handed back a palette the config no longer yields. §5.8 is explicit: Portal
//     shows what the config NOW says, not a stale copy it happens to still hold,
//     so a close lands on §8.5's fallback exactly as the open did.
//   - FORWARDS — a Phase 9 commit writes prefs and leaves the panel OPEN, so an
//     `Esc` AFTER one must resolve to the NEWLY persisted state (§9.2). `Esc`
//     equals "what you had before" only when nothing was committed, which is why
//     the mechanism has to be re-resolution from the start rather than a snapshot
//     that happens to agree today.
//
// §11.1 names this the caller that MATTERS MOST: a missed re-point here leaves a
// preview the user explicitly discarded painting the main screen, with no surface
// left open to explain it — and §13.4's completeness guard drives the
// arrow-preview entry point only.
//
// THE ORDER IS LOAD-BEARING: the resolution reads the retained enumeration, so
// the DISCARD IS LAST. Zeroing the whole struct is the point rather than a
// shortcut — the enumeration, the union, the badge table, the message and the
// list are one lifetime, and clearing a subset is how a panel comes to show rows
// from one read and badges from another.
//
// THE PAGE BENEATH NEEDS NO RE-LAYOUT, BECAUSE IT WAS NEVER REDUCED ON OPEN.
// overlayThemePanel composites over a base composed at the UNREDUCED content
// width, so there is no frame to reclaim here. That is stated as a NEGATIVE
// deliberately: a reader who "completes" the close with a reclaim step is one
// step from adding the open-time reduction that would justify it, which reflows
// the surface being previewed and falsifies both §9.1's cut-mid-label cost and
// the panel's Projects fixture. A notice band raised or cleared while the panel
// was open is already handled on its own path — resyncPageLayouts, which re-sizes
// both page lists for the band (§14A gave Projects its own flash slot, so both
// pages route through it); closing adds nothing to it.
//
// NOTHING IS WRITTEN — no prefs write, no tmux option, no file. Every write is an
// explicit keypress (§9.2), which is what eliminates the "applied but not
// persisted" state persist-on-close would reach, where Portal dies with the
// visually-applied theme never written. And closing is ONE FRAME: no animation,
// no transition, no intermediate width.
//
// THE POST-CLOSE STEP BELONGS TO THE CALLER, and this is the ONE close every
// caller routes through. §9.8's forced close calls it and then raises its flash;
// Phase 9's `⚠ theme not saved` line and its outstanding-failure discharge attach
// the same way. Neither forks the path — a second close implementation is exactly
// what "a forced close takes the `Esc` path exactly" forbids, since two of them
// can drift.
func (m *Model) closeThemePanel() {
	m.applyInForceTheme(m.themePanel.enumeration)
	m.themePanel = themePanel{}
}

// resizeThemePanel is §9.8's RESIZE CONDITION, and it is the second of
// themePanelFloor's two callers (§9.7's entry gate is the other): while the terminal
// stays above the render floor the panel DEGRADES IN PLACE, and below either
// dimension's floor it FORCE-CLOSES with §14A's pinned per-dimension copy.
//
// DEGRADE IN PLACE IS THREE THINGS, and the third is the one that is easy to miss:
// the ladder re-runs (so the width follows the terminal), the body arithmetic
// re-runs (so the model's list derives the SAME page the drawn frame does — a stale
// PerPage makes §9.2's `Ctrl+↑`/`Ctrl+↓` move a different distance than the screen
// scrolls, which no rendered frame reveals), and the panel's DELEGATE is re-pointed.
// The delegate matters because it holds its composition budget as a FIELD — unlike
// SessionDelegate, which reads the width off the list it renders into — and the only
// other caller of Model.themeRowDelegate runs on a THEME SWAP, which a
// tea.WindowSizeMsg is not. A resize with no arrow after it would otherwise leave
// every row composing against the pre-resize budget: §11.2's "invisible until a
// resize during a live preview", exactly. All three come from re-invoking
// applyThemePanelListStyles — the SAME function the open runs — rather than from a
// second copy of the arithmetic here.
//
// THE MAIN SCREEN IS DELIBERATELY NOT RE-LAID-OUT to the reduced width (§9.1). The
// panel is composited over a page composed at the UNREDUCED content width, so a
// panel width change never reflows the surface being previewed; the caller has
// already re-sized both page lists to the full content region and this adds nothing
// to that.
//
// THE FORCED CLOSE TAKES THE `Esc` PATH EXACTLY — closeThemePanel, the same
// function, never a second teardown — and then raises its flash, which is the
// post-close step that function's contract leaves to its caller. Any other
// behaviour would strand the user rendering a theme they never chose, with the
// surface that could change it gone and a terminal too narrow to reopen it: the
// state §11.4 names as the one where a colour the user never chose survives Portal's
// exit. A live slot-from-constant confirm is SILENTLY CANCELLED by this close —
// nothing has been written at that point (§9.2), so there is no partial state to
// leave behind; it is worth stating because the confirm is otherwise specified as
// resolvable only by a keypress.
//
// THE SEAM IS LIVE ON THIS PATH. closeThemePanel re-resolves through
// m.themeEnumerator with no nil guard, and this new caller is safe for the same
// reason `Esc` is: it fires only while themePanel.open, and `open` is set in exactly
// one place (armThemePanel), reachable only through openThemePanel's nil guard.
//
// The flash is raised WITHOUT scheduling an auto-clear tick, because the resize is
// not a cmd-returning site — it clears on the next actionable key exactly as the
// burst's unsupported no-op flash does, and a geometry report the user can see for
// themselves is the right thing to leave standing until they touch a key.
func (m *Model) resizeThemePanel() {
	if !m.themePanel.open {
		return
	}
	if dim, ok := themePanelFloor(m.contentWidth(), m.contentHeight(), m.themePanel.union.DirUnusable); !ok {
		m.closeThemePanel()
		m.setFlash(themePanelForcedCloseFlash(dim))
		return
	}
	m.themePanel.width, _ = themePanelWidthFor(m.contentWidth())
	m.applyThemePanelListStyles()
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

// applyThemePanelListStyles sizes the panel's list and re-points the chrome it
// draws FOR ITSELF onto the active theme.
//
// The list is SIZED first because the centred dot row pins an explicit width off
// it. The size comes from themePanelListSize against the CONTENT HEIGHT the panel
// is composited into (overlayThemePanelOnContent passes exactly m.contentHeight()),
// which is the same function and the same height renderThemePanel sizes its own
// per-frame copy with — so the model's page and the drawn page are the same page,
// with no second piece of geometry invented here.
//
// SIZING TO themePanelMinBodyRows WOULD BE A DEFECT, not a deferral. `bubbles/list`
// derives Paginator.PerPage from the height it is given, so a list left at the floor
// has a ONE-ROW page and NextPage/PrevPage advance the cursor by exactly one row —
// §9.2's `Ctrl+↑`/`Ctrl+↓` would route perfectly and still do nothing `↑`/`↓` does
// not, and §9.8's "overflow: scroll, through the bubbles/list machinery" would be
// unimplemented on a panel that renders as though it were. resizeThemePanel re-applies
// this on a tea.WindowSizeMsg; that path owns the panel's page CHANGING, not its
// existing at all.
//
// THE TWO SIZINGS LAND ON THE SAME PAGE, which is worth stating because the
// arithmetic is not obviously convergent: `bubbles/list` charges its own pagination
// block against the height BEFORE dividing, and that block is two rows while the
// list paginates and none once it does not — so one SetSize derives its page from
// the page count it is replacing. The derivation settles on the second pass (a list
// that fits on one page still fits when handed the spare row back; one that does not
// keeps the two-row charge), and applyThemePanelCanvasMode's SetDelegate below
// re-runs it immediately — so the per-frame copy always re-derives from a settled
// count. Verified across the boundary where the count collapses to one page; the only
// shape a residual difference could take is a spare row on a list that already holds
// every one of its rows.
//
// The re-point itself is applyThemePanelCanvasMode — the SAME function the restyle
// path calls — rather than a second copy of it here, so what the open assigns and
// what an arrow re-points can never diverge.
func (m *Model) applyThemePanelListStyles() {
	width, rows := themePanelListSize(m.themePanel, m.contentHeight())
	m.themePanel.list.SetSize(width, rows)
	m.applyThemePanelCanvasMode()
}

// applyThemePanelCanvasMode is the restyle path's THIRD arm (§11.2), beside the
// Sessions and Projects ones: it re-points the panel list's `bubbles/list`-owned
// styles and its row delegate onto the model's active palette.
//
// §11.2 names the panel's instance the WORST CASE of the cached-style class — its
// styles are assigned once at open while its theme changes on EVERY arrow keypress
// (§9.11 requires the panel's own chrome to re-theme, no exceptions) — and assigns
// it to "the same restyle path as the main list, extended to cover the panel's
// instance". IT IS NOT A REBUILD: no item is re-derived and no content is touched.
// §11.1 rules the rebuild out as the expensive path, and it would be worse here, on
// a per-keypress surface.
//
// The DOTS are the class's exemplar and the reason this cannot be skipped:
// `bubbles/list` reads its dot STRINGS out of the styles once at construction, so
// restyling without re-feeding the paginator leaves the library's hardcoded greys
// rendering under every theme — identical before and after a swap, which §13.4's
// swap-and-diff guard structurally cannot see, precisely because nothing changed.
// The shared canvas/colourless helpers are reused verbatim so the panel's dots
// cannot drift from the two lists'.
//
// The HELP, TITLE and NO-ITEMS styles are re-pointed although the panel disables the
// first two (newThemePanelList turns off its title, status bar and help, drawing all
// of that itself) and the built-in rows make the third unreachable (§7.6's build-time
// guarantee means the union always holds at least one row, so `bubbles/list` never
// renders its zero-items body here). They are here because §11.2 names them, and
// because the alternative is a carve-out: a surface that deliberately ignores the
// active theme is exactly the shape §13.4's guard exists to catch, and re-pointing
// them costs one assignment each while making a future SetShowTitle(true) — or a
// union that could genuinely empty — incapable of shipping a stale palette. Skipping
// Styles.NoItems on an unreachability argument is precisely the shape applyCanvasMode's
// residue record was rewritten to forbid, since a blanket claim of that kind is what
// let this style sit unnoticed on the two main lists while reaching a real frame. The
// TitleBar padding the two main lists set is NOT copied — that serves the
// section-header surgery, which the panel has no equivalent of.
//
// The DELEGATE goes through Model.themeRowDelegate, the single construction point
// (§11.2), so the previewed theme, the colourless flag and the panel's inner width
// are assembled in exactly one place.
//
// The `open` GUARD is what makes armThemePanel's mid-arm ApplyTheme safe: the panel
// struct is installed before its list is built, so this path would otherwise run
// against the zero list.Model. See armThemePanel's ordering note.
func (m *Model) applyThemePanelCanvasMode() {
	if !m.themePanel.open {
		return
	}
	l := &m.themePanel.list
	l.SetDelegate(m.themeRowDelegate())
	// Hoisted above the branch: unsetting the title box's third-party default
	// colours is the same act on both paths (the two main lists repeat it per
	// branch only because their TitleBar padding differs alongside it).
	l.Styles.Title = stripListTitleColours(l.Styles.Title)
	if m.colourless {
		colourlessHelpStyles(l)
		colourlessNoItemsStyle(l)
		colourlessPaginationDots(l)
		l.Styles.TitleBar = l.Styles.TitleBar.UnsetBackground()
		return
	}
	canvasHelpStyles(l, m.activeTheme)
	canvasNoItemsStyle(l, m.activeTheme)
	canvasPaginationDots(l, m.activeTheme)
	l.Styles.TitleBar = l.Styles.TitleBar.Background(m.activeTheme.Canvas.Color())
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
// NON-BLANKING AND KEY-EXCLUSIVE ARE NOT IN TENSION, which is worth saying because
// the pair reads like a contradiction: seeing the list without being able to drive it
// IS the live-preview premise (§9.1). The page stays fully rendered because it is what
// the theme is being judged against, not because it is still interactive.
//
// `d` AND `l` ARE PANEL-OWNED (§9.2 gives them the dark and light slots), not
// swallowed page keys. They fall through this switch's default while Phase 8 has
// nothing for them to commit, so today they are observationally inert — but what is
// asserted of them is that the PAGE's binding never fires (no Projects delete modal),
// which stays true once task 9-3 makes them write.
//
// THE NAVIGATION ARM SITS AHEAD OF THE SWALLOW-EVERYTHING DEFAULT and is matched
// against the panel LIST'S OWN KeyMap — the same way the Sessions page matches
// CursorUp / CursorDown / PrevPage / NextPage — so §12.2's arrow-only revision is
// stated once, at newThemePanelList's pinArrowOnlyNav, rather than restated as a
// second literal key list here. A key the keymap does not bind cannot match, so
// there is nothing to keep the two in step.
//
// `Esc` IS THE ONLY WAY OUT (§9.2 — `Enter` deliberately does not close), and it
// routes to closeThemePanel, which re-resolves persisted state before dropping
// what the panel retained. It is consumed HERE and never reaches the page
// beneath, where it is the progressive-back key: closing must not clear an
// applied filter, must not exit multi-select, and must not quit — the innermost
// surface resolves first, which is how the panel NESTS over multi-select (§9.7).
//
// `Enter` COMMITS AND STAYS (theme_panel_commit.go). Its arm sits ahead of the
// swallow-everything default and returns the panel exactly as it found it bar the
// write: no close, no re-theme, no cursor move. It is deliberately NOT an arm of
// the `Esc` case — one key, one job — because a dual-purpose exit is precisely
// what would let a user who had just set both slots wipe the pair on their way
// out.
//
// The ENTRY side of this routing is themePanelEntry / handleThemePanelKey above.
func (m Model) updateThemePanel(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyIsCtrlC(msg):
		return m, tea.Quit
	case keyIsCode(msg, tea.KeyEscape):
		(&m).closeThemePanel()
		return m, nil
	case keyIsCode(msg, tea.KeyEnter):
		// §9.2's commit-a-constant, and it deliberately does NOT close: the panel
		// stays open, the cursor stays where it is, and nothing is re-themed. The
		// error is DISCARDED HERE and only here — task 9-7 owns §9.13's message
		// slot line and the outstanding-failure state, and until it lands a failed
		// write is silent except for cmd's own `theme: commit failed` record. It
		// leaves nothing behind either way: on failure the raw keys are untouched,
		// so the `●` cannot move.
		_ = (&m).commitSelectedConstant()
		return m, nil
	case themePanelNavKey(m.themePanel.list.KeyMap, msg):
		return m, (&m).moveThemePanelCursor(msg)
	default:
		return m, nil
	}
}

// themePanelNavKey reports whether msg is one of the four keys §9.2 gives the
// panel's cursor: `↑`/`↓` to step and `Ctrl+↑`/`Ctrl+↓` to page.
//
// It matches against the LIVE KeyMap rather than against key literals, so the
// panel's routing and the list's own dispatch are driven by one binding set.
// GoToStart / GoToEnd are deliberately absent: pinArrowOnlyNav empties them
// (§12.2 drops `g`/`G` and `Home`/`End`), so there is nothing to route.
func themePanelNavKey(km list.KeyMap, msg tea.KeyPressMsg) bool {
	return key.Matches(msg, km.CursorUp, km.CursorDown, km.PrevPage, km.NextPage)
}

// moveThemePanelCursor is §9.2's arrow-preview in the three steps it is specified
// as: move, skip, preview.
//
// THE PREVIEW IS THE POINT — a panel that lists themes without showing them is a
// config screen with extra steps — and it takes the §11.1 RESTYLE and nothing else:
// Model.ApplyTheme, the production entry point §13.4's completeness guard drives,
// against a palette that is ALREADY IN HAND (§5.8). No file is read, no union is
// reassembled, no directory is touched and the session list is not rebuilt.
//
// OSC 11 NEEDS NOTHING HERE, DELIBERATELY. View assigns v.BackgroundColor
// declaratively from the active theme's canvas and Bubble Tea DIFFS it, so hovering
// N themes emits exactly once per DISTINCT canvas landed on (§11.3). The query is
// issued only from Init, so a later switch opens no new race and the canvas-echo
// guard needs no new handling — which is why there is no suppression and no
// debounce here, and why none should be added.
//
// The MIXED-MODE FLASH is likewise left alone: arrowing past a light theme in a
// dark terminal flips the whole canvas near-white and back, and §9.2 is explicit
// that this is the feature rather than a defect (ordering same-mode themes first
// was proposed as a mitigation and rejected).
func (m *Model) moveThemePanelCursor(msg tea.KeyPressMsg) tea.Cmd {
	var cmd tea.Cmd
	m.themePanel.list, cmd = m.themePanel.list.Update(msg)
	m.skipUnselectableThemeRow(msg)
	m.previewSelectedThemeRow()
	return cmd
}

// skipUnselectableThemeRow keeps the panel cursor off the unselectable rows after
// the list has processed a navigation key — §9.5's "arrow keys skip invalid rows,
// REUSING THE MECHANISM that already skips group-header rows on the Sessions
// list". It is model.go's skipHeaderRow applied to Row.Selectable, and it composes
// with paging for the same reason that one does: the step is taken on the list, so
// it crosses a page boundary exactly as an ordinary move would.
//
// TWO DELIBERATE DIFFERENCES FROM skipHeaderRow, both structural:
//
//   - IT LOOPS. No two group headers are ever adjacent, so one step always clears
//     one; several broken drop-ins in one directory are adjacent by nature, so one
//     step can land straight on another invalid row.
//   - IT REVERSES rather than falling off. skipHeaderRow flips an upward intent to
//     a downward step only at index 0; here either end can be reached with no
//     selectable row in the direction of travel, so the loop turns round at
//     whichever boundary it hits and settles on the NEAREST selectable row back
//     along the way it came. After a single-row step that is the row the cursor
//     started from, since everything between was walked and rejected — but after a
//     PAGE move it need not be, because the page jumps OVER rows without checking
//     them: on `[V,V,I,I]` at PerPage=2, `Ctrl+↓` from index 0 lands on 2, walks
//     down to 3, reverses, and settles on 1, one row FORWARD of the start. The
//     nearest selectable row is the right answer in both, and the 2×N bound below
//     covers the longer walk either way.
//
// THE BOUND IS TWICE THE ROW COUNT, and the doubling is the reversal's: the walk
// can reach a boundary and then retrace the whole span it just covered. Built-ins
// are always valid (§7.6's build-time guarantee), so a union with no selectable row
// is unreachable and the loop always terminates on the check — the bound exists so
// that a future all-invalid union degrades instead of spinning on a keypress.
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

// previewSelectedThemeRow applies the cursor's row to the whole frame — §9.2's "the
// app re-themes live behind the panel", and the one place the panel writes to the
// rendered palette.
//
// The palette comes off the ROW, which carries it because the enumeration parsed it
// at open and the panel retains the results for its lifetime (§5.8). That retention
// is what keeps the swap the O(1) restyle of §11.1: there is nothing here to read.
//
// A row already painting the screen is skipped, so an arrow that could not move —
// or a reversal that turned back to the row it started on — costs no restyle at all.
// A PAGE reversal can settle on a DIFFERENT row than it began on, which is an
// ordinary swap and is restyled as one. Swapping to the active theme is a legal
// no-op in any case (ApplyTheme is idempotent per swap); skipping it makes that
// explicit rather than incidental.
//
// An unselectable row is never previewed: it carries no palette, and the skip above
// is what makes the case unreachable in production. Checking anyway is what keeps a
// union with no selectable row at all — the shape the skip's bound also degrades on
// — from painting a zero Theme, which renders silently colourless.
func (m *Model) previewSelectedThemeRow() {
	row, ok := selectedThemeRow(m.themePanel.list)
	if !ok || !row.Selectable() || row.Theme == m.activeTheme {
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

// themePanelInnerWidth is the content width inside the left border AND the inner
// gutter — every panel row is composed against it and the list is sized to it.
//
// Both columns are charged exactly once, here, so no renderer has to know the panel
// has a gutter at all: themePanelBlock lays the two down and every surface composes
// against what is left.
func themePanelInnerWidth(width int) int {
	return max(width-themePanelBorderWidth-themePanelGutterWidth, 0)
}

// themePanelListSize is the (width, height) the panel's list is sized to at a given
// render height: the inner content width, and §9.1's layout remainder —
//
//	height − header − directory row(0 or 1) − message slot(0 or 1) − footer
//
// floored at one row. ALL FOUR subtrahends are MEASURED off the renderer that
// produces them (themePanelHeaderRows, themePanelDirRowHeight,
// themePanelMessageHeight, themePanelFooterHeight), so those reserved rows are by
// construction the rows that render and the two can never drift — the same
// discipline sessionFooterHeight applies on the main screen.
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
// the layout can be resolved before a theme is in hand (themePanelMinHeight's floor
// arithmetic reads the same measurements).
//
// The footer's entries are the panel scope's, resolved here and again at render
// time. Phase 9's nested confirm scope (§9.2) TEMPORARILY REPLACES that footer with
// a shorter one, so whatever threads the live scope through must thread it through
// BOTH — a budget reserving four rows while a two-row footer renders would leave two
// rows of the panel unaccounted for.
func themePanelListSize(p themePanel, height int) (width, rows int) {
	inner := themePanelInnerWidth(p.width)
	reserved := themePanelHeaderRows() +
		themePanelDirRowHeight(p.union.DirUnusable) +
		themePanelMessageHeight(p.message, inner, themePanelMessageWraps(p, height)) +
		themePanelFooterHeight(themePanelKeymap())
	return inner, max(height-reserved, themePanelMinBodyRows)
}

// renderThemePanel renders the §9.1 slide-over as a block of EXACTLY height rows,
// each EXACTLY p.width cells, laid out top to bottom as:
//
//	header (measured) · directory row (0 or 1) · list body · message slot (0 or 1) · footer
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

	rows := themePanelHeaderBlock(p.width, th, colourless)
	rows = appendBlock(rows, themePanelDirRow(p.union.DirUnusable, th, colourless))
	rows = appendBlock(rows, clampBlockHeight(p.list.View(), bodyRows))
	rows = appendBlock(rows, renderThemePanelMessage(p.message, inner, themePanelMessageWraps(p, height), th, colourless))
	rows = appendBlock(rows, renderThemePanelFooter(themePanelKeymap(), inner, th, colourless))

	return themePanelBlock(rows, height, p.width, th, colourless)
}

// themePanelHeaderBlock is §9.1's header region, cut to the PAGE's rhythm: a rule
// in the page's own rule lane, the label `Themes` in accent.mode (bold) on the
// page's section-header row, and blank rows everywhere else — one per row the page
// spends on its header block and its section header (see the measurement note above
// themePanelHeaderRuleRow).
//
// It carries NO THEME COUNT, deliberately: noise at this list size.
//
// THE REGION ABOVE THE RULE CARRIES NOTHING, BY DECISION rather than by omission.
// `esc close` was considered for it and rejected: Portal's modals carry no such
// header affordance, and the key stays in the panel's own vertical key list. The
// panel's body and the page's are painted the SAME `canvas` token, so those blank
// rows are indistinguishable from the page's own canvas — which is what lets the
// page's header band read as uninterrupted across the full width.
//
// THE RULE SPANS THE PANEL'S WHOLE WIDTH, border column included, and that is why
// the left border starts BELOW it (themePanelBorderFromRow). The panel is an opaque
// layer, so it covers the right end of the page's rule; drawing its own across every
// one of its columns is what CONTINUES that rule to the frame edge. A `│` in the
// rule's lane would notch it, and a border running the full height would cut the
// page's header band in two — which is precisely what makes a slide-over read as a
// second column rather than as a surface inside the content region.
func themePanelHeaderBlock(width int, th theme.Theme, colourless bool) []string {
	rows := make([]string, themePanelHeaderRows())
	rows[themePanelHeaderRuleRow()] = headerStyle(th.Border, th, colourless).
		Render(strings.Repeat(headerRuleGlyph, max(width, 0)))
	rows[themePanelHeaderLabelRow()] = headerStyle(th.AccentMode, th, colourless).Bold(true).
		Render(themePanelHeaderLabel)
	return rows
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

// renderThemePanelMessage renders §9.1's message slot: the row (or two) directly
// above the vertical keymap footer, or "" when there is no message.
//
// IT IS NOT RESERVED WHEN EMPTY — it appears and the list shrinks by one, the same
// way the main screen's notice band recomputes list height.
//
// THE TWO DIMENSIONS DEGRADE DIFFERENTLY, ON PURPOSE (§9.1). At the minimum panel
// WIDTH the slot may wrap to two rows — it is not a list delegate, so wrapping costs
// pagination nothing. At the minimum HEIGHT it is TRUNCATED to one line instead,
// because §9.8's floor counts exactly one message row: a two-row message there would
// leave zero list rows or overflow the frame, and truncating degrades the message
// the user is being asked to answer rather than the row they are answering ABOUT.
// wrap carries that decision in from the caller (themePanelMessageWraps), so the
// budget above and the block below resolve it once and identically.
//
// The wrap is CAPPED at themePanelMessageWrapRows with the overflow truncated into
// the last row, so the slot's cost is bounded whatever it is handed (see that
// constant). A zero or negative inner width takes the truncating path, which is also
// the only width ansi.Wrap declines to act on.
//
// The slot is a single-slot ARBITER with two contenders, and BOTH ARE PHASE 9's —
// the slot-from-constant confirm (§9.2, text.secondary, no band) and the failed
// commit write (§9.13, `⚠` and text in accent.attention). Phase 8 ships neither, so
// the text renders in the confirm's role and Phase 9 adds the discrimination along
// with the states that need it. There is deliberately no contender, setter or
// arbiter here.
func renderThemePanelMessage(message string, inner int, wrap bool, th theme.Theme, colourless bool) string {
	if message == "" {
		return ""
	}
	return headerStyle(th.TextSecondary, th, colourless).Render(themePanelMessageText(message, inner, wrap))
}

// themePanelMessageText lays §9.1's message out for the slot — one truncated line,
// or up to themePanelMessageWrapRows wrapped ones with the overflow truncated into
// the last.
func themePanelMessageText(message string, inner int, wrap bool) string {
	width := max(inner, 0)
	if !wrap || width == 0 {
		return ansi.Truncate(message, width, themeRowEllipsis)
	}
	lines := strings.Split(ansi.Wrap(message, width, ""), "\n")
	if len(lines) <= themePanelMessageWrapRows {
		return strings.Join(lines, "\n")
	}
	head := lines[:themePanelMessageWrapRows-1]
	tail := ansi.Truncate(strings.Join(lines[themePanelMessageWrapRows-1:], " "), width, themeRowEllipsis)
	return strings.Join(append(head, tail), "\n")
}

// themePanelMessageHeight is the message slot's MEASURED height — the value
// themePanelListSize subtracts, so a message appearing costs the list exactly the
// rows the slot renders. It measures the real renderer (with a zero theme and the
// colourless path, as themePanelFooterHeight does), so the two cannot drift.
func themePanelMessageHeight(message string, inner int, wrap bool) int {
	return blockHeight(renderThemePanelMessage(message, inner, wrap, theme.Theme{}, true))
}

// themePanelMessageWraps reports whether the slot may wrap at the height the panel
// is being rendered at — §9.1's per-dimension rule resolved in ONE place, so the
// budget themePanelListSize reserves and the block renderThemePanel assembles can
// never disagree about how many rows the message costs.
//
// At or below §9.8's floor the slot truncates; above it, it wraps. `at or below`
// rather than `at`, because renderThemePanel is a pure function of the height it is
// handed and a sub-floor height is a shape a fixture can hand it.
func themePanelMessageWraps(p themePanel, height int) bool {
	return height > themePanelMinHeight(themePanelKeymap(), p.union.DirUnusable)
}

// themePanelBlock assembles the panel's rows into the finished block: each row
// below the header rule prefixed with the one `border`-coloured `│` cell and the
// inner gutter, each row above it laid down bare, every row padded out to exactly
// width cells with the owned canvas, and the whole clamped and padded to exactly
// height rows.
//
// LEFT BORDER ONLY — no top, bottom or right edge. That is what makes the panel read
// as a slide-over rather than as an inset bordered dialog like the modals (§9.1), and
// it is the only thing distinguishing the panel from the list behind it.
//
// THE BORDER STARTS BELOW THE HEADER RULE, not at the top of the frame. It is the
// one prefix decision this function makes for itself, and it is what makes the
// slide-over read as a surface INSIDE the content region: above the rule the panel
// contributes nothing but canvas, so the page's header band and the rule beneath it
// run unbroken to the frame edge (themePanelHeaderBlock states the other half — the
// rule the panel draws in its own full width is what carries the page's rule across
// the columns the panel covers). A `│` running from row 0 cuts that band in two, and
// the panel then reads as a second column beside the page rather than as a layer over
// it.
//
// THE GUTTER IS CHARGED HERE AND NOWHERE ELSE, which is what makes it uniform: every
// surface below the rule — the label, the list rows with their cursor column, the
// pinned directory row, the message slot and the vertical key list — is composed
// against themePanelInnerWidth and laid down after the same two columns, so none of
// them can sit at a different inset from the others.
//
// Rows are padded but never truncated: every row this file composes is authored to
// fit the minimum inner width (the header label is 6 cells, the pinned warning 16,
// the widest footer row 15, and a list row is exactly inner cells by construction),
// and below §9.8's minimum the panel refuses to open at all (themePanelFloor) — so there
// is no width at which a row has to degrade.
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

// themePanelPadRow pads one composed row out to w cells with the owned canvas, with
// an EMPTY row rendered as a whole canvas blank rather than joined against one — the
// header region's unbordered rows carry no content at all, and lipgloss's horizontal
// join has no defined geometry for a zero-width segment.
func themePanelPadRow(row string, w int, th theme.Theme, colourless bool) string {
	if row == "" {
		return blankCanvasRow(max(w, 0), th, colourless)
	}
	return headerPadRight(row, lipgloss.Width(row), w, th, colourless)
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
