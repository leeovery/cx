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

// Verbatim, not the production constant: a test asserting a constant against
// itself pins nothing.
const wantThemeNotSavedFlash = "theme not saved — see portal.log"

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

func requireReportRaised(t *testing.T, m Model) {
	t.Helper()

	if got := m.flashText; got != wantThemeNotSavedFlash {
		t.Errorf("the close raised %q, want %q", got, wantThemeNotSavedFlash)
	}
	if m.flashOrigin != flashOriginTheme {
		t.Errorf("the report carries origin %v, want the theme origin — it claims the band over a filter line", m.flashOrigin)
	}
	if m.flashKind != flashWarning {
		t.Errorf("the report carries kind %v, want the ordinary warning flash", m.flashKind)
	}
	if m.themeState.commitFailed {
		t.Error("the report was raised with the failure still outstanding; raising it DISCHARGES the state")
	}
}

// Every tick is evaluated, so the walk costs flashAutoClearDuration per command.
func collectFlashTicks(cmd tea.Cmd) []flashTickMsg {
	if cmd == nil {
		return nil
	}
	switch msg := cmd().(type) {
	case flashTickMsg:
		return []flashTickMsg{msg}
	case tea.BatchMsg:
		var ticks []flashTickMsg
		for _, child := range msg {
			ticks = append(ticks, collectFlashTicks(child)...)
		}
		return ticks
	}
	return nil
}

func requireSingleFlashTick(t *testing.T, m Model, cmd tea.Cmd) flashTickMsg {
	t.Helper()

	ticks := collectFlashTicks(cmd)
	if len(ticks) != 1 {
		t.Fatalf("%d auto-clear tick(s) %+v reached Update's return (the close returned nothing at all: %t), want exactly one; the flash inherits the standard lifecycle", len(ticks), ticks, cmd == nil)
	}
	if ticks[0].Gen != m.flashGen {
		t.Fatalf("the tick carries generation %d, want the live %d — a stale generation is dropped by the guard and the flash never clears", ticks[0].Gen, m.flashGen)
	}
	return ticks[0]
}

func requireTickClears(t *testing.T, m Model, tick flashTickMsg) {
	t.Helper()

	cleared, _ := m.Update(tick)
	if got := cleared.(Model).flashText; got != "" {
		t.Errorf("the matching tick left the flash %q, want it cleared", got)
	}
}

func TestCloseReport_RaisesTheFlash(t *testing.T) {
	m, _ := newCloseReportModel(t)
	gen := m.flashGen

	m, cmd := closePanelForTest(t, m)

	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	requireReportRaised(t, m)
	requireFlashBandVisible(t, m, wantThemeNotSavedFlash)
	if got := themeNotSavedFlash; got != wantThemeNotSavedFlash {
		t.Errorf("the pinned constant is %q, want %q", got, wantThemeNotSavedFlash)
	}
	if got, want := m.flashGen, gen+1; got != want {
		t.Errorf("the report left the flash generation at %d, want %d — it rides the shared counter", got, want)
	}
	requireSingleFlashTick(t, m, cmd)

	superseded, _ := m.Update(flashTickMsg{Gen: m.flashGen - 1})
	if got := superseded.(Model).flashText; got != wantThemeNotSavedFlash {
		t.Errorf("a superseded tick left the report %q, want it standing", got)
	}
	cleared, _ := m.Update(flashTickMsg{Gen: m.flashGen})
	if got := cleared.(Model).flashText; got != "" {
		t.Errorf("the matching tick left the report %q, want it cleared", got)
	}
}

var closeReportFloorCrossings = []struct {
	name         string
	region       func() (contentW, contentH int)
	wantGeometry string
}{
	{name: "below the width floor", region: geometryBelowWidthFloor, wantGeometry: wantNarrowClosedFlash},
	{name: "below the height floor", region: geometryBelowHeightFloor, wantGeometry: wantShortClosedFlash},
}

func TestCloseReport_ForcedCloseCommitFlashWins(t *testing.T) {
	for _, tc := range closeReportFloorCrossings {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newCloseReportModel(t)
			contentW, contentH := tc.region()

			m, cmd := resizeForTestCmd(t, m, contentW, contentH)

			requireForcedClose(t, m, wantThemeNotSavedFlash)
			requireReportRaised(t, m)
			requireSingleFlashTick(t, m, cmd)
			if got := m.flashText; got == tc.wantGeometry {
				t.Errorf("the forced close raised the geometry copy %q, want the report — the band has ONE slot and the report is the one the user must act on", got)
			}
		})
	}
}

