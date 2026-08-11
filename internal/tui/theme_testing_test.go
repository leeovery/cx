package tui

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
	"github.com/leeovery/portal/internal/tmux"
)

// openPanelForTest sizes m to the given CONTENT region, seeds the standard session
// set behind it, sizes both pages' lists and opens the panel through the production
// `t` keypress.
//
// The dimensions are assigned directly rather than driven through a
// tea.WindowSizeMsg, which is what makes the open-time assertions honest: a fixture
// that resized on its way in could not tell the ladder running at open from the
// ladder running on the resize.
//
// The content region is asserted before the keypress: a fixture that opened at some
// other region would still pass most of its assertions, and the ones it failed would
// read as production faults.
func openPanelForTest(t *testing.T, m Model, contentW, contentH int) Model {
	t.Helper()
	return openPanelForTestWithSessions(t, m, contentW, contentH, closePanelSessions())
}

// openPanelForTestWithSessions is openPanelForTest over a chosen session set, for
// the fixtures whose assertions name the page's own rows.
func openPanelForTestWithSessions(t *testing.T, m Model, contentW, contentH int, sessions []tmux.Session) Model {
	t.Helper()

	m.termWidth, m.termHeight = geometryTerm(contentW, contentH)
	m.applySessions(sessions)
	m.applySessionListSize(m.contentWidth(), m.contentHeight())
	m.applyProjectListSize(m.contentWidth(), m.contentHeight())

	if got := m.contentWidth(); got != contentW {
		t.Fatalf("fixture: the content region is %d columns wide, want %d", got, contentW)
	}
	if got := m.contentHeight(); got != contentH {
		t.Fatalf("fixture: the content region is %d rows tall, want %d", got, contentH)
	}

	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("fixture: the panel did not open")
	}
	return m
}

// newDirBackedPanelModel builds a Sessions-page model wired to a REAL loader over
// dir, with its construction-time nomination resolved exactly as cmd/open.go
// resolves one — the by-name read of the same keys against the same directory — and
// hands back the panel's own enumerator.
//
// The light/dark answer is PINNED rather than detected (WithCanvasMode), because
// the appearance gate resolves exactly once and the panel must read THAT answer
// rather than ask again. Pinning it is what lets one fixture drive the in-force slot in both
// terminals without touching the async race.
func newDirBackedPanelModel(t *testing.T, dir string, keys theme.RawKeys, mode theme.Member) (Model, *countingThemeSource) {
	t.Helper()
	return newDirBackedPanelModelOver(t, dir, keys, mode, theme.NewSilentLoader())
}

// newDirBackedPanelModelOver is newDirBackedPanelModel with the PANEL's loader
// chosen by the caller, for the fixtures that read the panel's own log emissions.
//
// Construction always resolves through a loader of its own, so a caller passing a
// sink-backed one gets a sink holding the PANEL's emissions alone rather than a
// delta against construction's.
func newDirBackedPanelModelOver(t *testing.T, dir string, keys theme.RawKeys, mode theme.Member, panelLoader theme.Loader) (Model, *countingThemeSource) {
	t.Helper()

	setting, _ := theme.ResolveSetting(keys)
	resolution, err := theme.NewSilentLoader().ResolveNomination(setting, dir)
	if err != nil {
		t.Fatalf("construction-time resolution of %+v: %v", setting, err)
	}
	enumerator := countingEnumeratorOver(panelLoader, dir)
	m := New(fakeLister{},
		WithThemeSource(enumerator),
		WithThemeKeys(keys),
		WithThemeNomination(resolution.Nomination),
		WithCanvasMode(mode),
	)
	return m, enumerator
}

