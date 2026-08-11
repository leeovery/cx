package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/tmux"
)

// The panel layout's panel chrome, re-cut to the PAGE's vertical rhythm.
//
// The panel's header was once pinned at a restated two rows — a label above a rule
// with the list starting immediately beneath — so `Themes` landed on the page's
// WORDMARK row and the panel's first row sat three rows above the first session
// row. Two columns under one rule, running two different rhythms.
//
// EVERY ASSERTION HERE IS MADE ON THE COMPOSED FRAME, and that is the whole point:
// the panel block in isolation cannot show a row relationship with the page, which
// is exactly why that header passed every test it had. The page's row counts
// are READ OFF THE FRAME rather than restated, so a change to the page's own header
// or section header moves this suite's expectations with it — the same discipline
// the production measurement follows.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// The content region every chrome fixture composes into: wide enough for the panel
// to take its preferred width with the page's own rows still visible beside it, and
// tall enough that both lists run several rows past their headers.
const (
	chromeContentW = 96
	chromeContentH = 26
)

// chromeSessionNames are the page's session rows, in list order. They are distinct
// non-substrings of one another so a row lookup cannot match the wrong one.
func chromeSessionNames() []string {
	return []string{"alpha-one", "bravo-two", "charlie-three", "delta-four", "echo-five"}
}

// newChromePanelModel builds a renderable Sessions page at the chrome content
// region and opens the panel through the production `t` keypress.
func newChromePanelModel(t *testing.T) Model {
	t.Helper()
	rows := arrowValidRows(t, 6)
	m := Build(newArrowPanelDeps(t, rows, rows[0].Slug))

	sessions := make([]tmux.Session, 0, len(chromeSessionNames()))
	for _, name := range chromeSessionNames() {
		sessions = append(sessions, tmux.Session{Name: name, Windows: 1})
	}
	m = openPanelForTestWithSessions(t, m, chromeContentW, chromeContentH, sessions)

	if got := m.themePanel.width; got != themePanelPreferredWidth {
		t.Fatalf("fixture: the panel opened at %d cells, want the preferred %d", got, themePanelPreferredWidth)
	}
	return m
}

// chromeFrame is the composed frame's CONTENT REGION, row by row, with every SGR
// stripped — the page gutter cut away so column 0 is the page's own left edge.
func chromeFrame(t *testing.T, m Model) []string {
	t.Helper()
	lines := strings.Split(ansi.Strip(m.View().Content), "\n")
	contentW, contentH := m.contentWidth(), m.contentHeight()
	leftPad, _, topPad, _ := gutterPadding(m.termWidth, m.termHeight, contentW, contentH)

	rows := make([]string, 0, contentH)
	for i := range contentH {
		cells := []rune(lines[topPad+i])
		if len(cells) != m.termWidth {
			t.Fatalf("frame row %d is %d cells, want the terminal's %d", topPad+i, len(cells), m.termWidth)
		}
		rows = append(rows, string(cells[leftPad:leftPad+contentW]))
	}
	return rows
}

// chromeSplit cuts one content row into the PAGE's half and the PANEL's, at the
// column the slide-over's left edge falls on. Two halves rather than one string, so
// a lookup can never match the other column's text.
func chromeSplit(m Model, row string) (page, panel string) {
	at := m.contentWidth() - m.themePanel.width
	cells := []rune(row)
	return string(cells[:at]), string(cells[at:])
}

// chromePageRow / chromePanelRow are the index of the first content row whose page
// half / panel half carries want, or -1.
func chromePageRow(m Model, rows []string, want string) int {
	for i, row := range rows {
		if page, _ := chromeSplit(m, row); strings.Contains(page, want) {
			return i
		}
	}
	return -1
}

func chromePanelRow(m Model, rows []string, want string) int {
	for i, row := range rows {
		if _, panel := chromeSplit(m, row); strings.Contains(panel, want) {
			return i
		}
	}
	return -1
}

