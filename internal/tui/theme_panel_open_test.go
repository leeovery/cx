package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
	"github.com/leeovery/portal/internal/tmux"
)

// The panel-open rule `t` open path: the keypress that binds the slide-over on Sessions and
// Projects, performs the ONE directory read (construction enumerates
// nothing), retains the parse results for the panel's lifetime and drops
// them on close.
//
// The two config sources are read on DELIBERATELY OPPOSITE cadences: the
// themes directory fresh on every open, because that is what the drop-in loop
// edits by hand between opens, and prefs from the construction-time snapshot,
// because re-reading it would import another instance's commit — the
// cross-instance sync the concurrent-write rule declines.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// fixtureThemesDir is the directory the recording seam answers with. It is a
// SHARED constant rather than a literal in the fake, so an assertion on the
// retained enumeration is pinned to what the seam actually handed back.
const fixtureThemesDir = "/fixture/themes"

// recordingThemeEnumerator is the fixture seam shape: it answers with a
// hand-built union, touching no filesystem, and RECORDS what it was asked. The
// call count is what pins the open cadence, and the recorded keys are what pin
// the construction-time load rule's construction-time prefs snapshot.
//
// Its Resolve answers with the declared resolution — which is what the panel
// derives its badges, its applied theme and its cursor from, the injected slot
// record having been retired with it. The zero value names no slot, which the
// open's degrade policy reads as "leave all three exactly as they were": the
// fixtures below that declare none are asserting the open CADENCE, not its
// resolution.
type recordingThemeEnumerator struct {
	union      theme.Union
	resolution theme.Resolution
	opens      int
	keys       []theme.RawKeys
}

func (e *recordingThemeEnumerator) Open(keys theme.RawKeys) (theme.Enumeration, theme.Union) {
	e.opens++
	e.keys = append(e.keys, keys)
	return theme.Enumeration{DirPath: fixtureThemesDir}, e.union
}

func (e *recordingThemeEnumerator) Reassemble(theme.Enumeration, theme.RawKeys) theme.Union {
	return e.union
}

func (e *recordingThemeEnumerator) Resolve(theme.Enumeration, theme.Setting) (theme.Resolution, error) {
	return e.resolution, nil
}

func (e *recordingThemeEnumerator) ResolveSlot(_ theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error) {
	return theme.SlotResolution{Slot: slot, Requested: slug, Resolved: slug}, nil
}

// countingThemeEnumerator is the PRODUCTION adapter itself — theme.DirEnumerator
// over a real theme.Loader and a real directory — carrying the open count the
// cadence assertions below read. Driving the real one is the only way to assert
// that a mid-session file edit is picked up and that each open is a genuine
// directory read rather than a replayed fake.
//
// The adapter is EMBEDDED rather than restated, so counting is the only
// behaviour added: three of the four methods are production's untouched, and the
// fourth delegates to production's after incrementing. A re-implementation here
// could drift from the seam production actually wires while both still compiled.
type countingThemeEnumerator struct {
	theme.DirEnumerator
	opens int
}

func (e *countingThemeEnumerator) Open(keys theme.RawKeys) (theme.Enumeration, theme.Union) {
	e.opens++
	return e.DirEnumerator.Open(keys)
}

// countingEnumeratorOver binds the production adapter to a loader and a staged
// directory, wrapped in the open counter.
func countingEnumeratorOver(loader theme.Loader, dir string) *countingThemeEnumerator {
	return &countingThemeEnumerator{DirEnumerator: theme.DirEnumerator{Loader: loader, Dir: dir}}
}

// themeOpenTestUnion is a two-row union: one built-in and one unresolvable
// persisted slug, which is the shape every badge and reason assertion below needs.
func themeOpenTestUnion() theme.Union {
	rows := []theme.Row{
		{Slug: "nord", Source: theme.SourcePersisted, Rejection: &theme.Rejection{Reason: theme.ReasonNotFound}},
		{Slug: theme.DefaultDarkSlug, Source: theme.SourceBuiltin},
	}
	return theme.Union{Rows: rows, Count: len(rows), Rejected: 1}
}

