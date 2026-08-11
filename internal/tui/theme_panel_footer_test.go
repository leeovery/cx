package tui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// Derived rather than restated so a move of the width ladder reaches these
// assertions instead of leaving them measuring a width the panel never renders at.
var (
	themePanelFooterTestWidth    = themePanelInnerWidth(themePanelPreferredWidth)
	themePanelFooterTestMinWidth = themePanelInnerWidth(themePanelMinWidth)
)

// Verbatim rather than re-derived from the descriptor, so a copy change has to be
// made in two places. A function rather than a var: a shared mutable slice is one
// index assignment away from a test rewriting another test's expectations.
func themePanelFooterPinnedRows() []string {
	return []string{
		"⏎ set theme",
		"d set as dark",
		"l set as light",
		"esc close",
	}
}

func themePanelFooterLines(block string) []string {
	return strings.Split(ansi.Strip(block), "\n")
}

// Collapses the key column's padding and the row's pad-to-width, so a pinned
// phrase can be asserted verbatim against a row whose glyph sits in a fixed column.
func themePanelFooterCopy(row string) string {
	return strings.Join(strings.Fields(row), " ")
}

// The copy is a layout constraint as much as a copy choice — it has to fit 24–30
// columns — and a fifth row would silently grow the panel's height floor.
func TestThemePanelFooter_PinnedCopy(t *testing.T) {
	block := renderThemePanelFooter(themePanelKeymap(), themePanelFooterTestWidth, testDarkTheme(t), false)
	lines := themePanelFooterLines(block)

	if len(lines) != len(themePanelFooterPinnedRows()) {
		t.Fatalf("panel footer has %d rows, want %d:\n%s", len(lines), len(themePanelFooterPinnedRows()), block)
	}
	for i, want := range themePanelFooterPinnedRows() {
		if got := themePanelFooterCopy(lines[i]); got != want {
			t.Errorf("row %d reads %q, want the pinned copy %q", i, got, want)
		}
		if h := lipgloss.Height(lines[i]); h != 1 {
			t.Errorf("row %d is %d lines, want exactly 1", i, h)
		}
	}
}

// The descriptor is complete for the dispatch guard and the footer filters to
// Core, so neither the six-entry scope nor the four-row footer is a special case
// of the other.
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

// The key column width is fixed rather than derived from the entries it is handed:
// a per-slice column would step every label two cells left as the confirm raises
// and back as it resolves.
func TestThemePanelFooter_KeyColumnIsFixedWidth(t *testing.T) {
	th := testDarkTheme(t)
	wantEdge := themePanelFooterKeyColumnWidth + lipgloss.Width(footerKeyLabelGap)

	t.Run("it aligns the labels in a fixed key column", func(t *testing.T) {
		for _, tc := range []struct {
			name    string
			entries []keymapEntry
		}{
			{"panel scope", themePanelKeymap()},
			{"substituted confirm scope", themePanelConfirmKeymap()},
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

// A renderer that reached for themePanelKeymap() internally would have to be
// forked to render the confirm scope.
func TestThemePanelFooter_AcceptsASubstitutedScope(t *testing.T) {
	block := renderThemePanelFooter(themePanelConfirmKeymap(), themePanelFooterTestWidth, testDarkTheme(t), false)
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

// The panel's layout subtracts this value and its height floor adds it, so a
// height that is merely correct rather than measured is one refactor away from
// reserving a row the footer does not draw.
func TestThemePanelFooter_HeightMatchesRender(t *testing.T) {
	for _, tc := range []struct {
		name    string
		entries []keymapEntry
		want    int
	}{
		{"panel scope", themePanelKeymap(), 4},
		{"substituted confirm scope", themePanelConfirmKeymap(), 2},
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

// The panel's assembly relies on no composed row exceeding the minimum inner
// width, so the figure is measured against the rendered rows rather than cited.
func TestThemePanelFooter_WidestRowIsMeasured(t *testing.T) {
	const wantWidest = 16

	widest, at := 0, ""
	for _, row := range themePanelFooterLines(renderThemePanelFooter(themePanelKeymap(), 0, testDarkTheme(t), false)) {
		if got := lipgloss.Width(row); got > widest {
			widest, at = got, row
		}
	}

	if widest != wantWidest {
		t.Errorf("the widest footer row is %d cells (%q), want %d", widest, at, wantWidest)
	}
	if widest > themePanelFooterTestMinWidth {
		t.Errorf("the widest footer row is %d cells against a %d-cell minimum inner width — the panel pads rows, it never truncates them",
			widest, themePanelFooterTestMinWidth)
	}
}

// The panel is blocked under NO_COLOR outright, so this is the defence rather than
// the daily path: the footer must not reintroduce a colour the carve-out removed.
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

// Resolved from the descriptor so a test cannot hold its own second copy of the split.
func themePanelFooterCoreEntries(t *testing.T) []keymapEntry {
	t.Helper()
	core := coreEntriesOf(themePanelKeymap())
	if len(core) != len(themePanelFooterPinnedRows()) {
		t.Fatalf("panel scope has %d Core entries, want %d", len(core), len(themePanelFooterPinnedRows()))
	}
	return core
}

// Measures in cells rather than bytes: the key glyphs are multi-byte, and a byte
// offset would report a shared left edge as three different columns.
func themePanelFooterLabelColumn(t *testing.T, row, label string) int {
	t.Helper()
	at := strings.Index(row, label)
	if at < 0 {
		t.Fatalf("row %q carries no label %q", row, label)
	}
	return lipgloss.Width(row[:at])
}

func coreEntriesOf(entries []keymapEntry) []keymapEntry {
	core := make([]keymapEntry, 0, len(entries))
	for _, e := range entries {
		if e.Core {
			core = append(core, e)
		}
	}
	return core
}
