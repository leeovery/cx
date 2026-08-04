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
func (m Model) openThemePanel() (tea.Model, tea.Cmd) {
	if m.themeEnumerator == nil {
		return m, nil
	}

	enumeration, union := m.themeEnumerator.Open(m.themeKeys)
	(&m).armThemePanel(enumeration, union)
	return m, nil
}

// armThemePanel installs one enumeration's results as the live panel state, and
// §9.2's opening state with them: the badges, the theme actually rendering, and
// the row the cursor lands on.
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
func (m *Model) armThemePanel(enumeration theme.Enumeration, union theme.Union) {
	m.themePanel = themePanel{
		enumeration: enumeration,
		union:       union,
		width:       themePanelPreferredWidth,
	}
	cursor := m.applyThemePanelResolution(enumeration)
	m.themePanel.list = newThemePanelList(m.themePanel.rowItems(), m.themeRowDelegate())
	m.themePanel.open = true
	m.applyThemePanelListStyles()
	m.anchorThemePanelCursor(cursor)
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

// themePanelRowIndex is the index of the row identified by slug, degrading to the
// first SELECTABLE row and then to zero.
//
// The identity is Row.SortKey — the slug wherever one exists, else the filename,
// else the raw persisted string — which is the same value the ordering and the
// badge lookup key on, so a row can be found by exactly what it is listed under.
// A `reserved name` row shares its key with the built-in it collides with by
// definition (§6.2), and the built-in sorts first (§9.5), so the first match is
// always the selectable one the slug actually resolved to.
//
// THE CLAMP IS A STRUCTURAL GUARD, NOT A LIVE PATH. Built-ins are always valid
// (§7.6's build-time guarantee), so a union with no selectable row is unreachable
// and so is one holding no row for a slug that just resolved. The guard is here
// because the alternative to degrading is indexing out of range inside a list the
// user is looking at.
func themePanelRowIndex(rows []theme.Row, slug string) int {
	if at := slices.IndexFunc(rows, func(row theme.Row) bool { return row.SortKey() == slug }); at >= 0 {
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
// was open is already handled on its own path — resyncSessionLayout on Sessions,
// and on Projects the projectBandHeight reserve inside applyProjectListSize;
// closing adds nothing to either.
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
// unimplemented on a panel that renders as though it were. Task 8-11 owns the RESIZE
// path (re-applying this on a tea.WindowSizeMsg); it does not own the panel's page
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
// applied filter, must not exit multi-select, and must not quit. Task 8-13 owns
// the entry conditions and the blocked-`t` flashes.
func (m Model) updateThemePanel(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	switch {
	case keyIsCtrlC(msg):
		return m, tea.Quit
	case keyIsCode(msg, tea.KeyEscape):
		(&m).closeThemePanel()
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
