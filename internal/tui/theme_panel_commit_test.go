package tui

import (
	"errors"
	"go/ast"
	"os"
	"path/filepath"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/project"
	"github.com/leeovery/portal/internal/theme"
)

// The picker idiom's `Enter`: it COMMITS A CONSTANT — `theme = <the cursor's slug>`, both
// slots cleared in one atomic prefs write — and the panel STAYS OPEN.
//
// Four decisions that read as small all bite on this one keypress, and each is a
// named test below rather than a comment:
//
//   - IT DOES NOT CLOSE. A user who had just set both slots would press `Enter` to
//     leave and thereby commit a constant, wiping the pair they just built. `Esc`
//     is the only way out — one exit key, no dual-purpose keys, and the pair flow
//     needs no special case. The accepted cost is that "pick one and go" is two
//     keys rather than one.
//   - IT COMMITS THE CURSOR'S SLUG, never the persisted one. The row-rendering rule draws that
//     split deliberately: `●` is what is SET, the cursor is what is PREVIEWED.
//   - IT DOES NOT RE-THEME. A commit is a WRITE, NOT A NAVIGATION, so the frame is
//     unchanged across the keypress and committing to a non-active slot
//     will change nothing on screen.
//   - THE IN-MEMORY MUTATION IS ON THE CONSTRUCTION-TIME SNAPSHOT, never on
//     the merged bytes the persister's read-modify-write just had in hand —
//     re-deriving from those would make the panel jump to another instance's
//     choices at the moment the user presses a key, the cross-instance sync the
//     construction-time load rule explicitly declines.
//
// Most of the file is therefore NEGATIVES, and every negative carries a positive
// control: `Enter` was inert before this task, so an assertion that nothing
// happened passes just as readily over a keypress that was never wired.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// commitEnter is the picker idiom's commit-a-constant press.
var commitEnter = tea.KeyPressMsg{Code: tea.KeyEnter}

// errThemeCommitFailed is the failure a persister raises in this file. It is a
// package-level sentinel so the identity comparison on the returned value is
// exact — the failed-commit rule's report is rendered FROM that value, so "the error came
// back" has to mean this error rather than any error.
var errThemeCommitFailed = errors.New("prefs.json: no such file or directory")

// commitPanelProjects are the rows the Projects page carries in the fixtures that
// need both pages populated — the panel is bound on both, so `Enter` is a
// statement about both.
func commitPanelProjects() []project.Project {
	return []project.Project{{Path: "/p/one", Name: "one"}, {Path: "/p/two", Name: "two"}}
}

// openCommitPanel builds a renderable model with BOTH pages populated at the given
// page, and opens the panel through the production `t` keypress.
//
// It takes the whole seam set rather than wiring a persister for itself, because
// the NIL persister is one of the states under test (a fixture / capturetool
// model) and a helper that always wired one would leave it unreachable.
func openCommitPanel(t *testing.T, deps Deps, p page, cursorSlug string) Model {
	t.Helper()

	m := Build(deps)
	projects := commitPanelProjects()
	m.setProjects(projects)
	m.projectList.SetItems(ProjectsToListItems(projects))
	m.projectList.Select(0)
	m.activePage = p

	m = openPanelForTest(t, m, arrowTermW-2*Hinset, arrowTermH-2*Vinset)
	requireCursorOn(t, m, cursorSlug)
	return m
}

// newCommitPanelModelAt is the standard commit fixture on the given page: the stub
// union, a RECORDING persister, and the panel already open with the cursor on the
// persisted constant's row.
func newCommitPanelModelAt(t *testing.T, rows []theme.Row, cursorSlug string, p page) (Model, *fakeThemePersister) {
	t.Helper()

	persister := &fakeThemePersister{}
	deps := newArrowPanelDeps(t, rows, cursorSlug)
	deps.ThemePersister = persister
	return openCommitPanel(t, deps, p, cursorSlug), persister
}

