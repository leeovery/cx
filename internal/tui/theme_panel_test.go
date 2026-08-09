package tui

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// The panel layout slide-over surface gate. These tests pin the panel's block geometry
// (exactly height × width, left border only from the header rule down), its
// page-measured countless header and its inner gutter, the
// the `⚠ dir unreadable` row as pinned CHROME rather than a list delegate, the
// message slot's unreserved-when-empty budget, the re-theme-everything rule requirement that
// every chrome surface re-derives from the previewed theme, and the compositor wiring
// that puts the panel over the page without re-laying the page out.
//
// Colour roles are asserted as theme-resolved SGR runs (like the session-row and
// panel-row anatomy tests), so a token swap is caught rather than merely the
// presence of a glyph.
//
// No t.Parallel() — the package-level mock convention and the shared canvas
// helpers make parallelism unsafe across this package's tests.

// themePanelTestBodyRows / themePanelTestMessageRows are the test's own reading of
// the geometry rule's floor arithmetic: one list row and one message row. They are literals
// here — deliberately NOT re-derived from the production layout — so the floor the
// tests scan up from is an independent statement of the same rule.
const (
	themePanelTestBodyRows    = 1
	themePanelTestMessageRows = 1
)

// themePanelFixtureOpts is the panel state one test case needs. It is a struct
// rather than a parameter list because the fixture takes six inputs and a
// positional call would read as a row of anonymous literals.
type themePanelFixtureOpts struct {
	th          theme.Theme
	colourless  bool
	width       int
	rows        []theme.Row
	dirUnusable bool
	message     themePanelMessage
}

// newThemePanelFixture builds an OPEN panel over the given rows, wiring the list
// exactly as the production open sequence does: the panel's own list constructor
// and a row delegate carrying the same theme / colourless / inner width the panel
// renders at.
func newThemePanelFixture(o themePanelFixtureOpts) themePanel {
	items := make([]list.Item, 0, len(o.rows))
	for _, row := range o.rows {
		items = append(items, themeRowItem{Row: row})
	}
	delegate := themeRowDelegate{
		Theme:      o.th,
		Colourless: o.colourless,
		Width:      themePanelInnerWidth(o.width),
	}
	return themePanel{
		open:    true,
		list:    newThemePanelList(items, delegate),
		union:   theme.Union{Rows: o.rows, DirUnusable: o.dirUnusable, Count: len(o.rows)},
		message: o.message,
		width:   o.width,
	}
}

// themePanelTestRows returns n selectable built-in rows with predictable slugs, so
// a test can ask for "enough rows to paginate" without inventing a union each time.
func themePanelTestRows(n int) []theme.Row {
	rows := make([]theme.Row, 0, n)
	for i := range n {
		rows = append(rows, theme.Row{Slug: fmt.Sprintf("theme-%02d", i), Source: theme.SourceBuiltin})
	}
	return rows
}

// themePanelLines splits a rendered block into its rows with every SGR sequence
// stripped — the text a user actually reads, border cell included.
func themePanelLines(block string) []string {
	return strings.Split(ansi.Strip(block), "\n")
}

// themePanelContentPrefix is the cells every BORDERED panel row opens with — the panel
// layout's left `│` plus the inner gutter — so a row assertion states the panel's inset once
// rather than restating it at each call site.
func themePanelContentPrefix() string {
	return panelFrameSide + strings.Repeat(" ", themePanelGutterWidth)
}

// themePanelColourParams collects the distinct 24-bit colour SGR parameter lists a
// rendered block emits. Two renders of the same state under two themes must share
// none of them (the panel's own chrome re-themes, no exceptions).
func themePanelColourParams(block string) map[string]bool {
	params := map[string]bool{}
	for _, chunk := range strings.Split(block, "\x1b[")[1:] {
		p, _, ok := strings.Cut(chunk, "m")
		if !ok {
			continue
		}
		if strings.Contains(p, "38;2;") || strings.Contains(p, "48;2;") {
			params[p] = true
		}
	}
	return params
}

// TestThemePanel_BlockGeometry is the panel layout's shape made executable: the panel is a
// block of EXACTLY height rows, each EXACTLY width cells, at every height from its
// own floor upward and at both ends of the geometry rule's width ladder.
//
// The floor moves with the state — the pinned directory row and a live message
// each cost a viewport row — so each case scans up from its own.
func TestThemePanel_BlockGeometry(t *testing.T) {
	th := testDarkTheme(t)
	footerRows := themePanelFooterHeight(themePanelKeymap())

	for _, width := range []int{themePanelMinWidth, themePanelPreferredWidth} {
		for _, dirUnusable := range []bool{false, true} {
			for _, message := range []themePanelMessage{{}, messageTestConfirm()} {
				floor := themePanelHeaderRows() + themePanelTestBodyRows + footerRows
				if dirUnusable {
					floor++
				}
				if message.Kind != themeMessageNone {
					floor += themePanelTestMessageRows
				}

				for height := floor; height <= floor+12; height++ {
					name := fmt.Sprintf("w=%d/dir=%v/msg=%v/h=%d", width, dirUnusable, message.Kind != themeMessageNone, height)
					t.Run(name, func(t *testing.T) {
						p := newThemePanelFixture(themePanelFixtureOpts{
							th:          th,
							width:       width,
							rows:        themePanelTestRows(12),
							dirUnusable: dirUnusable,
							message:     message,
						})
						block := renderThemePanel(p, height, th, false)

						if got := lipgloss.Height(block); got != height {
							t.Fatalf("block height = %d, want %d", got, height)
						}
						for i, line := range strings.Split(block, "\n") {
							if got := lipgloss.Width(line); got != width {
								t.Errorf("line %d width = %d, want %d: %q", i, got, width, ansi.Strip(line))
							}
						}
					})
				}
			}
		}
	}
}

