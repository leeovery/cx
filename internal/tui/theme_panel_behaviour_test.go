package tui

import (
	"go/ast"
	"go/parser"
	"go/token"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// The theme panel's behaviour, driven END TO END THROUGH THE SEAM and observed on
// the RENDERED PANEL.
//
// WHAT THIS ADDS, STATED PLAINLY. internal/theme's own suites prove the pure
// functions — the union's one-slug-one-row rule, the comparator, the badge table.
// This suite proves the panel WIRES them: that what the seam assembled is what the
// list holds, in the order it holds it, with the badge on the row the derivation
// put it on, and that a commit re-derives all three without losing the cursor. A
// wiring mistake renders a plausible-looking panel that is quietly wrong — badges
// taken from the nomination instead of the resolution record, the union re-derived
// from the wrong keys, the cursor anchored by index — and none of those is visible
// from internal/theme's side. So every assertion here is PANEL-OBSERVABLE: rendered
// rows, the rendered cursor bar, rendered badge text, the cursor index, and the
// recorded persister calls. It re-states none of internal/theme's own unit
// assertions.
//
// THE SEAM IS FAKED WHOLESALE (§13.3). behaviourEnumerator holds a DECLARED
// theme.Enumeration and answers all four methods from internal/theme's own
// assembly over it, which reads no directory and no prefs.json — the architectural
// commitment §13.3 makes, rather than a convenience: an invalid-theme row has to be
// renderable with no themes directory in existence, and the panel's whole row
// vocabulary has no other test home.
//
// # Coverage checklist — §13.6's named set, and where each item is proven
//
//	1. §9.4's union and one-slug-one-row .... TestThemePanelBehaviour_Union (here)
//	2. §9.5's sort key rules ................ TestThemePanelBehaviour_Ordering (here)
//	3. Row composition and its floor ........ TestThemePanelBehaviour_RowComposition
//	                                          (here, at the panel's OWN minimum width;
//	                                          theme_row_test.go proves the delegate's
//	                                          own composition rules against widths of
//	                                          its own choosing)
//	4. The three badge-derivation rows ...... TestThemePanelBehaviour_Badges (here, on
//	                                          the RENDERED badge text; the derived
//	                                          item fields are pinned by
//	                                          theme_panel_cursor_test.go and
//	                                          theme_panel_commit_recompute_test.go)
//	5. §9.2's recompute + anchored cursor ... TestThemePanelBehaviour_CommitRecompute
//	                                          (here, as ONE walk over the rendered
//	                                          panel; the per-effect proofs live in
//	                                          theme_panel_commit_recompute_test.go)
//	6. The confirm's three inputs ........... TestThemePanelBehaviour_Confirm (here,
//	                                          as the CLOSURE over the panel's
//	                                          descriptor; per-key behaviour is proven
//	                                          in theme_panel_confirm_test.go)
//	7. §9.13's outstanding-failure machine .. TestThemePanelBehaviour_FailureStateMachine
//	                                          (here, as the composed transitions; the
//	                                          per-transition proofs live in
//	                                          theme_panel_commit_failure_test.go and
//	                                          theme_panel_close_report_test.go)
//
// TestThemePanelBehaviour_NoConfigAccess keeps the suite's own premise honest.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// behaviourEnumerator is §13.3's seam faked WHOLESALE: a DECLARED
// theme.Enumeration, with all four methods answered from internal/theme's own
// assembly over it.
//
// IT READS NOTHING. Enumeration is an ordinary value with exported fields, and the
// three derivations the panel asks for — Reassemble, ResolveNominationFrom and
// ResolveSlot — are pure functions of it plus the embedded built-in set. So the
// suite renders invalid rows, `not found` rows and a `reserved name` collision with
// no themes directory in existence and no prefs.json anywhere near it, which is
// exactly what §13.3 requires the seam to make possible.
//
// IT IS NOT A UNION OF DECLARED ROWS, AND THAT IS THE POINT. A hand-written row
// slice would make every membership and ordering assertion below a statement about
// the fixture. Deriving the union the way production derives it leaves the panel as
// the only thing under test: what it lists, what order it lists it in, and what it
// marks.
//
// It is LITERALLY the production adapter minus the ONE thing this suite must not
// do — the directory read: theme.DirEnumerator is embedded, so the three methods
// it does not override are production's own and a drift in the seam's contract
// fails here rather than in the wiring. Only Open is replaced, and only to swap
// the read for a declared enumeration.
type behaviourEnumerator struct {
	theme.DirEnumerator
	enumeration theme.Enumeration
}

// newBehaviourEnumerator declares an enumeration holding entries and NAMING NO
// DIRECTORY: DirPath is empty, because there was none — and so is the adapter's
// own Dir, which nothing on this path reads.
func newBehaviourEnumerator(entries []theme.Entry) *behaviourEnumerator {
	return &behaviourEnumerator{
		DirEnumerator: theme.DirEnumerator{Loader: theme.NewLoader(nil)},
		enumeration:   theme.Enumeration{Entries: entries},
	}
}

func (e *behaviourEnumerator) Open(keys theme.RawKeys) (theme.Enumeration, theme.Union) {
	return e.enumeration, e.Reassemble(e.enumeration, keys)
}

// behaviourFile is one VALID candidate the themes directory would have held: a
// slug, its own palette, no rejection.
func behaviourFile(slug string, palette int) theme.Entry {
	return theme.Entry{Filename: slug + ".theme", Slug: slug, Theme: arrowPalette(palette)}
}

// behaviourRejected is one candidate the §6.2 ladder rejected, keeping the slug its
// filename yields — every rung but the first leaves the name intact, so a broken
// file still has something to be listed under (§9.4).
func behaviourRejected(slug string, reason theme.Reason) theme.Entry {
	return theme.Entry{
		Filename:  slug + ".theme",
		Slug:      slug,
		Rejection: &theme.Rejection{Reason: reason},
	}
}

// behaviourBadName is §6.2's rung 1: a candidate whose FILENAME yields no slug at
// all, so the filename is the only thing it can be labelled or sorted by.
func behaviourBadName(filename string) theme.Entry {
	return theme.Entry{Filename: filename, Rejection: &theme.Rejection{Reason: theme.ReasonBadName}}
}

// The content region every fixture here opens at unless it says otherwise: wide
// enough for the panel's PREFERRED width and tall enough that the whole union fits
// one page, so a rendered-order assertion sees every row.
const (
	behaviourContentW = 96
	behaviourContentH = 26
)

// behaviourPanel builds a renderable Sessions model over a declared enumeration and
// declared persisted keys, and OPENS the panel through the production `t` keypress
// so every assertion covers the live dispatch.
func behaviourPanel(t *testing.T, entries []theme.Entry, keys theme.RawKeys) (Model, *fakeThemePersister) {
	t.Helper()
	return behaviourPanelAt(t, entries, keys, behaviourContentW, behaviourContentH)
}

// behaviourPanelAt is behaviourPanel at a chosen CONTENT REGION — the input the
// panel's width ladder and its page size are both functions of.
//
// The dimensions are assigned before the keypress, so the panel is sized by
// PRODUCTION at exactly the region the fixture renders at; nothing here sizes the
// panel's list.
func behaviourPanelAt(t *testing.T, entries []theme.Entry, keys theme.RawKeys, contentW, contentH int) (Model, *fakeThemePersister) {
	t.Helper()

	enumerator := newBehaviourEnumerator(entries)
	persister := &fakeThemePersister{}
	m := Build(Deps{
		Lister:          fakeLister{},
		Theme:           behaviourNomination(t, enumerator, keys),
		ThemeEnumerator: enumerator,
		ThemeKeys:       keys,
		ThemePersister:  persister,
	})
	m.termWidth, m.termHeight = geometryTerm(contentW, contentH)
	m.applySessions(closePanelSessions())
	m.applySessionListSize(m.contentWidth(), m.contentHeight())

	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("fixture: the panel did not open")
	}
	return m, persister
}

