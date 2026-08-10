package capture_test

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// How a rendered frame's slide-over is read: one frame→lines projection, one
// statement of a row's shape, and the lookups built from the two.
//
// Every assertion about a panel reads the RENDERED frame rather than model
// internals, because internal/tui exports no panel accessor — and because the
// frame is what the tapes screenshot, so a divergence between the two would be
// invisible.

// panelBorder is the panel layout's left-border-only glyph, the one column that
// marks where the slide-over starts on every row of the frame. It is a literal
// here because internal/tui keeps its own copy unexported; the assertions that
// read a panel are what notice if the two ever disagree (the panel would then be
// unfindable).
const panelBorder = "│"

// panelCursorBar is the glyph the cursor column paints on the row the cursor
// rests on.
const panelCursorBar = "▌"

// panelFixtureNamePrefix is what every panel fixture's registered name begins
// with, and the whole of what distinguishes one from the picker fixtures beside
// it in the registry.
const panelFixtureNamePrefix = "theme-panel-"

// panelLine is one rendered line of the slide-over: the panel side of a frame
// line, with and without its SGR runs.
type panelLine struct {
	visible string
	raw     string
}

// fields is the line's text past the cursor column, tokenised, and whether that
// column is painted.
//
// theme_row.go composes a row as `[2-cell cursor column][label][pad][badge]`, so
// a leading `▌` is dropped and the first field that survives is the label.
func (l panelLine) fields() (fields []string, cursor bool) {
	fields = strings.Fields(l.visible)
	if len(fields) > 0 && fields[0] == panelCursorBar {
		return fields[1:], true
	}
	return fields, false
}

// panelLines is every line of the rendered slide-over that carries its left
// border, top to bottom — the panel's own rows below the header rule.
//
// It keeps the panel side of each line rather than the whole line because the
// page behind it renders its own rows: a frame-wide scan for a slug would find
// the session list's text as readily as the panel's.
//
// Every lookup matches on `visible`; `raw` is retained beside it for the
// assertions that read which tokens painted a line.
func panelLines(t *testing.T, frame string) []panelLine {
	t.Helper()

	var lines []panelLine
	for line := range strings.SplitSeq(frame, "\n") {
		_, panel, onPanel := strings.Cut(line, panelBorder)
		if !onPanel {
			continue
		}
		lines = append(lines, panelLine{visible: strings.TrimRight(ansi.Strip(panel), " "), raw: panel})
	}
	if len(lines) == 0 {
		t.Fatalf("no frame line carries the panel's left border %q, so the panel did not render:\n%s", panelBorder, ansi.Strip(frame))
	}
	return lines
}

// panelText is the whole rendered slide-over as plain text, for a failure message
// that shows the frame the assertion was reading.
func panelText(lines []panelLine) string {
	visible := make([]string, 0, len(lines))
	for _, line := range lines {
		visible = append(visible, line.visible)
	}
	return strings.Join(visible, "\n")
}

// panelFieldText is the same frame with each line's runs of spaces collapsed — the
// form a footer row is SEARCHED for, since the fixed key column pads every glyph
// out and a literal `esc close` therefore appears nowhere in the rendered text. A
// search against the uncollapsed form would pass whatever the footer said.
func panelFieldText(lines []panelLine) string {
	fields := make([]string, 0, len(lines))
	for _, line := range lines {
		fields = append(fields, strings.Join(strings.Fields(line.visible), " "))
	}
	return strings.Join(fields, "\n")
}

// panelLineIndex is the index within a panelLines slice of the ONE line whose
// visible text carries want — fataling when there is none or more than one.
func panelLineIndex(t *testing.T, lines []panelLine, want string) int {
	t.Helper()

	found := -1
	for i, line := range lines {
		if !strings.Contains(line.visible, want) {
			continue
		}
		if found >= 0 {
			t.Fatalf("%q renders on panel lines %d AND %d; the assertions below locate blocks by position, which two matches make meaningless", want, found, i)
		}
		found = i
	}
	if found < 0 {
		t.Fatalf("no panel line carries %q:\n%s", want, panelText(lines))
	}
	return found
}

// uniquePanelLine tests every panel line against the predicate, requires exactly
// one match, and returns its position and its un-stripped text.
func uniquePanelLine(t *testing.T, frame, subject string, matches func(line panelLine) bool) (index int, raw string) {
	t.Helper()

	found := -1
	for i, line := range panelLines(t, frame) {
		if !matches(line) {
			continue
		}
		if found >= 0 {
			t.Fatalf("%s renders on panel lines %d AND %d; §9.5 puts every row on exactly one delegate line:\n%s", subject, found, i, ansi.Strip(frame))
		}
		found, raw = i, line.raw
	}
	if found < 0 {
		t.Fatalf("no panel line carries %s:\n%s", subject, ansi.Strip(frame))
	}
	return found, raw
}

// panelLineWith is the ONE panel line whose visible text carries want, with its
// position — fataling when there is none or more than one.
//
// It matches on SUBSTRING, so it is for the panel's CHROME — the header label, the
// pinned directory row, the paginator — and never for a row label: `tokyo-night` is
// a substring of `tokyo-night-day`, so a row lookup goes through panelRowLine.
func panelLineWith(t *testing.T, frame, want string) (index int, raw string) {
	t.Helper()
	return uniquePanelLine(t, frame, want, func(line panelLine) bool {
		return strings.Contains(line.visible, want)
	})
}

// panelRowLine is the ONE panel line whose LABEL is exactly label — the row
// lookup, matching on the label rather than on a substring.
//
// THE UNIQUENESS IS AN ASSERTION, not a convenience. The row-rendering rule's row invariant is
// that a row NEVER WRAPS — every list row is exactly one delegate line, which is what
// `bubbles/list` pagination, the invalid-row skip and the geometry rule's paging all rest on —
// so a label found on two lines is that invariant broken.
func panelRowLine(t *testing.T, frame, label string) (index int, raw string) {
	t.Helper()
	return uniquePanelLine(t, frame, "the row "+label, func(line panelLine) bool {
		fields, _ := line.fields()
		return len(fields) > 0 && fields[0] == label
	})
}

// panelRow is one parsed row of the rendered slide-over: whether the cursor bar
// is on it, its label, and the badge text to its right.
type panelRow struct {
	cursor bool
	label  string
	badge  string
}

// panelRows parses the slide-over out of a rendered frame, keyed by label.
//
// It parses the whole panel rather than only its list, so the header row and the
// footer rows are keyed too — which is what lets an assertion state that the
// `Themes` header is on the frame without a second parser.
func panelRows(t *testing.T, frame string) map[string]panelRow {
	t.Helper()

	rows := make(map[string]panelRow)
	for _, line := range panelLines(t, frame) {
		fields, cursor := line.fields()
		if len(fields) == 0 {
			continue
		}
		rows[fields[0]] = panelRow{cursor: cursor, label: fields[0], badge: strings.Join(fields[1:], " ")}
	}
	if len(rows) == 0 {
		t.Fatalf("no panel rows were parsed out of the frame; the slide-over did not render:\n%s", ansi.Strip(frame))
	}
	return rows
}
