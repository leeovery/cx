package tui

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

var (
	themeRowTestPreferredWidth = themePanelInnerWidth(themePanelPreferredWidth)
	themeRowTestMinWidth       = themePanelInnerWidth(themePanelMinWidth)
)

func renderThemeRow(d themeRowDelegate, items []list.Item, index, selIndex int) string {
	m := list.New(items, d, max(d.Width, 1), 10)
	m.Select(selIndex)
	var buf bytes.Buffer
	d.Render(&buf, m, index, items[index])
	return buf.String()
}

// The second, never-rendered row parks the cursor off the row under test, so
// that row carries no selection treatment.
func renderOneThemeRow(d themeRowDelegate, it themeRowItem) string {
	items := []list.Item{it, themeRowItem{Row: theme.Row{Slug: "cursor-parking"}}}
	return renderThemeRow(d, items, 0, 1)
}

func validThemeRow(slug string) theme.Row {
	return theme.Row{Slug: slug, Source: theme.SourceBuiltin}
}

func invalidThemeRow(slug string, reason theme.Reason) theme.Row {
	return theme.Row{
		Slug:      slug,
		Filename:  slug + ".theme",
		Source:    theme.SourceFile,
		Rejection: &theme.Rejection{Reason: reason},
	}
}

func visibleThemeRow(out string) string { return ansi.Strip(out) }

// Returns the text a given SGR sequence painted: that the sequence appears at
// all says nothing about which text carries it.
func themeRowRunAfter(t *testing.T, out, params string) string {
	t.Helper()
	at := strings.Index(out, params)
	if at < 0 {
		t.Fatalf("row carries no run with SGR params %q: %q", params, escSeq(out))
	}
	rest := out[at:]
	end := strings.IndexByte(rest, 'm')
	if end < 0 {
		t.Fatalf("SGR params %q are unterminated: %q", params, escSeq(out))
	}
	run := rest[end+1:]
	if next := strings.IndexByte(run, '\x1b'); next >= 0 {
		run = run[:next]
	}
	return run
}

func TestThemeRow_AlwaysOneDelegateLine(t *testing.T) {
	th := testDarkTheme(t)
	rows := map[string]theme.Row{
		"valid":   validThemeRow("nord"),
		"invalid": invalidThemeRow("nord", theme.ReasonMissingTokens),
		"valid long label": validThemeRow(
			"a-very-long-user-slug-that-cannot-possibly-fit-the-panel"),
		"invalid long label": invalidThemeRow(
			"a-very-long-user-slug-that-cannot-possibly-fit-the-panel", theme.ReasonBadColour),
	}
	badges := map[string]theme.Badge{
		"no badge":   theme.BadgeNone,
		"bare badge": theme.BadgeConstant,
		"slot badge": theme.BadgeLight,
		"both badge": theme.BadgeBoth,
	}

	for _, width := range []int{themeRowTestPreferredWidth, themeRowTestMinWidth} {
		for rowName, row := range rows {
			for badgeName, badge := range badges {
				for _, selected := range []bool{false, true} {
					name := fmt.Sprintf("%s/%s/w=%d/selected=%v", rowName, badgeName, width, selected)
					t.Run(name, func(t *testing.T) {
						d := themeRowDelegate{Theme: th, Width: width}
						items := []list.Item{themeRowItem{Row: row, Badge: badge}}
						selIndex := -1
						if selected {
							selIndex = 0
						}
						out := renderThemeRow(d, items, 0, selIndex)

						if got := lipgloss.Height(out); got != 1 {
							t.Errorf("[w=%d selected=%v] row height = %d, want exactly 1: %q", width, selected, got, out)
						}
					})
				}
			}
		}
	}
}

