package cmd

// Tests in this file seed prefs.json / PORTAL_THEMES_DIR through t.Setenv and
// drive package-level cmd seams, so they MUST NOT use t.Parallel.

import (
	"image/color"
	"maps"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
	"github.com/leeovery/portal/internal/tui"
)

// THE COMMIT ROUND TRIP: a keypress in the panel, the persister, prefs.json, and
// the theme the NEXT launch renders — driven end to end through the production
// seams.
//
// The promise crosses four packages and three translations (the row under the
// cursor, the typed prefs slot, the on-disk key, the raw keys construction
// resolves on the next launch), and each END of it is already covered in
// isolation: internal/tui drives the commit keys against a fake persister, and
// this package drives the persister against a real store. What neither reaches
// is the JOIN, where a keypress writing the wrong slug, the wrong slot or the
// wrong file is invisible to both.
//
// So the model here is built through the production chokepoint carrying the
// PRODUCTION persister and the PRODUCTION enumerator over a real themes
// directory, and the assertions are the two ends the user cares about: the bytes
// that reach disk, and the palette a relaunch resolves back out of them.
//
// EVERY OBSERVATION IS THROUGH THE FRAME OR THE FILE. This package cannot see
// the panel's cursor — internal/tui holds it unexported — so the fixtures locate
// it the way the user does, by the canvas the previewed row paints. That is what
// makes "it commits the cursor's slug" an assertion here rather than an
// assumption about row order.
//
// NO TMUX AND NO REAL CONFIG PATH. The model's tmux-touching seams are the
// package's stubs (defaultTestTUIConfig), and prefs.json and the themes
// directory are both temp dirs pointed at through the env — so the round trip
// runs in the unit lane and touches nothing of the developer's.

const (
	// The two drop-in slugs every fixture below is built from: the one already in
	// prefs.json when the panel opens, and the one the user arrows to and commits.
	// They are DROP-INS rather than built-ins because only a file proves the panel
	// read the themes directory the production enumerator resolved.
	roundTripStandingSlug = "aurora"
	roundTripChosenSlug   = "sunset"

	// Their canvases. The canvas is the one token that says WHICH file was
	// parsed, and the cursor tracking below reads it back off the rendered frame,
	// so the two must differ from each other and from every built-in's.
	roundTripStandingCanvas = "#101010"
	roundTripChosenCanvas   = "#202020"

	// roundTripDropIns is how many files seedRoundTripThemes writes; the cursor
	// walk adds it to the embedded set to bound the panel's row count.
	roundTripDropIns = 2

	// The terminal the picker is sized to: wide and tall enough that the panel's
	// render floor clears with room to spare, so no fixture here is one column
	// away from a blocked-entry flash.
	roundTripTermWidth  = 100
	roundTripTermHeight = 28

	// roundTripPanelBorder is the panel's left-border glyph, the one column every
	// row of the slide-over begins with. It is a literal because internal/tui keeps
	// its own copy unexported; should the two ever disagree the parse below finds no
	// panel rows at all and says so.
	roundTripPanelBorder = "│"
)

var (
	// themePanelOpenKey is `t`.
	themePanelOpenKey = tea.KeyPressMsg{Code: 't', Text: "t"}
	// themePanelConstantKey is `Enter` — commit the cursor's row as the constant
	// theme. themePanelDarkSlotKey is `d` — commit it into the dark half of the
	// adaptive pair.
	themePanelConstantKey = tea.KeyPressMsg{Code: tea.KeyEnter}
	themePanelDarkSlotKey = tea.KeyPressMsg{Code: 'd', Text: "d"}
	// themePanelCursorUpKey / themePanelCursorDownKey move the panel cursor one row,
	// previewing as they go.
	themePanelCursorUpKey   = tea.KeyPressMsg{Code: tea.KeyUp}
	themePanelCursorDownKey = tea.KeyPressMsg{Code: tea.KeyDown}
	// themePanelConfirmKey is `y` — the answer that lets a slot commit through the
	// gate `d`/`l` raise over a constant, so the write on that route happens here
	// rather than on `d`.
	themePanelConfirmKey = tea.KeyPressMsg{Code: 'y', Text: "y"}

	// roundTripBadgeCopy is the panel's `●` vocabulary, restated here because
	// internal/tui keeps the four strings unexported. It is what turns the badge
	// table prefs' own bytes derive to into the words a frame can be searched for.
	roundTripBadgeCopy = map[theme.Badge]string{
		theme.BadgeConstant: "●",
		theme.BadgeLight:    "● light",
		theme.BadgeDark:     "● dark",
		theme.BadgeBoth:     "● both",
	}
)

