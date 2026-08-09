package tui

import (
	"os"
	"strings"
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
)

// §9.2's arrow-preview: `↑`/`↓` move the panel cursor, `Ctrl+↑`/`Ctrl+↓` page it,
// the cursor steps past unselectable rows (§9.5), and every landing re-themes the
// WHOLE frame — main screen and panel chrome alike (§9.11) — through the §11.1
// restyle path.
//
// Most of this file asserts NEGATIVES, because live preview's whole risk profile
// is what a keypress must NOT do: no file read (§5.8), no rebuild and no lazy pane
// read (§11.1), no movement of the startup canvas hex (§11.4), and no write of any
// kind (§9.2 — "nothing is written").
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// The four navigation presses the panel routes, and the banned forms §12.2 drops.
var (
	arrowUp       = tea.KeyPressMsg{Code: tea.KeyUp}
	arrowDown     = tea.KeyPressMsg{Code: tea.KeyDown}
	arrowPageUp   = tea.KeyPressMsg{Code: tea.KeyUp, Mod: tea.ModCtrl}
	arrowPageDown = tea.KeyPressMsg{Code: tea.KeyDown, Mod: tea.ModCtrl}
)

// arrowTermW / arrowTermH size every renderable arrow fixture. They are wide and
// tall enough that the panel composites over a real content region with the main
// screen's own rows still visible to the left of it — which is what makes "the
// main screen AND the panel both re-themed" two separate observations.
const (
	arrowTermW = 100
	arrowTermH = 28
)

// arrowPagingTermH / arrowPagingPerPage are the fixture terminal height the PAGING
// cases run at, and the page the production sizing gives the panel's list there.
//
// A fixture that wants a page boundary sizes the TERMINAL and lets
// applyThemePanelListStyles derive the page, because that is the only shape in
// which the paging assertions are about production: the arithmetic is
// themePanelListSize's, shared with the per-frame copy renderThemePanel sizes, so
// a test that installed a page of its own would be asserting against a page the
// user never gets. At arrowTermH the body is 17 rows and a handful of fixture rows
// fit on one page, so `Ctrl+↓` would have nowhere to go at all.
//
// The arithmetic: contentHeight(15) = 13, less the header's 5 (the page-measured
// region — header block plus section header) and the four-row vertical footer leaves
// a 4-row body, of which `bubbles/list` spends 2 on its own pagination block (the
// dot row plus its leading blank) — a two-row page. It is the SHORTEST terminal that
// paginates, one row above §9.8's floor.
const (
	arrowPagingTermH   = 15
	arrowPagingPerPage = 2
)

// arrowPaletteReds are the leading red bytes of the distinct synthetic palettes
// the arrow fixtures paint their rows from. Every channel of syntheticProbePalette
// is three decimal digits, so one palette's rendered SGR core can never be a
// substring of another's — the property the "the stale colour is absent" half of
// each assertion rests on.
var arrowPaletteReds = []uint8{0xAA, 0xBB, 0xCC, 0xDD, 0xEE, 0xFF}

// arrowPalette returns the i-th distinct synthetic palette.
func arrowPalette(i int) theme.Theme {
	return syntheticProbePalette(arrowPaletteReds[i%len(arrowPaletteReds)])
}

// arrowValidRow is one SELECTABLE union row carrying its own distinct palette, so
// landing on it is observable as a whole-frame colour change.
func arrowValidRow(slug string, palette int) theme.Row {
	return theme.Row{Slug: slug, Source: theme.SourceBuiltin, Theme: arrowPalette(palette)}
}

// arrowInvalidRow is one UNSELECTABLE union row — a drop-in the loader rejected.
// It carries no palette, exactly as the loader's Result does not.
func arrowInvalidRow(slug string) theme.Row {
	return theme.Row{
		Slug:      slug,
		Filename:  slug + ".theme",
		Source:    theme.SourceFile,
		Rejection: &theme.Rejection{Reason: theme.ReasonBadColour},
	}
}

// arrowValidRows returns n selectable rows, each with its own palette.
func arrowValidRows(n int) []theme.Row {
	rows := make([]theme.Row, 0, n)
	for i := range n {
		rows = append(rows, arrowValidRow(arrowSlug(i), i))
	}
	return rows
}

// arrowSlug names the i-th fixture row. Zero-padded so the fixture order is also
// the alphabetical order a real union would arrive in.
func arrowSlug(i int) string {
	return "theme-" + string(rune('a'+i/10)) + string(rune('0'+i%10))
}

// arrowRowBySlug returns the fixture row with the given slug.
func arrowRowBySlug(t *testing.T, rows []theme.Row, slug string) theme.Row {
	t.Helper()
	for _, row := range rows {
		if row.Slug == slug {
			return row
		}
	}
	t.Fatalf("fixture has no row %q", slug)
	return theme.Row{}
}

// newArrowPanelDeps assembles the Build deps for an arrow fixture: a stub seam
// answering with the given rows verbatim (so the fixture's ORDER is the panel's
// order) and a resolution naming cursorSlug's row, which is where §9.2's open
// lands the cursor.
func newArrowPanelDeps(t *testing.T, rows []theme.Row, cursorSlug string) Deps {
	t.Helper()
	target := arrowRowBySlug(t, rows, cursorSlug)
	return Deps{
		Lister: fakeLister{},
		Theme:  theme.ConstantNomination(target.Theme),
		ThemeEnumerator: &stubThemeEnumerator{
			union: theme.Union{Rows: rows, Count: len(rows), Rejected: arrowRejectedCount(rows)},
			resolution: theme.Resolution{
				Nomination: theme.ConstantNomination(target.Theme),
				Slots: []theme.SlotResolution{{
					Slot:      theme.SlotConstant,
					Requested: cursorSlug,
					Resolved:  cursorSlug,
					Theme:     target.Theme,
				}},
			},
		},
		ThemeKeys: theme.RawKeys{Theme: cursorSlug},
	}
}