// newCommitPanelModel is the fixture on the Sessions page.
func newCommitPanelModel(t *testing.T, rows []theme.Row, cursorSlug string) (Model, *fakeThemePersister) {
	t.Helper()
	return newCommitPanelModelAt(t, rows, cursorSlug, PageSessions)
}

// commitPairPanelDeps is the ADAPTIVE PAIR fixture's seam set, WITHOUT a
// persister: the light slot on rows[0], the dark slot on rows[1], and the DARK
// one in force (the standing no-answer canvas, the appearance-gate rule) — so the cursor opens
// on the dark slot's row and there are two slots for a commit to act on.
//
// The persister is left out because the NIL one is a state under test (a
// slot commit over a capture model), exactly as openCommitPanel takes the whole
// seam set rather than wiring one for itself.
func commitPairPanelDeps(t *testing.T, rows []theme.Row) Deps {
	t.Helper()

	light, dark := rows[0], rows[1]
	return Deps{
		Lister: fakeLister{},
		Theme:  theme.ConstantNomination(dark.Theme),
		ThemeEnumerator: &fakeThemeEnumerator{
			union: theme.Union{Rows: rows, Count: len(rows)},
			resolution: theme.Resolution{
				Nomination: theme.ConstantNomination(dark.Theme),
				Slots: []theme.SlotResolution{
					{Slot: theme.SlotLight, Requested: light.Slug, Resolved: light.Slug, Theme: light.Theme},
					{Slot: theme.SlotDark, Requested: dark.Slug, Resolved: dark.Slug, Theme: dark.Theme},
				},
			},
		},
		ThemeKeys: theme.RawKeys{Light: light.Slug, Dark: dark.Slug},
	}
}

// newCommitPairPanelModel is the commit fixture under an ADAPTIVE PAIR, with a
// RECORDING persister and the panel already open on the dark slot's row.
func newCommitPairPanelModel(t *testing.T, rows []theme.Row) (Model, *fakeThemePersister) {
	t.Helper()

	persister := &fakeThemePersister{}
	deps := commitPairPanelDeps(t, rows)
	deps.ThemePersister = persister
	return openCommitPanel(t, deps, PageSessions, rows[1].Slug), persister
}

// pressCommitKey drives `Enter` through the live Update and returns the command
// alongside the model.
//
// The COMMAND is the observation a counter cannot make: a write deferred off the
// keypress would have to be scheduled as a tea.Cmd.
func pressCommitKey(t *testing.T, m Model) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(commitEnter)
	return updated.(Model), cmd
}

// requireCommitted fails unless the persister recorded exactly the given sequence
// of CONSTANT commits — CommitTheme calls, never the slot saver the picker idiom gives
// `d`/`l`.
func requireCommitted(t *testing.T, p *fakeThemePersister, want ...string) {
	t.Helper()

	if len(p.constants) != len(want) {
		t.Fatalf("the persister recorded constants %v, want %v", p.constants, want)
	}
	for i, slug := range want {
		if p.constants[i] != slug {
			t.Fatalf("commit %d recorded %q, want %q", i+1, p.constants[i], slug)
		}
	}
	if len(p.slots) != 0 {
		t.Errorf("`Enter` committed slot(s) %v; it writes the CONSTANT and clears both slots (§9.2)", p.slots)
	}
}

// requireConstantKeys fails unless the model's raw keys are the constant-or-pair rule's
// constant shape: the given slug, with BOTH slots cleared.
func requireConstantKeys(t *testing.T, m Model, slug string) {
	t.Helper()
	if got := m.themeState.keys; got != (theme.RawKeys{Theme: slug}) {
		t.Errorf("themeKeys = %+v, want {Theme:%s} — a constant clears both slots (§8.2)", got, slug)
	}
}

