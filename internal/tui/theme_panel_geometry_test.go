package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// §9.8's geometry: the panel takes its preferred width on a normal terminal,
// SHRINKS toward its minimum as the terminal narrows, refuses below the floor,
// degrades IN PLACE on a resize, and force-closes onto the resolved persisted
// state — with §14A's pinned copy — when a resize crosses the floor.
//
// The doctrine is the point. MV §2.7's degrade-never-break governs a space
// SHORTAGE, which is what a narrow terminal is; the multi-select precedent (a
// proactive block at entry) governs a capability ABSENCE and deliberately does not
// transfer. Applying the wrong one either opens a broken frame on a 34-column
// terminal or refuses a panel that would have fitted.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// TestPanelGeometry_WidthLadder: it stages the width between preferred and
// minimum.
//
// §9.8 leaves the exact thresholds to implementation, as §2.7 already does for its
// own steps, so the table pins them: ≥60 content columns take the preferred width,
// 48–59 shrink through the band, and 24–47 sit on the minimum. The half-width cap
// is what keeps the previewed page visible while the terminal is wide enough to
// afford it.
//
// MONOTONICITY IS THE PROPERTY, not the individual rows: a ladder that jumped back
// up as the terminal narrowed would satisfy every point below and still not be a
// staged degradation.
func TestPanelGeometry_WidthLadder(t *testing.T) {
	for _, tc := range []struct {
		name     string
		contentW int
		want     int
	}{
		{name: "far wider than the cap", contentW: 200, want: themePanelPreferredWidth},
		{name: "exactly twice the preferred width", contentW: 68, want: themePanelPreferredWidth},
		{name: "one column into the degraded band", contentW: 67, want: 33},
		{name: "mid band", contentW: 60, want: 30},
		{name: "the bottom of the degraded band", contentW: 54, want: themePanelMinWidth},
		{name: "below the band, above the floor", contentW: 40, want: themePanelMinWidth},
		{name: "exactly at the floor", contentW: themePanelMinWidth, want: themePanelMinWidth},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := themePanelWidthFor(tc.contentW)
			if got != tc.want {
				t.Errorf("themePanelWidthFor(%d) = %d, want %d", tc.contentW, got, tc.want)
			}
			if !ok {
				t.Errorf("themePanelWidthFor(%d) refused; the panel renders at every width down to %d", tc.contentW, themePanelMinWidth)
			}
		})
	}

	t.Run("it never widens as the terminal narrows", func(t *testing.T) {
		prev := themePanelPreferredWidth
		sawStrictlyBetween := false
		for contentW := 200; contentW >= themePanelMinWidth; contentW-- {
			got, ok := themePanelWidthFor(contentW)
			if !ok {
				t.Fatalf("themePanelWidthFor(%d) refused above the floor", contentW)
			}
			if got > prev {
				t.Fatalf("narrowing the content region to %d WIDENED the panel from %d to %d — the ladder is not staged", contentW, prev, got)
			}
			if got < themePanelMinWidth || got > themePanelPreferredWidth {
				t.Fatalf("themePanelWidthFor(%d) = %d, outside the [%d, %d] ladder", contentW, got, themePanelMinWidth, themePanelPreferredWidth)
			}
			if got > themePanelMinWidth && got < themePanelPreferredWidth {
				sawStrictlyBetween = true
			}
			prev = got
		}
		if prev != themePanelMinWidth {
			t.Errorf("the bottom of the ladder is %d, want the minimum %d", prev, themePanelMinWidth)
		}
		if !sawStrictlyBetween {
			t.Error("no content width produced a panel strictly between the minimum and the preferred width — the ladder is a switch, not a staged shrink")
		}
	})
}

// TestPanelGeometry_WidthFloor: it refuses below the minimum width.
//
// §9.8 refuses "only when even the minimum panel cannot render", and the floor is
// `contentW >= themePanelMinWidth` and nothing more — the spec does not require any
// of the previewed page to stay visible, so at exactly the floor the panel covers
// nearly the whole content region. That is accepted rather than fixed with an
// invented "keep N columns of page" rule.
//
// The WIDTH IS STILL CLAMPED on the refusing path, because both callers of the
// ladder take the value and ignore `ok` — the floor predicate has already refused
// by the time either reaches it — so an unclamped return would be the one shape
// that renders a sub-minimum panel.
func TestPanelGeometry_WidthFloor(t *testing.T) {
	for contentW := themePanelMinWidth - 1; contentW >= 0; contentW-- {
		got, ok := themePanelWidthFor(contentW)
		if ok {
			t.Errorf("themePanelWidthFor(%d) accepted a content region below the %d-column minimum", contentW, themePanelMinWidth)
		}
		if got != themePanelMinWidth {
			t.Errorf("themePanelWidthFor(%d) = %d on the refusing path, want the defensively clamped %d", contentW, got, themePanelMinWidth)
		}
	}
}

// TestPanelGeometry_HeightFloorArithmetic: it computes the height floor from the
// measured footer.
//
// §9.8's floor is header + footer + one row + one message row. The MESSAGE ROW is
// counted although §9.1 does not reserve the slot when empty, because both of its
// contenders are non-suppressible — a floor computed without it puts the panel
// exactly one row short at the moment a message appears, asking "clear constant
// `<slug>`?" about a row that has just been pushed off screen.
//
// THE FOOTER IS MEASURED, NEVER A LITERAL. The second case is what proves it:
// Phase 9's confirm scope substitutes a SHORTER footer (§9.2), and a floor carrying
// a hardcoded four would not move with it.
func TestPanelGeometry_HeightFloorArithmetic(t *testing.T) {
	entries := themePanelKeymap()
	footer := themePanelFooterHeight(entries)
	if footer != lipgloss.Height(renderThemePanelFooter(entries, 0, testDarkTheme(t), false)) {
		t.Fatalf("fixture: themePanelFooterHeight(%d) is not the rendered footer's height", footer)
	}

	const listRow, messageRow = 1, 1
	header := themePanelHeaderRows()
	want := header + footer + listRow + messageRow
	if got := themePanelMinHeight(entries, false); got != want {
		t.Errorf("themePanelMinHeight = %d, want header(%d) + footer(%d) + %d list row + %d message row = %d",
			got, header, footer, listRow, messageRow, want)
	}

	// The confirm scope's shorter footer moves the floor with it — the assertion a
	// literal cannot satisfy.
	shorter := []keymapEntry{
		{Key: "y", Action: "confirm", Core: true},
		{Key: "n", Action: "cancel", Core: true},
	}
	shorterFooter := themePanelFooterHeight(shorter)
	if shorterFooter >= footer {
		t.Fatalf("fixture: the substituted footer is %d rows, not shorter than the panel scope's %d", shorterFooter, footer)
	}
	if got, wantShort := themePanelMinHeight(shorter, false), header+shorterFooter+listRow+messageRow; got != wantShort {
		t.Errorf("themePanelMinHeight under a %d-row footer = %d, want %d — the floor reads the MEASURED footer height", shorterFooter, got, wantShort)
	}
}

