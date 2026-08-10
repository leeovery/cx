package tui

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
)

// The picker idiom's `Esc`: the panel CLOSES, discarding an uncommitted preview and rendering
// the RESOLVED PERSISTED STATE — never a theme snapshotted at open.
//
// The snapshot is the naive implementation and it is wrong in two directions.
// Backwards: a user who broke their active theme's file mid-session would be
// handed back a palette the config no longer yields, "a stale copy Portal happens
// to still hold". Forwards: a commit writes prefs and leaves the
// panel open, so an `Esc` AFTER one must resolve to the NEWLY persisted state.
//
// Behaviourally the two are indistinguishable while nothing commits —
// so the distinguishing assertion is structural and lives in
// TestPanelClose_DiscardsThePreview: the close CALLS the seam's Resolve, against
// the RETAINED enumeration, and issues no read of its own.
//
// The rest of this file is negatives, because the close's risk profile is what it
// must NOT do: no write of any kind, no directory read, no
// re-layout of the page beneath, and no escape of the key-exclusivity the
// panel holds while it is open.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// newClosePanelModel is the dir-backed panel model over a SINK-BACKED panel
// loader, handing back the panel's own enumerator plus that sink.
//
// The sink holds the PANEL's emissions alone — the `theme` log component's cadence on this
// path is a statement about what an open and an `Esc` emit, and a sink construction also
// resolved through would make every count a delta against construction's own lines
// instead of an absolute.
func newClosePanelModel(t *testing.T, dir string, keys theme.RawKeys) (Model, *countingThemeSource, *logtest.Sink) {
	t.Helper()

	loader, sink := themeOpenTestLoader(t)
	m, enumerator := newDirBackedPanelModelOver(t, dir, keys, appearanceDarkCanvas, loader)
	return m, enumerator, sink
}

// closePanelForTest drives `Esc` through the live Update and returns the command
// alongside the model.
//
// The COMMAND is how "closing is one frame" is observed: a transition would have
// to be scheduled as a tea.Cmd, so a nil one says there is no second frame and no
// intermediate width to render.
func closePanelForTest(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	return updated.(Model), cmd
}

// closePanelSessions are the rows every renderable close fixture lists behind the
// panel, matching the arrow fixtures' so the two suites' frames are comparable.
func closePanelSessions() []tmux.Session {
	return []tmux.Session{
		{Name: "alpha", Windows: 3, Attached: true},
		{Name: "bravo", Windows: 1},
		{Name: "charlie", Windows: 2},
	}
}

// newClosePanelStubModel builds a RENDERABLE Sessions model over hand-declared
// union rows, with the panel still CLOSED — the pre-open state a "restores the
// frame" comparison needs a frame from.
func newClosePanelStubModel(t *testing.T, rows []theme.Row) (Model, *fakeThemeSource) {
	t.Helper()
	deps := newArrowPanelDeps(t, rows, rows[0].Slug)
	stub, ok := deps.ThemeSource.(*fakeThemeSource)
	if !ok {
		t.Fatalf("the arrow deps' seam is %T, want the recording fake", deps.ThemeSource)
	}
	m := Build(deps)
	m.termWidth, m.termHeight = arrowTermW, arrowTermH
	m.applySessions(closePanelSessions())
	return m, stub
}

// newClosePanelLayoutModel builds a model with BOTH pages populated and both
// lists sized by production, so the page beneath the panel has a layout to be
// unchanged.
func newClosePanelLayoutModel(t *testing.T) Model {
	t.Helper()
	m, _ := newClosePanelStubModel(t, arrowValidRows(4))
	projects := []project.Project{{Path: "/p/one", Name: "one"}, {Path: "/p/two", Name: "two"}}
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	return m
}

