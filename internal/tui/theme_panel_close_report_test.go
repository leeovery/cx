package tui

import (
	"io"
	"os"
	"slices"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
)

// The failed-commit rule's CLOSE REPORT: the failed-commit state has to SURVIVE the panel
// closing, and composed naively it does not.
//
// `Esc` is the only way out and it re-resolves from persisted state, so the
// very next keypress both clears the panel's message and drops the theme the user
// chose — with no `●` movement to signal it (the failed-commit rule forbids that) and nothing
// on the main screen. "Reported rather than silent" would hold for exactly one keystroke.
//
// So closing with a failure outstanding raises a main-screen flash, and RAISING IT
// DISCHARGES THE STATE — it is the report the state exists to produce. The two
// edges either side of that are decided in opposite directions and each is a named
// test below: a FORCED CLOSE has both flashes due at once into a single-slot
// band and the commit flash wins, while `Ctrl-C` is an accepted UNDELIVERED report
// with `theme: commit failed` in the log as the record.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// specThemeNotSavedFlash is the pinned copy's close-report copy, written out VERBATIM here
// rather than read from the production constant — a test that asserts a constant
// against itself pins nothing, and this copy is the spec's, not the
// implementation's.
const specThemeNotSavedFlash = "theme not saved — see portal.log"

// newCloseReportModel is this file's standard fixture: the failed-commit model with
// the write ALREADY ATTEMPTED and failed, so the panel is open with the failure
// outstanding and the close below has something to report.
//
// The persister is returned still primed to fail, so a test that wants a
// successful retry clears the error itself.
func newCloseReportModel(t *testing.T) (Model, *fakeThemePersister) {
	t.Helper()

	m, persister := newFailedCommitModel(t)
	m, _ = pressSlotKey(t, m, slotDarkPress)
	requireCommitFailedMessage(t, m)
	if !m.themeState.commitFailed {
		t.Fatal("fixture: the failed commit left nothing outstanding, so a close reports nothing")
	}
	return m, persister
}

// requireReportRaised fails unless the model carries the pinned copy's close report as an
// ordinary theme-origin warning flash, and unless the state that produced it has
// been DISCHARGED — the two halves are one act, so they are asserted
// together everywhere the report is due.
func requireReportRaised(t *testing.T, m Model) {
	t.Helper()

	if got := m.flashText; got != specThemeNotSavedFlash {
		t.Errorf("the close raised %q, want §14A's %q", got, specThemeNotSavedFlash)
	}
	if m.flashOrigin != flashOriginTheme {
		t.Errorf("the report carries origin %v, want the theme origin — it claims the band over a filter line (§14A)", m.flashOrigin)
	}
	if m.flashKind != flashWarning {
		t.Errorf("the report carries kind %v, want the ordinary warning flash", m.flashKind)
	}
	if m.themeState.commitFailed {
		t.Error("the report was raised with the failure still outstanding; raising it DISCHARGES the state (§9.13)")
	}
}

// flashTickFrom resolves a command to the flash tick it carries, tolerating a
// tea.Batch wrapper around it.
//
// The report's tick currently travels UNWRAPPED wherever it is asserted — tea.Batch
// compacts the nil sibling the forced close's pre-step folds it against — but the
// wrapping is the runtime's business rather than the behaviour under test, and a
// second command joining it on any of these paths would change the shape without
// changing what the tick does.
func flashTickFrom(cmd tea.Cmd) (flashTickMsg, bool) {
	if cmd == nil {
		return flashTickMsg{}, false
	}
	switch msg := cmd().(type) {
	case flashTickMsg:
		return msg, true
	case tea.BatchMsg:
		for _, child := range msg {
			if tick, ok := flashTickFrom(child); ok {
				return tick, true
			}
		}
	}
	return flashTickMsg{}, false
}