// TestPanelGeometry_ChromeRowsIsSharedByBothArithmetics: it charges the panel's
// chrome from one sum.
//
// The height floor and the list body's budget are the SAME question asked at two
// heights — header + directory row + message rows + footer — and the floor is only
// meaningful while it agrees with what renders: a floor that admits a terminal the
// body arithmetic then overflows is precisely the broken frame the floor exists to
// refuse. Each component is single-sourced already; the SET of them is what this
// pins, so a component added to the panel's chrome cannot reach one arithmetic and
// miss the other.
//
// Each side is asserted against the shared sum plus the one term it alone owns: the
// floor adds the list row it guarantees, the body takes the remainder.
func TestPanelGeometry_ChromeRowsIsSharedByBothArithmetics(t *testing.T) {
	entries := themePanelKeymap()

	t.Run("the floor is the chrome sum plus its guaranteed list row", func(t *testing.T) {
		for _, dirUnusable := range []bool{false, true} {
			want := themePanelChromeRows(dirUnusable, themePanelFloorMessageRows, entries) + themePanelMinBodyRows
			if got := themePanelMinHeight(entries, dirUnusable); got != want {
				t.Errorf("themePanelMinHeight(dirUnusable=%v) = %d, want the chrome sum plus its %d list row = %d",
					dirUnusable, got, themePanelMinBodyRows, want)
			}
		}
	})

	t.Run("the body is the render height less the chrome sum", func(t *testing.T) {
		for _, tc := range []struct {
			name        string
			dirUnusable bool
			message     themePanelMessage
		}{
			{name: "an empty slot"},
			{name: "an unusable directory", dirUnusable: true},
			{name: "the confirm's shorter footer", message: geometryMessage()},
		} {
			t.Run(tc.name, func(t *testing.T) {
				p := newThemePanelFixture(themePanelFixtureOpts{
					th:          testDarkTheme(t),
					width:       themePanelPreferredWidth,
					rows:        themePanelTestRows(12),
					dirUnusable: tc.dirUnusable,
					message:     tc.message,
				})

				inner := themePanelInnerWidth(p.width)
				messageRows := themePanelMessageHeight(p.message, inner, themePanelMessageWraps(p, geometryContentH))
				want := geometryContentH - themePanelChromeRows(tc.dirUnusable, messageRows, themePanelFooterScope(p.message))
				if want <= themePanelMinBodyRows {
					t.Fatalf("fixture: a %d-row content region leaves a %d-row body, at or below the %d-row floor — the remainder is clamped, so the assertion proves nothing",
						geometryContentH, want, themePanelMinBodyRows)
				}

				if _, got := themePanelListSize(p, geometryContentH); got != want {
					t.Errorf("the list body at %d rows = %d, want the height less the chrome sum = %d",
						geometryContentH, got, want)
				}
			})
		}
	})
}

// TestPanelGeometry_DirRowRaisesTheFloor: it adds a row to the floor for an
// unreadable directory.
//
// §9.5: the pinned `⚠ dir unreadable` row "costs a viewport row while present, so
// §9.8's minimum-height floor gains it conditionally". CONDITIONALLY is the word
// that matters in both directions: counting it always would refuse terminals that
// would have rendered a good panel, and counting it never would let the warning
// consume the single list row while §9.5 simultaneously requires built-in and
// persisted rows to render beneath it.
func TestPanelGeometry_DirRowRaisesTheFloor(t *testing.T) {
	entries := themePanelKeymap()
	usable := themePanelMinHeight(entries, false)
	unusable := themePanelMinHeight(entries, true)

	if got := unusable - usable; got != 1 {
		t.Errorf("an unusable themes directory raises the floor by %d row(s), want exactly 1", got)
	}
	if got := themePanelDirRowHeight(true); got != 1 {
		t.Fatalf("fixture: the directory row measures %d rows, so the floor delta says nothing", got)
	}
	if got := themePanelDirRowHeight(false); got != 0 {
		t.Fatalf("fixture: a usable directory measures %d rows, want 0", got)
	}
}

// TestPanelGeometry_FloorReportsWidthFirst: it reports width before height.
//
// §9.7's entry condition and §9.8's resize condition are the same predicate, so it
// has to name WHICH dimension failed — §14A pins a string per dimension, and two
// call sites each deciding for themselves is how a terminal comes to pass one check
// and fail the other.
//
// WIDTH IS CHECKED FIRST, so a terminal failing both reports narrow. That is an
// ordering decision rather than an arbitrary one: it is the dimension the user can
// act on with the same gesture that broke it.
func TestPanelGeometry_FloorReportsWidthFirst(t *testing.T) {
	entries := themePanelKeymap()
	tallEnough := themePanelMinHeight(entries, false)

	for _, tc := range []struct {
		name        string
		contentW    int
		contentH    int
		dirUnusable bool
		wantDim     themePanelDim
		wantOK      bool
	}{
		{name: "both dimensions clear", contentW: 80, contentH: tallEnough, wantDim: dimNone, wantOK: true},
		{name: "exactly at both floors", contentW: themePanelMinWidth, contentH: tallEnough, wantDim: dimNone, wantOK: true},
		{name: "too narrow only", contentW: themePanelMinWidth - 1, contentH: tallEnough, wantDim: dimWidth},
		{name: "too short only", contentW: 80, contentH: tallEnough - 1, wantDim: dimHeight},
		{name: "both fail", contentW: themePanelMinWidth - 1, contentH: 0, wantDim: dimWidth},
		{name: "an unusable directory takes the last row", contentW: 80, contentH: tallEnough, dirUnusable: true, wantDim: dimHeight},
		{name: "an unusable directory with its row to spare", contentW: 80, contentH: tallEnough + 1, dirUnusable: true, wantDim: dimNone, wantOK: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dim, ok := themePanelFloor(tc.contentW, tc.contentH, tc.dirUnusable)
			if ok != tc.wantOK {
				t.Fatalf("themePanelFloor(%d, %d, %v) ok = %v, want %v", tc.contentW, tc.contentH, tc.dirUnusable, ok, tc.wantOK)
			}
			if dim != tc.wantDim {
				t.Errorf("themePanelFloor(%d, %d, %v) reported %v, want %v", tc.contentW, tc.contentH, tc.dirUnusable, dim, tc.wantDim)
			}
		})
	}

	// ONE PREDICATE, TWO CALLERS. §9.7's entry condition and §9.8's resize condition
	// are the same question, and a terminal that passes one check and fails the other
	// is exactly what two independent derivations produce. The resize path is the
	// caller that exists today, and its whole outcome — stays open, or closes with
	// which pinned copy — is asserted to be a function of nothing but this predicate's
	// answer; task 8-13's gate consumes the same one rather than re-deriving it.
	t.Run("the resize path decides by this predicate alone", func(t *testing.T) {
		for _, region := range [][2]int{
			{geometryWideW, geometryContentH},
			{themePanelMinWidth, tallEnough},
			{themePanelMinWidth - 1, tallEnough},
			{geometryWideW, tallEnough - 1},
			{themePanelMinWidth - 1, 1},
			{60, tallEnough},
		} {
			contentW, contentH := region[0], region[1]
			t.Run(fmt.Sprintf("%dx%d", contentW, contentH), func(t *testing.T) {
				m := resizeForTest(t, newGeometryPanelModel(t, geometryWideW, geometryContentH), contentW, contentH)
				if got := m.contentWidth(); got != contentW {
					t.Fatalf("fixture: the resized content region is %d columns, want %d", got, contentW)
				}
				if got := m.contentHeight(); got != contentH {
					t.Fatalf("fixture: the resized content region is %d rows, want %d", got, contentH)
				}

				dim, ok := themePanelFloor(contentW, contentH, false)
				if m.themePanel.open != ok {
					t.Fatalf("the panel is open=%v after the resize, want %v — the predicate said (%v, ok=%v)", m.themePanel.open, ok, dim, ok)
				}
				if ok {
					if got := m.flashText; got != "" {
						t.Errorf("a resize above the floor raised %q, want no flash", got)
					}
					return
				}
				if got, want := m.flashText, themePanelForcedCloseFlash(dim); got != want {
					t.Errorf("the forced close raised %q, want the %v copy %q", got, dim, want)
				}
			})
		}
	})
}

