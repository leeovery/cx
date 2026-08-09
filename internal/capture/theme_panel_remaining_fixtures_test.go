package capture_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/capture"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// §13.3's FIVE REMAINING panel surfaces, as frames.
//
// Task 8-15 made the slide-over capturable and delivered the two SETTING-state
// frames. Five specified surfaces still had no way to be seen before release, and
// §13.4's guard structurally cannot report their absence — it enumerates whatever
// fixtures exist, so a missing one READS AS COVERAGE. Each of the five carries
// something no other frame does:
//
//   - the INVALID row is the only place §9.5's four-element composition priority is
//     observable at all, and the only place the reason can be watched losing its
//     slot to a badge;
//   - the pinned `⚠ dir unreadable` row has its own placement rule, its own token
//     and its own pinned copy, and §9.5 gives it NO OTHER WAY TO BE CHECKED;
//   - the NARROW panel is the only observable check on §9.8's ladder between the
//     preferred and minimum widths;
//   - the PAGINATING panel is §11.2's own coverage consequence — the dots render
//     only when the panel's list overflows, so without a fixture carrying enough
//     rows §13.4's guard is blind at exactly the `bubbles/list` instance the panel
//     adds;
//   - the panel over PROJECTS exists because `t` is bound there (§9.6), Projects
//     carries its own flash slot (§14A), and every other panel fixture is
//     implicitly Sessions-based.
//
// The assertions read the RENDERED frame rather than model internals, exactly as
// the sibling setting-state assertions do: internal/tui exports no panel accessor,
// and the frame is what the tapes screenshot.
//
// No t.Parallel() anywhere — the project bans it outright.

// remainingPanelFixtureNames is the set this task registers, named once so a
// per-fixture assertion cannot cover four and forget the fifth.
func remainingPanelFixtureNames() []string {
	return []string{
		"theme-panel-invalid-row",
		"theme-panel-dir-unreadable",
		"theme-panel-narrow",
		"theme-panel-paginated",
		"theme-panel-projects",
	}
}

// allPanelFixtureNames is every `theme-panel-*` fixture §13.3 specifies: the two
// setting-state frames, the five that followed them, and the message-slot set —
// the slot-from-constant confirm, the failed-commit line and the panel at its
// minimum height with a message live.
//
// The last three arrived separately because all three are COMMIT-PATH states
// (§9.2, §9.13): until a commit key could write, the message slot had no setter
// and a fixture for any of them could only render a state nothing could reach.
func allPanelFixtureNames() []string {
	names := append(capturePanelFixtureNames(), remainingPanelFixtureNames()...)
	return append(names, messagePanelFixtureNames()...)
}

// The sizes the two size-sensitive frames are captured at, declared here because
// they are what their tapes resolve to and what these assertions must render at.
//
// VHS sets the terminal in PIXELS (`Set Width` / `Set Height` against
// `Set FontSize`), so a tape cannot state a column count directly — each tape's
// comment records the count its pixel size resolves to, and these constants are
// the same numbers on the Go side.
const (
	// narrowPanelTermWidth lands the panel in §9.8's DEGRADED BAND. The ladder is
	// `min(max(contentW/2, minimum), preferred)` against a content region of
	// termW − 2·tui.Hinset, so 64 columns gives 60 content columns and a 30-column
	// panel — strictly inside the band, which is the only thing this frame exists
	// to show.
	narrowPanelTermWidth = 64

	// minimumPanelTermWidth is the narrowest terminal that still renders a panel,
	// used ONLY to derive the ladder's bottom end for comparison (see
	// TestPanelFixture_NarrowIsBetweenMinAndPreferred). 31 columns leaves 27
	// content columns, which is the minimum panel width.
	minimumPanelTermWidth = 31

	// dirUnreadablePanelTermHeight is short enough that the five-row union
	// OVERFLOWS the panel body, which is what makes the `Ctrl+↓` in the fixture's
	// captureKeys move a page rather than do nothing. 16 rows leaves 14 content
	// rows; the panel spends 5 on its header, 1 on the pinned directory row and 4
	// on its footer, so the body is 4 rows and `bubbles/list` pages it two at a
	// time.
	dirUnreadablePanelTermHeight = 16
)

