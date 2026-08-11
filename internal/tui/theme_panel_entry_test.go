package tui

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
)

// The entry-condition rule's ENTRY CONDITIONS: `t` opens the panel on Sessions and Projects,
// and nothing blocks it except a modal, a pending burst, `NO_COLOR`, a terminal below
// either render-floor dimension, and the pages where it is not bound at all.
//
// THE TWO REFUSALS ARE DELIBERATELY OPPOSITE CALLS. `NO_COLOR` is a
// CAPABILITY ABSENCE — no canvas, no hues, a preview of nothing — so it is blocked
// proactively exactly as `m` already is; a narrow or short terminal is a SPACE
// SHORTAGE, which the degrade-never-break doctrine mandates degrading for, and which refuses
// only once even the minimum panel cannot render. Conflating them gets one of them wrong in
// the one direction that matters: a proactive width block refuses terminals that would have
// rendered a good panel.
//
// FEEDBACK FOLLOWS THE EXISTING PRECEDENT: FLASH where the key IS bound and the
// user could reasonably expect it to work, SILENT where it is not bound at all —
// exactly how `s` already behaves. Hence the Preview / Loading / modal / burst cases
// assert an ABSENCE of both the panel and the flash.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// The pinned copy entry copy, written out VERBATIM here rather than read from the
// production constants — a test that asserts a constant against itself pins
// nothing, and this copy is the spec's, not the implementation's. (Its resize
// siblings live beside the geometry suite as specNarrowClosedFlash /
// specShortClosedFlash.)
const (
	specNoColorEntryFlash = "theme picker needs colour — NO_COLOR is set"
	specNarrowEntryFlash  = "terminal too narrow for the theme picker"
	specShortEntryFlash   = "terminal too short for the theme picker"
)

// entryContentW / entryContentH are the content region every UNBLOCKED entry
// fixture runs at — clear of both floors with room to spare, and wide enough that
// the page beneath still shows its own left-hand columns beside the panel (which is
// what makes the multi-select banner observable behind it).
const (
	entryContentW = 96
	entryContentH = 26
)

// entryRows are the panel's union for an entry fixture: four selectable rows, so an
// opened panel has a body to render and a row for the cursor to anchor on.
func entryRows(t *testing.T) []theme.Row {
	t.Helper()
	return arrowValidRows(t, 4)
}

// newEntryEnumerator is the recording seam an entry fixture reads through. The
// recorded OPEN COUNT is what discriminates the two refusal shapes: a proactive
// block reads nothing, while the post-read re-evaluation has already enumerated by
// the time it refuses.
func newEntryEnumerator(t *testing.T, dirUnusable bool) *fakeThemeSource {
	t.Helper()
	rows := entryRows(t)
	return &fakeThemeSource{
		enumeration: theme.Enumeration{DirPath: fixtureThemesDir},
		union:       themeRowsUnionDirUnusable(rows, dirUnusable),
		resolution:  constantResolution(rows[0].Slug, rows[0].Theme),
	}
}

// entryModelOpts is the state one entry fixture needs. It is a struct rather than a
// parameter list because a positional call would read as a row of anonymous
// literals — the same reason themePanelFixtureOpts is one.
type entryModelOpts struct {
	page       page
	colourless bool
	contentW   int
	contentH   int
}

// newEntryModel builds a model with BOTH pages populated, both lists sized by
// production and the panel still CLOSED, at the requested content region.
//
// Every page-action seam is wired (killer, renamer, creator, project store and
// editors) because the routing table below needs each key's own effect as a CONTROL:
// a key that silently stopped working outside the panel would otherwise pass a
// swallow assertion for the wrong reason.
//
// The dimensions are assigned directly rather than through a tea.WindowSizeMsg, so
// the entry gate is asked about the region the fixture declares and nothing has
// resized on the way in.
func newEntryModel(t *testing.T, e ThemeSource, o entryModelOpts) (Model, *fakeThemeSource) {
	t.Helper()

	rec, _ := e.(*fakeThemeSource)
	rows := entryRows(t)
	deps := Deps{
		Lister:        fakeLister{},
		Killer:        keymapParityKiller{},
		Renamer:       keymapParityRenamer{},
		Creator:       sessionsGuardCreator{},
		ProjectStore:  projectsParityStore{},
		ProjectEditor: stubTagsProjectEditor{},
		AliasEditor:   stubTagsAliasEditor{},
		Enumerator:    keymapParityEnumerator{},
		Reader:        keymapParityReader{},
		ThemeSource:   e,
		Theme:         theme.ConstantNomination(rows[0].Theme),
		ThemeKeys:     theme.RawKeys{Theme: rows[0].Slug},
		NoColor:       o.colourless,
	}
	m := Build(deps)
	m.termWidth, m.termHeight = geometryTerm(o.contentW, o.contentH)
	m.applySessions(closePanelSessions())
	projects := []project.Project{{Path: "/p/one", Name: "one"}, {Path: "/p/two", Name: "two"}}
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.projectList.Select(0)
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())
	m.activePage = o.page

	if got := m.contentWidth(); got != o.contentW {
		t.Fatalf("fixture: the content region is %d columns wide, want %d", got, o.contentW)
	}
	if got := m.contentHeight(); got != o.contentH {
		t.Fatalf("fixture: the content region is %d rows tall, want %d", got, o.contentH)
	}
	if m.themePanel.open {
		t.Fatal("fixture: the panel is already open")
	}
	return m, rec
}