func TestThemeRow_InvalidAlwaysCarriesTheGlyph(t *testing.T) {
	th := testDarkTheme(t)
	longSlug := "a-very-long-user-slug-that-cannot-possibly-fit-the-panel"

	for _, width := range []int{themeRowTestPreferredWidth, themeRowTestMinWidth} {
		for _, tc := range []struct {
			name  string
			row   theme.Row
			badge theme.Badge
		}{
			{name: "no badge", row: invalidThemeRow("nord", theme.ReasonMissingTokens)},
			{name: "with badge", row: invalidThemeRow("nord", theme.ReasonNotFound), badge: theme.BadgeDark},
			{name: "long label", row: invalidThemeRow(longSlug, theme.ReasonBadColour)},
			{name: "long label with badge", row: invalidThemeRow(longSlug, theme.ReasonBadColour), badge: theme.BadgeBoth},
		} {
			t.Run(tc.name, func(t *testing.T) {
				d := themeRowDelegate{Theme: th, Width: width}
				vis := visibleThemeRow(renderOneThemeRow(d, themeRowItem{Row: tc.row, Badge: tc.badge}))
				if !strings.Contains(vis, flashWarningGlyph) {
					t.Errorf("[w=%d] invalid row dropped the ⚠ invalidity signal: %q", width, vis)
				}
			})
		}
	}

	d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
	valid := visibleThemeRow(renderOneThemeRow(d, themeRowItem{Row: validThemeRow("nord"), Badge: theme.BadgeConstant}))
	if strings.Contains(valid, flashWarningGlyph) {
		t.Errorf("valid row rendered the ⚠ invalidity signal: %q", valid)
	}
}

func TestThemeRow_ReasonLabelsAreTheSevenTerseStrings(t *testing.T) {
	th := testDarkTheme(t)
	reasons := []theme.Reason{
		theme.ReasonMissingTokens,
		theme.ReasonBadColour,
		theme.ReasonBadSyntax,
		theme.ReasonBadName,
		theme.ReasonReservedName,
		theme.ReasonUnreadable,
		theme.ReasonNotFound,
	}
	if len(reasons) != 7 {
		t.Fatalf("the table covers %d reasons, want all 7 of the rejection vocabulary", len(reasons))
	}

	for _, reason := range reasons {
		t.Run(string(reason), func(t *testing.T) {
			d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
			row := theme.Row{Slug: "x", Source: theme.SourceFile, Rejection: &theme.Rejection{Reason: reason}}
			vis := visibleThemeRow(renderOneThemeRow(d, themeRowItem{Row: row}))

			want := flashWarningGlyph + " " + string(reason)
			if !strings.Contains(vis, want) {
				t.Errorf("row does not carry the terse reason %q: %q", want, vis)
			}
		})
	}
}

func TestThemePanelBadgeText_RendersTheFourBadges(t *testing.T) {
	tests := []struct {
		name  string
		badge theme.Badge
		want  string
	}{
		{name: "a constant", badge: theme.BadgeConstant, want: "●"},
		{name: "the light slot", badge: theme.BadgeLight, want: "● light"},
		{name: "the dark slot", badge: theme.BadgeDark, want: "● dark"},
		{name: "one slug in both slots", badge: theme.BadgeBoth, want: "● both"},
		{name: "no badge", badge: theme.BadgeNone, want: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := themePanelBadgeText(tt.badge); got != tt.want {
				t.Errorf("themePanelBadgeText(%v) = %q, want %q", tt.badge, got, tt.want)
			}
		})
	}
}

// Measured with lipgloss rather than len: these are multi-byte glyphs and the
// truncation budget is counted in terminal cells.
func TestThemePanelBadgeText_BothIsNoWiderThanLight(t *testing.T) {
	both := lipgloss.Width(themePanelBadgeText(theme.BadgeBoth))
	for _, badge := range []theme.Badge{theme.BadgeLight, theme.BadgeDark} {
		if got := lipgloss.Width(themePanelBadgeText(badge)); both > got {
			t.Errorf("%q is %d cells wide, wider than %q at %d — the collapsed badge must not move the truncation budget", themePanelBadgeText(theme.BadgeBoth), both, themePanelBadgeText(badge), got)
		}
	}
}