// panelFrameAt renders one fixture at an explicit size, driven through its own
// captureKeys by ModelAt — the identical drive a tape performs.
//
// The size is explicit because some of these frames are SIZE-SENSITIVE by
// construction: the narrow one is about the width ladder and the directory one
// about pagination, so neither can be asserted at the guard's one pinned size.
func panelFrameAt(t *testing.T, fixture string, palette theme.Theme, w, h int) string {
	t.Helper()

	fx, err := capture.FixtureByName(fixture)
	if err != nil {
		t.Fatalf("FixtureByName(%s): %v", fixture, err)
	}
	return fx.ModelAt(palette, w, h).View().Content
}

// panelLineWith is the ONE panel line whose visible text carries want, with its
// index in the frame — fataling when there is none or more than one.
//
// It reads the PANEL side of each line (everything past the slide-over's one
// left-border column) rather than the whole frame, because the page behind it
// renders its own rows: a frame-wide scan for a slug would find the session list's
// text as readily as the panel's.
//
// It matches on SUBSTRING, so it is for the panel's CHROME — the header label, the
// pinned directory row, the paginator — and never for a row label: `tokyo-night` is
// a substring of `tokyo-night-day`, so a row lookup goes through panelRowLine.
func panelLineWith(t *testing.T, frame, want string) (index int, raw string) {
	t.Helper()
	return uniquePanelLine(t, frame, want, func(visible string) bool {
		return strings.Contains(visible, want)
	})
}

// panelRowLine is the ONE panel line whose LABEL is exactly label — the row
// lookup, matching on the first field after the optional cursor bar rather than on
// a substring.
//
// THE UNIQUENESS IS AN ASSERTION, not a convenience. §9.5's row invariant is that a
// row NEVER WRAPS — every list row is exactly one delegate line, which is what
// `bubbles/list` pagination, the invalid-row skip and §9.8's paging all rest on —
// so a label found on two lines is that invariant broken.
func panelRowLine(t *testing.T, frame, label string) (index int, raw string) {
	t.Helper()
	return uniquePanelLine(t, frame, "the row "+label, func(visible string) bool {
		fields := strings.Fields(visible)
		if len(fields) > 0 && fields[0] == "▌" {
			fields = fields[1:]
		}
		return len(fields) > 0 && fields[0] == label
	})
}

// uniquePanelLine is the shared search both lookups above are: the panel side of
// each frame line (everything past the slide-over's one left-border column) tested
// against the predicate, with exactly one match required.
//
// It reads the panel side rather than the whole line because the page behind it
// renders its own rows: a frame-wide scan for a slug would find the session list's
// text as readily as the panel's.
func uniquePanelLine(t *testing.T, frame, subject string, matches func(visible string) bool) (index int, raw string) {
	t.Helper()

	found := -1
	for i, line := range strings.Split(frame, "\n") {
		_, panel, onPanel := strings.Cut(line, panelBorder)
		if !onPanel || !matches(ansi.Strip(panel)) {
			continue
		}
		if found >= 0 {
			t.Fatalf("%s renders on panel lines %d AND %d; §9.5 puts every row on exactly one delegate line:\n%s", subject, found, i, ansi.Strip(frame))
		}
		found, raw = i, panel
	}
	if found < 0 {
		t.Fatalf("no panel line carries %s:\n%s", subject, ansi.Strip(frame))
	}
	return found, raw
}

// fgSeq is a token's rendered FOREGROUND SGR core (`38;2;R;G;B`) — the
// foreground counterpart of bgSeq. Styled output carries no hex, so a line is
// searched for this rather than for the value a theme file declares.
func fgSeq(t *testing.T, tok theme.Token) string {
	t.Helper()
	return sgrParameterRun(t, lipgloss.NewStyle().Foreground(tok.Color()))
}