// seedRoundTripThemes writes the two drop-ins into a fresh themes directory and
// points the production path resolution at it, so the panel's rows come from a
// real directory read rather than from the embedded set alone.
func seedRoundTripThemes(t *testing.T) {
	t.Helper()

	dir := useThemesDir(t)
	writeThemeFile(t, dir, roundTripStandingSlug, roundTripStandingCanvas)
	writeThemeFile(t, dir, roundTripChosenSlug, roundTripChosenCanvas)
}

// themeRoundTripConfig wires the panel's four slots over whatever prefs.json and
// themes directory the test seeded, exactly as TUI construction wires them: ONE
// loader shared by the construction-time resolution and the panel's enumerator,
// the control-stripped keys that resolution hands back, and the production
// persister over the store the same load produced.
//
// The persister is wired only where the store actually loaded. A typed-nil
// *prefs.Store wrapped in a persister boxes into a NON-NIL seam whose every
// write panics, so the slot is left empty instead — the state
// TestThemePanelCommit_NoPrefsStoreWritesNothing drives.
func themeRoundTripConfig(t *testing.T) tuiConfig {
	t.Helper()

	// The construction-time load and the panel's enumeration both emit under the
	// `theme` component. These tests judge the FILE and the FRAME, so the sink is
	// installed to keep those emissions off the process-wide handler rather than
	// to be read.
	installMigrateCapture(t)

	load, err := loadPrefsStore()
	if err != nil {
		t.Fatalf("load the prefs store: %v", err)
	}
	loader := newThemeLoader()
	resolution, keys, err := themeResolution(load.Keys, loader)
	if err != nil {
		t.Fatalf("resolve the persisted theme setting: %v", err)
	}

	cfg := defaultTestTUIConfig()
	cfg.theme = resolution.Nomination
	cfg.themeKeys = keys
	cfg.themeEnumerator = newThemeEnumerator(loader)
	if load.Store != nil {
		cfg.themePersister = newThemePersister(load.Store)
	}
	return cfg
}

// startRoundTripPicker builds the model through the production chokepoint and
// brings it to the state a user first sees: sized by the terminal's own resize,
// and past the light/dark gate.
//
// The gate is answered DARK because every fixture here persists an adaptive
// pair, whose gate holds the first paint until a reply lands — without one the
// frame the cursor is tracked through would be the pre-resolution blank, and the
// pair's in-force member would be undecided.
func startRoundTripPicker(t *testing.T, cfg tuiConfig) tui.Model {
	t.Helper()

	m := buildTUIModel(cfg, "", nil)
	m = update(t, m, tea.WindowSizeMsg{Width: roundTripTermWidth, Height: roundTripTermHeight})
	return update(t, m, darkBackgroundReply)
}

// openRoundTripPanel presses `t` and fails unless the panel actually opened, so
// no commit assertion below can pass over a keypress that opened nothing.
func openRoundTripPanel(t *testing.T, m tui.Model) tui.Model {
	t.Helper()

	m = update(t, m, themePanelOpenKey)
	if view := m.View().Content; !strings.Contains(view, themePanelHeaderCopy) {
		t.Fatalf("the frame after `t` carries no %q header, so the commit keys would land on no panel:\n%s", themePanelHeaderCopy, view)
	}
	return m
}