func TestThemeRow_ReasonIsDroppedBeforeBadge(t *testing.T) {
	th := testDarkTheme(t)
	row := invalidThemeRow("nord", theme.ReasonBadColour)
	reason := string(theme.ReasonBadColour)

	d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
	bare := visibleThemeRow(renderOneThemeRow(d, themeRowItem{Row: row}))
	if !strings.Contains(bare, reason) {
		t.Errorf("unbadged invalid row dropped its terse reason %q: %q", reason, bare)
	}

	badged := visibleThemeRow(renderOneThemeRow(d, themeRowItem{Row: row, Badge: theme.BadgeDark}))
	if strings.Contains(badged, reason) {
		t.Errorf("badged invalid row kept its terse reason %q — the badge outranks it: %q", reason, badged)
	}
	if !strings.Contains(badged, flashWarningGlyph) {
		t.Errorf("badged invalid row dropped the ⚠ invalidity signal: %q", badged)
	}
	if !strings.Contains(badged, row.Label()) {
		t.Errorf("badged invalid row dropped its label %q: %q", row.Label(), badged)
	}
	if !strings.HasSuffix(badged, themePanelBadgeText(theme.BadgeDark)) {
		t.Errorf("badge %q is not right-aligned to the row's edge: %q", themePanelBadgeText(theme.BadgeDark), badged)
	}
}

func TestThemeRow_ShortLabelDropsTheReasonRatherThanOverflow(t *testing.T) {
	th := testDarkTheme(t)
	reason := string(theme.ReasonMissingTokens)

	d := themeRowDelegate{Theme: th, Width: themeRowTestMinWidth}
	out := renderOneThemeRow(d, themeRowItem{Row: invalidThemeRow("ab", theme.ReasonMissingTokens)})
	vis := visibleThemeRow(out)

	if got := lipgloss.Width(out); got != themeRowTestMinWidth {
		t.Errorf("the row renders %d cells at the panel's inner minimum of %d: %q", got, themeRowTestMinWidth, vis)
	}
	if strings.Contains(vis, reason) {
		t.Errorf("the row kept its terse reason %q at a width that cannot hold it and the label's floor together: %q", reason, vis)
	}
	if !strings.Contains(vis, flashWarningGlyph) {
		t.Errorf("the row dropped the ⚠ invalidity signal, which outranks the reason: %q", vis)
	}
	if !strings.Contains(vis, "ab") {
		t.Errorf("the row dropped its label: %q", vis)
	}
}

func TestThemeRow_LabelTruncationFloor(t *testing.T) {
	th := testDarkTheme(t)
	longSlug := "a-very-long-user-slug-that-cannot-possibly-fit-the-panel"

	d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
	vis := visibleThemeRow(renderOneThemeRow(d, themeRowItem{Row: validThemeRow(longSlug)}))
	if strings.Contains(vis, longSlug) {
		t.Errorf("an over-long label must be truncated, but rendered in full: %q", vis)
	}
	if !strings.Contains(vis, themeRowEllipsis) {
		t.Errorf("a truncated label must carry the ellipsis: %q", vis)
	}

	// Squeezed below the floor: the `⚠` and widest badge together claim more than
	// the width holds.
	squeezed := themeRowDelegate{Theme: th, Width: 8}
	row := invalidThemeRow("abcdefghij", theme.ReasonBadSyntax)
	vis = visibleThemeRow(renderOneThemeRow(squeezed, themeRowItem{Row: row, Badge: theme.BadgeLight}))
	if want := "abc" + themeRowEllipsis; !strings.Contains(vis, want) {
		t.Errorf("label fell below the three-characters-plus-ellipsis floor, want %q in: %q", want, vis)
	}
}

// text.faint is floored below the UI contrast threshold, so it can never carry
// content.
func TestThemeRow_InvalidLabelIsSubtleWarningIsAttention(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		subtle := tokenFgSeq(t, th.TextSubtle)
		attention := tokenFgSeq(t, th.AccentAttention)
		if subtle == attention {
			t.Fatalf("[%v] test precondition broken: text.subtle == accent.attention", themeLabel(th))
		}

		d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
		row := invalidThemeRow("nord", theme.ReasonBadColour)
		out := renderOneThemeRow(d, themeRowItem{Row: row})

		if got := themeRowRunAfter(t, out, subtle); !strings.Contains(got, row.Label()) {
			t.Errorf("[%v] the text.subtle run holds %q, want the label %q", themeLabel(th), got, row.Label())
		}
		want := flashWarningGlyph + " " + string(theme.ReasonBadColour)
		if got := themeRowRunAfter(t, out, attention); !strings.Contains(got, want) {
			t.Errorf("[%v] the accent.attention run holds %q, want %q", themeLabel(th), got, want)
		}
		if seq := tokenFgSeq(t, th.TextPrimary); strings.Contains(out, seq) {
			t.Errorf("[%v] invalid row painted a run in text.primary %q: %q", themeLabel(th), seq, escSeq(out))
		}
	}
}