// arrowToThemeRow presses `↓` until the panel's cursor lands on the row labelled
// label, so a fixture moves the cursor the way a user does rather than by seeding
// an index — which is what keeps "the cursor's slug" the slug PRODUCTION put
// there.
//
// The walk is bounded by the row count, which is enough: `↓` steps one selectable
// row at a time and reverses at the end, so every reachable row is
// visited within one pass.
func arrowToThemeRow(t *testing.T, m Model, label string) Model {
	t.Helper()

	for range len(m.themePanel.list.Items()) {
		if themePanelCursorRow(t, m).Label() == label {
			return m
		}
		m = pressPanelKey(t, m, arrowDown)
	}
	t.Fatalf("arrowing never reached the %q row; rows: %v", label, themePanelRowLabels(m))
	return m
}

// TestPanelEnter_CommitsTheCursorSlug: it commits the cursor's slug, not the
// persisted one.
//
// The row-rendering rule draws the split deliberately — `●` is what is SET, the cursor is what
// is PREVIEWED — so the commit takes the row the user is looking at. The fixture
// ARROWS AWAY from the persisted row first, which is the only shape in which the
// two are distinguishable: with the cursor still on the constant's row a commit of
// the persisted slug and a commit of the cursor's are the same string.
func TestPanelEnter_CommitsTheCursorSlug(t *testing.T) {
	rows := arrowValidRows(4)
	persisted, target := rows[0].Slug, rows[2].Slug
	m, persister := newCommitPanelModel(t, rows, persisted)

	m = arrowToThemeRow(t, m, target)
	if got := m.themeState.keys.Theme; got != persisted {
		t.Fatalf("fixture: the persisted constant is %q, want %q — the two must differ for this to assert anything", got, persisted)
	}

	m, _ = pressCommitKey(t, m)

	requireCommitted(t, persister, target)
	requireConstantKeys(t, m, target)
}

// TestPanelEnter_DoesNotClose: it keeps the panel open.
//
// The picker idiom: "**`Enter` does not close.** If it did, a user who had just set both slots
// would press `Enter` to exit and thereby commit a constant, wiping the pair they
// just built." Asserted on BOTH pages, because `t` is bound on both and the
// panel owns the keyboard identically over each.
//
// The COMMIT is the control. `Enter` was swallowed before this task, so "the panel
// is still open" is true of a keypress that did nothing at all.
func TestPanelEnter_DoesNotClose(t *testing.T) {
	for _, tc := range entryPages {
		t.Run(tc.name, func(t *testing.T) {
			rows := arrowValidRows(4)
			m, persister := newCommitPanelModelAt(t, rows, rows[0].Slug, tc.page)

			m, cmd := pressCommitKey(t, m)

			requireCommitted(t, persister, rows[0].Slug)
			if !m.themePanel.open {
				t.Error("`Enter` closed the panel; `Esc` is the ONLY way out (§9.2)")
			}
			if isQuitCmd(cmd) {
				t.Error("`Enter` quit Portal from the open panel")
			}
			if m.activePage != tc.page {
				t.Errorf("`Enter` moved the active page to %d, want it left on %d", m.activePage, tc.page)
			}
		})
	}
}

// TestPanelEnter_MutatesRawKeysToAConstant: it clears both slots in memory.
//
// The constant-or-pair rule's mutual exclusion is performed ON DISK by prefs.SaveTheme in one
// atomic write; the panel MIRRORS the same rule on its own raw keys rather than
// re-implementing it or reading anything back. Driven from an ADAPTIVE PAIR, so
// there are two slots there to be cleared.
func TestPanelEnter_MutatesRawKeysToAConstant(t *testing.T) {
	rows := arrowValidRows(4)
	m, persister := newCommitPairPanelModel(t, rows)
	before := m.themeState.keys
	if before.Light == "" || before.Dark == "" {
		t.Fatalf("fixture: the persisted keys are %+v, want both slots set", before)
	}

	m, _ = pressCommitKey(t, m)

	requireCommitted(t, persister, before.Dark)
	requireConstantKeys(t, m, before.Dark)
}

