package tui

import (
	"reflect"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// The picker idiom's opening state: the cursor lands on the theme that is ACTUALLY
// RENDERING, and opening previews nothing.
//
// The open is not a passive read. The re-read-on-open rule makes the panel's fresh parse
// supersede the construction-time one, so opening applies a mid-session file edit — in both
// directions: an edit that changes an active theme's values re-renders the same
// slug with them, and one that INVALIDATES it flips to the per-slot fallback rule fallback on
// OPEN rather than deferring to `Esc`, which would leave the panel listing a theme as
// invalid while the screen still renders it.
//
// The invariant surviving every case: the cursor is on a SELECTABLE row, and that
// row is what is painted behind the panel.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// themeCursorModel is the dir-backed panel model for the cursor fixtures, which
// need nothing off the enumerator itself.
func themeCursorModel(t *testing.T, dir string, keys theme.RawKeys, mode theme.Member) Model {
	t.Helper()
	m, _ := newDirBackedPanelModel(t, dir, keys, mode)
	return m
}

// themePanelCursorRow returns the union row the panel's cursor is on, failing
// when the cursor is on nothing at all.
func themePanelCursorRow(t *testing.T, m Model) theme.Row {
	t.Helper()

	item := m.themePanel.list.SelectedItem()
	if item == nil {
		t.Fatalf("the panel cursor (index %d) is on no row at all; rows: %v", m.themePanel.list.Index(), themePanelRowLabels(m))
	}
	row, ok := item.(themeRowItem)
	if !ok {
		t.Fatalf("panel item %#v is not a themeRowItem", item)
	}
	return row.Row
}

// requireCursorOn fails unless the panel's cursor is on the row labelled label.
func requireCursorOn(t *testing.T, m Model, label string) {
	t.Helper()

	if got := themePanelCursorRow(t, m).Label(); got != label {
		t.Errorf("the cursor landed on %q, want %q — the cursor's row is always what is painted behind the panel (§9.2); rows: %v", got, label, themePanelRowLabels(m))
	}
}

// requireBadge fails unless the row labelled label carries want.
func requireBadge(t *testing.T, m Model, label string, want theme.Badge) {
	t.Helper()

	if got := themePanelRowFor(t, m, label).Badge; got != want {
		t.Errorf("the %q row's badge = %v, want %v — the `●` marks what is SET (§9.5)", label, got, want)
	}
}

// TestPanelOpenCursor_Constant: it lands on the constant's row.
//
// The degenerate case, and the one that fixes the reference point for every other
// one: with a single slug there is exactly one row the screen can be painted from.
func TestPanelOpenCursor_Constant(t *testing.T) {
	m := themeCursorModel(t, t.TempDir(), theme.RawKeys{Theme: "nord"}, theme.MemberDark)

	m = pressThemeKey(t, m)

	requireCursorOn(t, m, "nord")
	requireBadge(t, m, "nord", theme.BadgeConstant)
}

// TestPanelOpenCursor_InForceSlot: it lands on the in-force slot's row.
//
// Under a pair only ONE slot is painting the screen — the light one in a light
// terminal, the dark one otherwise — so only that slot's row may carry the
// cursor. The other slot's row still carries its badge: `●` is what is SET and
// the cursor is what is PREVIEWED, and only the cursor is singular.
func TestPanelOpenCursor_InForceSlot(t *testing.T) {
	keys := theme.RawKeys{Light: theme.DefaultLightSlug, Dark: "nord"}

	for _, tc := range []struct {
		name       string
		mode       theme.Member
		wantCursor string
		wantOther  string
		otherBadge theme.Badge
	}{
		{
			name:       "a light terminal previews the light slot",
			mode:       theme.MemberLight,
			wantCursor: theme.DefaultLightSlug,
			wantOther:  "nord",
			otherBadge: theme.BadgeDark,
		},
		{
			name:       "a dark terminal previews the dark slot",
			mode:       theme.MemberDark,
			wantCursor: "nord",
			wantOther:  theme.DefaultLightSlug,
			otherBadge: theme.BadgeLight,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m := themeCursorModel(t, t.TempDir(), keys, tc.mode)

			m = pressThemeKey(t, m)

			requireCursorOn(t, m, tc.wantCursor)
			requireBadge(t, m, tc.wantOther, tc.otherBadge)
		})
	}
}