// themeOpenTestModel builds a Sessions-page model with the two constructor slots
// this task threads, through the SHARED Build chokepoint so the assertions are
// about the production wiring rather than about a struct literal.
//
// There is no slot record to pass: the panel's badges come from the seam's own
// Resolve against the enumeration it just read, which is what the injected
// record was retired in favour of.
func themeOpenTestModel(t *testing.T, enumerator ThemeEnumerator, keys theme.RawKeys) Model {
	t.Helper()
	return Build(Deps{
		Lister:          fakeLister{},
		ThemeEnumerator: enumerator,
		ThemeKeys:       keys,
	})
}

// themeOpenTestPopulatedModel builds a model with real rows on BOTH pages, which
// the filter and swallow assertions need: `bubbles/list` will not focus a filter
// input over an empty list, and a swallowed row action has to have a row to act
// on.
func themeOpenTestPopulatedModel(t *testing.T, enumerator ThemeEnumerator) Model {
	t.Helper()
	m := NewModelWithSessions([]tmux.Session{{Name: "alpha", Windows: 1}, {Name: "bravo", Windows: 2}})
	m.sessionKiller = keymapParityKiller{}
	m.projectList.SetItems(ProjectsToListItems([]project.Project{{Path: "/p/one", Name: "one"}}))
	m.projectList.Select(0)
	WithThemeEnumerator(enumerator)(&m)
	return m
}

// pressThemeKey drives the model's LIVE Update with `t`, so the assertion covers
// the page dispatch and the panel arm rather than a handler called directly.
func pressThemeKey(t *testing.T, m Model) Model {
	t.Helper()
	return pressPanelKey(t, m, tea.KeyPressMsg{Code: 't', Text: "t"})
}

func pressPanelKey(t *testing.T, m Model, msg tea.KeyPressMsg) Model {
	t.Helper()
	updated, _ := m.Update(msg)
	return updated.(Model)
}

// closeThemePanelForTest drives the `Esc` close (closeThemePanel) through Update.
func closeThemePanelForTest(t *testing.T, m Model) Model {
	t.Helper()
	return pressPanelKey(t, m, tea.KeyPressMsg{Code: tea.KeyEscape})
}

// themeOpenTestLoader returns a real loader emitting into a capture sink, so the
// the `theme: enumerated` cadence is observable.
func themeOpenTestLoader(t *testing.T) (theme.Loader, *logtest.Sink) {
	t.Helper()
	logger, sink := logtest.NewCaptureLogger(t)
	return theme.NewLoader(theme.NewEventLogger(logger)), sink
}

// countThemeEvents counts sink records carrying the given message. It is the
// count-only read of themeEventRecords rather than a second walk of the sink, so
// the two cannot come to disagree about what "carrying the message" means.
func countThemeEvents(sink *logtest.Sink, msg string) int {
	return len(themeEventRecords(sink, msg))
}

// writeThemeFileForTest writes a COMPLETE theme file whose every token carries
// value.
//
// One colour across the whole palette is deliberate: comparing a single token
// then identifies which file was parsed, and an unparseable value makes EVERY
// token bad, so a rejection's detail does not turn on which token the loader
// reaches first.
func writeThemeFileForTest(t *testing.T, dir, base, value string) {
	t.Helper()

	lines := themetest.Lines()
	for _, name := range theme.TokenNames() {
		lines = themetest.WithValue(lines, name, value)
	}
	themetest.Write(t, dir, base, lines)
}

// themePanelRowFor returns the panel item whose row is labelled label.
func themePanelRowFor(t *testing.T, m Model, label string) themeRowItem {
	t.Helper()
	for _, item := range m.themePanel.list.Items() {
		row, ok := item.(themeRowItem)
		if !ok {
			t.Fatalf("panel item %#v is not a themeRowItem", item)
		}
		if row.Row.Label() == label {
			return row
		}
	}
	t.Fatalf("panel has no row labelled %q (rows: %v)", label, themePanelRowLabels(m))
	return themeRowItem{}
}

// themePanelRowLabels lists the panel's row labels, for failure messages.
func themePanelRowLabels(m Model) []string {
	labels := make([]string, 0, len(m.themePanel.list.Items()))
	for _, item := range m.themePanel.list.Items() {
		if row, ok := item.(themeRowItem); ok {
			labels = append(labels, row.Row.Label())
		}
	}
	return labels
}