// requireReportTick fails unless the command a close returned carries the report's
// auto-clear tick, stamped with the generation the report was raised under.
//
// The tick's IDENTITY is the assertion, not merely a non-nil command: a close path
// can return a command for reasons of its own, so a nil check would be satisfied by
// one that schedules no auto-clear at all, and the report would stand on the band
// until the next actionable key. The generation is checked with it because a tick
// stamped with a stale one is dropped by the guard and clears nothing.
func requireReportTick(t *testing.T, m Model, cmd tea.Cmd) {
	t.Helper()

	tick, ok := flashTickFrom(cmd)
	if !ok {
		t.Fatalf("no auto-clear tick reached Update's return (the close returned nothing at all: %t); the report inherits the standard flash lifecycle (§9.13)", cmd == nil)
	}
	if tick.Gen != m.flashGen {
		t.Errorf("the tick carries generation %d, want the live %d — a stale generation is dropped by the guard and the report never clears", tick.Gen, m.flashGen)
	}
}

// TestCloseReport_RaisesTheFlash: it raises the pinned report on close.
//
// The failed-commit rule: "closing the panel with a failed commit outstanding raises a
// main-screen flash: `theme not saved — see portal.log`". The user is left on the main screen
// reading it, on whichever page they were on — which is the whole of what stops the
// revert being silent.
//
// The TICK is asserted because the report takes the STANDARD flash lifecycle rather
// than the panel message's bespoke one: it is a main-screen flash like every other,
// so it auto-clears on the shared timer and its generation guard drops a superseded
// tick.
func TestCloseReport_RaisesTheFlash(t *testing.T) {
	m, _ := newCloseReportModel(t)
	gen := m.flashGen

	m, cmd := closePanelForTest(t, m)

	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	requireReportRaised(t, m)
	requireFlashBandVisible(t, m, specThemeNotSavedFlash)
	if got := themeNotSavedFlash; got != specThemeNotSavedFlash {
		t.Errorf("the pinned constant is %q, want §14A's %q", got, specThemeNotSavedFlash)
	}
	if got, want := m.flashGen, gen+1; got != want {
		t.Errorf("the report left the flash generation at %d, want %d — it rides the shared counter", got, want)
	}
	requireReportTick(t, m, cmd)

	// The lifecycle it inherited, driven: a superseded tick is dropped and the
	// matching one clears the report.
	superseded, _ := m.Update(flashTickMsg{Gen: m.flashGen - 1})
	if got := superseded.(Model).flashText; got != specThemeNotSavedFlash {
		t.Errorf("a superseded tick left the report %q, want it standing", got)
	}
	cleared, _ := m.Update(flashTickMsg{Gen: m.flashGen})
	if got := cleared.(Model).flashText; got != "" {
		t.Errorf("the matching tick left the report %q, want it cleared", got)
	}
}

// closeReportFloorCrossings are the two ways the geometry rule's resize condition can cross
// the render floor, each with the geometry copy it would raise on its own.
var closeReportFloorCrossings = []struct {
	name         string
	region       func() (contentW, contentH int)
	wantGeometry string
}{
	{name: "below the width floor", region: geometryBelowWidthFloor, wantGeometry: specNarrowClosedFlash},
	{name: "below the height floor", region: geometryBelowHeightFloor, wantGeometry: specShortClosedFlash},
}

// TestCloseReport_ForcedCloseCommitFlashWins: it wins over the geometry flash on a
// forced close.
//
// The failed-commit rule: "on a forced close both flashes are due at once, and the
// failed-commit flash wins. The notice band has one slot, and the two report
// different things: a geometry event the user can see for themselves — their
// terminal just got smaller and the panel vanished — versus an unsaved setting they
// must act on."
//
// Losing the geometry flash costs nothing; losing the commit flash on the one path
// where the user cannot reopen the panel to retry is exactly the failure the failed-commit
// rule closes. The state is discharged either way, because the report was made.
//
// The TICK is asserted here for a reason that is particular to this path: the resize
// is handled in a pre-step of Update, so the report's auto-clear reaches the runtime
// only by being folded onto the arm that returns. A close that raised the report and
// dropped its tick would leave the band standing until the next actionable key, on
// the one path where the user's terminal has just shrunk out from under them.
func TestCloseReport_ForcedCloseCommitFlashWins(t *testing.T) {
	for _, tc := range closeReportFloorCrossings {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newCloseReportModel(t)
			contentW, contentH := tc.region()

			m, cmd := resizeForTestCmd(t, m, contentW, contentH)

			requireForcedClose(t, m, specThemeNotSavedFlash)
			requireReportRaised(t, m)
			requireReportTick(t, m, cmd)
			if got := m.flashText; got == tc.wantGeometry {
				t.Errorf("the forced close raised the geometry copy %q, want the report — the band has ONE slot and the report is the one the user must act on (§9.13)", got)
			}
		})
	}
}