// TestPanelOpenCursor_BothSlotsSameSlug: it lands on the `● both` row.
//
// Two slots naming one slug collapse to ONE row, so there is no second row
// for the cursor to be tempted onto — and the collapsed badge must survive the
// resolution rather than being one slot's badge overwriting the other's.
func TestPanelOpenCursor_BothSlotsSameSlug(t *testing.T) {
	keys := theme.RawKeys{Light: "nord", Dark: "nord"}

	for _, mode := range []theme.Member{theme.MemberLight, theme.MemberDark} {
		m := themeCursorModel(t, t.TempDir(), keys, mode)

		m = pressThemeKey(t, m)

		requireCursorOn(t, m, "nord")
		requireBadge(t, m, "nord", theme.BadgeBoth)
		if got := themePanelRowLabels(m); countLabel(got, "nord") != 1 {
			t.Errorf("the union lists %q %d times, want exactly 1 — one slug is one row (§9.4); rows: %v", "nord", countLabel(got, "nord"), got)
		}
	}
}

// countLabel counts how many of labels equal want.
func countLabel(labels []string, want string) int {
	n := 0
	for _, label := range labels {
		if label == want {
			n++
		}
	}
	return n
}

// TestPanelOpenCursor_FallbackRow: it lands on the fallback's row, not the broken
// one.
//
// Parking the cursor on the persisted-but-broken row would put it somewhere
// navigation cannot return to — arrows skip unselectable rows — and it would show
// a row that is emphatically not what is on screen.
func TestPanelOpenCursor_FallbackRow(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "not-a-colour")
	m := themeCursorModel(t, dir, theme.RawKeys{Theme: "sunset"}, theme.MemberDark)

	m = pressThemeKey(t, m)

	requireCursorOn(t, m, theme.DefaultDarkSlug)
	broken := themePanelRowFor(t, m, "sunset")
	if broken.Row.Selectable() {
		t.Errorf("the persisted row is selectable; want it present and unselectable with its reason")
	}
	if broken.Row.Rejection == nil || broken.Row.Rejection.Reason != theme.ReasonBadColour {
		t.Errorf("the persisted row's rejection = %v, want %q", broken.Row.Rejection, theme.ReasonBadColour)
	}
	requireBadge(t, m, "sunset", theme.BadgeConstant)
	requireBadge(t, m, theme.DefaultDarkSlug, theme.BadgeNone)
}

// TestPanelOpenCursor_BadgeStaysOnPersisted: it leaves the badge on the persisted
// slug.
//
// The pair's half of the same split, on a slug that resolves to NOTHING at all —
// so the badge's only home is the row the union rule mints for it. A badge keyed on what
// LOADED would sit on the fallback and silently claim it was the user's choice.
func TestPanelOpenCursor_BadgeStaysOnPersisted(t *testing.T) {
	keys := theme.RawKeys{Light: "gone-light", Dark: "nord"}
	m := themeCursorModel(t, t.TempDir(), keys, theme.MemberLight)

	m = pressThemeKey(t, m)

	requireCursorOn(t, m, theme.DefaultLightSlug)
	requireBadge(t, m, "gone-light", theme.BadgeLight)
	requireBadge(t, m, "nord", theme.BadgeDark)
	requireBadge(t, m, theme.DefaultLightSlug, theme.BadgeNone)
}

// TestPanelOpen_DoesNotChangeTheRenderedTheme: it changes nothing when the
// directory is unchanged.
//
// Because the cursor starts on what is already rendering, opening the panel never
// changes WHICH theme is shown — the mixed-mode flash fires only on deliberate
// navigation.
func TestPanelOpen_DoesNotChangeTheRenderedTheme(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	m := themeCursorModel(t, dir, theme.RawKeys{Theme: "sunset"}, theme.MemberDark)
	before := m.themeState.active

	m = pressThemeKey(t, m)

	if m.themeState.active != before {
		t.Errorf("opening re-painted the screen: activeTheme canvas = %s, want the unchanged %s", m.themeState.active.Canvas.Value, before.Canvas.Value)
	}
	if got := m.themeRowDelegate().Theme; got != before {
		t.Errorf("the panel's rows are drawn at canvas %s, want the rendered %s", got.Canvas.Value, before.Canvas.Value)
	}
}