// panelOuterWidth measures the slide-over's OUTER width off a rendered frame —
// the value §9.8's ladder chose, recovered from the one thing a frame shows: where
// the left border falls.
//
// overlayThemePanel places the panel at the RIGHT edge of the content region, and
// the content region sits tui.Hinset cells in from the terminal's own edge, so the
// border's column determines the width outright.
func panelOuterWidth(t *testing.T, frame string, termW int) int {
	t.Helper()

	for line := range strings.SplitSeq(ansi.Strip(frame), "\n") {
		page, _, onPanel := strings.Cut(line, panelBorder)
		if !onPanel {
			continue
		}
		return termW - tui.Hinset - lipgloss.Width(page)
	}
	t.Fatalf("no frame line carries the panel's left border %q, so the panel did not render:\n%s", panelBorder, ansi.Strip(frame))
	return 0
}

// TestPanelFixture_InvalidRowFrame: it renders the invalid-row composition
// priority.
//
// §13.3 mandates an invalid-theme row, and this frame is the ONLY place §9.5's
// four-element priority is observable at all. The three rows it carries are the
// three things that priority decides: a reason that FITS, a reason DROPPED by a
// badge competing for the same right edge, and a label long enough to be
// truncated.
func TestPanelFixture_InvalidRowFrame(t *testing.T) {
	palette := builtinPalette(t, "tokyo-night")
	frame := panelFixtureFrame(t, "theme-panel-invalid-row", palette)
	rows := panelRows(t, frame)

	t.Run("the reason-only row renders the glyph and the reason on one line", func(t *testing.T) {
		if got, want := rows["aurora-glow"].badge, "⚠ bad syntax"; got != want {
			t.Errorf("the aurora-glow row's trailing elements = %q, want %q", got, want)
		}
		// panelRowLine fatals on a second line, which IS §9.5's one-delegate-line
		// invariant: an invalid row never wraps.
		_, line := panelRowLine(t, frame, "aurora-glow")
		if !strings.Contains(ansi.Strip(line), "bad syntax") {
			t.Errorf("the label and its reason are not on the same line: %q", ansi.Strip(line))
		}
	})

	t.Run("the invalid label is text.subtle and its reason accent.attention", func(t *testing.T) {
		_, line := panelRowLine(t, frame, "aurora-glow")
		if !strings.Contains(line, fgSeq(t, palette.TextSubtle)) {
			t.Errorf("the aurora-glow row does not carry text.subtle; §9.1 makes an invalid label de-emphasised BUT READABLE, and §13.5 forbids text.faint reaching the UI floor:\n%q", ansi.Strip(line))
		}
		if !strings.Contains(line, fgSeq(t, palette.AccentAttention)) {
			t.Errorf("the aurora-glow row does not carry accent.attention; the `⚠` and its reason are one accent.attention run (§9.1's table):\n%q", ansi.Strip(line))
		}
	})

	t.Run("the persisted invalid row keeps its badge and loses its reason", func(t *testing.T) {
		if got := rows["nord-lee"].badge; !strings.Contains(got, "● dark") {
			t.Errorf("the nord-lee row's trailing elements = %q, want the `● dark` badge — §9.5's badge marks what is SET, and a fallback never moves it", got)
		}
		if got := rows["nord-lee"].badge; strings.Contains(got, "bad colour") {
			t.Errorf("the nord-lee row still renders its reason (%q); §9.5's priority 2 outranks priority 4, so a badged row has no reason slot to fill", got)
		}
	})

	t.Run("the over-long label is truncated with an ellipsis", func(t *testing.T) {
		visible := ansi.Strip(frame)
		// The exact cut: the label flexes to what §9.5's fixed columns leave, which
		// at the preferred width is 28 cells — 27 visible characters plus the
		// ellipsis, since ansi.Truncate counts the tail inside the width it is given.
		if !strings.Contains(visible, "My Gorgeous Midnight Palett…") {
			t.Errorf("the over-long `bad name` label is not truncated to the panel's budget:\n%s", visible)
		}
		if strings.Contains(visible, "My Gorgeous Midnight Palette.theme") {
			t.Error("the over-long label renders in full; §9.5 truncates it with `…` to whatever the fixed columns leave")
		}
	})

	t.Run("the cursor rests on a valid row", func(t *testing.T) {
		if !rows["tokyo-night"].cursor {
			t.Error("the cursor is not on tokyo-night; §9.5's arrow skip keeps it off invalid rows, so a frame with it parked on one is a state production cannot reach")
		}
		for _, invalid := range []string{"aurora-glow", "nord-lee"} {
			if rows[invalid].cursor {
				t.Errorf("the cursor is on the invalid row %s", invalid)
			}
		}
	})
}