// behaviourNomination is the CONSTRUCTION-TIME nomination for these keys: the
// member that will be in force, injected as a CONSTANT.
//
// The constant shape is what skips §8.8's gate, so the model is resolved at
// construction and the frame is un-gated — the same property capturetool relies on.
// The setting's own shape still reaches the panel through the seam's Resolve, which
// is where a pair's two badges come from, so pinning the palette here costs nothing
// the assertions need.
func behaviourNomination(t *testing.T, e *behaviourEnumerator, keys theme.RawKeys) theme.Nomination {
	t.Helper()

	setting, _ := theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)
	resolution, err := e.Resolve(e.enumeration, setting)
	if err != nil {
		t.Fatalf("construction-time resolution of %+v: %v", setting, err)
	}
	inForce, ok := inForceSlot(resolution, appearanceDarkCanvas)
	if !ok {
		t.Fatalf("construction-time resolution %+v names no slot in force", resolution)
	}
	return theme.ConstantNomination(inForce.Theme)
}

// themeBadgeGlyph is §9.5's `●` marker as it is DRAWN — the one glyph every badge
// shape opens with, so counting it counts markers whichever of the four a row
// carries. It is written out here rather than read off the panel's own badge
// copy so the badge vocabulary is asserted against the spec's glyph rather than
// against itself.
const themeBadgeGlyph = "●"

// renderedPanelRow is one list row of the RENDERED panel, split into the three
// things a reader can name on it: the label, the trailing elements to its right
// (§9.5's `⚠ <reason>` phrase and its `●` badge), and whether the cursor bar is on
// it.
type renderedPanelRow struct {
	label   string
	trailer string
	cursor  bool
}

// renderedPanelRows reads the panel's list body off the RENDERED block — the rows
// as a user sees them, in the order they are drawn.
//
// It is the suite's central observation, and it is taken from the DRAWN BLOCK
// rather than from list.Items() deliberately: an item slice in the right order says
// nothing about what the delegate put on screen, and §9.5's composition, its badges
// and the cursor treatment are all render-time facts.
//
// The body's first line is fixed by the panel's own measurements — the header
// region plus the pinned directory row — and one union row is exactly one line
// (§9.5), so the body is the next len(Rows) lines. The single-page precondition is
// asserted rather than assumed: a paginated panel draws a page rather than the
// union, and every order assertion below would be reading a window.
func renderedPanelRows(t *testing.T, m Model) []renderedPanelRow {
	t.Helper()

	if got := m.themePanel.list.Paginator.TotalPages; got != 1 {
		t.Fatalf("fixture: the panel paginates over %d pages, so the drawn body is not the whole union", got)
	}
	lines := themePanelLines(renderRecomputePanel(m))
	from := themePanelHeaderRows() + themePanelDirRowHeight(m.themePanel.union.DirUnusable)

	rows := make([]renderedPanelRow, 0, len(m.themePanel.union.Rows))
	for i := range m.themePanel.union.Rows {
		if from+i >= len(lines) {
			t.Fatalf("the panel drew %d lines, want a body row at %d for union row %d", len(lines), from+i, i)
		}
		rows = append(rows, parseRenderedPanelRow(t, lines[from+i]))
	}
	return rows
}

// parseRenderedPanelRow splits one drawn body line into its label, its trailing
// elements and the cursor bar.
//
// Every panel row below the header rule opens with §9.1's border-and-gutter prefix
// and then §9.5's fixed 2-cell cursor column, so the content begins at a known
// offset and the `▌` is decided by what sits in that column. A label carries no
// whitespace — it is a slug or a filename — so the first field is the label and
// everything after it is the trailing elements, whitespace-collapsed so the pad to
// the panel's width falls away.
func parseRenderedPanelRow(t *testing.T, line string) renderedPanelRow {
	t.Helper()

	prefix := themePanelContentPrefix()
	if !strings.HasPrefix(line, prefix) {
		t.Fatalf("panel line %q does not open with the border-and-gutter prefix %q", line, prefix)
	}
	content := []rune(strings.TrimPrefix(line, prefix))
	if len(content) < leftBarColumnWidth {
		t.Fatalf("panel line %q is shorter than the %d-cell cursor column", line, leftBarColumnWidth)
	}

	fields := strings.Fields(string(content[leftBarColumnWidth:]))
	row := renderedPanelRow{cursor: string(content[0]) == selectorBar}
	if len(fields) > 0 {
		row.label, row.trailer = fields[0], strings.Join(fields[1:], " ")
	}
	return row
}