// TestThemePanelOpen_BoundOnBothPages: it opens the panel from Sessions and
// Projects.
//
// The panel-open rule binds `t` on BOTH pages because theme is a GLOBAL setting — refusing on
// Projects would make it feel page-scoped for no reason — so the two are asserted
// as one table rather than one page with the other left to a later phase.
func TestThemePanelOpen_BoundOnBothPages(t *testing.T) {
	for _, tc := range []struct {
		name string
		page page
	}{
		{name: "sessions", page: PageSessions},
		{name: "projects", page: PageProjects},
	} {
		t.Run(tc.name, func(t *testing.T) {
			union := themeOpenTestUnion()
			enumerator := &recordingThemeEnumerator{union: union}
			m := themeOpenTestModel(t, enumerator, theme.RawKeys{Theme: "nord"})
			m.activePage = tc.page

			m = pressThemeKey(t, m)

			if !m.themePanel.open {
				t.Fatalf("t on %s did not open the panel", tc.name)
			}
			if enumerator.opens != 1 {
				t.Errorf("the keypress ran %d enumerations, want exactly 1", enumerator.opens)
			}
			if got, want := len(m.themePanel.list.Items()), len(union.Rows); got != want {
				t.Errorf("panel list holds %d items, want %d — one per union row", got, want)
			}
			if m.themePanel.union.Count != union.Count {
				t.Errorf("panel union count = %d, want the enumerator's %d", m.themePanel.union.Count, union.Count)
			}
			if got := m.themePanel.enumeration.DirPath; got != fixtureThemesDir {
				t.Errorf("panel enumeration DirPath = %q, want the enumerator's %q — the open retains BOTH returned values", got, fixtureThemesDir)
			}
			if m.themePanel.width != themePanelPreferredWidth {
				t.Errorf("panel width = %d, want the preferred %d", m.themePanel.width, themePanelPreferredWidth)
			}
			if m.activePage != tc.page {
				t.Errorf("opening the panel changed the active page to %d", m.activePage)
			}
		})
	}
}