// TestPanelEnter_IsAWriteNotANavigation: it does not re-theme.
//
// The picker idiom: "A commit is a **write, not a navigation** — the panel keeps previewing
// whatever the cursor is on; the display resolves from persisted state only on
// close." So the composed frame is BYTE-IDENTICAL across the keypress and the
// active theme does not move.
//
// THE FIXTURE ARROWS AWAY FIRST, which is what makes the byte comparison
// load-bearing in the direction that matters: a commit that re-resolved from
// persisted state would flip the frame back to the persisted row's palette, which
// is a visible change. The direction it CANNOT see is an ApplyTheme call with the
// cursor's own palette — that is a no-op by construction, since the cursor's row is
// already what is painted — so the structural half below covers it.
func TestPanelEnter_IsAWriteNotANavigation(t *testing.T) {
	t.Run("the frame is byte-identical across the keypress", func(t *testing.T) {
		rows := arrowValidRows(4)
		persisted, target := rows[0].Slug, rows[2].Slug
		m, persister := newCommitPanelModel(t, rows, persisted)

		m = arrowToThemeRow(t, m, target)
		previewed := m.themeState.active
		if previewed == rows[0].Theme {
			t.Fatal("fixture: arrowing previewed nothing, so an unchanged frame says nothing about the commit")
		}
		before := m.View().Content

		m, _ = pressCommitKey(t, m)

		requireCommitted(t, persister, target)
		if m.themeState.active != previewed {
			t.Errorf("the commit rendered canvas %s, want the previewed %s left alone — a commit is a write, not a navigation (§9.2)", m.themeState.active.Canvas.Value, previewed.Canvas.Value)
		}
		if got := m.View().Content; got != before {
			t.Errorf("the commit changed the composed frame\nbefore: %q\nafter:  %q", escSeq(before), escSeq(got))
		}
	})

	// The non-vacuity control on that byte comparison: an ARROW over the same
	// fixture does change the frame, so "identical" is a commit that re-themed
	// nothing rather than a frame that never moves.
	t.Run("an arrow over the same fixture does change the frame", func(t *testing.T) {
		rows := arrowValidRows(4)
		m, _ := newCommitPanelModel(t, rows, rows[0].Slug)
		before := m.View().Content

		if got := pressPanelKey(t, m, arrowDown).View().Content; got == before {
			t.Fatal("an arrow left the frame unchanged, so the byte comparison above proves nothing")
		}
	})

	// The structural half. Committing the cursor's own palette is invisible — it is
	// already what is painted — so a stray ApplyTheme on this path costs a restyle
	// per keypress and reaches no assertion. The picker idiom says the commit takes the degrade
	// POLICY without taking the apply, and this is what holds it there.
	t.Run("the commit path calls no ApplyTheme", func(t *testing.T) {
		if sites := applyThemeCallSitesIn(t, "theme_panel_commit.go"); len(sites) != 0 {
			t.Errorf("%v call Model.ApplyTheme; a commit is a WRITE, not a navigation (§9.2) — the frame must not move on this keypress", sites)
		}
	})

	// The non-vacuity control on that scan, which is a NEGATIVE: a scan looking for
	// the wrong shape passes it exactly as readily as a file that genuinely calls
	// nothing. The panel's own file DOES apply — the open, the close and the
	// arrow-preview all route through Model.ApplyTheme — so the scan must see those.
	t.Run("the scan reports the applies that are there", func(t *testing.T) {
		if sites := applyThemeCallSitesIn(t, "theme_panel.go"); len(sites) == 0 {
			t.Error("the scan found no ApplyTheme call in theme_panel.go, where the open, the close and the arrow-preview all make one; it would pass over the commit path whatever that file held")
		}
	})
}