// arrowToPreviewedCanvas walks the panel's rows with the arrow keys and stops on
// the one that paints the given canvas.
//
// The panel previews the row under the cursor, so the painted canvas IS the
// cursor's position as seen from outside internal/tui — which is what lets a
// fixture move to a named row without knowing the union's ordering.
//
// IT PARKS AT THE TOP FIRST. `↓` clamps at the last row rather than wrapping, so
// a walk starting wherever the panel opened could only ever reach a row BELOW
// that one — and every fixture here would quietly depend on where its chosen slug
// sorts against its standing one.
func arrowToPreviewedCanvas(t *testing.T, m tui.Model, canvas string) tui.Model {
	t.Helper()

	want := canvasColour(canvas)
	rows := len(theme.BuiltinSlugs()) + roundTripDropIns
	for range rows {
		m = update(t, m, themePanelCursorUpKey)
	}
	// Each pass checks the row it is on and then steps off it, so rows passes
	// cover every row from the top one down.
	for range rows {
		if m.View().BackgroundColor == want {
			return m
		}
		m = update(t, m, themePanelCursorDownKey)
	}
	t.Fatalf("arrowing never previewed the theme whose canvas is %s, so the commit below would be about a row the cursor never reached", canvas)
	return m
}

// canvasColour resolves a canvas hex the way the render path does — through the
// token accessor the frame's own background is taken from — so the comparison is
// between two values of one derivation rather than two conversions.
func canvasColour(canvas string) color.Color {
	return theme.Token{Value: canvas}.Color()
}

// assertBadgesMatchPersistedKeys binds the panel's `●` set to the keys prefs
// ACTUALLY WROTE: ONE read-back off disk, resolved into the badge table, compared
// against the markers on the frame.
//
// IT IS THE JOIN BETWEEN THE TWO ENDS. A commit writes through the persister and
// then restates prefs' mutual exclusion over the keys the model holds, so the
// panel can re-derive its rows and badges without a read-back. Those are two
// statements of one rule, and what they admit is the invariant the whole panel
// rests on: a `●` claiming a state the file does not hold, with no other surface
// to contradict it until relaunch.
//
// The badges are the observation this package CAN make: internal/tui keeps the
// model's keys unexported, and the marker is derived from them and from nothing
// else.
//
// THEY ARE A PROJECTION OF THE KEYS RATHER THAN THE KEYS. A stale slot sitting
// beside a constant is invisible to them — the `theme`-wins tiebreak leaves the
// slots unread, so a constant commit that failed to clear them moves no badge and
// mints no row — and that drift surfaces only on the NEXT commit, once clearing
// the constant makes those slots live. A constant commit is therefore bound by
// the commit that FOLLOWS it rather than by itself, which is what a sequence of
// two is for.
func assertBadgesMatchPersistedKeys(t *testing.T, m tui.Model) {
	t.Helper()

	want := badgesForPersistedKeys(t)
	got := panelBadgesOnFrame(t, m)
	if !maps.Equal(got, want) {
		t.Errorf("the panel marks %v, want %v — the badges and prefs.json disagree about what is set\n%s",
			got, want, ansi.Strip(m.View().Content))
	}
}

// badgesForPersistedKeys is the badge table prefs.json's own bytes derive to,
// rendered in the panel's copy.
//
// The read is the NON-MIGRATING one, so a diagnosis of what was written cannot
// itself write, and the resolution is the production construction-time one — the
// same derivation the next launch performs on the same bytes.
func badgesForPersistedKeys(t *testing.T) map[string]string {
	t.Helper()

	store, err := loadPrefsStoreNoMigrate()
	if err != nil {
		t.Fatalf("resolve the prefs store: %v", err)
	}
	keys, err := store.LoadThemeKeys()
	if err != nil {
		t.Fatalf("read the theme keys back off disk: %v", err)
	}
	resolution, _, err := themeResolution(keys, newThemeLoader())
	if err != nil {
		t.Fatalf("resolve the persisted theme setting %+v: %v", keys, err)
	}

	badges := make(map[string]string)
	for slug, badge := range theme.Badges(resolution.Slots) {
		marker, ok := roundTripBadgeCopy[badge]
		if !ok {
			t.Fatalf("prefs.json's keys derive badge %d for %q, which the panel has no copy for", badge, slug)
		}
		badges[slug] = marker
	}
	return badges
}

