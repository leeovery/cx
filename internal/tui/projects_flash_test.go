package tui

import (
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/spawn"
)

// Tests for task 8-12: the §14A Projects transient-flash slot. The §11 notice
// band was a SESSIONS-only arbiter, yet §9.6 binds `t` on Projects, §14.2 puts
// `t theme` in its footer, and all six of §14A's theme flashes are reachable
// there — so a flash raised on Projects set state and rendered nothing.
//
// Projects gets the FLASH CONTENDER ALONE (§14A), arbitrated against the
// existing §11.4 command-pending banner and rendered through the SAME
// renderNoticeBand primitive the Sessions band uses. No other contender has a
// Projects analogue.
//
// No t.Parallel() — the package's shared canvas/mock helpers make parallelism
// unsafe across these tests.

// newProjectsFlashModel builds a Projects-page Model seeded with the sample
// projects at the given size, sized so the band/footer budgets are applied. It
// takes the Model options so the NO_COLOR carve-out variant can be built from the
// same fixture rather than a second near-copy.
func newProjectsFlashModel(t *testing.T, w, h int, opts ...Option) Model {
	t.Helper()
	m := New(fakeLister{}, opts...)
	m.termWidth = w
	m.termHeight = h
	m.activePage = PageProjects
	projects := sampleProjects()
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	return m
}

// newSessionsFlashPeer builds the SESSIONS-page peer of a Projects flash model at
// the same size, so the two pages' arbitrated band renders can be compared
// byte-for-byte (the §14A "same band, other page" contract). Both models take the
// constructor's dark seed with no nomination applied, so the palette matches
// without either side naming a theme.
func newSessionsFlashPeer(t *testing.T, w, h int) Model {
	t.Helper()
	m := New(fakeLister{})
	m.termWidth = w
	m.termHeight = h
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	return m
}

// bandSlotRowCount counts the `▌` band rows in the composed Projects view's
// NOTICE SLOT — the region between the title separator and the `Projects` section
// header. The scan stops at the section header because the selected project ROW
// draws its cursor with the same glyph further down the page; the single-slot rule
// is about the slot, so the count has to be about the slot too.
func bandSlotRowCount(view string) int {
	n := 0
	for line := range strings.SplitSeq(ansi.Strip(view), "\n") {
		if strings.Contains(line, "Projects") {
			break
		}
		if strings.HasPrefix(line, noticeBarGlyph) {
			n++
		}
	}
	return n
}

// sectionHeaderRow returns line 0 of a bubbles/list view — the row
// applyProjectsSectionHeader swaps its header into.
func sectionHeaderRow(listView string) string {
	return strings.SplitN(listView, "\n", 2)[0]
}

// TestProjectsFlash_RendersInTheBandSlot asserts a flash raised while the
// Projects page is active renders the §11 `▌` band beneath the title separator
// (above the section header, with the one blank breathing row between) and that
// its bytes are IDENTICAL to the Sessions band for the same message and width.
func TestProjectsFlash_RendersInTheBandSlot(t *testing.T) {
	const flash = "__PROJECTS_FLASH__"
	m := newProjectsFlashModel(t, 90, 30)
	m.setFlash(flash)

	// Byte-identical to the Sessions band: same role, tint, glyph, width.
	peer := newSessionsFlashPeer(t, 90, 30)
	peer.setFlash(flash)
	if got, want := m.renderActiveProjectNoticeBand(), peer.renderActiveNoticeBand(); got != want {
		t.Errorf("Projects flash band differs from the Sessions band for the same message/width:\n got %q\nwant %q", got, want)
	}

	// Placement: under the title separator rule, above the section header, with
	// exactly one blank breathing row between band and section header.
	lines := strings.Split(ansi.Strip(m.viewProjectList()), "\n")
	ruleIdx := -1
	for i, l := range lines {
		if strings.Contains(l, strings.Repeat(headerRuleGlyph, 4)) {
			ruleIdx = i
			break
		}
	}
	bandIdx := lineIndexContaining(lines, flash)
	sectionIdx := lineIndexContaining(lines, "Projects")
	if ruleIdx < 0 || bandIdx < 0 || sectionIdx < 0 {
		t.Fatalf("missing a landmark: rule=%d band=%d section=%d\n%s", ruleIdx, bandIdx, sectionIdx, strings.Join(lines, "\n"))
	}
	if bandIdx <= ruleIdx {
		t.Errorf("band index %d must be > separator-rule index %d (band under the separator)", bandIdx, ruleIdx)
	}
	if sectionIdx-bandIdx != 2 {
		t.Errorf("section header is %d rows below the band, want 2 (band → blank → section header)", sectionIdx-bandIdx)
	}
	if blank := lines[bandIdx+1]; strings.TrimSpace(blank) != "" {
		t.Errorf("row between the band and section header must be blank, got %q", blank)
	}
	// The ⚠ glyph treatment travels with the role.
	if !strings.Contains(lines[bandIdx], flashWarningGlyph) {
		t.Errorf("Projects flash band missing the %q status glyph: %q", flashWarningGlyph, lines[bandIdx])
	}
}