// chromeRuleRow is the index of the single content row carrying the header separator
// rule, failing when the frame carries none or more than one.
//
// ONE LANE IS THE ASSERTION, not merely "a rule exists": the page and the panel each
// draw their own, and two rules on two rows is precisely the split this task closes.
func chromeRuleRow(t *testing.T, rows []string) int {
	t.Helper()
	at := -1
	for i, row := range rows {
		if !strings.Contains(row, headerRuleGlyph) {
			continue
		}
		if at >= 0 {
			t.Fatalf("content rows %d and %d both carry the rule glyph; the page's rule and the panel's are in two lanes", at, i)
		}
		at = i
	}
	if at < 0 {
		t.Fatal("no content row carries the rule glyph")
	}
	return at
}

// chromePanelFooterTop is the index of the first row of the panel's vertical key
// list — the footer is the last themePanelFooterHeight rows of the content region.
func chromePanelFooterTop(m Model) int {
	return m.contentHeight() - themePanelFooterHeight(themePanelKeymap())
}

// chromeInkColumn is the panel column of the first non-space cell PAST the left
// border — so the border itself (column 0) never answers, and the result reads as
// "this many cells in from the border". It is -1 for a row with nothing on it.
func chromeInkColumn(panel string) int {
	for i, r := range []rune(panel) {
		if i == 0 {
			continue // the border column
		}
		if r != ' ' {
			return i
		}
	}
	return -1
}

// TestPanelChrome_LabelSharesTheSectionHeaderRow: it lands the panel label on the
// page's section-header row.
//
// The panel layout's header is measured against the PAGE's, not restated: the panel's header
// region costs the page's header block plus its section header, so `Themes` lands
// on the very row `Sessions N` occupies. Asserted on the composed frame, because the
// panel block in isolation cannot express a relationship with the page.
func TestPanelChrome_LabelSharesTheSectionHeaderRow(t *testing.T) {
	m := newChromePanelModel(t)
	rows := chromeFrame(t, m)

	section := chromePageRow(m, rows, sectionLabel)
	label := chromePanelRow(m, rows, themePanelHeaderLabel)
	if section < 0 {
		t.Fatalf("no content row carries the page's %q section header", sectionLabel)
	}
	if label < 0 {
		t.Fatalf("no content row carries the panel's %q label", themePanelHeaderLabel)
	}
	if label != section {
		t.Errorf("the panel's %q label renders on content row %d and the page's %q header on row %d — the two columns run different rhythms",
			themePanelHeaderLabel, label, sectionLabel, section)
	}
}

// TestPanelChrome_ListsStartOnTheSameRow: it lands the first theme row on the first
// session row.
func TestPanelChrome_ListsStartOnTheSameRow(t *testing.T) {
	m := newChromePanelModel(t)
	rows := chromeFrame(t, m)

	session := chromePageRow(m, rows, chromeSessionNames()[0])
	themeRow := chromePanelRow(m, rows, arrowSlug(0))
	if session < 0 {
		t.Fatalf("no content row carries the first session %q", chromeSessionNames()[0])
	}
	if themeRow < 0 {
		t.Fatalf("no content row carries the first theme %q", arrowSlug(0))
	}
	if themeRow != session {
		t.Errorf("the panel's first row renders on content row %d and the page's first session row on %d", themeRow, session)
	}
}

// TestPanelChrome_ListsStayInStep: it keeps the two lists in step.
//
// One row apiece is a coincidence; every row apiece is the rhythm. Both delegates
// are one line with no spacing, so once the two bodies START together they can only
// diverge if one of them is not the list it appears to be.
func TestPanelChrome_ListsStayInStep(t *testing.T) {
	m := newChromePanelModel(t)
	rows := chromeFrame(t, m)

	names := chromeSessionNames()
	for i, name := range names {
		session := chromePageRow(m, rows, name)
		themeRow := chromePanelRow(m, rows, arrowSlug(i))
		if session < 0 {
			t.Fatalf("no content row carries session %q", name)
		}
		if themeRow < 0 {
			t.Fatalf("no content row carries theme row %q", arrowSlug(i))
		}
		if themeRow != session {
			t.Errorf("list row %d: the theme %q is on content row %d and the session %q on row %d", i, arrowSlug(i), themeRow, name, session)
		}
	}
}