// TestCloseReport_ForcedCloseGeometryFlashSurvives: it keeps the geometry flash when
// nothing is outstanding.
//
// The commit flash winning is scoped to the case where one is DUE. With nothing
// outstanding the geometry rule's forced close is untouched — its own pinned per-dimension
// copy, and no report about a theme that was saved.
//
// It also keeps the geometry flash's LIFECYCLE, which is the opposite of the
// report's: it schedules no auto-clear and stands until the next actionable key,
// because a geometry event the user can see for themselves is the right thing to
// leave on the screen while they are still resizing. The assertion is on the message
// rather than on a nil command, so it keeps its meaning if a close path ever returns
// a command of its own.
func TestCloseReport_ForcedCloseGeometryFlashSurvives(t *testing.T) {
	for _, tc := range closeReportFloorCrossings {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newCommitFailureFixture(t)
			if m.themeState.commitFailed {
				t.Fatal("fixture: the model already carries an outstanding failure")
			}
			contentW, contentH := tc.region()

			m, cmd := resizeForTestCmd(t, m, contentW, contentH)

			requireForcedClose(t, m, tc.wantGeometry)
			if tick, ok := flashTickFrom(cmd); ok {
				t.Errorf("the geometry flash scheduled the auto-clear tick %+v; it clears on the next actionable key instead (§9.8)", tick)
			}
			if m.themeState.commitFailed {
				t.Error("the forced close left a failure outstanding where none was")
			}
		})
	}
}

// requireCloseIsSilent fails unless a close raised nothing at all: no flash, no
// generation bump, no scheduled tick.
func requireCloseIsSilent(t *testing.T, m Model, cmd tea.Cmd, gen uint64) {
	t.Helper()

	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	if got := m.flashText; got != "" {
		t.Errorf("the close raised %q, want nothing — only an OUTSTANDING failure is reported (§9.13)", got)
	}
	if m.flashGen != gen {
		t.Errorf("the close moved the flash generation to %d, want the untouched %d — it raised no flash", m.flashGen, gen)
	}
	if cmd != nil {
		t.Errorf("the close scheduled %T, want nothing", cmd)
	}
}

// TestCloseReport_DischargedOnRaise: it discharges the state.
//
// The failed-commit rule: "raising the flash discharges the state — it is the report the state
// exists to produce, so once made the state has done its job. Without that, reopening the
// panel and pressing `Esc` would re-fire the flash about a failure already reported,
// on every close for the life of the process."
//
// So the SECOND close is the assertion: same model, same panel, nothing new failed,
// and nothing raised.
func TestCloseReport_DischargedOnRaise(t *testing.T) {
	m, _ := newCloseReportModel(t)

	m, _ = closePanelForTest(t, m)
	requireReportRaised(t, m)

	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("`t` did not re-open the panel, so the second close proves nothing")
	}
	gen := m.flashGen

	m, cmd := closePanelForTest(t, m)

	requireCloseIsSilent(t, m, cmd, gen)
	if m.themeState.commitFailed {
		t.Error("the second close left a failure outstanding; the first close discharged it")
	}
}

// TestCloseReport_SilentWhenNothingOutstanding: it raises nothing with no failure.
//
// The ordinary close is untouched by any of this: nothing failed, so there is
// nothing to report, and a flash about an unsaved theme on a panel that saved
// nothing would be a lie the user cannot check.
func TestCloseReport_SilentWhenNothingOutstanding(t *testing.T) {
	m, _ := newCommitFailureFixture(t)
	if m.themeState.commitFailed {
		t.Fatal("fixture: the model already carries an outstanding failure")
	}
	gen := m.flashGen

	m, cmd := closePanelForTest(t, m)

	requireCloseIsSilent(t, m, cmd, gen)
}

