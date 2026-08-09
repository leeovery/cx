package tui

import (
	"reflect"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tmux"
)

// This file guards the §11.1 live theme-swap entry point, Model.ApplyTheme — the
// one Phase 8's panel drives from arrow-preview, panel open and panel close.
//
// Every assertion here is about what a swap must NOT do. §11.1 splits the model's
// two re-render paths into the cheap RESTYLE (applyCanvasMode: re-point the
// once-assigned caches, O(1), no I/O) and the expensive REBUILD
// (rebuildSessionList: re-derive the item list and, in grouped modes, run the lazy
// per-session tmux pane reads worth ~0.5s at ~38 sessions). A swap takes the first
// and never the second, so the prohibitions are asserted rather than merely
// documented on the entry point.

// swapProbeSessions is the number of sessions the probe models carry. It is large
// enough that a rebuild's lazy dir-resolution pass is unmistakable in the read
// counter (one read per empty-Dir session) rather than a single ambiguous call.
const swapProbeSessions = 12

// newSwapProbeModel builds a production-shaped model painted from `before`, in
// the given grouping mode, with the lazy dir-resolution seam wired to `reader`
// and every session's Dir DELIBERATELY EMPTY so the grouped-render pane-read pass
// is armed.
//
// Both pages are rendered once under `before`, because the caches these tests
// reason about are assigned at construction: a model rendered only after the swap
// proves nothing. The probe re-arms the lazy pass afterwards (applySessions
// caches each derived dir back onto m.sessions), so at swap time the reads are
// still there to be paid — which is what makes "zero reads" a real observation
// rather than a consequence of the cache being warm.
func newSwapProbeModel(t *testing.T, before theme.Theme, mode prefs.SessionListMode, reader *fakeStamper) Model {
	t.Helper()
	const w, h = 120, 40

	projects := []project.Project{{Path: reader.path, Name: "Portal", Tags: []string{"work"}}}
	sessions := make([]tmux.Session, 0, swapProbeSessions)
	for i := range swapProbeSessions {
		sessions = append(sessions, tmux.Session{Name: nameN(i), Windows: 1})
	}

	m := Build(Deps{
		Lister:      fakeLister{},
		Theme:       theme.ConstantNomination(before),
		InitialMode: mode,
		DirReader:   reader,
		// The runner resolves the git root of whatever path the reader reports, so
		// the derived directory matches the project record above and the grouped
		// render actually groups.
		DirRunner: &fakeDirRunner{gitRoot: reader.path},
	})
	m.termWidth = w
	m.termHeight = h
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	m.applySessions(sessions)

	// Populate every construction-time cache by rendering both pages under the
	// pre-swap palette.
	m.activePage = PageSessions
	_ = m.viewSessionList()
	m.activePage = PageProjects
	_ = m.viewProjectList()
	m.activePage = PageSessions

	// Re-arm the lazy pass: applySessions cached each derived dir onto m.sessions,
	// and a warm cache would make "zero reads" true for the wrong reason.
	for i := range m.sessions {
		m.sessions[i].Dir = ""
	}
	return m
}

// swapProbeMode is one grouping mode under test, with whether a REBUILD in that
// mode pays lazy pane reads — which decides whether the positive control below
// can run for it.
type swapProbeMode struct {
	name         string
	mode         prefs.SessionListMode
	rebuildReads bool
}

// swapProbeGroupingModes is the full grouping-mode set a swap must be read-free
// in. Flat is included deliberately: it pays no pane reads even on a rebuild, so
// it is the one mode where the counter alone cannot distinguish a restyle from a
// rebuild — the positive control is skipped for it and the mode is covered by the
// rendered-palette half instead.
func swapProbeGroupingModes() []swapProbeMode {
	return []swapProbeMode{
		{"flat", prefs.ModeFlat, false},
		{"by-project", prefs.ModeByProject, true},
		{"by-tag", prefs.ModeByTag, true},
	}
}