// TestPanelClose_DiscardsThePreview: it restores the pre-open frame when nothing
// changed.
//
// The picker idiom's `Esc` equals "what you had before" ONLY when nothing was committed, so
// with nothing changed the composed frame must come back byte for byte after
// three rows of preview.
//
// THE RESOLUTION COUNT IS THE LOAD-BEARING HALF. With nothing committed, a
// theme snapshotted at open would produce this same frame — the mechanism is what
// distinguishes them, and it must be re-resolution from the start because a
// commit changes the persisted state this close resolves against. The same
// counter pins the arrows as pure preview: they resolve nothing, previewing from
// the parse already in hand.
func TestPanelClose_DiscardsThePreview(t *testing.T) {
	rows := arrowValidRows(6)
	m, stub := newClosePanelStubModel(t, rows)
	before := m.View().Content

	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("fixture: the panel did not open")
	}
	if len(stub.settings) != 1 {
		t.Fatalf("the open ran %d resolutions, want exactly 1", len(stub.settings))
	}
	for range 3 {
		m = pressPanelKey(t, m, arrowDown)
	}
	if m.themeState.active == rows[0].Theme {
		t.Fatal("fixture: three arrows previewed nothing, so the close below restores nothing")
	}
	if len(stub.settings) != 1 {
		t.Errorf("three arrows ran %d resolutions in total, want the open's 1 — an arrow previews from the retained parse (§5.8)", len(stub.settings))
	}

	m, cmd := closePanelForTest(t, m)

	if cmd != nil {
		t.Errorf("Esc scheduled %T; closing is ONE frame — no animation, no transition, no intermediate width (§9.1)", cmd)
	}
	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	if len(stub.settings) != 2 {
		t.Errorf("the close ran %d resolutions in total, want 2 — `Esc` RE-RESOLVES persisted state rather than restoring a snapshot (§9.2)", len(stub.settings))
	}
	if m.themeState.active != rows[0].Theme {
		t.Errorf("the close left canvas %s, want the resolved persisted %s", m.themeState.active.Canvas.Value, rows[0].Theme.Canvas.Value)
	}
	if got := m.View().Content; got != before {
		t.Errorf("the post-close frame is not the pre-open frame\nbefore: %q\nafter:  %q", escSeq(before), escSeq(got))
	}
}

// TestPanelClose_ResolvesEditedValues: it renders an edited-but-valid theme's new
// values.
//
// The mid-session edit lands at OPEN and must SURVIVE the close:
// `Esc` resolves against the panel's enumeration, so it lands on the edited values
// rather than on the palette Portal was holding when the panel opened.
func TestPanelClose_ResolvesEditedValues(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	m, _, _ := newClosePanelModel(t, dir, theme.RawKeys{Theme: "sunset"})
	if got := m.themeState.active.Canvas.Value; got != "#101010" {
		t.Fatalf("precondition: the launch rendered canvas %s, want the drop-in's #101010", got)
	}

	writeThemeFileForTest(t, dir, "sunset.theme", "#202020")
	m = pressThemeKey(t, m)
	if got := m.themeState.active.Canvas.Value; got != "#202020" {
		t.Fatalf("precondition: the open rendered canvas %s, want the edited #202020 (§9.2)", got)
	}
	m = pressPanelKey(t, m, arrowDown)
	if m.themeState.active.Canvas.Value == "#202020" {
		t.Fatal("fixture: the arrow previewed nothing, so the close below restores nothing")
	}

	m = closeThemePanelForTest(t, m)

	if got := m.themeState.active.Canvas.Value; got != "#202020" {
		t.Errorf("the close rendered canvas %s, want the enumeration's edited #202020 — #101010 would be the palette held when the panel opened, which `Esc` must not restore", got)
	}
}

// TestPanelClose_ResolvesToFallback: it lands on the fallback when the active
// theme was invalidated.
//
// The flip already happened at open, so this asserts the CLOSE
// RESOLVES rather than restores: a snapshot taken before that flip would hand the
// user back a palette the config no longer yields, with no surface left open to
// explain it.
func TestPanelClose_ResolvesToFallback(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	m, _, _ := newClosePanelModel(t, dir, theme.RawKeys{Theme: "sunset"})
	if got := m.themeState.active.Canvas.Value; got != "#101010" {
		t.Fatalf("precondition: the launch rendered canvas %s, want the drop-in's #101010", got)
	}

	writeThemeFileForTest(t, dir, "sunset.theme", "not-a-colour")
	m = pressThemeKey(t, m)
	fallback := testDarkTheme(t)
	if m.themeState.active != fallback {
		t.Fatalf("precondition: the open rendered canvas %s, want the §8.5 fallback's %s", m.themeState.active.Canvas.Value, fallback.Canvas.Value)
	}
	m = pressPanelKey(t, m, arrowUp)
	if m.themeState.active == fallback {
		t.Fatal("fixture: the arrow previewed nothing, so the close below restores nothing")
	}

	m = closeThemePanelForTest(t, m)

	if m.themeState.active != fallback {
		t.Errorf("the close rendered canvas %s, want the §8.5 fallback's %s — the persisted `sunset` no longer resolves, and Portal shows what the config NOW says", m.themeState.active.Canvas.Value, fallback.Canvas.Value)
	}
}

