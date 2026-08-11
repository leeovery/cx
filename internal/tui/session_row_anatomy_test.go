package tui

import (
	"bytes"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
)

func renderRow(d SessionDelegate, width int, items []list.Item, index, selIndex int) string {
	m := list.New(items, d, width, 10)
	m.Select(selIndex)
	var buf bytes.Buffer
	d.Render(&buf, m, index, items[index])
	return buf.String()
}

func visibleColOf(line, sub string) int {
	stripped := ansi.Strip(line)
	before, _, ok := strings.Cut(stripped, sub)
	if !ok {
		return -1
	}
	return ansi.StringWidth(before)
}

func selectionBgParams(t *testing.T, th theme.Theme) string {
	t.Helper()
	return sgrParams(t, lipgloss.NewStyle().Background(th.BgSelection.Color()))
}

func flatItems(specs ...tmux.Session) []list.Item {
	items := make([]list.Item, len(specs))
	for i, s := range specs {
		items[i] = SessionItem{Session: s}
	}
	return items
}

func TestSessionRow_FlexNameFixedTrailingSlots(t *testing.T) {
	const w = 80
	items := flatItems(tmux.Session{Name: "alpha", Windows: 3, Attached: true})
	out := renderRow(SessionDelegate{}, w, items, 0, 0)
	vis := ansi.Strip(out)

	if !strings.Contains(vis, "alpha") {
		t.Errorf("row missing name 'alpha': %q", vis)
	}
	if !strings.Contains(vis, "3 windows") {
		t.Errorf("row missing window count '3 windows': %q", vis)
	}
	if !strings.Contains(vis, "● attached") {
		t.Errorf("row missing attached marker '● attached': %q", vis)
	}
	nameCol := visibleColOf(out, "alpha")
	countCol := visibleColOf(out, "3 windows")
	if countCol <= nameCol {
		t.Errorf("count slot (col %d) must sit right of the name (col %d): %q", countCol, nameCol, vis)
	}
	if got := lipgloss.Width(out); got != w {
		t.Errorf("row width = %d, want exactly %d (trailing slots right-pinned to the list width)", got, w)
	}
}

func TestSessionRow_ColumnAlignsRegardlessOfNameLength(t *testing.T) {
	const w = 80
	items := flatItems(
		tmux.Session{Name: "a", Windows: 1, Attached: true},
		tmux.Session{Name: "a-much-longer-session-name-here", Windows: 5, Attached: true},
	)
	short := renderRow(SessionDelegate{}, w, items, 0, 0)
	long := renderRow(SessionDelegate{}, w, items, 1, 0)

	shortCount := visibleColOf(short, "window")
	longCount := visibleColOf(long, "window")
	if shortCount < 0 || longCount < 0 {
		t.Fatalf("a count column is missing: short=%q long=%q", ansi.Strip(short), ansi.Strip(long))
	}
	if shortCount != longCount {
		t.Errorf("window counts not column-aligned: short name count col %d, long name count col %d", shortCount, longCount)
	}

	shortBullet := visibleColOf(short, "●")
	longBullet := visibleColOf(long, "●")
	if shortBullet < 0 || longBullet < 0 {
		t.Fatalf("an attached bullet is missing: short=%q long=%q", ansi.Strip(short), ansi.Strip(long))
	}
	if shortBullet != longBullet {
		t.Errorf("attached bullets not column-aligned: short col %d, long col %d", shortBullet, longBullet)
	}
}

func TestSessionRow_EmptyAttachedSlotPreservesAlignment(t *testing.T) {
	const w = 80
	items := flatItems(
		tmux.Session{Name: "attached-one", Windows: 2, Attached: true},
		tmux.Session{Name: "detached-one", Windows: 2, Attached: false},
	)
	attached := renderRow(SessionDelegate{}, w, items, 0, 0)
	detached := renderRow(SessionDelegate{}, w, items, 1, 0)

	if strings.Contains(ansi.Strip(detached), "attached") {
		t.Errorf("unattached row must not render the attached marker: %q", ansi.Strip(detached))
	}
	if a, d := visibleColOf(attached, "window"), visibleColOf(detached, "window"); a != d {
		t.Errorf("count columns misaligned across attached/unattached: %d vs %d", a, d)
	}
	if a, d := lipgloss.Width(attached), lipgloss.Width(detached); a != d {
		t.Errorf("row widths differ across attached/unattached: %d vs %d (empty slot must match marker width)", a, d)
	}
}

