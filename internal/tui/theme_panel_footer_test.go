package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// The §9.1 vertical-footer gate. These tests pin the theme panel's footer: §14A's
// four rows verbatim, the two-column key-column geometry it borrows from
// helpModalRow, the accent.key / text.muted split it borrows from the horizontal
// footer, its self-measured height, and the substitution point Phase 9's nested
// confirm scope replaces it through.
//
// Colour roles are asserted as DISTINCT theme-resolved SGR runs (like the
// session-row and panel-row anatomy tests), so the assertion says WHICH text a
// token painted rather than merely that the token appears somewhere on the line.
//
// No t.Parallel() — the package-level mock convention and the shared canvas
// helpers make parallelism unsafe across this package's tests.

// The two ends of the inner content width §9.8's ~27–34 column panel leaves for
// its footer. The footer owns neither end of the ladder (task 8-11 declares it and
// task 8-6 subtracts the left border), so these are the test's own representatives
// of the band every row must survive.
const (
	themePanelFooterTestWidth    = 29
	themePanelFooterTestMinWidth = 23
)

// themePanelFooterPinnedRows is §14A's panel-footer copy, verbatim: "footer
// `⏎ set theme` / `d set as dark` / `l set as light` / `esc close`". The rendered
// rows are asserted against THESE strings rather than against anything re-derived
// from the descriptor, so a copy change has to be made here and in §14A together.
//
// It is a FUNCTION rather than a package-level var, following this package's
// test-side convention (see testDarkTheme): a shared mutable slice is one careless
// index assignment away from a test rewriting another test's expectations.
func themePanelFooterPinnedRows() []string {
	return []string{
		"⏎ set theme",
		"d set as dark",
		"l set as light",
		"esc close",
	}
}

// themePanelFooterLines splits a rendered footer block into its rows with every
// SGR sequence stripped — the text a user actually reads.
func themePanelFooterLines(block string) []string {
	return strings.Split(ansi.Strip(block), "\n")
}

// themePanelFooterCopy reduces one rendered row to its `<glyph> <label>` reading:
// the key column's padding and the row's pad-to-width collapse away, leaving the
// pinned phrase. It is what lets the §14A copy be asserted verbatim against a row
// whose glyph sits in a fixed-width column.
func themePanelFooterCopy(row string) string {
	return strings.Join(strings.Fields(row), " ")
}

// themePanelConfirmScope is Phase 9's nested confirm scope (§9.2) as a stand-in:
// the two keys its footer TEMPORARILY REPLACES the panel footer with while the
// slot-from-constant confirm is live. It lives in the test because Phase 9 owns
// the real scope — this task only has to leave room for it, and the room is
// exactly "the renderer renders whatever entries it is handed".
func themePanelConfirmScope() []keymapEntry {
	return []keymapEntry{
		{Key: "y", Action: "confirm", Core: true},
		{Key: "n", Action: "cancel", Core: true},
	}
}

// TestThemePanelFooter_PinnedCopy asserts the rendered footer is EXACTLY the four
// §14A rows, in descriptor order, one line each.
//
// The copy is a layout constraint as much as a copy choice (§14A) — it has to fit
// 27–34 columns — so it is pinned verbatim rather than paraphrased, and the row
// count is pinned with it: a fifth row would silently grow task 8-11's height
// floor.
func TestThemePanelFooter_PinnedCopy(t *testing.T) {
	block := renderThemePanelFooter(themePanelKeymap(), themePanelFooterTestWidth, testDarkTheme(t), false)
	lines := themePanelFooterLines(block)

	if len(lines) != len(themePanelFooterPinnedRows()) {
		t.Fatalf("panel footer has %d rows, want %d (§14A's four):\n%s", len(lines), len(themePanelFooterPinnedRows()), block)
	}
	for i, want := range themePanelFooterPinnedRows() {
		if got := themePanelFooterCopy(lines[i]); got != want {
			t.Errorf("row %d reads %q, want the §14A copy %q", i, got, want)
		}
		if h := lipgloss.Height(lines[i]); h != 1 {
			t.Errorf("row %d is %d lines, want exactly 1", i, h)
		}
	}
}

// TestThemePanelFooter_NonCoreEntriesAreNotRendered asserts the §9.12 split from
// the render side: the arrows and paging the descriptor must carry are ABSENT
// from the footer.
//
// The two halves are one rule, not two: the descriptor is complete for the
// dispatch guard, and the footer filters to Core — so neither the six-entry scope
// nor §14A's four-row footer is a special case of the other.
func TestThemePanelFooter_NonCoreEntriesAreNotRendered(t *testing.T) {
	entries := themePanelKeymap()
	visible := ansi.Strip(renderThemePanelFooter(entries, themePanelFooterTestWidth, testDarkTheme(t), false))

	for _, e := range entries {
		if e.Core {
			continue
		}
		if strings.Contains(visible, e.Action) {
			t.Errorf("footer renders the non-core label %q — arrows and paging are descriptor-only:\n%s", e.Action, visible)
		}
		for _, glyph := range []string{e.Key, helpKeyGlyph(e)} {
			if strings.Contains(visible, glyph) {
				t.Errorf("footer renders the non-core glyph %q:\n%s", glyph, visible)
			}
		}
	}
}