// TestThemePanel_FooterSurvivesAtTheFloor is the case TestThemePanel_BlockGeometry
// structurally cannot see: it scans the same heights but asserts only the block's
// TOTAL height and per-line width, both of which stay correct while the panel's
// bottom rows are being eaten.
//
// They can be eaten because `bubbles/list` has a hard minimum rendered height of
// three rows (one item, a blank, the paginator) whatever height it is given, while
// the geometry rule's floor budgets the body ONE row. At every floor case the list therefore
// overshoots its budget, and the assembly's clamp drops the overshoot off the
// BOTTOM of the block — off the footer, silently, keymap first.
//
// That is the failure the geometry rule exists to prevent, reached in band: on an 11-row
// terminal raising the picker idiom's confirm shrinks the body budget to two and takes `esc
// close` with it — the one key that closes a panel the user can no longer read the way out of.
//
// So the footer is asserted WHOLE — against its own renderer, row for row — at each
// of the four floor cases (the pinned directory row and a live message each cost a
// viewport row, the row-rendering rule / the geometry rule), together with the message slot
// that is due directly above it.
//
// The footer it is asserted against is the SLOT'S OWN SCOPE (the picker idiom's nested confirm
// scope while the confirm is live), because that is the footer the panel renders —
// and each case's floor is scanned from that footer's height, so the confirm's
// shorter one is exercised at ITS floor rather than at the standing footer's.
func TestThemePanel_FooterSurvivesAtTheFloor(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelPreferredWidth)

	for _, dirUnusable := range []bool{false, true} {
		for _, message := range []themePanelMessage{{}, messageTestConfirm()} {
			wantFooter := themePanelLines(renderThemePanelFooter(themePanelFooterScope(message), inner, th, false))
			floor := themePanelHeaderRows() + themePanelTestBodyRows + len(wantFooter)
			if dirUnusable {
				floor++
			}
			if message.Kind != themeMessageNone {
				floor += themePanelTestMessageRows
			}

			name := fmt.Sprintf("dir=%v/msg=%v/h=%d", dirUnusable, message.Kind != themeMessageNone, floor)
			t.Run(name, func(t *testing.T) {
				p := newThemePanelFixture(themePanelFixtureOpts{
					th:          th,
					width:       themePanelPreferredWidth,
					rows:        themePanelTestRows(12),
					dirUnusable: dirUnusable,
					message:     message,
				})
				lines := themePanelLines(renderThemePanel(p, floor, th, false))

				footer := lines[len(lines)-len(wantFooter):]
				for i, want := range wantFooter {
					if got, wantRow := footer[i], themePanelContentPrefix()+want; got != wantRow {
						t.Errorf("footer row %d = %q, want %q", i, got, wantRow)
					}
				}
				if message.Kind == themeMessageNone {
					return
				}
				slot := lines[len(lines)-len(wantFooter)-1]
				// The floor is the height at which the panel layout truncates rather than wraps, so
				// the expected slot is the truncating render.
				want := themePanelContentPrefix() + themePanelLines(renderThemePanelMessage(message, inner, false, th, false))[0]
				if got := strings.TrimRight(slot, " "); got != want {
					t.Errorf("message slot = %q, want %q", got, want)
				}
			})
		}
	}
}

// TestThemePanel_LeftBorderOnly is the panel layout's "left border only — deliberately not an
// inset bordered panel like the modals": every row from the header rule down opens
// with the border-coloured `│`, no row above it carries one, and NO other frame
// glyph is emitted anywhere in the block.
//
// The absence assertions are what make it a SLIDE-OVER rather than a floating
// dialog: a top/bottom edge or a right side would read as the modals' frame, which
// is exactly the shape the panel layout refuses. The border's ORIGIN is the same argument one
// row further in — a `│` from row 0 cuts the page's header band in two, and the
// panel reads as a second column beside the page rather than as a layer over it.
func TestThemePanel_LeftBorderOnly(t *testing.T) {
	th := testDarkTheme(t)
	p := newThemePanelFixture(themePanelFixtureOpts{
		th:    th,
		width: themePanelPreferredWidth,
		rows:  themePanelTestRows(6),
	})
	block := renderThemePanel(p, 16, th, false)

	for i, line := range themePanelLines(block) {
		runes := []rune(line)
		if len(runes) == 0 {
			t.Fatalf("line %d is empty", i)
		}
		want := panelFrameSide
		switch {
		case i < themePanelHeaderRuleRow():
			want = " "
		case i == themePanelHeaderRuleRow():
			// The rule runs THROUGH the border's column: it is what carries the page's
			// own rule across the columns the panel covers, so a `│` here would notch it.
			want = headerRuleGlyph
		}
		if got := string(runes[0]); got != want {
			t.Fatalf("line %d opens with %q, want %q (the border starts on row %d)", i, got, want, themePanelBorderFromRow())
		}
		if strings.Contains(string(runes[1:]), panelFrameSide) {
			t.Errorf("line %d carries a second %q (a right edge): %q", i, panelFrameSide, line)
		}
	}

	for _, glyph := range []string{
		panelFrameTopLeft, panelFrameTopRight,
		panelFrameBottomLeft, panelFrameBottomRight,
		panelFrameTeeLeft, panelFrameTeeRight,
		panelRuleGlyph,
	} {
		if strings.Contains(ansi.Strip(block), glyph) {
			t.Errorf("block carries the inset-frame glyph %q — the panel draws a left border only", glyph)
		}
	}

	// The border cell is the `border` token, not an accent: it is the ONLY thing
	// distinguishing the panel from the list behind it. It is read from the
	// rows BELOW the header rule, which is where the border lives — the rule itself is
	// the same token, so the first border-painted run in the whole block is its.
	bordered := strings.Join(strings.Split(block, "\n")[themePanelBorderFromRow():], "\n")
	if got := themeRowRunAfter(t, bordered, tokenFgSeq(t, th.Border)); !strings.HasPrefix(got, panelFrameSide) {
		t.Errorf("the border run does not paint %q, got %q", panelFrameSide, got)
	}
}