// TestPanelClose_ReadsNothing: it resolves against the retained enumeration.
//
// The construction-time load rule refuses an open-time and a commit-time directory read alike,
// because a read here would produce a THIRD parse of the same slug — neither construction's
// nor the panel's — that can disagree with the rows the user was just looking at.
// Driven the only way it cannot be faked: the themes directory is DELETED after
// the panel opened, and the close still resolves.
func TestPanelClose_ReadsNothing(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	m, enumerator, _ := newClosePanelModel(t, dir, theme.RawKeys{Theme: "aurora"})

	m = pressThemeKey(t, m)
	if got := m.themeState.active.Canvas.Value; got != "#101010" {
		t.Fatalf("precondition: the open rendered canvas %s, want the drop-in's #101010", got)
	}
	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove the themes directory: %v", err)
	}
	m = pressPanelKey(t, m, arrowDown)
	if m.themeState.active.Canvas.Value == "#101010" {
		t.Fatal("fixture: the arrow previewed nothing, so the close below restores nothing")
	}

	m = closeThemePanelForTest(t, m)

	if got := m.themeState.active.Canvas.Value; got != "#101010" {
		t.Errorf("with the themes directory gone the close rendered canvas %s, want the RETAINED parse's #101010 — the close resolves against the panel's enumeration, never the filesystem (§8.4)", got)
	}
	if enumerator.opens != 1 {
		t.Errorf("the close ran %d enumerations in total, want the single one the open performed", enumerator.opens)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("the themes directory exists again after the close (err %v); nothing on this path touches it", err)
	}
}

// TestPanelClose_EnumerationDiscarded: it discards the enumeration so the next
// open re-reads.
//
// The re-read-on-open rule retains the parse for the panel's LIFETIME and drops it on close —
// which is what makes fixing a previously-invalid theme take effect without relaunching
// Portal. Every retained field is asserted rather than a subset: rows from one
// read beside badges from another is precisely what a partial clear produces.
func TestPanelClose_EnumerationDiscarded(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	m, enumerator, _ := newClosePanelModel(t, dir, theme.RawKeys{Theme: "aurora"})

	m = pressThemeKey(t, m)
	if got := m.themePanel.enumeration; len(got.Entries) != 1 || got.DirPath != dir {
		t.Fatalf("precondition: the open retained %d entries read from %q, want the 1 seeded entry read from %q", len(got.Entries), got.DirPath, dir)
	}
	if len(m.themePanel.badges) == 0 {
		t.Fatal("precondition: the open populated no badges, so clearing them proves nothing")
	}
	if len(m.themePanel.list.Items()) == 0 {
		t.Fatal("precondition: the open listed no rows")
	}

	m = closeThemePanelForTest(t, m)

	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	if got := m.themePanel.enumeration; len(got.Entries) != 0 || got.DirPath != "" {
		t.Errorf("close retained the enumeration %d entries from %q, want the zero value", len(got.Entries), got.DirPath)
	}
	if got := m.themePanel.union; len(got.Rows) != 0 || got.Count != 0 || got.Rejected != 0 || got.DirUnusable {
		t.Errorf("close retained the union %+v, want the zero value", got)
	}
	if got := m.themePanel.badges; got != nil {
		t.Errorf("close retained the badge table %v, want none", got)
	}
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("close retained the message %+v, want it cleared", got)
	}
	if got := m.themePanel.width; got != 0 {
		t.Errorf("close retained width %d, want 0", got)
	}
	// The LIST is replaced by the zero value, delegate included. `bubbles/list`
	// exports no delegate accessor, so its KEYMAP stands in: newThemePanelList pins
	// `up` on a live list and the zero value binds nothing at all.
	if got := len(m.themePanel.list.Items()); got != 0 {
		t.Errorf("close retained %d list items, want 0", got)
	}
	if w, h := m.themePanel.list.Width(), m.themePanel.list.Height(); w != 0 || h != 0 {
		t.Errorf("close retained a list sized %dx%d, want the zero value", w, h)
	}
	if got := m.themePanel.list.KeyMap.CursorUp.Keys(); len(got) != 0 {
		t.Errorf("close retained the list's keymap (CursorUp binds %v), so the list — and the delegate it carries — was not replaced by the zero value", got)
	}

	writeThemeFileForTest(t, dir, "sunset.theme", "#202020")
	m = pressThemeKey(t, m)

	themePanelRowFor(t, m, "sunset")
	if enumerator.opens != 2 {
		t.Errorf("two opens ran %d enumerations, want 2 — the discard is what makes the next open re-read (§5.8)", enumerator.opens)
	}
}

