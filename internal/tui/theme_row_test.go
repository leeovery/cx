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

// The row-rendering rule row-composition gate. These tests pin the theme panel's one-line row
// delegate: the fixed four-element priority (cursor column → `⚠` → right-aligned
// `●` badge → truncated label → terse reason, dropped first), the panel layout token
// split that keeps an invalid row's `⚠` legible on a deliberately dimmed line,
// and the completeness risk requirement that every frame re-derives from the previewed theme.
//
// Colour roles are asserted with exact theme-resolved SGR sequences (like the
// session-row anatomy tests), so a token swap is caught rather than merely the
// presence of a glyph.
//
// No t.Parallel() — the package-level mock convention and the shared canvas
// helpers make parallelism unsafe across this package's tests.

// The two ends of the geometry rule's ~27–34 column band. The delegate is HANDED its width and
// owns neither end of the ladder (the panel declares its own constants), so
// these are the test's own representatives of the band the rows must survive.
const (
	themeRowTestPreferredWidth = 30
	themeRowTestMinWidth       = 24
)

// renderThemeRow renders one panel row through the delegate with the cursor on
// selIndex, returning the styled string the delegate emitted for index.
func renderThemeRow(d themeRowDelegate, items []list.Item, index, selIndex int) string {
	m := list.New(items, d, max(d.Width, 1), 10)
	m.Select(selIndex)
	var buf bytes.Buffer
	d.Render(&buf, m, index, items[index])
	return buf.String()
}

// renderOneThemeRow renders a single-item list's only row, with the cursor parked
// OFF it (the list holds a second, never-rendered row), so the row under test
// carries no selection treatment.
func renderOneThemeRow(d themeRowDelegate, it themeRowItem) string {
	items := []list.Item{it, themeRowItem{Row: theme.Row{Slug: "cursor-parking"}}}
	return renderThemeRow(d, items, 0, 1)
}

// validThemeRow is a selectable row: a slug, a palette, no rejection.
func validThemeRow(slug string) theme.Row {
	return theme.Row{Slug: slug, Source: theme.SourceBuiltin}
}

// invalidThemeRow is a rejected row labelled by its SLUG — every reason except
// the two rows the row-rendering rule labels by filename, which the filename-labelled test
// builds itself.
func invalidThemeRow(slug string, reason theme.Reason) theme.Row {
	return theme.Row{
		Slug:      slug,
		Filename:  slug + ".theme",
		Source:    theme.SourceFile,
		Rejection: &theme.Rejection{Reason: reason},
	}
}

// visibleThemeRow returns the row with every SGR sequence stripped — the text a
// user actually reads.
func visibleThemeRow(out string) string { return ansi.Strip(out) }

// themeRowRunAfter returns the visible text of the run opened by the first SGR
// sequence carrying params — the text between that sequence's terminating "m" and
// the next escape. It is how the row's separate colour runs are told apart:
// asserting that a token's sequence merely APPEARS somewhere on the line would not
// say WHICH text it painted.
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

// TestThemeRow_AlwaysOneDelegateLine is the row-rendering rule pagination invariant: EVERY row
// is exactly one delegate line, across every combination of valid/invalid,
// badge/no badge and short/long label, at both ends of the panel's width band.
//
// It is the invariant `bubbles/list` pagination depends on, the one the
// invalid-row arrow skip and the panel's paging both rest on, and the one Portal
// already has the scar from breaking (the in-SessionItem heading injection drew
// uncounted extra lines and scrolled the title and cursor off the top).
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
						if strings.Contains(out, "\n") {
							t.Errorf("[w=%d selected=%v] row carries a newline: %q", width, selected, out)
						}
					})
				}
			}
		}
	}
}

// TestThemeRow_InvalidAlwaysCarriesTheGlyph pins the row-rendering rule's second composition
// priority: the `⚠` is ALWAYS rendered on an invalid row. It is the invalidity
// signal, so it survives every competitor — a badge taking the right edge, a label
// long enough to swallow the row, and the panel's minimum width — and it never
// appears on a valid row.
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

// TestThemeRow_ReasonLabelsAreTheSevenTerseStrings pins the terse reason vocabulary
// as the panel renders it: the reason's own string value, VERBATIM, behind the pinned copy's
// `⚠ ` prefix. Nothing here re-words a reason, and the full detail stays in doctor.
//
// The COUNT is asserted rather than left to the test's name, because `not found`
// is the one reason the reason vocabulary keeps outside the rejection ladder: LoadFile never
// produces it, so it reaches a row only through the union
// — which makes this panel the sole surface that renders it, and this table the
// only place it is covered at all.
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
		t.Fatalf("the table covers %d reasons, want all 7 of §6.2's vocabulary", len(reasons))
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