// arrowRejectedCount counts the unselectable fixture rows.
func arrowRejectedCount(rows []theme.Row) int {
	n := 0
	for _, row := range rows {
		if !row.Selectable() {
			n++
		}
	}
	return n
}

// newArrowPanelModel builds a renderable Sessions model over the given rows and
// OPENS the panel through the production `t` keypress, so every assertion below is
// about the live dispatch rather than a hand-installed panel.
func newArrowPanelModel(t *testing.T, rows []theme.Row, cursorSlug string) Model {
	t.Helper()
	return newArrowPanelModelAt(t, rows, cursorSlug, arrowTermH)
}

// newArrowPanelModelAt is newArrowPanelModel at a chosen terminal HEIGHT — the one
// input the panel's page size is a function of.
//
// The height is set BEFORE the `t` keypress, so the panel's list is sized by
// PRODUCTION (applyThemePanelListStyles) at exactly the height the fixture renders
// at. Nothing here sizes the list.
func newArrowPanelModelAt(t *testing.T, rows []theme.Row, cursorSlug string, termH int) Model {
	t.Helper()
	m := Build(newArrowPanelDeps(t, rows, cursorSlug))
	m.termWidth, m.termHeight = arrowTermW, termH
	m.applySessions([]tmux.Session{
		{Name: "alpha", Windows: 3, Attached: true},
		{Name: "bravo", Windows: 1},
		{Name: "charlie", Windows: 2},
	})
	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("fixture: the panel did not open")
	}
	requireCursorOn(t, m, cursorSlug)
	return m
}

// requireArrowPanelPageSize pins the page PRODUCTION gives the panel's list at this
// fixture's terminal height.
//
// It is a PRECONDITION, never a setter. A one-row page makes `Ctrl+↑`/`Ctrl+↓`
// behaviourally identical to `↑`/`↓` and makes every skip-across-a-page-boundary
// fixture degenerate, so the paging cases state the page they expect and fail loudly
// if the production arithmetic stops delivering it.
func requireArrowPanelPageSize(t *testing.T, m Model, want int) {
	t.Helper()
	if got := m.themePanel.list.Paginator.PerPage; got != want {
		t.Fatalf("fixture: the panel's list paginates %d row(s) per page, want %d", got, want)
	}
}

// arrowPanelIndex is the panel cursor's index into the union.
func arrowPanelIndex(m Model) int { return m.themePanel.list.Index() }

// requireArrowUnskippedLandingAt pins where the panel's list lands the cursor with
// NO skip applied — the fixture precondition every skip case needs, since a landing
// on a selectable row leaves no skip to exercise at all.
//
// AFTER A SINGLE-ROW STEP a reversal also ends where it began (there is no
// selectable row in the direction of travel, so the only place to turn back to is
// the row the cursor was already on), which makes "the cursor did not move"
// indistinguishable from "the arrow was never routed"; a PAGE move jumps over rows
// without checking them, so it can settle elsewhere and carries no such blindness.
// Driving the list directly says what the keypress would have done unskipped: land
// on an unselectable row. `bubbles/list`'s Update has a value receiver, so this
// observes without mutating the model.
func requireArrowUnskippedLandingAt(t *testing.T, m Model, press tea.KeyPressMsg, want int) {
	t.Helper()
	raw, _ := m.themePanel.list.Update(press)
	if got := raw.Index(); got != want {
		t.Fatalf("fixture: unskipped, %v lands the list at index %d, want the unselectable %d", press, got, want)
	}
	if item, ok := raw.SelectedItem().(themeRowItem); !ok || item.Row.Selectable() {
		t.Fatalf("fixture: unskipped, %v lands on a SELECTABLE row, so there is no skip to exercise", press)
	}
}

// requireArrowCursorAt fails unless the cursor is on the row at index want.
func requireArrowCursorAt(t *testing.T, m Model, want int) {
	t.Helper()
	if got := arrowPanelIndex(m); got != want {
		t.Fatalf("the panel cursor is at index %d, want %d; rows: %v", got, want, themePanelRowLabels(m))
	}
}