// TestPanelFixture_InvalidPersistedRowDropsTheReason: it drops the reason when a
// badge competes.
//
// §9.5 makes the reason THE FIRST ELEMENT DROPPED, and the badge outranking it is
// the one half of that rule no other frame can show: `⚠` still says the row is
// invalid and doctor says why, which is exactly the split §6.3 draws.
//
// It is asserted as a PAIR, over one frame, because either half alone is
// worthless. "`bad colour` renders nowhere" passes just as readily on a panel that
// renders no reasons at all, and "`bad syntax` renders" says nothing about
// competition. The two reasons sit on rows that differ ONLY in carrying a badge.
func TestPanelFixture_InvalidPersistedRowDropsTheReason(t *testing.T) {
	visible := ansi.Strip(panelFixtureFrame(t, "theme-panel-invalid-row", builtinPalette(t, "tokyo-night")))

	if !strings.Contains(visible, "bad syntax") {
		t.Errorf("the UNBADGED invalid row renders no reason at all, so a missing reason on the badged row demonstrates nothing about the priority:\n%s", visible)
	}
	if strings.Contains(visible, "bad colour") {
		t.Errorf("the BADGED invalid row still renders its reason; §9.4 exists so the marker always has a home, so the badge takes the right edge and the reason is dropped:\n%s", visible)
	}
}

// TestPanelFixture_DirUnreadableIsChromeOnPageTwo: it pins the directory row on
// page two.
//
// §9.5: the `⚠ dir unreadable` row is "chrome pinned to the viewport directly
// beneath the header, NOT a list row… a list row participates in pagination, so
// the warning would vanish the moment the user paged down".
//
// THE FRAME IS TAKEN ON PAGE 2 FOR THAT REASON ALONE. A page-1 capture is
// indistinguishable from one of a warning implemented as a list delegate — both
// show the row — so the claim is only observable once the user has paged past it.
func TestPanelFixture_DirUnreadableIsChromeOnPageTwo(t *testing.T) {
	frame := panelFrameAt(t, "theme-panel-dir-unreadable", builtinPalette(t, "tokyo-night"), harnessWidth, dirUnreadablePanelTermHeight)
	visible := ansi.Strip(frame)

	// The non-vacuity leg, and it comes first: a frame still on page 1 would carry
	// the warning whether it were chrome or a list row, so every claim below rests
	// on the `Ctrl+↓` having genuinely moved a page.
	if strings.Contains(visible, "nord-lee") {
		t.Fatalf("the frame still shows the page-1 rows, so the `Ctrl+↓` never paged and this asserts nothing about chrome:\n%s", visible)
	}

	dirRow, _ := panelLineWith(t, frame, "dir unreadable")
	if !strings.Contains(visible, "⚠ dir unreadable") {
		t.Errorf("the pinned copy is not rendered verbatim; §9.5 fixes it at 16 columns precisely so it never truncates:\n%s", visible)
	}

	header, _ := panelLineWith(t, frame, "Themes")
	if dirRow <= header {
		t.Errorf("the directory row is on line %d and the header on line %d; §9.5 pins it DIRECTLY BENEATH the header", dirRow, header)
	}
	for _, row := range []string{"solarized-lee", "tokyo-night"} {
		at, _ := panelRowLine(t, frame, row)
		if at <= dirRow {
			t.Errorf("theme row %s renders on line %d, above the pinned directory row on line %d", row, at, dirRow)
		}
	}
}