// The three content-region widths every geometry fixture is sized against, and the
// panel width the ladder gives each. They are stated as CONTENT widths (what the
// ladder is a function of) and converted to terminal dimensions by geometryTerm, so
// a change to the §3 gutter moves the fixtures rather than silently moving the band
// they are meant to sit in.
//
// geometryDegradedW is chosen so the panel lands STRICTLY between the two ends —
// the only band in which "the open used the ladder" and "the open used the preferred
// width" are different answers.
const (
	geometryWideW         = 96
	geometryDegradedW     = 56
	geometryDegradedPanel = 28
	geometryContentH      = 26
)

// geometryLabel is 24 cells — inside the label budget a BADGED row has at the
// preferred width (inner 32, less the 2-cell cursor column and the badge's `●` plus
// its gap = 28) and beyond the 22 it has at geometryDegradedPanel (inner 26).
//
// That is the whole point of the value: the same row renders untruncated at the
// preferred width and truncated with `…` in the degraded band, so a delegate still
// composing against the pre-resize budget is visible as a MISSING ellipsis rather
// than as an off-by-one nobody reads.
const geometryLabel = "aurora-midnight-drifting"

// geometryTerm converts a content region to the terminal dimensions that produce
// it, folding in the §3 gutter exactly as contentWidth / contentHeight do.
func geometryTerm(contentW, contentH int) (termW, termH int) {
	return contentW + 2*Hinset, contentH + 2*Vinset
}

// geometryRows are the panel's union for a geometry fixture: the long-labelled row
// the cursor lands on (and therefore the one carrying `●`), plus enough plain rows
// that the list has a body to render.
func geometryRows() []theme.Row {
	rows := []theme.Row{arrowValidRow(geometryLabel, 0)}
	for i := 1; i < 5; i++ {
		rows = append(rows, arrowValidRow(arrowSlug(i), i))
	}
	return rows
}

// newGeometryPanelModel builds a renderable Sessions model at the given CONTENT
// region and opens the panel through the production `t` keypress.
func newGeometryPanelModel(t *testing.T, contentW, contentH int) Model {
	t.Helper()
	m := Build(newArrowPanelDeps(t, geometryRows(), geometryLabel))
	return openPanelForTest(t, m, contentW, contentH)
}

// resizeForTest drives a tea.WindowSizeMsg for the given CONTENT region through the
// live Update, so every resize assertion covers the production path.
func resizeForTest(t *testing.T, m Model, contentW, contentH int) Model {
	t.Helper()
	m, _ = resizeForTestCmd(t, m, contentW, contentH)
	return m
}

// resizeForTestCmd is resizeForTest's cmd-returning twin, for the assertions that
// are ABOUT the command rather than about the model the resize leaves behind.
//
// A resize is handled in a PRE-STEP of Update, before the message reaches the arms
// that return: a command raised there reaches the runtime only by being folded onto
// whichever arm fires. Discarding it — which resizeForTest does, and which is the
// right convenience everywhere the command is beside the point — cannot tell a
// propagated command from a swallowed one.
func resizeForTestCmd(t *testing.T, m Model, contentW, contentH int) (Model, tea.Cmd) {
	t.Helper()
	termW, termH := geometryTerm(contentW, contentH)
	updated, cmd := m.Update(tea.WindowSizeMsg{Width: termW, Height: termH})
	return updated.(Model), cmd
}

// requireRenderedPanelWidth asserts the panel is want cells wide in the model, in
// its own rendered block, AND in the composed frame at the content region's right
// edge — so the width is a property of what reaches the screen rather than of a
// field.
func requireRenderedPanelWidth(t *testing.T, m Model, want int) {
	t.Helper()
	if got := m.themePanel.width; got != want {
		t.Fatalf("the panel's width is %d, want %d", got, want)
	}

	contentW, contentH := m.contentWidth(), m.contentHeight()
	panelLines := strings.Split(ansi.Strip(renderThemePanel(m.themePanel, contentH, m.themeState.active, m.colourless)), "\n")
	if len(panelLines) != contentH {
		t.Fatalf("the panel rendered %d rows, want the content height %d", len(panelLines), contentH)
	}
	for i, line := range panelLines {
		if got := lipgloss.Width(line); got != want {
			t.Errorf("panel row %d is %d cells, want %d: %q", i, got, want, line)
		}
	}

	frame := strings.Split(ansi.Strip(m.View().Content), "\n")
	_, _, topPad, _ := gutterPadding(m.termWidth, m.termHeight, contentW, contentH)
	left := (m.termWidth-contentW)/2 + contentW - want
	for j, wantLine := range panelLines {
		row := []rune(frame[topPad+j])
		if len(row) != m.termWidth {
			t.Fatalf("frame row %d is %d cells, want %d", topPad+j, len(row), m.termWidth)
		}
		if got := string(row[left : left+want]); got != wantLine {
			t.Errorf("frame row %d under the panel = %q, want the panel row %q", topPad+j, got, wantLine)
		}
	}
}