// TestPanelClose_WritesNothing: it writes nothing on close.
//
// The picker idiom: "every write is an explicit keypress; nothing writes on close" — which is
// what eliminates the "applied but not persisted" state persist-on-close would
// reach, where Portal dies with the visually-applied theme never written.
//
// The two persister seams are the routes a write could actually take: internal/tui
// resolves no config path and reads no PORTAL_* env var, so every path from this
// package to prefs.json runs through one of them, and the package's tmux writers
// (the burst's ack channel, the session action seams) are neither wired to the
// panel nor reachable from it. The COMMAND is the third observation, and the one
// a counter cannot make: a write deferred off the keypress would have to be
// scheduled as a tea.Cmd. The files are watched as well, because the enumeration
// seam really does reach the themes directory.
func TestPanelClose_WritesNothing(t *testing.T) {
	t.Run("no persister is reached", func(t *testing.T) {
		mode := &countingModePersister{}
		committer := &fakeThemePersister{}
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
		m, _, _ := newClosePanelModel(t, dir, theme.RawKeys{Theme: "sunset"})
		WithModePersister(mode)(&m)
		WithThemePersister(committer)(&m)

		m = pressThemeKey(t, m)
		m = pressPanelKey(t, m, arrowDown)
		m, cmd := closePanelForTest(t, m)

		if mode.calls != 0 {
			t.Errorf("closing persisted %d preference(s); every write is an explicit keypress (§9.2)", mode.calls)
		}
		if len(committer.slugs) != 0 {
			t.Errorf("closing committed %v; nothing writes on close (§9.2)", committer.slugs)
		}
		if cmd != nil {
			t.Errorf("closing scheduled %T; a deferred write is the one shape the counters above cannot see", cmd)
		}

		// Positive control: the mode seam is wired to THIS model and live, so the
		// zero above is an absence of writes rather than an absence of wiring.
		m = pressPanelKey(t, m, tea.KeyPressMsg{Code: 's', Text: "s"})
		if mode.calls != 1 {
			t.Fatalf("positive control: `s` on the closed picker persisted %d time(s), want 1 — the counting persister proves nothing about the close", mode.calls)
		}
	})

	t.Run("a present prefs.json survives byte for byte", func(t *testing.T) {
		const persisted = `{"session_list_mode":"by-project","theme":"sunset"}`
		prefsFile := filepath.Join(t.TempDir(), "prefs.json")
		if err := os.WriteFile(prefsFile, []byte(persisted), 0o644); err != nil {
			t.Fatalf("write prefs: %v", err)
		}
		t.Setenv("PORTAL_PREFS_FILE", prefsFile)
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "sunset.theme", "not-a-colour")
		m, _, _ := newClosePanelModel(t, dir, theme.RawKeys{Theme: "sunset"})

		closeThemePanelForTest(t, pressThemeKey(t, m))

		after, err := os.ReadFile(prefsFile)
		if err != nil {
			t.Fatalf("read back prefs: %v", err)
		}
		if string(after) != persisted {
			t.Errorf("prefs.json =\n%s\nwant it byte-identical:\n%s", after, persisted)
		}
	})

	t.Run("an absent prefs.json stays absent and the themes directory is untouched", func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("PORTAL_PREFS_FILE", filepath.Join(configDir, "prefs.json"))
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
		m, _, _ := newClosePanelModel(t, dir, theme.RawKeys{Theme: "sunset"})

		closeThemePanelForTest(t, pressPanelKey(t, pressThemeKey(t, m), arrowDown))

		if entries, err := os.ReadDir(configDir); err != nil || len(entries) != 0 {
			t.Errorf("closing left %d entries in the config directory (err %v), want none", len(entries), err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		if len(entries) != 1 || entries[0].Name() != "sunset.theme" {
			t.Errorf("the themes directory holds %d entries after a close, want only the seeded drop-in", len(entries))
		}
	})
}