// TestCloseReport_SuccessfulRetryIsSilent: it raises nothing after a successful
// retry.
//
// The failed-commit rule: "because a successful retry clears it, a `d` that fails followed by
// an `l` that succeeds raises no flash — the user is not told a theme was not saved when it
// was." The state was already discharged by the landed write, so the close finds
// nothing outstanding and this task adds nothing to that path.
func TestCloseReport_SuccessfulRetryIsSilent(t *testing.T) {
	m, persister := newCloseReportModel(t)
	persister.err = nil

	m, _ = pressSlotKey(t, m, slotLightPress)
	if m.themeState.commitFailed {
		t.Fatal("the successful commit left the failure outstanding, so the silence below is not this task's")
	}
	gen := m.flashGen

	m, cmd := closePanelForTest(t, m)

	requireCloseIsSilent(t, m, cmd, gen)
	requireSlotCommits(t, persister,
		slotCommit{slug: commitFailureTarget, member: theme.MemberDark},
		slotCommit{slug: commitFailureTarget, member: theme.MemberLight},
	)
}

// TestCloseReport_CtrlCIsAnUndeliveredReport: it delivers nothing on `Ctrl-C`.
//
// The failed-commit rule: "`Ctrl-C` with a failure outstanding is accepted as an undelivered
// report. It is the one exit the entry-condition rule keeps live inside the panel, and the
// main screen is going away, so there is nowhere to raise a flash. The log is the record —
// `theme: commit failed` is already written — and the alternative, a post-TUI stderr warning,
// would put a message about a colour preference on the same channel Portal reserves for
// bootstrap failures."
//
// So: it quits, it raises nothing, it writes nothing to stderr, and it leaves the
// state UNDISCHARGED — there is no report to discharge against.
func TestCloseReport_CtrlCIsAnUndeliveredReport(t *testing.T) {
	m, _ := newCloseReportModel(t)
	gen := m.flashGen

	var cmd tea.Cmd
	stderr := captureStderrForTest(t, func() {
		updated, quit := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		m, cmd = updated.(Model), quit
	})

	if !isQuitCmd(cmd) {
		t.Fatalf("Ctrl-C returned %T, want tea.Quit — it is the one exit §9.7 keeps live inside the panel", cmd)
	}
	if !m.themePanel.open {
		t.Error("Ctrl-C closed the panel; it quits from inside it, which is why no close hook runs and nothing is discharged")
	}
	if got := m.flashText; got != "" {
		t.Errorf("Ctrl-C raised %q; the main screen is going away, so there is nowhere to report", got)
	}
	if m.flashGen != gen {
		t.Errorf("Ctrl-C moved the flash generation to %d, want the untouched %d", m.flashGen, gen)
	}
	if stderr != "" {
		t.Errorf("Ctrl-C wrote %q to stderr; the log is the record, and stderr is the channel Portal reserves for bootstrap failures", stderr)
	}
	if !m.themeState.commitFailed {
		t.Error("Ctrl-C discharged the outstanding failure; no report was made, so there is nothing to discharge against")
	}
}

// captureStderrForTest runs fn with os.Stderr redirected and returns everything
// written to it.
//
// The whole file descriptor is swapped rather than a writer seam being injected,
// because the assertion is that NOTHING reaches the stream — a seam would only prove
// that nothing reached the seam. The package's tests never run in parallel, so the
// process-global swap is safe here.
func captureStderrForTest(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("open a pipe for stderr: %v", err)
	}
	original := os.Stderr
	os.Stderr = w
	fn()
	os.Stderr = original
	if err := w.Close(); err != nil {
		t.Fatalf("close the stderr pipe: %v", err)
	}
	written, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read the captured stderr: %v", err)
	}
	if err := r.Close(); err != nil {
		t.Fatalf("close the stderr reader: %v", err)
	}
	return string(written)
}

