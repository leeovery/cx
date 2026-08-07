package tui

import (
	"fmt"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
)

// This file is the executable half of the §11.2 init-time-derived-style sweep:
// the hand-maintained list of colour-bearing values that are assigned ONCE —
// at construction, not per frame — and must therefore be re-pointed explicitly
// by the restyle path (applyCanvasMode → applyProjectCanvasMode +
// styleFilterInput) or silently keep the previous theme's colours.
//
// §11.2 states the list in prose and calls it hand-maintained with no test. These
// assertions ARE that test for each named member. They deliberately assert on the
// LIVE cached value after the restyle path has run — not on the constructor that
// would build it — because "the constructor is correct" is exactly the vacuous
// pass a once-assigned cache produces.
//
// The swap is driven through Model.ApplyTheme — the live theme-swap entry point
// the panel's arrow-preview, open and close use — so what is proven here is the
// production mechanism §11.1 names, not a test-only setter.
//
// No standing structural guard lives here by design (§13.4): recognising "this is
// a cached style" in the AST is not mechanically well-defined, so the protection
// against the class returning is the behavioural swap-and-diff guard, not this
// file.

// syntheticProbePalette builds a whole 19-token palette whose values are unique
// within the palette and — for two different `red` bytes — disjoint across a pair
// of palettes.
//
// Two shipped themes would not do: a hex both palettes happen to share survives a
// swap legitimately, and a token with the same value either side of the swap
// renders identically, so the assertion passes whether or not the site updated
// (§13.4 makes the same argument for the completeness guard).
//
// Every channel is deliberately THREE decimal digits (red ≥ 0xAA, green 0x81…,
// blue 0xC9…), so a rendered SGR core is fixed-width `38;2;RRR;GGG;BBB` and one
// token's core can never be a substring of another's — which would otherwise make
// the "the stale value is absent" half of each assertion pass vacuously.
func syntheticProbePalette(red uint8) theme.Theme {
	v := func(i int) theme.Token {
		return theme.Token{Value: fmt.Sprintf("#%02X%02X%02X", red, 0x80+i, 0xC8+i)}
	}
	return theme.Theme{
		TextPrimary:      v(1),
		TextSecondary:    v(2),
		TextTertiary:     v(3),
		TextMuted:        v(4),
		TextSubtle:       v(5),
		TextFaint:        v(6),
		TextOnSelection:  v(7),
		AccentPrimary:    v(8),
		AccentKey:        v(9),
		AccentMode:       v(10),
		AccentAttention:  v(11),
		StatePositive:    v(12),
		StateDestructive: v(13),
		Canvas:           v(14),
		BgSelection:      v(15),
		BgAttention:      v(16),
		BgSubtle:         v(17),
		Border:           v(18),
		TextOnAttention:  v(19),
	}
}

// probeThemeBefore / probeThemeAfter are the two disjoint palettes every
// assertion in this file swaps between: the model is built and rendered under
// "before" (which is what populates the once-assigned caches) and then restyled
// to "after".
func probeThemeBefore() theme.Theme { return syntheticProbePalette(0xAA) }
func probeThemeAfter() theme.Theme  { return syntheticProbePalette(0xBB) }

// newRestyleProbeModel builds a production-shaped model painted from `before`,
// with enough sessions and projects for BOTH lists to paginate, and renders both
// pages once.
//
// The render is not decoration: the caches under test are assigned at
// construction, so a model that is only rendered AFTER the swap would pass
// trivially. Rendering first is what makes the post-swap assertions meaningful.
func newRestyleProbeModel(t *testing.T, before theme.Theme) Model {
	t.Helper()
	return populateRestyleProbe(t, Build(Deps{Lister: fakeLister{}, Theme: theme.ConstantNomination(before)}))
}