// newUnblockedEntryModel is the fixture at the standard unblocked content region.
func newUnblockedEntryModel(t *testing.T, p page) (Model, *fakeThemeSource) {
	t.Helper()
	return newEntryModel(t, newEntryEnumerator(t, false), entryModelOpts{
		page:     p,
		contentW: entryContentW,
		contentH: entryContentH,
	})
}

// entryPages is the pair of pages the panel-open rule binds `t` on. Theme is a GLOBAL setting,
// so every entry rule is a statement about both — one gate, two dispatch sites.
var entryPages = []struct {
	name string
	page page
}{
	{name: "sessions", page: PageSessions},
	{name: "projects", page: PageProjects},
}

// requireBlocked asserts a `t` press was refused with the pinned copy: no panel,
// the flash raised, and a scheduled auto-clear.
func requireBlocked(t *testing.T, m Model, cmd tea.Cmd, wantFlash string) {
	t.Helper()
	if m.themePanel.open {
		t.Fatal("t opened the panel where the entry gate must refuse")
	}
	if got := m.flashText; got != wantFlash {
		t.Errorf("the blocked t raised %q, want §14A's %q", got, wantFlash)
	}
	if cmd == nil {
		t.Error("the blocked t scheduled no auto-clear tick; a blocked entry inherits the standard flash lifecycle")
	}
}

// requireFlashBandVisible asserts the raised copy reaches the RENDERED frame — the
// page's own notice band actually carrying it, rather than a model field nobody sees.
//
// It is called only where the content region is wide enough to hold the message on
// ONE line. The band wraps below that (by design — renderNoticeBand wraps to the
// available width), which is a property of the band rather than of the block, and the
// blocked-entry copy is pinned by requireBlocked either way.
func requireFlashBandVisible(t *testing.T, m Model, wantFlash string) {
	t.Helper()
	if got := ansi.Strip(m.View().Content); !strings.Contains(got, wantFlash) {
		t.Errorf("the frame carries no %q band:\n%s", wantFlash, got)
	}
}

// pressThemeKeyCmd drives the production `t` through the live Update and returns
// the command alongside the model — the blocked path's tick rides it.
func pressThemeKeyCmd(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	return updated.(Model), cmd
}

// requireSilentRefusal asserts `t` did nothing at all: no panel, no directory read,
// and — the half that distinguishes this from a block — NO FLASH.
func requireSilentRefusal(t *testing.T, m Model, rec *fakeThemeSource, where string) {
	t.Helper()
	if m.themePanel.open {
		t.Errorf("t opened the panel on %s, where it is not bound", where)
	}
	if rec.opens != 0 {
		t.Errorf("t ran %d enumerations on %s, want 0", rec.opens, where)
	}
	if got := m.flashText; got != "" {
		t.Errorf("t raised %q on %s; the refusal is SILENT where the key is not bound at all (§9.7)", got, where)
	}
}

// armPanelUnderNoColorForTest opens the panel WITHOUT the `t` keypress.
//
// The NO_COLOR panel block blocks `t` under NO_COLOR, so a COLOURLESS panel is unreachable
// through the production keypress BY DECISION. The colourless RENDER path is still real — the
// renderer and the row delegate both carry the flag, and the NO_COLOR carve-out is asserted
// across every other surface — so the two fixtures that assert it (the arrow suite's
// colourless twin and the open suite's colourless dot row) arm the panel here instead
// of pressing a key the gate now refuses.
//
// It is the SMALLEST POSSIBLE BYPASS: the same enumeration the open would have read,
// installed by the same armThemePanel, with only the gate skipped.
func armPanelUnderNoColorForTest(t *testing.T, m Model) Model {
	t.Helper()
	if !m.colourless {
		t.Fatal("fixture: the model is not colourless, so `t` is not blocked and this bypass says nothing")
	}
	enumeration, union := m.themeState.source.Open(m.themeState.keys)
	(&m).armThemePanel(enumeration, union)
	if !m.themePanel.open {
		t.Fatal("fixture: arming the panel directly left it closed")
	}
	return m
}

// TestPanelEntry_OpensOnSessionsAndProjects: it opens from both pages when
// unblocked.
//
// The positive control the whole suite rests on: with nothing blocking, `t` reads
// the directory once and arms the panel on either page, and raises no flash — so
// every refusal below is a refusal rather than a fixture that never worked.
func TestPanelEntry_OpensOnSessionsAndProjects(t *testing.T) {
	for _, tc := range entryPages {
		t.Run(tc.name, func(t *testing.T) {
			m, rec := newUnblockedEntryModel(t, tc.page)

			m, _ = pressThemeKeyCmd(t, m)

			if !m.themePanel.open {
				t.Fatalf("t did not open the panel on %s", tc.name)
			}
			if rec.opens != 1 {
				t.Errorf("the keypress ran %d enumerations, want exactly 1", rec.opens)
			}
			if got := m.flashText; got != "" {
				t.Errorf("an unblocked t raised %q, want no flash", got)
			}
			if m.activePage != tc.page {
				t.Errorf("opening the panel moved the active page to %d, want %d", m.activePage, tc.page)
			}
		})
	}
}