// requireCommitDoesNoOtherIO fails unless the commit press reads nothing and
// writes nothing but the prefs call.
//
// The prefs call is the ONE write a commit keypress may make. Everything else is
// asserted absent: no directory read and no fresh enumeration (a commit
// changes prefs, not the directory, and a read here would produce a third parse of
// the same slug that can disagree with the row the user is looking at), no other
// file-touching seam, and no deferred write riding a tea.Cmd.
//
// internal/tui resolves no config path and reads no PORTAL_* env var, so every
// route from this package to disk runs through one of the counted seams; the
// package's tmux writers are neither wired to the panel nor reachable from it.
//
// subject names the commit under test in every assertion message, so a shared
// failure still says which keypress broke the contract.
func requireCommitDoesNoOtherIO(
	t *testing.T,
	keys theme.RawKeys,
	subject string,
	press func(*testing.T, Model) (Model, tea.Cmd),
	assertCommitted func(*testing.T, *fakeThemePersister),
) {
	t.Helper()

	configDir := t.TempDir()
	t.Setenv("PORTAL_PREFS_FILE", filepath.Join(configDir, "prefs.json"))
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")

	stores := newCountingStores()
	// The theme persister is deliberately the RECORDING fake rather than the
	// counted one: it is the single write the keypress is allowed to make, so the
	// counted set covers everything else and this records what the one write was.
	persister := &fakeThemePersister{}
	loader := theme.NewSilentLoader()
	enumerator := countingEnumeratorOver(loader, dir)
	setting, _ := theme.ResolveSetting(keys)
	resolution, err := loader.ResolveNomination(setting, dir)
	if err != nil {
		t.Fatalf("construction-time resolution of %+v: %v", setting, err)
	}

	m := Build(Deps{
		Lister:         stores.lister,
		Theme:          resolution.Nomination,
		ProjectStore:   stores.projectStore,
		ProjectEditor:  stores.projectEditor,
		AliasEditor:    stores.aliasEditor,
		ModePersister:  stores.modePersister,
		Reader:         stores.scrollback,
		ThemeSource:    enumerator,
		ThemeKeys:      keys,
		ThemePersister: persister,
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

	m, cmd := press(t, m)

	assertCommitted(t, persister)
	if got := stores.calls(); got != 0 {
		t.Errorf("%s made %d file-touching seam call(s) — the prefs write is the only one (project store %d, project editor %d, alias editor %d, mode persister %d, theme persister %d, scrollback %d, lister %d)",
			subject, got, stores.projectStore.calls, stores.projectEditor.calls, stores.aliasEditor.calls,
			stores.modePersister.calls, stores.themePersister.calls, stores.scrollback.calls, stores.lister.calls)
	}
	if enumerator.opens != opens {
		t.Errorf("%s ran %d enumerations in total, want the open's %d — a commit re-derives from the retained parse and never re-reads the directory (§8.4)", subject, enumerator.opens, opens)
	}
	if cmd != nil {
		t.Errorf("%s scheduled %T; a deferred write is the one shape the counters above cannot see", subject, cmd)
	}
	if entries, err := os.ReadDir(configDir); err != nil || len(entries) != 0 {
		t.Errorf("%s left %d entries in the config directory (err %v), want none — the model writes through the seam and touches no path of its own", subject, len(entries), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	if len(entries) != 1 || entries[0].Name() != "sunset.theme" {
		t.Errorf("the themes directory holds %d entries after %s, want only the seeded drop-in", len(entries), subject)
	}
}

// panelReadOnlyPath names one of the panel's read-only paths for the shared
// no-write proof: the verb its assertion failures read with, and the name that
// path's absent-prefs subtest runs under.
//
// The subtest name is carried rather than composed, so the string a maintainer
// greps a failure by is a literal at the call site and stays whatever it has
// always been on each path.
type panelReadOnlyPath struct {
	verb          string
	absentSubtest string
}

// requireNoPrefsOrThemesWrite fails unless act leaves a present prefs.json byte
// for byte, and leaves an absent one absent with the config and themes
// directories as it found them.
//
// The proof is owned here rather than per path because it is ONE invariant: the
// panel's read-only paths write no file. Authored twice, one copy would keep
// proving something about a fixture the other no longer stages the moment the
// prefs seam or the theme files move.
//
// act stages nothing of its own — it receives the themes directory and the keys
// naming the theme staged in it, builds its own model and performs its own
// action, so the two paths differ by their model and their keypresses alone.
func requireNoPrefsOrThemesWrite(t *testing.T, path panelReadOnlyPath, act func(t *testing.T, dir string, keys theme.RawKeys)) {
	t.Helper()

	const persisted = `{"session_list_mode":"by-project","theme":"sunset"}`
	keys := theme.RawKeys{Theme: "sunset"}

	t.Run("a present prefs.json survives byte for byte", func(t *testing.T) {
		prefsFile := filepath.Join(t.TempDir(), "prefs.json")
		if err := os.WriteFile(prefsFile, []byte(persisted), 0o644); err != nil {
			t.Fatalf("write prefs: %v", err)
		}
		t.Setenv("PORTAL_PREFS_FILE", prefsFile)
		dir := t.TempDir()
		// The staged theme is INVALID, so the path runs over the fallback the
		// rejection surface takes — the case where a write would overwrite the
		// persisted name the user set.
		writeThemeFileForTest(t, dir, "sunset.theme", "not-a-colour")

		act(t, dir, keys)

		after, err := os.ReadFile(prefsFile)
		if err != nil {
			t.Fatalf("read back prefs: %v", err)
		}
		if string(after) != persisted {
			t.Errorf("prefs.json after %s =\n%s\nwant it byte-identical:\n%s", path.verb, after, persisted)
		}
	})

	t.Run(path.absentSubtest, func(t *testing.T) {
		configDir := t.TempDir()
		t.Setenv("PORTAL_PREFS_FILE", filepath.Join(configDir, "prefs.json"))
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "sunset.theme", "#101010")

		act(t, dir, keys)

		if entries, err := os.ReadDir(configDir); err != nil || len(entries) != 0 {
			t.Errorf("%s left %d entries in the config directory (err %v), want none", path.verb, len(entries), err)
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("read %s: %v", dir, err)
		}
		if len(entries) != 1 || entries[0].Name() != "sunset.theme" {
			t.Errorf("the themes directory holds %d entries after %s, want only the seeded drop-in", len(entries), path.verb)
		}
	})
}

// constantResolution is what a stub theme seam answers with under a CONSTANT
// setting: the one loaded palette, plus the single slot record naming slug as
// both the slug that was asked for and the slug that loaded.
//
// Requested and Resolved are set from ONE argument deliberately: theme.Badges keys
// its map on Requested and the cursor anchors on the in-force slot's Resolved, so a
// shape free to transpose them can be wrong in one file while every reader of it
// still resolves.
func constantResolution(slug string, th theme.Theme) theme.Resolution {
	return theme.Resolution{
		Nomination: theme.ConstantNomination(th),
		Slots: []theme.SlotResolution{{
			Slot:      theme.SlotConstant,
			Requested: slug,
			Resolved:  slug,
			Theme:     th,
		}},
	}
}

// pairResolution is constantResolution's ADAPTIVE counterpart: the pair the two
// rows nominate, plus one record per slot in light-then-dark order, each carrying
// its own row's slug and palette.
func pairResolution(light, dark theme.Row) theme.Resolution {
	return theme.Resolution{
		Nomination: theme.AdaptivePair(theme.MemberLight.Palette(light.Theme), dark.Theme),
		Slots: []theme.SlotResolution{
			{Slot: theme.SlotLight, Requested: light.Slug, Resolved: light.Slug, Theme: light.Theme},
			{Slot: theme.SlotDark, Requested: dark.Slug, Resolved: dark.Slug, Theme: dark.Theme},
		},
	}
}

// stubPanelDeps is the stub-backed panel seam skeleton: the stub theme source,
// the nomination the model is constructed with, and the raw keys the panel reads
// its persisted setting from.
//
// A builder adds only what it differs by — its rows, its priming, its persister.
func stubPanelDeps(source *fakeThemeSource, nomination theme.Nomination, keys theme.RawKeys) Deps {
	return Deps{
		Lister:      fakeLister{},
		Theme:       nomination,
		ThemeSource: source,
		ThemeKeys:   keys,
	}
}

// TestConstantResolution_IsTheConstantStubShape pins what the shared constant
// fixture answers with — the shape a stub-backed suite's badges and in-force slot
// are derived from under a constant setting.
func TestConstantResolution_IsTheConstantStubShape(t *testing.T) {
	th := testDarkTheme(t)
	got := constantResolution("aurora", th)

	if !got.Nomination.IsConstant() {
		t.Errorf("the nomination is %+v, want the CONSTANT shape", got.Nomination)
	}
	if palette := got.Nomination.Constant(); palette != th {
		t.Errorf("the nomination carries canvas %s, want the given palette's %s", palette.Canvas.Value, th.Canvas.Value)
	}
	want := []theme.SlotResolution{{
		Slot:      theme.SlotConstant,
		Requested: "aurora",
		Resolved:  "aurora",
		Theme:     th,
	}}
	if !reflect.DeepEqual(got.Slots, want) {
		t.Errorf("the slots are %+v, want the single constant record %+v", got.Slots, want)
	}
}

// TestPairResolution_IsTheAdaptiveStubShape pins the adaptive counterpart: two
// records in light-then-dark order, each carrying ITS OWN row's slug and palette,
// under a nomination selecting the same two.
func TestPairResolution_IsTheAdaptiveStubShape(t *testing.T) {
	light := theme.Row{Slug: "day", Source: theme.SourceBuiltin, Theme: testLightTheme(t)}
	dark := theme.Row{Slug: "night", Source: theme.SourceBuiltin, Theme: testDarkTheme(t)}
	got := pairResolution(light, dark)

	if got.Nomination.IsConstant() {
		t.Errorf("the nomination is %+v, want the ADAPTIVE shape", got.Nomination)
	}
	if palette := got.Nomination.Select(theme.MemberLight); palette != light.Theme {
		t.Errorf("the nomination's light member carries canvas %s, want %s", palette.Canvas.Value, light.Theme.Canvas.Value)
	}
	if palette := got.Nomination.Select(theme.MemberDark); palette != dark.Theme {
		t.Errorf("the nomination's dark member carries canvas %s, want %s", palette.Canvas.Value, dark.Theme.Canvas.Value)
	}
	want := []theme.SlotResolution{
		{Slot: theme.SlotLight, Requested: "day", Resolved: "day", Theme: light.Theme},
		{Slot: theme.SlotDark, Requested: "night", Resolved: "night", Theme: dark.Theme},
	}
	if !reflect.DeepEqual(got.Slots, want) {
		t.Errorf("the slots are %+v, want the light-then-dark pair %+v", got.Slots, want)
	}
}

// sgrParams renders a one-cell probe through style and returns the SGR parameter
// run it opens with — everything between the CSI `[` and the terminating `m`.
//
// The bare run is the shape most frame assertions want: a style carrying a
// foreground, a background and attributes emits them as ONE sequence, so the
// wrapper a single-property style renders is not what lands in composed output —
// the `38;2;...` / `48;2;...` core is. A call site that needs the whole `ESC[…m`
// (a background-only fill does emit it standalone) rebuilds it around this run.
func sgrParams(t *testing.T, style lipgloss.Style) string {
	t.Helper()
	probe := style.Render("x")
	start := strings.IndexByte(probe, '[')
	end := strings.IndexByte(probe, 'm')
	if start < 0 || end <= start {
		t.Fatalf("could not derive an SGR parameter run from %q", probe)
	}
	return probe[start+1 : end]
}

// tokenFgSeq returns the bare `38;2;r;g;b` foreground SGR parameter run a role
// token renders as.
func tokenFgSeq(t *testing.T, tok theme.Token) string {
	t.Helper()
	return sgrParams(t, lipgloss.NewStyle().Foreground(tok.Color()))
}

// tokenBgSeq returns the bare `48;2;r;g;b` background SGR parameter run a role
// token renders as — the background analogue of tokenFgSeq, used to assert a tint
// IS or is NOT painted.
func tokenBgSeq(t *testing.T, tok theme.Token) string {
	t.Helper()
	return sgrParams(t, lipgloss.NewStyle().Background(tok.Color()))
}

// testDarkTheme and testLightTheme load the two embedded built-ins the model's
// transitional theme source selects between — the same two `internal/theme`
// resolves for the dark and light halves of the shipped default.
//
// They exist as ONE pair of helpers rather than an ad-hoc LoadBuiltin at each of
// the several hundred render-assertion sites: a test that needs "the palette the
// dark canvas renders" needs exactly this, and a per-site load would make the
// suite's notion of "the dark theme" re-derivable in several hundred places.
//
// They are FUNCTIONS, not package-level vars, deliberately. The active-theme plumbing forbids
// package-level mutable theme state on the render path, and TestNoPackageLevelThemeVar
// guards production against it; keeping the test-side source a function too means
// the suite cannot grow the very shape the guard exists to prevent.
func testDarkTheme(t *testing.T) theme.Theme {
	t.Helper()
	return themetest.DefaultDark(t)
}

func testLightTheme(t *testing.T) theme.Theme {
	t.Helper()
	return themetest.DefaultLight(t)
}

// builtinThemeCase is one shipped built-in under the subtest name the render
// suite runs it as.
type builtinThemeCase struct {
	name string
	th   theme.Theme
}

// builtinThemeCases is the render suite's single declaration of the built-in
// pair: the dark palette then the light one, named as their subtests.
//
// The pair follows internal/theme's shipped-default slugs through testDarkTheme
// and testLightTheme rather than restating which themes those are, so a change to
// the shipped pair carries the whole suite with it.
//
// forEachBuiltinTheme is the form to reach for. The slice form is for a site that
// iterates the pair INSIDE a subtest it names itself, where running a dark/light
// subtest would rename tests.
func builtinThemeCases(t *testing.T) []builtinThemeCase {
	t.Helper()
	return []builtinThemeCase{
		{"dark", testDarkTheme(t)},
		{"light", testLightTheme(t)},
	}
}

// forEachBuiltinTheme runs fn against each shipped built-in's palette, as a
// subtest named for that built-in's half of the pair.
func forEachBuiltinTheme(t *testing.T, fn func(t *testing.T, th theme.Theme)) {
	t.Helper()
	for _, tc := range builtinThemeCases(t) {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc.th)
		})
	}
}

