package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// The §9.1 MESSAGE SLOT gate: the single-slot arbiter's two contenders, their
// §14A copy, their §9.1 tokens, the per-dimension degrade that governs both, and
// the §9.2 nested confirm scope the footer substitutes while the confirm is live.
//
// THE EXCLUSION IS THE SUBJECT, not a precedence rule. The two contenders are the
// slot-from-constant confirm (§9.2) and the failed-commit line (§9.13), and a
// confirm resolves BEFORE any write happens — so they can never be live at once,
// and the tests assert that rather than ranking them.
//
// Colour roles are asserted as theme-resolved SGR runs (like the panel-row and
// footer anatomy tests), so the assertion says WHICH text a token painted rather
// than merely that the token appears somewhere on the row.
//
// No t.Parallel() — the package-level mock convention and the shared canvas
// helpers make parallelism unsafe across this package's tests.

// The two §14A strings, restated here VERBATIM rather than re-derived from the
// production constants. A test that formatted the production format string would
// pass whatever the copy became; these are the byte comparison §14A's pinning
// exists for, double space included.
const (
	messageTestConfirmSlug = "nord"
	messageTestConfirmCopy = "clear constant nord?  y / n"
	messageTestFailedCopy  = "⚠ couldn't save theme"
)

// messageTestConfirm / messageTestFailed are the two contenders as the panel holds
// them. They are FUNCTIONS rather than package-level vars, following this
// package's test-side convention (see themePanelFooterPinnedRows).
func messageTestConfirm() themePanelMessage {
	return themePanelMessage{Kind: themeMessageConfirm, Slug: messageTestConfirmSlug}
}

func messageTestFailed() themePanelMessage {
	return themePanelMessage{Kind: themeMessageCommitFailed}
}

// messageTestPanel is an open panel at the given width carrying one message —
// enough rows behind it that the list body is never the thing under test.
func messageTestPanel(th theme.Theme, width int, message themePanelMessage) themePanel {
	return newThemePanelFixture(themePanelFixtureOpts{
		th:      th,
		width:   width,
		rows:    themePanelTestRows(12),
		message: message,
	})
}

// messageTestVisible renders one message slot and returns its rows with every SGR
// sequence stripped and the pad-to-width trimmed — the text a user reads.
func messageTestVisible(message themePanelMessage, inner int, wrap bool, th theme.Theme) []string {
	block := renderThemePanelMessage(message, inner, wrap, th, false)
	rows := strings.Split(ansi.Strip(block), "\n")
	for i, row := range rows {
		rows[i] = strings.TrimRight(row, " ")
	}
	return rows
}

// TestPanelMessage_ConfirmPinnedCopy: it renders the pinned confirm copy.
//
// §14A pins `clear constant <slug>?  y / n` — TWO spaces before the `y`. The copy
// is a layout constraint as much as a copy choice (§9.1: the panel is 27–34
// columns), so it is byte-compared against the §14A string rather than matched
// loosely, and the message carries NO FREE TEXT: the contender renders its own
// pinned copy, so no call site can paraphrase it.
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

// TestPanelMessage_CommitFailedPinnedCopy: it renders the pinned failed-commit
// copy.
//
// §14A pins `⚠ couldn't save theme` and §9.13 requires it glyph-backed, so the
// `⚠` is part of the string rather than a colour-only signal.
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

// TestPanelMessage_SingleSlotExclusion: it keeps the two contenders mutually
// exclusive.
//
// §9.1's slot is a single-slot arbiter with two contenders "which can never be
// live at once because a confirm resolves before any write happens". The
// exclusion is asserted DIRECTLY rather than resolved with a precedence rule:
// each raise installs a whole value, so the other contender — and any residue it
// left — is gone by construction.
func TestPanelMessage_SingleSlotExclusion(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelPreferredWidth)
	m := newArrowPanelModel(t, arrowValidRows(4), arrowValidRows(4)[0].Slug)
	m.themeKeys = theme.RawKeys{Theme: messageTestConfirmSlug}

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

// requireOnlyContender fails unless the rendered slot carries want and not other —
// the single-slot property stated at the render surface rather than only on the
// value, so a renderer that drew both would be caught too.
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

// TestPanelMessage_UnreservedWhenEmpty: it costs no row when empty.
//
// §9.1: the slot is "not reserved when empty — it appears and the list shrinks by
// one, the same way the main screen's notice band recomputes list height". The
// wrap flag is passed both ways because that is a statement about the slot at
// every height.
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

	// The list body is exactly one row taller with the slot empty — the failed
	// commit is the contender that leaves the standing footer in place, so the row
	// the slot costs is the only difference between the two layouts.
	_, empty := themePanelListSize(messageTestPanel(th, themePanelPreferredWidth, themePanelMessage{}), height)
	_, live := themePanelListSize(messageTestPanel(th, themePanelPreferredWidth, messageTestFailed()), height)
	if live != empty-1 {
		t.Errorf("the list body is %d rows with a message and %d without, want exactly one fewer", live, empty)
	}
}