// TestPanelEntry_NoColorBlocked: it blocks under NO_COLOR with the pinned copy.
//
// The NO_COLOR panel block: under `NO_COLOR` Portal paints no canvas and imposes no hues, so
// the panel previews nothing, its cursor tint and slot dots have no colour, and committing
// would persist a choice with zero visible feedback. It is a CAPABILITY ABSENCE and
// is blocked PROACTIVELY, following the multi-select precedent exactly — the block
// is what keeps the user out of a walkable dead end.
//
// THE DIRECTORY IS NOT READ. A proactive block decides before the keypress does any
// work, which is what distinguishes this refusal from the post-read one below.
//
// Both pages are driven here, and the flash is asserted in the RENDERED FRAME on
// each: the pinned copy gave Projects its own transient-flash slot precisely so this block is
// not a silent no-op there. That makes this the suite's page-parity statement — the
// floor cases below drive one page, since both consume the same gate.
func TestPanelEntry_NoColorBlocked(t *testing.T) {
	for _, tc := range entryPages {
		t.Run(tc.name, func(t *testing.T) {
			m, rec := newEntryModel(t, newEntryEnumerator(t, false), entryModelOpts{
				page:       tc.page,
				colourless: true,
				contentW:   entryContentW,
				contentH:   entryContentH,
			})

			m, cmd := pressThemeKeyCmd(t, m)

			requireBlocked(t, m, cmd, specNoColorEntryFlash)
			requireFlashBandVisible(t, m, specNoColorEntryFlash)
			if rec.opens != 0 {
				t.Errorf("the blocked t ran %d enumerations, want 0 — the NO_COLOR block is proactive", rec.opens)
			}
		})
	}
}

// TestPanelEntry_FloorBlocked: it blocks below each render-floor dimension.
//
// The entry-condition rule: below the render floor `t` refuses with a flash rather than
// opening a broken frame — "an ENTRY condition, not only the resize condition the geometry
// rule describes" — and the pinned copy pins a string per dimension.
//
// WIDTH IS CHECKED FIRST, so a terminal failing BOTH reports narrow. That is the
// dimension the user can act on with the same gesture that broke it, and pinning the
// order is what keeps the entry copy and the forced-close copy identical on a
// terminal that is both.
func TestPanelEntry_FloorBlocked(t *testing.T) {
	floor := themePanelMinHeight(themePanelKeymap(), false)

	for _, tc := range []struct {
		name      string
		contentW  int
		contentH  int
		wantFlash string
	}{
		{name: "too narrow", contentW: themePanelMinWidth - 1, contentH: entryContentH, wantFlash: specNarrowEntryFlash},
		{name: "too short", contentW: entryContentW, contentH: floor - 1, wantFlash: specShortEntryFlash},
		{name: "both fail", contentW: themePanelMinWidth - 1, contentH: floor - 1, wantFlash: specNarrowEntryFlash},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, rec := newEntryModel(t, newEntryEnumerator(t, false), entryModelOpts{
				page:     PageSessions,
				contentW: tc.contentW,
				contentH: tc.contentH,
			})

			m, cmd := pressThemeKeyCmd(t, m)

			requireBlocked(t, m, cmd, tc.wantFlash)
			if rec.opens != 0 {
				t.Errorf("the blocked t ran %d enumerations, want 0 — the floor is read before the directory is", rec.opens)
			}
		})
	}
}

// TestPanelEntry_UsableDirectoryOpensAtTheNonDirFloor: it does not reserve the
// directory row before it knows.
//
// The floor is CONDITIONAL on `DirUnusable`, and that flag does not exist
// until the enumeration runs on the keypress — so the pre-read evaluation passes
// `dirUnusable = false`. Assuming `true` up front is the shortcut this test forbids:
// it would refuse terminals that render a perfectly good panel, contradicting the geometry
// rule's degrade-don't-refuse doctrine outright.
//
// The height is EXACTLY the non-directory floor, which is one row below the
// directory-inclusive one — the only height at which the two assumptions differ.
func TestPanelEntry_UsableDirectoryOpensAtTheNonDirFloor(t *testing.T) {
	entries := themePanelKeymap()
	floor := themePanelMinHeight(entries, false)
	if themePanelMinHeight(entries, true) != floor+1 {
		t.Fatalf("fixture: the directory-inclusive floor is %d and the plain one %d; the two must differ by exactly one row or this height discriminates nothing",
			themePanelMinHeight(entries, true), floor)
	}

	m, rec := newEntryModel(t, newEntryEnumerator(t, false), entryModelOpts{
		page:     PageSessions,
		contentW: entryContentW,
		contentH: floor,
	})

	m, _ = pressThemeKeyCmd(t, m)

	if !m.themePanel.open {
		t.Fatalf("t refused at exactly the %d-row floor with a USABLE directory — the gate reserved the `%s` row speculatively", floor, themePanelDirUnreadable)
	}
	if rec.opens != 1 {
		t.Errorf("the keypress ran %d enumerations, want exactly 1", rec.opens)
	}
	if got := m.flashText; got != "" {
		t.Errorf("the open raised %q, want no flash", got)
	}
}

