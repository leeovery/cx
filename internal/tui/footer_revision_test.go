package tui

import (
	"reflect"
	"slices"
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
)

// theming-system-8-14 — §14's footer keymap revision.
//
// §14 exists because the theming feature would otherwise be near-invisible:
// `--theme` and `portal theme list` are ruled out, the themes directory is silent
// and never seeded, built-in rows are indistinguishable from drop-ins, the
// reserved-slug set is invisible, and there is no active-theme indicator when the
// panel is closed. Putting `t theme` in BOTH footers is the answer, and it forces
// three further changes this file pins:
//
//   - `↑↓ navigate` comes OUT of both footers (§14.1) — arrows in a list are a
//     given, and they are the entry that genuinely deserves non-core status. It
//     stays listed in `?` help.
//   - `m` is promoted alongside `t` and shortened to `m multi` (§14.3), buying back
//     ~47px against an arithmetic that fits with ~5px spare and no headroom at the
//     reference width.
//   - Both surfaces are filtered IN LOCKSTEP through the same call-site filter
//     (§14.3) — advertising a key that only produces a blocked flash is the dead end
//     the proactive block exists to prevent.
//
// It also pins §14.4's degrade rule and the live defect that rule uncovered: the
// right-anchored assembler used to drop the RIGHT ANCHOR first, so at narrow widths
// Portal lost precisely the escape hatch (`? help`) that makes every dropped entry
// recoverable.
//
// §15.1 names these footer changes as the AMENDMENT to the MV spec's §12.2 keymap
// revision — the moved goldens across this package are that amendment landing, not
// a regression.
//
// No t.Parallel() — the package's shared canvas/mock helpers make parallelism unsafe.

// The §14.2 DECIDED FOOTERS, verbatim. Written out here rather than assembled from
// the descriptor: a test that composes the copy from the thing under test pins
// nothing, and this copy is the spec's, not the implementation's.
const (
	specSessionsFooterCluster = "⏎ attach · / filter · ␣ preview · s switch view · x projects · t theme · m multi"
	specProjectsFooterCluster = "⏎ new session · x sessions · e edit · / filter · t theme"
	specFooterHelpAnchor      = "? help"
)

// footerRowVisible returns a rendered footer's key row, ANSI-stripped. The footer
// is two rows (the 1px top rule then the single key row), so the key row is the
// LAST line; stripping lets the copy be asserted independent of the canvas/colour
// SGR painted across every cell.
func footerRowVisible(t *testing.T, footer string) string {
	t.Helper()
	lines := strings.Split(footer, "\n")
	if len(lines) != 2 {
		t.Fatalf("a condensed footer must be 2 rows (rule + key row), got %d:\n%s", len(lines), footer)
	}
	return footerVisible(lines[1])
}

// splitFooterRow splits a visible key row into its left cluster and its right
// anchor. The row is `<cluster><flex spacer><anchor>` when the anchor survives and
// `<cluster><pad>` when it does not, so the anchor is recognised by its suffix and
// the spacer/pad is trimmed off the cluster.
func splitFooterRow(row string) (cluster, anchor string) {
	if trimmed := strings.TrimRight(row, " "); strings.HasSuffix(trimmed, specFooterHelpAnchor) {
		return strings.TrimRight(strings.TrimSuffix(trimmed, specFooterHelpAnchor), " "), specFooterHelpAnchor
	}
	return strings.TrimRight(row, " "), ""
}

// TestFooterRevision_SessionsPinnedCopy: it renders the pinned Sessions footer.
//
// §14.2 pins the row verbatim — `⏎ attach · / filter · ␣ preview · s switch view ·
// x projects · t theme · m multi` with a right-aligned `? help`.
func TestFooterRevision_SessionsPinnedCopy(t *testing.T) {
	row := footerRowVisible(t, renderSessionsFooter(sessionsKeymap(), referenceFooterWidth, testDarkTheme(t), false))
	cluster, anchor := splitFooterRow(row)

	if cluster != specSessionsFooterCluster {
		t.Errorf("Sessions footer cluster:\n got  %q\n want %q (§14.2)", cluster, specSessionsFooterCluster)
	}
	if anchor != specFooterHelpAnchor {
		t.Errorf("Sessions footer anchor = %q, want the right-aligned %q (§14.2)", anchor, specFooterHelpAnchor)
	}
}