func TestThemeRow_NeverUsesTextFaint(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		faint := tokenFgSeq(t, th.TextFaint)
		for _, tc := range []struct {
			name  string
			row   theme.Row
			badge theme.Badge
		}{
			{name: "valid", row: validThemeRow("nord")},
			{name: "valid badged", row: validThemeRow("nord"), badge: theme.BadgeBoth},
			{name: "invalid", row: invalidThemeRow("nord", theme.ReasonMissingTokens)},
			{name: "invalid badged", row: invalidThemeRow("nord", theme.ReasonUnreadable), badge: theme.BadgeConstant},
		} {
			t.Run(themeLabel(th)+"/"+tc.name, func(t *testing.T) {
				d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
				items := []list.Item{themeRowItem{Row: tc.row, Badge: tc.badge}}
				for _, selIndex := range []int{-1, 0} {
					out := renderThemeRow(d, items, 0, selIndex)
					if strings.Contains(out, faint) {
						t.Errorf("row painted a run in text.faint %q: %q", faint, escSeq(out))
					}
				}
			})
		}
	}
}

// A `bad name` row has no slug at all, and a `reserved name` row's slug is
// identical to the built-in's it collides with.
func TestThemeRow_FilenameLabelledRows(t *testing.T) {
	th := testDarkTheme(t)
	for _, tc := range []struct {
		name string
		row  theme.Row
	}{
		{
			name: "bad name",
			row: theme.Row{
				Filename:  "Nord.Theme",
				Source:    theme.SourceFile,
				Rejection: &theme.Rejection{Reason: theme.ReasonBadName},
			},
		},
		{
			name: "reserved name",
			row: theme.Row{
				Slug:      "nord",
				Filename:  "nord.theme",
				Source:    theme.SourceFile,
				Rejection: &theme.Rejection{Reason: theme.ReasonReservedName},
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.row.Label() != tc.row.Filename {
				t.Fatalf("test precondition broken: Label() = %q, want the filename %q", tc.row.Label(), tc.row.Filename)
			}
			d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
			out := renderOneThemeRow(d, themeRowItem{Row: tc.row})

			if got := themeRowRunAfter(t, out, tokenFgSeq(t, th.TextSubtle)); got != tc.row.Filename {
				t.Errorf("row label = %q, want the filename %q", got, tc.row.Filename)
			}
		})
	}
}

// Badges are looked up through theme.Row.BadgeKey, never Slug: that is what
// keeps a `reserved name` row off the `●` of the slug it collides with.
func themeRowItemsFor(union theme.Union, badges map[string]theme.Badge) []list.Item {
	items := make([]list.Item, 0, len(union.Rows))
	for _, row := range union.Rows {
		items = append(items, themeRowItem{Row: row, Badge: badges[row.BadgeKey()]})
	}
	return items
}