// TestPanelEntry_UnusableDirectoryBlocksOnTheReEvaluation: it blocks after the read
// when the directory row raises the floor.
//
// The other half of the conditional floor, and the other shortcut refused: assuming
// `false` and never re-checking would open a panel whose list body is ZERO ROWS —
// the pinned warning consuming the single row the row-rendering rule requires built-in and
// persisted rows to render BENEATH, which is the "completely in the dark" state that row
// exists to prevent.
//
// So ONE PREDICATE IS EVALUATED TWICE: once before the read with the flag assumed
// false, once as soon as `Open` returns with the real one. The COST IS ACCEPTED
// rather than worked around — the blocked open has already read the directory and
// already emitted `theme: enumerated`, and the read genuinely happened; splitting it
// from its emission would fork the seam for one edge. The recorded open count is
// that cost, asserted rather than hidden.
func TestPanelEntry_UnusableDirectoryBlocksOnTheReEvaluation(t *testing.T) {
	entries := themePanelKeymap()
	floor := themePanelMinHeight(entries, false)

	t.Run("at the non-directory floor it discards the enumeration and refuses", func(t *testing.T) {
		m, rec := newEntryModel(t, newEntryEnumerator(t, true), entryModelOpts{
			page:     PageSessions,
			contentW: entryContentW,
			contentH: floor,
		})

		m, cmd := pressThemeKeyCmd(t, m)

		requireBlocked(t, m, cmd, specShortEntryFlash)
		requireFlashBandVisible(t, m, specShortEntryFlash)
		if rec.opens != 1 {
			t.Errorf("the re-evaluation ran %d enumerations, want exactly 1 — the read happens, then the floor refuses", rec.opens)
		}
		if got := m.themePanel; got.width != 0 || len(got.union.Rows) != 0 || len(got.enumeration.Entries) != 0 || got.badges != nil {
			t.Errorf("the refused open retained panel state %+v, want the zero value — the enumeration is discarded", got)
		}
	})

	t.Run("one row higher it opens with a list row beneath the warning", func(t *testing.T) {
		height := themePanelMinHeight(entries, true)
		m, _ := newEntryModel(t, newEntryEnumerator(t, true), entryModelOpts{
			page:     PageSessions,
			contentW: entryContentW,
			contentH: height,
		})

		m, _ = pressThemeKeyCmd(t, m)

		if !m.themePanel.open {
			t.Fatalf("t refused at the %d-row directory-inclusive floor, which §9.8 admits", height)
		}
		lines := themePanelLines(renderThemePanel(m.themePanel, m.contentHeight(), m.themeState.active, m.colourless))
		if len(lines) != height {
			t.Fatalf("the panel rendered %d rows at the %d-row floor", len(lines), height)
		}
		header := panelHeaderRowsOf(m)
		if got, want := strings.TrimRight(lines[header], " "), themePanelContentPrefix()+themePanelDirUnreadable; got != want {
			t.Fatalf("the row under the header = %q, want the pinned %q", got, want)
		}
		if got, want := lines[header+1], entryRows(t)[0].Label(); !strings.Contains(got, want) {
			t.Errorf("the row beneath the warning = %q, want the list row %q — §9.5 requires rows BENEATH it", got, want)
		}
	})
}

// TestPanelEntry_SameFloorAsResize: it shares one floor predicate with the resize
// path.
//
// The entry condition and the resize condition are the
// SAME question, and a terminal that passes one check and fails the other is exactly what two
// independent derivations produce — a panel refused where it would have fitted, or a broken
// frame opened where a resize would have closed it.
//
// The table drives both paths at each region and asserts the SAME answer: a size
// that blocks entry also force-closes an open panel, on the same dimension, with the
// entry / forced-close halves of the pinned copy's per-dimension pair.
func TestPanelEntry_SameFloorAsResize(t *testing.T) {
	floor := themePanelMinHeight(themePanelKeymap(), false)

	for _, region := range [][2]int{
		{entryContentW, entryContentH},
		{themePanelMinWidth, floor},
		{themePanelMinWidth - 1, entryContentH},
		{entryContentW, floor - 1},
		{themePanelMinWidth - 1, floor - 1},
		{60, entryContentH},
	} {
		contentW, contentH := region[0], region[1]
		t.Run(fmt.Sprintf("%dx%d", contentW, contentH), func(t *testing.T) {
			dim, ok := themePanelFloor(contentW, contentH, false)

			entered, _ := pressThemeKeyCmd(t, mustEntryModel(t, contentW, contentH))
			if entered.themePanel.open != ok {
				t.Fatalf("t left the panel open=%v, want %v — the predicate said (%v, ok=%v)", entered.themePanel.open, ok, dim, ok)
			}

			opened, _ := pressThemeKeyCmd(t, mustEntryModel(t, entryContentW, entryContentH))
			if !opened.themePanel.open {
				t.Fatal("fixture: the panel did not open at the unblocked region")
			}
			resized := resizeForTest(t, opened, contentW, contentH)
			if resized.themePanel.open != ok {
				t.Fatalf("the resize left the panel open=%v, want %v — one predicate, two callers", resized.themePanel.open, ok)
			}

			if ok {
				if got := entered.flashText; got != "" {
					t.Errorf("an admitted entry raised %q, want no flash", got)
				}
				if got := resized.flashText; got != "" {
					t.Errorf("an admitted resize raised %q, want no flash", got)
				}
				return
			}
			if got, want := entered.flashText, specEntryFlash(dim); got != want {
				t.Errorf("the blocked entry raised %q, want the %v copy %q", got, dim, want)
			}
			// The forced-close half reads production's own selector: its copy is pinned
			// verbatim against the pinned copy by the geometry suite (specNarrowClosedFlash /
			// specShortClosedFlash), so what is asserted here is that the two callers agree
			// on the DIMENSION, not what that dimension's words are.
			if got, want := resized.flashText, themePanelForcedCloseFlash(dim); got != want {
				t.Errorf("the forced close raised %q, want the %v copy %q", got, dim, want)
			}
		})
	}
}