func TestSessionRow_SelectedShowsVioletBarTintAndOnSelectionName(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := SessionDelegate{Theme: th}
		items := flatItems(tmux.Session{Name: "selected-row", Windows: 2, Attached: false})
		out := renderRow(d, 80, items, 0, 0)

		if !strings.Contains(ansi.Strip(out), "▌") {
			t.Errorf("[%v] selected row missing the ▌ selector bar: %q", themeLabel(th), ansi.Strip(out))
		}
		if seq := tokenFgSeq(t, th.AccentPrimary); !strings.Contains(out, seq) {
			t.Errorf("[%v] selected bar missing accent.violet fg %q", themeLabel(th), seq)
		}
		if params := selectionBgParams(t, th); !lineHasBgParams(out, params) {
			t.Errorf("[%v] selected row missing the bg.selection tint %q: %q", themeLabel(th), params, escSeq(out))
		}
		if seq := tokenFgSeq(t, th.TextOnSelection); !strings.Contains(out, seq) {
			t.Errorf("[%v] selected name missing text.on-selection fg %q", themeLabel(th), seq)
		}
	}
}

func TestSessionRow_UnselectedHasNoBarOrTint(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := SessionDelegate{Theme: th}
		items := flatItems(
			tmux.Session{Name: "row-zero", Windows: 1, Attached: false},
			tmux.Session{Name: "row-one", Windows: 1, Attached: false},
		)
		out := renderRow(d, 80, items, 1, 0)

		if strings.Contains(ansi.Strip(out), "▌") {
			t.Errorf("[%v] unselected row must not carry the ▌ bar: %q", themeLabel(th), ansi.Strip(out))
		}
		if params := selectionBgParams(t, th); lineHasBgParams(out, params) {
			t.Errorf("[%v] unselected row must not carry the bg.selection tint %q: %q", themeLabel(th), params, escSeq(out))
		}
		if params := wantCanvasBgParams(t, th); !lineHasBgParams(out, params) {
			t.Errorf("[%v] unselected row missing the canvas paint %q: %q", themeLabel(th), params, escSeq(out))
		}
	}
}

func TestSessionRow_AttachedKeepsStateGreenWhenSelected(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := SessionDelegate{Theme: th}
		items := flatItems(
			tmux.Session{Name: "attached-selected", Windows: 1, Attached: true},
			tmux.Session{Name: "attached-unselected", Windows: 1, Attached: true},
		)

		green := tokenFgSeq(t, th.StatePositive)
		onSelName := tokenFgSeq(t, th.TextOnSelection)

		sel := renderRow(d, 80, items, 0, 0)
		if !strings.Contains(sel, green) {
			t.Errorf("[%v] selected attached marker missing state.green fg %q", themeLabel(th), green)
		}
		if th == testLightTheme(t) && green == onSelName {
			t.Fatalf("[light] test precondition broken: state.green == text.on-selection")
		}

		uns := renderRow(d, 80, items, 1, 0)
		if !strings.Contains(uns, green) {
			t.Errorf("[%v] unselected attached marker missing state.green fg %q", themeLabel(th), green)
		}
	}
}

func TestSessionRow_SelectedCountInTextStrong(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := SessionDelegate{Theme: th}
		items := flatItems(
			tmux.Session{Name: "row-zero", Windows: 4, Attached: false},
			tmux.Session{Name: "row-one", Windows: 4, Attached: false},
		)

		sel := renderRow(d, 80, items, 0, 0)
		uns := renderRow(d, 80, items, 1, 0)

		strong := tokenFgSeq(t, th.TextSecondary)
		detail := tokenFgSeq(t, th.TextMuted)

		if !strings.Contains(sel, strong) {
			t.Errorf("[%v] selected-row count missing text.strong fg %q", themeLabel(th), strong)
		}
		if !strings.Contains(uns, detail) {
			t.Errorf("[%v] unselected-row count missing text.detail fg %q", themeLabel(th), detail)
		}
	}
}