// TestThemePanel_HeaderIsMeasuredAndCountless pins the panel layout's header region within the
// block: a full-width `border` rule in the rule lane, the label `Themes` in
// accent.mode on the section-header row, NOTHING anywhere else in the region, and NO
// theme count.
//
// The row COUNTS are the composed frame's business (theme_panel_chrome_test.go) —
// this is the block's own anatomy: which row carries what, that the rule spans every
// column so it continues the page's to the frame edge, and that the region above it
// is empty by decision rather than by omission.
func TestThemePanel_HeaderIsMeasuredAndCountless(t *testing.T) {
	th := testDarkTheme(t)
	rows := themePanelTestRows(7)
	p := newThemePanelFixture(themePanelFixtureOpts{
		th:    th,
		width: themePanelPreferredWidth,
		rows:  rows,
	})
	block := renderThemePanel(p, 16, th, false)
	lines := themePanelLines(block)

	rule, label := themePanelHeaderRuleRow(), themePanelHeaderLabelRow()
	if got, want := lines[rule], strings.Repeat(headerRuleGlyph, themePanelPreferredWidth); got != want {
		t.Errorf("header rule row = %q, want the glyph across all %d panel columns", got, themePanelPreferredWidth)
	}
	if got, want := strings.TrimRight(lines[label], " "), themePanelContentPrefix()+themePanelHeaderLabel; got != want {
		t.Errorf("header label row = %q, want %q", got, want)
	}
	for i := range themePanelHeaderRows() {
		if i == rule || i == label {
			continue
		}
		if got := strings.TrimSpace(lines[i]); got != "" && got != panelFrameSide {
			t.Errorf("header row %d carries %q, want nothing — the region's other rows are blank by decision", i, lines[i])
		}
	}
	if strings.ContainsAny(lines[rule]+lines[label], "0123456789") {
		t.Errorf("the header carries a count: %q / %q", lines[rule], lines[label])
	}
	// The header ends where the list body begins.
	if !strings.Contains(lines[themePanelHeaderRows()], rows[0].Label()) {
		t.Errorf("row %d is not the first list row (%q): %q", themePanelHeaderRows(), rows[0].Label(), lines[themePanelHeaderRows()])
	}

	if got := themeRowRunAfter(t, block, tokenFgSeq(t, th.AccentMode)); got != themePanelHeaderLabel {
		t.Errorf("accent.mode paints %q, want %q", got, themePanelHeaderLabel)
	}
}

// TestThemePanel_DirUnreadableIsPinnedChrome is the row-rendering rule's central claim about
// the `⚠ dir unreadable` row: it is CHROME pinned directly beneath the header, not a
// list delegate.
//
// The proof is page 2. A list row participates in pagination, so a delegate would
// vanish the moment the user paged down — and this row is what stands between the
// user and the "completely in the dark" state, which a page-1-only warning does not
// do. It is asserted at the same line index on both pages, and asserted absent from
// the list's own items.
func TestThemePanel_DirUnreadableIsPinnedChrome(t *testing.T) {
	th := testDarkTheme(t)
	rows := themePanelTestRows(24)
	p := newThemePanelFixture(themePanelFixtureOpts{
		th:          th,
		width:       themePanelPreferredWidth,
		rows:        rows,
		dirUnusable: true,
	})

	const height = 14
	firstPage := themePanelLines(renderThemePanel(p, height, th, false))
	if got, want := strings.TrimRight(firstPage[themePanelHeaderRows()], " "), themePanelContentPrefix()+themePanelDirUnreadable; got != want {
		t.Fatalf("page 1 directory row = %q, want %q", got, want)
	}

	// Park the cursor deep enough that the list is on a later page, then re-render.
	p.list.Select(len(rows) - 1)
	lastPage := themePanelLines(renderThemePanel(p, height, th, false))
	if got, want := strings.TrimRight(lastPage[themePanelHeaderRows()], " "), themePanelContentPrefix()+themePanelDirUnreadable; got != want {
		t.Errorf("last page directory row = %q, want %q — the row is paginating, so it is a list item", got, want)
	}
	if strings.Join(firstPage, "\n") == strings.Join(lastPage, "\n") {
		t.Fatal("the list did not page: the page-2 assertion proves nothing")
	}

	for i, item := range p.list.Items() {
		row, ok := item.(themeRowItem)
		if !ok {
			t.Fatalf("list item %d is not a themeRowItem: %T", i, item)
		}
		if row.Row.Label() == themePanelDirUnreadable {
			t.Errorf("the directory warning is a list item at index %d", i)
		}
	}

	block := renderThemePanel(p, height, th, false)
	if got := themeRowRunAfter(t, block, tokenFgSeq(t, th.AccentAttention)); got != themePanelDirUnreadable {
		t.Errorf("accent.attention paints %q, want %q", got, themePanelDirUnreadable)
	}
}