// specEntryFlash is the pinned copy entry copy for the dimension the floor refused on,
// stated HERE rather than taken from production's own selector — an expectation
// derived from the code under test asserts nothing about which copy is right.
func specEntryFlash(dim themePanelDim) string {
	if dim == dimHeight {
		return specShortEntryFlash
	}
	return specNarrowEntryFlash
}

// mustEntryModel is newEntryModel at a content region, discarding the seam.
func mustEntryModel(t *testing.T, contentW, contentH int) Model {
	t.Helper()
	m, _ := newEntryModel(t, newEntryEnumerator(t, false), entryModelOpts{
		page:     PageSessions,
		contentW: contentW,
		contentH: contentH,
	})
	return m
}

// TestPanelEntry_SilentOnPreviewAndLoading: it is silent where `t` is not bound.
//
// The panel-open rule refuses both pages, and the entry-condition rule's feedback rule makes
// both SILENT: a flash is for a key that IS bound where the user could reasonably expect it to
// work, which is exactly how `s` already behaves.
//
// PREVIEW is refused on two grounds — its body is captured real ANSI scrollback that
// is deliberately OUT OF THEME, so live preview would re-theme the frame chrome
// alone (a weak surface), and it is ALREADY a full-screen overlay, so the panel would
// stack an overlay on an overlay.
//
// LOADING is inert by design (animation only, which is what contains the
// restore/daemon race surface) and renders no notice band to flash into. It is not a
// corner case: on the cold + TUI path it holds for at least LoadingMinDuration with
// the user watching.
func TestPanelEntry_SilentOnPreviewAndLoading(t *testing.T) {
	t.Run("preview", func(t *testing.T) {
		m, rec := newUnblockedEntryModel(t, PageSessions)
		m = pressPanelKey(t, m, tea.KeyPressMsg{Code: tea.KeySpace})
		if m.activePage != pagePreview {
			t.Fatalf("fixture: Space landed page %d, want the preview page", m.activePage)
		}

		m, cmd := pressThemeKeyCmd(t, m)

		requireSilentRefusal(t, m, rec, "the preview page")
		if cmd != nil {
			t.Error("t on the preview page returned a command; it is not bound there at all")
		}
		if m.activePage != pagePreview {
			t.Errorf("t moved the preview page to %d", m.activePage)
		}
	})

	t.Run("loading", func(t *testing.T) {
		m, rec := newUnblockedEntryModel(t, PageLoading)

		m, cmd := pressThemeKeyCmd(t, m)

		requireSilentRefusal(t, m, rec, "the loading page")
		if cmd != nil {
			t.Error("t on the loading page returned a command; the page is inert by design")
		}
		if m.activePage != PageLoading {
			t.Errorf("t moved the loading page to %d", m.activePage)
		}
	})
}

// TestPanelEntry_SwallowedWhileBurstPending: it is swallowed during a pending burst.
//
// The entry-condition rule: the burst input-locks the model (only `Ctrl-C`/`Esc` live) because
// it is MID-ASYNC-OPERATION, so swallowing `t` is CONSISTENCY WITH THAT LOCK rather than
// an exception to it. The existing guard at the top of updateSessionList returns
// before any rune dispatch, so this is asserted rather than branched on — a `t` arm
// that had to know about the burst would be the second place the lock is decided.
func TestPanelEntry_SwallowedWhileBurstPending(t *testing.T) {
	m, rec := newUnblockedEntryModel(t, PageSessions)
	m.burstPending = true

	m, cmd := pressThemeKeyCmd(t, m)

	requireSilentRefusal(t, m, rec, "a pending burst")
	if cmd != nil {
		t.Error("t returned a command while the burst input-lock was engaged")
	}
	if !m.burstPending {
		t.Error("t cleared the pending burst")
	}
}