// TestPanelClose_EventCadence: it emits one fallback record across many closes and
// no loaded record.
//
// The `theme` log component names the panel open "and again on every `Esc`" as the reason
// `theme: fallback applied` is deduplicated per process on slug+reason — so a
// persistently broken active theme produces ONE WARN, not one per close.
//
// `theme: loaded` takes the opposite call and it is NOT deduplicated: its
// catalogued cadence is construction plus the one commit-time load outside it, so
// emitting per open/close would turn a per-load INFO into exactly the running
// commentary its neighbours' dedup rules exist to prevent.
func TestPanelClose_EventCadence(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "not-a-colour")
	m, enumerator, sink := newClosePanelModel(t, dir, theme.RawKeys{Theme: "sunset"})

	const cycles = 10
	for i := range cycles {
		m = pressThemeKey(t, m)
		if !m.themePanel.open {
			t.Fatalf("cycle %d did not open the panel", i+1)
		}
		m = closeThemePanelForTest(t, m)
		if m.themePanel.open {
			t.Fatalf("cycle %d did not close the panel", i+1)
		}
	}

	if got := countThemeEvents(sink, "fallback applied"); got != 1 {
		t.Errorf("%d open/close cycles emitted %d `theme: fallback applied` records, want exactly 1 — the WARN dedups per process on slug+reason (§12.3)", cycles, got)
	}
	if got := countThemeEvents(sink, "loaded"); got != 0 {
		t.Errorf("%d open/close cycles emitted %d `theme: loaded` records, want 0 — its cadence is construction plus the one commit-time load (§12.3)", cycles, got)
	}

	// Positive controls: the sink IS wired to the panel's loader (the per-event
	// INFO fires once per open), and `theme: loaded` is reachable through that same
	// loader — so the zero above is an absence of emission rather than an absence
	// of recording.
	if got := countThemeEvents(sink, "enumerated"); got != cycles {
		t.Fatalf("%d opens emitted %d `theme: enumerated` records, want %d — the sink is not recording this loader", cycles, got, cycles)
	}
	setting, _ := theme.ResolveSetting(theme.RawKeys{Theme: "sunset"})
	if _, err := enumerator.Loader.ResolveNomination(setting, dir); err != nil {
		t.Fatalf("positive-control by-name resolution: %v", err)
	}
	if countThemeEvents(sink, "loaded") == 0 {
		t.Fatal("positive control: a by-name resolution through the SAME loader emitted no `theme: loaded`, so the zero above says nothing about the panel path")
	}
}

// TestPanelClose_DoesNotClearTheFilter: it leaves an applied filter alone.
//
// The entry-condition rule's key-exclusivity has to hold THROUGH the close: `Esc` is consumed
// by the panel and never reaches the page beneath, where it is the progressive-back key
// that clears an applied filter. The control closes the same model a second time
// with the panel already gone, where `Esc` DOES clear — so the assertion is about
// the panel consuming the key rather than about a filter that was never clearable.
func TestPanelClose_DoesNotClearTheFilter(t *testing.T) {
	m := themeOpenTestPopulatedModel(t, newOpenEnumerator(themeOpenTestUnion()))

	m = pressPanelKey(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
	m = pressPanelKey(t, m, tea.KeyPressMsg{Code: 'a', Text: "a"})
	m = pressPanelKey(t, m, tea.KeyPressMsg{Code: tea.KeyEnter})
	if got := m.sessionList.FilterState(); got != list.FilterApplied {
		t.Fatalf("precondition: the sessions filter state is %v, want FilterApplied", got)
	}

	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("precondition: the panel did not open over the applied filter")
	}
	m = closeThemePanelForTest(t, m)

	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	if got := m.sessionList.FilterState(); got != list.FilterApplied {
		t.Errorf("the sessions filter state is %v after the close, want it left FilterApplied — Esc is consumed by the panel (§9.7)", got)
	}
	if got := m.sessionList.FilterValue(); got != "a" {
		t.Errorf("the sessions filter query = %q after the close, want the applied %q", got, "a")
	}

	// Positive control: with the panel closed the SAME key clears the filter.
	m = closeThemePanelForTest(t, m)
	if got := m.sessionList.FilterState(); got == list.FilterApplied {
		t.Fatal("positive control: Esc on the closed picker left the filter applied, so the assertion above says nothing about the panel")
	}
}