// TestFooterRevision_ProjectsPinnedCopy: it renders the pinned Projects footer.
//
// §14.2 pins the row verbatim — `⏎ new session · x sessions · e edit · / filter ·
// t theme` with a right-aligned `? help`. `t theme` lands here because theme is a
// GLOBAL setting (§9.6), which is also why the Projects footer needs its own
// call-site filter.
func TestFooterRevision_ProjectsPinnedCopy(t *testing.T) {
	row := footerRowVisible(t, renderProjectsFooter(projectsKeymap(), referenceFooterWidth, testDarkTheme(t), false))
	cluster, anchor := splitFooterRow(row)

	if cluster != specProjectsFooterCluster {
		t.Errorf("Projects footer cluster:\n got  %q\n want %q (§14.2)", cluster, specProjectsFooterCluster)
	}
	if anchor != specFooterHelpAnchor {
		t.Errorf("Projects footer anchor = %q, want the right-aligned %q (§14.2)", anchor, specFooterHelpAnchor)
	}
}

// TestFooterRevision_NavigateIsNonCore: it drops navigate from the footers and
// keeps it in help.
//
// §14.1: arrows in a list are a given, and they are the entry that genuinely
// deserves non-core status — so `↑↓ navigate` leaves BOTH footers while staying
// listed in BOTH help bodies. The distinction (core vs non-core) applied to the
// right thing.
func TestFooterRevision_NavigateIsNonCore(t *testing.T) {
	th := testDarkTheme(t)
	for _, tc := range []struct {
		page    string
		footer  string
		help    string
		entries []keymapEntry
	}{
		{
			page:    "sessions",
			footer:  footerRowVisible(t, renderSessionsFooter(sessionsKeymap(), referenceFooterWidth, th, false)),
			help:    footerVisible(helpModalBody(sessionsKeymap(), th, false)),
			entries: sessionsKeymap(),
		},
		{
			page:    "projects",
			footer:  footerRowVisible(t, renderProjectsFooter(projectsKeymap(), referenceFooterWidth, th, false)),
			help:    footerVisible(helpModalBody(projectsKeymap(), th, false)),
			entries: projectsKeymap(),
		},
	} {
		t.Run(tc.page, func(t *testing.T) {
			if strings.Contains(tc.footer, "navigate") {
				t.Errorf("the %s footer still lists `↑↓ navigate` (§14.1 drops it):\n%s", tc.page, tc.footer)
			}
			if !strings.Contains(tc.help, "Move selection") {
				t.Errorf("the %s help body must still list the nav row (§14.1 keeps it in help):\n%s", tc.page, tc.help)
			}
			for _, e := range tc.entries {
				if e.Key == "↑↓" && e.Core {
					t.Errorf("the %s nav entry is still Core; §14.1 makes it non-core (listed, not footed)", tc.page)
				}
			}
		})
	}
}

// TestFooterRevision_MultiLabelIsShort: it shortens the multi label.
//
// §14.3: the footer label is `m multi`, NOT `m multi-select` — the ~47px that buys
// back is what makes the row fit at the reference width with no headroom. The HELP
// label is untouched (`Multi-select mode`), which is the footer-vs-help split the
// descriptor already models.
func TestFooterRevision_MultiLabelIsShort(t *testing.T) {
	th := testDarkTheme(t)
	row := footerRowVisible(t, renderSessionsFooter(sessionsKeymap(), referenceFooterWidth, th, false))

	if !strings.Contains(row, "m multi") {
		t.Errorf("the Sessions footer must carry `m multi` (§14.3):\n%s", row)
	}
	if strings.Contains(row, "multi-select") {
		t.Errorf("the Sessions footer must NOT carry the long `m multi-select` label (§14.3):\n%s", row)
	}
	if body := footerVisible(helpModalBody(sessionsKeymap(), th, false)); !strings.Contains(body, "Multi-select mode") {
		t.Errorf("the help body keeps the long `Multi-select mode` label (§14.3 shortens the FOOTER only):\n%s", body)
	}
}