// TestPanelGeometry_OpenUsesTheWidthLadder: it opens at the ladder width.
//
// The ladder applies at OPEN, not only on a resize. A panel that always opened at
// the preferred width and narrowed only if the user happened to resize afterwards
// would contradict §9.8's staged shrink — and would make the `theme-panel-narrow`
// fixture uncapturable, since a fixture opens through `captureKeys` and never
// resizes, so the degraded band would only ever be entered by a gesture no capture
// makes.
//
// NO tea.WindowSizeMsg IS SENT. The fixture assigns the dimensions directly, so the
// very first rendered frame is the one asserted.
func TestPanelGeometry_OpenUsesTheWidthLadder(t *testing.T) {
	t.Run("a wide terminal takes the preferred width", func(t *testing.T) {
		m := newGeometryPanelModel(t, geometryWideW, geometryContentH)
		requireRenderedPanelWidth(t, m, themePanelPreferredWidth)
	})

	t.Run("a terminal in the degraded band opens already narrowed", func(t *testing.T) {
		want, ok := themePanelWidthFor(geometryDegradedW)
		if !ok || want != geometryDegradedPanel {
			t.Fatalf("fixture: a %d-column content region gives a %d-cell panel (ok=%v), want %d", geometryDegradedW, want, ok, geometryDegradedPanel)
		}
		if want >= themePanelPreferredWidth || want <= themePanelMinWidth {
			t.Fatalf("fixture: %d is not strictly between the ladder's ends, so the assertion below cannot fail", want)
		}

		m := newGeometryPanelModel(t, geometryDegradedW, geometryContentH)
		requireRenderedPanelWidth(t, m, want)
	})
}

// TestPanelGeometry_OpenAndResizeWidthsAgree: it agrees between open and resize.
//
// ONE FUNCTION, TWO CALLERS. A panel opened at a given content width and a panel
// resized to the same one must be the same panel — two ladders that agree today are
// two ladders that can drift, and the drift is invisible until a user resizes into
// a band a fixture only ever entered at open.
//
// The LIST SIZE is asserted alongside the width because it is the half a width
// comparison cannot see: the resize re-runs the same body arithmetic the open ran,
// so the two panels paginate identically as well as measuring identically.
func TestPanelGeometry_OpenAndResizeWidthsAgree(t *testing.T) {
	for _, contentW := range []int{geometryWideW, 60, 59, geometryDegradedW, 48, 40, themePanelMinWidth} {
		t.Run(fmt.Sprintf("contentW=%d", contentW), func(t *testing.T) {
			want, ok := themePanelWidthFor(contentW)
			if !ok {
				t.Fatalf("fixture: a %d-column content region is below the floor", contentW)
			}

			opened := newGeometryPanelModel(t, contentW, geometryContentH)
			resized := resizeForTest(t, newGeometryPanelModel(t, geometryWideW, geometryContentH), contentW, geometryContentH)

			if got := opened.themePanel.width; got != want {
				t.Errorf("the panel OPENED at %d cells, want the ladder's %d", got, want)
			}
			if got := resized.themePanel.width; got != want {
				t.Errorf("the panel RESIZED to %d cells, want the ladder's %d", got, want)
			}

			openW, openH := opened.themePanel.list.Width(), opened.themePanel.list.Height()
			resizeW, resizeH := resized.themePanel.list.Width(), resized.themePanel.list.Height()
			if openW != resizeW || openH != resizeH {
				t.Errorf("the opened panel's list is %dx%d and the resized one's is %dx%d; one ladder, one body arithmetic", openW, openH, resizeW, resizeH)
			}
		})
	}
}

// requirePanelListMatchesTheRenderCopy asserts the MODEL's list is sized exactly as
// the per-frame copy renderThemePanel sizes its own — the two must derive the same
// page from the same arithmetic.
//
// It is the assertion a width comparison structurally cannot make. `bubbles/list`
// derives Paginator.PerPage from the height it is handed, so a model list left at a
// stale size pages by the OLD amount while the drawn page is the new one — §9.2's
// `Ctrl+↑`/`Ctrl+↓` would move the cursor a different distance than the screen
// scrolls, which is the one paging failure no rendered frame reveals.
func requirePanelListMatchesTheRenderCopy(t *testing.T, m Model) {
	t.Helper()
	wantW, wantH := themePanelListSize(m.themePanel, m.contentHeight())
	gotW, gotH := m.themePanel.list.Width(), m.themePanel.list.Height()
	if gotW != wantW || gotH != wantH {
		t.Errorf("the model's panel list is sized %dx%d, want the render copy's %dx%d", gotW, gotH, wantW, wantH)
	}

	if got, want := m.themePanel.list.Paginator.PerPage, themePanelDrawnPerPage(m); got != want {
		t.Errorf("the model's panel list paginates %d row(s) per page and the drawn page holds %d — Ctrl+↑/↓ moves by the stale amount", got, want)
	}
}

// themePanelDrawnPerPage is the page the per-frame render copy derives at the
// model's current content height — the page the user actually sees, as against the
// one the model's own list would page by.
func themePanelDrawnPerPage(m Model) int {
	copied := m.themePanel
	copied.list.SetSize(themePanelListSize(copied, m.contentHeight()))
	return copied.list.Paginator.PerPage
}