// TestProjectsFlash_RecomputesListHeight asserts the Projects list height shrinks
// by the slot's MEASURED height when the flash appears and is restored when it
// clears — asserted through projectBandHeight, with no separate arithmetic, so
// the one-row-per-delegate pagination invariant holds by construction.
func TestProjectsFlash_RecomputesListHeight(t *testing.T) {
	m := newProjectsFlashModel(t, 90, 30)
	base := m.projectList.Height()

	m.setFlash("__HEIGHT_FLASH__")
	slot := m.projectBandHeight()
	if slot < 2 {
		t.Fatalf("Projects flash slot height = %d, want >=2 (band + blank breathing row)", slot)
	}
	if got, want := m.projectList.Height(), base-slot; got != want {
		t.Errorf("project list height with the flash = %d, want %d (base %d − slot %d)", got, want, base, slot)
	}

	m.clearFlash()
	if got := m.projectBandHeight(); got != 0 {
		t.Errorf("projectBandHeight after clear = %d, want 0", got)
	}
	if got := m.projectList.Height(); got != base {
		t.Errorf("project list height after clear = %d, want the base %d (slot released)", got, base)
	}
}

// TestProjectsFlash_WinsTheSlotOverCommandPending asserts the flash outranks the
// §11.4 command-pending banner for its duration, and the banner returns to the
// slot once the flash clears.
func TestProjectsFlash_WinsTheSlotOverCommandPending(t *testing.T) {
	const flash = "__OUTRANKS__"
	m := newCommandPendingTestModel(t, 90, 30, sampleProjects(), []string{"npm", "run", "dev"})

	role, _, ok := m.activeProjectNoticeBand()
	if !ok || role != bandCommand {
		t.Fatalf("baseline: activeProjectNoticeBand = (role %v, ok %v), want (bandCommand, true)", role, ok)
	}

	m.setFlash(flash)
	role, message, ok := m.activeProjectNoticeBand()
	if !ok {
		t.Fatalf("no band arbitrated while a flash is live")
	}
	if role != bandWarning || message != flash {
		t.Errorf("arbitrated band = (role %v, message %q), want (bandWarning, %q) — the flash outranks the banner", role, message, flash)
	}

	m.clearFlash()
	role, _, ok = m.activeProjectNoticeBand()
	if !ok || role != bandCommand {
		t.Errorf("after the flash clears: activeProjectNoticeBand = (role %v, ok %v), want the banner back (bandCommand, true)", role, ok)
	}
}