// TestThemePanelFooter_KeyIsAccentKeyLabelIsTextMuted asserts §9.1's token table
// for the footer — "key glyphs accent.key, labels text.muted, the same split the
// horizontal footer uses" — as DISTINCT SGR runs, so the assertion says which text
// each token painted rather than merely that both tokens appear on the line.
func TestThemePanelFooter_KeyIsAccentKeyLabelIsTextMuted(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		block := renderThemePanelFooter(themePanelKeymap(), themePanelFooterTestWidth, th, false)
		rows := strings.Split(block, "\n")

		core := themePanelFooterCoreEntries(t)
		if len(rows) != len(core) {
			t.Fatalf("[%v] footer has %d rows, want %d", themeLabel(th), len(rows), len(core))
		}
		for i, e := range core {
			if got, want := themeRowRunAfter(t, rows[i], tokenFgSeq(t, th.AccentKey)), helpKeyGlyph(e); got != want {
				t.Errorf("[%v] row %d accent.key run painted %q, want the key glyph %q", themeLabel(th), i, got, want)
			}
			if got, want := themeRowRunAfter(t, rows[i], tokenFgSeq(t, th.TextMuted)), e.Action; got != want {
				t.Errorf("[%v] row %d text.muted run painted %q, want the label %q", themeLabel(th), i, got, want)
			}
		}
	}
}

// TestThemePanelFooter_KeyColumnIsFixedWidth asserts the labels share one left
// edge: the key column is padded to a FIXED width, so `⏎`, `d`, `l` and `esc` all
// hand off to their labels at the same column.
//
// The width is fixed rather than derived from the entries it is handed, and the
// substituted confirm scope is asserted to land on the SAME edge — Phase 9's
// `y`/`n` footer replaces this one in place, and a per-slice column would step
// every label two cells left as the confirm raises and back as it resolves.
func TestThemePanelFooter_KeyColumnIsFixedWidth(t *testing.T) {
	th := testDarkTheme(t)
	wantEdge := themePanelFooterKeyColumnWidth + lipgloss.Width(footerKeyLabelGap)

	t.Run("it aligns the labels in a fixed key column", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			entries []keymapEntry
		}{
			{"panel scope", themePanelKeymap()},
			{"substituted confirm scope", themePanelConfirmScope()},
		} {
			t.Run(tc.name, func(t *testing.T) {
				rows := themePanelFooterLines(renderThemePanelFooter(tc.entries, themePanelFooterTestWidth, th, false))
				for i, e := range coreEntriesOf(tc.entries) {
					at := themePanelFooterLabelColumn(t, rows[i], e.Action)
					if at != wantEdge {
						t.Errorf("row %d label %q starts at column %d, want the fixed key column edge %d: %q",
							i, e.Action, at, wantEdge, rows[i])
					}
				}
			})
		}
	})

	t.Run("it pads every row to the panel's inner width over the canvas", func(t *testing.T) {
		for _, w := range []int{themePanelFooterTestWidth, themePanelFooterTestMinWidth} {
			block := renderThemePanelFooter(themePanelKeymap(), w, th, false)
			if seq := canvasSeq(t, th); !strings.Contains(block, seq) {
				t.Errorf("at width %d the footer does not paint the canvas background %q:\n%s", w, seq, block)
			}
			for i, row := range strings.Split(block, "\n") {
				if got := lipgloss.Width(row); got != w {
					t.Errorf("at width %d, row %d is %d cells wide, want exactly %d (canvas covers every cell)", w, i, got, w)
				}
			}
		}
	})
}

// TestThemePanelFooter_AcceptsASubstitutedScope is the Phase 9 substitution point:
// handed the nested confirm scope's entries the SAME renderer produces the
// confirm's footer, with no change to the renderer and no second one.
//
// §9.2 switches the panel footer to `y confirm` / `n cancel` while the
// slot-from-constant confirm is live and back when it resolves, so a renderer that
// reached for themePanelKeymap() internally would have to be forked to do it.
func TestThemePanelFooter_AcceptsASubstitutedScope(t *testing.T) {
	block := renderThemePanelFooter(themePanelConfirmScope(), themePanelFooterTestWidth, testDarkTheme(t), false)
	rows := themePanelFooterLines(block)

	want := []string{"y confirm", "n cancel"}
	if len(rows) != len(want) {
		t.Fatalf("substituted footer has %d rows, want %d:\n%s", len(rows), len(want), block)
	}
	for i, w := range want {
		if got := themePanelFooterCopy(rows[i]); got != w {
			t.Errorf("substituted row %d reads %q, want %q", i, got, w)
		}
	}
	for _, pinned := range themePanelFooterPinnedRows() {
		if strings.Contains(ansi.Strip(block), themePanelFooterCopy(pinned)) {
			t.Errorf("substituted footer still carries the panel scope's %q:\n%s", pinned, ansi.Strip(block))
		}
	}
}