// TestPanelEntry_ModalKeepsTheKey: it never reaches a modal.
//
// The panel-open rule/the entry-condition rule: modals are key-exclusive BY DESIGN, so `t`
// never reaches the gate at all — no panel, no read, and no flash, since the key is not bound
// on the surface that owns the keyboard.
func TestPanelEntry_ModalKeepsTheKey(t *testing.T) {
	for _, tc := range []struct {
		name  string
		page  page
		press tea.KeyPressMsg
		modal modalState
	}{
		{name: "sessions help", page: PageSessions, press: tea.KeyPressMsg{Code: '?', Text: "?"}, modal: modalHelp},
		{name: "sessions kill", page: PageSessions, press: tea.KeyPressMsg{Code: 'k', Text: "k"}, modal: modalKillConfirm},
		{name: "projects delete", page: PageProjects, press: tea.KeyPressMsg{Code: 'd', Text: "d"}, modal: modalDeleteProject},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, rec := newUnblockedEntryModel(t, tc.page)
			m = pressPanelKey(t, m, tc.press)
			if m.modal != tc.modal {
				t.Fatalf("fixture: %v opened modal %d, want %d", tc.press, m.modal, tc.modal)
			}

			m, cmd := pressThemeKeyCmd(t, m)

			requireSilentRefusal(t, m, rec, "an open modal")
			if cmd != nil {
				t.Error("t returned a command while a modal owned the keyboard")
			}
			if m.modal != tc.modal {
				t.Errorf("t closed the modal (now %d, want %d)", m.modal, tc.modal)
			}
		})
	}
}

// TestPanelEntry_BlockedFlashLifecycle: its flash auto-clears like every other.
//
// A blocked `t` raises an ORDINARY transient flash — no bespoke lifecycle — so both
// existing clear routes apply on both pages: the generation-guarded tick, and the
// next actionable key ("one key, one intent").
//
// The tick is dispatched as a MESSAGE rather than by invoking the returned command,
// which would sleep for the real flashAutoClearDuration; requireBlocked has already
// asserted a command was scheduled.
func TestPanelEntry_BlockedFlashLifecycle(t *testing.T) {
	for _, tc := range entryPages {
		blocked := func(t *testing.T) Model {
			t.Helper()
			m, _ := newEntryModel(t, newEntryEnumerator(t, false), entryModelOpts{
				page:       tc.page,
				colourless: true,
				contentW:   entryContentW,
				contentH:   entryContentH,
			})
			m, cmd := pressThemeKeyCmd(t, m)
			requireBlocked(t, m, cmd, specNoColorEntryFlash)
			return m
		}

		t.Run(tc.name+"/the tick clears it", func(t *testing.T) {
			m := blocked(t)

			updated, _ := m.Update(flashTickMsg{Gen: m.flashGen})

			if got := updated.(Model).flashText; got != "" {
				t.Errorf("the matching tick left the flash %q, want it cleared", got)
			}
		})

		t.Run(tc.name+"/the next actionable key clears it", func(t *testing.T) {
			m := blocked(t)

			m = pressPanelKey(t, m, tea.KeyPressMsg{Code: tea.KeyDown})

			if got := m.flashText; got != "" {
				t.Errorf("the next actionable key left the flash %q, want it cleared", got)
			}
		})
	}
}

// TestPanelEntry_PinnedCopy: it single-sources the pinned copy's entry copy.
//
// Portal's convention (spawn.UnsupportedNoopMessage) is one constant per pinned
// string, so no call site can re-word it. The constants are compared against this
// file's own verbatim transcription of the pinned copy rather than against themselves.
func TestPanelEntry_PinnedCopy(t *testing.T) {
	for _, tc := range []struct {
		got, want string
	}{
		{got: themePanelNoColorFlash, want: specNoColorEntryFlash},
		{got: themePanelNarrowEntryFlash, want: specNarrowEntryFlash},
		{got: themePanelShortEntryFlash, want: specShortEntryFlash},
	} {
		if tc.got != tc.want {
			t.Errorf("the pinned constant is %q, want §14A's %q", tc.got, tc.want)
		}
	}
}

// The entry-condition rule's INPUT ROUTING: while the panel is open it OWNS the keyboard.
//
// Pass-through is genuinely bad — `k` would kill the highlighted session while you
// pick a theme, `x` would swap to Projects with the panel open, `m` would start a
// multi-select behind it — so everything the page binds is SWALLOWED. None of that
// reasoning reaches the global quit, and swallowing THAT would take away the user's
// exit key inside a settings surface, so `Ctrl-C` stays live exactly as it does under
// the burst input-lock.
//
// NON-BLANKING AND KEY-EXCLUSIVE ARE NOT IN TENSION: seeing the list without being
// able to drive it IS the live-preview premise (a modal picker would blank the
// page and preview nothing).

// openEntryPanel builds an unblocked fixture on the page and opens the panel through
// the production `t`.
func openEntryPanel(t *testing.T, p page) Model {
	t.Helper()
	m, _ := newUnblockedEntryModel(t, p)
	m, _ = pressThemeKeyCmd(t, m)
	if !m.themePanel.open {
		t.Fatal("fixture: the panel did not open")
	}
	return m
}