// TestThemePanel_RowsRenderBeneathDirRow is the row-rendering rule's other half: built-in rows
// and persisted-slug rows STILL render beneath the pinned warning — the persisted rows
// especially, or a user with an unreadable directory loses the `●` entirely.
func TestThemePanel_RowsRenderBeneathDirRow(t *testing.T) {
	th := testDarkTheme(t)
	builtin := theme.Row{Slug: "tokyo-night", Source: theme.SourceBuiltin}
	persisted := theme.Row{
		Slug:      "zzz-gone",
		Source:    theme.SourcePersisted,
		Rejection: &theme.Rejection{Reason: theme.ReasonUnreadable},
	}
	p := newThemePanelFixture(themePanelFixtureOpts{
		th:          th,
		width:       themePanelPreferredWidth,
		rows:        []theme.Row{builtin, persisted},
		dirUnusable: true,
	})
	p.list.SetDelegate(themeRowDelegate{Theme: th, Width: themePanelInnerWidth(themePanelPreferredWidth)})
	p.list.Items()[1] = themeRowItem{Row: persisted, Badge: theme.BadgeDark}

	lines := themePanelLines(renderThemePanel(p, themePanelHeaderRows()+10, th, false))
	below := strings.Join(lines[themePanelHeaderRows()+1:], "\n")

	for _, want := range []string{builtin.Label(), persisted.Label(), themePanelBadgeText(theme.BadgeDark)} {
		if !strings.Contains(below, want) {
			t.Errorf("nothing beneath the directory row carries %q:\n%s", want, below)
		}
	}
}

// TestThemePanel_DirRowFitsMinimumWidthUntruncated pins the row-rendering rule's deliberate
// 16-column copy: none of the row-rendering rule's four composition priorities apply to this
// row (no label, no badge, no reason), so the truncation-floor argument does not transfer to
// it — it has to FIT.
func TestThemePanel_DirRowFitsMinimumWidthUntruncated(t *testing.T) {
	th := testDarkTheme(t)

	if got, want := themePanelDirUnreadable, flashWarningGlyph+" dir unreadable"; got != want {
		t.Fatalf("directory row copy = %q, want %q", got, want)
	}
	const wantCells = 16
	if got := lipgloss.Width(themePanelDirUnreadable); got != wantCells {
		t.Fatalf("directory row copy is %d cells, want %d", got, wantCells)
	}
	if inner := themePanelInnerWidth(themePanelMinWidth); wantCells > inner {
		t.Fatalf("the %d-cell copy does not fit the %d-cell minimum inner width", wantCells, inner)
	}

	p := newThemePanelFixture(themePanelFixtureOpts{
		th:          th,
		width:       themePanelMinWidth,
		rows:        themePanelTestRows(3),
		dirUnusable: true,
	})
	lines := themePanelLines(renderThemePanel(p, 10, th, false))
	row := strings.TrimRight(lines[themePanelHeaderRows()], " ")
	if want := themePanelContentPrefix() + themePanelDirUnreadable; row != want {
		t.Errorf("at the minimum width the directory row = %q, want %q", row, want)
	}
	if strings.Contains(row, themeRowEllipsis) {
		t.Errorf("the directory row was truncated at the minimum width: %q", row)
	}
}

// TestThemePanel_MessageSlotUnreservedWhenEmpty is the panel layout's "not reserved when
// empty": an empty message renders NOTHING and costs NO row of the panel's
// vertical budget.
//
// The wrap flag (the per-dimension degrade rule) is passed both ways, because
// "unreserved when empty" is a statement about the slot at every height — the
// message-shortage rules govern a slot that HAS something in it.
func TestThemePanel_MessageSlotUnreservedWhenEmpty(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelPreferredWidth)

	for _, wrap := range []bool{false, true} {
		if got := renderThemePanelMessage(themePanelMessage{}, inner, wrap, th, false); got != "" {
			t.Errorf("an empty message rendered %q with wrap=%v, want the empty string", got, wrap)
		}
		if got := themePanelMessageHeight(themePanelMessage{}, inner, wrap); got != 0 {
			t.Errorf("an empty message reserved %d rows with wrap=%v, want 0", got, wrap)
		}
		if got := themePanelMessageHeight(messageTestConfirm(), inner, wrap); got != themePanelTestMessageRows {
			t.Errorf("a live message reserved %d rows with wrap=%v, want %d", got, wrap, themePanelTestMessageRows)
		}
	}
}

// TestThemePanel_MessageSlotRecomputesListHeight is the other half of the panel layout's slot
// rule: the message appears and the list shrinks by one, exactly the way the main
// screen's notice band recomputes list height.
//
// It is driven by setting the field directly, since raising either contender is a
// commit-path dispatch. The contender is the FAILED-COMMIT line rather than
// the picker idiom's confirm, because the confirm additionally substitutes its own shorter
// footer (the picker idiom's nested scope) and would net the list a row rather than costing it
// one — the substitution has its own gate in theme_panel_message_test.go, and this
// one is about the slot alone.
func TestThemePanel_MessageSlotRecomputesListHeight(t *testing.T) {
	th := testDarkTheme(t)
	const height = 14
	opts := themePanelFixtureOpts{
		th:    th,
		width: themePanelPreferredWidth,
		rows:  themePanelTestRows(12),
	}

	empty := newThemePanelFixture(opts)
	opts.message = messageTestFailed()
	live := newThemePanelFixture(opts)

	_, emptyRows := themePanelListSize(empty, height)
	_, liveRows := themePanelListSize(live, height)
	if liveRows != emptyRows-1 {
		t.Errorf("list body with a message = %d rows, want %d (one fewer than %d)", liveRows, emptyRows-1, emptyRows)
	}

	// The rendered block still totals exactly height, and the message occupies the
	// single row directly above the footer.
	block := renderThemePanel(live, height, th, false)
	if got := lipgloss.Height(block); got != height {
		t.Fatalf("block height with a message = %d, want %d", got, height)
	}
	lines := themePanelLines(block)
	slot := lines[height-themePanelFooterHeight(themePanelKeymap())-1]
	if !strings.Contains(slot, themePanelCommitFailedMessage) {
		t.Errorf("the row above the footer is not the message slot: %q", slot)
	}
}