// forEachCanvasMode is forEachBuiltinTheme for the sites parameterised by the
// canvas answer in force rather than by a palette — the ones that build a model
// for a mode and derive the palette from it.
func forEachCanvasMode(t *testing.T, fn func(t *testing.T, m theme.Member)) {
	t.Helper()
	for _, tc := range []struct {
		name string
		mode theme.Member
	}{
		{"dark", theme.MemberDark},
		{"light", theme.MemberLight},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fn(t, tc.mode)
		})
	}
}

// themeLabel names th for a failure message — "dark", "light", or the theme's
// canvas hex for anything else.
//
// A token carries its VALUE now, so a whole Theme through %v is 19 {name value}
// pairs: ~500 characters of noise on a line whose only job is to say WHICH theme
// failed, where the mode it replaced printed 0 or 1. This says exactly that much.
//
// It identifies a theme by its CANVAS because that is the one token the two
// built-ins can never share — the canvas is what the mode *is*. Anything the
// built-in pair does not claim falls back to that hex rather than to "", so a
// hand-built or future theme still names itself in the message; a zero Theme
// (which several helper-built models legitimately hold) says so outright rather
// than borrowing a built-in's label.
func themeLabel(th theme.Theme) string {
	switch v := th.Canvas.Value; v {
	case "":
		return "zero-theme"
	case builtinCanvasValue(theme.DefaultDarkSlug):
		return "dark"
	case builtinCanvasValue(theme.DefaultLightSlug):
		return "light"
	default:
		return v
	}
}