// TestPanelRouting_KeyExclusive: it swallows every page key while open.
//
// Each row carries the page the key is BOUND on and that key's own effect, and each
// is driven twice: once with the panel CLOSED as a control, once with it open. The
// control is what makes the swallow an assertion — a key that silently stopped
// working outside the panel would otherwise pass for the wrong reason.
func TestPanelRouting_KeyExclusive(t *testing.T) {
	for _, tc := range []struct {
		name   string
		page   page
		press  tea.KeyPressMsg
		effect func(Model, tea.Cmd) bool
		desc   string
	}{
		{
			name:   "k does not kill",
			page:   PageSessions,
			press:  tea.KeyPressMsg{Code: 'k', Text: "k"},
			effect: func(m Model, _ tea.Cmd) bool { return m.modal == modalKillConfirm },
			desc:   "the kill confirm modal",
		},
		{
			name:   "x does not switch page",
			page:   PageSessions,
			press:  tea.KeyPressMsg{Code: 'x', Text: "x"},
			effect: func(m Model, _ tea.Cmd) bool { return m.activePage == PageProjects },
			desc:   "the Projects page",
		},
		{
			name:   "m does not enter multi-select",
			page:   PageSessions,
			press:  tea.KeyPressMsg{Code: 'm', Text: "m"},
			effect: func(m Model, _ tea.Cmd) bool { return m.MultiSelectActive() },
			desc:   "multi-select mode",
		},
		{
			name:   "/ does not filter",
			page:   PageSessions,
			press:  tea.KeyPressMsg{Code: '/', Text: "/"},
			effect: func(m Model, _ tea.Cmd) bool { return m.sessionList.SettingFilter() },
			desc:   "the filter input",
		},
		{
			name:   "? opens no help",
			page:   PageSessions,
			press:  tea.KeyPressMsg{Code: '?', Text: "?"},
			effect: func(m Model, _ tea.Cmd) bool { return m.modal == modalHelp },
			desc:   "the help modal",
		},
		{
			name:   "s does not cycle the grouping",
			page:   PageSessions,
			press:  tea.KeyPressMsg{Code: 's', Text: "s"},
			effect: func(m Model, _ tea.Cmd) bool { return m.sessionListMode != prefs.ModeFlat },
			desc:   "the next grouping mode",
		},
		{
			name:   "e opens no editor",
			page:   PageProjects,
			press:  tea.KeyPressMsg{Code: 'e', Text: "e"},
			effect: func(m Model, _ tea.Cmd) bool { return m.modal == modalEditProject },
			desc:   "the project edit modal",
		},
		{
			name:   "n mints nothing",
			page:   PageSessions,
			press:  tea.KeyPressMsg{Code: 'n', Text: "n"},
			effect: func(_ Model, cmd tea.Cmd) bool { return cmd != nil },
			desc:   "the new-session-in-cwd command",
		},
		{
			name:   "r opens no rename",
			page:   PageSessions,
			press:  tea.KeyPressMsg{Code: 'r', Text: "r"},
			effect: func(m Model, _ tea.Cmd) bool { return m.modal == modalRename },
			desc:   "the rename modal",
		},
		{
			name:   "q does not quit",
			page:   PageSessions,
			press:  tea.KeyPressMsg{Code: 'q', Text: "q"},
			effect: func(_ Model, cmd tea.Cmd) bool { return isQuitCmd(cmd) },
			desc:   "the quit",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			closed, _ := newUnblockedEntryModel(t, tc.page)
			controlModel, controlCmd := closed.Update(tc.press)
			if !tc.effect(controlModel.(Model), controlCmd) {
				t.Fatalf("precondition: %v does not reach %s with the panel CLOSED, so a swallow proves nothing", tc.press, tc.desc)
			}

			updated, cmd := openEntryPanel(t, tc.page).Update(tc.press)
			m := updated.(Model)

			if tc.effect(m, cmd) {
				t.Errorf("%v reached %s while the panel was open — the panel is key-exclusive (§9.7)", tc.press, tc.desc)
			}
			if !m.themePanel.open {
				t.Errorf("%v closed the panel; only Esc closes it", tc.press)
			}
			if m.activePage != tc.page {
				t.Errorf("%v moved the active page to %d, want it left on %d", tc.press, m.activePage, tc.page)
			}
		})
	}
}

// TestPanelRouting_PanelOwnedKeysNeverReachThePage: it keeps the panel's own keys off
// the page beneath.
//
// `d` and `l` are PANEL-OWNED (the picker idiom gives them the dark and light slots), not
// swallowed — they are inert only until the commit path makes them write. So the assertion
// is an ABSENCE OF THE PAGE'S EFFECT rather than "the model is unchanged": on Projects
// `d` must open NO delete modal, and `l`, which no page binds, must reach no page
// binding either. Stated that way the criterion still holds once the two become
// commit keys.
func TestPanelRouting_PanelOwnedKeysNeverReachThePage(t *testing.T) {
	t.Run("d opens no delete modal on Projects", func(t *testing.T) {
		press := tea.KeyPressMsg{Code: 'd', Text: "d"}

		closed, _ := newUnblockedEntryModel(t, PageProjects)
		control, _ := closed.Update(press)
		if control.(Model).modal != modalDeleteProject {
			t.Fatal("precondition: `d` opens no delete modal with the panel CLOSED, so the absence below proves nothing")
		}

		m := pressPanelKey(t, openEntryPanel(t, PageProjects), press)

		if m.modal != modalNone {
			t.Errorf("`d` opened modal %d while the panel was open; it is the panel's key, not the page's", m.modal)
		}
		if m.activePage != PageProjects {
			t.Errorf("`d` moved the active page to %d", m.activePage)
		}
	})

	t.Run("l reaches no page binding", func(t *testing.T) {
		press := tea.KeyPressMsg{Code: 'l', Text: "l"}

		for _, tc := range entryPages {
			t.Run(tc.name, func(t *testing.T) {
				before := openEntryPanel(t, tc.page)
				updated, cmd := before.Update(press)
				m := updated.(Model)

				if m.modal != modalNone {
					t.Errorf("`l` opened modal %d on %s", m.modal, tc.name)
				}
				if m.activePage != tc.page {
					t.Errorf("`l` moved the active page to %d on %s", m.activePage, tc.name)
				}
				if cmd != nil {
					t.Errorf("`l` produced a page command on %s", tc.name)
				}
				if got, want := m.sessionList.Index(), before.sessionList.Index(); got != want {
					t.Errorf("`l` moved the sessions cursor %d → %d on %s", want, got, tc.name)
				}
				if got, want := m.projectList.Index(), before.projectList.Index(); got != want {
					t.Errorf("`l` moved the projects cursor %d → %d on %s", want, got, tc.name)
				}
				if m.sessionList.SettingFilter() || m.projectList.SettingFilter() {
					t.Errorf("`l` focused a filter input on %s", tc.name)
				}
			})
		}
	})
}