// panelBadgesOnFrame reads the `●` off the rendered slide-over, keyed by the slug
// whose row carries it.
//
// ONLY THE UNION'S OWN SLUGS ARE LOOKED UP. The parse sees the whole panel, and
// the vertical footer's rows read `d set as dark` and `l set as light` — a scan
// for the badge vocabulary over everything would match on chrome the panel always
// draws. Each slug is required PRESENT, so a badged row that paginated off the
// frame fails rather than reading as an absent marker.
func panelBadgesOnFrame(t *testing.T, m tui.Model) map[string]string {
	t.Helper()

	frame := ansi.Strip(m.View().Content)
	trailing := panelRowTrailing(t, frame)

	badges := make(map[string]string)
	for _, slug := range roundTripUnionSlugs() {
		row, listed := trailing[slug]
		if !listed {
			t.Fatalf("the frame carries no %q row, so a badge on it could not be seen:\n%s", slug, frame)
		}
		if row != "" {
			badges[slug] = row
		}
	}
	return badges
}

// panelRowTrailing is every row of the rendered slide-over, keyed by its first
// word and holding whatever follows it.
//
// Every panel line begins with the one border column, so the panel's own text is
// whatever follows the first such glyph, and a list row composes as
// `[cursor column][label][pad][badge]` — leaving the label as the key and the
// badge as the value.
func panelRowTrailing(t *testing.T, frame string) map[string]string {
	t.Helper()

	trailing := make(map[string]string)
	for line := range strings.SplitSeq(frame, "\n") {
		_, panel, onPanel := strings.Cut(line, roundTripPanelBorder)
		if !onPanel {
			continue
		}
		fields := strings.Fields(panel)
		if len(fields) > 0 && fields[0] == "▌" {
			fields = fields[1:]
		}
		if len(fields) == 0 {
			continue
		}
		trailing[fields[0]] = strings.Join(fields[1:], " ")
	}
	if len(trailing) == 0 {
		t.Fatalf("no panel rows parsed out of the frame; the slide-over did not render:\n%s", frame)
	}
	return trailing
}

// roundTripUnionSlugs is every row the panel lists in these fixtures: the
// embedded set plus the two seeded drop-ins.
func roundTripUnionSlugs() []string {
	return append(theme.BuiltinSlugs(), roundTripStandingSlug, roundTripChosenSlug)
}

// TestThemePanelCommit_EnterRoundTripsAConstantToPrefs: `Enter` writes the
// cursor's slug to prefs.json as the constant theme, and the next launch renders
// it.
//
// The fixture persists an adaptive PAIR while the commit is a CONSTANT, so one
// read of the record proves both halves of the mutual exclusion: the key that
// landed, and the slot the same atomic write cleared. session_list_mode is
// asserted as a survivor — the write merges into the record rather than
// replacing it.
//
// THE CURSOR IS ARROWED AWAY from the row the panel opened on, which is the only
// shape in which "the cursor's slug" and "the persisted slug" are
// distinguishable strings: with the cursor left where the open put it, a commit
// of the wrong one of the two would write the same bytes.
//
// THE PANEL IS HELD TO THE SAME BYTES. Both slots being cleared is a rule the
// write applies to the file and the panel applies to the keys it holds, so the
// badges are asserted against what a read-back derives: two `●` collapsing to one
// bare marker on the committed row is the visible half of the clear.
func TestThemePanelCommit_EnterRoundTripsAConstantToPrefs(t *testing.T) {
	seedRoundTripThemes(t)
	path := setPrefsFile(t, `{"session_list_mode":"by-tag","theme_dark":"`+roundTripStandingSlug+`"}`)

	m := startRoundTripPicker(t, themeRoundTripConfig(t))
	m = openRoundTripPanel(t, m)
	assertPaintedCanvas(t, m, canvasColour(roundTripStandingCanvas))
	m = arrowToPreviewedCanvas(t, m, roundTripChosenCanvas)

	m = update(t, m, themePanelConstantKey)

	assertPrefsOnDisk(t, path, prefsOnDisk{SessionListMode: "by-tag", Theme: roundTripChosenSlug})
	assertBadgesMatchPersistedKeys(t, m)

	// The relaunch: prefs.json read fresh off disk and resolved through the same
	// construction-time resolution the next launch runs.
	nomination := themeNominationForTest(t)
	if !nomination.IsConstant() {
		t.Fatalf("the relaunch resolved an adaptive pair; want the constant `Enter` committed")
	}
	assertCanvasValue(t, nomination.Constant(), roundTripChosenCanvas)
}