// TestPanelOpen_AppliesMidSessionEdit: it applies an edited-but-valid active
// theme on open.
//
// Opening CAN change the active theme's values, and that is correct: the re-read-on-open
// rule's fresh enumeration supersedes the construction-time parse, so the panel holds the
// truth. Making the user arrow away and back to see their own edit would be a bug
// wearing a rule's clothing.
func TestPanelOpen_AppliesMidSessionEdit(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	m := themeCursorModel(t, dir, theme.RawKeys{Theme: "sunset"}, theme.MemberDark)
	if got := m.themeState.active.Canvas.Value; got != "#101010" {
		t.Fatalf("precondition: the launch rendered canvas %s, want the drop-in's #101010", got)
	}

	writeThemeFileForTest(t, dir, "sunset.theme", "#202020")
	m = pressThemeKey(t, m)

	if got := m.themeState.active.Canvas.Value; got != "#202020" {
		t.Errorf("after the edit the screen renders canvas %s, want the edited #202020 — with no arrowing required", got)
	}
	requireCursorOn(t, m, "sunset")
}

// TestPanelOpen_InvalidatedActiveThemeFlipsOnOpen: it flips to the fallback on
// open when the active theme is broken.
//
// The flip happens on OPEN, never deferred to `Esc` — deferring would leave the
// panel listing a theme as invalid while the screen still renders it.
func TestPanelOpen_InvalidatedActiveThemeFlipsOnOpen(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")
	m := themeCursorModel(t, dir, theme.RawKeys{Theme: "sunset"}, theme.MemberDark)
	if got := m.themeState.active.Canvas.Value; got != "#101010" {
		t.Fatalf("precondition: the launch rendered canvas %s, want the drop-in's #101010", got)
	}

	writeThemeFileForTest(t, dir, "sunset.theme", "not-a-colour")
	m = pressThemeKey(t, m)

	if want := testDarkTheme(t); m.themeState.active != want {
		t.Errorf("the screen renders canvas %s, want the §8.5 fallback's %s — the flip happens on open", m.themeState.active.Canvas.Value, want.Canvas.Value)
	}
	requireCursorOn(t, m, theme.DefaultDarkSlug)
	broken := themePanelRowFor(t, m, "sunset")
	if broken.Row.Selectable() {
		t.Errorf("the persisted row is selectable; want it unselectable and reasoned")
	}
	requireBadge(t, m, "sunset", theme.BadgeConstant)
}

// TestPanelOpen_RepairedThemeAppliesOnOpen: it applies a repaired theme on open
// with no relaunch.
//
// The re-read-on-open rule's MIRROR CASE, and the payoff re-reading exists to buy. It is the
// only case that moves the cursor ONTO a row that was unselectable at construction, so an
// implementation trusting the construction-time classification of an
// already-broken slug would satisfy every other case here while leaving the user
// on the fallback after they fixed their file.
func TestPanelOpen_RepairedThemeAppliesOnOpen(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "not-a-colour")
	m := themeCursorModel(t, dir, theme.RawKeys{Theme: "sunset"}, theme.MemberDark)
	if want := testDarkTheme(t); m.themeState.active != want {
		t.Fatalf("precondition: the launch rendered canvas %s, want the fallback's %s", m.themeState.active.Canvas.Value, want.Canvas.Value)
	}

	writeThemeFileForTest(t, dir, "sunset.theme", "#303030")
	m = pressThemeKey(t, m)

	if got := m.themeState.active.Canvas.Value; got != "#303030" {
		t.Errorf("the screen renders canvas %s, want the repaired drop-in's #303030 — fixing the file takes effect on the next open, with no relaunch", got)
	}
	requireCursorOn(t, m, "sunset")
	if repaired := themePanelRowFor(t, m, "sunset"); !repaired.Row.Selectable() {
		t.Errorf("the repaired row is still rejected (%v); the cursor may only sit on a selectable row", repaired.Row.Rejection)
	}
	requireBadge(t, m, "sunset", theme.BadgeConstant)
	requireBadge(t, m, theme.DefaultDarkSlug, theme.BadgeNone)
}