// TestPanelMessage_WrappedMessageCostsTwoRows: it measures a wrapped message.
//
// §9.1: "at the minimum panel width the slot may wrap to two rows — it is not a
// list delegate, so wrapping costs pagination nothing". It costs the LIST two
// rows rather than one, and the point of the assertion is that the cost is
// MEASURED off the rendered block rather than assumed to be one.
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

	// The whole message survives the wrap, word for word: what §9.1 promises is
	// that nothing is lost, not where the break lands.
	if got, want := chromeWords(strings.Join(rows, " ")), chromeWords(messageTestConfirmCopy); got != want {
		t.Errorf("the wrapped slot reads %q, want the whole confirm %q", got, want)
	}

	// The list body absorbs BOTH rows, not one. The two panels compared carry the
	// SAME message and therefore the same footer scope, so the footer term cancels
	// and the only difference between their budgets is what the slot measured: two
	// rows at the minimum width, one at the preferred. A slot whose cost was assumed
	// to be one row would make the two bodies equal.
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

// footerScopeSaving is the rows the confirm's own two-row footer hands BACK to the
// list body (§9.2's nested scope substituted into the standing four-row footer).
// It is stated once here because every confirm-driven layout assertion has to
// account for it, and stating it as a derivation rather than a literal keeps it
// true if either scope's Core membership changes.
func footerScopeSaving() int {
	return themePanelFooterHeight(themePanelKeymap()) - themePanelFooterHeight(themePanelConfirmKeymap())
}

// TestPanelMessage_TruncatesAtFloorHeight: it truncates at the minimum height.
//
// §9.1: the slot "does cost a row of vertical budget, so at the minimum HEIGHT the
// message is truncated to one line rather than wrapped". §9.8's floor counts
// exactly one message row, so a two-row message there would leave zero list rows
// or overflow the frame — and truncation degrades the message the user is being
// asked to answer rather than the row they are answering ABOUT.
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

	// Rendered at the floor the panel is still exactly its floor in rows, with the
	// slot on the single row above the footer.
	lines := themePanelLines(renderThemePanel(p, floor, th, false))
	if len(lines) != floor {
		t.Fatalf("the panel rendered %d rows at its floor of %d", len(lines), floor)
	}
	slot := strings.TrimRight(lines[floor-themePanelFooterHeight(themePanelConfirmKeymap())-1], " ")
	if !strings.HasPrefix(slot, themePanelContentPrefix()+"clear constant") {
		t.Errorf("the row above the footer is not the message slot: %q", slot)
	}
}

// TestPanelMessage_ConfirmSlugTruncation: it truncates a long constant slug.
//
// §14A: "the slug truncated by §9.5's rule if needed". §9.5's rule is the row
// composition one — the pinned frame is charged first and the flexing element
// takes what is left, floored at three visible characters plus the ellipsis — so
// a long slug can never push `?  y / n` out of the copy.
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
	// The pinned frame survives the truncation at BOTH ends: the phrase that says
	// what is being cleared, and the keys that answer it.
	if !strings.HasPrefix(joined, "clear constant ") {
		t.Errorf("the confirm reads %q, want §14A's leading phrase intact", joined)
	}
	if !strings.HasSuffix(joined, "? y / n") {
		t.Errorf("the confirm reads %q, want §14A's trailing keys intact", joined)
	}

	// The floor is §9.5's: three visible characters plus the ellipsis, so the slug
	// never degrades below being recognisable.
	if got := lipgloss.Width(strings.Fields(joined)[2]); got != themeRowLabelFloor+1 {
		t.Errorf("the truncated slug plus its `?` is %d cells, want §9.5's floor of %d", got, themeRowLabelFloor+1)
	}

	// The row still renders at the minimum width: every line is exactly the panel's
	// width, with nothing spilling past the border.
	p := messageTestPanel(th, themePanelMinWidth, long)
	block := renderThemePanel(p, themePanelMinHeight(themePanelKeymap(), false)+4, th, false)
	for i, line := range strings.Split(block, "\n") {
		if got := lipgloss.Width(line); got != themePanelMinWidth {
			t.Errorf("line %d is %d cells wide at the minimum width, want %d: %q", i, got, themePanelMinWidth, ansi.Strip(line))
		}
	}
}