// applyThemeCallSitesIn returns the name of every function in the named production
// file that calls ApplyTheme, at any depth within its body.
func applyThemeCallSitesIn(t *testing.T, file string) []string {
	t.Helper()

	parsed, ok := parsePackageFilesByName(t)[file]
	if !ok {
		t.Fatalf("the package holds no %s, so scanning it proves nothing", file)
	}
	var sites []string
	for _, decl := range parsed.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "ApplyTheme" {
				sites = append(sites, fn.Name.Name)
			}
			return true
		})
	}
	return sites
}

// TestPanelEnter_EscResolvesTheCommittedTheme: it makes `Esc` resolve to the new
// constant.
//
// The picker idiom: "`Esc` discards the preview and renders the resolved persisted state" —
// which equals "what you had before" ONLY when nothing was committed. This is the
// whole reason the in-memory key mutation exists: without it the close
// re-resolves the STALE keys and lands the user back on the theme they just
// replaced.
//
// It runs against a REAL loader over a REAL themes directory, because the
// stub seam answers with a fixed resolution whatever it is handed — which is
// precisely the thing this test must not fake.
func TestPanelEnter_EscResolvesTheCommittedTheme(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	writeThemeFileForTest(t, dir, "sunset.theme", "#202020")
	m, _, _ := newClosePanelModel(t, dir, theme.RawKeys{Theme: "aurora"})
	persister := &fakeThemePersister{}
	WithThemePersister(persister)(&m)

	m = pressThemeKey(t, m)
	if got := m.themeState.active.Canvas.Value; got != "#101010" {
		t.Fatalf("precondition: the open rendered canvas %s, want the persisted aurora's #101010", got)
	}
	m = arrowToThemeRow(t, m, "sunset")

	m, _ = pressCommitKey(t, m)
	requireCommitted(t, persister, "sunset")

	m = closeThemePanelForTest(t, m)

	if m.themePanel.open {
		t.Fatal("Esc left the panel open")
	}
	if got := m.themeState.active.Canvas.Value; got != "#202020" {
		t.Errorf("the close rendered canvas %s, want the newly committed sunset's #202020 — #101010 is what was persisted at OPEN, which a commit replaced", got)
	}
}

// TestPanelEnter_FailedWriteLeavesKeysAlone: it mutates nothing on a failed write.
//
// The failed-commit rule: a failed commit "does not move the `●` — the marker means 'what is
// persisted' and would be lying if it moved". The marker is derived from the raw
// keys, so the mechanism is that the keys are not touched at all.
//
// The error is RETURNED rather than swallowed, and the commit path also REPORTS
// it: the failed-commit rule's message-slot line and its outstanding-failure state are raised
// inside the commit, so a path that only logged would recreate the silent "applied but not
// persisted" state.
func TestPanelEnter_FailedWriteLeavesKeysAlone(t *testing.T) {
	rows := arrowValidRows(4)
	persisted, target := rows[0].Slug, rows[2].Slug

	t.Run("the keypress mutates nothing", func(t *testing.T) {
		m, persister := newCommitPanelModel(t, rows, persisted)
		persister.err = errThemeCommitFailed
		m = arrowToThemeRow(t, m, target)
		previewed := m.themeState.active

		m, _ = pressCommitKey(t, m)

		requireCommitted(t, persister, target)
		requireConstantKeys(t, m, persisted)
		if !m.themePanel.open {
			t.Error("a failed commit closed the panel; `Esc` is the only way out (§9.2)")
		}
		if m.themeState.active != previewed {
			t.Errorf("a failed commit rendered canvas %s, want the previewed %s — §9.13 KEEPS the theme applied in memory", m.themeState.active.Canvas.Value, previewed.Canvas.Value)
		}
	})

	t.Run("the helper returns the error", func(t *testing.T) {
		m, persister := newCommitPanelModel(t, rows, persisted)
		persister.err = errThemeCommitFailed

		if err := (&m).commitSelectedConstant(); !errors.Is(err, errThemeCommitFailed) {
			t.Errorf("commitSelectedConstant returned %v, want the persister's error — the caller reads the outcome from it", err)
		}
	})
}