// builtinCanvasValue returns the canvas value of the embedded built-in named
// slug, or "" if it does not load.
//
// It takes no *testing.T — themeLabel is evaluated inside an argument list, where
// there is no failing to be done — so a broken embedded file degrades to "",
// which themeLabel has already ruled out as a candidate value before it asks.
func builtinCanvasValue(slug string) string {
	loaded, rejection, found := theme.NewSilentLoader().LoadBuiltin(slug)
	if !found || rejection != nil {
		return ""
	}
	return loaded.Theme.Canvas.Value
}

// tokenNamed returns the role token called name off th.
//
// It exists because a token now carries its VALUE, so a table of tokens is a
// table of one theme's values: a test that walks several themes must re-resolve
// each role per theme rather than hold a fixed []theme.Token. Naming the role and
// resolving it here keeps those tables reading as the role list they are, and
// resolves through Theme.All() so a role that stops existing fails loudly rather
// than silently yielding a zero token.
func tokenNamed(t *testing.T, th theme.Theme, name string) theme.Token {
	t.Helper()
	for _, tok := range th.All() {
		if tok.Name == name {
			return tok
		}
	}
	t.Fatalf("theme carries no token named %q", name)
	return theme.Token{}
}

// testConstantFor is the CONSTANT nomination painting the built-in that the given
// canvas answer names.
//
// It is how a test pins a palette from frame one now that there is no light/dark
// appearance to pin: a constant skips the gate entirely, so the model is
// resolved at construction and the frame is un-gated — the same property the
// capture harness relies on.
func testConstantFor(t *testing.T, appearance theme.Member) theme.Nomination {
	t.Helper()
	return theme.ConstantNomination(themeForAppearance(t, appearance))
}

// appearanceForTheme returns the gate answer that selects th from the built-in
// pair — the inverse of themeForAppearance, for the tests that drive a model
// through WithCanvasMode (which pins the ANSWER, from which the model re-derives
// the palette) while asserting against a palette.
func appearanceForTheme(t *testing.T, th theme.Theme) theme.Member {
	t.Helper()
	switch th.Canvas.Value {
	case testLightTheme(t).Canvas.Value:
		return theme.MemberLight
	case testDarkTheme(t).Canvas.Value:
		return theme.MemberDark
	}
	t.Fatalf("theme with canvas %q is neither built-in", th.Canvas.Value)
	return theme.MemberDark
}