// TestApplyTheme_RestylesWithoutRebuild pins §11.1's central prohibition: the
// swap takes the RESTYLE path and never the REBUILD one. A model wired with a
// counting DirReader records ZERO reads across a swap in every grouping mode,
// while the frame it renders afterwards carries the new palette and none of the
// old — so the zero is a restyle that happened, not a swap that did nothing.
//
// The grouped modes additionally run a positive control: a real
// rebuildSessionList immediately after the swap DOES pay the reads. Without it
// the zero would be indistinguishable from a dead seam.
func TestApplyTheme_RestylesWithoutRebuild(t *testing.T) {
	before, after := probeThemeBefore(), probeThemeAfter()

	for _, tc := range swapProbeGroupingModes() {
		t.Run(tc.name, func(t *testing.T) {
			reader := &fakeStamper{path: t.TempDir()}
			m := newSwapProbeModel(t, before, tc.mode, reader)

			reader.reads = nil
			m.ApplyTheme(after)

			if len(reader.reads) != 0 {
				t.Errorf("ApplyTheme performed %d lazy pane read(s) %v — a swap is the O(1) restyle (§11.1), never the rebuild's dir-resolution pass", len(reader.reads), reader.reads)
			}

			frame := m.viewSessionList()
			assertRepointed(t, tc.name+" sessions frame after ApplyTheme",
				frame, tokenBgSeq(t, after.Canvas), tokenBgSeq(t, before.Canvas))

			if len(reader.reads) != 0 {
				t.Errorf("rendering after ApplyTheme performed %d lazy pane read(s) %v — the render path pays no reads either", len(reader.reads), reader.reads)
			}

			if !tc.rebuildReads {
				return
			}
			// Positive control: the seam is live and the mode's rebuild really
			// would have paid these reads, so the zero above is a genuine absence.
			m.rebuildSessionList()
			if len(reader.reads) == 0 {
				t.Fatal("positive control: rebuildSessionList performed no pane reads, so the counting DirReader proves nothing about ApplyTheme")
			}
		})
	}
}

// countingStores is the set of file-touching seams a Model can reach, each
// counting its calls. Together they cover every route from the model to disk:
// projects.json (store + editor), the aliases file, prefs.json — BOTH of its
// seams, the `s`-key mode persister and §8.9's theme persister — and a pane's
// scrollback .bin.
//
// The theme persister is counted for the same reason the mode persister is: it
// is a second prefs.json-touching seam, so a set that held only the first would
// let a swap path grow a prefs write while this guard's "every route from the
// model to disk" claim stayed nominally true.
//
// The theme FILES are NOT among them, and cannot be: a palette arrives at
// ApplyTheme as a loaded value, and the Model holds no theme.Loader at all —
// which the companion assertion below pins structurally, since a loader field is
// the only way a future swap path could grow a theme file read.
type countingStores struct {
	projectStore   *countingProjectStore
	projectEditor  *countingProjectEditor
	aliasEditor    *countingAliasEditor
	modePersister  *countingModePersister
	themePersister *countingThemePersister
	scrollback     *countingScrollbackReader
	lister         *countingLister
}

// calls is the total number of seam calls recorded across every store.
func (c countingStores) calls() int {
	return c.projectStore.calls + c.projectEditor.calls + c.aliasEditor.calls +
		c.modePersister.calls + c.themePersister.calls + c.scrollback.calls + c.lister.calls
}

// reset zeroes every counter, so a subsequent observation measures only what
// happened after it.
func (c countingStores) reset() {
	c.projectStore.calls = 0
	c.projectEditor.calls = 0
	c.aliasEditor.calls = 0
	c.modePersister.calls = 0
	c.themePersister.calls = 0
	c.scrollback.calls = 0
	c.lister.calls = 0
}

// exercise calls every seam once. It is the counters' own positive control: a
// counter that never increments would make the zero-calls assertion pass for the
// wrong reason.
//
// The theme persister's two methods share ONE counter, so exercising either is
// exercising the seam.
func (c countingStores) exercise() {
	_, _ = c.projectStore.List()
	_ = c.projectEditor.AddTag("/x", "y")
	_, _ = c.aliasEditor.Load()
	_ = c.modePersister.Save(prefs.ModeFlat)
	_ = c.themePersister.CommitTheme("nord")
	_, _ = c.scrollback.Tail("pane")
	_, _ = c.lister.ListSessions()
}