// TestPanelArrow_NavigationBindings: it moves the cursor on arrows and pages on
// ctrl-arrows.
//
// §9.2's first two rows, end to end: `↑`/`↓` step one row and `Ctrl+↑`/`Ctrl+↓`
// move a whole page, driven through the live Update so what is proven is the
// panel's own routing rather than a call into `bubbles/list`.
//
// THE PAGE IS THE PRODUCTION ONE (see arrowPagingTermH). `Ctrl+↓` moving MORE THAN
// ONE ROW is the load-bearing assertion: a panel list left at a one-row page routes
// the paging keys perfectly and still steps a single row, which is indistinguishable
// from `↓` on the screen.
func TestPanelArrow_NavigationBindings(t *testing.T) {
	rows := arrowValidRows(6)
	m := newArrowPanelModelAt(t, rows, arrowSlug(0), arrowPagingTermH)
	requireArrowPanelPageSize(t, m, arrowPagingPerPage)
	requireArrowCursorAt(t, m, 0)

	m = pressPanelKey(t, m, arrowDown)
	requireArrowCursorAt(t, m, 1)

	m = pressPanelKey(t, m, arrowUp)
	requireArrowCursorAt(t, m, 0)

	m = pressPanelKey(t, m, arrowPageDown)
	if got := arrowPanelIndex(m); got <= 1 {
		t.Fatalf("Ctrl+↓ moved the panel cursor from index 0 to %d — it steps one row rather than paging, so it does nothing `↓` does not", got)
	}
	requireArrowCursorAt(t, m, arrowPagingPerPage)
	if got := m.themePanel.list.Paginator.Page; got != 1 {
		t.Errorf("Ctrl+↓ left the paginator on page %d, want 1 — it pages rather than stepping", got)
	}

	m = pressPanelKey(t, m, arrowPageUp)
	requireArrowCursorAt(t, m, 0)

	if !m.themePanel.open {
		t.Error("navigating closed the panel; arrows leave it open (§9.2)")
	}
}

// TestPanelArrow_ArrowOnlyNavigation: it binds no vim alias or page-jump key.
//
// §12.2's arrow-only revision applies to the panel's list exactly as
// pinArrowOnlyNav applies it to the other two: the v2 DefaultKeyMap re-introduces
// h/j/k/l, g/G and PgUp/PgDn/Home/End/b/u/f/d, and `l`/`d` additionally collide with
// the panel's own commit keys (§9.2). The binding table is asserted alongside the
// behaviour, because a key that reaches the list at all is a key the panel has
// already lost control of.
//
// `←`/`→`/`d` are in the banned set because the v2 default binds all three to a
// PAGE (`left`/`h`/`pgup`/`b`/`u` and `right`/`l`/`pgdown`/`f`/`d`), and `d` is the
// one §9.2 hands to the dark slot — a `d` that reached the list would page the panel
// instead of committing.
//
// The fixture runs at the PAGING terminal height so the page keys have a page to
// move to: on a single-page list every banned page key is a no-op whatever it is
// bound to, and the behavioural half would pass for the wrong reason.
func TestPanelArrow_ArrowOnlyNavigation(t *testing.T) {
	rows := arrowValidRows(6)
	m := newArrowPanelModelAt(t, rows, arrowSlug(2), arrowPagingTermH)
	requireArrowPanelPageSize(t, m, arrowPagingPerPage)

	km := m.themePanel.list.KeyMap
	for _, tc := range []struct {
		name    string
		binding key.Binding
		want    []string
	}{
		{name: "CursorUp", binding: km.CursorUp, want: []string{"up"}},
		{name: "CursorDown", binding: km.CursorDown, want: []string{"down"}},
		{name: "PrevPage", binding: km.PrevPage, want: []string{"ctrl+up"}},
		{name: "NextPage", binding: km.NextPage, want: []string{"ctrl+down"}},
		{name: "GoToStart", binding: km.GoToStart, want: nil},
		{name: "GoToEnd", binding: km.GoToEnd, want: nil},
	} {
		if got := tc.binding.Keys(); strings.Join(got, ",") != strings.Join(tc.want, ",") {
			t.Errorf("the panel list's %s binds %v, want %v — the panel is arrow-only (§12.2)", tc.name, got, tc.want)
		}
	}

	start := arrowPanelIndex(m)
	for _, press := range []tea.KeyPressMsg{
		{Code: 'j', Text: "j"},
		{Code: 'k', Text: "k"},
		{Code: 'h', Text: "h"},
		{Code: 'l', Text: "l"},
		{Code: 'g', Text: "g"},
		{Code: 'G', Text: "G"},
		{Code: 'd', Text: "d"},
		{Code: tea.KeyLeft},
		{Code: tea.KeyRight},
		{Code: tea.KeyPgUp},
		{Code: tea.KeyPgDown},
		{Code: tea.KeyHome},
		{Code: tea.KeyEnd},
		{Code: ' ', Text: " "},
	} {
		moved := pressPanelKey(t, m, press)
		if got := arrowPanelIndex(moved); got != start {
			t.Errorf("%v moved the panel cursor to index %d, want it left at %d — only arrows navigate", press, got, start)
		}
		if !moved.themePanel.open {
			t.Errorf("%v closed the panel", press)
		}
	}
}

// arrowFrameCells returns the composed frame's per-cell background state, row by
// row — the observation both halves of the re-theme assertion are made on. A
// glyph-only comparison could not tell a re-themed canvas from an unchanged one.
func arrowFrameCells(t *testing.T, m Model) [][]bgState {
	t.Helper()
	lines := strings.Split(m.View().Content, "\n")
	cells := make([][]bgState, 0, len(lines))
	for _, line := range lines {
		cells = append(cells, scanCellBackgrounds(line))
	}
	return cells
}

// arrowPanelColumn / arrowMainColumn are the composed frame's column indices
// INSIDE the panel and to the LEFT of it, so "the main screen re-themed" and "the
// panel re-themed" are two separate observations rather than one.
func arrowPanelColumn(m Model) int {
	left := (m.termWidth-m.contentWidth())/2 + m.contentWidth() - themePanelPreferredWidth
	// +1 clears the one-cell left border, which is the `border` token rather than the
	// canvas. The cell it lands on is the panel's inner gutter — canvas on every row,
	// which is exactly what a canvas observation wants.
	return left + 1
}