// populateRestyleProbe is the probe's shared body: size the model, fill both lists
// with enough content to paginate, and render both pages under whatever palette the
// caller built it from.
//
// It takes an already-built model so a probe needing extra seams — the panel's
// enumerator, in newRestylePanelProbeModel — assembles its own Deps without this
// file growing a second copy of the population.
func populateRestyleProbe(t *testing.T, m Model) Model {
	t.Helper()
	const w, h = 120, 24

	sessions := make([]tmux.Session, 0, 60)
	for i := range 60 {
		sessions = append(sessions, tmux.Session{Name: nameN(i), Windows: 1})
	}
	projects := make([]project.Project, 0, 40)
	for i := range 40 {
		projects = append(projects, project.Project{
			Name: nameN(i),
			Path: "/Users/leeovery/Code/" + nameN(i),
		})
	}

	m.termWidth = w
	m.termHeight = h
	m.applySessions(sessions)
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())

	if m.sessionList.Paginator.TotalPages < 2 {
		t.Fatalf("probe setup: want a multi-page session list, got TotalPages=%d", m.sessionList.Paginator.TotalPages)
	}
	if m.projectList.Paginator.TotalPages < 2 {
		t.Fatalf("probe setup: want a multi-page project list, got TotalPages=%d", m.projectList.Paginator.TotalPages)
	}

	// Populate every construction-time cache by rendering both pages under the
	// pre-swap palette.
	m.activePage = PageSessions
	_ = m.viewSessionList()
	m.activePage = PageProjects
	_ = m.viewProjectList()
	m.activePage = PageSessions

	return m
}

// restyleTo swaps the model's active palette through the PRODUCTION entry point,
// ApplyTheme, which drives applyCanvasMode and its fan-out to
// applyProjectCanvasMode and styleFilterInput. This is the mechanism §11.1 names
// as the theme-swap path; nothing here is a test-only setter.
func restyleTo(m *Model, after theme.Theme) {
	m.ApplyTheme(after)
}

// probedList names one of the two PAGE bubbles/list instances the restyle path
// owns.
type probedList struct {
	name string
	list *list.Model
}

// probedLists returns the two page list instances the per-style probes run over.
// The panel's third instance exists only while the panel is open, so it is probed
// at the rendered-output granularity instead (restyledSurfaces).
func probedLists(m *Model) []probedList {
	return []probedList{
		{"sessions", &m.sessionList},
		{"projects", &m.projectList},
	}
}

// assertRepointed asserts a rendered run carries the post-swap SGR core and none
// of the pre-swap one — the two halves of "this cache was re-pointed", stated
// together so a run that lost its colour entirely cannot pass on the negative
// alone.
func assertRepointed(t *testing.T, what, rendered, want, stale string) {
	t.Helper()
	if !strings.Contains(rendered, want) {
		t.Errorf("%s does not carry the post-swap SGR %q — the restyle path did not re-point it: %q", what, want, escSeq(rendered))
	}
	if strings.Contains(rendered, stale) {
		t.Errorf("%s still carries the pre-swap SGR %q — the previous theme's colour survived the restyle: %q", what, stale, escSeq(rendered))
	}
}