// TestProjectsFlash_SingleSlot asserts the band never CO-RENDERS with the
// command-pending banner: exactly one band row set is present, carrying the
// flash and none of the banner's caret / fixed text / command chip.
func TestProjectsFlash_SingleSlot(t *testing.T) {
	const flash = "__SINGLE_SLOT__"
	m := newCommandPendingTestModel(t, 90, 30, sampleProjects(), []string{"npm", "run", "dev"})
	m.setFlash(flash)

	view := m.viewProjectList()
	visible := ansi.Strip(view)
	if !strings.Contains(visible, flash) {
		t.Errorf("flash must render while it holds the slot:\n%s", visible)
	}
	for _, banned := range []string{commandBandText, commandBandCaret, "npm run dev"} {
		if strings.Contains(visible, banned) {
			t.Errorf("command-pending banner element %q must NOT co-render with the flash:\n%s", banned, visible)
		}
	}
	if got, want := bandSlotRowCount(view), lipgloss.Height(m.renderActiveProjectNoticeBand()); got != want {
		t.Errorf("notice slot carries %d `%s` band rows, want %d (exactly one arbitrated band)", got, noticeBarGlyph, want)
	}
	// The slot is the one arbitrated band plus its single blank breathing row —
	// never two stacked bands.
	if got, want := lipgloss.Height(m.renderProjectBandSlot()), lipgloss.Height(m.renderActiveProjectNoticeBand())+1; got != want {
		t.Errorf("Projects band slot height = %d, want %d (one band + one blank row)", got, want)
	}
}

// TestProjectsFlash_ActionableKeyClearsAndFallsThrough asserts an actionable
// keypress on Projects clears the flash AND still reaches its normal handler —
// "one key, one intent", exactly as the Sessions arm behaves.
func TestProjectsFlash_ActionableKeyClearsAndFallsThrough(t *testing.T) {
	t.Run("x clears the flash and still switches to Sessions", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m.setFlash("__CLEAR_ME__")

		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: 'x', Text: "x"})
		if m.flashText != "" {
			t.Errorf("actionable key must clear the flash: flashText = %q, want empty", m.flashText)
		}
		if m.activePage != PageSessions {
			t.Errorf("x must still fall through to the page switch: active page = %d, want PageSessions", m.activePage)
		}
	})

	t.Run("e clears the flash and still opens the edit modal", func(t *testing.T) {
		m := projectsDispatchModel(t)
		m.setFlash("__CLEAR_ME__")

		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: 'e', Text: "e"})
		if m.flashText != "" {
			t.Errorf("actionable key must clear the flash: flashText = %q, want empty", m.flashText)
		}
		if m.modal != modalEditProject {
			t.Errorf("e must still fall through to the edit modal: modal = %v, want modalEditProject", m.modal)
		}
	})
}

// TestProjectsFlash_SurvivesWindowSize asserts a NON-key event does not clear a
// Projects flash — window size, focus, and blur never enter the KeyPressMsg arm.
func TestProjectsFlash_SurvivesWindowSize(t *testing.T) {
	const flash = "__SURVIVES__"
	for _, tc := range []struct {
		name string
		msg  tea.Msg
	}{
		{"window size", tea.WindowSizeMsg{Width: 100, Height: 40}},
		{"focus", tea.FocusMsg{}},
		{"blur", tea.BlurMsg{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newProjectsFlashModel(t, 90, 30)
			m.setFlash(flash)

			updated, _ := m.Update(tc.msg)
			mm, ok := updated.(Model)
			if !ok {
				t.Fatalf("Update returned %T, want tui.Model", updated)
			}
			if mm.flashText != flash {
				t.Errorf("flashText after %s: got %q, want %q (non-key events never clear)", tc.name, mm.flashText, flash)
			}
		})
	}
}