// TestPanelClose_NestsOverMultiSelect: it returns to multi-select with the set
// intact.
//
// The entry-condition rule: the panel NESTS over the mode and `Esc` resolves innermost-first,
// exactly as MV already specifies for modals — closing the panel returns to
// multi-select with the selections intact rather than exiting the mode.
func TestPanelClose_NestsOverMultiSelect(t *testing.T) {
	m := newClosePanelLayoutModel(t)

	m = pressPanelKey(t, m, tea.KeyPressMsg{Code: 'm', Text: "m"})
	if !m.MultiSelectActive() {
		t.Fatal("precondition: `m` did not enter multi-select")
	}
	marked := m.SelectedSessionCount()
	if marked == 0 {
		t.Fatal("precondition: entering multi-select marked nothing, so an intact set proves nothing")
	}

	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("precondition: `t` did not open the panel over multi-select")
	}
	m = closeThemePanelForTest(t, m)

	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	if !m.MultiSelectActive() {
		t.Error("the close exited multi-select; Esc resolves innermost-first (§9.7)")
	}
	if got := m.SelectedSessionCount(); got != marked {
		t.Errorf("the marked set holds %d session(s) after the close, want the %d marked before", got, marked)
	}
	if !m.IsSessionSelected("alpha") {
		t.Error("the auto-marked `alpha` is no longer selected after the close")
	}
	if got := ansi.Strip(m.View().Content); !strings.Contains(got, "1 selected") {
		t.Errorf("the post-close frame carries no `1 selected` banner; it sits in the notice band and must survive the close")
	}

	// Positive control: with the panel closed the SAME key exits the mode.
	m = closeThemePanelForTest(t, m)
	if m.MultiSelectActive() {
		t.Fatal("positive control: Esc on the closed picker stayed in multi-select, so the assertion above says nothing about the panel")
	}
}

// TestPanelClose_EscDoesNotQuit: it never quits.
//
// The panel is the innermost surface, so its `Esc` closes it and nothing else —
// on BOTH pages, where an un-panelled `Esc` with no filter and no marked set is
// the progressive-back QUIT. The control is that same quit, so the assertion is
// about the panel consuming the key.
func TestPanelClose_EscDoesNotQuit(t *testing.T) {
	for _, tc := range []struct {
		name string
		page page
	}{
		{name: "sessions", page: PageSessions},
		{name: "projects", page: PageProjects},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newClosePanelLayoutModel(t)
			m.activePage = tc.page

			_, control := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
			if !isQuitCmd(control) {
				t.Fatalf("precondition: Esc on the closed %s page does not quit, so the assertion below proves nothing", tc.name)
			}

			m = pressThemeKey(t, m)
			if !m.themePanel.open {
				t.Fatalf("precondition: `t` did not open the panel on %s", tc.name)
			}
			m, cmd := closePanelForTest(t, m)

			if isQuitCmd(cmd) {
				t.Errorf("Esc quit Portal from the open panel on %s; it closes the panel and nothing else", tc.name)
			}
			if m.themePanel.open {
				t.Errorf("Esc did not close the panel on %s", tc.name)
			}
			if m.activePage != tc.page {
				t.Errorf("the close moved the active page to %d, want it left on %d", m.activePage, tc.page)
			}
		})
	}
}

// TestPanelClose_PageLayoutUnchangedAcrossOpenAndClose: it re-lays-out nothing on
// the page beneath.
//
// The panel layout composites the panel over a base composed at the UNREDUCED content width,
// so there is no frame to reclaim on close. The three measurements are taken at
// one fixed terminal size and must agree: a close that "completed" itself with a
// reclaim step would be one step from the open-time reduction that justifies it,
// which reflows the surface being previewed.
func TestPanelClose_PageLayoutUnchangedAcrossOpenAndClose(t *testing.T) {
	for _, tc := range []struct {
		name string
		page page
		size func(Model) (int, int)
	}{
		{name: "sessions", page: PageSessions, size: Model.SessionListSize},
		{name: "projects", page: PageProjects, size: Model.ProjectListSize},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := newClosePanelLayoutModel(t)
			m.activePage = tc.page

			beforeW, beforeH := tc.size(m)
			if beforeW <= 0 || beforeH <= 0 {
				t.Fatalf("fixture: the %s list is sized %dx%d, so an unchanged size says nothing", tc.name, beforeW, beforeH)
			}

			m = pressThemeKey(t, m)
			if !m.themePanel.open {
				t.Fatalf("precondition: `t` did not open the panel on %s", tc.name)
			}
			duringW, duringH := tc.size(m)
			if duringW != beforeW || duringH != beforeH {
				t.Errorf("the open re-laid-out the %s list to %dx%d, want the unreduced %dx%d", tc.name, duringW, duringH, beforeW, beforeH)
			}

			m = closeThemePanelForTest(t, m)
			afterW, afterH := tc.size(m)
			if afterW != beforeW || afterH != beforeH {
				t.Errorf("the close re-laid-out the %s list to %dx%d, want the unreduced %dx%d — the page was never reduced, so there is nothing to reclaim", tc.name, afterW, afterH, beforeW, beforeH)
			}
		})
	}
}