// TestThemePanelOpen_FilterCarveOut: it treats t as a filter character while
// filtering.
//
// The panel-open rule pins the carve-out explicitly — while `/` is focused `t` is a literal
// filter character, exactly as `s` already is — so the binding must sit BELOW each
// page's SettingFilter guard rather than above it.
func TestThemePanelOpen_FilterCarveOut(t *testing.T) {
	t.Run("sessions", func(t *testing.T) {
		enumerator := &recordingThemeEnumerator{union: themeOpenTestUnion()}
		m := themeOpenTestPopulatedModel(t, enumerator)

		m = pressPanelKey(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
		if !m.sessionList.SettingFilter() {
			t.Fatal("precondition: the sessions filter input is not focused after /")
		}
		m = pressThemeKey(t, m)

		if m.themePanel.open {
			t.Error("t opened the panel while the / filter was focused")
		}
		if enumerator.opens != 0 {
			t.Errorf("a literal filter character ran %d enumerations, want 0", enumerator.opens)
		}
		if got := m.sessionList.FilterValue(); got != "t" {
			t.Errorf("sessions filter query = %q, want %q — t must reach the input as text", got, "t")
		}
	})

	t.Run("projects", func(t *testing.T) {
		enumerator := &recordingThemeEnumerator{union: themeOpenTestUnion()}
		m := themeOpenTestPopulatedModel(t, enumerator)
		m.activePage = PageProjects

		m = pressPanelKey(t, m, tea.KeyPressMsg{Code: '/', Text: "/"})
		if !m.projectList.SettingFilter() {
			t.Fatal("precondition: the projects filter input is not focused after /")
		}
		m = pressThemeKey(t, m)

		if m.themePanel.open {
			t.Error("t opened the panel while the / filter was focused")
		}
		if enumerator.opens != 0 {
			t.Errorf("a literal filter character ran %d enumerations, want 0", enumerator.opens)
		}
		if got := m.projectList.FilterValue(); got != "t" {
			t.Errorf("projects filter query = %q, want %q — t must reach the input as text", got, "t")
		}
	})
}

// TestThemePanelOpen_NoEnumerationAtConstruction: it reads the directory on the
// keypress, not at construction.
//
// The lazy-discovery rule's whole point: auto-discovery must not turn one config read into an
// N-file scan-parse-validate sweep on a cold path that is explicitly latency-engineered.
// The directory here is POPULATED, so a construction-time sweep would be visible
// both as an enumerator call and as a `theme: enumerated` record.
func TestThemePanelOpen_NoEnumerationAtConstruction(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	loader, sink := themeOpenTestLoader(t)
	enumerator := countingEnumeratorOver(loader, dir)

	m := themeOpenTestModel(t, enumerator, theme.RawKeys{})

	if enumerator.opens != 0 {
		t.Errorf("construction ran %d enumerations, want 0 — discovery is lazy (§5.7)", enumerator.opens)
	}
	if got := countThemeEvents(sink, "enumerated"); got != 0 {
		t.Errorf("construction emitted %d `theme: enumerated` records, want 0", got)
	}

	m = pressThemeKey(t, m)

	if enumerator.opens != 1 {
		t.Errorf("the keypress ran %d enumerations, want exactly 1", enumerator.opens)
	}
	if got := countThemeEvents(sink, "enumerated"); got != 1 {
		t.Errorf("the keypress emitted %d `theme: enumerated` records, want exactly 1", got)
	}
	// The drop-in is proof the read was of the REAL directory rather than of a
	// cached or faked answer.
	themePanelRowFor(t, m, "sunset")
}

// TestThemePanelOpen_ReEnumeratesPerOpen: it re-enumerates on every open.
//
// The re-read-on-open rule: the directory is enumerated on EVERY panel open, not once per
// process — caching buys nothing measurable while breaking the loop the drop-in route
// exists for. The `theme` log component makes `theme: enumerated` a per-EVENT INFO with no
// dedup, so three opens are three records.
func TestThemePanelOpen_ReEnumeratesPerOpen(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	loader, sink := themeOpenTestLoader(t)
	enumerator := countingEnumeratorOver(loader, dir)
	m := themeOpenTestModel(t, enumerator, theme.RawKeys{})

	const opens = 3
	for i := range opens {
		m = pressThemeKey(t, m)
		if !m.themePanel.open {
			t.Fatalf("open %d did not open the panel", i+1)
		}
		m = closeThemePanelForTest(t, m)
	}

	if enumerator.opens != opens {
		t.Errorf("%d opens ran %d enumerations, want %d — the read is per open (§5.8)", opens, enumerator.opens, opens)
	}
	if got := countThemeEvents(sink, "enumerated"); got != opens {
		t.Errorf("%d opens emitted %d `theme: enumerated` records, want %d (§12.3 — per event, no dedup)", opens, got, opens)
	}
}

// TestThemePanelOpen_SeesAMidSessionEdit: it picks up a mid-session file edit on
// the next open.
//
// This is the loop the re-read-on-open rule exists to serve — copy a built-in, edit it, see
// it, WITHOUT relaunching Portal — driven in the direction that matters most: a
// previously-invalid file becomes valid and is selectable on the next open.
func TestThemePanelOpen_SeesAMidSessionEdit(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "not-a-colour")
	loader, _ := themeOpenTestLoader(t)
	m := themeOpenTestModel(t, countingEnumeratorOver(loader, dir), theme.RawKeys{})

	m = pressThemeKey(t, m)
	if broken := themePanelRowFor(t, m, "sunset"); broken.Row.Selectable() {
		t.Fatalf("precondition: the bad-colour file came back selectable")
	}
	m = closeThemePanelForTest(t, m)

	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	m = pressThemeKey(t, m)

	fixed := themePanelRowFor(t, m, "sunset")
	if !fixed.Row.Selectable() {
		t.Errorf("the repaired file is still rejected (%v) — the second open must re-read the directory", fixed.Row.Rejection)
	}
}