// renderedPanelLabels is the drawn body reduced to its labels, in order.
func renderedPanelLabels(t *testing.T, m Model) []string {
	t.Helper()

	rows := renderedPanelRows(t, m)
	labels := make([]string, 0, len(rows))
	for _, row := range rows {
		labels = append(labels, row.label)
	}
	return labels
}

// requireRenderedOrder fails unless the panel DRAWS exactly these labels in exactly
// this order.
func requireRenderedOrder(t *testing.T, m Model, want ...string) {
	t.Helper()

	if got := renderedPanelLabels(t, m); !slices.Equal(got, want) {
		t.Errorf("the panel draws %v, want %v", got, want)
	}
}

// renderedPanelRowFor returns the drawn row labelled label.
func renderedPanelRowFor(t *testing.T, m Model, label string) renderedPanelRow {
	t.Helper()

	rows := renderedPanelRows(t, m)
	at := slices.IndexFunc(rows, func(row renderedPanelRow) bool { return row.label == label })
	if at < 0 {
		t.Fatalf("the panel draws no row labelled %q; it draws %v", label, renderedPanelLabels(t, m))
	}
	return rows[at]
}

// requireRenderedBadge fails unless the row labelled label ENDS with the badge text
// want — or carries no `●` at all when want is empty.
//
// It reads the badge off the DRAWN row rather than off the item's field, which is
// the half a badge map cannot state: §9.5 right-aligns the badge to the row's edge,
// and a marker derived correctly but drawn on the wrong row is exactly the wiring
// mistake this suite exists to catch.
func requireRenderedBadge(t *testing.T, m Model, label string, want theme.Badge) {
	t.Helper()

	row := renderedPanelRowFor(t, m, label)
	if want == theme.BadgeNone {
		if strings.Contains(row.trailer, themeBadgeGlyph) {
			t.Errorf("the %q row draws a badge in %q, want none — the `●` marks what is SET (§9.5)", label, row.trailer)
		}
		return
	}
	if !strings.HasSuffix(row.trailer, themePanelBadgeText(want)) {
		t.Errorf("the %q row draws %q, want it to end with the right-aligned %q (§9.5)", label, row.trailer, themePanelBadgeText(want))
	}
}

// requireRenderedBadgeCount fails unless the whole drawn panel carries exactly want
// `●` glyphs — the count no per-row assertion can make, and the one that catches a
// marker painted twice.
func requireRenderedBadgeCount(t *testing.T, m Model, want int) {
	t.Helper()

	rows := renderedPanelRows(t, m)
	got := 0
	for _, row := range rows {
		got += strings.Count(row.trailer, themeBadgeGlyph)
	}
	if got != want {
		t.Errorf("the panel draws %d `●` markers, want %d; rows: %+v", got, want, rows)
	}
}

// requireRenderedCursorOn fails unless the `▌` cursor bar is drawn on the row
// labelled label, and on no other row.
func requireRenderedCursorOn(t *testing.T, m Model, label string) {
	t.Helper()

	var on []string
	for _, row := range renderedPanelRows(t, m) {
		if row.cursor {
			on = append(on, row.label)
		}
	}
	if !slices.Equal(on, []string{label}) {
		t.Errorf("the panel draws the cursor bar on %v, want it on %q alone", on, label)
	}
}