// TestPanelGeometry_ResizeDegradesInPlace: it degrades in place on a resize.
//
// §9.8's resize condition: while the terminal stays above the render floor the
// panel STAYS OPEN and re-lays itself out — it does not close, and it does not keep
// the width it opened at.
//
// The HEIGHT case is the one the width case cannot cover: a taller terminal changes
// no panel width at all, so a resize path that re-ran only the ladder would pass the
// width assertions and leave the list paging by the pre-resize page.
func TestPanelGeometry_ResizeDegradesInPlace(t *testing.T) {
	t.Run("narrowing shrinks the panel and keeps it open", func(t *testing.T) {
		m := newGeometryPanelModel(t, geometryWideW, geometryContentH)
		requireRenderedPanelWidth(t, m, themePanelPreferredWidth)

		m = resizeForTest(t, m, geometryDegradedW, geometryContentH)

		if !m.themePanel.open {
			t.Fatal("a resize above the floor closed the panel; §9.8 degrades in place")
		}
		requireRenderedPanelWidth(t, m, geometryDegradedPanel)
		requirePanelListMatchesTheRenderCopy(t, m)
	})

	t.Run("widening restores the preferred width", func(t *testing.T) {
		m := newGeometryPanelModel(t, geometryDegradedW, geometryContentH)
		requireRenderedPanelWidth(t, m, geometryDegradedPanel)

		m = resizeForTest(t, m, geometryWideW, geometryContentH)

		if !m.themePanel.open {
			t.Fatal("a resize above the floor closed the panel")
		}
		requireRenderedPanelWidth(t, m, themePanelPreferredWidth)
		requirePanelListMatchesTheRenderCopy(t, m)
	})

	t.Run("a taller terminal re-derives the panel's page", func(t *testing.T) {
		// Exactly §9.8's floor: the shortest content region the panel opens at, and so
		// the smallest page it can be given.
		shortH := themePanelMinHeight(themePanelKeymap(), false)
		m := newGeometryPanelModel(t, geometryWideW, shortH)
		short := m.themePanel.list.Paginator.PerPage
		requirePanelListMatchesTheRenderCopy(t, m)

		m = resizeForTest(t, m, geometryWideW, geometryContentH)

		if !m.themePanel.open {
			t.Fatal("a taller terminal closed the panel")
		}
		if drawn := themePanelDrawnPerPage(m); drawn == short {
			t.Fatalf("fixture: the drawn page is %d rows at both %d and %d content rows, so the assertion below proves nothing", drawn, shortH, geometryContentH)
		}
		requirePanelListMatchesTheRenderCopy(t, m)
		if got := lipgloss.Height(renderThemePanel(m.themePanel, m.contentHeight(), m.themeState.active, m.colourless)); got != geometryContentH {
			t.Errorf("the panel rendered %d rows, want the content height %d", got, geometryContentH)
		}
	})
}

// themePanelBodyRow returns the panel's list row at the given offset into the body,
// with every SGR stripped — the row the user reads, border cell included.
func themePanelBodyRow(t *testing.T, m Model, offset int) string {
	t.Helper()
	lines := themePanelLines(renderThemePanel(m.themePanel, m.contentHeight(), m.themeState.active, m.colourless))
	at := themePanelHeaderRows() + themePanelDirRowHeight(m.themePanel.union.DirUnusable) + offset
	if at >= len(lines) {
		t.Fatalf("the panel rendered %d rows, so body row %d does not exist", len(lines), offset)
	}
	return lines[at]
}

// TestPanelGeometry_ResizeRepointsTheDelegate: it re-points the delegate on a
// resize.
//
// THE DELEGATE IS THE HALF `SetSize` CANNOT REACH. The panel's row delegate holds
// its composition budget as a FIELD (unlike SessionDelegate, which reads the width
// off the list it is rendering into), and the only other caller of the single
// construction point runs on a THEME SWAP — a tea.WindowSizeMsg calls no ApplyTheme.
// So a resize that only re-sized the list would leave every row composing against
// the pre-resize budget, which is exactly the disagreement §11.2's single
// construction point exists to prevent and the failure it names: "invisible until a
// resize during a live preview".
//
// NO ARROW IS PRESSED between the resize and the assertion, deliberately — an arrow
// would re-point the delegate through the restyle path and hide the defect.
func TestPanelGeometry_ResizeRepointsTheDelegate(t *testing.T) {
	m := newGeometryPanelModel(t, geometryWideW, geometryContentH)
	requireCursorOn(t, m, geometryLabel)
	if got := themePanelRowFor(t, m, geometryLabel).Badge; got != theme.BadgeConstant {
		t.Fatalf("fixture: the cursor row carries badge %v, want the constant `●` the right edge competes for", got)
	}

	before := themePanelBodyRow(t, m, 0)
	if !strings.Contains(before, geometryLabel) {
		t.Fatalf("fixture: the %d-cell label is not rendered in full at the preferred width: %q", lipgloss.Width(geometryLabel), before)
	}
	if strings.Contains(before, themeRowEllipsis) {
		t.Fatalf("fixture: the label is already truncated at the preferred width, so a truncation below proves nothing: %q", before)
	}

	m = resizeForTest(t, m, geometryDegradedW, geometryContentH)

	after := themePanelBodyRow(t, m, 0)
	if !strings.Contains(after, themeRowEllipsis) {
		t.Errorf("after narrowing with no arrow the cursor row = %q, want it truncated with %q — the delegate is still composing against the pre-resize budget", after, themeRowEllipsis)
	}
	if strings.Contains(after, geometryLabel) {
		t.Errorf("the cursor row still carries the full %d-cell label at a %d-cell panel: %q", lipgloss.Width(geometryLabel), m.themePanel.width, after)
	}
	if !strings.HasSuffix(after, themePanelBadgeText(theme.BadgeConstant)) {
		t.Errorf("the cursor row = %q, want it to end in the %q badge — the badge outranks the label and stays inside the inner edge", after, themePanelBadgeText(theme.BadgeConstant))
	}
	for i, line := range themePanelLines(renderThemePanel(m.themePanel, m.contentHeight(), m.themeState.active, m.colourless)) {
		if got := lipgloss.Width(line); got != m.themePanel.width {
			t.Errorf("panel row %d is %d cells, want the panel's %d: %q", i, got, m.themePanel.width, line)
		}
	}
}