// TestThemePanel_ListIsConstructedWithPanelChromeDisabled pins the panel's own
// `bubbles/list` construction: the panel supplies its own header, footer and
// warning chrome, so the list's title, status bar, help and filtering are all off
// (the panel layout — panel search is deferred by decision).
//
// It also pins the sizing contract: the list is fed the panel's INNER width (the
// left border is not list space) and the computed body height.
func TestThemePanel_ListIsConstructedWithPanelChromeDisabled(t *testing.T) {
	l := newThemePanelList(nil, themeRowDelegate{})

	if l.FilteringEnabled() {
		t.Error("the panel's list has filtering enabled")
	}
	if l.ShowTitle() {
		t.Error("the panel's list shows a title")
	}
	if l.ShowStatusBar() {
		t.Error("the panel's list shows a status bar")
	}
	if l.ShowHelp() {
		t.Error("the panel's list shows its own help")
	}

	th := testDarkTheme(t)
	const height = 15
	p := newThemePanelFixture(themePanelFixtureOpts{
		th:    th,
		width: themePanelPreferredWidth,
		rows:  themePanelTestRows(12),
	})
	wantW, wantH := themePanelInnerWidth(themePanelPreferredWidth), height-themePanelHeaderRows()-themePanelFooterHeight(themePanelKeymap())
	gotW, gotH := themePanelListSize(p, height)
	if gotW != wantW || gotH != wantH {
		t.Fatalf("themePanelListSize = (%d, %d), want (%d, %d)", gotW, gotH, wantW, wantH)
	}

	// The renderer applies that size itself, so the block is exactly height rows
	// whatever the list was last sized to.
	p.list.SetSize(3, 3)
	if got := lipgloss.Height(renderThemePanel(p, height, th, false)); got != height {
		t.Errorf("block height after a stale list size = %d, want %d", got, height)
	}
}

// TestThemePanel_EveryChromeSurfaceIsATokenLookup is the re-theme-everything rule made
// executable: the slide-over's OWN chrome re-themes with the previewed theme, no exceptions.
//
// Rendering the same state under the two shipped built-ins must produce identical
// text painted in wholly disjoint colours — a surface holding a cached style, or a
// literal the reference frames tempted in (the panel layout refuses `#0C0C16` / `#2B3050`
// precisely here), would survive the swap and show up as a shared parameter list.
func TestThemePanel_EveryChromeSurfaceIsATokenLookup(t *testing.T) {
	dark, light := testDarkTheme(t), testLightTheme(t)
	rows := themePanelTestRows(6)

	// The height is DERIVED so the list holds every row on ONE page and draws no
	// paginator: this fixture builds its list through newThemePanelList without the
	// production restyle (applyThemePanelCanvasMode), so a rendered paginator would
	// show `bubbles/list`'s own hardcoded dot greys on BOTH renders and read as a
	// chrome surface surviving the swap — a fixture artefact rather than the defect
	// this test hunts. The dot re-point has its own coverage, on a panel opened
	// the production way.
	//
	// It is the floor with its single body row swapped for one row per theme, plus the
	// two rows `bubbles/list` charges for its pagination block — which it stops drawing
	// once the body can absorb them. The dot-glyph guard below keeps that honest.
	height := themePanelMinHeight(themePanelKeymap(), true) - themePanelMinBodyRows + len(rows) + 2

	render := func(th theme.Theme) string {
		p := newThemePanelFixture(themePanelFixtureOpts{
			th:          th,
			width:       themePanelPreferredWidth,
			rows:        rows,
			dirUnusable: true,
			message:     messageTestConfirm(),
		})
		return renderThemePanel(p, height, th, false)
	}

	darkBlock, lightBlock := render(dark), render(light)
	if strings.Contains(ansi.Strip(darkBlock), paginationDotGlyph) {
		t.Fatalf("fixture: the list paginated at height %d, so its un-restyled dot greys would read as a surviving surface:\n%s", height, ansi.Strip(darkBlock))
	}
	if ansi.Strip(darkBlock) != ansi.Strip(lightBlock) {
		t.Fatalf("the two renders differ in TEXT, so a colour diff proves nothing:\n%s\n---\n%s",
			ansi.Strip(darkBlock), ansi.Strip(lightBlock))
	}

	darkParams, lightParams := themePanelColourParams(darkBlock), themePanelColourParams(lightBlock)
	if len(darkParams) == 0 || len(lightParams) == 0 {
		t.Fatalf("a render carried no colour at all: dark=%d light=%d", len(darkParams), len(lightParams))
	}
	for p := range darkParams {
		if lightParams[p] {
			t.Errorf("SGR params %q survive the theme swap — a chrome surface is not a token lookup", p)
		}
	}
}

// themePanelOverlayBase builds a base "page view" of contentH rows × contentW
// cells whose every cell is identifiable by row AND column, so a reflow, a shift or
// a re-wrap shows up as a mismatch rather than passing on a coincidence.
func themePanelOverlayBase(contentW, contentH int) []string {
	base := make([]string, contentH)
	for i := range base {
		row := fmt.Sprintf("row-%02d:", i) + strings.Repeat("abcdefghij", 1+contentW/10)
		base[i] = row[:contentW]
	}
	return base
}

// themePanelPaintedOverlayBase is themePanelOverlayBase with every row painted in
// th's canvas, so the composite can be asserted at CELL level and not merely at
// glyph level.
//
// The paint is what makes the overlay's two failure modes visible: a composite that
// flattened the base's own canvas behind the panel — the live-preview premise, since
// the page beneath goes on carrying the previewed paint — and one that recoloured
// the panel's cells both survive a text-only comparison untouched.
func themePanelPaintedOverlayBase(contentW, contentH int, th theme.Theme) []string {
	rows := themePanelOverlayBase(contentW, contentH)
	for i, row := range rows {
		rows[i] = headerCanvasBg(th, false).Render(row)
	}
	return rows
}