// TestPanelOpenCursor_AnchoredByIdentity: it anchors the cursor by identity, not
// index.
//
// Anchoring to an index would silently break the picker idiom's invariant the moment a row is
// inserted above the cursor — which is exactly what the commit recompute
// does. The two fixtures resolve the IDENTICAL setting and differ only in a row
// standing above the target, so an index-anchored cursor cannot land on the target
// in both.
func TestPanelOpenCursor_AnchoredByIdentity(t *testing.T) {
	target := theme.Row{Slug: "nord", Source: theme.SourceBuiltin, Theme: themetest.Builtin(t, "nord")}
	above := theme.Row{Slug: "aurora", Source: theme.SourceFile, Filename: "aurora.theme", Theme: testDarkTheme(t)}
	resolution := constantResolution("nord", target.Theme)

	for _, tc := range []struct {
		name      string
		rows      []theme.Row
		wantIndex int
	}{
		{name: "the target leads the list", rows: []theme.Row{target}, wantIndex: 0},
		{name: "a row is inserted above the target", rows: []theme.Row{above, target}, wantIndex: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			enumerator := &fakeThemeSource{
				union:      themeRowsUnion(tc.rows),
				resolution: resolution,
			}
			m := New(fakeLister{}, WithThemeSource(enumerator), WithThemeKeys(theme.RawKeys{Theme: "nord"}))

			m = pressThemeKey(t, m)

			if got := m.themePanel.list.Index(); got != tc.wantIndex {
				t.Errorf("the cursor landed at index %d, want %d", got, tc.wantIndex)
			}
			requireCursorOn(t, m, "nord")
		})
	}
}

// TestPanelOpenCursor_DegradesOnMissingIdentity: it degrades rather than indexing
// out of range.
//
// Built-ins are always valid, so a union with no row for the resolved slug
// — and one with no selectable row at all — are unreachable in a correctly built
// binary. The clamp is a structural guard, not a live path, which is why it is
// driven through a stub: nothing else can produce the state.
func TestPanelOpenCursor_DegradesOnMissingIdentity(t *testing.T) {
	ghost := constantResolution("ghost", testDarkTheme(t))

	t.Run("it clamps to the first selectable row", func(t *testing.T) {
		rows := []theme.Row{
			{Slug: "broken", Filename: "broken.theme", Source: theme.SourceFile, Rejection: &theme.Rejection{Reason: theme.ReasonBadColour}},
			{Slug: "nord", Source: theme.SourceBuiltin, Theme: themetest.Builtin(t, "nord")},
		}
		enumerator := &fakeThemeSource{union: themeRowsUnion(rows), resolution: ghost}
		m := New(fakeLister{}, WithThemeSource(enumerator), WithThemeKeys(theme.RawKeys{Theme: "ghost"}))

		m = pressThemeKey(t, m)

		if got := m.themePanel.list.Index(); got != 1 {
			t.Errorf("the cursor landed at index %d, want 1 — the first SELECTABLE row", got)
		}
		if row := themePanelCursorRow(t, m); !row.Selectable() {
			t.Errorf("the cursor landed on the unselectable %q; arrows cannot return to it", row.Label())
		}
	})

	t.Run("an empty union leaves the cursor at index 0", func(t *testing.T) {
		enumerator := &fakeThemeSource{union: theme.Union{}, resolution: ghost}
		m := New(fakeLister{}, WithThemeSource(enumerator), WithThemeKeys(theme.RawKeys{Theme: "ghost"}))

		m = pressThemeKey(t, m)

		if got := m.themePanel.list.Index(); got != 0 {
			t.Errorf("the cursor landed at index %d over an empty union, want 0", got)
		}
	})
}