// TestThemePanelOpen_EnumerationDiscardedOnClose: it retains the enumeration for
// the panel's lifetime and discards it on close.
//
// The re-read-on-open rule retains the parse results for the panel's LIFETIME and drops them
// when it closes, so the next open re-reads. Asserted from three ends: the PARSE the open
// retained, that it is gone after close, and that a directory mutated between
// close and re-open is reflected.
//
// The retention half asserts the ENTRIES and not merely the path, because the
// entries are what the retention is FOR: the open-time re-resolution and the picker idiom's
// post-commit recompute both re-derive from them, so an enumeration dropped on
// the floor at open would silently re-derive a union of built-ins alone — every
// drop-in row vanishing the moment the user commits, with nothing erroring.
func TestThemePanelOpen_EnumerationDiscardedOnClose(t *testing.T) {
	dir := t.TempDir()
	// Seeded BEFORE the first open, so the retained enumeration has an entry to
	// carry. Its name is not `sunset`, or the mid-test write below would no longer
	// be a mutation and the re-read half would prove nothing.
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	loader, _ := themeOpenTestLoader(t)
	m := themeOpenTestModel(t, countingEnumeratorOver(loader, dir), theme.RawKeys{})

	m = pressThemeKey(t, m)
	if len(m.themePanel.union.Rows) == 0 {
		t.Fatal("precondition: the first open retained no rows")
	}
	// Reported by count and path rather than through %+v: an Entry carries a whole
	// Theme, which is 19 {name value} pairs of noise per row.
	if got := m.themePanel.enumeration; len(got.Entries) != 1 || got.DirPath != dir {
		t.Errorf("the open retained %d entries read from %q, want the 1 seeded entry read from %q", len(got.Entries), got.DirPath, dir)
	}
	m = closeThemePanelForTest(t, m)

	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	if got := m.themePanel.enumeration; len(got.Entries) != 0 || got.DirPath != "" {
		t.Errorf("close retained the enumeration %+v, want the zero value", got)
	}
	if got := m.themePanel.union; len(got.Rows) != 0 || got.Count != 0 {
		t.Errorf("close retained the union %+v, want the zero value", got)
	}
	if got := len(m.themePanel.list.Items()); got != 0 {
		t.Errorf("close retained %d list items, want 0", got)
	}

	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	m = pressThemeKey(t, m)
	themePanelRowFor(t, m, "sunset")
}

// TestThemePanelOpen_UsesConstructionTimePrefsSnapshot: it does not re-read prefs
// on open.
//
// The construction-time load rule's deliberate asymmetry with the re-read-on-open rule's fresh
// directory read: the themes directory is what the drop-in loop edits by hand, whereas
// prefs.json is what Portal itself writes — re-reading it would let another instance's commit
// silently change what this panel shows and marks, the cross-instance sync the
// concurrent-write rule explicitly declines.
//
// The staged prefs file is what makes the assertion mean anything: it puts a
// DIFFERENT answer on disk, at the path the config layer resolves to, for the
// whole window between construction and the second open. The keys handed to the
// seam must still be the ones injected at construction — so a re-read wired onto
// the open path by ANY route, env-resolved or newly seamed, changes what this
// records.
func TestThemePanelOpen_UsesConstructionTimePrefsSnapshot(t *testing.T) {
	prefsFile := filepath.Join(t.TempDir(), "prefs.json")
	if err := os.WriteFile(prefsFile, []byte(`{"theme":"nord"}`), 0o644); err != nil {
		t.Fatalf("write prefs: %v", err)
	}
	t.Setenv("PORTAL_PREFS_FILE", prefsFile)

	construction := theme.RawKeys{Light: theme.DefaultLightSlug, Dark: "nord"}
	enumerator := &recordingThemeEnumerator{union: themeOpenTestUnion()}
	m := themeOpenTestModel(t, enumerator, construction)

	m = pressThemeKey(t, m)
	m = closeThemePanelForTest(t, m)

	// Another instance commits a constant while this one is live.
	if err := os.WriteFile(prefsFile, []byte(`{"theme":"tokyo-night-day"}`), 0o644); err != nil {
		t.Fatalf("rewrite prefs: %v", err)
	}
	m = pressThemeKey(t, m)

	if len(enumerator.keys) != 2 {
		t.Fatalf("the seam was asked %d times, want 2", len(enumerator.keys))
	}
	for i, got := range enumerator.keys {
		if got != construction {
			t.Errorf("open %d handed the seam %+v, want the construction-time snapshot %+v", i+1, got, construction)
		}
	}
	if m.themeState.keys != construction {
		t.Errorf("model keys = %+v, want the construction-time snapshot %+v", m.themeState.keys, construction)
	}
}