// TestPanelEnter_NilPersisterIsInert: it tolerates a nil persister.
//
// A fixture or `capturetool` model carries NO persister, so a commit
// during a capture writes nowhere. It is the ABSENCE OF A WRITER rather than a
// failed write — it raises no message and no outstanding-failure state
// — which is why it mutates nothing either: there is nothing on disk for the
// in-memory keys to mirror.
func TestPanelEnter_NilPersisterIsInert(t *testing.T) {
	rows := arrowValidRows(4)
	persisted, target := rows[0].Slug, rows[2].Slug

	m := openCommitPanel(t, newArrowPanelDeps(t, rows, persisted), PageSessions, persisted)
	if m.themeState.persister != nil {
		t.Fatalf("fixture: the model holds persister %#v, want none", m.themeState.persister)
	}
	m = arrowToThemeRow(t, m, target)
	before := m.View().Content

	m, cmd := pressCommitKey(t, m)

	requireConstantKeys(t, m, persisted)
	if !m.themePanel.open {
		t.Error("`Enter` closed the panel over a nil persister")
	}
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("a nil persister raised the message %+v; it is the absence of a WRITER, not a failed write", got)
	}
	if cmd != nil {
		t.Errorf("`Enter` over a nil persister scheduled %T, want nothing", cmd)
	}
	if got := m.View().Content; got != before {
		t.Errorf("`Enter` over a nil persister changed the frame\nbefore: %q\nafter:  %q", escSeq(before), escSeq(got))
	}

	// Positive control: the SAME fixture with a persister wired does mutate the
	// keys, so the untouched keys above are the nil seam rather than a dead arm.
	wired, persister := newCommitPanelModel(t, rows, persisted)
	wired, _ = pressCommitKey(t, arrowToThemeRow(t, wired, target))
	requireCommitted(t, persister, target)
	requireConstantKeys(t, wired, target)
}

// TestPanelEnter_RepeatCommitIsIdempotent: it is idempotent.
//
// The failed-commit rule: "A commit is always re-attemptable. The commit keys are
// unconditional writes, so pressing `d`/`l`/`Enter` again simply retries — no special
// retry affordance, and no state to clear first."
func TestPanelEnter_RepeatCommitIsIdempotent(t *testing.T) {
	rows := arrowValidRows(4)
	target := rows[2].Slug
	m, persister := newCommitPanelModel(t, rows, rows[0].Slug)
	m = arrowToThemeRow(t, m, target)

	m, _ = pressCommitKey(t, m)
	once := m.View().Content
	onceKeys := m.themeState.keys

	m, cmd := pressCommitKey(t, m)

	requireCommitted(t, persister, target, target)
	if m.themeState.keys != onceKeys {
		t.Errorf("the second commit left keys %+v, want the first's %+v", m.themeState.keys, onceKeys)
	}
	requireConstantKeys(t, m, target)
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("the repeat commit raised the message %+v; there is no retry affordance and no state to clear first (§9.13)", got)
	}
	if cmd != nil {
		t.Errorf("the repeat commit scheduled %T, want nothing", cmd)
	}
	if got := m.View().Content; got != once {
		t.Errorf("the repeat commit changed the frame\nonce:  %q\ntwice: %q", escSeq(once), escSeq(got))
	}
}