type countingProjectStore struct{ calls int }

func (c *countingProjectStore) List() ([]project.Project, error) {
	c.calls++
	return nil, nil
}

func (c *countingProjectStore) CleanStale() ([]project.Project, error) {
	c.calls++
	return nil, nil
}

func (c *countingProjectStore) Remove(string, string) error {
	c.calls++
	return nil
}

type countingProjectEditor struct{ calls int }

func (c *countingProjectEditor) Rename(string, string, string) error { c.calls++; return nil }
func (c *countingProjectEditor) AddTag(string, string) error         { c.calls++; return nil }
func (c *countingProjectEditor) RemoveTag(string, string) error      { c.calls++; return nil }

type countingAliasEditor struct{ calls int }

func (c *countingAliasEditor) Load() (map[string]string, error) { c.calls++; return nil, nil }
func (c *countingAliasEditor) SetAndSave(string, string, string) error {
	c.calls++
	return nil
}

func (c *countingAliasEditor) DeleteAndSave(string, string) (bool, error) {
	c.calls++
	return true, nil
}

type countingModePersister struct{ calls int }

func (c *countingModePersister) Save(prefs.SessionListMode) error { c.calls++; return nil }

// countingThemePersister is §8.9's commit seam, counted. Both methods share one
// counter: what the guard asks is whether the swap reached prefs.json at all, and
// which of the two commit keys would have done it is a distinction it has no use
// for.
type countingThemePersister struct{ calls int }

func (c *countingThemePersister) CommitTheme(string) error { c.calls++; return nil }

func (c *countingThemePersister) CommitThemeSlot(string, theme.Member) error {
	c.calls++
	return nil
}

type countingScrollbackReader struct{ calls int }

func (c *countingScrollbackReader) Tail(string) ([]byte, error) { c.calls++; return nil, nil }

type countingLister struct{ calls int }

func (c *countingLister) ListSessions() ([]tmux.Session, error) { c.calls++; return nil, nil }

// newCountingStores wires a fresh counter onto every file-touching seam.
func newCountingStores() countingStores {
	return countingStores{
		projectStore:   &countingProjectStore{},
		projectEditor:  &countingProjectEditor{},
		aliasEditor:    &countingAliasEditor{},
		modePersister:  &countingModePersister{},
		themePersister: &countingThemePersister{},
		scrollback:     &countingScrollbackReader{},
		lister:         &countingLister{},
	}
}

// TestApplyTheme_PerformsNoFileRead pins the second §11.1 prohibition: a swap
// touches no file. §5.8 is what makes this reachable at all — the panel retains
// its enumeration's parse results for its lifetime, so arrowing previews from
// values already in hand and the palette reaches ApplyTheme as a loaded value.
//
// Both routes to disk are covered: every file-touching seam the model holds is
// counted across fifty swaps, and the Model is asserted to hold no theme.Loader,
// which is the only shape through which a swap path could grow a theme file read
// of its own.
func TestApplyTheme_PerformsNoFileRead(t *testing.T) {
	before, after := probeThemeBefore(), probeThemeAfter()

	t.Run("no seam reaches disk across fifty swaps", func(t *testing.T) {
		stores := newCountingStores()

		// Positive control FIRST: prove each counter increments, so the zero
		// below is an absence of calls rather than an absence of counting.
		stores.exercise()
		if stores.calls() != 7 {
			t.Fatalf("positive control: exercising the seven seams recorded %d calls, want 7 — the counters do not count", stores.calls())
		}

		m := Build(Deps{
			Lister:         stores.lister,
			Theme:          theme.ConstantNomination(before),
			ProjectStore:   stores.projectStore,
			ProjectEditor:  stores.projectEditor,
			AliasEditor:    stores.aliasEditor,
			ModePersister:  stores.modePersister,
			ThemePersister: stores.themePersister,
			Reader:         stores.scrollback,
		})
		m.termWidth, m.termHeight = 120, 40
		m.applySessionListSize(m.contentWidth(), m.contentHeight())
		_ = m.viewSessionList()

		stores.reset()
		for range 50 {
			m.ApplyTheme(after)
			m.ApplyTheme(before)
		}

		if got := stores.calls(); got != 0 {
			t.Errorf("fifty swaps made %d file-touching seam call(s) — a swap reads nothing (project store %d, project editor %d, alias editor %d, mode persister %d, theme persister %d, scrollback %d, lister %d)",
				got, stores.projectStore.calls, stores.projectEditor.calls, stores.aliasEditor.calls,
				stores.modePersister.calls, stores.themePersister.calls, stores.scrollback.calls, stores.lister.calls)
		}
	})

	t.Run("the model holds no theme loader", func(t *testing.T) {
		for _, path := range loaderFields(reflect.TypeFor[Model](), "Model") {
			t.Errorf("%s is a theme.Loader — a swap takes a LOADED palette (§5.8); a loader on the model is the shape through which one would grow a file read", path)
		}
	})
}