// footerRevisionSessionsModel builds a Sessions-page model at the reference footer
// width, in the requested colour state. The dimensions are assigned directly (not
// through a tea.WindowSizeMsg) so the footer resolves against the region the
// fixture declares and nothing has resized on the way in.
func footerRevisionSessionsModel(t *testing.T, colourless bool) Model {
	t.Helper()
	m := NewModelWithSessions([]tmux.Session{
		{Name: "alpha", Windows: 1},
		{Name: "bravo", Windows: 2},
	})
	m.termWidth = referenceFooterWidth + 2*Hinset
	m.termHeight = 30 + 2*Vinset
	m.colourless = colourless
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	if got := m.contentWidth(); got != referenceFooterWidth {
		t.Fatalf("fixture: content width is %d, want the reference %d", got, referenceFooterWidth)
	}
	return m
}

// footerRevisionProjectsModel is the same fixture landed on the Projects page with
// a populated project list, so renderProjectsFooterForFilterState resolves to the
// standard §6.3 footer rather than the §11.1 empty-state replacement.
func footerRevisionProjectsModel(t *testing.T, colourless bool) Model {
	t.Helper()
	m := footerRevisionSessionsModel(t, colourless)
	projects := []project.Project{{Path: "/p/one", Name: "one"}, {Path: "/p/two", Name: "two"}}
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.projectList.Select(0)
	m.activePage = PageProjects
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	return m
}

// keymapHasKey reports whether a descriptor slice carries the given Key glyph.
func keymapHasKey(entries []keymapEntry, key string) bool {
	for _, e := range entries {
		if e.Key == key {
			return true
		}
	}
	return false
}

// TestFooterRevision_BlockedThemeKeyFilteredInLockstep: it filters t from footer
// and help together, on BOTH pages.
//
// §14.3: the footer is filtered in lockstep with `?` help, through the same
// call-site filter. Advertising a key in the footer that only produces a blocked
// flash is the dead end the proactive block (§9.10) exists to prevent, and help and
// footer disagreeing about one key is a live inconsistency.
//
// §9.10 names only `sessionsHelpKeymap()`, but §14.2 puts `t theme` in the Projects
// footer too — hence the second call site, asserted here as an equal partner rather
// than as a Sessions afterthought.
//
// BOTH surfaces are asserted through their COMPOSED render (the page view with the
// help modal up, and renderXFooterForFilterState), never by feeding the filtered
// slice to helpModalBody by hand: composing the slice in the test would assert the
// filter function and leave the wiring — which call-site slice each page's view
// actually passes to the help modal — entirely unpinned.
func TestFooterRevision_BlockedThemeKeyFilteredInLockstep(t *testing.T) {
	// helpBodyThroughView renders the page view with the help modal open, which is
	// the only way to reach the modal's descriptor argument from outside.
	helpBodyThroughView := func(view func(m Model) string, m Model) string {
		m.modal = modalHelp
		return footerVisible(view(m))
	}

	for _, tc := range []struct {
		page     string
		model    func(t *testing.T, colourless bool) Model
		keymap   func(m Model) []keymapEntry
		footer   func(m Model) string
		view     func(m Model) string
		clusters string
	}{
		{
			page:     "sessions",
			model:    footerRevisionSessionsModel,
			keymap:   Model.sessionsHelpKeymap,
			footer:   Model.renderSessionsFooterForFilterState,
			view:     Model.viewSessionList,
			clusters: specSessionsFooterCluster,
		},
		{
			page:     "projects",
			model:    footerRevisionProjectsModel,
			keymap:   Model.projectsHelpKeymap,
			footer:   Model.renderProjectsFooterForFilterState,
			view:     Model.viewProjectList,
			clusters: specProjectsFooterCluster,
		},
	} {
		t.Run(tc.page, func(t *testing.T) {
			// Control: with colour available `t` is functional, so BOTH surfaces list it.
			live := tc.model(t, false)
			if !keymapHasKey(tc.keymap(live), "t") {
				t.Errorf("the %s call-site keymap must LIST t when the panel is available", tc.page)
			}
			liveRow := footerRowVisible(t, tc.footer(live))
			if cluster, _ := splitFooterRow(liveRow); cluster != tc.clusters {
				t.Errorf("the unblocked %s footer cluster:\n got  %q\n want %q", tc.page, cluster, tc.clusters)
			}
			if body := helpBodyThroughView(tc.view, live); !strings.Contains(body, "Theme picker") {
				t.Errorf("the unblocked %s help modal must list the Theme picker row:\n%s", tc.page, body)
			}

			// Blocked: under NO_COLOR the panel previews nothing, so `t` is proactively
			// blocked (§9.10) and BOTH surfaces drop it in lockstep.
			blocked := tc.model(t, true)
			if keymapHasKey(tc.keymap(blocked), "t") {
				t.Errorf("the %s call-site keymap must DROP t under NO_COLOR (§9.10)", tc.page)
			}
			blockedRow := footerRowVisible(t, tc.footer(blocked))
			if strings.Contains(blockedRow, "t theme") {
				t.Errorf("the blocked %s footer still advertises `t theme` (§14.3 filters in lockstep):\n%s", tc.page, blockedRow)
			}
			if body := helpBodyThroughView(tc.view, blocked); strings.Contains(body, "Theme picker") {
				t.Errorf("the blocked %s help modal still lists the Theme picker row (its view passes the UNFILTERED descriptor):\n%s", tc.page, body)
			}
		})
	}
}

