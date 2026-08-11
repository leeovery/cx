package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// Restated verbatim rather than re-derived: a test that formatted the production
// format string would pass whatever the copy became. Double space included.
const (
	messageTestConfirmSlug = "nord"
	messageTestConfirmCopy = "clear constant nord?  y / n"
	messageTestFailedCopy  = "⚠ couldn't save theme"
)

func messageTestConfirm() themePanelMessage {
	return themePanelMessage{Kind: themeMessageConfirm, Slug: messageTestConfirmSlug}
}

func messageTestFailed() themePanelMessage {
	return themePanelMessage{Kind: themeMessageCommitFailed}
}

func messageTestPanel(th theme.Theme, width int, message themePanelMessage) themePanel {
	return newThemePanelFixture(themePanelFixtureOpts{
		th:      th,
		width:   width,
		rows:    themePanelTestRows(12),
		message: message,
	})
}

func messageTestVisible(message themePanelMessage, inner int, wrap bool, th theme.Theme) []string {
	block := renderThemePanelMessage(message, inner, wrap, th, false)
	rows := strings.Split(ansi.Strip(block), "\n")
	for i, row := range rows {
		rows[i] = strings.TrimRight(row, " ")
	}
	return rows
}

// The copy is a layout constraint as much as a copy choice (the panel is 24–30
// columns), so it is byte-compared rather than matched loosely.
func TestPanelMessage_ConfirmPinnedCopy(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelPreferredWidth)

	rows := messageTestVisible(messageTestConfirm(), inner, false, th)
	if len(rows) != 1 {
		t.Fatalf("the confirm rendered %d rows at inner width %d, want 1: %q", len(rows), inner, rows)
	}
	if got := rows[0]; got != messageTestConfirmCopy {
		t.Errorf("the confirm reads %q, want §14A's %q", got, messageTestConfirmCopy)
	}
	if !strings.Contains(rows[0], "?  y") {
		t.Errorf("the confirm reads %q, want the DOUBLE space §14A pins before `y`", rows[0])
	}
}

// The `⚠` is part of the string rather than a colour-only signal.
func TestPanelMessage_CommitFailedPinnedCopy(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelPreferredWidth)

	rows := messageTestVisible(messageTestFailed(), inner, false, th)
	if len(rows) != 1 {
		t.Fatalf("the failed-commit line rendered %d rows, want 1: %q", len(rows), rows)
	}
	if got := rows[0]; got != messageTestFailedCopy {
		t.Errorf("the failed-commit line reads %q, want §14A's %q", got, messageTestFailedCopy)
	}
	if !strings.HasPrefix(rows[0], flashWarningGlyph) {
		t.Errorf("the failed-commit line reads %q, want it glyph-backed with %q", rows[0], flashWarningGlyph)
	}
}

// The exclusion is asserted directly rather than resolved with a precedence rule:
// each raise installs a whole value, so the other contender is gone by construction.
func TestPanelMessage_SingleSlotExclusion(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelPreferredWidth)
	m := newArrowPanelModel(t, arrowValidRows(t, 4), arrowValidRows(t, 4)[0].Slug)
	m.themeState.keys = theme.RawKeys{Theme: messageTestConfirmSlug}

	t.Run("a raised confirm is the only live contender", func(t *testing.T) {
		(&m).raiseThemePanelConfirm()
		if got := m.themePanel.message.Kind; got != themeMessageConfirm {
			t.Fatalf("the slot holds kind %d, want the confirm", got)
		}
		requireOnlyContender(t, m.themePanel.message, inner, th, messageTestConfirmCopy, messageTestFailedCopy)
	})

	t.Run("raising the failure clears the confirm and its slug", func(t *testing.T) {
		(&m).raiseThemePanelCommitFailed()
		if got := m.themePanel.message.Kind; got != themeMessageCommitFailed {
			t.Fatalf("the slot holds kind %d, want the failed commit", got)
		}
		if got := m.themePanel.message.Slug; got != "" {
			t.Errorf("the failed-commit contender carries the slug %q — the confirm's residue survived the swap", got)
		}
		requireOnlyContender(t, m.themePanel.message, inner, th, messageTestFailedCopy, messageTestConfirmCopy)
	})

	t.Run("raising the confirm clears the failure", func(t *testing.T) {
		(&m).raiseThemePanelConfirm()
		if got := m.themePanel.message.Kind; got != themeMessageConfirm {
			t.Fatalf("the slot holds kind %d, want the confirm", got)
		}
		requireOnlyContender(t, m.themePanel.message, inner, th, messageTestConfirmCopy, messageTestFailedCopy)
	})

	t.Run("clearing leaves neither", func(t *testing.T) {
		(&m).clearThemePanelMessage()
		if got := m.themePanel.message; got != (themePanelMessage{}) {
			t.Errorf("the cleared slot holds %+v, want the zero value", got)
		}
		if got := renderThemePanelMessage(m.themePanel.message, inner, false, th, false); got != "" {
			t.Errorf("the cleared slot rendered %q, want nothing", got)
		}
	})
}