// TestThemePanelOpen_BadgesFromTheSeamsResolution: it renders badges from the
// seam's own re-resolution.
//
// The construction-time load rule: a badge needs the PERSISTED slug, not the nomination's —
// under a fallback the two differ by design. The fixture is exactly that case: `nord` was
// nominated, did not load, and the dark default rendered instead. The `●` must
// stay on `nord`, and the fallback's own row must carry none.
//
// The record comes from the SEAM rather than from a constructor slot, and that is
// the whole of why the injected one is retired: the panel is closed until the open
// sequence runs, so `Resolve` is the only badge source there has ever been a badge
// to come from, and a `Deps` field alongside it would be a second and staler one.
func TestThemePanelOpen_BadgesFromTheSeamsResolution(t *testing.T) {
	enumerator := &recordingThemeEnumerator{
		union: themeOpenTestUnion(),
		resolution: theme.Resolution{
			Nomination: theme.ConstantNomination(testDarkTheme(t)),
			Slots: []theme.SlotResolution{{
				Slot:      theme.SlotConstant,
				Requested: "nord",
				Resolved:  theme.DefaultDarkSlug,
				FellBack:  true,
				Reason:    theme.ReasonNotFound,
				Theme:     testDarkTheme(t),
			}},
		},
	}
	m := themeOpenTestModel(t, enumerator, theme.RawKeys{Theme: "nord"})

	m = pressThemeKey(t, m)

	if got := themePanelRowFor(t, m, "nord").Badge; got != theme.BadgeConstant {
		t.Errorf("the persisted row's badge = %v, want %v — the ● marks what is SET", got, theme.BadgeConstant)
	}
	if got := themePanelRowFor(t, m, theme.DefaultDarkSlug).Badge; got != theme.BadgeNone {
		t.Errorf("the fallback row's badge = %v, want %v — a fallback must never move the ●", got, theme.BadgeNone)
	}
}

// TestThemePanelOpen_NilSeamIsASilentNoOp: it tolerates a nil enumerator.
//
// The mode-persister nil-guard precedent: a fixture or capturetool model that
// wires no seam makes `t` a silent no-op rather than a panic.
func TestThemePanelOpen_NilSeamIsASilentNoOp(t *testing.T) {
	m := themeOpenTestModel(t, nil, theme.RawKeys{Theme: "nord"})

	m = pressThemeKey(t, m)

	if m.themePanel.open {
		t.Error("t opened the panel with no enumerator wired")
	}
}

// TestThemePanelOpen_ThemesThePaginationDots: it themes the panel's own
// pagination dots at open.
//
// The completeness risk assigns the panel's chrome to the SAME restyle path as the main list,
// and the dots are the class it names: `bubbles/list` reads its dot strings out of
// its styles once, so a panel list left at the library's defaults renders the same
// hardcoded greys under every theme — identical before and after a swap, which
// the completeness guard structurally cannot see, since nothing changed.
//
// Both built-ins are driven, and the LIGHT case is the one that bites: the model's
// construction seed is the dark built-in, so dots taken from the seed rather than
// from the active theme would pass the dark case alone.
func TestThemePanelOpen_ThemesThePaginationDots(t *testing.T) {
	rows := themePanelTestRows(20)
	union := theme.Union{Rows: rows, Count: len(rows)}

	for _, tc := range []struct {
		name string
		th   theme.Theme
	}{
		{name: "dark", th: testDarkTheme(t)},
		{name: "light", th: testLightTheme(t)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := themeOpenTestModel(t, &recordingThemeEnumerator{union: union}, theme.RawKeys{})
			m.themeState.active = tc.th

			m = pressThemeKey(t, m)

			row := themePanelDotRow(t, renderThemePanel(m.themePanel, 16, tc.th, false))
			for _, want := range []struct {
				role  string
				token theme.Token
			}{
				{role: "active dot accent.primary", token: tc.th.AccentPrimary},
				{role: "inactive dot text.faint", token: tc.th.TextFaint},
			} {
				if seq := tokenFgSeq(t, want.token); !strings.Contains(row, seq) {
					t.Errorf("the panel dot row is missing the %s sequence %q:\n%q", want.role, seq, row)
				}
			}
			for _, grey := range bubblesDefaultDotGreys {
				if strings.Contains(row, grey) {
					t.Errorf("the panel dot row still carries the bubbles/list default grey %q — a colour belonging to no theme:\n%q", grey, row)
				}
			}
		})
	}

	t.Run("colourless", func(t *testing.T) {
		m := themeOpenTestModel(t, &recordingThemeEnumerator{union: union}, theme.RawKeys{})
		m.colourless = true

		// The NO_COLOR panel block's entry gate BLOCKS `t` under NO_COLOR, so the colourless panel
		// is armed directly (armPanelUnderNoColorForTest states why): this case asserts the
		// colourless RENDER of the dot row, not that a colourless panel can be opened.
		m = armPanelUnderNoColorForTest(t, m)

		row := themePanelDotRow(t, renderThemePanel(m.themePanel, 16, testDarkTheme(t), true))
		for col, cell := range scanCellBackgrounds(row) {
			if cell.set {
				t.Fatalf("colourless dot row col %d carries a background SGR (%s): %q", col, cell.params, escSeq(row))
			}
		}
		if strings.Contains(row, "38;2;") {
			t.Errorf("colourless dot row emits a foreground hue: %q", escSeq(row))
		}
	})
}