// TestThemePanelBadgeText_RendersTheFourBadges pins the row-rendering rule's badge vocabulary
// VERBATIM, as the panel's own copy: the constant's bare `●`, a slot's `● light` /
// `● dark`, and the collapsed `● both`.
//
// An unbadged row renders NOTHING rather than a placeholder, which is what lets
// the composition rule ask for the text without first asking whether there is
// one — and it is what a map lookup for a row with no badge yields for free.
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

// TestThemePanelBadgeText_BothIsNoWiderThanLight asserts the row-rendering rule's width
// relation DIRECTLY rather than leaving it to prose: the collapsed badge is no wider than
// the widest slot badge, so it cannot move the row-composition truncation budget
// the panel's ~27–34 columns are apportioned by.
//
// The collapse is reachable in two keypresses, so a wider `● both` would steal
// columns from the label on rows users routinely produce. Width is measured with
// lipgloss rather than len, because these are multi-byte glyphs and the budget is
// counted in terminal cells.
func TestThemePanelBadgeText_BothIsNoWiderThanLight(t *testing.T) {
	both := lipgloss.Width(themePanelBadgeText(theme.BadgeBoth))
	for _, badge := range []theme.Badge{theme.BadgeLight, theme.BadgeDark} {
		if got := lipgloss.Width(themePanelBadgeText(badge)); both > got {
			t.Errorf("%q is %d cells wide, wider than %q at %d — the collapsed badge must not move the truncation budget", themePanelBadgeText(theme.BadgeBoth), both, themePanelBadgeText(badge), got)
		}
	}
}

// TestThemeRow_ReasonIsDroppedBeforeBadge pins the row-rendering rule's third priority: the
// `●` badge OUTRANKS the terse reason, because the union rule exists so the marker always has
// a home. The badge and the reason compete for the same right edge, so a badged row
// keeps the badge and drops the reason — the `⚠` still says the row is invalid and
// doctor says why.
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

// TestThemeRow_LabelTruncationFloor pins the row-rendering rule's fourth priority and its
// floor: a label longer than the cells left is truncated with `…`, and it never shrinks
// below three visible characters plus the ellipsis.
//
// The floor is where the row-rendering rule's degradation STOPS: below it the panel is already
// at the geometry rule's refuse threshold, so there is deliberately no further rule — the
// label simply stops giving columns back, and a row squeezed that hard belongs to a
// panel that would have refused to open.
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

	// Squeezed below the floor: an invalid row whose `⚠` and widest badge together
	// claim more than the width holds. The label still renders three characters plus
	// the ellipsis rather than vanishing.
	squeezed := themeRowDelegate{Theme: th, Width: 8}
	row := invalidThemeRow("abcdefghij", theme.ReasonBadSyntax)
	vis = visibleThemeRow(renderOneThemeRow(squeezed, themeRowItem{Row: row, Badge: theme.BadgeLight}))
	if want := "abc" + themeRowEllipsis; !strings.Contains(vis, want) {
		t.Errorf("label fell below the three-characters-plus-ellipsis floor, want %q in: %q", want, vis)
	}
}

// TestThemeRow_InvalidLabelIsSubtleWarningIsAttention pins the panel layout's token split on
// an invalid row, as two DISTINCT runs on the same line.
//
// The label is `text.subtle` and never `text.faint`: the contrast gate floors text.faint BELOW
// the UI threshold precisely so it can never carry content, while this label is
// the filename or slug the user must read to know which of their files is broken
// — which is the union rule's entire justification for listing invalid files at all.
//
// The `⚠` and its reason keep their OWN `accent.attention` token rather than
// inheriting the row's dimmed treatment, so the invalidity signal stays legible on
// a line that is deliberately dimmed.
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

// TestThemeRow_NeverUsesTextFaint is the negative half of the token split, across
// every row shape the panel can hold. The contrast floors bands text.faint BELOW the UI floor
// — visible but decorative-only — so a panel row, every element of which is content
// a user must read, may never reach for it.
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

// TestThemeRow_FilenameLabelledRows pins the row-rendering rule's two filename-labelled rows,
// both of them through theme.Row.Label() with NO second derivation in the delegate.
//
// A `bad name` row has no slug at all — the slug charset rule rejects rather than normalises —
// and a `reserved name` row's slug is IDENTICAL to the built-in's it collides with, so
// `nord.theme` beside `nord` tells the user which one is theirs where two rows
// reading `nord` would not.
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