// TestRestylePath_RepointsListOwnedStyles proves every bubbles/list-owned style
// §11.2 names is genuinely re-pointed by the restyle path: the help style, both
// pagination dot STYLES, the two rendered Paginator dot STRINGS (which list.New
// reads off those styles once at construction and never re-reads), the title bar
// and the title box — on the Sessions and the Projects list alike.
func TestRestylePath_RepointsListOwnedStyles(t *testing.T) {
	before, after := probeThemeBefore(), probeThemeAfter()
	m := newRestyleProbeModel(t, before)
	restyleTo(&m, after)

	for _, pl := range probedLists(&m) {
		t.Run(pl.name, func(t *testing.T) {
			// HelpStyle — the wrapper around the footer area, canvas-backed.
			assertRepointed(t, pl.name+" Styles.HelpStyle",
				pl.list.Styles.HelpStyle.Render("x"),
				tokenBgSeq(t, after.Canvas), tokenBgSeq(t, before.Canvas))

			// ActivePaginationDot — accent.primary over the canvas.
			activeDot := pl.list.Styles.ActivePaginationDot.String()
			assertRepointed(t, pl.name+" Styles.ActivePaginationDot foreground",
				activeDot, tokenFgSeq(t, after.AccentPrimary), tokenFgSeq(t, before.AccentPrimary))
			assertRepointed(t, pl.name+" Styles.ActivePaginationDot background",
				activeDot, tokenBgSeq(t, after.Canvas), tokenBgSeq(t, before.Canvas))

			// InactivePaginationDot — text.faint over the canvas.
			inactiveDot := pl.list.Styles.InactivePaginationDot.String()
			assertRepointed(t, pl.name+" Styles.InactivePaginationDot foreground",
				inactiveDot, tokenFgSeq(t, after.TextFaint), tokenFgSeq(t, before.TextFaint))
			assertRepointed(t, pl.name+" Styles.InactivePaginationDot background",
				inactiveDot, tokenBgSeq(t, after.Canvas), tokenBgSeq(t, before.Canvas))

			// The RENDERED dot strings the live paginator actually prints. These are
			// the trap: list.New snapshots them from the styles above at construction,
			// so re-pointing only the styles leaves the paginator printing the old
			// theme's dots forever.
			assertRepointed(t, pl.name+" Paginator.ActiveDot",
				pl.list.Paginator.ActiveDot, tokenFgSeq(t, after.AccentPrimary), tokenFgSeq(t, before.AccentPrimary))
			assertRepointed(t, pl.name+" Paginator.InactiveDot",
				pl.list.Paginator.InactiveDot, tokenFgSeq(t, after.TextFaint), tokenFgSeq(t, before.TextFaint))

			// NoItems — the zero-items body bubbles/list renders ITSELF, out of its
			// own hardcoded grey. It is reachable: viewProjectList only replaces the
			// empty body when NOT command-pending, so the command-pending +
			// zero-projects frame falls through to this style. No fixture renders that
			// state, so the §13.4 swap-and-diff guard is blind to it — this assertion
			// is its only cover.
			noItems := pl.list.Styles.NoItems.Render("x")
			assertRepointed(t, pl.name+" Styles.NoItems foreground",
				noItems, tokenFgSeq(t, after.TextMuted), tokenFgSeq(t, before.TextMuted))
			assertRepointed(t, pl.name+" Styles.NoItems background",
				noItems, tokenBgSeq(t, after.Canvas), tokenBgSeq(t, before.Canvas))

			// TitleBar — the canvas painted behind the section-header row and its
			// trailing gap row.
			assertRepointed(t, pl.name+" Styles.TitleBar",
				pl.list.Styles.TitleBar.Render("x"),
				tokenBgSeq(t, after.Canvas), tokenBgSeq(t, before.Canvas))

			// PaginationStyle — the centred wrapper the dot row is laid into.
			assertRepointed(t, pl.name+" Styles.PaginationStyle",
				pl.list.Styles.PaginationStyle.Render("x"),
				tokenBgSeq(t, after.Canvas), tokenBgSeq(t, before.Canvas))

			// Title — the one list-owned style no token paints: the section-header
			// surgery replaces the whole title line, so the correct end state is that
			// it emits NO colour at all. Asserting "carries the new theme" would be
			// wrong; asserting "carries no colour" is what keeps bubbles/list's own
			// hardcoded default palette (and any pre-swap value) out of the model.
			if titleRun := pl.list.Styles.Title.Render("x"); strings.ContainsRune(titleRun, '\x1b') {
				t.Errorf("%s Styles.Title emits colour %q — nothing paints the title box (the section header replaces the row), so it must carry none", pl.name, escSeq(titleRun))
			}
		})
	}
}

// TestRestylePath_RepointsBothFilterInputs proves both bubbles/list FilterInputs
// are re-pointed. Their styles are set through FilterInput.SetStyles — a
// write-back of a whole struct read off the input — so a list left out of
// styleFilterInput keeps typing in the previous theme's accent.
func TestRestylePath_RepointsBothFilterInputs(t *testing.T) {
	before, after := probeThemeBefore(), probeThemeAfter()
	m := newRestyleProbeModel(t, before)
	restyleTo(&m, after)

	wantSeq := tokenFgSeq(t, after.AccentAttention)
	staleSeq := tokenFgSeq(t, before.AccentAttention)

	for _, pl := range probedLists(&m) {
		t.Run(pl.name, func(t *testing.T) {
			styles := pl.list.FilterInput.Styles()
			assertRepointed(t, pl.name+" FilterInput Focused.Prompt",
				styles.Focused.Prompt.Render(filterPromptPrefix), wantSeq, staleSeq)
			assertRepointed(t, pl.name+" FilterInput Focused.Text",
				styles.Focused.Text.Render("query"), wantSeq, staleSeq)
			assertRepointed(t, pl.name+" FilterInput Cursor.Color",
				lipgloss.NewStyle().Foreground(styles.Cursor.Color).Render("x"), wantSeq, staleSeq)
		})
	}
}