func TestSessionRow_OverLongNameTruncatesWithoutPushingSlots(t *testing.T) {
	const w = 40
	longName := "this-is-a-really-very-long-session-name-that-overflows"
	items := flatItems(tmux.Session{Name: longName, Windows: 7, Attached: true})
	out := renderRow(SessionDelegate{}, w, items, 0, 0)
	vis := ansi.Strip(out)

	if strings.Contains(vis, longName) {
		t.Errorf("over-long name should be truncated, but the full name rendered: %q", vis)
	}
	if !strings.Contains(vis, "…") {
		t.Errorf("truncated name should carry the ellipsis glyph: %q", vis)
	}
	if !strings.Contains(vis, "7 windows") {
		t.Errorf("window-count slot pushed off-row by the long name: %q", vis)
	}
	if !strings.Contains(vis, "● attached") {
		t.Errorf("attached slot pushed off-row by the long name: %q", vis)
	}
	if got := lipgloss.Width(out); got != w {
		t.Errorf("truncated row width = %d, want exactly %d (no overflow, slots right-pinned)", got, w)
	}
}

func TestSessionRow_NeverOverflowsAtNarrowWidths(t *testing.T) {
	for _, w := range []int{1, 5, 10, 20, 25, 26, 29, 40, 80} {
		for _, sess := range []tmux.Session{
			{Name: "x", Windows: 1, Attached: false},
			{Name: "agentic-workflows-code-based-that-is-quite-long", Windows: 12, Attached: true},
		} {
			items := flatItems(sess)
			for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
				out := renderRow(SessionDelegate{Theme: th}, w, items, 0, 0)
				if got := lipgloss.Width(out); got > w {
					t.Errorf("[w=%d %v %q] row width = %d, overflows the list width %d", w, themeLabel(th), sess.Name, got, w)
				}
			}
		}
	}
}

func TestSessionRow_FlatIsNameOnly(t *testing.T) {
	items := flatItems(tmux.Session{
		Name:     "flat-name",
		Windows:  2,
		Attached: false,
		Dir:      "/home/user/code/some-project",
	})
	out := renderRow(SessionDelegate{}, 80, items, 0, 0)
	vis := ansi.Strip(out)

	if !strings.Contains(vis, "flat-name") {
		t.Errorf("flat row missing the name: %q", vis)
	}
	if strings.Contains(vis, "/home/user") || strings.Contains(vis, "some-project") {
		t.Errorf("flat row leaked the directory/path column: %q", vis)
	}
}

func TestSessionRow_NoRawAnsiColourLiterals(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := SessionDelegate{Theme: th}
		items := flatItems(tmux.Session{Name: "alpha", Windows: 3, Attached: true})
		out := renderRow(d, 80, items, 0, 0)

		for _, banned := range []string{"38;5;212", "38;5;76", "48;5;212", "48;5;76"} {
			if strings.Contains(out, banned) {
				t.Errorf("[%v] delegate emitted a legacy ANSI-256 colour sequence %q: %q", themeLabel(th), banned, escSeq(out))
			}
		}
		if strings.Contains(out, "38;2;119;119;119") {
			t.Errorf("[%v] delegate emitted the legacy #777777 grey: %q", themeLabel(th), escSeq(out))
		}
	}
}

func TestSessionRow_HeightStaysOne(t *testing.T) {
	d := SessionDelegate{}
	if d.Height() != 1 {
		t.Fatalf("Height() = %d, want 1", d.Height())
	}
	items := flatItems(tmux.Session{Name: "alpha", Windows: 3, Attached: true})
	out := renderRow(d, 80, items, 0, 0)
	if strings.Contains(out, "\n") {
		t.Errorf("session row emitted more than one line: %q", out)
	}
}

func lineHasBgParams(line, params string) bool {
	for _, c := range scanCellBackgrounds(line) {
		if c.set && c.params == params {
			return true
		}
	}
	return false
}

func escSeq(s string) string { return strings.ReplaceAll(s, "\x1b", "\\e") }