// TestFooterRevision_BlockedMultiKeyFilteredInLockstep: it filters m from footer
// and help together.
//
// The `m` filter predates §14 (it dropped the help row only, `m` being non-core).
// §14.1 promotes `m` to Core, so the SAME call-site filter now has to reach the
// footer as well — otherwise the footer advertises a key whose only effect is the
// blocked flash the proactive entry block raises.
func TestFooterRevision_BlockedMultiKeyFilteredInLockstep(t *testing.T) {
	th := testDarkTheme(t)

	// Control: a supported terminal keeps `m` on both surfaces (the filter is inert).
	supported := unsupportedResolvedModel(t, ghosttyIdentity())
	if !keymapHasKey(supported.sessionsHelpKeymap(), "m") {
		t.Fatal("precondition: a supported terminal must keep the m entry")
	}
	supportedRow := footerRowVisible(t, renderSessionsFooter(supported.sessionsHelpKeymap(), referenceFooterWidth, th, false))
	if !strings.Contains(supportedRow, "m multi") {
		t.Errorf("a supported terminal's footer must carry `m multi`:\n%s", supportedRow)
	}

	// Blocked: a resolved-unsupported terminal, not already in the mode.
	blocked := unsupportedResolvedModel(t, appleTerminalIdentity())
	if !blocked.DetectUnsupported() || blocked.multiSelectMode {
		t.Fatal("precondition: the fixture must resolve unsupported and not be in multi-select")
	}
	if keymapHasKey(blocked.sessionsHelpKeymap(), "m") {
		t.Error("the call-site keymap must DROP m on a resolved-unsupported terminal")
	}
	blockedRow := footerRowVisible(t, renderSessionsFooter(blocked.sessionsHelpKeymap(), referenceFooterWidth, th, false))
	if strings.Contains(blockedRow, "m multi") {
		t.Errorf("the blocked footer still advertises `m multi` (§14.3 filters in lockstep):\n%s", blockedRow)
	}
	if body := footerVisible(helpModalBody(blocked.sessionsHelpKeymap(), th, false)); strings.Contains(body, "Multi-select mode") {
		t.Errorf("the blocked help body still lists the multi-select row:\n%s", body)
	}
}