// TestPanelClose_ForcedCloseUsesTheSameFunction: it is the single close path.
//
// The geometry rule's forced close "takes the `Esc` path EXACTLY", and the failed-commit
// flash and outstanding-failure discharge attach to the same function. Two
// implementations that can drift is precisely what that language forbids, so the
// guard is structural: the panel is discarded in exactly ONE place, which leaves
// a later caller nowhere else to close from.
//
// The behavioural half is the other side of the same statement — a DIRECT call
// (what the forced close and the commit path make) lands in the same state as the
// keypress, so the post-close step a caller adds is additive rather than a fork.
func TestPanelClose_ForcedCloseUsesTheSameFunction(t *testing.T) {
	t.Run("the panel is discarded in exactly one place", func(t *testing.T) {
		sites := panelDiscardSites(t)
		if len(sites) != 1 || sites[0] != "closeThemePanel" {
			t.Errorf("the panel struct is zeroed in %v, want exactly [closeThemePanel] — §9.8's forced close and Phase 9's hooks must have one close to route through, not a second to drift from", sites)
		}
	})

	t.Run("a direct call is the whole close", func(t *testing.T) {
		rows := arrowValidRows(4)
		open := func() Model {
			m, _ := newClosePanelStubModel(t, rows)
			m = pressThemeKey(t, m)
			if !m.themePanel.open {
				t.Fatal("fixture: the panel did not open")
			}
			m = pressPanelKey(t, m, arrowDown)
			if m.themeState.active == rows[0].Theme {
				t.Fatal("fixture: the arrow previewed nothing, so the two closes have nothing to undo")
			}
			return m
		}

		viaEsc := closeThemePanelForTest(t, open())
		direct := open()
		(&direct).closeThemePanel()

		if direct.themePanel.open {
			t.Fatal("the direct call left the panel open")
		}
		if direct.themeState.active != viaEsc.themeState.active {
			t.Errorf("the direct call rendered canvas %s and Esc rendered %s; §9.8's forced close takes the Esc path EXACTLY", direct.themeState.active.Canvas.Value, viaEsc.themeState.active.Canvas.Value)
		}
		if got, want := direct.View().Content, viaEsc.View().Content; got != want {
			t.Errorf("the direct call's frame is not Esc's\ndirect: %q\nesc:    %q", escSeq(got), escSeq(want))
		}
	})
}

// panelDiscardSites returns the name of every production function that ZEROES the
// panel struct — an assignment to a `.themePanel` field whose right-hand side is
// the empty `themePanel{}` composite literal.
//
// It matches the zero literal specifically, so armThemePanel's populated install
// is not a discard and a field-level write (the resize sets width) is not
// one either.
func panelDiscardSites(t *testing.T) []string {
	t.Helper()
	var sites []string
	for _, file := range parsePackageFilesByName(t) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				assign, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for i, lhs := range assign.Lhs {
					sel, ok := lhs.(*ast.SelectorExpr)
					if !ok || sel.Sel.Name != "themePanel" || i >= len(assign.Rhs) {
						continue
					}
					lit, ok := assign.Rhs[i].(*ast.CompositeLit)
					if !ok || len(lit.Elts) != 0 {
						continue
					}
					if ident, ok := lit.Type.(*ast.Ident); ok && ident.Name == "themePanel" {
						sites = append(sites, fn.Name.Name)
					}
				}
				return true
			})
		}
	}
	return sites
}