// TestThemePanel_OverlayDoesNotRelayoutTheBase is the panel layout's compositor contract: the
// panel is an OPAQUE LAYER over a page laid out at the UNREDUCED content width.
//
// Every base cell to the left of the panel survives untouched and every cell under
// it is replaced. That is what keeps the swap the O(1) restyle of the swap-speed finding and
// keeps the surface being previewed from reflowing under the user — reflowing to the
// reduced width would produce a cleaner edge and was rejected for exactly that cost.
//
// "Untouched" and "replaced" are asserted at CELL level — glyphs AND background
// params, through the same scanCellBackgrounds the canvas gates use — because the
// base is a painted surface, not text: the live-preview premise is that the page
// beneath keeps its own paint while the panel keeps its own, and a text-only
// comparison cannot tell either from a flatten. The base is deliberately painted in
// the OTHER built-in's canvas so "the base survived" and "the base was flattened to
// the panel's paint" are distinguishable answers rather than the same bytes.
func TestThemePanel_OverlayDoesNotRelayoutTheBase(t *testing.T) {
	th, baseTh := testDarkTheme(t), testLightTheme(t)
	const contentW, contentH = 100, 20

	basePaint, panelPaint := canvasBgParams(baseTh.Canvas.Color()), canvasBgParams(th.Canvas.Color())
	if basePaint == panelPaint {
		t.Fatalf("base and panel paint identically (%s) — the fixture cannot tell a flatten from a survival", basePaint)
	}

	base := themePanelPaintedOverlayBase(contentW, contentH, baseTh)
	p := newThemePanelFixture(themePanelFixtureOpts{
		th:    th,
		width: themePanelPreferredWidth,
		rows:  themePanelTestRows(8),
	})
	panel := renderThemePanel(p, contentH, th, false)

	composed := overlayThemePanel(strings.Join(base, "\n"), panel, contentW)

	composedRows := strings.Split(composed, "\n")
	panelRows := strings.Split(panel, "\n")
	composedLines := strings.Split(ansi.Strip(composed), "\n")
	panelLines := strings.Split(ansi.Strip(panel), "\n")
	if len(composedLines) != contentH {
		t.Fatalf("composed height = %d, want %d", len(composedLines), contentH)
	}

	cut := contentW - themePanelPreferredWidth
	for i, line := range composedLines {
		cells := []rune(line)
		if len(cells) != contentW {
			t.Fatalf("composed line %d is %d cells, want %d: %q", i, len(cells), contentW, line)
		}
		if got, want := string(cells[:cut]), string([]rune(ansi.Strip(base[i]))[:cut]); got != want {
			t.Errorf("line %d left of the panel = %q, want %q", i, got, want)
		}
		if got, want := string(cells[cut:]), panelLines[i]; got != want {
			t.Errorf("line %d under the panel = %q, want the panel row %q", i, got, want)
		}

		composedCells := scanCellBackgrounds(composedRows[i])
		baseCells := scanCellBackgrounds(base[i])
		panelCells := scanCellBackgrounds(panelRows[i])
		if len(composedCells) != contentW {
			t.Fatalf("composed line %d has %d painted cells, want %d: %q", i, len(composedCells), contentW, escSeq(composedRows[i]))
		}
		for col := range cut {
			if baseCells[col].params != basePaint {
				t.Fatalf("base line %d col %d is not canvas-painted (%+v) — the fixture proves nothing", i, col, baseCells[col])
			}
			if got := composedCells[col]; got != baseCells[col] {
				t.Errorf("line %d col %d left of the panel is painted %+v, want the base's own %+v", i, col, got, baseCells[col])
			}
		}
		if len(panelCells) != themePanelPreferredWidth {
			t.Fatalf("panel line %d has %d painted cells, want %d: %q", i, len(panelCells), themePanelPreferredWidth, escSeq(panelRows[i]))
		}
		for col, want := range panelCells {
			if got := composedCells[cut+col]; got != want {
				t.Errorf("line %d col %d under the panel is painted %+v, want the panel's own %+v", i, cut+col, got, want)
			}
		}
	}
}