// bubblesDefaultDotGreys are `bubbles/list`'s own paginator dot colours (#979797
// active, #3C3C3C inactive) as truecolor SGR parameters — the values a panel list
// left at the library defaults renders under EVERY theme. They are stated as the
// thing that must be ABSENT, because their presence is invisible to the completeness guard's
// swap-and-diff guard: a value identical before and after a swap is exactly what
// that guard cannot report.
var bubblesDefaultDotGreys = []string{"38;2;151;151;151", "38;2;60;60;60"}

// themePanelDotRow locates the paginator's dot row inside a rendered panel block:
// the one line whose visible content is nothing but the left border cell, the dot
// glyphs and padding.
func themePanelDotRow(t *testing.T, block string) string {
	t.Helper()
	for line := range strings.SplitSeq(block, "\n") {
		visible := strings.TrimSpace(ansi.Strip(line))
		if !strings.Contains(visible, paginationDotGlyph) {
			continue
		}
		if strings.Trim(visible, paginationDotGlyph+panelFrameSide+" ") == "" {
			return line
		}
	}
	t.Fatalf("no pagination dot row in the rendered panel:\n%s", ansi.Strip(block))
	return ""
}

// TestThemePanelOpen_SwallowsPageKeys: it swallows every key but Ctrl-C while
// open.
//
// The entry-condition rule: the panel is key-exclusive. Pass-through is genuinely bad — `k`
// would kill the highlighted session while you pick a theme, `x` would swap to Projects with
// the panel open, `m` would start a multi-select behind it. None of that reasoning
// reaches the global quit, and swallowing it would take away the user's exit key
// inside a settings surface.
//
// Each swallowed key carries a CONTROL press with the panel closed, so a key that
// silently stopped working outside the panel could not pass this as a swallow.
func TestThemePanelOpen_SwallowsPageKeys(t *testing.T) {
	newModel := func(t *testing.T) Model {
		t.Helper()
		return themeOpenTestPopulatedModel(t, &recordingThemeEnumerator{union: themeOpenTestUnion()})
	}

	for _, tc := range []struct {
		name    string
		press   tea.KeyPressMsg
		effect  func(Model) bool
		effectS string
	}{
		{
			name:    "k does not kill",
			press:   tea.KeyPressMsg{Code: 'k', Text: "k"},
			effect:  func(m Model) bool { return m.modal == modalKillConfirm },
			effectS: "the kill confirm modal",
		},
		{
			name:    "x does not switch page",
			press:   tea.KeyPressMsg{Code: 'x', Text: "x"},
			effect:  func(m Model) bool { return m.activePage == PageProjects },
			effectS: "the Projects page",
		},
		{
			name:    "m does not enter multi-select",
			press:   tea.KeyPressMsg{Code: 'm', Text: "m"},
			effect:  func(m Model) bool { return m.MultiSelectActive() },
			effectS: "multi-select mode",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			control := pressPanelKey(t, newModel(t), tc.press)
			if !tc.effect(control) {
				t.Fatalf("precondition: %v does not reach %s with the panel CLOSED, so a swallow proves nothing", tc.press, tc.effectS)
			}

			m := pressThemeKey(t, newModel(t))
			if !m.themePanel.open {
				t.Fatal("precondition: the panel did not open")
			}
			m = pressPanelKey(t, m, tc.press)

			if tc.effect(m) {
				t.Errorf("%v reached %s while the panel was open — the panel is key-exclusive (§9.7)", tc.press, tc.effectS)
			}
			if !m.themePanel.open {
				t.Errorf("%v closed the panel; only Esc closes it", tc.press)
			}
		})
	}

	t.Run("Ctrl-C stays live", func(t *testing.T) {
		m := pressThemeKey(t, newModel(t))
		_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
		if !isQuitCmd(cmd) {
			t.Error("Ctrl-C did not quit while the panel was open — it must never be swallowed")
		}
	})

	t.Run("Esc closes", func(t *testing.T) {
		m := closeThemePanelForTest(t, pressThemeKey(t, newModel(t)))
		if m.themePanel.open {
			t.Error("Esc did not close the panel")
		}
	})
}