func requireOnlyContender(t *testing.T, message themePanelMessage, inner int, th theme.Theme, want, other string) {
	t.Helper()
	got := strings.Join(messageTestVisible(message, inner, false, th), " ")
	if !strings.Contains(got, want) {
		t.Errorf("the slot renders %q, want it carrying %q", got, want)
	}
	if strings.Contains(got, other) {
		t.Errorf("the slot renders %q, want the other contender %q absent", got, other)
	}
}

func TestPanelMessage_UnreservedWhenEmpty(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelPreferredWidth)
	const height = 16

	for _, wrap := range []bool{false, true} {
		if got := renderThemePanelMessage(themePanelMessage{}, inner, wrap, th, false); got != "" {
			t.Errorf("themeMessageNone rendered %q with wrap=%v, want nothing", got, wrap)
		}
		if got := themePanelMessageHeight(themePanelMessage{}, inner, wrap); got != 0 {
			t.Errorf("themeMessageNone reserved %d rows with wrap=%v, want 0", got, wrap)
		}
	}

	// The failed-commit contender leaves the standing footer in place, so the row
	// the slot costs is the only difference between the two layouts.
	_, empty := themePanelListSize(messageTestPanel(th, themePanelPreferredWidth, themePanelMessage{}), height)
	_, live := themePanelListSize(messageTestPanel(th, themePanelPreferredWidth, messageTestFailed()), height)
	if live != empty-1 {
		t.Errorf("the list body is %d rows with a message and %d without, want exactly one fewer", live, empty)
	}
}

// The cost is measured off the rendered block rather than assumed to be one row.
func TestPanelMessage_WrappedMessageCostsTwoRows(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelMinWidth)
	confirm := messageTestConfirm()

	rows := messageTestVisible(confirm, inner, true, th)
	if len(rows) != themePanelMessageWrapRows {
		t.Fatalf("the confirm wrapped to %d rows at inner width %d, want %d: %q", len(rows), inner, themePanelMessageWrapRows, rows)
	}
	if got := themePanelMessageHeight(confirm, inner, true); got != themePanelMessageWrapRows {
		t.Errorf("the wrapped slot measures %d rows, want %d", got, themePanelMessageWrapRows)
	}

	// Nothing is lost in the wrap; where the break lands is not promised.
	if got, want := chromeWords(strings.Join(rows, " ")), chromeWords(messageTestConfirmCopy); got != want {
		t.Errorf("the wrapped slot reads %q, want the whole confirm %q", got, want)
	}

	// The two panels carry the same message and therefore the same footer scope, so
	// the footer term cancels and the only difference is what the slot measured.
	height := themePanelMinHeight(themePanelKeymap(), false) + 6
	if got := themePanelMessageHeight(confirm, themePanelInnerWidth(themePanelPreferredWidth), true); got != 1 {
		t.Fatalf("fixture: the confirm measures %d rows at the preferred width, want 1", got)
	}
	_, oneRow := themePanelListSize(messageTestPanel(th, themePanelPreferredWidth, confirm), height)
	_, twoRows := themePanelListSize(messageTestPanel(th, themePanelMinWidth, confirm), height)
	if want := oneRow - (themePanelMessageWrapRows - 1); twoRows != want {
		t.Errorf("the list body is %d rows under a wrapped message and %d under a one-row one, want %d", twoRows, oneRow, want)
	}
}

// Derived rather than written as a literal, so it stays true if either scope's
// Core membership changes.
func footerScopeSaving() int {
	return themePanelFooterHeight(themePanelKeymap()) - themePanelFooterHeight(themePanelConfirmKeymap())
}