// TestPanelGeometry_ResizeDoesNotReflowTheBase: it never re-lays-out the page
// beneath.
//
// §9.1 composites the panel over a page composed at the UNREDUCED content width, and
// a resize must keep that true — reflowing to the reduced width would produce a
// cleaner edge and reflow the surface being previewed, which is the opposite of what
// a preview wants.
//
// The control is the same resize with NO panel open, so "the page laid out at the
// unreduced width" is asserted against production's own answer rather than against
// an arithmetic restatement of it. The cut-mid-label half is the visible
// consequence: the covered footer is CUT wherever the border falls, never
// re-laid-out to fit beside the panel.
func TestPanelGeometry_ResizeDoesNotReflowTheBase(t *testing.T) {
	m := newGeometryPanelModel(t, geometryWideW, geometryContentH)
	m = resizeForTest(t, m, geometryDegradedW, geometryContentH)
	if !m.themePanel.open {
		t.Fatal("fixture: the resize closed the panel")
	}

	control := closeThemePanelForTest(t, newGeometryPanelModel(t, geometryWideW, geometryContentH))
	control = resizeForTest(t, control, geometryDegradedW, geometryContentH)
	if control.themePanel.open {
		t.Fatal("fixture: the control still has a panel open")
	}

	wantW, wantH := control.SessionListSize()
	if gotW, gotH := m.SessionListSize(); gotW != wantW || gotH != wantH {
		t.Errorf("with the panel open the sessions list resized to %dx%d, want the unreduced %dx%d the same resize gives with no panel", gotW, gotH, wantW, wantH)
	}
	if wantW <= m.themePanel.width {
		t.Fatalf("fixture: the unreduced list width %d is not wider than the panel's %d, so a reduction would be invisible", wantW, m.themePanel.width)
	}

	contentW := m.contentWidth()
	footer := strings.Split(ansi.Strip(renderSessionsFooter(m.sessionsHelpKeymap(), contentW, m.themeState.active, m.colourless)), "\n")
	unreduced := []rune(footer[len(footer)-1])
	cut := contentW - m.themePanel.width
	if unreduced[cut-1] == ' ' || unreduced[cut] == ' ' {
		t.Fatalf("fixture: the panel's left border falls on a word boundary at col %d (%q) — the cut-mid-label case is not exercised",
			cut, string(unreduced[max(cut-8, 0):min(cut+8, len(unreduced))]))
	}

	frame := strings.Split(ansi.Strip(m.View().Content), "\n")
	_, _, topPad, _ := gutterPadding(m.termWidth, m.termHeight, contentW, m.contentHeight())
	covered := []rune(frame[topPad+m.contentHeight()-1])
	left := (m.termWidth - contentW) / 2
	if got, want := string(covered[left:left+cut]), string(unreduced[:cut]); got != want {
		t.Errorf("the covered footer reflowed to the reduced width: got %q, want the unreduced footer's first %d cells %q", got, cut, want)
	}
}

// The §14A forced-close copy, written out VERBATIM here rather than read from the
// production constants — a test that asserts a constant against itself pins
// nothing, and this copy is the spec's, not the implementation's.
const (
	specNarrowClosedFlash = "terminal too narrow — theme picker closed"
	specShortClosedFlash  = "terminal too short — theme picker closed"
)

// geometryBelowWidthFloor / geometryBelowHeightFloor are content regions that fail
// exactly one dimension of §9.8's floor.
func geometryBelowWidthFloor() (contentW, contentH int) {
	return themePanelMinWidth - 1, geometryContentH
}

func geometryBelowHeightFloor() (contentW, contentH int) {
	return geometryWideW, themePanelMinHeight(themePanelKeymap(), false) - 1
}

// requireForcedClose asserts a resize took §9.8's forced close: the panel is gone,
// everything it retained is discarded, and the pinned copy is in the flash slot.
func requireForcedClose(t *testing.T, m Model, wantFlash string) {
	t.Helper()
	if m.themePanel.open {
		t.Fatal("a resize below the floor left the panel open")
	}
	if got := m.themePanel; got.width != 0 || len(got.union.Rows) != 0 || len(got.enumeration.Entries) != 0 || got.badges != nil {
		t.Errorf("the forced close retained panel state %+v, want the zero value — it takes the `Esc` path exactly", got)
	}
	if got := m.flashText; got != wantFlash {
		t.Errorf("the forced close raised %q, want §14A's %q", got, wantFlash)
	}
}

// TestPanelGeometry_ResizeBelowWidthFloorClosesWithFlash: it force-closes with the
// pinned narrow copy.
func TestPanelGeometry_ResizeBelowWidthFloorClosesWithFlash(t *testing.T) {
	m := newGeometryPanelModel(t, geometryWideW, geometryContentH)
	contentW, contentH := geometryBelowWidthFloor()
	if dim, ok := themePanelFloor(contentW, contentH, false); ok || dim != dimWidth {
		t.Fatalf("fixture: a %dx%d content region reports (%v, ok=%v), want (dimWidth, false)", contentW, contentH, dim, ok)
	}

	m = resizeForTest(t, m, contentW, contentH)

	requireForcedClose(t, m, specNarrowClosedFlash)
	if got := themePanelNarrowClosedFlash; got != specNarrowClosedFlash {
		t.Errorf("the pinned constant is %q, want §14A's %q", got, specNarrowClosedFlash)
	}
}

// TestPanelGeometry_ResizeBelowHeightFloorClosesWithFlash: it force-closes with the
// pinned short copy.
//
// The terminal is left WIDE, so the copy is the height dimension's rather than the
// width-first answer a terminal failing both would get.
func TestPanelGeometry_ResizeBelowHeightFloorClosesWithFlash(t *testing.T) {
	m := newGeometryPanelModel(t, geometryWideW, geometryContentH)
	contentW, contentH := geometryBelowHeightFloor()
	if dim, ok := themePanelFloor(contentW, contentH, false); ok || dim != dimHeight {
		t.Fatalf("fixture: a %dx%d content region reports (%v, ok=%v), want (dimHeight, false)", contentW, contentH, dim, ok)
	}

	m = resizeForTest(t, m, contentW, contentH)

	requireForcedClose(t, m, specShortClosedFlash)
	if got := themePanelShortClosedFlash; got != specShortClosedFlash {
		t.Errorf("the pinned constant is %q, want §14A's %q", got, specShortClosedFlash)
	}
	if got := ansi.Strip(m.View().Content); !strings.Contains(got, specShortClosedFlash) {
		t.Errorf("the post-close frame carries no %q band:\n%s", specShortClosedFlash, got)
	}
}