// TestProjectsFlash_PageEnteredByMessageIsSized closes the frame-overflow hole the
// §14A slot opened. A page is NOT only ever entered by a keypress: a MESSAGE can
// enter one too, and a message never runs the actionable-key flash clear.
//
// The live route is the §10.2 concurrent cold boot (driven end-to-end by
// TestProjectsFlash_ColdBootWarningsLandOnASizedProjectsFrame below): the flash is
// raised while Sessions is active, then the post-restore refetch's SessionsMsg
// flips the page to Projects with the flash still live. Whichever page the flash is
// raised on, BOTH lists must already reserve the slot they would render, so the
// page that is entered is sized for the band it is carrying.
func TestProjectsFlash_PageEnteredByMessageIsSized(t *testing.T) {
	m := newProjectsFlashModel(t, 90, 30)
	base := m.projectList.Height()

	// Raise the flash while SESSIONS is active, then flip to Projects with no
	// keypress in between — the message route, in miniature.
	m.activePage = PageSessions
	m.setFlash("__ENTERED_BY_MESSAGE__")
	m.activePage = PageProjects

	slot := m.projectBandHeight()
	if slot < 2 {
		t.Fatalf("Projects flash slot height = %d, want >=2 (band + blank breathing row)", slot)
	}
	if got, want := m.projectList.Height(), base-slot; got != want {
		t.Errorf("project list height after a flash raised off-page = %d, want %d (base %d − slot %d)", got, want, base, slot)
	}
}

// TestProjectsFlash_ColdBootWarningsLandOnASizedProjectsFrame drives the REAL route
// the off-page flash arrives by, end to end: a cold TUI boot with soft bootstrap
// warnings and zero sessions after restore — a degraded or first-run install.
//
//  1. transitionFromLoading lands PageSessions and RETURNS EARLY on this route
//     (progressReceiver != nil), deliberately leaving defaultPageEvaluated false —
//     it has not landed the final page.
//  2. surfaceBufferedWarnings raises the warnings flash there.
//  3. The post-restore refetch's SessionsMsg sets sessionsLoaded and runs
//     evaluateDefaultPage, which flips to PageProjects for an empty list. It is a
//     MESSAGE, so the actionable-key clear never runs and the flash is still live
//     (the 3 s tick has not fired — the refetch resolves in milliseconds).
//
// The composed Projects frame must still fit its content region: fillCanvas CLAMPS
// at contentHeight, so a list budgeted for no band silently loses the footer off the
// bottom of the frame.
func TestProjectsFlash_ColdBootWarningsLandOnASizedProjectsFrame(t *testing.T) {
	const warningLine = "the session saver is not running"

	// §10.2 cold/TUI route: the non-nil progress receiver is the discriminator; the
	// project store makes Projects a real landing surface.
	var model tea.Model = New(postloadStubLister{},
		WithServerStarted(true),
		WithProgressReceiver(func() tea.Msg { return nil }),
		WithProjectStore(stubProjectStore{}),
	)
	model, _ = model.Update(tea.WindowSizeMsg{Width: 80, Height: 24})
	// projectsLoaded must be true BEFORE the transition, as production delivers it
	// (Init dispatches loadProjects on frame one).
	model, _ = model.Update(ProjectsLoadedMsg{Projects: oneProjectLoaded()})

	// Both loading gates close: the transition lands Sessions undecided and the
	// warnings flash is raised there.
	model, _ = model.Update(LoadingMinElapsedMsg{})
	model, _ = model.Update(BootstrapCompleteMsg{Warnings: []BootstrapWarning{{Lines: []string{warningLine}}}})
	interim := model.(Model)
	if interim.ActivePage() != PageSessions {
		t.Fatalf("route invariant: expected PageSessions after both loading gates, got %v", interim.ActivePage())
	}
	if interim.flashText == "" {
		t.Fatal("route invariant: expected the post-load warnings flash to be live on Sessions")
	}

	// The post-restore refetch's SessionsMsg, fed directly rather than by draining
	// the transition batch (which also carries the flash's 3 s auto-clear tick, a
	// real timer). Zero sessions → evaluateDefaultPage flips to Projects.
	model, _ = interim.Update(SessionsMsg{})
	final := model.(Model)
	if final.ActivePage() != PageProjects {
		t.Fatalf("route invariant: a zero-session cold boot must land on PageProjects, got %v", final.ActivePage())
	}
	if final.flashText == "" {
		t.Fatal("route invariant: a SessionsMsg is not a keypress — the flash must still be live on the Projects landing")
	}

	if got, want := lipgloss.Height(final.viewProjectList()), final.contentHeight(); got > want {
		t.Errorf("composed Projects view is %d rows, want <= %d (the content region fillCanvas clamps to): the list was not sized for the flash band it renders\n%s",
			got, want, ansi.Strip(final.viewProjectList()))
	}
}