// The floor counts exactly one message row, so a two-row message there would leave
// zero list rows or overflow the frame.
func TestPanelMessage_TruncatesAtFloorHeight(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelMinWidth)
	confirm := messageTestConfirm()
	floor := themePanelMinHeight(themePanelKeymap(), false)
	p := messageTestPanel(th, themePanelMinWidth, confirm)

	if lipgloss.Width(messageTestConfirmCopy) <= inner {
		t.Fatalf("fixture: the %d-cell confirm fits the %d-cell inner width, so neither degradation is exercised",
			lipgloss.Width(messageTestConfirmCopy), inner)
	}

	t.Run("at the floor it renders on exactly one line", func(t *testing.T) {
		if got := themePanelMessageWraps(p, floor); got {
			t.Fatal("the slot wraps at the floor; §9.1 truncates there")
		}
		rows := messageTestVisible(confirm, inner, false, th)
		if len(rows) != 1 {
			t.Fatalf("the slot rendered %d rows at the floor, want exactly 1: %q", len(rows), rows)
		}
		if !strings.Contains(rows[0], themeRowEllipsis) {
			t.Errorf("the slot reads %q, want it truncated with %q", rows[0], themeRowEllipsis)
		}
	})

	t.Run("above the floor the same message may occupy two", func(t *testing.T) {
		if got := themePanelMessageWraps(p, floor+1); !got {
			t.Fatal("the slot truncates one row above the floor; §9.1 wraps there")
		}
		if got := themePanelMessageHeight(confirm, inner, true); got != themePanelMessageWrapRows {
			t.Errorf("above the floor the slot measures %d rows, want %d", got, themePanelMessageWrapRows)
		}
	})

	lines := themePanelLines(renderThemePanel(p, floor, th, false))
	if len(lines) != floor {
		t.Fatalf("the panel rendered %d rows at its floor of %d", len(lines), floor)
	}
	slot := strings.TrimRight(lines[floor-themePanelFooterHeight(themePanelConfirmKeymap())-1], " ")
	if !strings.HasPrefix(slot, themePanelContentPrefix()+"clear constant") {
		t.Errorf("the row above the footer is not the message slot: %q", slot)
	}
}

// The pinned frame is charged first and the flexing slug takes what is left, so a
// long slug can never push `?  y / n` out of the copy.
func TestPanelMessage_ConfirmSlugTruncation(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelMinWidth)
	long := themePanelMessage{Kind: themeMessageConfirm, Slug: "a-very-long-drop-in-theme-slug"}

	rows := messageTestVisible(long, inner, true, th)
	joined := chromeWords(strings.Join(rows, " "))

	if !strings.Contains(joined, themeRowEllipsis) {
		t.Errorf("the confirm reads %q, want the slug truncated with %q", joined, themeRowEllipsis)
	}
	if strings.Contains(joined, long.Slug) {
		t.Errorf("the confirm reads %q, want the %d-cell slug truncated", joined, lipgloss.Width(long.Slug))
	}
	if !strings.HasPrefix(joined, "clear constant ") {
		t.Errorf("the confirm reads %q, want §14A's leading phrase intact", joined)
	}
	if !strings.HasSuffix(joined, "? y / n") {
		t.Errorf("the confirm reads %q, want §14A's trailing keys intact", joined)
	}

	// Three visible characters plus the ellipsis, so the slug stays recognisable.
	if got := lipgloss.Width(strings.Fields(joined)[2]); got != themeRowLabelFloor+1 {
		t.Errorf("the truncated slug plus its `?` is %d cells, want §9.5's floor of %d", got, themeRowLabelFloor+1)
	}

	p := messageTestPanel(th, themePanelMinWidth, long)
	block := renderThemePanel(p, themePanelMinHeight(themePanelKeymap(), false)+4, th, false)
	for i, line := range strings.Split(block, "\n") {
		if got := lipgloss.Width(line); got != themePanelMinWidth {
			t.Errorf("line %d is %d cells wide at the minimum width, want %d: %q", i, got, themePanelMinWidth, ansi.Strip(line))
		}
	}
}

// Under a fallback the persisted slug is the one being cleared and the resolved
// one is a built-in the user never chose, so naming the resolution would ask the
// user to confirm clearing a theme that was never set.
func TestPanelMessage_ConfirmReadsRawKeys(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelPreferredWidth)
	dir := t.TempDir()

	// `ghost` resolves to nothing, so the shipped dark default is on screen while
	// `ghost` stays the persisted constant.
	m, _, _ := newRecomputePanelModel(t, dir, theme.RawKeys{Theme: "ghost"})
	requireCursorOn(t, m, theme.DefaultDarkSlug)

	(&m).raiseThemePanelConfirm()

	got := strings.Join(messageTestVisible(m.themePanel.message, inner, false, th), " ")
	if want := "clear constant ghost?"; !strings.Contains(got, want) {
		t.Errorf("the confirm reads %q, want it naming the persisted constant (%q)", got, want)
	}
	if strings.Contains(got, theme.DefaultDarkSlug) {
		t.Errorf("the confirm reads %q, want the persisted slug rather than the fallback %q", got, theme.DefaultDarkSlug)
	}
}