// loaderFields returns the dotted paths of every theme.Loader-typed field reachable
// from tp, descending through the model's OWN nested state structs (themeState,
// themePanel and their kin).
//
// The descent is what keeps the guard honest: the model groups its theme machinery
// into a nested struct, so a top-level-only walk would miss a loader parked exactly
// where one would be reached for. Third-party field types are not descended into —
// a bubbles/list is not somewhere this package can grow a file read.
func loaderFields(tp reflect.Type, path string) []string {
	var found []string
	for f := range tp.Fields() {
		at := path + "." + f.Name
		switch {
		case f.Type == reflect.TypeFor[theme.Loader]():
			found = append(found, at)
		case f.Type.Kind() == reflect.Struct && f.Type.PkgPath() == tp.PkgPath():
			found = append(found, loaderFields(f.Type, at)...)
		}
	}
	return found
}

// newSwapFrameModel builds a production-shaped, fully renderable model painted
// from `before` — enough sessions and projects for both lists to have content —
// and renders one full frame under it, populating the construction-time caches.
//
// The colourless flag is the NO_COLOR carve-out (§2.5); a colourless model paints
// no canvas at all, which is what the carve-out swap assertion needs.
func newSwapFrameModel(t *testing.T, before theme.Theme, colourless bool) Model {
	t.Helper()
	const w, h = 120, 40

	sessions := make([]tmux.Session, 0, swapProbeSessions)
	projects := make([]project.Project, 0, swapProbeSessions)
	for i := range swapProbeSessions {
		sessions = append(sessions, tmux.Session{Name: nameN(i), Windows: 1, Dir: "/Users/leeovery/Code/" + nameN(i)})
		projects = append(projects, project.Project{Name: nameN(i), Path: "/Users/leeovery/Code/" + nameN(i)})
	}

	m := Build(Deps{
		Lister:  fakeLister{},
		Theme:   theme.ConstantNomination(before),
		NoColor: colourless,
	})
	m.termWidth = w
	m.termHeight = h
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	m.applySessions(sessions)

	// The pre-swap render is not set-up: the caches under test are assigned once,
	// so a frame taken only after the swap would pass trivially (§13.4).
	frame := m.View().Content
	if !strings.Contains(frame, nameN(0)) {
		t.Fatalf("probe setup: the pre-swap frame does not render the session rows, so a frame comparison would compare nothing: %q", escSeq(frame))
	}
	return m
}

// TestApplyTheme_DoesNotMoveStartupCanvasHex pins the third §11.1 prohibition and
// the anchor §11.4's exit-time canvas restore rests on: startupCanvasHex is the
// canvas in force during the STARTUP window, frozen when the gate resolved. A
// mid-session commit — or a quit with an uncommitted preview — moves the active
// theme; neither may move this, or RestoreTerminalBackground would compare the
// terminal's echoed original against the wrong canvas.
//
// Both counts are asserted because the failure modes differ: a single swap that
// writes it moves it once, while a swap that merely re-derives it would drift only
// under repetition.
func TestApplyTheme_DoesNotMoveStartupCanvasHex(t *testing.T) {
	before, after := probeThemeBefore(), probeThemeAfter()
	m := newSwapFrameModel(t, before, false)

	startup := m.themeState.startupCanvasHex
	if startup != before.Canvas.Value {
		t.Fatalf("probe setup: startupCanvasHex = %q, want the pre-swap canvas %q", startup, before.Canvas.Value)
	}

	m.ApplyTheme(after)
	if m.themeState.startupCanvasHex != startup {
		t.Errorf("after one swap startupCanvasHex = %q, want %q unchanged — it is frozen at gate resolution (§11.4), never re-derived from the active theme", m.themeState.startupCanvasHex, startup)
	}

	for range 50 {
		m.ApplyTheme(before)
		m.ApplyTheme(after)
	}
	if m.themeState.startupCanvasHex != startup {
		t.Errorf("after fifty swaps startupCanvasHex = %q, want %q unchanged", m.themeState.startupCanvasHex, startup)
	}
}