// TestPanelFixture_RowsBeneathDirRow: it renders rows beneath the directory row.
//
// §9.5: "built-in rows and persisted-slug rows still render beneath it — the
// PERSISTED ROWS ESPECIALLY, or a user with an unreadable directory loses the `●`
// entirely."
//
// Both kinds are asserted on the SAME page-2 frame, which is what makes the claim
// about the warning's chrome-ness and the claim about the surviving `●` one
// observation rather than two.
func TestPanelFixture_RowsBeneathDirRow(t *testing.T) {
	frame := panelFrameAt(t, "theme-panel-dir-unreadable", builtinPalette(t, "tokyo-night"), harnessWidth, dirUnreadablePanelTermHeight)
	rows := panelRows(t, frame)

	t.Run("a built-in row renders beneath it", func(t *testing.T) {
		if !rows["tokyo-night"].cursor {
			t.Errorf("the built-in row tokyo-night does not carry the cursor; the parsed rows are %v", labelSet(rows))
		}
	})

	t.Run("a persisted-slug row keeps its badge beneath it", func(t *testing.T) {
		if got := rows["solarized-lee"].badge; !strings.Contains(got, "● light") {
			t.Errorf("the persisted row solarized-lee's trailing elements = %q, want the `● light` badge — losing it is the state §9.5 names", got)
		}
	})
}

// TestPanelFixture_NarrowIsBetweenMinAndPreferred: it renders in the degraded
// width band.
//
// §9.8's ladder is a STAGED shrink — the panel takes half the content region,
// clamped to the two ends — and this frame is its ONLY observable check: the two
// setting-state frames render at the preferred width, and nothing else exercises
// the band between.
//
// THE TWO ENDS ARE DERIVED FROM RENDERS RATHER THAN RESTATED. internal/tui keeps
// both constants unexported, and a literal here would agree with a broken ladder
// exactly as readily as with a working one — so the same fixture is rendered wide
// (which takes the preferred width) and at the narrowest terminal that still
// renders a panel (which takes the minimum), and the narrow frame must sit
// STRICTLY between them.
func TestPanelFixture_NarrowIsBetweenMinAndPreferred(t *testing.T) {
	palette := builtinPalette(t, "nord")

	preferred := panelOuterWidth(t, panelFixtureFrame(t, "theme-panel-narrow", palette), harnessWidth)
	minimum := panelOuterWidth(t, panelFrameAt(t, "theme-panel-narrow", palette, minimumPanelTermWidth, harnessHeight), minimumPanelTermWidth)
	if minimum >= preferred {
		t.Fatalf("the ladder's ends measure %d (minimum) and %d (preferred); with no band between them nothing can sit strictly inside it", minimum, preferred)
	}

	narrow := panelFrameAt(t, "theme-panel-narrow", palette, narrowPanelTermWidth, harnessHeight)
	got := panelOuterWidth(t, narrow, narrowPanelTermWidth)
	if got <= minimum || got >= preferred {
		t.Errorf("at %d columns the panel renders %d wide, want strictly between the minimum %d and the preferred %d", narrowPanelTermWidth, got, minimum, preferred)
	}

	// §9.5's one-delegate-line invariant, restated at the degraded width because
	// that is where a composition budget miscount would first show: every row still
	// renders on exactly one line (panelRowLine fatals on a second).
	t.Run("every row still renders on exactly one line", func(t *testing.T) {
		for _, slug := range panelUnionSlugs() {
			panelRowLine(t, narrow, slug)
		}
	})

	t.Run("the badges survive the degraded width", func(t *testing.T) {
		rows := panelRows(t, narrow)
		if got, want := rows["nord"].badge, "● dark"; got != want {
			t.Errorf("the nord row's badge = %q, want %q — §9.5's priority 2 reserves the badge before anything can crowd it out", got, want)
		}
		if got, want := rows["tokyo-night-day"].badge, "● light"; got != want {
			t.Errorf("the tokyo-night-day row's badge = %q, want %q", got, want)
		}
	})
}