func arrowMainColumn(m Model) int {
	return (m.termWidth - m.contentWidth()) / 2
}

// TestPanelArrow_PreviewsThroughApplyTheme: it re-themes the main screen and the
// panel together.
//
// §9.2 ("the app re-themes live behind the panel") and §9.11 ("the slide-over's own
// chrome re-themes with the previewed theme, no exceptions") are ONE keypress, so
// they are asserted as one diff of the composed frame across a single `↓`: a cell
// inside the panel and a cell of the main screen both carry the newly-selected
// row's canvas and neither carries the previous row's.
func TestPanelArrow_PreviewsThroughApplyTheme(t *testing.T) {
	rows := arrowValidRows(4)
	before, after := rows[0].Theme, rows[1].Theme
	m := newArrowPanelModel(t, rows, rows[0].Slug)

	if m.activeTheme != before {
		t.Fatalf("fixture: the open painted canvas %s, want the cursor row's %s", m.activeTheme.Canvas.Value, before.Canvas.Value)
	}
	beforeParams, afterParams := canvasBgParams(before.Canvas.Color()), canvasBgParams(after.Canvas.Color())
	if beforeParams == afterParams {
		t.Fatalf("fixture: the two rows paint identically (%s), so a diff proves nothing", beforeParams)
	}

	beforeCells := arrowFrameCells(t, m)
	m = pressPanelKey(t, m, arrowDown)
	afterCells := arrowFrameCells(t, m)

	if m.activeTheme != after {
		t.Fatalf("the arrow left the active theme at canvas %s, want the newly-selected row's %s", m.activeTheme.Canvas.Value, after.Canvas.Value)
	}

	row := arrowTermH / 2
	for _, tc := range []struct {
		name string
		col  int
	}{
		{name: "the main screen behind the panel", col: arrowMainColumn(m)},
		{name: "the panel's own chrome", col: arrowPanelColumn(m)},
	} {
		if got := beforeCells[row][tc.col].params; got != beforeParams {
			t.Fatalf("fixture: %s was painted %q before the arrow, want the pre-swap canvas %q", tc.name, got, beforeParams)
		}
		if got := afterCells[row][tc.col].params; got != afterParams {
			t.Errorf("%s is painted %q after the arrow, want the previewed canvas %q — every surface re-themes (§9.11)", tc.name, got, afterParams)
		}
	}
}

// TestPanelArrow_SkipsConsecutiveInvalidRows: it skips a block of adjacent invalid
// rows.
//
// The one deliberate difference from the group-header skip the mechanism is
// modelled on: two headers are never adjacent, whereas several broken drop-ins in
// one directory ARE — so a single step is not enough and the skip loops.
func TestPanelArrow_SkipsConsecutiveInvalidRows(t *testing.T) {
	rows := []theme.Row{
		arrowValidRow("aaa", 0),
		arrowInvalidRow("bbb"),
		arrowInvalidRow("ccc"),
		arrowInvalidRow("ddd"),
		arrowValidRow("eee", 1),
	}
	m := newArrowPanelModel(t, rows, "aaa")

	m = pressPanelKey(t, m, arrowDown)

	requireArrowCursorAt(t, m, 4)
	requireCursorOn(t, m, "eee")
	if row := themePanelCursorRow(t, m); !row.Selectable() {
		t.Errorf("the cursor rests on the unselectable %q", row.Label())
	}
	if want := rows[4].Theme; m.activeTheme != want {
		t.Errorf("the skip landed without previewing: canvas %s, want %s", m.activeTheme.Canvas.Value, want.Canvas.Value)
	}
}

// TestPanelArrow_SkipReversesAtTheBoundary: it reverses rather than falling off the
// list.
//
// The second deliberate difference from skipHeaderRow, in both directions: with no
// selectable row left in the direction of travel the loop turns round rather than
// parking on an invalid row or running off the end. Built-ins are always valid
// (§7.6), so a selectable row always exists for it to turn back to.
//
// AFTER A SINGLE-ROW STEP — which is what both cases below take — A REVERSAL ENDS
// WHERE IT BEGAN: everything between the boundary and the starting row was walked
// and found unselectable, so the row it turns back to is the one the cursor was
// already on. (A PAGE move jumps over rows without checking them, so its reversal
// can settle on one of those instead.) "The cursor did not move" is therefore not
// self-evidently a reversal, and each case pins it from both sides: the unskipped
// landing (an unselectable row, so there IS a skip to run) and the opposite arrow
// (which does move, so the arm is routed at all).
func TestPanelArrow_SkipReversesAtTheBoundary(t *testing.T) {
	t.Run("upward into a leading invalid block", func(t *testing.T) {
		rows := []theme.Row{
			arrowInvalidRow("aaa"),
			arrowInvalidRow("bbb"),
			arrowValidRow("ccc", 0),
			arrowValidRow("ddd", 1),
		}
		m := newArrowPanelModel(t, rows, "ccc")
		requireArrowUnskippedLandingAt(t, m, arrowUp, 1)
		requireArrowMoves(t, m, arrowDown, 3)

		m = pressPanelKey(t, m, arrowUp)

		requireArrowCursorAt(t, m, 2)
		requireCursorOn(t, m, "ccc")
		if want := rows[2].Theme; m.activeTheme != want {
			t.Errorf("the reversal moved the preview to canvas %s, want the unchanged %s", m.activeTheme.Canvas.Value, want.Canvas.Value)
		}
	})

	t.Run("downward into a trailing invalid block", func(t *testing.T) {
		rows := []theme.Row{
			arrowValidRow("aaa", 0),
			arrowValidRow("bbb", 1),
			arrowInvalidRow("ccc"),
			arrowInvalidRow("ddd"),
		}
		m := newArrowPanelModel(t, rows, "bbb")
		requireArrowUnskippedLandingAt(t, m, arrowDown, 2)
		requireArrowMoves(t, m, arrowUp, 0)

		m = pressPanelKey(t, m, arrowDown)

		requireArrowCursorAt(t, m, 1)
		requireCursorOn(t, m, "bbb")
		if want := rows[1].Theme; m.activeTheme != want {
			t.Errorf("the reversal moved the preview to canvas %s, want the unchanged %s", m.activeTheme.Canvas.Value, want.Canvas.Value)
		}
	})
}