// TestCloseReport_RevertStands: it still reverts to persisted state.
//
// The failed-commit rule: "the revert itself is correct and stays — the write did not land, so
// the theme is not persisted and `Esc` resolving to persisted state is right — but the
// user is told, on the surface they are left looking at."
//
// The flash is the ONLY thing that changed about the close: the previewed theme is
// still discarded, the raw keys a failed write left untouched are still untouched,
// and nothing further reaches the persister.
func TestCloseReport_RevertStands(t *testing.T) {
	m, persister := newCloseReportModel(t)
	previewed := m.themeState.active
	persisted := themePanelRowFor(t, m, "aurora").Row.Theme
	keys := m.themeState.keys
	if previewed == persisted {
		t.Fatal("fixture: the previewed theme is the persisted one, so a revert is invisible")
	}

	m, _ = closePanelForTest(t, m)

	requireReportRaised(t, m)
	if m.themeState.active != persisted {
		t.Errorf("the close rendered %s, want the resolved persisted %s — the write did not land, so `Esc` resolving to persisted state is right (§9.13)", themeLabel(m.themeState.active), themeLabel(persisted))
	}
	if m.themeState.keys != keys {
		t.Errorf("the close left keys %+v, want the untouched %+v", m.themeState.keys, keys)
	}
	requireSlotCommits(t, persister, slotCommit{slug: commitFailureTarget, member: theme.MemberDark})
}

// TestCloseReport_ProjectsFlashSlot: it reports on Projects too.
//
// The pinned copy gave Projects the transient-flash contender precisely so these signals are
// not Sessions-only, and closing the panel is reachable from both pages (the panel-open rule
// binds `t` on each). A report that vanished on Projects would be the silent revert the
// failed-commit rule closes, reached by another route — and worse than a missing band, because
// the discharge happens whether or not one rendered.
func TestCloseReport_ProjectsFlashSlot(t *testing.T) {
	rows := arrowValidRows(t, 4)
	persister := &fakeThemePersister{err: errThemeCommitFailed}
	deps := commitPairPanelDeps(t, rows)
	deps.ThemePersister = persister
	m := openCommitPanel(t, deps, PageProjects, rows[1].Slug)
	m = arrowToThemeRow(t, m, rows[2].Slug)

	m, _ = pressSlotKey(t, m, slotDarkPress)
	if !m.themeState.commitFailed {
		t.Fatal("fixture: the commit did not fail, so the close reports nothing")
	}

	m, cmd := closePanelForTest(t, m)

	requireReportRaised(t, m)
	requireReportTick(t, m, cmd)
	if m.activePage != PageProjects {
		t.Fatalf("the close moved the active page to %d, want it left on Projects", m.activePage)
	}
	role, message, ok := m.activeProjectNoticeBand()
	if !ok || role != bandWarning || message != specThemeNotSavedFlash {
		t.Errorf("the Projects band is (role %v, message %q, ok %v), want (bandWarning, %q, true)", role, message, ok, specThemeNotSavedFlash)
	}
	if got := ansi.Strip(m.viewProjectList()); !strings.Contains(got, specThemeNotSavedFlash) {
		t.Errorf("the composed Projects frame carries no %q band:\n%s", specThemeNotSavedFlash, got)
	}
}