// TestThemePanelBehaviour_Union: it lists one row per slug.
//
// §9.4: "One slug is one row, always" — a persisted slug naming a built-in IS that
// built-in's row, and a persisted slug naming an existing-but-invalid file IS that
// file's row, carrying both the reason and the badge. Keying the union on file
// existence instead would mint a second `⚠ not found` row for every persisted
// built-in, which is the state the panel's most common action produces.
//
// The one legitimate exception is a `reserved name` file, whose slug is IDENTICAL
// to the built-in's it collides with by definition (§6.2) — two rows for one slug,
// and the only place that collision has an observable consequence.
//
// What the panel adds to internal/theme's own union tests is that it LISTS what was
// assembled: one drawn row per union row, with the reason and the marker landing
// where the derivation put them rather than where a second derivation in the panel
// would.
func TestThemePanelBehaviour_Union(t *testing.T) {
	t.Run("a persisted built-in slug is that built-in's row", func(t *testing.T) {
		m, _ := behaviourPanel(t, nil, theme.RawKeys{Theme: "nord"})

		requireRenderedOrder(t, m, theme.BuiltinSlugs()...)
		row := themePanelRowFor(t, m, "nord")
		if !row.Row.Selectable() {
			t.Errorf("the `nord` row is unselectable (%v); a persisted slug naming a built-in IS that built-in's row (§9.4)", row.Row.Rejection)
		}
		if row.Row.Source != theme.SourceBuiltin {
			t.Errorf("the `nord` row is a %v, want the built-in's own row rather than a minted persisted one", row.Row.Source)
		}
		requireRenderedBadge(t, m, "nord", theme.BadgeConstant)
		requireRenderedBadgeCount(t, m, 1)
		for _, drawn := range renderedPanelRows(t, m) {
			if strings.Contains(drawn.trailer, string(theme.ReasonNotFound)) {
				t.Errorf("the panel drew a %q row (%+v); the persisted slug RESOLVES, so no second row is minted for it (§9.4)", theme.ReasonNotFound, drawn)
			}
		}
	})

	t.Run("a persisted invalid file is that file's row, with both the reason and the badge", func(t *testing.T) {
		m, _ := behaviourPanel(t,
			[]theme.Entry{behaviourRejected("sunset", theme.ReasonBadColour)},
			theme.RawKeys{Theme: "sunset"})

		requireRenderedOrder(t, m, "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
		row := themePanelRowFor(t, m, "sunset")
		if row.Row.Selectable() {
			t.Error("the `sunset` row is selectable; an invalid file is present and named but not committable (§9.5)")
		}
		if got := row.Row.Rejection.Reason; got != theme.ReasonBadColour {
			t.Errorf("the `sunset` row carries reason %q, want the file's own %q — nothing re-derives it", got, theme.ReasonBadColour)
		}
		requireRenderedBadge(t, m, "sunset", theme.BadgeConstant)
		requireRenderedBadgeCount(t, m, 1)
		// §9.2: the cursor is on the FALLBACK's row, never on the persisted-but-broken
		// one, so the marker and the cursor are visibly two different signals here.
		requireRenderedCursorOn(t, m, theme.DefaultDarkSlug)
	})

	t.Run("a reserved name file is the one two-rows-for-one-slug case", func(t *testing.T) {
		m, _ := behaviourPanel(t,
			[]theme.Entry{behaviourRejected("nord", theme.ReasonReservedName)},
			theme.RawKeys{Theme: "nord"})

		requireRenderedOrder(t, m, "nord", "nord.theme", theme.DefaultDarkSlug, theme.DefaultLightSlug)
		collided := themePanelRowFor(t, m, "nord.theme")
		if got := collided.Row.Slug; got != "nord" {
			t.Errorf("the `nord.theme` row's slug is %q, want the built-in's %q — the collision IS the reason (§6.2)", got, "nord")
		}
		if collided.Row.Selectable() {
			t.Error("the `nord.theme` row is selectable; a reserved name is a rejection (§5.4)")
		}
		// The marker belongs to the BUILT-IN, which is what the persisted slug actually
		// resolved to. The panel looks badges up through Row.BadgeKey, and a bare Slug
		// lookup would paint `●` on both rows — silently, on the one install that has a
		// drop-in shadowing a built-in.
		requireRenderedBadge(t, m, "nord", theme.BadgeConstant)
		requireRenderedBadge(t, m, "nord.theme", theme.BadgeNone)
		requireRenderedBadgeCount(t, m, 1)
	})
}

// TestThemePanelBehaviour_Ordering: it orders rows and resolves the guaranteed tie.
//
// §9.5's order is applied by the ASSEMBLER, so what the panel owes is to DRAW it —
// no sort of its own, no reversal, no assembly order leaking through. The fixtures
// below interleave files among the built-ins precisely so that assembly order and
// display order are different answers.
//
// The tie between a `reserved name` row and the built-in it collides with is
// guaranteed by construction and the byte-wise leg cannot settle it, so §9.5 fixes
// the built-in first. The panel's half of that rule is its CURSOR: the anchor
// resolves an identity two rows share, and it must land on the selectable one —
// parking on the rejected file would put the cursor where the arrows, which skip
// unselectable rows, cannot return to.
func TestThemePanelBehaviour_Ordering(t *testing.T) {
	t.Run("it draws the union's order rather than the assembly's", func(t *testing.T) {
		// `aurora` sorts BEFORE the first built-in and `zebra` after the last, so
		// appending the files, prepending them, or leaving the built-ins leading all
		// produce a different list from this one.
		m, _ := behaviourPanel(t, []theme.Entry{
			behaviourFile("aurora", 0),
			behaviourRejected("zebra", theme.ReasonBadSyntax),
		}, theme.RawKeys{})

		requireRenderedOrder(t, m, "aurora", "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug, "zebra")
	})

	t.Run("it compares case-insensitively with a byte-wise tie-break", func(t *testing.T) {
		// `Zed.theme` is the case-insensitivity assertion: byte-wise alone files every
		// uppercase name ahead of every lowercase one, which would lead the whole list
		// with it instead of placing it last. The two `alpha` spellings are the
		// tie-break: their keys fold to the same string, so only the byte-wise leg
		// decides, and it puts the uppercase one first.
		m, _ := behaviourPanel(t, []theme.Entry{
			behaviourBadName("alpha.THEME"),
			behaviourBadName("Zed.theme"),
			behaviourBadName("Alpha.theme"),
		}, theme.RawKeys{})

		requireRenderedOrder(t, m,
			"Alpha.theme", "alpha.THEME", "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug, "Zed.theme")
	})

	t.Run("the guaranteed tie puts the built-in first and the cursor on it", func(t *testing.T) {
		m, _ := behaviourPanel(t,
			[]theme.Entry{behaviourRejected("nord", theme.ReasonReservedName)},
			theme.RawKeys{Theme: "nord"})

		rows := renderedPanelLabels(t, m)
		builtin := slices.Index(rows, "nord")
		collided := slices.Index(rows, "nord.theme")
		if builtin < 0 || collided != builtin+1 {
			t.Fatalf("the panel draws %v, want `nord.theme` immediately after `nord` — the valid thing first, then the row explaining why theirs is not it (§9.5)", rows)
		}
		requireRenderedCursorOn(t, m, "nord")
		if got, want := m.themePanel.list.Index(), builtin; got != want {
			t.Errorf("the cursor sits at index %d, want the built-in's %d — the anchor resolves a shared identity to the SELECTABLE row (§9.2)", got, want)
		}
	})
}

// behaviourLongSlug is longer than any label budget the panel has, so it truncates
// at both ends of §9.8's width ladder.
const behaviourLongSlug = "aurora-midnight-drifting-forever-and-ever"

// TestThemePanelBehaviour_RowComposition: it composes a row within the width
// budget.
//
// §9.5's four elements compete for the panel's columns in a FIXED priority, and the
// place that priority actually bites is the panel's own MINIMUM width — the
// narrowest frame §9.8 will open at, where the badge and the reason genuinely
// cannot both have the right edge. The row delegate's own suite pins the rules
// against widths of its own choosing; what is proven here is that the panel's real
// budget sustains them.
//
// The one-delegate-line invariant is asserted alongside, because it is what
// `bubbles/list` pagination, the invalid-row skip and §9.8's paging all rest on —
// and Portal already has the scar from breaking it elsewhere.
func TestThemePanelBehaviour_RowComposition(t *testing.T) {
	// A content region inside the ladder's minimum band: half of it is below the
	// minimum panel width, so the clamp holds the panel at exactly that minimum.
	const narrowContentW = 40

	entries := []theme.Entry{
		behaviourFile(behaviourLongSlug, 0),
		behaviourRejected("sunset", theme.ReasonBadColour),
	}
	order := []string{"nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug}

	badged, _ := behaviourPanelAt(t, entries, theme.RawKeys{Theme: "sunset"}, narrowContentW, behaviourContentH)
	if got := badged.themePanel.width; got != themePanelMinWidth {
		t.Fatalf("fixture: the panel opened %d columns wide, want the ladder's minimum %d", got, themePanelMinWidth)
	}

	t.Run("every row is exactly one line at the panel's width", func(t *testing.T) {
		block := themePanelLines(renderRecomputePanel(badged))
		from := themePanelHeaderRows()
		for i := range badged.themePanel.union.Rows {
			line := block[from+i]
			if strings.Contains(line, "\n") {
				t.Errorf("body line %d carries a newline: %q", i, line)
			}
			if got := lipgloss.Width(line); got != themePanelMinWidth {
				t.Errorf("body line %d is %d cells, want the panel's %d: %q", i, got, themePanelMinWidth, line)
			}
		}
		// One drawn line per union row, in the union's order — the invariant the two
		// assertions above are only meaningful against.
		wantOrder := append([]string{truncatedBehaviourLabel(t, badged)}, order...)
		requireRenderedOrder(t, badged, wantOrder...)
	})

	t.Run("the reason is dropped before the badge", func(t *testing.T) {
		row := renderedPanelRowFor(t, badged, "sunset")
		if !strings.Contains(row.trailer, flashWarningGlyph) {
			t.Errorf("the badged invalid row drew %q, want the `⚠` invalidity signal — it survives every competitor (§9.5)", row.trailer)
		}
		if strings.Contains(row.trailer, string(theme.ReasonBadColour)) {
			t.Errorf("the badged invalid row drew the reason in %q; the badge outranks it, because §9.4 exists so the marker always has a home", row.trailer)
		}
		requireRenderedBadge(t, badged, "sunset", theme.BadgeConstant)

		// The control: the SAME row at the SAME width with nothing persisted draws its
		// reason, so the drop above is the badge competing rather than a reason the
		// panel never had room for.
		bare, _ := behaviourPanelAt(t, entries, theme.RawKeys{}, narrowContentW, behaviourContentH)
		control := renderedPanelRowFor(t, bare, "sunset")
		if !strings.Contains(control.trailer, string(theme.ReasonBadColour)) {
			t.Errorf("the unbadged invalid row drew %q, want the terse reason %q — the comparison above proves nothing without it", control.trailer, theme.ReasonBadColour)
		}
		requireRenderedBadge(t, bare, "sunset", theme.BadgeNone)
	})

	t.Run("an over-long label truncates and holds the floor", func(t *testing.T) {
		label := truncatedBehaviourLabel(t, badged)
		if label == behaviourLongSlug {
			t.Fatalf("the label drew in full at the panel's minimum width: %q", label)
		}
		if !strings.HasSuffix(label, themeRowEllipsis) {
			t.Errorf("the truncated label %q carries no ellipsis", label)
		}
		if got := len([]rune(label)); got < themeRowLabelFloor {
			t.Errorf("the label drew %d cells, below §9.5's floor of three characters plus the ellipsis (%d)", got, themeRowLabelFloor)
		}
	})
}

// truncatedBehaviourLabel is behaviourLongSlug as the panel actually drew it —
// resolved from the drawn body rather than recomputed, so the composition
// assertions state what reached the screen rather than restating the delegate's own
// arithmetic.
func truncatedBehaviourLabel(t *testing.T, m Model) string {
	t.Helper()

	for _, row := range renderedPanelRows(t, m) {
		stem := strings.TrimSuffix(row.label, themeRowEllipsis)
		if stem != "" && strings.HasPrefix(behaviourLongSlug, stem) {
			return row.label
		}
	}
	t.Fatalf("the panel drew no row for %q; it drew %v", behaviourLongSlug, renderedPanelLabels(t, m))
	return ""
}

// TestThemePanelBehaviour_Badges: it derives every badge row.
//
// §9.5's badge table has exactly three rows, and the panel has to render all three
// from ONE rule — the slug a slot resolves to BEFORE fallback:
//
//	set and loadable  → the persisted slug
//	set but unloadable → STILL the persisted slug; the fallback's row carries none
//	never set          → the SHIPPED DEFAULT's slug
//
// The third row is the most common install there is (§8.1 leaves a fresh install
// with no prefs.json at all) and the one a persisted-slug-only rule would drop
// entirely, falsifying §9.4's whole justification for the union — that the `●`
// always has something to sit on.
//
// The assertions are on the DRAWN badge text, which is the half the derived map
// cannot state: a badge derived correctly and drawn on the wrong row reads as the
// user having set a theme they never chose.
func TestThemePanelBehaviour_Badges(t *testing.T) {
	t.Run("set and loadable badges the persisted slug", func(t *testing.T) {
		m, _ := behaviourPanel(t, nil, theme.RawKeys{Light: theme.DefaultLightSlug, Dark: "nord"})

		requireRenderedBadge(t, m, "nord", theme.BadgeDark)
		requireRenderedBadge(t, m, theme.DefaultLightSlug, theme.BadgeLight)
		requireRenderedBadge(t, m, theme.DefaultDarkSlug, theme.BadgeNone)
		requireRenderedBadgeCount(t, m, 2)
	})

	t.Run("set but unloadable still badges the persisted slug", func(t *testing.T) {
		// `gone-light` resolves to nothing, so §9.4 mints it a row and §8.5 falls the
		// light slot back to the shipped light default. The marker stays on what is
		// SET; the fallback's row carries none.
		m, _ := behaviourPanel(t, nil, theme.RawKeys{Light: "gone-light", Dark: "nord"})

		requireRenderedBadge(t, m, "gone-light", theme.BadgeLight)
		requireRenderedBadge(t, m, "nord", theme.BadgeDark)
		requireRenderedBadge(t, m, theme.DefaultLightSlug, theme.BadgeNone)
		requireRenderedBadgeCount(t, m, 2)

		fallback := renderedPanelRowFor(t, m, "gone-light")
		if !strings.Contains(fallback.trailer, flashWarningGlyph) {
			t.Errorf("the `gone-light` row drew %q, want the `⚠` — the user sees what is set AND why it is not applying (§9.4)", fallback.trailer)
		}
	})

	t.Run("a never-set slot badges the shipped default", func(t *testing.T) {
		// The virgin install: no prefs.json at all, so both slots arrive as the shipped
		// defaults and the panel opens carrying `● light` and `● dark`.
		m, _ := behaviourPanel(t, []theme.Entry{behaviourFile("aurora", 0)}, theme.RawKeys{})

		requireRenderedBadge(t, m, theme.DefaultDarkSlug, theme.BadgeDark)
		requireRenderedBadge(t, m, theme.DefaultLightSlug, theme.BadgeLight)
		requireRenderedBadge(t, m, "nord", theme.BadgeNone)
		requireRenderedBadge(t, m, "aurora", theme.BadgeNone)
		requireRenderedBadgeCount(t, m, 2)
	})
}

// TestThemePanelBehaviour_CommitRecompute: it recomputes and re-anchors after a
// commit.
//
// §9.2: "a successful commit recomputes the panel's full row set, not just the
// badges", and it "re-anchors the cursor to the previewed theme's identity, never
// to its index".
//
// This is ONE WALK rather than five cases, because the five effects are one act and
// the composition is where they interfere: the row that appears lands ABOVE the
// cursor, which is the only shape in which an index anchor and an identity anchor
// disagree, and the badge that moves is the same row's. Every assertion is made on
// the DRAWN panel, so what is proven is what the user is shown after the keypress —
// the rows, their order, the markers and the cursor bar — rather than the fields
// they were derived from.
//
// The walk goes constant → pair → constant, because §9.2 makes the row set move in
// BOTH directions: a slot commit makes the other slot live so a row APPEARS, and
// `Enter` clears both slots so that row loses its reason to exist and DISAPPEARS.
func TestThemePanelBehaviour_CommitRecompute(t *testing.T) {
	// A hand-edited prefs.json carrying all three keys, which §8.2 makes legal and
	// resolves as a CONSTANT — the one shape in which a persisted slot names
	// something the open-time union is blind to.
	keys := theme.RawKeys{Theme: "sunset", Light: "ghost", Dark: "sunset"}
	m, persister := behaviourPanel(t, []theme.Entry{behaviourFile("sunset", 0)}, keys)

	requireRenderedOrder(t, m, "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	requireRenderedCursorOn(t, m, "sunset")
	requireRenderedBadge(t, m, "sunset", theme.BadgeConstant)
	requireRenderedBadgeCount(t, m, 1)
	previewed := m.activeTheme
	before := m.themePanel.list.Index()

	// `d` over a constant asks first (§9.2), and `y` performs the write.
	m, _ = pressSlotKey(t, m, slotDarkPress)
	if !m.themePanel.confirming() {
		t.Fatal("`d` over a constant did not raise the confirm, so the commit below is not the one §9.2 gates")
	}
	m, _ = pressConfirmKey(t, m, confirmYes)

	requireSlotCommits(t, persister, slotCommit{slug: "sunset", slot: prefs.SlotDark})
	requirePairKeys(t, m, "ghost", "sunset")
	// A ROW APPEARS and it is RE-SORTED into place: `ghost` was invisible to the
	// open-time union, because a `theme`-wins file's slots are not read at all, and
	// it lands FIRST rather than appended.
	requireRenderedOrder(t, m, "ghost", "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	// THE BADGES MOVE: the bare `●` of a constant becomes the pair's two slot
	// markers, one of them on the row that has just appeared.
	requireRenderedBadge(t, m, "sunset", theme.BadgeDark)
	requireRenderedBadge(t, m, "ghost", theme.BadgeLight)
	requireRenderedBadgeCount(t, m, 2)
	// THE CURSOR IS ANCHORED BY IDENTITY: the inserted row pushed `sunset` down, so
	// the INDEX MOVING is as much the assertion as the cursor bar not moving.
	requireRenderedCursorOn(t, m, "sunset")
	if got, want := m.themePanel.list.Index(), before+1; got != want {
		t.Errorf("the cursor sits at index %d, want %d — the row inserted above it pushed the previewed row down", got, want)
	}
	if m.activeTheme != previewed {
		t.Errorf("the commit rendered %s, want the previewed %s — a commit is a WRITE, not a navigation (§9.2)", themeLabel(m.activeTheme), themeLabel(previewed))
	}

	// The other direction: `Enter` clears both slots, so the row that existed only
	// because a slot named it disappears and the two markers collapse to one.
	m, _ = pressCommitKey(t, m)

	// Asserted against the recorder directly rather than through requireCommitted,
	// which reads a persister that has seen no slot write: this walk deliberately
	// drives both commit shapes through one recorder, so the slot call above is
	// still on it.
	if got := persister.constants; !slices.Equal(got, []string{"sunset"}) {
		t.Fatalf("`Enter` recorded the constants %v, want [sunset]", got)
	}
	if got := len(persister.slotCommits); got != 1 {
		t.Errorf("`Enter` recorded %d slot commits, want the 1 the confirm made — it writes the CONSTANT and clears both slots (§9.2)", got)
	}
	requireConstantKeys(t, m, "sunset")
	requireRenderedOrder(t, m, "nord", "sunset", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	requireRenderedBadge(t, m, "sunset", theme.BadgeConstant)
	requireRenderedBadgeCount(t, m, 1)
	requireRenderedCursorOn(t, m, "sunset")
	if got := m.themePanel.list.Index(); got != before {
		t.Errorf("the cursor sits at index %d, want %d — the removed row above it pulled the previewed row back up", got, before)
	}
}

// behaviourConfirmAnswers is §9.2's CLOSED set of resolving inputs: `y`/`Y`
// confirm, `n`/`N`/`Esc` cancel. The uppercase forms carry ModShift because that is
// what a terminal actually sends; the lock-modifier shapes the matcher additionally
// forgives are not this suite's subject.
var behaviourConfirmAnswers = []tea.KeyPressMsg{
	confirmYes, confirmYesShift, confirmNo, confirmNoShift, confirmEsc,
}

// TestThemePanelBehaviour_Confirm: it resolves the confirm on three inputs.
//
// §9.2: the confirm "resolves on exactly three inputs" and "every other key is
// swallowed — arrows, `Enter`, the other slot key, all of it".
//
// What is proven here is the CLOSURE, which a hand-written key list cannot state:
// the swallowed set is derived from the panel's own keymap descriptor, so the
// partition covers every key the panel binds and a seventh one added later is
// carried into it automatically. Per-key behaviour — the modifier shapes an answer
// arrives in, the positive control behind each swallowed key, the untouched frame —
// is proven in theme_panel_confirm_test.go; this says that the two sets between
// them are all of it.
func TestThemePanelBehaviour_Confirm(t *testing.T) {
	t.Run("the three answers resolve it", func(t *testing.T) {
		for _, press := range behaviourConfirmAnswers {
			t.Run(press.String(), func(t *testing.T) {
				m := behaviourConfirmModel(t)

				m, _ = pressConfirmKey(t, m, press)

				if m.themePanel.confirming() {
					t.Errorf("%v left the question standing; it is one of §9.2's three resolving inputs", press)
				}
				if !m.themePanel.open {
					t.Errorf("%v closed the panel; the confirm resolves, and only an `Esc` with no confirm live closes (§9.2)", press)
				}
			})
		}
	})

	t.Run("every other key the panel binds is swallowed", func(t *testing.T) {
		swallowed := 0
		for name, probe := range themePanelProbes(livePanelDispatch) {
			if slices.Contains(behaviourConfirmAnswers, probe.press) {
				continue
			}
			swallowed++
			t.Run(name, func(t *testing.T) {
				m := behaviourConfirmModel(t)
				pending, index := m.themePanel.pending, m.themePanel.list.Index()
				persister, ok := m.themePersister.(*fakeThemePersister)
				if !ok {
					t.Fatalf("the fixture wired a %T persister, want the recording fake", m.themePersister)
				}

				got, cmd := pressConfirmKey(t, m, probe.press)

				if !got.themePanel.confirming() || got.themePanel.pending != pending {
					t.Errorf("%v resolved the question (pending %+v, want the standing %+v); only §9.2's three inputs may", probe.press, got.themePanel.pending, pending)
				}
				if len(persister.slugs) != 0 {
					t.Errorf("%v wrote %v while the question was live; nothing is written until `y`", probe.press, persister.slugs)
				}
				if got.themePanel.list.Index() != index {
					t.Errorf("%v moved the cursor to row %d, want it left on %d — a move mid-question re-themes the screen behind the answer", probe.press, got.themePanel.list.Index(), index)
				}
				if cmd != nil {
					t.Errorf("%v scheduled %T while the question was live, want nothing", probe.press, cmd)
				}
			})
		}
		// The descriptor is what makes this closed, so an empty (or emptied) one must
		// fail rather than pass over nothing.
		if want := len(themePanelKeymap()) - 1; swallowed != want {
			t.Errorf("the partition covered %d swallowed panel keys, want %d — every descriptor entry but `esc`, which is an answer", swallowed, want)
		}
	})
}

// behaviourConfirmModel opens a panel over a CONSTANT with the cursor arrowed off
// the persisted row, and presses `l` to raise §9.2's question.
//
// The cursor is moved first because it is the only shape in which the pending slug
// and the constant being cleared are distinguishable strings — a question naming
// the wrong one is invisible while they are the same.
func behaviourConfirmModel(t *testing.T) Model {
	t.Helper()

	m, _ := behaviourPanel(t, []theme.Entry{
		behaviourFile("aurora", 0),
		behaviourFile("sunset", 1),
	}, theme.RawKeys{Theme: "aurora"})
	m = arrowToThemeRow(t, m, "sunset")

	m, _ = pressSlotKey(t, m, slotLightPress)
	if !m.themePanel.confirming() {
		t.Fatal("fixture: `l` over a constant raised no confirm, so there is no question to resolve")
	}
	if m.themePanel.pending != (themeSlotConfirm{slug: "sunset", slot: prefs.SlotLight}) {
		t.Fatalf("fixture: the pending assignment is %+v, want `sunset` into the light slot", m.themePanel.pending)
	}
	return m
}

// TestThemePanelBehaviour_FailureStateMachine: it runs the outstanding-failure
// state machine.
//
// §9.13 splits one failed write into TWO lifetimes, and the split is the whole
// mechanism: the MESSAGE persists only until the next keypress, while the STATE
// runs until a subsequent commit SUCCEEDS. Composed without that, the very next
// `Esc` both takes the message down and drops the theme the user chose — with no
// `●` movement to signal it, because §9.13 forbids the marker moving on a write
// that did not land — and "reported rather than silent" would hold for exactly one
// keystroke.
//
// Each transition has its own proof in theme_panel_commit_failure_test.go and
// theme_panel_close_report_test.go. What is walked here is the MACHINE: the
// transitions composed in one model, where a state carried across a cleared message
// and discharged by a later success is the thing that either works or does not.
func TestThemePanelBehaviour_FailureStateMachine(t *testing.T) {
	t.Run("the message clears on a keypress and the state stands until a commit succeeds", func(t *testing.T) {
		m, persister := behaviourFailureModel(t)
		persister.err = errThemeCommitFailed

		m, _ = pressSlotKey(t, m, slotDarkPress)

		if got := themePanelMessageRow(m); got != messageTestFailedCopy {
			t.Fatalf("the failed commit drew %q in the message slot, want §14A's %q", got, messageTestFailedCopy)
		}
		if !m.themeCommitFailed {
			t.Fatal("the failed commit left nothing outstanding")
		}

		// The next keypress takes the MESSAGE down and still performs its own action —
		// one key, one intent — while the STATE runs on.
		index := m.themePanel.list.Index()
		m = pressPanelKey(t, m, arrowDown)
		if got := themePanelMessageRow(m); got != "" {
			t.Errorf("the message slot still reads %q after the next keypress, want it cleared", got)
		}
		if m.themePanel.list.Index() == index {
			t.Error("the keypress that cleared the message was swallowed; it clears AND acts (§9.13)")
		}
		if !m.themeCommitFailed {
			t.Fatal("arrowing away discharged the state; only a SUCCESSFUL commit does (§9.13)")
		}

		// A landed write discharges it, and the close that follows is therefore silent:
		// the user is not told a theme was not saved when it was.
		persister.err = nil
		m, _ = pressSlotKey(t, m, slotLightPress)
		if m.themeCommitFailed {
			t.Error("the successful commit left the failure outstanding")
		}

		m, _ = closePanelForTest(t, m)
		if got := m.flashText; got != "" {
			t.Errorf("the close raised %q after a successful retry, want nothing", got)
		}
	})

	t.Run("closing with a failure outstanding reports once and discharges", func(t *testing.T) {
		m, persister := behaviourFailureModel(t)
		persister.err = errThemeCommitFailed
		m, _ = pressSlotKey(t, m, slotDarkPress)

		m, _ = closePanelForTest(t, m)

		if got := m.flashText; got != specThemeNotSavedFlash {
			t.Errorf("the close raised %q, want §14A's %q", got, specThemeNotSavedFlash)
		}
		requireFlashBandVisible(t, m, specThemeNotSavedFlash)
		if m.themeCommitFailed {
			t.Error("the report was raised with the failure still outstanding; raising it DISCHARGES the state (§9.13)")
		}

		// Re-opening and closing again reports NOTHING: the state has done its job, and
		// without the discharge this would re-fire on every close for the life of the
		// process.
		m = pressThemeKey(t, m)
		if !m.themePanel.open {
			t.Fatal("`t` did not re-open the panel, so the second close proves nothing")
		}
		m, _ = closePanelForTest(t, m)
		if got := m.flashText; got != "" {
			t.Errorf("the second close raised %q, want nothing — the first discharged the state", got)
		}
	})

	t.Run("a forced close reports the failure over the geometry event", func(t *testing.T) {
		m, persister := behaviourFailureModel(t)
		persister.err = errThemeCommitFailed
		m, _ = pressSlotKey(t, m, slotDarkPress)

		contentW, contentH := geometryBelowWidthFloor()
		m = resizeForTest(t, m, contentW, contentH)

		if m.themePanel.open {
			t.Fatal("the resize below the floor left the panel open")
		}
		if got := m.flashText; got != specThemeNotSavedFlash {
			t.Errorf("the forced close raised %q, want §9.13's report to win the single band slot over %q", got, specNarrowClosedFlash)
		}
		if m.themeCommitFailed {
			t.Error("the forced close left the failure outstanding; the report was made, so the state is discharged")
		}

		// The control: the SAME crossing with nothing outstanding raises the geometry
		// copy, so the precedence above is the report winning rather than the geometry
		// flash having gone missing.
		control, _ := behaviourFailureModel(t)
		control = resizeForTest(t, control, contentW, contentH)
		if got := control.flashText; got != specNarrowClosedFlash {
			t.Errorf("the control's forced close raised %q, want §14A's %q", got, specNarrowClosedFlash)
		}
	})
}

// behaviourFailureModel opens a panel under an ADAPTIVE PAIR, which is the setting
// shape in which `d`/`l` write straight through — over a constant §9.2's confirm
// would gate them and the walk would be about the question rather than the write.
func behaviourFailureModel(t *testing.T) (Model, *fakeThemePersister) {
	t.Helper()

	m, persister := behaviourPanel(t, []theme.Entry{
		behaviourFile("aurora", 0),
		behaviourFile("sunset", 1),
	}, theme.RawKeys{Light: "aurora", Dark: "sunset"})
	if m.themeSetting().IsConstant {
		t.Fatal("fixture: the setting resolved as a constant, so `d`/`l` would ask rather than write")
	}
	return m, persister
}

// behaviourSuiteFile is this file, as the guard below reads it.
const behaviourSuiteFile = "theme_panel_behaviour_test.go"

// behaviourBannedImports are the packages that would let this suite reach a real
// directory or a real config file. Naming one is the only way in: the seam is
// declared data, the persister is a recorder, and neither takes a path.
var behaviourBannedImports = []string{"os", "io/fs", "path", "path/filepath"}

// behaviourBannedCalls are the call names that reach the filesystem or the process
// environment without an import this file could be caught by — t.TempDir and
// t.Setenv are testing's own, and a store constructor would arrive through the
// prefs package this file legitimately imports for its slot enum.
var behaviourBannedCalls = []string{
	"TempDir", "Setenv", "MkdirTemp", "WriteFile", "ReadFile", "ReadDir", "NewStore",
}

// TestThemePanelBehaviour_NoConfigAccess: it touches no config.
//
// §13.3 requires the seam as an ARCHITECTURAL COMMITMENT rather than a convenience,
// and this is the property that commitment buys: an invalid-theme row renders with
// no themes directory in existence, so the panel's whole row vocabulary is
// reachable from a suite that opens no directory and reads no prefs.json.
//
// Both halves are needed. The BEHAVIOURAL half shows the property holding — the
// panel lists a rejected drop-in off an enumeration that names no directory at all.
// The SOURCE half is what keeps it true: "reads no config" leaves no trace to assert
// against, so the guard is the file's own import set and call vocabulary, and a
// fixture that later reaches for a temp directory fails here rather than quietly
// re-introducing the dependency the seam exists to remove.
func TestThemePanelBehaviour_NoConfigAccess(t *testing.T) {
	t.Run("it renders an invalid row with no themes directory", func(t *testing.T) {
		m, _ := behaviourPanel(t,
			[]theme.Entry{behaviourRejected("sunset", theme.ReasonMissingTokens)},
			theme.RawKeys{})

		if got := m.themePanel.enumeration.DirPath; got != "" {
			t.Errorf("the retained enumeration names the directory %q, want none — the seam returns the finished union, not a directory listing (§13.3)", got)
		}
		row := renderedPanelRowFor(t, m, "sunset")
		if !strings.Contains(row.trailer, flashWarningGlyph) || !strings.Contains(row.trailer, string(theme.ReasonMissingTokens)) {
			t.Errorf("the rejected row drew %q, want the `⚠` and the terse reason %q", row.trailer, theme.ReasonMissingTokens)
		}
	})

	t.Run("the suite names no filesystem or config entry point", func(t *testing.T) {
		file, err := parser.ParseFile(token.NewFileSet(), behaviourSuiteFile, nil, parser.SkipObjectResolution)
		if err != nil {
			t.Fatalf("parse %s: %v", behaviourSuiteFile, err)
		}

		for _, imported := range file.Imports {
			path := strings.Trim(imported.Path.Value, `"`)
			if slices.Contains(behaviourBannedImports, path) {
				t.Errorf("%s imports %q; the seam is declared data and the persister is a recorder, so this suite needs no path at all", behaviourSuiteFile, path)
			}
		}

		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			if slices.Contains(behaviourBannedCalls, sel.Sel.Name) {
				t.Errorf("%s calls %s; this suite opens no directory, reads no prefs.json and constructs no real store", behaviourSuiteFile, sel.Sel.Name)
			}
			return true
		})
	})
}