// TestPanelRouting_CtrlCQuits: it keeps Ctrl-C live.
//
// The one exit the entry-condition rule keeps live inside the panel: swallowing it would take
// away the user's exit key inside a settings surface, which is the same call the burst
// input-lock makes for the same reason.
func TestPanelRouting_CtrlCQuits(t *testing.T) {
	for _, tc := range entryPages {
		t.Run(tc.name, func(t *testing.T) {
			_, cmd := openEntryPanel(t, tc.page).Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})

			if !isQuitCmd(cmd) {
				t.Errorf("Ctrl-C did not quit from the open panel on %s — it must never be swallowed", tc.name)
			}
		})
	}
}

// TestPanelRouting_NestsOverMultiSelect: it nests over multi-select without
// disturbing the set.
//
// The entry-condition rule: `t` opens and the marked set is UNAFFECTED — the panel NESTS over
// the mode and `Esc` resolves innermost-first, the rule modals already follow. Previewing
// mid-selection is legitimate, since the marked-row `●` is itself themed.
//
// The BANNER is asserted in the rendered frame WHILE THE PANEL IS OPEN: it sits in
// the notice band on the left, and the panel covers the right-hand column, so it stays
// visible behind it. (The close half's own suite covers the return in depth; it is
// re-asserted here because "unaffected" is a statement about the round trip.)
func TestPanelRouting_NestsOverMultiSelect(t *testing.T) {
	m, _ := newUnblockedEntryModel(t, PageSessions)
	m = pressPanelKey(t, m, tea.KeyPressMsg{Code: 'm', Text: "m"})
	if !m.MultiSelectActive() {
		t.Fatal("precondition: `m` did not enter multi-select")
	}
	marked := m.SelectedSessionCount()
	if marked == 0 {
		t.Fatal("precondition: entering multi-select marked nothing, so an intact set proves nothing")
	}
	first := closePanelSessions()[0].Name
	if !m.IsSessionSelected(first) {
		t.Fatalf("precondition: entering multi-select did not mark %q", first)
	}

	m, _ = pressThemeKeyCmd(t, m)

	if !m.themePanel.open {
		t.Fatal("`t` did not open the panel over multi-select")
	}
	if !m.MultiSelectActive() {
		t.Error("opening the panel exited multi-select; the panel NESTS over the mode (§9.7)")
	}
	if got := m.SelectedSessionCount(); got != marked {
		t.Errorf("the marked set holds %d session(s) with the panel open, want the %d marked before", got, marked)
	}
	if !m.IsSessionSelected(first) {
		t.Errorf("%q is no longer marked with the panel open", first)
	}
	if got := ansi.Strip(m.View().Content); !strings.Contains(got, fmt.Sprintf("%d selected", marked)) {
		t.Errorf("the panelled frame carries no `%d selected` banner; it sits in the notice band and stays visible behind the panel:\n%s", marked, got)
	}

	m = closeThemePanelForTest(t, m)

	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	if !m.MultiSelectActive() {
		t.Error("the close exited multi-select; Esc resolves innermost-first (§9.7)")
	}
	if got := m.SelectedSessionCount(); got != marked {
		t.Errorf("the marked set holds %d session(s) after the close, want %d", got, marked)
	}
}

// The former TestPanelEntry_LeavesDescriptorsUnfiltered lived here. That test owned
// the DISPATCH and the flash only, so its claim ("the blocked entry filters no
// descriptor") could not fail: both page descriptors were nullary pure functions
// returning literals, and no mutation of that task's code could touch them.
//
// The display half of the block landed with the footer revision — the `t` row dropped from the
// footer and from `?` help IN LOCKSTEP through the same call-site filter that
// already drops `m` (the NO_COLOR panel block and the footer rule). The claim is CONSTRAINING
// now, so it is folded into TestFooterRevision_StaticDescriptorsUnfiltered, which asserts the
// static descriptors stay whole in exactly the states that DO strip the call-site slices.