// TestThemePanelCommit_DarkKeyOverAConstantRoundTripsThePairToPrefs: over a
// CONSTANT, `d` asks first and `y` writes the cursor's slug into the dark half,
// clearing the constant in the same write.
//
// It is the other direction of the mutual exclusion, and the only one in which
// the clear is destructive: the constant the user chose goes, and the untouched
// light slot becomes live at the shipped default. The confirm exists for exactly
// that, so the write is driven through it rather than around it.
//
// THE BADGES ARE ASSERTED AGAINST THE SAME READ-BACK the bytes are, which is what
// binds the panel's in-memory restatement of the rule to the file: one bare `●`
// becoming a `● light` on a default nobody set and a `● dark` on the committed row
// is the whole of the transition, and it is claimed by the panel with nothing else
// on screen to contradict it.
func TestThemePanelCommit_DarkKeyOverAConstantRoundTripsThePairToPrefs(t *testing.T) {
	seedRoundTripThemes(t)
	path := setPrefsFile(t, `{"theme":"`+roundTripStandingSlug+`"}`)

	m := startRoundTripPicker(t, themeRoundTripConfig(t))
	m = openRoundTripPanel(t, m)
	assertPaintedCanvas(t, m, canvasColour(roundTripStandingCanvas))
	m = arrowToPreviewedCanvas(t, m, roundTripChosenCanvas)

	// `d` alone only raises the question, so the file is asserted untouched before
	// the answer: a keypress that wrote here would make the confirm decorative.
	m = update(t, m, themePanelDarkSlotKey)
	assertPrefsOnDisk(t, path, prefsOnDisk{Theme: roundTripStandingSlug})
	m = update(t, m, themePanelConfirmKey)

	assertPrefsOnDisk(t, path, prefsOnDisk{ThemeDark: roundTripChosenSlug})
	assertBadgesMatchPersistedKeys(t, m)

	nomination := themeNominationForTest(t)
	if nomination.IsConstant() {
		t.Fatalf("the relaunch resolved a constant; want the pair `d` converted it into")
	}
	assertCanvasValue(t, nomination.Select(theme.MemberDark), roundTripChosenCanvas)
	assertCanvasValue(t, nomination.Select(theme.MemberLight), themetest.Builtin(t, theme.DefaultLightSlug).Canvas.Value)
}

// TestThemePanelCommit_ConsecutiveCommitsStayBoundToPrefs: a SECOND commit in the
// same panel session mirrors from the keys the FIRST one actually wrote.
//
// A CONSTANT'S CLEAR IS NOT VISIBLE ON THE FRAME IT HAPPENS ON. Under a constant
// the slots are not read at all, so slots the commit failed to clear move no
// badge and mint no row — one constant commit renders identically whether the
// rule was applied or dropped. The next commit is where it bites:
// clearing the constant makes those stale slots live, and the panel then marks a
// `● light` on a slug prefs.json has no key for, claiming a setting the file does
// not hold.
//
// So the sequence is a constant over a pair and then a slot over that constant,
// with the bytes and the badges asserted after EACH. The second commit goes
// through the confirm because the setting it acts on is the constant the first one
// just made.
func TestThemePanelCommit_ConsecutiveCommitsStayBoundToPrefs(t *testing.T) {
	seedRoundTripThemes(t)
	path := setPrefsFile(t, `{"theme_light":"`+roundTripStandingSlug+`","theme_dark":"`+nordSlug+`"}`)

	m := startRoundTripPicker(t, themeRoundTripConfig(t))
	m = openRoundTripPanel(t, m)
	assertPaintedCanvas(t, m, themetest.Builtin(t, nordSlug).Canvas.Color())
	m = arrowToPreviewedCanvas(t, m, roundTripChosenCanvas)

	m = update(t, m, themePanelConstantKey)
	assertPrefsOnDisk(t, path, prefsOnDisk{Theme: roundTripChosenSlug})
	assertBadgesMatchPersistedKeys(t, m)

	m = update(t, m, themePanelDarkSlotKey)
	m = update(t, m, themePanelConfirmKey)

	assertPrefsOnDisk(t, path, prefsOnDisk{ThemeDark: roundTripChosenSlug})
	assertBadgesMatchPersistedKeys(t, m)
}