// TestProjectsFlash_TickClearsWithGenerationGuard asserts a Projects flash rides
// the EXISTING shared tick: a matching generation clears it, a superseded one is
// dropped. No Projects-specific duration, kind, or tick exists.
func TestProjectsFlash_TickClearsWithGenerationGuard(t *testing.T) {
	t.Run("a matching tick clears the flash", func(t *testing.T) {
		m := newProjectsFlashModel(t, 90, 30)
		m.setFlash("timeout-me") // gen 1

		updated, _ := m.Update(flashTickMsg{Gen: m.flashGen})
		mm := updated.(Model)
		if mm.flashText != "" {
			t.Errorf("matching tick must clear the Projects flash: flashText = %q, want empty", mm.flashText)
		}
	})

	t.Run("a superseded tick does not clear a newer flash", func(t *testing.T) {
		m := newProjectsFlashModel(t, 90, 30)
		m.setFlash("first")  // gen 1
		m.setFlash("second") // gen 2 — supersedes

		updated, _ := m.Update(flashTickMsg{Gen: 1})
		mm := updated.(Model)
		if mm.flashText != "second" {
			t.Errorf("superseded tick must not early-clear: flashText = %q, want %q", mm.flashText, "second")
		}
	})
}

// TestProjectsFlash_OnlyTheFlashContender asserts Projects gained the FLASH
// CONTENDER ALONE (§14A): with every Sessions-only contender's model state set
// and no flash live, the Projects slot stays empty and none of their copy leaks
// onto the page.
func TestProjectsFlash_OnlyTheFlashContender(t *testing.T) {
	m := newProjectsFlashModel(t, 90, 30, WithInitialBurstOpening(1, 3))
	// Every Sessions-only contender's state, all at once: the §11.3 no-tags
	// signpost, §5 multi-select, a resolved-unsupported named terminal, and the
	// §6-5 in-burst Opening band (seeded by the option above).
	m.byTagSignpost = true
	m.multiSelectMode = true
	m.selectedSessions = map[string]struct{}{"alpha": {}}
	m.detectResolved = true
	m.detectResolution = spawn.ResolutionUnsupported
	m.detectIdentity = appleTerminalIdentity()

	if _, _, ok := m.activeProjectNoticeBand(); ok {
		t.Errorf("no Sessions-only contender may claim the Projects slot")
	}
	view := m.viewProjectList()
	if slot := m.renderProjectBandSlot(); slot != "" {
		t.Errorf("Projects band slot must be empty with no flash live, got %q", slot)
	}
	if got := bandSlotRowCount(view); got != 0 {
		t.Errorf("Projects notice slot carries %d `%s` band rows with no flash live, want 0", got, noticeBarGlyph)
	}
	for _, banned := range []string{byTagSignpostText, "selected", multiSelectCancelHint, unsupportedDocsHint, "Opening"} {
		if strings.Contains(ansi.Strip(view), banned) {
			t.Errorf("Sessions-only contender copy %q leaked onto the Projects page:\n%s", banned, ansi.Strip(view))
		}
	}
}