// TestPanelGeometry_ForcedCloseIsTheEscPath: it force-closes through the `Esc` path
// exactly.
//
// §9.8: "a forced close takes the `Esc` path exactly — it discards an uncommitted
// preview and renders the resolved persisted state". Two teardowns that agree today
// are two teardowns that can drift, and the drift lands on the one path where the
// user cannot reopen the panel to see what happened.
//
// The comparison is against the `Esc` path's OWN output at the same final size, with
// the SAME notice-band history driven through both — so what is compared is the
// close, not the band the forced one additionally raises.
//
// The band has to be driven through the control rather than merely cleared from the
// forced model, because `bubbles/list` derives its page from the height at the moment
// SetSize runs and that derivation is PATH-DEPENDENT: a list squeezed by a band and
// handed the row back settles on a different page than one that was never squeezed.
// That is the library's hysteresis, not a difference between the two closes, and
// leaving it in the comparison would fail this test for a reason it is not about.
func TestPanelGeometry_ForcedCloseIsTheEscPath(t *testing.T) {
	contentW, contentH := geometryBelowHeightFloor()
	rows := geometryRows()

	previewed := func() Model {
		m := newGeometryPanelModel(t, geometryWideW, geometryContentH)
		m = pressPanelKey(t, m, arrowDown)
		if m.themeState.active == rows[0].Theme {
			t.Fatal("fixture: the arrow previewed nothing, so the two closes have nothing to discard")
		}
		return m
	}

	forced := resizeForTest(t, previewed(), contentW, contentH)
	viaEsc := resizeForTest(t, closeThemePanelForTest(t, previewed()), contentW, contentH)

	if forced.themePanel.open {
		t.Fatal("the forced close left the panel open")
	}
	if forced.themeState.active != rows[0].Theme {
		t.Errorf("the forced close rendered canvas %s, want the resolved persisted %s", forced.themeState.active.Canvas.Value, rows[0].Theme.Canvas.Value)
	}
	if forced.themeState.active != viaEsc.themeState.active {
		t.Errorf("the forced close rendered canvas %s and `Esc` rendered %s", forced.themeState.active.Canvas.Value, viaEsc.themeState.active.Canvas.Value)
	}
	if got := len(forced.themePanel.union.Rows); got != 0 {
		t.Errorf("the forced close retained %d union row(s), want the enumeration discarded", got)
	}

	if got := forced.flashText; got == "" {
		t.Fatal("fixture: the forced close raised no flash, so the control's band history matches nothing")
	}
	(&viaEsc).setFlash(forced.flashText)
	(&forced).clearFlash()
	(&viaEsc).clearFlash()
	if got, want := forced.View().Content, viaEsc.View().Content; got != want {
		t.Errorf("the forced close's frame is not `Esc`'s\nforced: %q\nesc:    %q", escSeq(got), escSeq(want))
	}
}

// TestPanelGeometry_ForcedCloseWritesNothing: it writes nothing.
//
// §9.2's "every write is an explicit keypress" has to hold on the one close the user
// did not ask for. The seams are the routes a write could take: internal/tui
// resolves no config path and reads no PORTAL_* env var, so every path from this
// package to prefs.json runs through one of them.
func TestPanelGeometry_ForcedCloseWritesNothing(t *testing.T) {
	mode := &countingModePersister{}
	committer := &fakeThemePersister{}
	m := newGeometryPanelModel(t, geometryWideW, geometryContentH)
	WithModePersister(mode)(&m)
	WithThemePersister(committer)(&m)
	m = pressPanelKey(t, m, arrowDown)

	contentW, contentH := geometryBelowWidthFloor()
	m = resizeForTest(t, m, contentW, contentH)
	if m.themePanel.open {
		t.Fatal("fixture: the resize did not force-close the panel")
	}

	if mode.calls != 0 {
		t.Errorf("the forced close persisted %d preference(s); every write is an explicit keypress (§9.2)", mode.calls)
	}
	if len(committer.slugs) != 0 {
		t.Errorf("the forced close committed %v; nothing writes on a close", committer.slugs)
	}

	// Positive control: the mode seam is wired to THIS model and live, so the zeros
	// above are an absence of writes rather than an absence of wiring.
	m = pressPanelKey(t, m, tea.KeyPressMsg{Code: 's', Text: "s"})
	if mode.calls != 1 {
		t.Fatalf("positive control: `s` on the closed picker persisted %d time(s), want 1", mode.calls)
	}
}

// geometryMessage is §14A's slot-from-constant confirm at a slug SHORT enough to
// survive §9.5's truncation floor at the minimum panel width, so the only
// degradation this suite sees is the slot's own. The composed copy is 27 cells,
// which overflows the minimum panel's inner width (the test asserts that it does)
// and so exercises both halves of §9.1's slot rule from one message.
//
// It is a FUNCTION rather than a package-level var, following this package's
// test-side convention (see themePanelFooterPinnedRows).
func geometryMessage() themePanelMessage {
	return themePanelMessage{Kind: themeMessageConfirm, Slug: "nord"}
}

// geometryMessageText is that confirm's composed copy at the minimum panel's inner
// width — the string the slot lays out, taken from the production composer so the
// test states the SLOT's rule rather than restating the copy.
func geometryMessageText() string {
	return themePanelConfirmText(geometryMessage().Slug, themePanelInnerWidth(themePanelMinWidth))
}

