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

	m := Build(Deps{Lister: fakeLister{}, Theme: theme.ConstantNomination(before)})
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

// probedList names one of the two bubbles/list instances the restyle path owns.
type probedList struct {
	name string
	list *list.Model
}

// probedLists returns the two list instances under test. The panel's third
// instance is a later phase's; it is deliberately not pre-built here.
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