func TestThemeRow_ReservedNameRowCarriesNoBadge(t *testing.T) {
	th := testDarkTheme(t)
	dir := t.TempDir()
	themetest.Write(t, dir, "nord.theme", themetest.Lines())

	_, union := theme.Assembler{Loader: theme.NewSilentLoader()}.Open(dir, theme.RawKeys{Theme: "nord"})
	badges := theme.Badges([]theme.SlotResolution{{Slot: theme.SlotConstant, Requested: "nord", Resolved: "nord"}})
	items := themeRowItemsFor(union, badges)

	builtinAt, fileAt := -1, -1
	for i, row := range union.Rows {
		switch {
		case row.Source == theme.SourceBuiltin && row.Slug == "nord":
			builtinAt = i
		case row.Rejection != nil && row.Rejection.Reason == theme.ReasonReservedName:
			fileAt = i
		}
	}
	if builtinAt < 0 || fileAt < 0 {
		t.Fatalf("union does not hold both rows: builtin at %d, reserved-name file at %d", builtinAt, fileAt)
	}
	if fileAt != builtinAt+1 {
		t.Fatalf("the collided rows are not adjacent: builtin at %d, file at %d", builtinAt, fileAt)
	}

	d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
	builtin := visibleThemeRow(renderThemeRow(d, items, builtinAt, -1))
	if !strings.Contains(builtin, themePanelBadgeText(theme.BadgeConstant)) {
		t.Errorf("the built-in row lost its ● badge — the persisted slug resolved to it: %q", builtin)
	}

	file := visibleThemeRow(renderThemeRow(d, items, fileAt, -1))
	if strings.Contains(file, themePanelBadgeText(theme.BadgeConstant)) {
		t.Errorf("the reserved-name row carries a ● badge; only the built-in it collides with may: %q", file)
	}
	if !strings.Contains(file, "nord.theme") {
		t.Errorf("the reserved-name row is not labelled by its filename: %q", file)
	}
}

func TestThemeRow_BuiltinRendersLikeADropIn(t *testing.T) {
	th := testDarkTheme(t)
	builtin := theme.Row{Slug: "alpha", Source: theme.SourceBuiltin, Theme: th}
	dropIn := theme.Row{
		Slug:     "alpha",
		Filename: "alpha.theme",
		Source:   theme.SourceFile,
		Theme:    testLightTheme(t),
	}

	plain := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
	out := renderOneThemeRow(plain, themeRowItem{Row: builtin})
	if got := themeRowRunAfter(t, out, tokenFgSeq(t, th.TextPrimary)); got != builtin.Label() {
		t.Errorf("the text.primary run holds %q, want a valid row's label %q", got, builtin.Label())
	}

	for _, badge := range []theme.Badge{theme.BadgeNone, theme.BadgeLight} {
		d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
		items := []list.Item{themeRowItem{Row: builtin, Badge: badge}, themeRowItem{Row: dropIn, Badge: badge}}

		for _, tc := range []struct {
			name            string
			builtinCursorAt int
			dropInCursorAt  int
		}{
			{name: "unselected", builtinCursorAt: -1, dropInCursorAt: -1},
			{name: "selected", builtinCursorAt: 0, dropInCursorAt: 1},
		} {
			a := renderThemeRow(d, items, 0, tc.builtinCursorAt)
			b := renderThemeRow(d, items, 1, tc.dropInCursorAt)
			if a != b {
				t.Errorf("built-in and drop-in rows differ (badge %v, %s):\n built-in %q\n drop-in  %q",
					badge, tc.name, escSeq(a), escSeq(b))
			}
		}
	}
}

func TestThemeRow_CursorRowSelectionTreatment(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
		items := []list.Item{themeRowItem{Row: validThemeRow("nord"), Badge: theme.BadgeDark}}
		out := renderThemeRow(d, items, 0, 0)

		if !strings.Contains(visibleThemeRow(out), selectorBar) {
			t.Errorf("[%v] cursor row missing the ▌ selector bar: %q", themeLabel(th), visibleThemeRow(out))
		}
		if seq := tokenFgSeq(t, th.AccentPrimary); !strings.Contains(out, seq) {
			t.Errorf("[%v] cursor row's ▌ missing the accent.primary fg %q: %q", themeLabel(th), seq, escSeq(out))
		}
		if got := themeRowRunAfter(t, out, tokenFgSeq(t, th.TextOnSelection)); got != "nord" {
			t.Errorf("[%v] the text.on-selection run holds %q, want the label %q", themeLabel(th), got, "nord")
		}

		params := selectionBgParams(t, th)
		cells := scanCellBackgrounds(out)
		if len(cells) != themeRowTestPreferredWidth {
			t.Errorf("[%v] cursor row spans %d cells, want the full width %d", themeLabel(th), len(cells), themeRowTestPreferredWidth)
		}
		for i, c := range cells {
			if !c.set || c.params != params {
				t.Errorf("[%v] cell %d is not tinted with bg.selection (%q): %q", themeLabel(th), i, params, escSeq(out))
				break
			}
		}
	}
}