// TestCloseReport_OutranksFilterLine: it claims the band with a filter applied.
//
// The pinned copy: "the filter line is the one contender above flash that can be live
// throughout a panel open/use/close, and the theme flashes take precedence over it"
// — because "the failed-commit report would never reach the band, and because
// raising the flash DISCHARGES the outstanding state, the report would be destroyed
// rather than deferred. That is the silent revert the section exists to close."
//
// The filter is applied on the list the whole time and is left exactly as it was:
// the two contenders occupy different physical rows, so both are read at once.
func TestCloseReport_OutranksFilterLine(t *testing.T) {
	s := sessionsFlashSurface()
	m, _ := newFailedCommitModel(t)
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	m = closeThemePanelForTest(t, m)

	m = applyThemeFlashFilter(t, m, s.query, true)
	if got := s.filterState(m); got != list.FilterApplied {
		t.Fatalf("the sessions filter state is %v, want %v", got, list.FilterApplied)
	}
	baseline := s.headerRow(m)
	if !strings.Contains(ansi.Strip(baseline), filterPromptPrefix+s.query) {
		t.Fatalf("the filter row does not carry %q, so a comparison against it says nothing: %q", filterPromptPrefix+s.query, ansi.Strip(baseline))
	}

	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("`t` did not open the panel over the applied filter")
	}
	m, _ = pressSlotKey(t, m, slotDarkPress)
	if !m.themeState.commitFailed {
		t.Fatal("fixture: the commit did not fail, so the close reports nothing")
	}

	m, _ = closePanelForTest(t, m)

	requireReportRaised(t, m)
	requireBandOwnsTheSlot(t, s, m, specThemeNotSavedFlash)
	requireFilterRowUnaffected(t, s, m, baseline, s.query)
	requireBandAboveTheFilterRow(t, s, m, specThemeNotSavedFlash, s.query)
	if got := s.filterState(m); got != list.FilterApplied {
		t.Errorf("the report moved the filter state to %v, want it left applied", got)
	}
}

// TestCloseReport_SingleClosePath: it uses the one close path.
//
// The geometry rule's forced close "takes the `Esc` path exactly", and this report attaches to
// that ONE close through its post-close step rather than to a second teardown beside
// it. Two implementations that agree today are two that can drift, and the drift
// lands on the one path where the user cannot reopen the panel to see what happened.
//
// The structural half pins the routing — one close, one report site — and the
// behavioural half is the consequence: with a failure outstanding the two closes
// leave IDENTICAL model state, flash included, because on that path they raise the
// same thing.
func TestCloseReport_SingleClosePath(t *testing.T) {
	t.Run("one close, one report site", func(t *testing.T) {
		if got, want := themePanelSeamCallers(t, "closeThemePanel"), []string{"resizeThemePanel", "updateThemePanel"}; !slices.Equal(got, want) {
			t.Errorf("closeThemePanel is called from %v, want exactly %v — the forced close and `Esc` route through the one close", got, want)
		}
		if got, want := themePanelSeamCallers(t, "reportOutstandingCommitFailure"), []string{"closeThemePanel"}; !slices.Equal(got, want) {
			t.Errorf("the close report is raised from %v, want exactly %v — it is a step of the single close, never a second raise beside it", got, want)
		}
	})

	t.Run("the two closes agree", func(t *testing.T) {
		contentW, contentH := geometryBelowHeightFloor()

		forced, _ := newCloseReportModel(t)
		forced = resizeForTest(t, forced, contentW, contentH)

		// The control is closed by `Esc` FIRST and resized after, so the two are
		// compared at the same final size with the same band history — the resize a
		// closed panel sees is a no-op, so nothing it does can stand in for the close.
		viaEsc, _ := newCloseReportModel(t)
		viaEsc, _ = closePanelForTest(t, viaEsc)
		viaEsc = resizeForTest(t, viaEsc, contentW, contentH)

		requireReportRaised(t, forced)
		requireReportRaised(t, viaEsc)
		if forced.themeState.active != viaEsc.themeState.active {
			t.Errorf("the forced close rendered %s and `Esc` rendered %s; the forced close takes the `Esc` path exactly", themeLabel(forced.themeState.active), themeLabel(viaEsc.themeState.active))
		}
		for name, m := range map[string]Model{"the forced close": forced, "Esc": viaEsc} {
			if got := m.themePanel; got.open || got.width != 0 || len(got.union.Rows) != 0 || len(got.enumeration.Entries) != 0 || got.badges != nil {
				t.Errorf("%s retained panel state %+v, want the zero value", name, got)
			}
		}
		if got, want := forced.View().Content, viaEsc.View().Content; got != want {
			t.Errorf("the forced close's frame is not `Esc`'s\nforced: %q\nesc:    %q", escSeq(got), escSeq(want))
		}
	})
}