// TestPanelFixture_PaginatedDrawsDots: it paginates and draws the dots.
//
// §11.2's own coverage consequence: "pagination dots only render when the panel's
// list paginates, so one of §13.3's panel fixtures must carry enough theme rows to
// overflow. OTHERWISE THE GUARD IS BLIND at exactly the new site this paragraph
// adds." The dots are the exemplar of the cached-style class — `bubbles/list` reads
// their STRINGS out of the styles once at construction — so a restyle that failed
// to re-feed the paginator would render the library's hardcoded greys under every
// theme, identically before and after a swap.
//
// The non-vacuity leg is a sibling fixture whose union is four rows: it must draw
// NO dots at the same size, or "the dots render" would be a statement about the
// panel rather than about overflow.
func TestPanelFixture_PaginatedDrawsDots(t *testing.T) {
	palette := builtinPalette(t, "nord")
	frame := panelFixtureFrame(t, "theme-panel-paginated", palette)

	t.Run("a four-row panel draws no dots at the same size", func(t *testing.T) {
		if panelCarriesDots(panelFixtureFrame(t, "theme-panel-adaptive-pair", palette)) {
			t.Error("the four-row panel already draws pagination dots, so drawing them on the paginating one demonstrates nothing about overflow")
		}
	})

	t.Run("the overflowing panel draws them", func(t *testing.T) {
		if !panelCarriesDots(frame) {
			t.Errorf("the panel draws no pagination dots, so §13.4's guard is blind at the panel's bubbles/list instance:\n%s", ansi.Strip(frame))
		}
	})

	t.Run("the dots are painted from the active theme", func(t *testing.T) {
		_, line := panelLineWith(t, frame, paginationDot)
		if !strings.Contains(line, fgSeq(t, palette.AccentPrimary)) {
			t.Error("the ACTIVE dot is not accent.primary; `bubbles/list` reads the dot strings out of its styles once at construction, so this is the site a restyle must re-feed")
		}
		if !strings.Contains(line, fgSeq(t, palette.TextFaint)) {
			t.Error("no INACTIVE dot is text.faint; a single-dot row is not a paginating one")
		}
	})
}

// paginationDot is the glyph `bubbles/list` draws its paginator with. It is a
// literal because internal/tui keeps its own copy unexported; it is deliberately
// NOT the `●` of §9.5's badge, which is what lets a scan tell the two apart.
const paginationDot = "•"

// panelCarriesDots reports whether any panel line carries the paginator glyph.
func panelCarriesDots(frame string) bool {
	for line := range strings.SplitSeq(ansi.Strip(frame), "\n") {
		_, panel, onPanel := strings.Cut(line, panelBorder)
		if onPanel && strings.Contains(panel, paginationDot) {
			return true
		}
	}
	return false
}