// The return leg is what a cached run would fail: it would make the third
// render echo the second.
func TestThemeRow_NoCachedStyles(t *testing.T) {
	dark, light := testDarkTheme(t), testLightTheme(t)
	items := []list.Item{themeRowItem{Row: invalidThemeRow("nord", theme.ReasonBadColour)}}

	first := renderThemeRow(themeRowDelegate{Theme: dark, Width: themeRowTestPreferredWidth}, items, 0, 0)
	swapped := renderThemeRow(themeRowDelegate{Theme: light, Width: themeRowTestPreferredWidth}, items, 0, 0)
	back := renderThemeRow(themeRowDelegate{Theme: dark, Width: themeRowTestPreferredWidth}, items, 0, 0)

	if first == swapped {
		t.Errorf("the row renders identically under two themes — the delegate is not re-deriving: %q", escSeq(first))
	}
	if visibleThemeRow(first) != visibleThemeRow(swapped) {
		t.Errorf("swapping the theme changed the row's TEXT, want colour only:\n %q\n %q",
			visibleThemeRow(first), visibleThemeRow(swapped))
	}
	if first != back {
		t.Errorf("returning to the first theme did not reproduce its frame — a style was cached:\n %q\n %q",
			escSeq(first), escSeq(back))
	}
}

func TestThemeRow_ColourlessIsGlyphBacked(t *testing.T) {
	d := themeRowDelegate{Theme: testDarkTheme(t), Colourless: true, Width: themeRowTestPreferredWidth}
	items := []list.Item{themeRowItem{
		Row:   invalidThemeRow("nord", theme.ReasonMissingTokens),
		Badge: theme.BadgeDark,
	}}
	out := renderThemeRow(d, items, 0, 0)

	assertThemeRowHasNoColour(t, out)
	for _, glyph := range []string{flashWarningGlyph, themePanelBadgeDark, selectorBar} {
		if !strings.Contains(out, glyph) {
			t.Errorf("a colourless row dropped the %q glyph: %q", glyph, out)
		}
	}
}

// Walks the SGR parameters rather than banning escape sequences outright, so
// the bold the carve-out keeps is not mistaken for colour.
func assertThemeRowHasNoColour(t *testing.T, out string) {
	t.Helper()
	for _, params := range themeRowSGRRuns(out) {
		for _, p := range params {
			if p != "1" {
				t.Errorf("a colourless row emitted the non-bold SGR parameter %q: %q", p, escSeq(out))
			}
		}
	}
}

func themeRowSGRRuns(out string) [][]string {
	var runs [][]string
	rest := out
	for {
		at := strings.Index(rest, "\x1b[")
		if at < 0 {
			return runs
		}
		rest = rest[at:]
		end := strings.IndexByte(rest, 'm')
		if end < 0 {
			return runs
		}
		runs = append(runs, splitSGRParams(rest[:end+1]))
		rest = rest[end+1:]
	}
}

func themeRowOpeningParams(t *testing.T, out, text string) []string {
	t.Helper()
	at := strings.Index(out, text)
	if at < 0 {
		t.Fatalf("row holds no run rendering %q: %q", text, escSeq(out))
	}
	start := strings.LastIndex(out[:at], "\x1b[")
	if start < 0 {
		t.Fatalf("the run rendering %q opened with no SGR sequence: %q", text, escSeq(out))
	}
	seq := out[start:]
	end := strings.IndexByte(seq, 'm')
	if end < 0 {
		t.Fatalf("the SGR sequence opening %q is unterminated: %q", text, escSeq(out))
	}
	return splitSGRParams(seq[:end+1])
}