// TestRestylePath_RepointsBothDelegates proves both row delegates are re-pointed.
// The assertion is on RENDERED rows rather than on the delegate struct:
// bubbles/list keeps the delegate unexported, and a row is the only observation
// that cannot pass while the cached delegate is stale.
//
// text.on-selection is the probe token because the two delegates are its only
// consumers on these two screens, so a hit is unambiguously a delegate-painted
// run rather than surrounding chrome.
func TestRestylePath_RepointsBothDelegates(t *testing.T) {
	before, after := probeThemeBefore(), probeThemeAfter()
	m := newRestyleProbeModel(t, before)
	restyleTo(&m, after)

	wantSeq := tokenFgSeq(t, after.TextOnSelection)
	staleSeq := tokenFgSeq(t, before.TextOnSelection)

	m.activePage = PageSessions
	assertRepointed(t, "SessionDelegate selected-row name", m.viewSessionList(), wantSeq, staleSeq)

	m.activePage = PageProjects
	assertRepointed(t, "ProjectDelegate selected-row name", m.viewProjectList(), wantSeq, staleSeq)
}

// TestRestylePath_RepointsPreviewChrome covers the offender the construction-time
// half of the sweep found: previewModel.th is a whole palette copied onto the
// preview once, in the Space handler that opens the page, and nothing re-points
// it — so the preview's chrome would keep painting the theme that was active when
// it was opened.
func TestRestylePath_RepointsPreviewChrome(t *testing.T) {
	before, after := probeThemeBefore(), probeThemeAfter()
	m := newRestyleProbeModel(t, before)

	pv := newPreviewModelForHelpers("alpha", []tmux.WindowGroup{{WindowIndex: 0, PaneIndices: []int{0}}}, 0, 0)
	pv.th = before
	m.preview = pv

	restyleTo(&m, after)

	assertRepointed(t, "previewModel chrome (accent.mode marker)",
		chromeLineForTest(m.preview),
		tokenFgSeq(t, after.AccentMode), tokenFgSeq(t, before.AccentMode))
}

// restylePanelProbeRows is the panel union size the probe below opens over: enough
// rows that the panel's own list PAGINATES at the probe's terminal height, so the
// rendered dot row — the exemplar of the once-assigned cache class, since
// bubbles/list reads its dot strings out of the styles at construction — is on the
// frame the scan reads.
const restylePanelProbeRows = 20

// newRestylePanelProbeModel is newRestyleProbeModel with the §9.1 slide-over OPEN
// over it, so all THREE bubbles/list instances the restyle path owns hold caches
// assigned under `before`.
//
// EVERY UNION ROW CARRIES `before`, which is what makes the open safe as set-up:
// opening applies the cursor row's theme, so a row painted from anything else would
// repaint the model off the palette the two page lists were just rendered under and
// the swap below would no longer be a swap. The panel is opened through the
// production `t` keypress rather than assembled here, and the closing View is the
// panel's own A-render — the counterpart of the two page renders
// populateRestyleProbe performs, and equally not optional: the caches under test are
// assigned once, so a surface rendered only after the swap passes trivially.
func newRestylePanelProbeModel(t *testing.T, before theme.Theme) Model {
	t.Helper()

	rows := make([]theme.Row, 0, restylePanelProbeRows)
	for i := range restylePanelProbeRows {
		rows = append(rows, theme.Row{Slug: arrowSlug(i), Source: theme.SourceBuiltin, Theme: before})
	}

	m := populateRestyleProbe(t, Build(newArrowPanelDeps(t, rows, arrowSlug(0))))
	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("probe setup: the panel did not open, so its list instance was never built")
	}
	if got := m.themePanel.list.Paginator.TotalPages; got < 2 {
		t.Fatalf("probe setup: want a multi-page panel list, got TotalPages=%d", got)
	}
	if m.activeTheme != before {
		t.Fatalf("probe setup: the open painted canvas %s, want the pre-swap %s", m.activeTheme.Canvas.Value, before.Canvas.Value)
	}
	_ = m.View()

	return m
}