// TestFooterRevision_BudgetMatchesFilteredRender: it reserves exactly what it
// renders in a blocked state.
//
// §14.3's consequence for the height budget: the footer's reserved height and its
// rendered height must resolve against the IDENTICAL entry set. Filtering only ever
// removes entries, so a blocked-state footer is strictly narrower and needs no
// separate budget case — but only if the budget reads the same filtered slice the
// render does.
//
// The HEIGHT equality is degenerate today (both footers are two rows at every
// width, since §14.4 degrades on one line rather than wrapping) and is asserted
// because §14.3 states it. The assertion that actually bites is the one above it:
// the composed view must render byte-for-byte the FILTERED footer, checked in a
// state where the filtered and unfiltered renders provably differ.
func TestFooterRevision_BudgetMatchesFilteredRender(t *testing.T) {
	for _, tc := range []struct {
		page       string
		model      func(t *testing.T, colourless bool) Model
		filtered   func(m Model) string
		unfiltered func(m Model) string
		composed   func(m Model) string
		budget     func(m Model) int
	}{
		{
			page:  "sessions",
			model: footerRevisionSessionsModel,
			filtered: func(m Model) string {
				return renderSessionsFooter(m.sessionsHelpKeymap(), m.contentWidth(), m.themeState.active, m.colourless)
			},
			unfiltered: func(m Model) string {
				return renderSessionsFooter(sessionsKeymap(), m.contentWidth(), m.themeState.active, m.colourless)
			},
			composed: Model.renderSessionsFooterForFilterState,
			budget:   func(m Model) int { return m.sessionFooterHeight(m.contentWidth()) },
		},
		{
			page:  "projects",
			model: footerRevisionProjectsModel,
			filtered: func(m Model) string {
				return renderProjectsFooter(m.projectsHelpKeymap(), m.contentWidth(), m.themeState.active, m.colourless)
			},
			unfiltered: func(m Model) string {
				return renderProjectsFooter(projectsKeymap(), m.contentWidth(), m.themeState.active, m.colourless)
			},
			composed: Model.renderProjectsFooterForFilterState,
			budget:   func(m Model) int { return m.projectFooterHeight(m.contentWidth()) },
		},
	} {
		t.Run(tc.page, func(t *testing.T) {
			m := tc.model(t, true)

			// Precondition: in this blocked state the filtered and unfiltered footers
			// genuinely differ, so "reads the filtered slice" is a claim with teeth.
			filtered := tc.filtered(m)
			if filtered == tc.unfiltered(m) {
				t.Fatal("precondition: the blocked footer must differ from the unfiltered one")
			}

			// The composed view renders the FILTERED footer…
			if got := tc.composed(m); got != filtered {
				t.Errorf("the composed %s footer is not the filtered render:\n got=%q\nwant=%q", tc.page, got, filtered)
			}
			// …and the height budget reserves exactly that render's rows.
			if got, want := tc.budget(m), lipgloss.Height(filtered); got != want {
				t.Errorf("the %s footer budget reserves %d rows, the filtered render is %d", tc.page, got, want)
			}
		})
	}
}

// TestFooterRevision_StaticDescriptorsUnfiltered: it keeps the static descriptors
// unfiltered.
//
// The filter lives at the CALL SITE and nowhere else: `keymap_dispatch_guard_test`
// probes the STATIC descriptors with detection unwired and `colourless` false, so a
// filter re-homed into sessionsKeymap()/projectsKeymap() is exactly what breaks
// descriptor↔dispatch parity.
//
// This folds the former TestPanelEntry_LeavesDescriptorsUnfiltered, which asserted
// only that the nullary descriptors were unchanged — tautological while no filter
// existed. The claim is constraining now: the same blocked states that DO strip the
// call-site slices must leave the static descriptors whole.
func TestFooterRevision_StaticDescriptorsUnfiltered(t *testing.T) {
	wantSessions, wantProjects := sessionsKeymap(), projectsKeymap()

	assertStatic := func(t *testing.T, state string) {
		t.Helper()
		if got := sessionsKeymap(); !reflect.DeepEqual(got, wantSessions) {
			t.Errorf("[%s] the static Sessions descriptor changed:\ngot  %+v\nwant %+v", state, got, wantSessions)
		}
		if got := projectsKeymap(); !reflect.DeepEqual(got, wantProjects) {
			t.Errorf("[%s] the static Projects descriptor changed:\ngot  %+v\nwant %+v", state, got, wantProjects)
		}
		for _, key := range []string{"t", "m"} {
			if !keymapHasKey(sessionsKeymap(), key) {
				t.Errorf("[%s] the static Sessions descriptor must always carry %q", state, key)
			}
		}
		if !keymapHasKey(projectsKeymap(), "t") {
			t.Errorf("[%s] the static Projects descriptor must always carry t", state)
		}
	}

	// The NO_COLOR block strips `t` from BOTH call-site slices…
	blockedTheme := footerRevisionProjectsModel(t, true)
	if keymapHasKey(blockedTheme.sessionsHelpKeymap(), "t") || keymapHasKey(blockedTheme.projectsHelpKeymap(), "t") {
		t.Fatal("precondition: NO_COLOR must strip t from both call-site keymaps")
	}
	assertStatic(t, "theme blocked")

	// …and the unsupported-terminal block strips `m` from the Sessions slice.
	blockedMulti := unsupportedResolvedModel(t, appleTerminalIdentity())
	if keymapHasKey(blockedMulti.sessionsHelpKeymap(), "m") {
		t.Fatal("precondition: an unsupported terminal must strip m from the Sessions call-site keymap")
	}
	assertStatic(t, "multi blocked")
}