// requireArrowMoves fails unless press moves the panel cursor to want — the
// companion half of each reversal case's non-vacuity, proving the arrow arm is live
// in this exact fixture and state.
func requireArrowMoves(t *testing.T, m Model, press tea.KeyPressMsg, want int) {
	t.Helper()
	if got := arrowPanelIndex(pressPanelKey(t, m, press)); got != want {
		t.Fatalf("fixture: %v left the cursor at index %d, want %d — the arrow arm is not live here, so the reversal below proves nothing", press, got, want)
	}
}

// TestPanelArrow_SkipComposesWithPaging: it composes the skip with paging.
//
// §9.5: "the skip composes with paging exactly as the group-header skip already
// does". `Ctrl+↓` lands on a page whose FIRST row is invalid, and the skip must
// resolve that within the page it just landed on rather than undoing the page move.
//
// The fixture is only a fixture at the PRODUCTION page size: on a one-row page
// `Ctrl+↓` would land on index 1 — a valid row — and there would be no skip to
// compose with anything, which is why the page is pinned rather than installed.
func TestPanelArrow_SkipComposesWithPaging(t *testing.T) {
	rows := []theme.Row{
		arrowValidRow("aaa", 0),
		arrowValidRow("bbb", 1),
		arrowInvalidRow("ccc"),
		arrowValidRow("ddd", 2),
	}
	m := newArrowPanelModelAt(t, rows, "aaa", arrowPagingTermH)
	requireArrowPanelPageSize(t, m, arrowPagingPerPage)
	requireArrowUnskippedLandingAt(t, m, arrowPageDown, 2)

	m = pressPanelKey(t, m, arrowPageDown)

	requireArrowCursorAt(t, m, 3)
	requireCursorOn(t, m, "ddd")
	if got := m.themePanel.list.Paginator.Page; got != 1 {
		t.Errorf("the skip left the paginator on page %d, want the page Ctrl+↓ moved to (1)", got)
	}
	if want := rows[3].Theme; m.activeTheme != want {
		t.Errorf("the paged landing did not preview: canvas %s, want %s", m.activeTheme.Canvas.Value, want.Canvas.Value)
	}
}

// TestPanelArrow_NoFileReadPerKeystroke: it reads no file per keystroke.
//
// §5.8's retention made executable, in the only way that cannot be faked: the
// themes directory is DELETED after the panel opened, and arrowing still previews
// the drop-in's own palette. A preview reading the file per keystroke could not
// produce that colour at all.
func TestPanelArrow_NoFileReadPerKeystroke(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	writeThemeFileForTest(t, dir, "sunset.theme", "#202020")
	loader, _ := themeOpenTestLoader(t)
	enumerator := countingEnumeratorOver(loader, dir)

	m := New(fakeLister{},
		WithThemeEnumerator(enumerator),
		WithThemeKeys(theme.RawKeys{Theme: "aurora"}),
		WithThemeNomination(theme.ConstantNomination(testDarkTheme(t))),
	)
	m = pressThemeKey(t, m)
	requireCursorOn(t, m, "aurora")
	if got := m.activeTheme.Canvas.Value; got != "#101010" {
		t.Fatalf("fixture: the open painted canvas %s, want the drop-in's #101010", got)
	}

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove the themes directory: %v", err)
	}

	// Walk down until the cursor reaches the OTHER drop-in. Its palette exists
	// nowhere but the retained parse now.
	const maxPresses = 12
	landed := false
	for range maxPresses {
		m = pressPanelKey(t, m, arrowDown)
		if themePanelCursorRow(t, m).Label() == "sunset" {
			landed = true
			break
		}
	}
	if !landed {
		t.Fatalf("the cursor never reached the `sunset` row in %d presses; rows: %v", maxPresses, themePanelRowLabels(m))
	}
	if got := m.activeTheme.Canvas.Value; got != "#202020" {
		t.Errorf("after the directory was removed the preview painted canvas %s, want the retained parse's #202020", got)
	}
	if enumerator.opens != 1 {
		t.Errorf("arrowing ran %d enumerations, want the single one the open performed", enumerator.opens)
	}
}