// themeRowItemsFor builds the panel's items from a union and a badge table the
// way the panel does: every badge looked up through theme.Row.BadgeKey, NEVER
// through Slug — the lookup that keeps a `reserved name` row from painting a
// second `●` on the slug it collides with.
func themeRowItemsFor(union theme.Union, badges map[string]theme.Badge) []list.Item {
	items := make([]list.Item, 0, len(union.Rows))
	for _, row := range union.Rows {
		items = append(items, themeRowItem{Row: row, Badge: badges[row.BadgeKey()]})
	}
	return items
}

// TestThemeRow_ReservedNameRowCarriesNoBadge pins the union rule's one legitimate
// two-rows-for-one-slug case as it RENDERS: a `nord.theme` drop-in beside the
// `nord` built-in, with `nord` the persisted slug.
//
// The badge belongs to the built-in, which is what the persisted slug actually
// resolved to; the rejected file carries none, because its slug is identical to
// the built-in's by definition and a bare identity lookup would paint `●` on BOTH
// rows. The two rows are ADJACENT in the same union (the row-rendering rule sorts the built-in
// first), which is the whole point of the ordering: the thing the user can act on,
// immediately followed by the row explaining why their file is not it.
func TestThemeRow_ReservedNameRowCarriesNoBadge(t *testing.T) {
	th := testDarkTheme(t)
	dir := t.TempDir()
	themetest.Write(t, dir, "nord.theme", themetest.Lines())

	_, union := theme.Assembler{Loader: theme.NewLoader(nil)}.Open(dir, theme.RawKeys{Theme: "nord"})
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

// TestThemeRow_BuiltinRendersLikeADropIn pins the row-rendering rule's deliberate
// indistinguishability: a valid row is `text.primary`, and a built-in row renders
// BYTE-IDENTICALLY to a valid drop-in with the same label and badge state. A valid drop-in is
// simply selectable, sitting alphabetically among the built-ins with no visual
// distinction — theme.RowSource exists for ORDERING and nothing else, so nothing
// about it may reach a row's rendered content.
//
// The drop-in also carries a DIFFERENT palette on its own row, which must not
// reach the paint either: a row's Theme is the panel's preview material, never the
// colours the row itself is drawn in (those come from the previewed theme on the
// delegate).
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

		// Compared in BOTH selection states, each row rendered as the row the cursor is
		// on and as one it is not — the cursor row differs by treatment, never by source.
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

// TestThemeRow_CursorRowSelectionTreatment pins the panel layout's cursor row: the SHIPPED
// selection treatment — a `bg.selection` tint across the FULL row width, an
// `accent.primary` `▌`, and the label in `text.on-selection` — so the panel's list
// reads as the same kind of list as Sessions rather than as a lookalike.
//
// The tint spanning every cell is what makes it read as a band: a row with an
// unpainted tail opens a terminal-background island inside the selection.
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

// TestThemeRow_NoCachedStyles is the completeness risk's per-frame requirement made
// executable. The panel's list is the WORST case of the cached-style class — its styles are
// assigned once at open while its theme changes on EVERY arrow keypress — so the
// delegate must hold no derived style: the same item rendered under two themes
// must produce two different frames, and returning to the first theme must
// reproduce the first frame byte for byte.
//
// The second half is what a package-scope or construction-time style would fail:
// a cached run would make the third render echo the second.
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

// TestThemeRow_ColourlessIsGlyphBacked pins the NO_COLOR carve-out on a row
// the NO_COLOR panel block already blocks the panel from reaching: no canvas background, no
// hue, and the row's state carried by its GLYPHS — the `⚠` invalidity signal, the `●`
// badge and the `▌` cursor bar all survive on the terminal's native fg/bg.
func TestThemeRow_ColourlessIsGlyphBacked(t *testing.T) {
	d := themeRowDelegate{Theme: testDarkTheme(t), Colourless: true, Width: themeRowTestPreferredWidth}
	items := []list.Item{themeRowItem{
		Row:   invalidThemeRow("nord", theme.ReasonMissingTokens),
		Badge: theme.BadgeDark,
	}}
	out := renderThemeRow(d, items, 0, 0)

	if strings.Contains(out, "\x1b") {
		t.Errorf("a colourless row emitted an escape sequence: %q", escSeq(out))
	}
	for _, glyph := range []string{flashWarningGlyph, multiSelectMarker, selectorBar} {
		if !strings.Contains(out, glyph) {
			t.Errorf("a colourless row dropped the %q glyph: %q", glyph, out)
		}
	}
}