func TestCloseReport_ForcedCloseGeometryFlashSelfClears(t *testing.T) {
	for _, tc := range closeReportFloorCrossings {
		t.Run(tc.name, func(t *testing.T) {
			m, _ := newCommitFailureFixture(t)
			if m.themeState.commitFailed {
				t.Fatal("fixture: the model already carries an outstanding failure")
			}
			contentW, contentH := tc.region()

			m, cmd := resizeForTestCmd(t, m, contentW, contentH)

			requireForcedClose(t, m, tc.wantGeometry)
			if m.themeState.commitFailed {
				t.Error("the forced close left a failure outstanding where none was")
			}
			tick := requireSingleFlashTick(t, m, cmd)
			requireTickClears(t, m, tick)

			superseding := m
			(&superseding).setThemeFlash(wantThemeNotSavedFlash)
			stood, _ := superseding.Update(tick)
			if got := stood.(Model).flashText; got != wantThemeNotSavedFlash {
				t.Errorf("the forced close's in-flight tick left the later flash %q, want %q standing — the generation guard drops a superseded tick", got, wantThemeNotSavedFlash)
			}
		})
	}
}

func requireCloseIsSilent(t *testing.T, m Model, cmd tea.Cmd, gen uint64) {
	t.Helper()

	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	if got := m.flashText; got != "" {
		t.Errorf("the close raised %q, want nothing — only an OUTSTANDING failure is reported", got)
	}
	if m.flashGen != gen {
		t.Errorf("the close moved the flash generation to %d, want the untouched %d — it raised no flash", m.flashGen, gen)
	}
	if cmd != nil {
		t.Errorf("the close scheduled %T, want nothing", cmd)
	}
}

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

func TestCloseReport_SilentWhenNothingOutstanding(t *testing.T) {
	m, _ := newCommitFailureFixture(t)
	if m.themeState.commitFailed {
		t.Fatal("fixture: the model already carries an outstanding failure")
	}
	gen := m.flashGen

	m, cmd := closePanelForTest(t, m)

	requireCloseIsSilent(t, m, cmd, gen)
}

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

func TestCloseReport_CtrlCIsAnUndeliveredReport(t *testing.T) {
	m, _ := newCloseReportModel(t)
	gen := m.flashGen

	var cmd tea.Cmd
	stderr := captureStderrForTest(t, func() {
		updated, quit := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		m, cmd = updated.(Model), quit
	})

	if !isQuitCmd(cmd) {
		t.Fatalf("Ctrl-C returned %T, want tea.Quit — it is the one exit kept live inside the panel", cmd)
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

// The whole file descriptor is swapped rather than a writer seam injected: a
// seam would only prove that nothing reached the seam.
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
		t.Errorf("the close rendered %s, want the resolved persisted %s — the write did not land, so `Esc` resolving to persisted state is right", themeLabel(m.themeState.active), themeLabel(persisted))
	}
	if m.themeState.keys != keys {
		t.Errorf("the close left keys %+v, want the untouched %+v", m.themeState.keys, keys)
	}
	requireSlotCommits(t, persister, slotCommit{slug: commitFailureTarget, member: theme.MemberDark})
}

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
	requireSingleFlashTick(t, m, cmd)
	if m.activePage != PageProjects {
		t.Fatalf("the close moved the active page to %d, want it left on Projects", m.activePage)
	}
	role, message, ok := m.activeProjectNoticeBand()
	if !ok || role != bandWarning || message != wantThemeNotSavedFlash {
		t.Errorf("the Projects band is (role %v, message %q, ok %v), want (bandWarning, %q, true)", role, message, ok, wantThemeNotSavedFlash)
	}
	if got := ansi.Strip(m.viewProjectList()); !strings.Contains(got, wantThemeNotSavedFlash) {
		t.Errorf("the composed Projects frame carries no %q band:\n%s", wantThemeNotSavedFlash, got)
	}
}

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
	requireBandOwnsTheSlot(t, s, m, wantThemeNotSavedFlash)
	requireFilterRowUnaffected(t, s, m, baseline, s.query)
	requireBandAboveTheFilterRow(t, s, m, wantThemeNotSavedFlash, s.query)
	if got := s.filterState(m); got != list.FilterApplied {
		t.Errorf("the report moved the filter state to %v, want it left applied", got)
	}
}

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