// TestPanelChrome_RulesShareOneLane: it shares the header rule's lane.
//
// The panel covers the right end of the page's rule, so it draws its own across its
// whole width — that is what carries the page's rule unbroken to the frame edge. One
// row of rule, spanning every column of the content region.
func TestPanelChrome_RulesShareOneLane(t *testing.T) {
	m := newChromePanelModel(t)
	rows := chromeFrame(t, m)

	at := chromeRuleRow(t, rows)
	if want := strings.Repeat(headerRuleGlyph, m.contentWidth()); rows[at] != want {
		t.Errorf("the rule row = %q, want the glyph across all %d content columns — the panel's rule does not continue the page's to the frame edge",
			rows[at], m.contentWidth())
	}
}

// TestPanelChrome_HeaderRegionIsEmpty: it renders nothing above the rule.
//
// The region above the rule carries nothing BY DECISION, not by omission: `esc
// close` was considered for it and rejected (Portal's modals carry no such header
// affordance, and it stays in the panel's own vertical key list). The panel's body
// and the page's are the same `canvas` token, so a blank panel region there is
// indistinguishable from the page's own canvas — which is what lets the header band
// read as uninterrupted.
func TestPanelChrome_HeaderRegionIsEmpty(t *testing.T) {
	m := newChromePanelModel(t)
	rows := chromeFrame(t, m)
	at := chromeRuleRow(t, rows)

	if at == 0 {
		t.Fatal("fixture: the rule is on content row 0, so there is no region above it to assert about")
	}
	for i := range at {
		_, panel := chromeSplit(m, rows[i])
		if strings.TrimSpace(panel) != "" {
			t.Errorf("content row %d above the rule carries %q in the panel's columns, want nothing", i, panel)
		}
	}
}

// TestPanelChrome_BorderStartsBelowTheRule: it starts the left border below the
// header rule.
//
// The panel's `│` running the FULL height cuts the page's header band in two, so the
// slide-over reads as a second column rather than as a surface inside the content
// region. Starting it below the rule is what makes the band and its rule run unbroken
// to the frame edge.
//
// ASSERTED PER ROW, so a partial application — a border that starts one row early, or
// one that stops short — fails on the row it is wrong about rather than passing on an
// aggregate.
func TestPanelChrome_BorderStartsBelowTheRule(t *testing.T) {
	m := newChromePanelModel(t)
	rows := chromeFrame(t, m)
	at := chromeRuleRow(t, rows)

	for i, row := range rows {
		_, panel := chromeSplit(m, row)
		got := string([]rune(panel)[0])
		want := panelFrameSide
		switch {
		case i < at:
			want = " "
		case i == at:
			// The rule runs THROUGH the border's column — that is what carries the
			// page's rule to the frame edge instead of notching it with a `│`.
			want = headerRuleGlyph
		}
		if got != want {
			t.Errorf("content row %d opens the panel with %q, want %q (the rule is on row %d)", i, got, want, at)
		}
	}
}