// TestPanelEnter_NoOtherIO: it reads nothing and writes nothing else.
//
// The prefs call is the ONE write this keypress may make. Everything else is
// asserted absent: no directory read and no fresh enumeration (a commit
// changes prefs, not the directory, and a read here would produce a third parse of
// the same slug that can disagree with the row the user is looking at), no other
// file-touching seam, and no deferred write riding a tea.Cmd.
//
// internal/tui resolves no config path and reads no PORTAL_* env var, so every
// route from this package to disk runs through one of the counted seams; the
// package's tmux writers are neither wired to the panel nor reachable from it.
func TestPanelEnter_NoOtherIO(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("PORTAL_PREFS_FILE", filepath.Join(configDir, "prefs.json"))
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")

	stores := newCountingStores()
	// The theme persister is deliberately the RECORDING fake rather than the
	// counted one: it is the single write `Enter` is allowed to make, so the
	// counted set covers everything else and this records what the one write was.
	persister := &fakeThemePersister{}
	loader := theme.NewLoader(nil)
	enumerator := countingEnumeratorOver(loader, dir)
	keys := theme.RawKeys{Theme: "sunset"}
	setting, _ := theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)
	resolution, err := loader.ResolveNomination(setting, dir)
	if err != nil {
		t.Fatalf("construction-time resolution of %+v: %v", setting, err)
	}

	m := Build(Deps{
		Lister:          stores.lister,
		Theme:           resolution.Nomination,
		ProjectStore:    stores.projectStore,
		ProjectEditor:   stores.projectEditor,
		AliasEditor:     stores.aliasEditor,
		ModePersister:   stores.modePersister,
		Reader:          stores.scrollback,
		ThemeEnumerator: enumerator,
		ThemeKeys:       keys,
		ThemePersister:  persister,
	})
	m.termWidth, m.termHeight = arrowTermW, arrowTermH
	m.applySessions(closePanelSessions())
	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("fixture: the panel did not open")
	}
	// The OPEN's own read is not what this measures — the re-read-on-open rule pins one directory
	// read per open, and the question here is what the COMMIT adds to it.
	stores.reset()
	opens := enumerator.opens

	m, cmd := pressCommitKey(t, m)

	requireCommitted(t, persister, "sunset")
	if got := stores.calls(); got != 0 {
		t.Errorf("the commit made %d file-touching seam call(s) — the prefs write is the only one (project store %d, project editor %d, alias editor %d, mode persister %d, theme persister %d, scrollback %d, lister %d)",
			got, stores.projectStore.calls, stores.projectEditor.calls, stores.aliasEditor.calls,
			stores.modePersister.calls, stores.themePersister.calls, stores.scrollback.calls, stores.lister.calls)
	}
	if enumerator.opens != opens {
		t.Errorf("the commit ran %d enumerations in total, want the open's %d — a commit re-derives from the retained parse and never re-reads the directory (§8.4)", enumerator.opens, opens)
	}
	if cmd != nil {
		t.Errorf("the commit scheduled %T; a deferred write is the one shape the counters above cannot see", cmd)
	}
	if entries, err := os.ReadDir(configDir); err != nil || len(entries) != 0 {
		t.Errorf("the commit left %d entries in the config directory (err %v), want none — the model writes through the seam and touches no path of its own", len(entries), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	if len(entries) != 1 || entries[0].Name() != "sunset.theme" {
		t.Errorf("the themes directory holds %d entries after a commit, want only the seeded drop-in", len(entries))
	}
}

// TestPanelEnter_UnselectableRowWritesNothing: it refuses a non-selectable row.
//
// STRUCTURALLY UNREACHABLE, so the guard is DEFENSIVE: the arrows skip
// unselectable rows and the open-time anchor lands on a selectable one.
// The cursor is therefore placed DIRECTLY here, bypassing both.
//
// The empty-slug arm is the same kind of guard one step further out — a selectable
// row with no slug is a shape the real union assembly cannot produce, and writing
// its empty string as a theme name would persist a setting nothing can resolve.
func TestPanelEnter_UnselectableRowWritesNothing(t *testing.T) {
	t.Run("an unselectable row", func(t *testing.T) {
		rows := []theme.Row{arrowValidRow(arrowSlug(0), 0), arrowInvalidRow(arrowSlug(1))}
		m, persister := newCommitPanelModel(t, rows, rows[0].Slug)
		m.themePanel.list.Select(1)
		if themePanelCursorRow(t, m).Selectable() {
			t.Fatal("fixture: the cursor is on a selectable row, so there is no refusal to exercise")
		}

		m, cmd := pressCommitKey(t, m)

		requireCommitted(t, persister)
		requireConstantKeys(t, m, rows[0].Slug)
		if !m.themePanel.open {
			t.Error("`Enter` on an unselectable row closed the panel")
		}
		if cmd != nil {
			t.Errorf("`Enter` on an unselectable row scheduled %T, want nothing", cmd)
		}
	})

	t.Run("a selectable row carrying no slug", func(t *testing.T) {
		rows := []theme.Row{arrowValidRow(arrowSlug(0), 0), {Source: theme.SourceBuiltin, Theme: arrowPalette(1)}}
		m, persister := newCommitPanelModel(t, rows, rows[0].Slug)
		m.themePanel.list.Select(1)
		row := themePanelCursorRow(t, m)
		if !row.Selectable() || row.Slug != "" {
			t.Fatalf("fixture: the cursor is on %+v, want a SELECTABLE row with no slug", row)
		}

		m, _ = pressCommitKey(t, m)

		requireCommitted(t, persister)
		requireConstantKeys(t, m, rows[0].Slug)
	})

	// Positive control: the same keypress on a SELECTABLE row does write, so the
	// two refusals above are a guard rather than an unwired arm.
	t.Run("a selectable row does write", func(t *testing.T) {
		rows := []theme.Row{arrowValidRow(arrowSlug(0), 0), arrowInvalidRow(arrowSlug(1))}
		m, persister := newCommitPanelModel(t, rows, rows[0].Slug)

		m, _ = pressCommitKey(t, m)

		requireCommitted(t, persister, rows[0].Slug)
		requireConstantKeys(t, m, rows[0].Slug)
	})
}

// TestPanelEnter_NoConfirmOverAPair: it raises no confirm over a pair.
//
// The picker idiom: "**The reverse direction needs no confirm.** `Enter` on a theme while a
// pair is set clears both slots — but `Enter` visibly does what it says: you get
// the theme you are looking at, and it is the theme already previewing behind the
// panel." The asymmetry with `d`/`l` is the point — the confirm guards
// the case where the RESOLVED theme changes as a side effect of a write the user
// was told is inert.
//
// So the write lands on the SAME keypress, with nothing in the panel layout's message slot and
// the panel's standing footer untouched.
func TestPanelEnter_NoConfirmOverAPair(t *testing.T) {
	for _, tc := range []struct {
		name  string
		build func(*testing.T, []theme.Row) (Model, *fakeThemePersister)
		want  func([]theme.Row) string
	}{
		{
			name:  "adaptive pair",
			build: newCommitPairPanelModel,
			want:  func(rows []theme.Row) string { return rows[1].Slug },
		},
		{
			name: "constant",
			build: func(t *testing.T, rows []theme.Row) (Model, *fakeThemePersister) {
				return newCommitPanelModel(t, rows, rows[0].Slug)
			},
			want: func(rows []theme.Row) string { return rows[0].Slug },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := arrowValidRows(4)
			m, persister := tc.build(t, rows)
			footer := renderThemePanelFooter(themePanelKeymap(), themePanelInnerWidth(m.themePanel.width), m.themeState.active, m.colourless)

			m, cmd := pressCommitKey(t, m)

			requireCommitted(t, persister, tc.want(rows))
			if got := m.themePanel.message; got.Kind != themeMessageNone {
				t.Errorf("`Enter` raised the message %+v; the reverse direction needs no confirm (§9.2)", got)
			}
			if cmd != nil {
				t.Errorf("`Enter` scheduled %T; the write lands on this keypress rather than awaiting an answer", cmd)
			}
			after := renderThemePanelFooter(themePanelKeymap(), themePanelInnerWidth(m.themePanel.width), m.themeState.active, m.colourless)
			if after != footer {
				t.Errorf("`Enter` swapped the panel footer\nbefore: %q\nafter:  %q", escSeq(footer), escSeq(after))
			}
		})
	}
}