// TestPanelGeometry_MessageTruncatesAtFloorHeight: it truncates the message at the
// minimum height.
//
// §9.1: "at the minimum panel width the slot may wrap to two rows… it does cost a
// row of vertical budget, so at the minimum HEIGHT the message is truncated to one
// line rather than wrapped."
//
// THE TWO DIMENSIONS DEGRADE DIFFERENTLY ON PURPOSE. §9.8's floor counts exactly one
// message row, so a two-row message at the floor would leave zero list rows or
// overflow the frame; truncation degrades the message the user is being asked to
// answer rather than the row they are answering ABOUT.
//
// The slot is driven by setting the field directly — the same way task 8-6 drove
// the height recompute — because raising the confirm is task 9-5's dispatch.
//
// THE BODY IS THE FLOOR'S MINIMUM PLUS THE CONFIRM'S FOOTER SAVING. §9.2's nested
// confirm scope substitutes a two-row footer for the standing four, and
// themePanelListSize hands that saving to the list body — so the message still
// costs its own row(s) and the body is correspondingly taller. The saving is
// derived from the two scopes rather than written as a literal, so it stays true if
// either scope's Core membership changes.
func TestPanelGeometry_MessageTruncatesAtFloorHeight(t *testing.T) {
	th := testDarkTheme(t)
	rows := themePanelTestRows(6)
	inner := themePanelInnerWidth(themePanelMinWidth)
	footer := themePanelFooterHeight(themePanelFooterScope(geometryMessage()))
	floor := themePanelMinHeight(themePanelKeymap(), false)
	wantBody := themePanelMinBodyRows + themePanelFooterHeight(themePanelKeymap()) - footer

	if lipgloss.Width(geometryMessageText()) <= inner {
		t.Fatalf("fixture: the %d-cell message fits the %d-cell inner width, so neither degradation is exercised", lipgloss.Width(geometryMessageText()), inner)
	}

	p := newThemePanelFixture(themePanelFixtureOpts{
		th:      th,
		width:   themePanelMinWidth,
		rows:    rows,
		message: geometryMessage(),
	})

	t.Run("at the minimum height it truncates to one line", func(t *testing.T) {
		if got := themePanelMessageHeight(geometryMessage(), inner, false); got != 1 {
			t.Fatalf("the truncated slot reserves %d rows, want 1", got)
		}
		if _, body := themePanelListSize(p, floor); body != wantBody {
			t.Errorf("at the floor the list body is %d rows, want %d", body, wantBody)
		}

		lines := themePanelLines(renderThemePanel(p, floor, th, false))
		if len(lines) != floor {
			t.Fatalf("the panel rendered %d rows at its floor of %d", len(lines), floor)
		}
		if listRow := lines[themePanelHeaderRows()]; !strings.Contains(listRow, rows[0].Label()) {
			t.Errorf("row %d is not the single list row (%q): %q", themePanelHeaderRows(), rows[0].Label(), listRow)
		}
		slot := strings.TrimRight(lines[floor-footer-1], " ")
		if !strings.Contains(slot, themeRowEllipsis) {
			t.Errorf("the message slot = %q, want it truncated with %q at the minimum height", slot, themeRowEllipsis)
		}
		if strings.Contains(slot, "y / n") {
			t.Errorf("the message slot = %q, want the tail dropped rather than wrapped onto a second row", slot)
		}
	})

	t.Run("one row above the floor it may wrap to two", func(t *testing.T) {
		height := floor + 1
		if got := themePanelMessageHeight(geometryMessage(), inner, true); got != themePanelMessageWrapRows {
			t.Fatalf("the wrapped slot reserves %d rows, want %d", got, themePanelMessageWrapRows)
		}
		if _, body := themePanelListSize(p, height); body != wantBody {
			t.Errorf("one row above the floor the list body is %d rows, want %d — the second message row comes out of the slot's own budget", body, wantBody)
		}

		lines := themePanelLines(renderThemePanel(p, height, th, false))
		if len(lines) != height {
			t.Fatalf("the panel rendered %d rows at height %d", len(lines), height)
		}
		slot := lines[height-footer-themePanelMessageWrapRows : height-footer]
		joined := strings.Join(slot, " ")
		if strings.Contains(joined, themeRowEllipsis) {
			t.Errorf("the wrapped slot %q is truncated; above the floor it wraps", slot)
		}
		// The WHOLE message survives, word for word. It is reassembled from the two
		// rows rather than searched for as a substring, because the wrap point is a
		// function of the inner width and may fall anywhere — including mid-phrase —
		// and what §9.1 promises is that nothing is lost, not where the break lands.
		if got, want := chromeWords(themePanelSlotText(slot)), chromeWords(geometryMessageText()); got != want {
			t.Errorf("the wrapped slot reads %q, want the whole message %q", got, want)
		}
		if listRow := lines[themePanelHeaderRows()]; !strings.Contains(listRow, rows[0].Label()) {
			t.Errorf("row %d is not the single list row (%q): %q", themePanelHeaderRows(), rows[0].Label(), listRow)
		}
	})
}

// TestPanelGeometry_RendersAtTheFloor: it renders coherently at exactly the floor.
//
// The floor is only meaningful if the panel it admits is one the user can act on:
// header, the pinned directory row when the directory is unusable, ONE list row, the
// message row's budget and the WHOLE footer — with nothing pushed off the bottom.
//
// The footer is the row that goes first when the arithmetic is wrong, and `esc
// close` is its first entry, so a floor that overflows takes the one key that closes
// a panel the user can no longer read the way out of.
//
// THE FLOOR IS THE STANDING SCOPE'S IN EVERY CASE (§9.8), but the FOOTER that must
// survive at it is the slot's own: with §9.2's confirm live the panel renders the
// nested scope's two rows, and the whole of THAT is what must be present.
func TestPanelGeometry_RendersAtTheFloor(t *testing.T) {
	th := testDarkTheme(t)
	rows := themePanelTestRows(12)
	inner := themePanelInnerWidth(themePanelMinWidth)

	for _, dirUnusable := range []bool{false, true} {
		for _, message := range []themePanelMessage{{}, geometryMessage()} {
			floor := themePanelMinHeight(themePanelKeymap(), dirUnusable)
			wantFooter := themePanelLines(renderThemePanelFooter(themePanelFooterScope(message), inner, th, false))
			name := fmt.Sprintf("dir=%v/msg=%v/h=%d", dirUnusable, message.Kind != themeMessageNone, floor)
			t.Run(name, func(t *testing.T) {
				p := newThemePanelFixture(themePanelFixtureOpts{
					th:          th,
					width:       themePanelMinWidth,
					rows:        rows,
					dirUnusable: dirUnusable,
					message:     message,
				})
				lines := themePanelLines(renderThemePanel(p, floor, th, false))

				if len(lines) != floor {
					t.Fatalf("the panel rendered %d rows at its floor of %d", len(lines), floor)
				}
				if got, want := strings.TrimRight(lines[themePanelHeaderLabelRow()], " "), themePanelContentPrefix()+themePanelHeaderLabel; got != want {
					t.Errorf("the header label row = %q, want %q", got, want)
				}
				at := themePanelHeaderRows()
				if dirUnusable {
					if got, want := strings.TrimRight(lines[at], " "), themePanelContentPrefix()+themePanelDirUnreadable; got != want {
						t.Errorf("the directory row = %q, want %q", got, want)
					}
					at++
				}
				if got := lines[at]; !strings.Contains(got, rows[0].Label()) {
					t.Errorf("the single list row = %q, want it carrying %q", got, rows[0].Label())
				}
				footer := lines[len(lines)-len(wantFooter):]
				for i, want := range wantFooter {
					if got, wantRow := footer[i], themePanelContentPrefix()+want; got != wantRow {
						t.Errorf("footer row %d = %q, want %q — the floor overflowed and the assembly cut the footer", i, got, wantRow)
					}
				}
				if message.Kind == themeMessageNone {
					return
				}
				slot := strings.TrimRight(lines[len(lines)-len(wantFooter)-1], " ")
				if !strings.HasPrefix(slot, themePanelContentPrefix()+"clear constant") {
					t.Errorf("the row above the footer is not the message slot: %q", slot)
				}
			})
		}
	}
}

// String renders a themePanelDim in a failure message. It is test-only: production
// selects copy through themePanelForcedCloseFlash, never by printing the value.
func (d themePanelDim) String() string {
	switch d {
	case dimWidth:
		return "dimWidth"
	case dimHeight:
		return "dimHeight"
	default:
		return fmt.Sprintf("dimNone(%d)", int(d))
	}
}

// themePanelSlotText joins a rendered message slot's rows back into one string,
// dropping each row's border-and-gutter prefix and its trailing canvas pad.
func themePanelSlotText(slot []string) string {
	text := make([]string, 0, len(slot))
	for _, row := range slot {
		text = append(text, strings.TrimSpace(strings.TrimPrefix(row, panelFrameSide)))
	}
	return strings.Join(text, " ")
}