// helpAnchorWidth is the rendered width of the `? help` right anchor — the
// threshold every §14.4 assertion is expressed against, MEASURED off the production
// render rather than assumed from a glyph count.
func helpAnchorWidth(t *testing.T, th theme.Theme) int {
	t.Helper()
	_, anchor := splitFooterEntries(sessionsKeymap())
	if anchor == nil {
		t.Fatal("the Sessions descriptor must carry the right-aligned ? help anchor")
	}
	return lipgloss.Width(renderFooterEntry(*anchor, th.AccentPrimary, th, false))
}

// allowedFooterClusters enumerates every left cluster §14.4's degrade rule permits
// for a descriptor: the full cluster, each leading PREFIX followed by the ` · …`
// drop marker, the bare marker, and nothing. A rendered cluster outside this set is
// either a wrapped row, a truncated label, or entries dropped from the wrong end.
func allowedFooterClusters(entries []keymapEntry) []string {
	core, _ := splitFooterEntries(entries)
	parts := make([]string, 0, len(core))
	for _, e := range core {
		parts = append(parts, e.Key+footerKeyLabelGap+e.Action)
	}
	allowed := []string{"", footerEllipsis, strings.Join(parts, footerEntrySeparator)}
	for n := 1; n < len(parts); n++ {
		allowed = append(allowed, strings.Join(parts[:n], footerEntrySeparator)+footerEntrySeparator+footerEllipsis)
	}
	return allowed
}

// footerClusterEntryCount counts the whole entries a rendered cluster carries, so a
// width sweep can assert the drop is monotonic. The bare drop marker and the empty
// cluster both carry none.
func footerClusterEntryCount(cluster string) int {
	trimmed := strings.TrimSuffix(cluster, footerEntrySeparator+footerEllipsis)
	if trimmed == "" || trimmed == footerEllipsis {
		return 0
	}
	return len(strings.Split(trimmed, footerEntrySeparator))
}

// TestFooterRevision_HelpAnchorSurvivesNarrowing: it degrades right-to-left and
// never drops help.
//
// §14.4: entries drop from the RIGHT until the row fits, and `? help` is NEVER
// dropped while it fits — it is right-aligned and it is the escape hatch that makes
// every dropped entry recoverable, since the help modal lists the full keymap
// regardless of footer width. A user on a narrow terminal loses the reminder, not
// the capability.
//
// This inverts the previous assembler, whose narrow path dropped the right anchor
// FIRST and padded the left cluster — losing precisely the escape hatch.
func TestFooterRevision_HelpAnchorSurvivesNarrowing(t *testing.T) {
	th := testDarkTheme(t)
	anchorW := helpAnchorWidth(t, th)
	allowed := allowedFooterClusters(sessionsKeymap())

	// A CONTIGUOUS sweep from comfortably wide down to the exact width at which the
	// anchor alone fits. Contiguous rather than sampled because §14.4's drop is "one
	// entry at a time": every entry is several cells wide, so narrowing by a single
	// cell can never legitimately cost two.
	previous := -1
	for w := 120; w >= anchorW; w-- {
		row := footerRowVisible(t, renderSessionsFooter(sessionsKeymap(), w, th, false))
		cluster, anchor := splitFooterRow(row)

		if anchor != specFooterHelpAnchor {
			t.Fatalf("at width %d the ? help anchor was dropped; §14.4 never drops it while it fits:\n%q", w, row)
		}
		if !slices.Contains(allowed, cluster) {
			t.Fatalf("at width %d the cluster %q is not a legal §14.4 degrade step (wrapped, truncated, or dropped from the wrong end)", w, cluster)
		}
		count := footerClusterEntryCount(cluster)
		if previous >= 0 {
			if count > previous {
				t.Fatalf("at width %d the cluster grew back to %d entries from %d — the drop must be monotonic as the row narrows", w, count, previous)
			}
			if previous-count > 1 {
				t.Fatalf("at width %d the cluster lost %d entries in one cell (from %d to %d) — §14.4 drops one at a time", w, previous-count, previous, count)
			}
		}
		previous = count
	}
	if previous != 0 {
		t.Errorf("at the anchor's own width the cluster still carries %d entries, want none", previous)
	}
}