// restyledSurface is one rendered surface the stale-colour scan runs over.
type restyledSurface struct {
	name     string
	rendered string
}

// restyledSurfaces renders the three lists the restyle path owns, each through the
// renderer production composes it with: the two page views, and the panel block at
// the content height overlayThemePanelOnContent renders it at.
//
// They are rendered SEPARATELY rather than read off one composed frame so a
// survivor names the surface it survived on; the composed frame would report only
// that some cell somewhere kept the old palette.
func restyledSurfaces(t *testing.T, m *Model) []restyledSurface {
	t.Helper()

	m.activePage = PageSessions
	sessions := m.viewSessionList()
	m.activePage = PageProjects
	projects := m.viewProjectList()
	m.activePage = PageSessions

	return []restyledSurface{
		{"sessions", sessions},
		{"projects", projects},
		{"theme panel", renderThemePanel(m.themePanel, m.contentHeight(), m.activeTheme, m.colourless)},
	}
}

// TestRestylePath_NoStaleColourSurvivesOnAnyList swaps the palette while the panel
// is OPEN and asserts no value of the pre-swap one survives on any of the three
// lists' rendered output.
//
// It is the file's per-style probes stated at the granularity §11.2's class
// actually bites at — a whole rendered surface rather than one named cache — and
// over all three instances rather than the two page lists: the panel's is the one
// §11.2 calls the worst case, assigned once at open and re-themed on every arrow.
// Every token of the pre-swap palette is scanned for, so a cache nobody thought to
// name is caught by the same pass as the ones that are.
//
// THE SWAP IS ApplyTheme, the production entry point the panel's arrow-preview,
// open and close all drive. A COMMIT is deliberately not it: a commit is a write,
// not a navigation (§9.2) — it recomputes rows and badges and never re-themes — so
// driving one here would assert over a frame nothing had swapped.
func TestRestylePath_NoStaleColourSurvivesOnAnyList(t *testing.T) {
	before, after := probeThemeBefore(), probeThemeAfter()
	m := newRestylePanelProbeModel(t, before)

	for _, surface := range restyledSurfaces(t, &m) {
		if !strings.Contains(surface.rendered, tokenBgSeq(t, before.Canvas)) {
			t.Fatalf("probe setup: the %s surface carries no pre-swap canvas, so a clean scan after the swap would prove nothing about it", surface.name)
		}
	}

	restyleTo(&m, after)

	for _, surface := range restyledSurfaces(t, &m) {
		t.Run(surface.name, func(t *testing.T) {
			if stale := staleRuns(t, surface.rendered, before); len(stale) > 0 {
				t.Errorf("the %s surface still renders %v from the pre-swap palette — a cached style on that list was never re-pointed: %q", surface.name, stale, escSeq(surface.rendered))
			}
			if !strings.Contains(surface.rendered, tokenBgSeq(t, after.Canvas)) {
				t.Errorf("the %s surface carries no post-swap canvas, so the clean scan above is an unpainted surface rather than a re-pointed one: %q", surface.name, escSeq(surface.rendered))
			}
		})
	}
}

// staleRuns returns every token of the given palette whose foreground or
// background run appears on the surface, named with the side it was found on.
//
// It scans the WHOLE palette rather than a list of named caches: a cache nobody
// thought to name is exactly the member this class loses, so the scan must not be
// derived from the same hand-maintained list it is meant to backstop.
func staleRuns(t *testing.T, rendered string, palette theme.Theme) []string {
	t.Helper()
	var found []string
	for _, tok := range palette.All() {
		if strings.Contains(rendered, tokenFgSeq(t, tok)) {
			found = append(found, tok.Name+" (foreground)")
		}
		if strings.Contains(rendered, tokenBgSeq(t, tok)) {
			found = append(found, tok.Name+" (background)")
		}
	}
	return found
}