// newArrowRebuildProbeModel builds a grouped-mode model whose lazy per-session
// dir-resolution pass is ARMED — every session's Dir empty, the reader counting —
// with the panel open over rows carrying distinct palettes.
//
// The pre-render and the re-arm mirror newSwapProbeModel's: applySessions caches
// each derived dir back onto m.sessions, and a warm cache would make "zero reads" a
// true statement about the wrong thing.
func newArrowRebuildProbeModel(t *testing.T, rows []theme.Row, reader *fakeStamper) Model {
	t.Helper()

	deps := newArrowPanelDeps(t, rows, rows[0].Slug)
	deps.InitialMode = prefs.ModeByProject
	deps.DirReader = reader
	deps.DirRunner = &fakeDirRunner{gitRoot: reader.path}

	projects := []project.Project{{Path: reader.path, Name: "Portal", Tags: []string{"work"}}}
	sessions := make([]tmux.Session, 0, swapProbeSessions)
	for i := range swapProbeSessions {
		sessions = append(sessions, tmux.Session{Name: nameN(i), Windows: 1})
	}

	m := Build(deps)
	m.termWidth, m.termHeight = arrowTermW, arrowTermH
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	m.applySessions(sessions)
	_ = m.viewSessionList()

	for i := range m.sessions {
		m.sessions[i].Dir = ""
	}
	return pressThemeKey(t, m)
}

// TestPanelArrow_DoesNotRebuildSessionList: it does not rebuild the session list.
//
// §11.1's central prohibition on the surface it matters most: a rebuild re-derives
// the item list and, in grouped modes, pays the lazy per-session tmux pane reads
// (the known ~0.5s By-Project cost at ~38 sessions). Nothing that heavy may sit on
// a path the user takes on every arrow keypress.
//
// The positive control is what makes the zero an absence of reads rather than an
// absence of counting.
func TestPanelArrow_DoesNotRebuildSessionList(t *testing.T) {
	rows := arrowValidRows(4)
	reader := &fakeStamper{path: t.TempDir()}
	m := newArrowRebuildProbeModel(t, rows, reader)

	reader.reads = nil
	for range 10 {
		m = pressPanelKey(t, m, arrowDown)
		if arrowPanelIndex(m) == 0 {
			t.Fatal("fixture: `↓` did not move the panel cursor, so a read count of zero says nothing about the preview path")
		}
		m = pressPanelKey(t, m, arrowUp)
	}

	if len(reader.reads) != 0 {
		t.Errorf("twenty arrow presses performed %d lazy pane read(s) %v — a preview is the O(1) restyle (§11.1), never the rebuild's dir-resolution pass", len(reader.reads), reader.reads)
	}
	if got := m.View().Content; !strings.Contains(got, tokenBgSeq(t, m.activeTheme.Canvas)) {
		t.Errorf("the post-arrow frame does not carry the previewed canvas, so the zero above is a swap that never happened")
	}
	if len(reader.reads) != 0 {
		t.Errorf("rendering after an arrow performed %d lazy pane read(s) %v — the render path pays no reads either", len(reader.reads), reader.reads)
	}

	m.rebuildSessionList()
	if len(reader.reads) == 0 {
		t.Fatal("positive control: rebuildSessionList performed no pane reads, so the counting DirReader proves nothing about the arrow path")
	}
}

// TestPanelArrow_StartupCanvasHexUnmoved: it never moves the startup canvas hex.
//
// §11.4's anchor: the exit-time canvas restore compares against the canvas in force
// during the STARTUP window, and an uncommitted preview is precisely the state that
// would otherwise poison it — the user quits with `Ctrl-C` and a colour they never
// chose stays stuck in their terminal. Both counts are asserted because a swap that
// WRITES the hex moves it once, while one that re-derives it drifts only under
// repetition.
func TestPanelArrow_StartupCanvasHexUnmoved(t *testing.T) {
	rows := arrowValidRows(6)
	m := newArrowPanelModel(t, rows, rows[0].Slug)

	startup := m.startupCanvasHex
	if startup != rows[0].Theme.Canvas.Value {
		t.Fatalf("fixture: startupCanvasHex = %q, want the launch canvas %q", startup, rows[0].Theme.Canvas.Value)
	}

	m = pressPanelKey(t, m, arrowDown)
	if m.activeTheme == rows[0].Theme {
		t.Fatal("fixture: one arrow did not change the active theme, so the assertion below is vacuous")
	}
	if m.startupCanvasHex != startup {
		t.Errorf("after one arrow startupCanvasHex = %q, want %q unchanged — it is frozen at gate resolution (§11.4)", m.startupCanvasHex, startup)
	}

	for range 25 {
		m = pressPanelKey(t, m, arrowDown)
		m = pressPanelKey(t, m, arrowUp)
	}
	if m.startupCanvasHex != startup {
		t.Errorf("after fifty arrows startupCanvasHex = %q, want %q unchanged", m.startupCanvasHex, startup)
	}
}