// TestProjectsFlash_Colourless asserts the NO_COLOR carve-out (§2.5) on the
// Projects band: hue and tint drop, while the `▌` bar and the ⚠ / ✓ status
// glyphs survive — the band's state stays glyph-backed, never colour-only.
func TestProjectsFlash_Colourless(t *testing.T) {
	for _, tc := range []struct {
		name  string
		set   func(*Model, string)
		glyph string
	}{
		{"warning", (*Model).setFlash, flashWarningGlyph},
		{"success", (*Model).setSuccessFlash, flashSuccessGlyph},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newProjectsFlashModel(t, 90, 30, WithColourless(true))
			tc.set(&m, "__COLOURLESS__")

			band := m.renderActiveProjectNoticeBand()
			if band != ansi.Strip(band) {
				t.Errorf("NO_COLOR Projects band must carry no SGR colour sequences; got raw %q", band)
			}
			if !strings.HasPrefix(band, noticeBarGlyph) {
				t.Errorf("NO_COLOR Projects band must keep the %q left-bar: %q", noticeBarGlyph, band)
			}
			if !strings.Contains(band, tc.glyph) {
				t.Errorf("NO_COLOR Projects band must keep the %q status glyph: %q", tc.glyph, band)
			}
		})
	}
}

// TestProjectsFlash_SessionsBandUnchanged asserts this task left the SESSIONS
// arbiter alone — including its deliberate multi-select CO-RENDER exception,
// which has no Projects analogue and must not have been "unified" away.
func TestProjectsFlash_SessionsBandUnchanged(t *testing.T) {
	const flash = "__SESSIONS_UNCHANGED__"

	t.Run("the multi-select co-render exception survives", func(t *testing.T) {
		m := noticeBandModel("alpha-row")
		m.multiSelectMode = true
		m.setFlash(flash)

		role, message, ok := m.activeNoticeBand()
		if !ok || role != bandWarning || message != flash {
			t.Errorf("Sessions arbiter = (role %v, message %q, ok %v), want the flash co-rendering with multi-select", role, message, ok)
		}
	})

	t.Run("the Sessions list still reserves the slot it always did", func(t *testing.T) {
		m := noticeBandModel("alpha-row")
		_, base := m.SessionListSize()

		m.setFlash(flash)
		slot := m.sessionBandHeight()
		if _, got := m.SessionListSize(); got != base-slot {
			t.Errorf("Sessions list height with the flash = %d, want %d (base %d − slot %d)", got, base-slot, base, slot)
		}

		m.clearFlash()
		if _, got := m.SessionListSize(); got != base {
			t.Errorf("Sessions list height after clear = %d, want the base %d", got, base)
		}
	})
}

// TestProjectsFlash_FilterHeaderPrecedenceUnchanged pins the deliberate
// NON-change: §14A's filter-line precedence flip (theme flashes outranking the
// filter line) is PHASE 9's. An applied Projects filter still owns its
// section-header row byte-for-byte while a flash holds the band slot beneath it —
// the two are different rows in this phase, and nothing yet suppresses either.
func TestProjectsFlash_FilterHeaderPrecedenceUnchanged(t *testing.T) {
	const query = "portal"
	m := newProjectsFlashModel(t, 90, 30)
	m, _ = pressProject(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	for _, r := range query {
		m, _ = pressProject(t, m, tea.KeyPressMsg{Code: r, Text: string(r)})
	}
	m, _ = pressProject(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.projectList.FilterState() != list.FilterApplied {
		t.Fatalf("precondition: project filter state = %v, want FilterApplied", m.projectList.FilterState())
	}
	// Compare the section-header ROW alone: the flash legitimately shortens the
	// list body beneath it (the slot's reserve), which is the F10 recompute, not a
	// precedence change.
	baseline := sectionHeaderRow(m.applyProjectsSectionHeader(m.projectList.View()))

	m.setFlash("__FILTER_PRECEDENCE__")
	if got := sectionHeaderRow(m.applyProjectsSectionHeader(m.projectList.View())); got != baseline {
		t.Errorf("a flash must not change the Projects filter header row (the §14A filter-line flip is Phase 9's):\n got %q\nwant %q", got, baseline)
	}
	if !strings.Contains(ansi.Strip(m.viewProjectList()), "__FILTER_PRECEDENCE__") {
		t.Errorf("the flash must still render in the band slot beneath an applied filter:\n%s", ansi.Strip(m.viewProjectList()))
	}
}