func TestPanelMessage_ConfirmTokens(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		inner := themePanelInnerWidth(themePanelPreferredWidth)
		row := renderThemePanelMessage(messageTestConfirm(), inner, false, th, false)

		if got := themeRowRunAfter(t, row, tokenFgSeq(t, th.TextSecondary)); got != messageTestConfirmCopy {
			t.Errorf("[%v] the text.secondary run painted %q, want the confirm %q", themeLabel(th), got, messageTestConfirmCopy)
		}
		requireNoBand(t, th, row)
	}
}

// No `bg.attention` band: the warning band is a full-width main-screen flash
// treatment and would read as heavy inside a 24–30 column panel.
func TestPanelMessage_CommitFailedTokens(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		inner := themePanelInnerWidth(themePanelPreferredWidth)
		row := renderThemePanelMessage(messageTestFailed(), inner, false, th, false)

		if got := themeRowRunAfter(t, row, tokenFgSeq(t, th.AccentAttention)); got != messageTestFailedCopy {
			t.Errorf("[%v] the accent.attention run painted %q, want the glyph AND the text (%q)", themeLabel(th), got, messageTestFailedCopy)
		}
		requireNoBand(t, th, row)
	}
}

// bg.attention is named explicitly because it is the one the failed-commit line
// would plausibly reach for.
func requireNoBand(t *testing.T, th theme.Theme, row string) {
	t.Helper()
	if strings.Contains(row, tokenBgSeq(t, th.BgAttention)) {
		t.Errorf("[%v] the message slot paints a bg.attention band: %q", themeLabel(th), escSeq(row))
	}
	for _, cell := range scanCellBackgrounds(row) {
		if cell.set && cell.params != strings.TrimSuffix(strings.TrimPrefix(canvasSeq(t, th), "\x1b["), "m") {
			t.Errorf("[%v] the message slot paints the background %q, want the canvas alone", themeLabel(th), cell.params)
		}
	}
}

// Substituted, not forked: the rendered rows are compared against the same
// renderer handed the confirm scope, so a second footer implementation shows up as
// a difference rather than passing on a coincidence.
func TestPanelFooter_ConfirmScopeSubstitution(t *testing.T) {
	th := testDarkTheme(t)
	const height = 18
	inner := themePanelInnerWidth(themePanelPreferredWidth)
	p := messageTestPanel(th, themePanelPreferredWidth, messageTestConfirm())

	wantFooter := themePanelLines(renderThemePanelFooter(themePanelConfirmKeymap(), inner, th, false))
	if len(wantFooter) != 2 {
		t.Fatalf("the confirm footer is %d rows, want exactly 2 (`y confirm` / `n cancel`)", len(wantFooter))
	}

	lines := themePanelLines(renderThemePanel(p, height, th, false))
	footer := lines[len(lines)-len(wantFooter):]
	for i, want := range wantFooter {
		if got, wantRow := footer[i], themePanelContentPrefix()+want; got != wantRow {
			t.Errorf("footer row %d = %q, want %q", i, got, wantRow)
		}
	}
	for _, pinned := range themePanelFooterPinnedRows() {
		if strings.Contains(strings.Join(lines[len(lines)-len(wantFooter):], "\n"), themePanelFooterCopy(pinned)) {
			t.Errorf("the substituted footer still carries the standing row %q", pinned)
		}
	}

	_, standing := themePanelListSize(messageTestPanel(th, themePanelPreferredWidth, messageTestFailed()), height)
	_, confirming := themePanelListSize(p, height)
	if want := standing + footerScopeSaving(); confirming != want {
		t.Errorf("the list body is %d rows while the confirm is live, want %d", confirming, want)
	}
}