// TestPanelMessage_ConfirmReadsRawKeys: it names the persisted constant even when
// it failed to load.
//
// The confirm names `m.themeKeys.Theme` — the PERSISTED constant — and never the
// nomination the panel resolved. The two differ exactly where it matters: under
// §8.5's fallback the persisted slug is the one being cleared and the resolved one
// is a built-in the user never chose, so naming the resolution would ask the user
// to confirm clearing a theme that was never set.
func TestPanelMessage_ConfirmReadsRawKeys(t *testing.T) {
	th := testDarkTheme(t)
	inner := themePanelInnerWidth(themePanelPreferredWidth)
	dir := t.TempDir()

	// `ghost` resolves to nothing — no file, no built-in — so §8.5's fallback puts
	// the shipped dark default on screen while `ghost` stays the persisted constant.
	// The cursor sitting on the fallback's row is that state made observable (§9.2).
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

// TestPanelMessage_ConfirmTokens: it uses text.secondary with no band.
//
// §9.1's table: "message slot — confirm: `text.secondary`, no band". The role is
// asserted as the run that painted the text, and the absence of a band as the
// absence of any background other than the panel's own canvas.
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

// TestPanelMessage_CommitFailedTokens: it uses accent.attention with no band.
//
// §9.1's table: "message slot — failed commit: `⚠` and text in `accent.attention`,
// NO `bg.attention` band — the warning band is a full-width main-screen flash
// treatment and would read as heavy inside a 27–34 column panel". The glyph and
// the text share ONE run, exactly as the pinned directory row's do.
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

// requireNoBand fails unless the only background the slot paints is the panel's
// own canvas — §9.1's "no band" for both contenders, with bg.attention named
// explicitly because it is the one the failed-commit line would plausibly reach
// for.
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

// TestPanelFooter_ConfirmScopeSubstitution: it substitutes the confirm footer.
//
// §9.2: "the panel footer switches to the confirm's own keys while it is live —
// `y confirm` / `n cancel`". The standing footer advertises four keys of which
// NONE would act during a confirm, and §14.3 is firm that advertising a key that
// will not act is the dead end a proactive block exists to prevent.
//
// SUBSTITUTED, NOT FORKED: the rendered rows are compared against the SAME
// renderer handed the confirm scope, so a second footer implementation would show
// up as a difference rather than pass on a coincidence.
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

	// The layout ABSORBS the difference: themePanelFooterHeight reports the
	// substituted footer's height, so the two rows it no longer spends land in the
	// list body rather than nowhere.
	_, standing := themePanelListSize(messageTestPanel(th, themePanelPreferredWidth, messageTestFailed()), height)
	_, confirming := themePanelListSize(p, height)
	if want := standing + footerScopeSaving(); confirming != want {
		t.Errorf("the list body is %d rows while the confirm is live, want %d", confirming, want)
	}
}

// TestPanelFooter_RevertsAfterConfirm: it restores the standing footer on resolve.
//
// §9.2: the footer "switches back when it resolves". Resolving is clearing the
// message — nothing else in the panel changes — and the restored footer is
// compared byte for byte against the standing render, through the SAME renderer.
func TestPanelFooter_RevertsAfterConfirm(t *testing.T) {
	th := testDarkTheme(t)
	const height = 18
	inner := themePanelInnerWidth(themePanelPreferredWidth)
	m := newArrowPanelModel(t, arrowValidRows(6), arrowValidRows(6)[0].Slug)
	m.themeKeys = theme.RawKeys{Theme: messageTestConfirmSlug}
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

// footerRowsOf returns the last n rows of a rendered panel block with each row's
// border-and-gutter prefix removed, so it can be compared against a footer
// rendered on its own.
func footerRowsOf(block string, n int) []string {
	lines := themePanelLines(block)
	rows := make([]string, 0, n)
	for _, row := range lines[len(lines)-n:] {
		rows = append(rows, strings.TrimPrefix(row, themePanelContentPrefix()))
	}
	return rows
}

// equalRows reports whether two rendered row slices match exactly.
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

// TestPanelMessage_FloorUsesStandingScope: it leaves the height floor unchanged.
//
// §9.8's floor is computed from the STANDING scope and gains no row for the
// confirm. The confirm's footer is strictly SHORTER, so a panel that clears the
// floor with the standing footer has room to spare the moment the confirm raises
// — which is the non-obvious half of "the slot's height feeds the floor arithmetic
// unchanged".
func TestPanelMessage_FloorUsesStandingScope(t *testing.T) {
	th := testDarkTheme(t)

	if got := footerScopeSaving(); got <= 0 {
		t.Fatalf("the confirm footer is %d rows shorter than the standing one; §9.2's nested scope must be strictly shorter or the floor would need a row", got)
	}

	for _, dirUnusable := range []bool{false, true} {
		floor := themePanelMinHeight(themePanelKeymap(), dirUnusable)

		// The floor is a function of the STANDING scope and the directory alone: the
		// panel's own predicate answers identically whatever the slot holds.
		if _, ok := themePanelFloor(themePanelPreferredWidth*2, floor, dirUnusable); !ok {
			t.Fatalf("the floor predicate refuses at exactly the floor of %d", floor)
		}

		// And the panel renders coherently at that floor with the confirm live: the
		// whole confirm footer, the message row, and at least the floor's one list row.
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

// TestPanelMessage_Colourless: it drops hue under colourless.
//
// §2.5's NO_COLOR carve-out: no canvas, no hue, state carried by the glyphs. §9.10
// blocks the panel under NO_COLOR outright, so this is the defence rather than the
// daily path — the message slot must not be the one surface that reintroduces a
// colour the carve-out removed everywhere else.
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