// TestThemePanel_OverlayCutsMidLabel pins the accepted cost the panel layout states outright:
// "the overlay cuts wherever its left border falls, mid-label included — `x proje▏`".
//
// That is NOT a violation of the footer's "never truncate a label" rule: that rule governs how
// the footer lays ITSELF out as the terminal narrows, and the panel is an opaque layer
// composited over a footer that laid out at full width. The assertion is therefore
// the opposite of the usual one — the covered footer must be CUT rather than
// re-laid-out.
func TestThemePanel_OverlayCutsMidLabel(t *testing.T) {
	th := testDarkTheme(t)
	// At this content width the panel's left border lands inside `x projects`,
	// reproducing the panel layout's own worked example (`x proje▏`) a character earlier. The
	// width has moved twice: with the re-authored footer row (the amendment this feature
	// carries to the keymap), and again with the panel's own widening — a wider panel cuts
	// further left, so the content width had to follow the cut column back into a label.
	const contentW, contentH = 90, 20

	footer := renderSessionsFooter(sessionsKeymap(), contentW, th, false)
	footerLines := strings.Split(ansi.Strip(footer), "\n")
	keyRow := footerLines[len(footerLines)-1]
	if !strings.Contains(keyRow, "? help") || !strings.Contains(keyRow, "x projects") {
		t.Fatalf("the un-overlaid footer is not the condensed Sessions row: %q", keyRow)
	}

	base := themePanelOverlayBase(contentW, contentH)
	base[contentH-1] = keyRow

	p := newThemePanelFixture(themePanelFixtureOpts{
		th:    th,
		width: themePanelPreferredWidth,
		rows:  themePanelTestRows(8),
	})
	panel := renderThemePanel(p, contentH, th, false)
	composed := overlayThemePanel(strings.Join(base, "\n"), panel, contentW)

	cut := contentW - themePanelPreferredWidth
	covered := []rune(strings.Split(ansi.Strip(composed), "\n")[contentH-1])

	// The cut lands INSIDE a word: the footer was not re-laid-out to fit, and no
	// entry was dropped from the right as the footer's right-to-left degrade would do for a
	// narrowing terminal.
	full := []rune(keyRow)
	if full[cut-1] == ' ' || full[cut] == ' ' {
		t.Fatalf("the panel's left border falls on a word boundary at col %d (%q) — the case is not exercised",
			cut, string(full[max(cut-8, 0):min(cut+8, len(full))]))
	}
	if got, want := string(covered[:cut]), string(full[:cut]); got != want {
		t.Errorf("the covered footer reflowed: got %q, want the unreduced footer's first %d cells %q", got, cut, want)
	}
	if want := "x proj"; !strings.HasSuffix(string(covered[:cut]), want) {
		t.Errorf("the cut does not land mid-label: %q does not end in %q", string(covered[:cut]), want)
	}
	if strings.Contains(string(covered), "? help") {
		t.Error("the right-aligned `? help` survived the overlay — the panel is not covering the right column")
	}
}

// TestThemePanel_ViewCompositesWhenOpen wires the surface to the screen: with the
// panel open the composed frame carries it, and with it closed the frame is
// BYTE-IDENTICAL to a Portal that has no panel at all.
//
// Of the open frame it asserts the criterion's OBSERVABLE — "the fill never paints
// over any panel cell": every panel cell survives verbatim at the content region's
// right edge, gutter offset included, in GLYPHS AND IN PAINT. The paint half matters
// because the panel is a painted surface, not text — its own backgrounds are what
// a cell of it IS, and a glyph-only comparison would accept the panel's text over
// anybody else's colour.
//
// It does NOT pin the layer ORDER, and no assertion about the panel's own cells
// could: themePanelPainter has already given every panel cell an explicit
// background, so fillCanvas's backfill (which touches only cells left on the
// terminal default) and its trailing pad (which strips only background-less spaces)
// have nothing to do to a panel cell whichever side of the composite they run.
// Compositing before the fill leaves this whole suite green — the frame's bytes
// shift elsewhere, but not one panel cell moves. The shipped order is therefore
// defence-in-depth on the painted path. It is load-bearing only under colourless,
// where the panel's pad IS background-less, and the NO_COLOR panel block blocks the panel
// there — on a path whose current output is ragged-width anyway, so asserting it would encode
// a shape nobody ships.
//
// `open` is set directly because nothing here opens the panel (the
// `t` keypress does).
func TestThemePanel_ViewCompositesWhenOpen(t *testing.T) {
	const termW, termH = 90, 24

	m := newCanvasTestModel(t, termW, termH, appearanceDarkCanvas)
	closed := m.View().Content

	m.themePanel = newThemePanelFixture(themePanelFixtureOpts{
		th:    m.themeState.active,
		width: themePanelPreferredWidth,
		rows:  themePanelTestRows(9),
	})
	opened := m.View().Content

	if opened == closed {
		t.Fatal("the composed frame is unchanged with the panel open")
	}
	if !strings.Contains(ansi.Strip(opened), themePanelHeaderLabel) {
		t.Errorf("the composed frame carries no %q header", themePanelHeaderLabel)
	}

	contentW, contentH := m.contentWidth(), m.contentHeight()
	panel := renderThemePanel(m.themePanel, contentH, m.themeState.active, m.colourless)
	panelLines := strings.Split(ansi.Strip(panel), "\n")
	_, _, topPad, _ := gutterPadding(termW, termH, contentW, contentH)
	left := (termW-contentW)/2 + contentW - themePanelPreferredWidth

	frame := strings.Split(ansi.Strip(opened), "\n")
	rawFrame, panelRows := strings.Split(opened, "\n"), strings.Split(panel, "\n")
	if len(frame) != termH {
		t.Fatalf("composed frame is %d rows, want %d", len(frame), termH)
	}
	for j, want := range panelLines {
		row := []rune(frame[topPad+j])
		if len(row) != termW {
			t.Fatalf("frame row %d is %d cells, want %d", topPad+j, len(row), termW)
		}
		if got := string(row[left : left+themePanelPreferredWidth]); got != want {
			t.Errorf("frame row %d under the panel = %q, want the panel row %q", topPad+j, got, want)
		}

		frameCells, panelCells := scanCellBackgrounds(rawFrame[topPad+j]), scanCellBackgrounds(panelRows[j])
		if len(frameCells) != termW {
			t.Fatalf("frame row %d has %d painted cells, want %d: %q", topPad+j, len(frameCells), termW, escSeq(rawFrame[topPad+j]))
		}
		if len(panelCells) != themePanelPreferredWidth {
			t.Fatalf("panel row %d has %d painted cells, want %d: %q", j, len(panelCells), themePanelPreferredWidth, escSeq(panelRows[j]))
		}
		for col, wantCell := range panelCells {
			if got := frameCells[left+col]; got != wantCell {
				t.Errorf("frame row %d col %d is painted %+v, want the panel's own %+v — the fill painted over a panel cell",
					topPad+j, left+col, got, wantCell)
			}
		}
	}

	m.themePanel.open = false
	if reclosed := m.View().Content; reclosed != closed {
		t.Error("the closed frame is not byte-identical to the pre-panel view")
	}
}