// TestPanelFixture_OverProjects: it renders over the Projects page.
//
// §9.6 binds `t` on Projects because theme is a GLOBAL setting, and §14A gave that
// page its own transient-flash slot for the panel's six signals — yet every other
// panel fixture is implicitly Sessions-based, which is why §13.3 requires this one.
//
// It also renders §9.1's ACCEPTED COST where it is most visible: the overlay cuts
// wherever its left border falls, mid-label included. That is asserted by diffing
// against the SAME page rendered without the panel, so the claim is "the overlay
// cut it" rather than "some footer text is missing".
func TestPanelFixture_OverProjects(t *testing.T) {
	palette := builtinPalette(t, "nord")

	fx, err := capture.FixtureByName("theme-panel-projects")
	if err != nil {
		t.Fatalf("FixtureByName(theme-panel-projects): %v", err)
	}
	model := fx.ModelAt(palette, harnessWidth, harnessHeight)
	frame := model.View().Content
	visible := ansi.Strip(frame)

	t.Run("the page beneath is Projects", func(t *testing.T) {
		if model.ActivePage() != tui.PageProjects {
			t.Errorf("ActivePage() = %d, want PageProjects — the `x` its captureKeys type never landed", model.ActivePage())
		}
		if !strings.Contains(visible, "flow-v1-api") {
			t.Errorf("the frame carries no project rows:\n%s", visible)
		}
	})

	t.Run("the panel is composited over it with its badges", func(t *testing.T) {
		rows := panelRows(t, frame)
		if got, want := rows["nord"].badge, "● dark"; got != want {
			t.Errorf("the nord row's badge = %q, want %q", got, want)
		}
		if got, want := rows["tokyo-night-day"].badge, "● light"; got != want {
			t.Errorf("the tokyo-night-day row's badge = %q, want %q", got, want)
		}
	})

	// §9.1's accepted cost: the overlay cuts the page's footer WHEREVER ITS BORDER
	// FALLS, because the page beneath is composed at the unreduced content width and
	// deliberately not re-laid-out. What that removes here is the right-aligned
	// `? help`; on the Sessions frames the same cut lands mid-entry instead. Which
	// of the two it is depends on where the border falls, so the assertion is the
	// PREFIX RELATION rather than a particular severed word — a reflow to the
	// reduced width would not be a prefix of the bare footer at all.
	t.Run("the Projects footer beneath is cut by the overlay", func(t *testing.T) {
		plain := ansi.Strip(panelFixtureFrame(t, "projects", palette))
		bare, covered := footerLine(t, plain), footerLine(t, visible)
		if bare == covered {
			t.Fatalf("the footer is identical with and without the panel, so nothing was covered:\n%s", covered)
		}
		if !strings.HasPrefix(bare, covered) {
			t.Errorf("the covered footer %q is not a prefix cut of the bare footer %q; §9.1's overlay cuts wherever its border falls rather than reflowing", covered, bare)
		}
	})
}

// footerLine is the PAGE side of the Projects footer row — everything left of the
// slide-over's border on a frame that carries one, and the whole line on a frame
// that does not.
//
// It is located by `new session`, the footer's leading entry: it is on the page
// rather than the panel, it is far enough left that the overlay never covers it,
// and it appears on no other row — where a search for `filter` would land on the
// section header's own `/ to filter` hint.
func footerLine(t *testing.T, visible string) string {
	t.Helper()
	for line := range strings.SplitSeq(visible, "\n") {
		if !strings.Contains(line, "new session") {
			continue
		}
		page, _, _ := strings.Cut(line, panelBorder)
		return strings.TrimRight(page, " ")
	}
	t.Fatalf("no line carries the Projects footer:\n%s", visible)
	return ""
}

// TestPanelFixture_AllRegistered: it registers all five in both lists.
//
// FixtureByName's switch and FixtureNames()'s slice are two hand-maintained lists,
// and a fixture present in one and absent from the other is INVISIBLE to §13.4's
// enumerating guard — it exists, it renders, and nothing ever swaps it. Absence
// reads as coverage, which is the shape task 4-3's drift check exists to close;
// this states the same claim for the five by name, so a half-registration fails
// here with the fixture named.
//
// The third leg is the one that matters most for §11.2: the guard NEVER NAMES A
// FIXTURE, so what is worth pinning is that these five arrive in the set its
// assertions range over with no edit to the guard itself.
func TestPanelFixture_AllRegistered(t *testing.T) {
	guarded := make([]string, 0, len(capture.FixtureNames()))
	for _, fx := range guardedFixtures(t) {
		guarded = append(guarded, fx.Name())
	}

	for _, name := range remainingPanelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			if _, err := capture.FixtureByName(name); err != nil {
				t.Errorf("FixtureByName(%s): %v — the fixture is not resolvable", name, err)
			}
			if !slices.Contains(capture.FixtureNames(), name) {
				t.Errorf("FixtureNames() %v omits %s — an unenumerated fixture is never driven by the guard", capture.FixtureNames(), name)
			}
			if !slices.Contains(guarded, name) {
				t.Errorf("%s is not in the guarded set %v; the guard enumerates, so a fixture it never sees is uncovered while reading as covered", name, guarded)
			}
		})
	}
}