// TestPanelOpen_ResolveErrorDegrades: it degrades rather than escalating an
// unresolvable fallback.
//
// The build-time guarantee's fatal is reachable here only from a binary whose embedded set
// cannot supply a fallback, which the build-time guarantee makes impossible — and a
// SETTINGS SURFACE MUST NOT BECOME THE ROUTE BY WHICH A BROKEN BINARY QUITS
// PORTAL MID-SESSION. The build-time guarantee puts the fatal on the startup path
// deliberately.
//
// ONE POLICY GOVERNS EVERY PANEL CALL SITE of `Resolve` — this open, `Esc`'s
// close and the recompute — so none of the three invents its own. It is
// stated once on applyInForceTheme, whose body the open and the close route
// through; the recompute takes the policy WITHOUT taking the apply, because a
// commit recomputes rows and badges and never the rendered theme.
//
// The fake returns a FULLY POPULATED resolution alongside the error, so the
// assertions are not vacuous: an open that ignored the error would badge `nord`,
// repaint the screen in its palette and move the cursor onto its row, and every
// one of those is asserted against.
func TestPanelOpen_ResolveErrorDegrades(t *testing.T) {
	nord := themetest.Builtin(t, "nord")
	rows := []theme.Row{{Slug: "nord", Source: theme.SourceBuiltin, Theme: nord}}
	enumerator := &fakeThemeSource{
		union:      themeRowsUnion(rows),
		resolution: constantResolution("nord", nord),
		err:        theme.BrokenBuiltinError(theme.DefaultDarkSlug),
	}
	m := New(fakeLister{}, WithThemeSource(enumerator), WithThemeKeys(theme.RawKeys{Theme: "nord"}))
	before := m.themeState.active

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	m = updated.(Model)

	if isQuitCmd(cmd) {
		t.Error("a failed resolution quit Portal; the panel degrades rather than escalating")
	}
	if !m.themePanel.open {
		t.Fatal("a failed resolution left the panel closed; it opens on the union already in hand")
	}
	if got := len(m.themePanel.list.Items()); got != len(rows) {
		t.Errorf("the panel lists %d rows, want the enumeration's %d", got, len(rows))
	}
	if len(m.themePanel.badges) != 0 {
		t.Errorf("badges = %v, want none — a failed resolution leaves them exactly as they were", m.themePanel.badges)
	}
	if m.themeState.active != before {
		t.Errorf("the active theme moved to canvas %s, want the unchanged %s", m.themeState.active.Canvas.Value, before.Canvas.Value)
	}
	if got := m.themePanel.list.Index(); got != 0 {
		t.Errorf("the cursor moved to index %d, want it left where it was (0)", got)
	}
}

// TestPanelOpen_CursorInvariant: it keeps the cursor on a selectable row that is
// what is painted.
//
// The picker idiom's invariant, asserted DIRECTLY rather than as an implication of the four
// opening cases: each of them could satisfy its own assertion while breaking this
// one, and everything the panel does afterwards rests on it.
func TestPanelOpen_CursorInvariant(t *testing.T) {
	for _, tc := range []struct {
		name  string
		write func(*testing.T, string)
		keys  theme.RawKeys
		mode  theme.Member
	}{
		{
			name: "a constant",
			keys: theme.RawKeys{Theme: "nord"},
			mode: theme.MemberDark,
		},
		{
			name: "the in-force slot of a pair",
			keys: theme.RawKeys{Light: theme.DefaultLightSlug, Dark: "nord"},
			mode: theme.MemberLight,
		},
		{
			name: "both slots on one slug",
			keys: theme.RawKeys{Light: "nord", Dark: "nord"},
			mode: theme.MemberDark,
		},
		{
			name:  "a fallback",
			write: func(t *testing.T, dir string) { writeThemeFileForTest(t, dir, "sunset.theme", "not-a-colour") },
			keys:  theme.RawKeys{Theme: "sunset"},
			mode:  theme.MemberDark,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if tc.write != nil {
				tc.write(t, dir)
			}
			m := themeCursorModel(t, dir, tc.keys, tc.mode)

			m = pressThemeKey(t, m)

			row := themePanelCursorRow(t, m)
			if !row.Selectable() {
				t.Errorf("the cursor is on the unselectable %q (%v)", row.Label(), row.Rejection)
			}
			if row.Theme != m.themeState.active {
				t.Errorf("the cursor's row %q carries canvas %s while the screen renders %s — the cursor's row IS what is painted behind the panel", row.Label(), row.Theme.Canvas.Value, m.themeState.active.Canvas.Value)
			}
		})
	}
}

// TestPanelOpen_NoNewOSC11Query: it issues no new detection query.
//
// The in-force answer comes from the gate's SINGLE resolution with its
// standing dark no-answer fallback. Re-querying would reopen the race the
// resolve-once rule closes, a second after the user is already reading the picker.
func TestPanelOpen_NoNewOSC11Query(t *testing.T) {
	m := themeCursorModel(t, t.TempDir(), theme.RawKeys{Light: theme.DefaultLightSlug, Dark: "nord"}, theme.MemberLight)

	updated, cmd := m.Update(tea.KeyPressMsg{Code: 't', Text: "t"})
	m = updated.(Model)

	queryType := reflect.TypeOf(tea.Cmd(tea.RequestBackgroundColor)())
	// Non-vacuity: Init issues the query unconditionally (restore-on-exit
	// needs the reply whatever the setting's shape), so the scan below is looking
	// for something it can demonstrably see.
	assertBackgroundQueryIssued(t, m)
	for _, msg := range initCmds(t, cmd) {
		if reflect.TypeOf(msg) == queryType {
			t.Errorf("opening the panel issued a %v; the gate resolves exactly once (§8.8)", queryType)
		}
		if _, ok := msg.(appearanceTimeoutMsg); ok {
			t.Error("opening the panel armed a detection timeout; there is no question left to ask")
		}
	}
	if !m.modeResolved() {
		t.Error("opening the panel reopened the first-paint gate")
	}
	if m.themeState.inForceMode() != theme.MemberLight {
		t.Errorf("inForceMode() = %v after opening, want the gate's own answer %v", m.themeState.inForceMode(), theme.MemberLight)
	}
}