// Steps over truecolor and indexed colour operands before testing for the bold
// attribute: `38;2;<r>;<g>;<b>` puts three arbitrary channel values in the
// parameter list, so a token with a channel of exactly 1 would otherwise read
// as bold.
func themeRowRunIsBold(t *testing.T, out, text string) bool {
	t.Helper()
	params := themeRowOpeningParams(t, out, text)
	for i := 0; i < len(params); i++ {
		switch params[i] {
		case "38", "48":
			switch {
			case i+1 < len(params) && params[i+1] == "2":
				i += 4
			case i+1 < len(params) && params[i+1] == "5":
				i += 2
			}
		case "1":
			return true
		}
	}
	return false
}

func TestThemeRow_CursorRowLabelIsBold(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}
		items := []list.Item{
			themeRowItem{Row: validThemeRow("nord")},
			themeRowItem{Row: validThemeRow("gruvbox")},
		}

		selected := renderThemeRow(d, items, 0, 0)
		if !themeRowRunIsBold(t, selected, "nord") {
			t.Errorf("[%v] the cursor row's label is not bold: %q", themeLabel(th), escSeq(selected))
		}

		unselected := renderThemeRow(d, items, 1, 0)
		if themeRowRunIsBold(t, unselected, "gruvbox") {
			t.Errorf("[%v] an unselected row's label is bold: %q", themeLabel(th), escSeq(unselected))
		}
	}
}

func TestThemeRow_CursorRowBoldsOnlyTheLabel(t *testing.T) {
	th := testDarkTheme(t)
	d := themeRowDelegate{Theme: th, Width: themeRowTestPreferredWidth}

	badged := []list.Item{themeRowItem{Row: validThemeRow("nord"), Badge: theme.BadgeDark}}
	out := renderThemeRow(d, badged, 0, 0)
	if themeRowRunIsBold(t, out, themePanelBadgeText(theme.BadgeDark)) {
		t.Errorf("the cursor row bolded its ● badge: %q", escSeq(out))
	}
	if themeRowRunIsBold(t, out, selectorBar) {
		t.Errorf("the cursor row bolded its ▌ selector bar: %q", escSeq(out))
	}

	invalid := []list.Item{themeRowItem{Row: invalidThemeRow("nord", theme.ReasonBadColour)}}
	out = renderThemeRow(d, invalid, 0, 0)
	if themeRowRunIsBold(t, out, flashWarningGlyph) {
		t.Errorf("the cursor row bolded its ⚠ and reason: %q", escSeq(out))
	}
}

func TestThemeRow_CursorRowLabelStyleMatchesSessionName(t *testing.T) {
	for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)} {
		for _, colourless := range []bool{false, true} {
			panel := themeRowDelegate{Theme: th, Colourless: colourless, Width: themeRowTestPreferredWidth}
			session := SessionDelegate{Theme: th, Colourless: colourless}

			it := themeRowItem{Row: validThemeRow("nord")}
			want := session.rowToken(nameBase, th.TextOnSelection, true).Render("sample")
			if got := panel.labelStyle(it, true).Render("sample"); got != want {
				t.Errorf("[%v colourless=%v] the panel's selected label style renders %q, want the Sessions selected name's %q",
					themeLabel(th), colourless, escSeq(got), escSeq(want))
			}
		}
	}
}

func TestThemeRow_ColourlessCursorRowKeepsBoldWithoutHue(t *testing.T) {
	d := themeRowDelegate{Theme: testDarkTheme(t), Colourless: true, Width: themeRowTestPreferredWidth}
	items := []list.Item{
		themeRowItem{Row: validThemeRow("nord")},
		themeRowItem{Row: validThemeRow("gruvbox")},
	}

	selected := renderThemeRow(d, items, 0, 0)
	assertThemeRowHasNoColour(t, selected)
	if !themeRowRunIsBold(t, selected, "nord") {
		t.Errorf("a colourless cursor row dropped the label's bold: %q", escSeq(selected))
	}

	unselected := renderThemeRow(d, items, 1, 0)
	assertThemeRowHasNoColour(t, unselected)
	if strings.Contains(unselected, "\x1b") {
		t.Errorf("a colourless unselected row emitted an escape sequence: %q", escSeq(unselected))
	}
}