// TestPanelArrow_WritesNothing: it writes nothing on an arrow.
//
// §9.2 states it as part of the keymap itself — "the app re-themes live behind the
// panel. Nothing is written." Every write is an explicit commit keypress (Phase 9),
// so an arrow must persist no preference and must not touch the themes directory it
// previews from.
//
// THE WRITE IS OBSERVED AT THE SEAM, NOT ON DISK. internal/tui resolves no config
// path and reads no PORTAL_* env var: every route from this package to prefs.json
// runs through an injected persister, so watching a prefs file here would be
// watching a file nothing in this package could write whatever the preview did. The
// mode persister is the only such seam an arrow could reach in this phase — §9.2's
// commit keys are Phase 9's and the panel's default arm swallows them, so the theme
// persister has no dispatch to reach it from yet.
//
// The themes directory IS watched on disk, because the enumeration seam really does
// reach it (§5.8's retention is what keeps a keypress off it).
func TestPanelArrow_WritesNothing(t *testing.T) {
	t.Run("arrowing persists no preference", func(t *testing.T) {
		persister := &countingModePersister{}
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
		m := themeCursorModel(t, dir, theme.RawKeys{Theme: "sunset"}, appearanceDarkCanvas)
		WithModePersister(persister)(&m)

		m = pressThemeKey(t, m)
		launch := m.activeTheme
		for range 8 {
			m = pressPanelKey(t, m, arrowDown)
			if m.activeTheme == launch {
				t.Fatal("fixture: `↓` previewed nothing, so a zero write count says nothing about the preview path")
			}
			m = pressPanelKey(t, m, arrowUp)
		}

		if persister.calls != 0 {
			t.Errorf("sixteen arrows persisted %d preference(s); every write is an explicit commit keypress (§9.2)", persister.calls)
		}

		// Positive control: the seam is wired to THIS model and live, so the zero
		// above is an absence of writes rather than an absence of wiring. `s` is
		// swallowed while the panel is open (§9.7), so it is closed first.
		m = pressPanelKey(t, closeThemePanelForTest(t, m), tea.KeyPressMsg{Code: 's', Text: "s"})
		if persister.calls != 1 {
			t.Fatalf("positive control: `s` on the closed picker persisted %d time(s), want 1 — the counting persister proves nothing about the arrows", persister.calls)
		}
	})

	t.Run("the themes directory is untouched", func(t *testing.T) {
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
		m := themeCursorModel(t, dir, theme.RawKeys{Theme: "sunset"}, appearanceDarkCanvas)

		m = pressThemeKey(t, m)
		launch := m.activeTheme
		for range 8 {
			m = pressPanelKey(t, m, arrowDown)
		}
		if m.activeTheme == launch {
			t.Fatal("fixture: arrowing previewed nothing, so an untouched directory says nothing about the preview path")
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		if len(entries) != 1 || entries[0].Name() != "sunset.theme" {
			t.Errorf("the themes directory holds %d entries after arrowing, want only the seeded drop-in", len(entries))
		}
	})
}

// TestPanelArrow_PanelListStylesRepointed: it re-points the panel's own list
// styles.
//
// §11.2 names the panel's `bubbles/list` instance the WORST CASE of the
// cached-style class: its styles are assigned once at open while its theme changes
// on every arrow. The pagination dots are the class's exemplar — `bubbles/list`
// reads its dot STRINGS out of the styles once at construction — so the union is
// large enough to paginate and the RENDERED dot row is diffed, which is the only
// observation a re-point of the styles alone cannot satisfy.
func TestPanelArrow_PanelListStylesRepointed(t *testing.T) {
	before, after := probeThemeBefore(), probeThemeAfter()
	rows := make([]theme.Row, 0, 20)
	for i := range 20 {
		palette := before
		if i == 1 {
			palette = after
		}
		rows = append(rows, theme.Row{Slug: arrowSlug(i), Source: theme.SourceBuiltin, Theme: palette})
	}
	m := newArrowPanelModel(t, rows, arrowSlug(0))

	const height = 16
	dotsBefore := themePanelDotRow(t, renderThemePanel(m.themePanel, height, m.activeTheme, m.colourless))

	m = pressPanelKey(t, m, arrowDown)
	if m.activeTheme != after {
		t.Fatalf("fixture: the arrow left the active theme at canvas %s, want %s", m.activeTheme.Canvas.Value, after.Canvas.Value)
	}
	panel := renderThemePanel(m.themePanel, height, m.activeTheme, m.colourless)
	dotsAfter := themePanelDotRow(t, panel)

	if dotsAfter == dotsBefore {
		t.Fatal("the rendered dot row is byte-identical across the swap — the paginator is still printing the pre-swap dots")
	}
	assertRepointed(t, "the panel's rendered active dot",
		dotsAfter, tokenFgSeq(t, after.AccentPrimary), tokenFgSeq(t, before.AccentPrimary))
	assertRepointed(t, "the panel's rendered inactive dot",
		dotsAfter, tokenFgSeq(t, after.TextFaint), tokenFgSeq(t, before.TextFaint))

	l := &m.themePanel.list
	assertRepointed(t, "the panel list's Paginator.ActiveDot",
		l.Paginator.ActiveDot, tokenFgSeq(t, after.AccentPrimary), tokenFgSeq(t, before.AccentPrimary))
	assertRepointed(t, "the panel list's Paginator.InactiveDot",
		l.Paginator.InactiveDot, tokenFgSeq(t, after.TextFaint), tokenFgSeq(t, before.TextFaint))
	assertRepointed(t, "the panel list's Styles.HelpStyle",
		l.Styles.HelpStyle.Render("x"), tokenBgSeq(t, after.Canvas), tokenBgSeq(t, before.Canvas))
	assertRepointed(t, "the panel list's Styles.TitleBar",
		l.Styles.TitleBar.Render("x"), tokenBgSeq(t, after.Canvas), tokenBgSeq(t, before.Canvas))
	// Styles.NoItems is the last bubbles/list-owned colour-bearing style on this
	// instance, and it is re-pointed on the same terms as the two main lists' rather
	// than justified as unreachable: the panel arm's coverage of §11.2's class is
	// asserted per VALUE, which is the discipline applyCanvasMode's residue record
	// adopted after a blanket "none of these renders" claim let this exact style sit
	// unnoticed while reaching a real frame.
	assertRepointed(t, "the panel list's Styles.NoItems",
		l.Styles.NoItems.Render("x"), tokenFgSeq(t, after.TextMuted), tokenFgSeq(t, before.TextMuted))
	if titleRun := l.Styles.Title.Render("x"); strings.ContainsRune(titleRun, '\x1b') {
		t.Errorf("the panel list's Styles.Title emits colour %q — nothing paints the title box", escSeq(titleRun))
	}

	// The delegate is the other half of §11.2's rule: the cursor row's label is
	// text.on-selection, which no other panel surface paints.
	assertRepointed(t, "the panel's row delegate (cursor-row label)",
		panel, tokenFgSeq(t, after.TextOnSelection), tokenFgSeq(t, before.TextOnSelection))
}

// TestPanelArrow_ColourlessStaysColourless: it keeps a colourless model colourless.
//
// §2.5's carve-out across a preview swap: Portal imposes no hue under NO_COLOR, and
// re-pointing three lists' worth of cached styles is exactly where one would leak
// back in. The coloured twin is the positive control, so the two negatives are
// absences rather than a frame that never had colour.
func TestPanelArrow_ColourlessStaysColourless(t *testing.T) {
	const (
		fgTruecolor = "38;2;"
		bgTruecolor = "48;2;"
	)
	rows := arrowValidRows(4)

	build := func(colourless bool) Model {
		deps := newArrowPanelDeps(t, rows, rows[0].Slug)
		deps.NoColor = colourless
		m := Build(deps)
		m.termWidth, m.termHeight = arrowTermW, arrowTermH
		m.applySessions([]tmux.Session{{Name: "alpha", Windows: 1}, {Name: "bravo", Windows: 2}})
		// §9.10's entry gate BLOCKS `t` under NO_COLOR, so the colourless twin arms the
		// panel directly (armPanelUnderNoColorForTest states why) while the coloured
		// control still opens through the production keypress. The preview swap below is
		// the same on both.
		if colourless {
			m = armPanelUnderNoColorForTest(t, m)
		} else {
			m = pressThemeKey(t, m)
		}
		m = pressPanelKey(t, m, arrowDown)
		if arrowPanelIndex(m) != 1 {
			t.Fatalf("fixture (colourless=%v): `↓` left the cursor at index %d, so the frame below is not a post-preview one", colourless, arrowPanelIndex(m))
		}
		return m
	}

	frame := build(true).View().Content
	if strings.Contains(frame, fgTruecolor) {
		t.Errorf("the post-preview colourless frame carries a %q foreground sequence: %q", fgTruecolor, escSeq(frame))
	}
	if strings.Contains(frame, bgTruecolor) {
		t.Errorf("the post-preview colourless frame carries a %q background sequence: %q", bgTruecolor, escSeq(frame))
	}

	control := build(false).View().Content
	if !strings.Contains(control, fgTruecolor) || !strings.Contains(control, bgTruecolor) {
		t.Fatal("positive control: the COLOURED post-preview frame carries no truecolor SGR, so the colourless assertions prove nothing")
	}
}

// TestPanelArrow_SameRowIsANoOp: it treats an unchanged selection as a no-op.
//
// Two shapes of "nothing to do", both reachable on an ordinary keypress: an arrow
// at the boundary that cannot move the cursor at all, and a round trip that lands
// back on the row already active. Neither may leave a mark on the frame, and a
// repeated swap must render exactly what the first one rendered — the restyle
// carries no accumulating state.
func TestPanelArrow_SameRowIsANoOp(t *testing.T) {
	rows := arrowValidRows(3)
	m := newArrowPanelModel(t, rows, rows[0].Slug)
	first := m.View().Content

	t.Run("an arrow that cannot move renders an identical frame", func(t *testing.T) {
		blocked := pressPanelKey(t, m, arrowUp)
		if got := blocked.View().Content; got != first {
			t.Errorf("`↑` on the first row changed the frame\nbefore: %q\nafter:  %q", escSeq(first), escSeq(got))
		}
	})

	t.Run("landing back on the active row restores the identical frame", func(t *testing.T) {
		moved := pressPanelKey(t, m, arrowDown)
		if moved.View().Content == first {
			t.Fatal("fixture: `↓` did not change the frame, so the round trip proves nothing")
		}
		back := pressPanelKey(t, moved, arrowUp)
		if got := back.View().Content; got != first {
			t.Errorf("`↓` then `↑` did not restore the frame\nbefore: %q\nafter:  %q", escSeq(first), escSeq(got))
		}
	})

	t.Run("repeated swaps are idempotent per swap", func(t *testing.T) {
		once := pressPanelKey(t, m, arrowDown).View().Content
		twice := pressPanelKey(t, pressPanelKey(t, pressPanelKey(t, m, arrowDown), arrowUp), arrowDown).View().Content
		if twice != once {
			t.Errorf("A→B→A→B does not render what A→B renders; the preview accumulates state\nonce:  %q\ntwice: %q", escSeq(once), escSeq(twice))
		}
	})
}