// TestPanelFixture_RemainingFramesNoConfigAccess: it reaches no config.
//
// Fixture data is not config discovery (§13.3) — the same reasoning that admits
// `--theme <path>` — so §7.1's import guard is untouched, and this is the runtime
// half of that claim for the five new frames: each renders its whole panel with the
// themes directory and prefs.json pointed somewhere observable, reads neither, and
// creates nothing.
//
// The decoy is a VALID theme file, so a fixture that reached the real directory
// would list it as a selectable row rather than as a rejection — which a reader
// could mistake for correct behaviour on precisely the frames that are ABOUT
// rejections.
func TestPanelFixture_RemainingFramesNoConfigAccess(t *testing.T) {
	for _, name := range remainingPanelFixtureNames() {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			themes := filepath.Join(dir, "portal", "themes")
			if err := os.MkdirAll(themes, 0o755); err != nil {
				t.Fatalf("stage the decoy themes directory: %v", err)
			}
			if err := os.WriteFile(filepath.Join(themes, "decoy-drop-in.theme"), validThemeFileBody(), 0o644); err != nil {
				t.Fatalf("stage the decoy theme: %v", err)
			}
			t.Setenv("XDG_CONFIG_HOME", dir)
			t.Setenv("PORTAL_THEMES_DIR", themes)
			t.Setenv("PORTAL_PREFS_FILE", filepath.Join(dir, "prefs.json"))

			frame := ansi.Strip(panelFixtureFrame(t, name, builtinPalette(t, theme.DefaultDarkSlug)))
			if strings.Contains(frame, "decoy-drop-in") {
				t.Error("the panel lists the decoy drop-in; the fixture reached the real themes directory")
			}
			entries, err := os.ReadDir(dir)
			if err != nil {
				t.Fatalf("read the isolated config dir: %v", err)
			}
			if len(entries) != 1 || entries[0].Name() != "portal" {
				t.Errorf("the config dir holds %v, want only the staged portal/ tree — the fixture wrote config", entries)
			}
		})
	}
}

// validThemeFileBody is a minimal VALID theme file body — every token of the
// closed vocabulary set to one legal colour.
func validThemeFileBody() []byte {
	var b strings.Builder
	for _, name := range theme.TokenNames() {
		b.WriteString(name + " = #123456\n")
	}
	return []byte(b.String())
}

// TestPanelFixture_RegistryHoldsTheSpecifiedPanelSet: it registers exactly the
// specified panel fixtures.
//
// §13.3 names the panel surfaces that must be visible before release, and a
// registry holding MORE than them is as much a problem as one holding fewer: an
// unnamed extra `theme-panel-*` fixture is a frame nobody specified, driven by
// §13.4's guard and reviewed by no one against anything.
//
// It is stated as an EXACT SET rather than as a list of required names, which is
// what lets it name a surplus. The cost is that a task adding a specified frame
// extends the set as it lands, which is the point rather than an inconvenience.
func TestPanelFixture_RegistryHoldsTheSpecifiedPanelSet(t *testing.T) {
	var panels []string
	for _, name := range capture.FixtureNames() {
		if strings.HasPrefix(name, "theme-panel-") {
			panels = append(panels, name)
		}
	}

	want := allPanelFixtureNames()
	slices.Sort(panels)
	slices.Sort(want)
	if !slices.Equal(panels, want) {
		t.Errorf("the registry holds panel fixtures %v, want exactly the specified set %v", panels, want)
	}
}