// TestThemePanelCommit_DarkKeyRoundTripsOneSlotToPrefs: `d` writes the cursor's
// slug into the DARK half of the adaptive pair, and the next launch renders the
// pair it completes.
//
// THE LIGHT SLOT IS ASSERTED UNTOUCHED. A slot commit writes one half and clears
// the constant in a single write; carrying the other half across is prefs' rule,
// and re-deriving it anywhere on this path would be a second implementation of
// it.
//
// The setting is ALREADY ADAPTIVE, so the keypress commits outright: the
// slot-from-constant confirm guards the opposite direction, where the same key
// would silently clear a constant the user chose.
func TestThemePanelCommit_DarkKeyRoundTripsOneSlotToPrefs(t *testing.T) {
	seedRoundTripThemes(t)
	path := setPrefsFile(t, `{"theme_light":"`+roundTripStandingSlug+`","theme_dark":"`+nordSlug+`"}`)

	m := startRoundTripPicker(t, themeRoundTripConfig(t))
	m = openRoundTripPanel(t, m)
	// A dark terminal renders the pair's DARK member, so the panel opens on the
	// built-in the dark slot names rather than on either drop-in.
	assertPaintedCanvas(t, m, themetest.Builtin(t, nordSlug).Canvas.Color())
	m = arrowToPreviewedCanvas(t, m, roundTripChosenCanvas)

	update(t, m, themePanelDarkSlotKey)

	assertPrefsOnDisk(t, path, prefsOnDisk{ThemeLight: roundTripStandingSlug, ThemeDark: roundTripChosenSlug})

	nomination := themeNominationForTest(t)
	if nomination.IsConstant() {
		t.Fatalf("the relaunch resolved a constant; want the pair `d` completed")
	}
	assertCanvasValue(t, nomination.Select(theme.MemberLight), roundTripStandingCanvas)
	assertCanvasValue(t, nomination.Select(theme.MemberDark), roundTripChosenCanvas)
}

// TestThemePanelCommit_NoPrefsStoreWritesNothing: with no prefs store there is
// no persister, and both commit keys write nothing at all.
//
// It is the behavioural statement of the guard TUI construction's wiring
// carries. A persister built unconditionally would wrap a nil store, box into a
// NON-NIL seam, and hand the panel a live-looking writer whose every commit
// panics — so the slot is left empty instead, which is the model this drives.
//
// THE SILENCE IS NOT VACUOUS. The panel is asserted open and the cursor is
// asserted to have MOVED before either key is pressed, and the panel is still
// open after both — so the keys landed on a live panel that simply had nowhere
// to write, rather than falling through to the page beneath. The two wired round
// trips above are the positive control, driving the same keys over the same rows
// into the same file.
func TestThemePanelCommit_NoPrefsStoreWritesNothing(t *testing.T) {
	seedRoundTripThemes(t)
	const seeded = `{"session_list_mode":"by-tag","theme_dark":"` + roundTripStandingSlug + `"}`
	path := setPrefsFile(t, seeded)

	cfg := themeRoundTripConfig(t)
	// The unwired slot is the whole fixture: an unresolvable prefs path leaves TUI
	// construction with no store, and it wires no persister from one.
	cfg.themePersister = nil

	m := startRoundTripPicker(t, cfg)
	m = openRoundTripPanel(t, m)
	assertPaintedCanvas(t, m, canvasColour(roundTripStandingCanvas))
	m = arrowToPreviewedCanvas(t, m, roundTripChosenCanvas)

	m = update(t, m, themePanelConstantKey)
	m = update(t, m, themePanelDarkSlotKey)

	assertPrefsUnchanged(t, path, []byte(seeded))
	if view := m.View().Content; !strings.Contains(view, themePanelHeaderCopy) {
		t.Errorf("a commit over the unwired seam closed the panel:\n%s", view)
	}
}