func TestPanelFooter_RevertsAfterConfirm(t *testing.T) {
	th := testDarkTheme(t)
	const height = 18
	inner := themePanelInnerWidth(themePanelPreferredWidth)
	m := newArrowPanelModel(t, arrowValidRows(t, 6), arrowValidRows(t, 6)[0].Slug)
	m.themeState.keys = theme.RawKeys{Theme: messageTestConfirmSlug}
	m.themePanel.width = themePanelPreferredWidth

	standing := themePanelLines(renderThemePanelFooter(themePanelKeymap(), inner, th, false))
	confirming := themePanelLines(renderThemePanelFooter(themePanelConfirmKeymap(), inner, th, false))

	(&m).raiseThemePanelConfirm()
	if got := footerRowsOf(renderThemePanel(m.themePanel, height, th, false), len(confirming)); !equalRows(got, confirming) {
		t.Fatalf("while the confirm is live the footer reads %q, want %q", got, confirming)
	}

	(&m).clearThemePanelMessage()
	if got := footerRowsOf(renderThemePanel(m.themePanel, height, th, false), len(standing)); !equalRows(got, standing) {
		t.Errorf("after the confirm resolved the footer reads %q, want the standing %q", got, standing)
	}
}

func footerRowsOf(block string, n int) []string {
	lines := themePanelLines(block)
	rows := make([]string, 0, n)
	for _, row := range lines[len(lines)-n:] {
		rows = append(rows, strings.TrimPrefix(row, themePanelContentPrefix()))
	}
	return rows
}

func equalRows(got, want []string) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}

// The confirm's footer is strictly shorter, so a panel that clears the floor with
// the standing footer has room to spare the moment the confirm raises.
func TestPanelMessage_FloorUsesStandingScope(t *testing.T) {
	th := testDarkTheme(t)

	if got := footerScopeSaving(); got <= 0 {
		t.Fatalf("the confirm footer is %d rows shorter than the standing one; §9.2's nested scope must be strictly shorter or the floor would need a row", got)
	}

	for _, dirUnusable := range []bool{false, true} {
		floor := themePanelMinHeight(themePanelKeymap(), dirUnusable)

		if _, ok := themePanelFloor(themePanelPreferredWidth*2, floor, dirUnusable); !ok {
			t.Fatalf("the floor predicate refuses at exactly the floor of %d", floor)
		}

		p := newThemePanelFixture(themePanelFixtureOpts{
			th:          th,
			width:       themePanelMinWidth,
			rows:        themePanelTestRows(8),
			dirUnusable: dirUnusable,
			message:     messageTestConfirm(),
		})
		if _, body := themePanelListSize(p, floor); body < themePanelMinBodyRows {
			t.Errorf("[dir=%v] the list body is %d rows at the floor with a confirm live, want at least %d",
				dirUnusable, body, themePanelMinBodyRows)
		}
		lines := themePanelLines(renderThemePanel(p, floor, th, false))
		if len(lines) != floor {
			t.Fatalf("[dir=%v] the panel rendered %d rows at its floor of %d", dirUnusable, len(lines), floor)
		}
		wantFooter := themePanelLines(renderThemePanelFooter(themePanelConfirmKeymap(), themePanelInnerWidth(themePanelMinWidth), th, false))
		if got := footerRowsOf(renderThemePanel(p, floor, th, false), len(wantFooter)); !equalRows(got, wantFooter) {
			t.Errorf("[dir=%v] the confirm footer at the floor reads %q, want %q — the floor overflowed and the assembly cut it",
				dirUnusable, got, wantFooter)
		}
	}
}

// The panel is blocked under NO_COLOR outright, so this is the defence rather than
// the daily path.
func TestPanelMessage_Colourless(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelPreferredWidth)

	for _, tc := range []struct {
		name    string
		message themePanelMessage
		want    string
	}{
		{"confirm", messageTestConfirm(), messageTestConfirmCopy},
		{"failed commit", messageTestFailed(), messageTestFailedCopy},
	} {
		t.Run(tc.name, func(t *testing.T) {
			block := renderThemePanelMessage(tc.message, inner, false, th, true)
			if frameHasAnyBackgroundSGR(t, block) {
				t.Errorf("the colourless slot activates a background SGR: %q", escSeq(block))
			}
			if frameHasAnyForegroundSGR(t, block) {
				t.Errorf("the colourless slot activates a foreground SGR: %q", escSeq(block))
			}
			if got := strings.TrimRight(ansi.Strip(block), " "); got != tc.want {
				t.Errorf("the colourless slot reads %q, want %q — the glyphs carry the state", got, tc.want)
			}
		})
	}
}