// TestPanelOpen_WritesNothing: it writes nothing.
//
// Opening is a READ PLUS A RESTYLE: no prefs write, no `@portal-*` option, no
// file. The rejection-surface split's rule that falling back must never overwrite the
// persisted theme name is exactly what a write here would break, on the surface where the user
// is least able to tell it happened.
func TestPanelOpen_WritesNothing(t *testing.T) {
	requireNoPrefsOrThemesWrite(t, panelReadOnlyPath{
		verb:          "opening",
		absentSubtest: "an absent prefs.json stays absent and the directory is untouched",
	}, func(t *testing.T, dir string, keys theme.RawKeys) {
		pressThemeKey(t, themeCursorModel(t, dir, keys, theme.MemberDark))
	})
}

// TestDeps_HasNoThemeSlots pins the retirement the re-resolution above performs:
// the injected per-slot record is GONE, not left beside it.
//
// Badges exist only while the panel is open, and the panel only opens through the
// sequence above — so `Resolve` is the sole badge source from this point, and a
// surviving `Deps` field would be a second, staler one. It is also the one a
// fixture author would reach for, which is why the exported option is guarded by
// source: an unreferenced exported function would compile forever without one.
func TestDeps_HasNoThemeSlots(t *testing.T) {
	depsType := reflect.TypeFor[Deps]()
	if _, found := depsType.FieldByName("ThemeSlots"); found {
		t.Errorf("Deps still carries a ThemeSlots field; the panel derives its badges from the seam's Resolve, and a dead injection is a second source of truth for which slug carries the `●`")
	}
	if _, found := depsType.FieldByName("ThemeKeys"); !found {
		t.Errorf("Deps carries no ThemeKeys field; the raw persisted keys are unaffected by the retirement and stay (§8.4)")
	}

	for _, name := range exportedFuncsInPackage(t) {
		if name == "WithThemeSlots" {
			t.Errorf("WithThemeSlots is still declared; the option is removed rather than left as a second, dead injection path")
		}
	}
}

// TestPanelOpenCursor_CaptureSeedSkipsAnUnselectableRow: it refuses to seed the
// cursor onto a rejected row.
//
// The harness contract's fourth fixture input re-anchors the cursor BY IDENTITY, from a string
// a fixture declares — and the harness contract mandates fixtures built from invalid drop-ins
// and an unreadable themes directory, whose rows are precisely the unselectable ones. The
// picker idiom's invariant is that the cursor is always on a selectable row, and the arrows
// SKIP unselectable ones, so a cursor parked on one would sit somewhere the user
// cannot navigate back to.
//
// The seed therefore falls through to the first selectable row, which is the same
// degrade a seed naming no row at all already takes. It is asserted here rather than
// left to the capture harness because internal/capture renders stills: a still with a
// mis-seeded cursor looks like a still with a correct one.
func TestPanelOpenCursor_CaptureSeedSkipsAnUnselectableRow(t *testing.T) {
	valid := arrowValidRow(t, "aurora", 0)
	broken := arrowInvalidRow("half-written")
	rows := []theme.Row{valid, broken}

	deps := newArrowPanelDeps(t, rows, valid.Slug)
	deps.Capture.ThemeCursor = broken.Identity()
	m := Build(deps)
	m.termWidth, m.termHeight = geometryTerm(geometryWideW, geometryContentH)

	if broken.Selectable() {
		t.Fatal("fixture: the seeded row is selectable, so the skip is not exercised")
	}
	if themePanelRowIndex(rows, broken.Identity()) == 1 {
		t.Fatalf("fixture: the seeded identity resolves to index 1, the rejected row itself")
	}

	m = pressThemeKey(t, m)

	row := themePanelCursorRow(t, m)
	if !row.Selectable() {
		t.Errorf("the capture seed parked the cursor on the unselectable %q; the arrows skip it, so there is no way back to it", row.Label())
	}
	requireCursorOn(t, m, valid.Label())
}