// TestPanelChrome_InnerGutter: it sits two cells in from the border.
//
// The page's content sits a clear gutter in from the frame edge (Hinset); the panel
// sat at half of it, hard against its own border. Two cells in from the border —
// counting the border's own column — puts the panel's content on the same inset the
// page uses, on all three of its surfaces.
func TestPanelChrome_InnerGutter(t *testing.T) {
	m := newChromePanelModel(t)
	rows := chromeFrame(t, m)
	at := chromeRuleRow(t, rows)

	const contentColumn = 2

	// Nothing anywhere in the panel sits one cell in from the border.
	for i := at + 1; i < len(rows); i++ {
		_, panel := chromeSplit(m, rows[i])
		if got := []rune(panel)[1]; got != ' ' {
			t.Errorf("content row %d has %q one cell in from the border, want the gutter blank", i, string(got))
		}
	}

	t.Run("the label", func(t *testing.T) {
		_, panel := chromeSplit(m, rows[chromePanelRow(m, rows, themePanelHeaderLabel)])
		if got := chromeInkColumn(panel); got != contentColumn {
			t.Errorf("the %q label starts at panel column %d, want %d", themePanelHeaderLabel, got, contentColumn)
		}
	})

	t.Run("the cursor row", func(t *testing.T) {
		row := chromePanelRow(m, rows, arrowSlug(0))
		_, panel := chromeSplit(m, rows[row])
		cells := []rune(panel)
		if got := string(cells[contentColumn]); got != selectorBar {
			t.Errorf("the cursor row carries %q at the content column, want the %q left bar — the cursor column did not move with the gutter", got, selectorBar)
		}
	})

	t.Run("an unselected row", func(t *testing.T) {
		row := chromePanelRow(m, rows, arrowSlug(1))
		_, panel := chromeSplit(m, rows[row])
		if got, want := chromeInkColumn(panel), contentColumn+leftBarColumnWidth; got != want {
			t.Errorf("an unselected row's label starts at panel column %d, want %d (the gutter plus the %d-cell cursor column)", got, want, leftBarColumnWidth)
		}
	})

	t.Run("the key list", func(t *testing.T) {
		top := chromePanelFooterTop(m)
		for i := top; i < m.contentHeight(); i++ {
			_, panel := chromeSplit(m, rows[i])
			if got := chromeInkColumn(panel); got != contentColumn {
				t.Errorf("key list row %d starts at panel column %d, want %d: %q", i-top, got, contentColumn, panel)
			}
		}
	})
}

// TestPanelChrome_LadderEnds: it pins the two ends of the ladder.
//
// The band is a decided one rather than a derived one — every pinned panel string
// is written to fit it — so the two constants are asserted against literals here,
// and the ladder's SHAPE is pinned separately by the geometry suite.
func TestPanelChrome_LadderEnds(t *testing.T) {
	const wantPreferred, wantMinimum = 30, 24

	if themePanelPreferredWidth != wantPreferred {
		t.Errorf("themePanelPreferredWidth = %d, want %d", themePanelPreferredWidth, wantPreferred)
	}
	if themePanelMinWidth != wantMinimum {
		t.Errorf("themePanelMinWidth = %d, want %d", themePanelMinWidth, wantMinimum)
	}
}

// chromeMeasuredFloor is the geometry rule's height floor derived INDEPENDENTLY of
// the production arithmetic: the two rows the panel's header DRAWS — a rule and the
// `Themes` label — plus the measured footer, one list row and one message row.
//
// The page-aligning blank rows are deliberately absent from it. They carry nothing,
// so a floor that charged for them would refuse a panel with every row it needs.
func chromeMeasuredFloor(t *testing.T, m Model) int {
	t.Helper()
	if got := chromeMeasuredAffordance(t, m); got <= specPanelHeaderRows+themePanelFooterHeight(themePanelKeymap())+2 {
		t.Fatalf("fixture: the page-aligned header costs no more than the %d rows the panel draws, so the floor and the affordance are the same number", specPanelHeaderRows)
	}
	const listRow, messageRow = 1, 1
	return specPanelHeaderRows + themePanelFooterHeight(themePanelKeymap()) + listRow + messageRow
}

// chromeMeasuredAffordance is the height from which the panel can keep the PAGE's
// header rhythm, measured off the page's own composed frame: the rows the page
// spends before its first session row, plus the footer, one list row and one
// message row.
func chromeMeasuredAffordance(t *testing.T, m Model) int {
	t.Helper()
	rows := chromeFrame(t, m)
	first := chromePageRow(m, rows, chromeSessionNames()[0])
	if first < 0 {
		t.Fatalf("no content row carries the first session %q", chromeSessionNames()[0])
	}
	const listRow, messageRow = 1, 1
	return first + themePanelFooterHeight(themePanelKeymap()) + listRow + messageRow
}