// TestThemePanelFooter_HeightMatchesRender asserts themePanelFooterHeight is the
// height of the block that actually renders — for the panel scope AND the
// substituted confirm scope, at both ends of the width band and in both palettes.
//
// Task 8-6's layout subtracts this value and task 8-11's height floor adds it, so
// a height that is merely CORRECT rather than MEASURED is one refactor away from
// reserving a row the footer does not draw.
func TestThemePanelFooter_HeightMatchesRender(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []keymapEntry
		want    int
	}{
		{"panel scope", themePanelKeymap(), 4},
		{"substituted confirm scope", themePanelConfirmScope(), 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := themePanelFooterHeight(tc.entries); got != tc.want {
				t.Errorf("themePanelFooterHeight = %d, want %d (one row per Core entry)", got, tc.want)
			}
			for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
				for _, w := range []int{themePanelFooterTestWidth, themePanelFooterTestMinWidth} {
					rendered := lipgloss.Height(renderThemePanelFooter(tc.entries, w, th, false))
					if got := themePanelFooterHeight(tc.entries); got != rendered {
						t.Errorf("[%v w=%d] themePanelFooterHeight = %d, but the rendered block is %d rows",
							themeLabel(th), w, got, rendered)
					}
				}
			}
		})
	}
}

// TestThemePanelFooter_Colourless asserts the NO_COLOR carve-out (§2.5): the
// footer drops the canvas and every hue and renders on the terminal's native
// fg/bg, with the pinned copy structurally intact.
//
// §9.10 blocks the panel under NO_COLOR outright, so this is the defence rather
// than the daily path — the footer must not be the one surface that reintroduces
// a colour the carve-out removed everywhere else.
func TestThemePanelFooter_Colourless(t *testing.T) {
	th := testDarkTheme(t)
	block := renderThemePanelFooter(themePanelKeymap(), themePanelFooterTestWidth, th, true)

	if frameHasAnyBackgroundSGR(t, block) {
		t.Errorf("colourless footer activates a background SGR — NO_COLOR paints no canvas: %q", escSeq(block))
	}
	if frameHasAnyForegroundSGR(t, block) {
		t.Errorf("colourless footer activates a foreground SGR — NO_COLOR imposes no hue: %q", escSeq(block))
	}
	rows := themePanelFooterLines(block)
	if len(rows) != len(themePanelFooterPinnedRows()) {
		t.Fatalf("colourless footer has %d rows, want %d", len(rows), len(themePanelFooterPinnedRows()))
	}
	for i, want := range themePanelFooterPinnedRows() {
		if got := themePanelFooterCopy(rows[i]); got != want {
			t.Errorf("colourless row %d reads %q, want %q", i, got, want)
		}
	}
}

// themePanelFooterCoreEntries is the panel scope's Core entries in descriptor
// order — the rows the footer renders, resolved from the descriptor so a test
// cannot hold its own second copy of the split.
func themePanelFooterCoreEntries(t *testing.T) []keymapEntry {
	t.Helper()
	core := coreEntriesOf(themePanelKeymap())
	if len(core) != len(themePanelFooterPinnedRows()) {
		t.Fatalf("panel scope has %d Core entries, want %d (§14A's four rows)", len(core), len(themePanelFooterPinnedRows()))
	}
	return core
}

// themePanelFooterLabelColumn returns the CELL column a row's label starts at.
//
// It measures in cells rather than bytes because the key glyphs are multi-byte
// (`⏎` alone is three), and a byte offset would report a shared left edge as three
// different columns — which is the thing the test is trying to tell apart.
func themePanelFooterLabelColumn(t *testing.T, row, label string) int {
	t.Helper()
	at := strings.Index(row, label)
	if at < 0 {
		t.Fatalf("row %q carries no label %q", row, label)
	}
	return lipgloss.Width(row[:at])
}

// coreEntriesOf filters any entry slice to its Core entries, preserving order.
func coreEntriesOf(entries []keymapEntry) []keymapEntry {
	core := make([]keymapEntry, 0, len(entries))
	for _, e := range entries {
		if e.Core {
			core = append(core, e)
		}
	}
	return core
}