// TestFooterRevision_ExtremeNarrowLadder: it renders help alone then empty at the
// extremes.
//
// §14.4's two bottom rungs, pinned as §2.7 already pins its own steps: at exactly
// the anchor's width the row is the anchor ALONE, right-aligned; one cell below
// that the row renders EMPTY — degrade-never-break, and Portal's documented
// 40-column minimum sits well above both.
func TestFooterRevision_ExtremeNarrowLadder(t *testing.T) {
	th := testDarkTheme(t)
	anchorW := helpAnchorWidth(t, th)

	t.Run("anchor alone", func(t *testing.T) {
		footer := renderSessionsFooter(sessionsKeymap(), anchorW, th, false)
		row := footerRowVisible(t, footer)
		if got := strings.TrimSpace(row); got != specFooterHelpAnchor {
			t.Errorf("at width %d the row = %q, want the bare %q", anchorW, got, specFooterHelpAnchor)
		}
		if got := lipgloss.Width(strings.Split(footer, "\n")[1]); got != anchorW {
			t.Errorf("at width %d the row is %d cells wide, want exactly %d", anchorW, got, anchorW)
		}
	})

	t.Run("empty below the anchor", func(t *testing.T) {
		w := anchorW - 1
		footer := renderSessionsFooter(sessionsKeymap(), w, th, false)
		row := footerRowVisible(t, footer)
		if got := strings.TrimSpace(row); got != "" {
			t.Errorf("at width %d the row = %q, want an empty row (§14.4 below the anchor's width)", w, got)
		}
		if got := lipgloss.Width(strings.Split(footer, "\n")[1]); got != w {
			t.Errorf("at width %d the empty row is %d cells wide, want exactly %d", w, got, w)
		}
	})
}

// TestFooterRevision_LabelsAreNeverTruncated: it never truncates a label.
//
// §14.4: never wrap, and never truncate a label — a half-rendered key hint is worse
// than an absent one, since a truncated `x proje…` advertises nothing while costing
// the same space. The ` · …` drop MARKER is not a label, so it stays.
//
// Swept across every width from comfortably wide down to one cell, on BOTH pages:
// each row must be exactly two lines (rule + key row, never a wrapped third), must
// never overflow, and its cluster must be one of the legal degrade steps.
func TestFooterRevision_LabelsAreNeverTruncated(t *testing.T) {
	th := testDarkTheme(t)
	for _, tc := range []struct {
		page    string
		entries []keymapEntry
		render  func(entries []keymapEntry, w int) string
	}{
		{page: "sessions", entries: sessionsKeymap(), render: func(e []keymapEntry, w int) string {
			return renderSessionsFooter(e, w, th, false)
		}},
		{page: "projects", entries: projectsKeymap(), render: func(e []keymapEntry, w int) string {
			return renderProjectsFooter(e, w, th, false)
		}},
	} {
		t.Run(tc.page, func(t *testing.T) {
			allowed := allowedFooterClusters(tc.entries)
			for w := 1; w <= 120; w++ {
				footer := tc.render(tc.entries, w)
				lines := strings.Split(footer, "\n")
				if len(lines) != 2 {
					t.Fatalf("at width %d the %s footer has %d rows, want 2 (no wrap):\n%s", w, tc.page, len(lines), footer)
				}
				for i, line := range lines {
					if lw := lipgloss.Width(line); lw != w {
						t.Errorf("at width %d the %s footer line %d is %d cells wide, want exactly %d", w, tc.page, i, lw, w)
					}
				}
				cluster, _ := splitFooterRow(footerVisible(lines[1]))
				if !slices.Contains(allowed, cluster) {
					t.Errorf("at width %d the %s cluster %q truncates a label or drops from the wrong end", w, tc.page, cluster)
				}
			}
		})
	}
}