// TestPanelChrome_FloorFollowsTheHeader: it charges the floor for the header the
// panel DRAWS and the page's rhythm for the rows it pads with.
//
// The panel's header is measured off the page's so the two surfaces run one band,
// but only two of those rows carry anything. The floor is therefore the two, and
// the page's own measurement is what decides whether the panel can still afford the
// blanks — asserted against the page's composed frame rather than against a
// literal.
func TestPanelChrome_FloorFollowsTheHeader(t *testing.T) {
	m := newChromePanelModel(t)
	entries := themePanelKeymap()

	want := chromeMeasuredFloor(t, m)
	if got := themePanelMinHeight(entries, false); got != want {
		t.Errorf("themePanelMinHeight = %d, want %d — the header draws a rule and a label, and the floor charges for those", got, want)
	}
	if got, wantDir := themePanelMinHeight(entries, true), want+1; got != wantDir {
		t.Errorf("the directory-inclusive floor = %d, want %d", got, wantDir)
	}

	// The page-aligned shape is spent from the height the page's own frame measures,
	// not from the floor.
	affordance := chromeMeasuredAffordance(t, m)
	if got := themePanelHeaderRows(affordance, false); got != affordance-themePanelFooterHeight(entries)-2 {
		t.Errorf("at %d rows the header costs %d, want the rows the page spends before its first session row", affordance, got)
	}
	if got := themePanelHeaderRows(affordance-1, false); got != specPanelHeaderRows {
		t.Errorf("one row below the page's rhythm the header costs %d, want the %d rows it draws", got, specPanelHeaderRows)
	}

	// The panel genuinely renders at its floor: header, one list row, the message
	// row's budget and the WHOLE footer, with nothing pushed off the bottom.
	th := testDarkTheme(t)
	p := newThemePanelFixture(themePanelFixtureOpts{
		th:    th,
		width: themePanelMinWidth,
		rows:  themePanelTestRows(8),
	})
	lines := themePanelLines(renderThemePanel(p, want, th, false))
	if len(lines) != want {
		t.Fatalf("the panel rendered %d rows at its floor of %d", len(lines), want)
	}
	wantFooter := themePanelLines(renderThemePanelFooter(entries, themePanelInnerWidth(themePanelMinWidth), th, false))
	for i, row := range wantFooter {
		if got := lines[len(lines)-len(wantFooter)+i]; !strings.HasSuffix(strings.TrimRight(got, " "), strings.TrimRight(row, " ")) {
			t.Errorf("footer row %d = %q, want it to carry %q — the floor overflowed and the assembly cut the footer", i, got, row)
		}
	}
}

// TestPanelChrome_EntryGateFollowsTheFloor: it moves the entry gate with the floor.
//
// The entry gate and the geometry rule's resize condition read ONE predicate, so a floor that
// grows moves the gate with it. The panel now refuses on terminals it previously
// admitted — correct, because it genuinely needs the rows.
func TestPanelChrome_EntryGateFollowsTheFloor(t *testing.T) {
	floor := chromeMeasuredFloor(t, newChromePanelModel(t))

	t.Run("it refuses at one row below the floor", func(t *testing.T) {
		m := newChromeGateModel(t, floor-1)
		m = pressThemeKey(t, m)
		if m.themePanel.open {
			t.Errorf("`t` opened the panel at %d content rows, one below the %d-row floor", floor-1, floor)
		}
		if got := m.flashText; got != themePanelShortEntryFlash {
			t.Errorf("the refusal raised %q, want §14A's %q", got, themePanelShortEntryFlash)
		}
	})

	t.Run("it admits at the floor", func(t *testing.T) {
		m := newChromeGateModel(t, floor)
		m = pressThemeKey(t, m)
		if !m.themePanel.open {
			t.Errorf("`t` refused at exactly the %d-row floor, which §9.8 admits", floor)
		}
	})
}

// newChromeGateModel builds a Sessions model at the chrome width and the given
// content HEIGHT, with the panel unopened.
func newChromeGateModel(t *testing.T, contentH int) Model {
	t.Helper()
	rows := arrowValidRows(t, 6)
	m := Build(newArrowPanelDeps(t, rows, rows[0].Slug))
	m.termWidth, m.termHeight = geometryTerm(chromeContentW, contentH)
	m.applySessions([]tmux.Session{{Name: chromeSessionNames()[0], Windows: 1}})
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	if got := m.contentHeight(); got != contentH {
		t.Fatalf("fixture: the content region is %d rows tall, want %d", got, contentH)
	}
	return m
}

// chromeWords normalises a string to its whitespace-separated words, so a
// comparison is about the text rather than about where a wrap or a pad fell.
func chromeWords(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