// TestThemePanel_DelegateHasASingleConstructionPoint is the completeness risk guard: the
// panel's row delegate has EXACTLY ONE construction site, and it takes all three of its
// inputs from the model.
//
// Two construction sites can disagree about width or colourlessness, and the
// disagreement is invisible until a resize during a live preview — on the surface
// the completeness risk calls the worst case of the cached-style class. The source half of the
// assertion is what stops a second site appearing later (the restyle path is
// specified to re-invoke this method, not to build its own).
func TestThemePanel_DelegateHasASingleConstructionPoint(t *testing.T) {
	th := testLightTheme(t)
	m := Model{themeState: themeState{active: th}, colourless: true}
	m.themePanel.width = themePanelMinWidth

	// Compared field by field: a whole Theme through %+v is 19 {name value} pairs of
	// noise on a line whose only job is to say WHICH input was wired wrong.
	got := m.themeRowDelegate()
	if got.Theme.Canvas != th.Canvas {
		t.Errorf("delegate theme = %s, want %s", themeLabel(got.Theme), themeLabel(th))
	}
	if !got.Colourless {
		t.Error("delegate did not take the model's colourless flag")
	}
	if want := themePanelInnerWidth(themePanelMinWidth); got.Width != want {
		t.Errorf("delegate width = %d, want the panel's inner width %d", got.Width, want)
	}

	sites := themeRowDelegateLiteralSites(t)
	if len(sites) != 1 {
		t.Fatalf("themeRowDelegate is constructed at %d production sites (%v), want exactly 1", len(sites), sites)
	}
	if sites[0] != "theme_panel.go" {
		t.Errorf("the single construction site is %s, want theme_panel.go", sites[0])
	}
}

// themeRowDelegateLiteralSites returns the production files in internal/tui that
// build a themeRowDelegate composite literal. Test files are excluded — a test-side
// delegate cannot reach the render path, exactly as the colour-literal guard scopes
// itself.
func themeRowDelegateLiteralSites(t *testing.T) []string {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(".", "*.go"))
	if err != nil {
		t.Fatalf("glob internal/tui package files: %v", err)
	}

	var sites []string
	for _, path := range matches {
		name := filepath.Base(path)
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		fset := token.NewFileSet()
		file, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			t.Fatalf("parse %s: %v", name, perr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if ident, isIdent := lit.Type.(*ast.Ident); isIdent && ident.Name == "themeRowDelegate" {
				sites = append(sites, name)
			}
			return true
		})
	}
	return sites
}

// TestThemePanel_BodyIsCanvas is the panel layout's body assignment: the panel is painted
// `canvas`, on EVERY cell.
//
// It is asserted cell-by-cell rather than by the presence of the colour, because the
// bare cells are real: `bubbles/list` pads its short lines (the filler rows on a
// part-full page, the pagination row's own padding) with unstyled spaces. Composited
// after the outer fill, those cells are past the reach of fillCanvas's backfill — so
// left bare they would be terminal-bg islands INSIDE the panel, on the one surface
// whose whole job is to show the user what a theme looks like.
func TestThemePanel_BodyIsCanvas(t *testing.T) {
	th := testDarkTheme(t)
	p := newThemePanelFixture(themePanelFixtureOpts{
		th:          th,
		width:       themePanelPreferredWidth,
		rows:        themePanelTestRows(9),
		dirUnusable: true,
	})
	block := renderThemePanel(p, 20, th, false)
	canvasParams := canvasBgParams(th.Canvas.Color())

	sawCanvas := false
	for i, line := range strings.Split(block, "\n") {
		cells := scanCellBackgrounds(line)
		if len(cells) != themePanelPreferredWidth {
			t.Fatalf("line %d has %d painted cells, want %d: %q", i, len(cells), themePanelPreferredWidth, ansi.Strip(line))
		}
		for col, cell := range cells {
			if !cell.set {
				t.Fatalf("line %d col %d falls back to the terminal default — a bg island inside the panel: %q",
					i, col, escSeq(line))
			}
			if cell.params == canvasParams {
				sawCanvas = true
			}
		}
	}
	if !sawCanvas {
		t.Errorf("no cell carried the canvas background %q", canvasParams)
	}
}

// TestThemePanel_Colourless is the NO_COLOR carve-out: Portal paints no
// canvas, so NO cell in the block may carry a background SGR. The NO_COLOR panel block blocks
// the panel under NO_COLOR outright, so this is the defence rather than the daily path.
func TestThemePanel_Colourless(t *testing.T) {
	th := testDarkTheme(t)
	p := newThemePanelFixture(themePanelFixtureOpts{
		th:          th,
		colourless:  true,
		width:       themePanelPreferredWidth,
		rows:        themePanelTestRows(5),
		dirUnusable: true,
		message:     messageTestConfirm(),
	})
	block := renderThemePanel(p, 14, th, true)

	for i, line := range strings.Split(block, "\n") {
		for col, cell := range scanCellBackgrounds(line) {
			if cell.set {
				t.Fatalf("colourless line %d col %d carries a background SGR (%s): %q",
					i, col, cell.params, escSeq(line))
			}
		}
	}

	// The structure survives: the border, the header, the pinned warning and the
	// footer are all still there, glyph-backed.
	lines := themePanelLines(block)
	label := lines[themePanelHeaderLabelRow()]
	if !strings.HasPrefix(label, themePanelContentPrefix()+themePanelHeaderLabel) {
		t.Errorf("colourless header row = %q", label)
	}
	if !strings.Contains(lines[themePanelHeaderRows()], themePanelDirUnreadable) {
		t.Errorf("colourless directory row = %q", lines[themePanelHeaderRows()])
	}
}