// TestApplyTheme_SameThemeIsANoOp pins that swapping to the theme already active
// is legal and observably inert: the post-swap frame is byte-identical to the
// pre-swap one. The panel reaches this on every arrow keypress that lands back on
// the active row, and on a close that discards nothing.
func TestApplyTheme_SameThemeIsANoOp(t *testing.T) {
	before := probeThemeBefore()
	m := newSwapFrameModel(t, before, false)

	frameBefore := m.View().Content
	m.ApplyTheme(before)
	frameAfter := m.View().Content

	if frameAfter != frameBefore {
		t.Errorf("swapping to the ACTIVE theme changed the frame; a same-theme swap is a no-op\nbefore: %q\nafter:  %q", escSeq(frameBefore), escSeq(frameAfter))
	}
}

// TestApplyTheme_RepeatedSwapsAreIdempotent pins that the swap carries no
// accumulating state: A→B→B renders exactly what A→B renders. It is driven over
// ONE model, because that is the only shape in which "repeated" means anything —
// two models would each be a first swap.
func TestApplyTheme_RepeatedSwapsAreIdempotent(t *testing.T) {
	before, after := probeThemeBefore(), probeThemeAfter()
	m := newSwapFrameModel(t, before, false)

	m.ApplyTheme(after)
	once := m.View().Content
	m.ApplyTheme(after)
	twice := m.View().Content

	if twice != once {
		t.Errorf("A→B→B does not render what A→B renders; the swap accumulates state\nonce:  %q\ntwice: %q", escSeq(once), escSeq(twice))
	}
}

// TestApplyTheme_ColourlessStaysColourless pins the NO_COLOR carve-out across a
// swap (§2.5): under NO_COLOR Portal imposes no hue at all, and a theme swap must
// not be the thing that reintroduces one. The post-swap frame carries no truecolor
// SGR in either slot.
//
// The coloured counterpart is asserted alongside as the positive control: without
// it, a frame that carried no colour for some unrelated reason would satisfy the
// negative vacuously.
func TestApplyTheme_ColourlessStaysColourless(t *testing.T) {
	const (
		fgTruecolor = "38;2;"
		bgTruecolor = "48;2;"
	)
	before, after := probeThemeBefore(), probeThemeAfter()

	colourless := newSwapFrameModel(t, before, true)
	colourless.ApplyTheme(after)
	frame := colourless.View().Content

	if strings.Contains(frame, fgTruecolor) {
		t.Errorf("post-swap colourless frame carries a %q foreground sequence — NO_COLOR imposes no hue, and a swap may not reintroduce one: %q", fgTruecolor, escSeq(frame))
	}
	if strings.Contains(frame, bgTruecolor) {
		t.Errorf("post-swap colourless frame carries a %q background sequence — NO_COLOR paints no canvas, and a swap may not reintroduce one: %q", bgTruecolor, escSeq(frame))
	}

	// Positive control: the same fixture WITH colour does carry both, so the two
	// negatives above are absences rather than a frame that never had colour.
	coloured := newSwapFrameModel(t, before, false)
	coloured.ApplyTheme(after)
	control := coloured.View().Content
	if !strings.Contains(control, fgTruecolor) || !strings.Contains(control, bgTruecolor) {
		t.Fatal("positive control: the COLOURED post-swap frame carries no truecolor SGR, so the colourless assertions prove nothing")
	}
}
