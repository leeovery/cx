package tui

import (
	"errors"
	"io/fs"
	"os"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// §8.4's ONE THEME LOAD OUTSIDE CONSTRUCTION: the newly-live opposite slot on a
// constant → adaptive conversion, and the commit-time `theme: loaded` §12.3's
// cadence column exists for.
//
// Converting a constant makes a SECOND slot live that construction never loaded —
// a constant nominates one theme, so the other member simply does not exist in the
// model's nomination at the moment the confirm resolves. Three properties make the
// tests below more than "a slot was resolved":
//
//   - IT IS THE OPPOSITE SLOT, NEVER THE ASSIGNED ONE. The assigned slug's parse
//     is already in hand — it is the row the user is looking at — and re-reading it
//     would re-parse a file the panel already holds, which is the half a reader is
//     most likely to add "for symmetry".
//   - IT READS NO DIRECTORY. A commit-time read would produce a THIRD parse of the
//     same slug, neither construction's nor the panel's, that can disagree with the
//     row on screen — the staleness split §5.8 exists to close. Asserted with the
//     themes directory REMOVED after the panel opened.
//   - THE ANSWER HALF IS ESTABLISHED HERE TOO (§9.3). A conversion makes light/dark
//     matter for a user whose launch deliberately never consulted detection, so the
//     in-force answer is classified from the reply already in hand — no new query,
//     no new race, no new gate — and NEVER read off the constant path's pre-resolved
//     gate, whose value is the standing dark fallback rather than a fact about the
//     terminal.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// conversionConstant is the persisted constant every conversion fixture starts
// from, staged as a real drop-in so the panel lists it and the cursor opens on it.
const conversionConstant = "aurora"

// conversionConstantValue is the colour every token of that drop-in carries. It is
// deliberately NOT any built-in's canvas, so the startup canvas hex captured from
// it is distinguishable from every palette a conversion could wrongly re-capture.
const conversionConstantValue = "#101010"

// newConversionThemesDir stages the themes directory a conversion fixture opens
// over: the persisted constant's own drop-in, plus whatever else the case needs.
func newConversionThemesDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, conversionConstant+".theme", conversionConstantValue)
	return dir
}

// newConversionPanelModel is the conversion fixture: a REAL loader over a REAL
// themes directory, emitting into a capture sink, with a recording persister and
// the panel already open.
//
// The loader is real for the reason the recompute fixture's is: a stub seam answers
// from declared values, so every resolved slug and every emitted line would be a
// statement about the fixture rather than about the resolution the commit drives.
//
// The persister is wired through Deps rather than attached afterwards because the
// conversion is the subject here — a model that cannot write cannot convert.
func newConversionPanelModel(t *testing.T, dir string, keys theme.RawKeys) (Model, *fakeThemePersister, *logtest.Sink) {
	t.Helper()

	loader, sink := themeOpenTestLoader(t)
	m, persister := newLoadPanelModel(t, dir, keys, loader)
	if !m.nomination.IsConstant() {
		t.Fatalf("fixture: keys %+v resolved to an adaptive nomination; a conversion starts from a CONSTANT", keys)
	}
	return m, persister, sink
}

// newAdaptivePanelModel is the NON-CONVERTING fixture: the same real loader and
// directory under an adaptive pair, with the light/dark gate resolved by a real OSC
// 11 reply rather than pinned — the launch shape §9.3 says a conversion never arises
// for, because the answer is already classified.
func newAdaptivePanelModel(t *testing.T, dir string, keys theme.RawKeys, reply tea.BackgroundColorMsg) (Model, *fakeThemePersister, *logtest.Sink) {
	t.Helper()

	loader, sink := themeOpenTestLoader(t)
	m, persister := newLoadPanelModel(t, dir, keys, loader)
	if m.nomination.IsConstant() {
		t.Fatalf("fixture: keys %+v resolved to a constant nomination; this fixture is the adaptive shape", keys)
	}
	m = deliverBackgroundReply(t, m, reply)
	if !m.modeResolved() {
		t.Fatal("fixture: the gate is still open after a reply; the panel must be driven against a resolved answer")
	}
	return m, persister, sink
}

// newLoadPanelModel builds the shared fixture body over a CHOSEN loader — which is
// what lets the discard-backed emission contract run through the identical wiring —
// and leaves the panel CLOSED.
//
// It stops short of the open deliberately: the light/dark reply lands within
// milliseconds of launch and the panel opens later, so a case that delivers one
// delivers it first, as production usually does. The other order IS reachable —
// §9.3's no-reply case is defined by the panel opening within milliseconds of
// launch, which is exactly the window a reply can land in with the panel already
// up — and is driven explicitly by one case below.
func newLoadPanelModel(t *testing.T, dir string, keys theme.RawKeys, loader theme.Loader) (Model, *fakeThemePersister) {
	t.Helper()

	setting, _ := theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)
	resolution, err := theme.NewLoader(nil).ResolveNomination(setting, dir)
	if err != nil {
		t.Fatalf("construction-time resolution of %+v: %v", setting, err)
	}
	persister := &fakeThemePersister{}
	m := Build(Deps{
		Lister:          fakeLister{},
		Theme:           resolution.Nomination,
		ThemeEnumerator: &realThemeEnumerator{loader: loader, dir: dir},
		ThemeKeys:       keys,
		ThemePersister:  persister,
	})
	m.termWidth, m.termHeight = arrowTermW, arrowTermH
	m.applySessions(closePanelSessions())
	return m, persister
}

// deliverBackgroundReply drives the OSC 11 reply through the live Update, exactly as
// the terminal's answer reaches the model at launch.
func deliverBackgroundReply(t *testing.T, m Model, reply tea.BackgroundColorMsg) Model {
	t.Helper()

	updated, cmd := m.Update(reply)
	if cmd != nil {
		t.Fatalf("the OSC 11 reply scheduled %T, want nothing", cmd)
	}
	return updated.(Model)
}

// openConversionPanel opens the panel through the production `t` keypress.
func openConversionPanel(t *testing.T, m Model) Model {
	t.Helper()

	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("fixture: the panel did not open")
	}
	requireCursorOn(t, m, conversionConstant)
	return m
}

// convertToSlot drives a whole conversion the way a user does: arrow to target,
// press the slot key over the constant, and answer `y`.
//
// The confirm is asserted LIVE between the two presses, so a case whose subject is
// the load does not silently start from a keypress that wrote directly.
func convertToSlot(t *testing.T, m Model, target string, press tea.KeyPressMsg) (Model, tea.Cmd) {
	t.Helper()

	m = arrowToThemeRow(t, m, target)
	m, _ = pressSlotKey(t, m, press)
	if got := m.themePanel.message; got.Kind != themeMessageConfirm {
		t.Fatalf("fixture: the slot key left the message %+v, want the confirm live over a constant", got)
	}
	return pressConfirmKey(t, m, confirmYes)
}

// themeEventRecords returns the sink records carrying the given message.
func themeEventRecords(sink *logtest.Sink, msg string) []logtest.Record {
	var found []logtest.Record
	for _, r := range sink.Records() {
		if r.Msg == msg {
			found = append(found, r)
		}
	}
	return found
}

// requireLoadedLine fails unless the sink holds EXACTLY ONE `theme: loaded`
// carrying this slug and this slot.
//
// Both attrs are asserted together because either alone passes over the mistake the
// event exists to catch: the slug alone cannot tell the assigned slot from the
// opposite one, and the slot alone cannot tell a nomination that loaded from the
// fallback that replaced it.
func requireLoadedLine(t *testing.T, sink *logtest.Sink, slug, slot string) {
	t.Helper()

	loaded := themeEventRecords(sink, "loaded")
	if len(loaded) != 1 {
		t.Fatalf("the conversion emitted %d `theme: loaded` lines, want exactly 1 (§12.3)\n%s", len(loaded), sink.Body())
	}
	if got := loaded[0].AttrString(t, "slug"); got != slug {
		t.Errorf("`theme: loaded` carries slug=%q, want %q", got, slug)
	}
	if got := loaded[0].AttrString(t, "slot"); got != slot {
		t.Errorf("`theme: loaded` carries slot=%q, want %q", got, slot)
	}
}

// requireNominationPair fails unless the model's nomination is §8.2's ADAPTIVE
// shape holding both members.
//
// It is asserted through Select rather than against the struct, which is
// constructor-only: an adaptive nomination answers the two answers with two
// DIFFERENT palettes, and a constant left in place would answer both with one.
func requireNominationPair(t *testing.T, m Model, light, dark theme.Theme) {
	t.Helper()

	if m.nomination.IsConstant() {
		t.Fatalf("the nomination is still a CONSTANT after a conversion; §8.4's load joins the pair")
	}
	if got := m.nomination.Select(false); got != light {
		t.Errorf("the nomination's light member is %s, want %s", themeLabel(got), themeLabel(light))
	}
	if got := m.nomination.Select(true); got != dark {
		t.Errorf("the nomination's dark member is %s, want %s", themeLabel(got), themeLabel(dark))
	}
}

// requireNoFallbackLine fails unless the sink holds NO `theme: fallback applied`.
//
// It is the other half of every "it resolved" assertion below: a slug that resolved
// and a slug that fell back to the same-named default are indistinguishable from the
// `loaded` line alone whenever the nomination IS the default (§8.5's values are
// §8.3's values), so the absence of the WARN is what says which happened.
func requireNoFallbackLine(t *testing.T, sink *logtest.Sink) {
	t.Helper()

	if got := themeEventRecords(sink, "fallback applied"); len(got) != 0 {
		t.Errorf("the conversion emitted %d `theme: fallback applied` line(s), want none — the slot resolved\n%s", len(got), sink.Body())
	}
}

// requireSlotCanvas fails unless the nomination's named member carries this canvas
// value.
//
// The canvas is what NAMES a palette here: each fixture drop-in carries ONE colour
// on every token (writeThemeFileForTest), chosen distinct from the other drop-ins in
// its directory and from every built-in's canvas, so comparing this one field
// identifies a palette no built-in helper can name — a drop-in's — without
// rebuilding the whole Theme.
func requireSlotCanvas(t *testing.T, m Model, slot theme.Slot, want string) {
	t.Helper()

	got := m.nomination.Select(slot == theme.SlotDark)
	if got.Canvas.Value != want {
		t.Errorf("the nomination's %v member has canvas %q, want %q", slot, got.Canvas.Value, want)
	}
}

// TestCommitSlotLoad_LoadsTheOppositeSlot: it loads the opposite slot, not the
// assigned one.
//
// §8.4: "The slot the user just assigned needs no read — §5.8's enumeration already
// holds its parse... The read that is needed is the OPPOSITE one." So a confirmed
// `l` loads the DARK slot and a confirmed `d` loads the LIGHT one, and the emitted
// line names that slot rather than the one the keypress wrote.
//
// The assigned member is the palette already in hand (the row the cursor is on),
// which is what makes the pair complete without a second parse.
func TestCommitSlotLoad_LoadsTheOppositeSlot(t *testing.T) {
	for _, tc := range []struct {
		name  string
		press tea.KeyPressMsg
		slot  string
		slug  string
	}{
		{name: "l loads the dark slot", press: slotLightPress, slot: "dark", slug: theme.DefaultDarkSlug},
		{name: "d loads the light slot", press: slotDarkPress, slot: "light", slug: theme.DefaultLightSlug},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := newConversionThemesDir(t)
			m, _, sink := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
			m = openConversionPanel(t, m)

			m, _ = convertToSlot(t, m, "nord", tc.press)

			requireLoadedLine(t, sink, tc.slug, tc.slot)
			assigned := testBuiltinTheme(t, "nord")
			loaded := testBuiltinTheme(t, tc.slug)
			if tc.slot == "dark" {
				requireNominationPair(t, m, assigned, loaded)
			} else {
				requireNominationPair(t, m, loaded, assigned)
			}
		})
	}
}

// TestCommitSlotLoad_UntouchedSlotIsTheShippedDefault: it resolves an untouched
// slot from the embedded set.
//
// §8.4: "An untouched slot holds a shipped default (§8.3), so it resolves from the
// embedded set and never touches the themes directory (§8.4's ordering rule) — cheap
// and infallible."
//
// The directory stages a DROP-IN NAMED FOR THE DEFAULT, so the slot has a candidate
// in the enumeration as well as in the embedded set. §5.4 reserves built-in slugs and
// the enumeration rejects the file for it, so consulting the embedded set is what
// makes the slot resolve AT ALL: a load that reached only the retained entries would
// find the rejection and fall back, which is what the absent fallback line below
// distinguishes.
//
// That absence carries the other half too: an untouched slot is NOT a fallback
// (§8.5's "no new mechanism"), and the two outcomes are otherwise indistinguishable
// here because the fallback's value IS the shipped default's.
func TestCommitSlotLoad_UntouchedSlotIsTheShippedDefault(t *testing.T) {
	dir := newConversionThemesDir(t)
	writeThemeFileForTest(t, dir, theme.DefaultLightSlug+".theme", "#202020")
	m, _, sink := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
	m = openConversionPanel(t, m)

	m, _ = convertToSlot(t, m, "nord", slotDarkPress)

	requireLoadedLine(t, sink, theme.DefaultLightSlug, "light")
	requireNoFallbackLine(t, sink)
	requireNominationPair(t, m, testLightTheme(t), testBuiltinTheme(t, "nord"))
}

// TestCommitSlotLoad_StaleSlotFromEnumeration: it resolves a stale slot from the
// retained enumeration.
//
// §8.4: "A stale hand-edited slot (§8.2, where a `theme`-wins file's slots were
// invisible until the constant cleared) resolves from the panel's RETAINED
// ENUMERATION (§5.8)... Only a slug the enumeration has no entry for falls through
// to the embedded set."
//
// The file is REWRITTEN AFTER THE PANEL OPENED, which is what separates "resolved
// from the retained parse" from "resolved from a file that happens to still say the
// same thing": §5.8 retains the enumeration for the panel's lifetime, so the palette
// that joins the nomination is the one the row on screen was built from and not the
// bytes now on disk.
func TestCommitSlotLoad_StaleSlotFromEnumeration(t *testing.T) {
	t.Run("a stale slug resolves from the panel's parse", func(t *testing.T) {
		const opened, edited = "#202020", "#303030"
		dir := newConversionThemesDir(t)
		writeThemeFileForTest(t, dir, "ghostly.theme", opened)
		keys := theme.RawKeys{Theme: conversionConstant, Light: "ghostly"}
		m, _, sink := newConversionPanelModel(t, dir, keys)
		m = openConversionPanel(t, m)
		writeThemeFileForTest(t, dir, "ghostly.theme", edited)

		m, _ = convertToSlot(t, m, "nord", slotDarkPress)

		requireLoadedLine(t, sink, "ghostly", "light")
		requireNoFallbackLine(t, sink)
		requireSlotCanvas(t, m, theme.SlotLight, opened)
	})

	t.Run("a slug the enumeration has no entry for falls through to the embedded set", func(t *testing.T) {
		dir := newConversionThemesDir(t)
		keys := theme.RawKeys{Theme: conversionConstant, Light: "nord"}
		m, _, sink := newConversionPanelModel(t, dir, keys)
		m = openConversionPanel(t, m)

		m, _ = convertToSlot(t, m, theme.DefaultDarkSlug, slotDarkPress)

		requireLoadedLine(t, sink, "nord", "light")
		requireNoFallbackLine(t, sink)
		requireNominationPair(t, m, testBuiltinTheme(t, "nord"), testDarkTheme(t))
	})
}

// TestCommitSlotLoad_UnresolvableTakesTheModeMatchedFallback: it falls back per slot
// when unresolvable.
//
// §8.4: "if it is in neither it is unresolvable and takes §8.5's fallback" — and
// §8.5's table is MODE-MATCHED: `theme_light` falls to `tokyo-night-day`,
// `theme_dark` to `tokyo-night`. Both directions are driven, because a single fixed
// fallback (the rejected alternative) passes either one alone.
func TestCommitSlotLoad_UnresolvableTakesTheModeMatchedFallback(t *testing.T) {
	for _, tc := range []struct {
		name     string
		keys     theme.RawKeys
		press    tea.KeyPressMsg
		slot     theme.Slot
		slotAttr string
		slug     string
	}{
		{
			name:     "an unresolvable light slot falls to the light default",
			keys:     theme.RawKeys{Theme: conversionConstant, Light: "nope"},
			press:    slotDarkPress,
			slot:     theme.SlotLight,
			slotAttr: "light",
			slug:     theme.DefaultLightSlug,
		},
		{
			name:     "an unresolvable dark slot falls to the dark default",
			keys:     theme.RawKeys{Theme: conversionConstant, Dark: "nope"},
			press:    slotLightPress,
			slot:     theme.SlotDark,
			slotAttr: "dark",
			slug:     theme.DefaultDarkSlug,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := newConversionThemesDir(t)
			m, _, sink := newConversionPanelModel(t, dir, tc.keys)
			m = openConversionPanel(t, m)

			m, _ = convertToSlot(t, m, "nord", tc.press)

			requireLoadedLine(t, sink, tc.slug, tc.slotAttr)
			requireSlotCanvas(t, m, tc.slot, testBuiltinTheme(t, tc.slug).Canvas.Value)
		})
	}
}

// TestCommitSlotLoad_NoDirectoryRead: it reads no directory at commit.
//
// §8.4: "No commit-time directory read. Issuing one would produce a THIRD parse of
// the same slug — neither construction's nor the panel's — that can disagree with the
// row the user is looking at, reintroducing exactly the staleness split §5.8 exists
// to close."
//
// The directory is REMOVED after the panel opened, so a read would fail rather than
// merely re-parse — and the two branches fail DIFFERENTLY, which is what makes the
// assertion non-vacuous on both: a stale slug would come back `not found` and land on
// the §8.5 fallback instead of on its own palette, while an untouched slot would…
// still resolve, from the embedded set, which is exactly §8.4's "never touches the
// themes directory" claim and is asserted here as the second branch.
func TestCommitSlotLoad_NoDirectoryRead(t *testing.T) {
	t.Run("a stale slot still resolves from the retained parse", func(t *testing.T) {
		const value = "#202020"
		dir := newConversionThemesDir(t)
		writeThemeFileForTest(t, dir, "ghostly.theme", value)
		keys := theme.RawKeys{Theme: conversionConstant, Light: "ghostly"}
		m, _, sink := newConversionPanelModel(t, dir, keys)
		m = openConversionPanel(t, m)
		removeThemesDirForTest(t, dir)

		m, _ = convertToSlot(t, m, "nord", slotDarkPress)

		requireLoadedLine(t, sink, "ghostly", "light")
		requireNoFallbackLine(t, sink)
		requireSlotCanvas(t, m, theme.SlotLight, value)
	})

	t.Run("an untouched slot still resolves the shipped default", func(t *testing.T) {
		dir := newConversionThemesDir(t)
		m, _, sink := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
		m = openConversionPanel(t, m)
		removeThemesDirForTest(t, dir)

		m, _ = convertToSlot(t, m, "nord", slotDarkPress)

		requireLoadedLine(t, sink, theme.DefaultLightSlug, "light")
		requireNoFallbackLine(t, sink)
		requireNominationPair(t, m, testLightTheme(t), testBuiltinTheme(t, "nord"))
	})
}

// removeThemesDirForTest deletes the themes directory and asserts it is gone, so a
// "no directory read" case cannot pass because the removal silently failed.
func removeThemesDirForTest(t *testing.T, dir string) {
	t.Helper()

	if err := os.RemoveAll(dir); err != nil {
		t.Fatalf("remove themes dir %q: %v", dir, err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("themes dir %q still stats as %v; the no-read assertion would be vacuous", dir, err)
	}
}

// TestCommitSlotLoad_EmitsLoadedOncePerConversion: it emits one undeduplicated
// loaded line.
//
// §12.3 catalogues `theme: loaded` as INFO and pointedly does NOT deduplicate it,
// unlike its WARN neighbours: it is per LOAD rather than per condition, and the
// commit-time entry is "the same event at a different cadence" (see
// EventLogger.Loaded).
//
// TWO CONVERSIONS IN ONE PANEL SESSION are reachable because `Enter` returns the
// setting to a constant (§9.2 clears both slots), so the second `d` raises the
// confirm again. Both conversions load the SAME slug into the SAME slot, which is
// exactly the shape a dedup key would collapse — so two lines is the assertion, not
// an incidental count.
func TestCommitSlotLoad_EmitsLoadedOncePerConversion(t *testing.T) {
	dir := newConversionThemesDir(t)
	m, _, sink := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
	m = openConversionPanel(t, m)

	m, _ = convertToSlot(t, m, "nord", slotDarkPress)
	requireLoadedLine(t, sink, theme.DefaultLightSlug, "light")

	// Back to a constant, then convert again — the same slug into the same slot.
	m, _ = pressCommitKey(t, m)
	requireConstantKeys(t, m, "nord")
	m, _ = convertToSlot(t, m, "nord", slotDarkPress)
	requireNominationPair(t, m, testLightTheme(t), testBuiltinTheme(t, "nord"))

	loaded := themeEventRecords(sink, "loaded")
	if len(loaded) != 2 {
		t.Fatalf("two conversions emitted %d `theme: loaded` lines, want 2 — the event is NOT deduplicated (§12.3)\n%s", len(loaded), sink.Body())
	}
	for i, record := range loaded {
		if got := record.AttrString(t, "slug"); got != theme.DefaultLightSlug {
			t.Errorf("`theme: loaded` %d carries slug=%q, want %q", i+1, got, theme.DefaultLightSlug)
		}
		if got := record.AttrString(t, "slot"); got != "light" {
			t.Errorf("`theme: loaded` %d carries slot=%q, want %q", i+1, got, "light")
		}
	}
}

// TestCommitSlotLoad_LoadedNamesTheFallbackSlug: it names the fallback in loaded and
// the nomination in fallback applied.
//
// §12.3: "When a nomination is unloadable it fires for the fallback too, carrying the
// FALLBACK's slug — otherwise `theme: fallback applied` and `theme: loaded` both name
// the slug that FAILED, and a `grep \"theme:\"` on a broken install cannot answer
// which palette is actually rendering."
//
// So the two lines name DIFFERENT slugs on the same commit, which is the whole point
// and is asserted as such rather than as two independent line checks.
//
// EXACTLY ONE `fallback applied` is the §12.3 dedup rather than a single resolution:
// two of them resolve the broken light slot on this keypress, in the order
// commitSlot → recompute → loadNewlyLiveSlot, so the recompute's badge re-resolution
// emits the line and the load's own ResolveSlot is suppressed as a repeat sighting of
// the same (slug, reason). Which SITE emits it is the ordering's doing and could
// swap; that there is one line, naming the nomination while `loaded` names the
// fallback, is the contract asserted below.
func TestCommitSlotLoad_LoadedNamesTheFallbackSlug(t *testing.T) {
	dir := newConversionThemesDir(t)
	keys := theme.RawKeys{Theme: conversionConstant, Light: "nope"}
	m, _, sink := newConversionPanelModel(t, dir, keys)
	m = openConversionPanel(t, m)

	m, _ = convertToSlot(t, m, "nord", slotDarkPress)

	requireLoadedLine(t, sink, theme.DefaultLightSlug, "light")
	applied := themeEventRecords(sink, "fallback applied")
	if len(applied) != 1 {
		t.Fatalf("the conversion emitted %d `theme: fallback applied` lines, want exactly 1\n%s", len(applied), sink.Body())
	}
	if got := applied[0].AttrString(t, "slug"); got != "nope" {
		t.Errorf("`theme: fallback applied` carries slug=%q, want the NOMINATION that failed (%q)", got, "nope")
	}
	if got := applied[0].AttrString(t, "slot"); got != "light" {
		t.Errorf("`theme: fallback applied` carries slot=%q, want %q", got, "light")
	}
	if got := applied[0].AttrString(t, "reason"); got != string(theme.ReasonNotFound) {
		t.Errorf("`theme: fallback applied` carries reason=%q, want %q", got, theme.ReasonNotFound)
	}
	if loaded := themeEventRecords(sink, "loaded")[0].AttrString(t, "slug"); loaded == applied[0].AttrString(t, "slug") {
		t.Errorf("both lines name %q; the pair is only useful because one names the theme that BROKE and the other the palette that RENDERED (§12.3)", loaded)
	}
	requireSlotCanvas(t, m, theme.SlotLight, testLightTheme(t).Canvas.Value)
}

// TestCommitSlotLoad_NonConvertingCommitIsSilent: it emits nothing when nothing
// converts.
//
// §8.4 puts the load on the constant → adaptive transition and nowhere else: a
// `d`/`l` over a pair changes which slug a LIVE slot names, and `Enter` collapses to
// a constant whose palette is already the previewed one. Both members are in hand
// either way, so there is no load to announce and no member to resolve.
//
// The unchanged NOMINATION is asserted alongside the silence because the two fail
// independently: a load that ran but emitted nothing would move the pair, and one
// that emitted without resolving would not.
func TestCommitSlotLoad_NonConvertingCommitIsSilent(t *testing.T) {
	adaptive := theme.RawKeys{Light: conversionConstant, Dark: "nord"}

	for _, tc := range []struct {
		name string
		run  func(*testing.T, Model) (Model, tea.Cmd)
	}{
		{
			name: "d over a pair",
			run:  func(t *testing.T, m Model) (Model, tea.Cmd) { return pressSlotKey(t, m, slotDarkPress) },
		},
		{
			name: "l over a pair",
			run:  func(t *testing.T, m Model) (Model, tea.Cmd) { return pressSlotKey(t, m, slotLightPress) },
		},
		{
			name: "Enter over a pair",
			run:  func(t *testing.T, m Model) (Model, tea.Cmd) { return pressCommitKey(t, m) },
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := newConversionThemesDir(t)
			m, persister, sink := newAdaptivePanelModel(t, dir, adaptive, lightBg)
			m = pressThemeKey(t, m)
			if !m.themePanel.open {
				t.Fatal("fixture: the panel did not open")
			}
			nomination, mode := m.nomination, m.canvasMode

			m, _ = tc.run(t, m)

			if len(persister.slugs) != 1 {
				t.Fatalf("fixture: the keypress wrote %v, want exactly one commit so the silence is not vacuous", persister.slugs)
			}
			if got := themeEventRecords(sink, "loaded"); len(got) != 0 {
				t.Errorf("a non-converting commit emitted %d `theme: loaded` line(s), want none (§8.4)\n%s", len(got), sink.Body())
			}
			if m.nomination != nomination {
				t.Error("a non-converting commit moved the nomination; both members were already in hand")
			}
			if m.canvasMode != mode {
				t.Errorf("a non-converting commit moved the light/dark answer to %v, want the classified %v — an adaptive launch already has one (§9.3)", m.canvasMode, mode)
			}
		})
	}

	t.Run("Enter over a constant", func(t *testing.T) {
		dir := newConversionThemesDir(t)
		m, persister, sink := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
		m = openConversionPanel(t, m)
		nomination, mode := m.nomination, m.canvasMode

		m = arrowToThemeRow(t, m, "nord")
		m, _ = pressCommitKey(t, m)

		requireCommitted(t, persister, "nord")
		requireConstantKeys(t, m, "nord")
		if got := themeEventRecords(sink, "loaded"); len(got) != 0 {
			t.Errorf("`Enter` emitted %d `theme: loaded` line(s), want none (§8.4)\n%s", len(got), sink.Body())
		}
		if m.nomination != nomination {
			t.Error("`Enter` moved the nomination; it commits the palette already previewing")
		}
		if m.canvasMode != mode {
			t.Errorf("`Enter` moved the light/dark answer to %v, want the untouched %v", m.canvasMode, mode)
		}
	})
}

// TestCommitSlotLoad_FailedCommitLoadsNothing: it emits nothing on a failed write.
//
// The load is specified to run "after a SUCCESSFUL slot commit" (§8.4), and the
// early return that enforces it is the one confirmSlotAssignment takes on
// commitSlot's error. Run past it, the load would resolve off keys the mirror never
// touched — still holding the constant, so both slot slugs are empty — and would
// report a commit-time `theme: loaded` for a write that never landed.
//
// That is what the assertions below distinguish: an un-guarded implementation emits
// a `loaded` (and a `fallback applied` for the empty slug), so ZERO lines is a
// statement about the guard rather than about a keypress that did nothing.
func TestCommitSlotLoad_FailedCommitLoadsNothing(t *testing.T) {
	dir := newConversionThemesDir(t)
	m, persister, sink := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
	m = openConversionPanel(t, m)
	nomination, mode, keys := m.nomination, m.canvasMode, m.themeKeys
	persister.err = errThemeCommitFailed

	m, _ = convertToSlot(t, m, "nord", slotDarkPress)

	requireSlotCommits(t, persister, slotCommit{slug: "nord", slot: prefs.SlotDark})
	if got := themeEventRecords(sink, "loaded"); len(got) != 0 {
		t.Errorf("a failed write emitted %d `theme: loaded` line(s), want none — nothing became live\n%s", len(got), sink.Body())
	}
	if got := themeEventRecords(sink, "fallback applied"); len(got) != 0 {
		t.Errorf("a failed write emitted %d `theme: fallback applied` line(s), want none\n%s", len(got), sink.Body())
	}
	if m.nomination != nomination {
		t.Error("a failed write moved the nomination; §9.13 leaves the constant standing in memory")
	}
	if !m.nomination.IsConstant() {
		t.Error("a failed write left an adaptive nomination; the constant is still what is persisted (§9.13)")
	}
	if m.canvasMode != mode {
		t.Errorf("a failed write moved the light/dark answer to %v, want the untouched %v", m.canvasMode, mode)
	}
	if m.themeKeys != keys {
		t.Errorf("a failed write left keys %+v, want the untouched %+v", m.themeKeys, keys)
	}

	// The control: the same fixture with the write LANDING does load and does emit,
	// so the silence above is the failure path rather than an unwired one.
	landed, _, landedSink := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
	landed = openConversionPanel(t, landed)
	landed, _ = convertToSlot(t, landed, "nord", slotDarkPress)
	requireLoadedLine(t, landedSink, theme.DefaultLightSlug, "light")
	if landed.nomination.IsConstant() {
		t.Fatal("positive control: a landed write left the nomination a constant")
	}
}

// TestCommitSlotLoad_ActiveThemeUnchanged: it completes the nomination without
// changing the active member.
//
// §9.2: "Committing to a non-active slot changes nothing on screen... A commit is a
// WRITE, NOT A NAVIGATION — the panel keeps previewing whatever the cursor is on."
// So the pair is completed while the frame keeps painting the previewed palette, and
// ApplyTheme is never called with the newly-loaded one.
//
// The FRAME is asserted through the canvas sequence rather than byte-for-byte,
// because a successful commit legitimately moves the badges and the row set (§9.2's
// recompute) — the canvas is the part a re-theme would move and the recompute cannot.
func TestCommitSlotLoad_ActiveThemeUnchanged(t *testing.T) {
	dir := newConversionThemesDir(t)
	m, _, _ := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
	m = openConversionPanel(t, m)
	m = arrowToThemeRow(t, m, "nord")
	previewed := m.activeTheme

	m, _ = pressSlotKey(t, m, slotDarkPress)
	m, _ = pressConfirmKey(t, m, confirmYes)

	if m.activeTheme != previewed {
		t.Errorf("the conversion rendered %s, want the previewed %s — a commit is a WRITE, not a navigation (§9.2)", themeLabel(m.activeTheme), themeLabel(previewed))
	}
	requireNominationPair(t, m, testLightTheme(t), previewed)

	frame := m.View().Content
	if seq := canvasSeq(t, previewed); !strings.Contains(frame, seq) {
		t.Errorf("the composed frame no longer paints the previewed canvas %q", seq)
	}
	if seq := canvasSeq(t, testLightTheme(t)); strings.Contains(frame, seq) {
		t.Errorf("the composed frame paints the newly-LOADED slot's canvas %q; the load joins the nomination, never the screen", seq)
	}
}

// TestCommitSlotLoad_SharesTheResolverBody: it agrees with the badge resolution.
//
// §8.4's commit-time load and §9.2's badge re-resolution answer for the SAME slug
// against the SAME retained enumeration, so they must share one rule body — the
// charset check, the embedded-set-first ordering and §8.5's per-slot fallback
// included. Two independently-authored ladders would let the panel mark one theme
// while the nomination held another's palette, which is the split §5.8 exists to
// close.
//
// The table walks every rung that can differ: a built-in, a drop-in, a slug nothing
// answers to, and one the charset refuses outright.
func TestCommitSlotLoad_SharesTheResolverBody(t *testing.T) {
	dir := newConversionThemesDir(t)
	writeThemeFileForTest(t, dir, "ghostly.theme", "#202020")
	loader, _ := themeOpenTestLoader(t)
	enumerator := &realThemeEnumerator{loader: loader, dir: dir}
	enumeration, _ := enumerator.Open(theme.RawKeys{})

	for _, slug := range []string{"nord", "ghostly", conversionConstant, "nope", "../escape"} {
		t.Run(slug, func(t *testing.T) {
			one, err := enumerator.ResolveSlot(enumeration, theme.SlotLight, slug)
			if err != nil {
				t.Fatalf("ResolveSlot(%q) returned %v", slug, err)
			}
			whole, err := enumerator.Resolve(enumeration, theme.Setting{Light: slug, Dark: theme.DefaultDarkSlug})
			if err != nil {
				t.Fatalf("Resolve(%q) returned %v", slug, err)
			}
			badge := whole.Slots[0]
			if badge.Slot != theme.SlotLight {
				t.Fatalf("the badge path's first slot is %v, want the light one", badge.Slot)
			}
			if one != badge {
				t.Errorf("ResolveSlot returned %+v and the badge path %+v for %q; the two share one rule body (§8.4)", one, badge, slug)
			}
		})
	}
}

// TestCommitSlotLoad_DiscardSilencesLoaded: it is silent on the discard logger.
//
// §12.3: "Emission is controlled by an injected logger, not by the loader deciding."
// A diagnose-shaped caller — `portal doctor`, `portal theme export`, capturetool — is
// constructed with log.Discard() and writes nothing at all, and the commit-time
// cadence is one more event that has to inherit that rather than emit unconditionally.
//
// The PROCESS handler captures everything for the test's duration, so a record
// reaching the sink would mean the discard logger had been bypassed rather than
// merely being quiet. The capture-backed control runs the identical conversion, so
// the silence cannot be a conversion that never happened.
func TestCommitSlotLoad_DiscardSilencesLoaded(t *testing.T) {
	control := func(t *testing.T) {
		t.Helper()
		dir := newConversionThemesDir(t)
		m, _, sink := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
		m = openConversionPanel(t, m)
		m, _ = convertToSlot(t, m, "nord", slotDarkPress)
		requireLoadedLine(t, sink, theme.DefaultLightSlug, "light")
		if m.nomination.IsConstant() {
			t.Fatal("control: the conversion did not complete the pair")
		}
	}
	control(t)

	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	dir := newConversionThemesDir(t)
	m, _ := newLoadPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant}, theme.NewLoader(theme.NewEventLogger(log.Discard())))
	m = openConversionPanel(t, m)

	m, _ = convertToSlot(t, m, "nord", slotDarkPress)

	if m.nomination.IsConstant() {
		t.Fatal("the discard-backed conversion did not complete the pair, so the silence is vacuous")
	}
	if lines := sink.Lines(); len(lines) != 0 {
		t.Errorf("a discard-backed loader emitted %d record(s) on a conversion, want none:\n%s", len(lines), sink.Body())
	}
}

// TestCommitSlotLoad_ConversionUsesTheRetainedAnswer: it classifies the retained
// background on conversion.
//
// §9.3: "`restore.go` issues the OSC 11 query from `Init` REGARDLESS — it needs the
// original background to restore on exit, independent of detection. The terminal's
// background is therefore already in hand; the detection decision only ever governed
// whether to CLASSIFY AND USE it."
//
// The CLOSE is what makes the answer observable end to end: task 8-10 re-resolves
// persisted state and selects the in-force member from this answer, so a light
// terminal must land on the light slot and a dark one on the dark slot with no
// change of its own.
//
// The fixture never pins canvasMode: it is the dark zero value until the conversion
// establishes it, so a light terminal landing on light is the conversion's doing.
func TestCommitSlotLoad_ConversionUsesTheRetainedAnswer(t *testing.T) {
	// inForce names the slug the close must land on: the newly-loaded LIGHT slot in
	// a light terminal, and the DARK slot the keypress just assigned in a dark one.
	for _, tc := range []struct {
		name    string
		reply   tea.BackgroundColorMsg
		want    canvasAppearance
		inForce string
	}{
		{name: "a light terminal", reply: lightBg, want: appearanceLightCanvas, inForce: theme.DefaultLightSlug},
		{name: "a dark terminal", reply: darkBg, want: appearanceDarkCanvas, inForce: "nord"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := newConversionThemesDir(t)
			m, _, _ := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
			m = deliverBackgroundReply(t, m, tc.reply)
			if m.canvasMode != appearanceDarkCanvas {
				t.Fatalf("fixture: the constant launch starts on %v, want the standing dark fallback so the answer is the conversion's", m.canvasMode)
			}
			m = openConversionPanel(t, m)

			m, _ = convertToSlot(t, m, "nord", slotDarkPress)

			if m.canvasMode != tc.want {
				t.Errorf("the conversion left the light/dark answer %v, want %v — it classifies the reply already in hand (§9.3)", m.canvasMode, tc.want)
			}

			// Task 8-10's close selects the in-force member from that answer.
			closed := closeThemePanelForTest(t, m)
			want := testBuiltinTheme(t, tc.inForce)
			if closed.activeTheme != want {
				t.Errorf("the close landed on %s, want %s — the in-force member is the one the answer names (§9.2)", themeLabel(closed.activeTheme), themeLabel(want))
			}
		})
	}

	// The third reachable ordering, between the two above and the no-reply case: the
	// reply lands with the panel ALREADY OPEN. §9.3's no-reply case is defined by the
	// panel being opened within milliseconds of launch, so that window is precisely
	// where a reply can arrive late enough to find the slide-over up.
	//
	// It must classify exactly as an early reply does, and the reason it does is
	// structural: the BackgroundColorMsg arm retains the arrival and the verdict
	// UNCONDITIONALLY, in the cross-view switch ahead of every page and panel route,
	// so no open surface can swallow or reorder it.
	t.Run("a reply that lands with the panel already open", func(t *testing.T) {
		dir := newConversionThemesDir(t)
		m, _, _ := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
		m = openConversionPanel(t, m)
		if m.bgReplyArrived {
			t.Fatal("fixture: a reply arrived before the open, so the late-arrival order is not being driven")
		}

		m = deliverBackgroundReply(t, m, lightBg)

		if !m.bgReplyArrived {
			t.Fatal("the reply was not retained with the panel open; the arm must sit ahead of every panel route")
		}
		if m.canvasMode != appearanceDarkCanvas {
			t.Fatalf("fixture: the reply moved the answer to %v before any conversion, want the constant's standing dark fallback", m.canvasMode)
		}

		m, _ = convertToSlot(t, m, "nord", slotDarkPress)

		if m.canvasMode != appearanceLightCanvas {
			t.Errorf("the conversion left the light/dark answer %v, want light — a reply retained with the panel open classifies exactly as an early one (§9.3)", m.canvasMode)
		}
		closed := closeThemePanelForTest(t, m)
		if want := testLightTheme(t); closed.activeTheme != want {
			t.Errorf("the close landed on %s, want %s", themeLabel(closed.activeTheme), themeLabel(want))
		}
	})
}

// TestCommitSlotLoad_ConversionIssuesNoQuery: it issues no new query on conversion.
//
// §9.3: "Converting to adaptive mid-session starts using an answer that already
// arrived: no new query, no race, no gate." §8.8's gate resolves exactly once, and a
// conversion is not a second resolution — so the GATE is untouched and the answer is
// recorded beside it.
//
// The gate's own appearance staying DARK on a light terminal is the assertion that
// matters: it is the standing no-answer fallback of a constant that never asked
// (appearanceGate.appearance), so a conversion reading it would answer "dark" for
// every light terminal in the product.
func TestCommitSlotLoad_ConversionIssuesNoQuery(t *testing.T) {
	dir := newConversionThemesDir(t)
	m, _, _ := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
	m = deliverBackgroundReply(t, m, lightBg)
	m = openConversionPanel(t, m)
	gate, original, arrived := m.gate, m.originalBg, m.bgReplyArrived

	m, cmd := convertToSlot(t, m, "nord", slotDarkPress)

	if cmd != nil {
		t.Errorf("the conversion scheduled %T; §9.3 needs no new query and arms no new gate", cmd)
	}
	if m.gate != gate {
		t.Errorf("the conversion moved the gate to %+v, want the untouched %+v — §8.8's gate resolves exactly once", m.gate, gate)
	}
	if m.originalBg != original || m.bgReplyArrived != arrived {
		t.Errorf("the conversion moved the retained reply to (%q, %v), want the untouched (%q, %v)", m.originalBg, m.bgReplyArrived, original, arrived)
	}
	if m.canvasMode != appearanceLightCanvas {
		t.Fatalf("the conversion answered %v, want light", m.canvasMode)
	}
	if m.gate.appearance == m.canvasMode {
		t.Error("the gate's appearance now equals the answer, so this fixture cannot tell a classified reply from the gate's standing dark fallback")
	}
}

// TestCommitSlotLoad_ConversionWithNoReplyIsDark: it falls back to dark when no reply
// landed.
//
// §9.3: "If the reply has not landed (requiring the panel to be opened within
// milliseconds of launch) it falls to DARK, the same rule as everywhere else."
//
// And the reply arriving AFTERWARDS still does not re-theme: §8.8's single-resolution
// rule is untouched by the conversion, so a late answer is consumed for
// restore-on-exit and changes nothing that is painted.
func TestCommitSlotLoad_ConversionWithNoReplyIsDark(t *testing.T) {
	dir := newConversionThemesDir(t)
	m, _, _ := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
	if m.bgReplyArrived {
		t.Fatal("fixture: a reply has already arrived, so the no-reply case is not being driven")
	}
	m = openConversionPanel(t, m)

	m, _ = convertToSlot(t, m, "nord", slotDarkPress)

	if m.canvasMode != appearanceDarkCanvas {
		t.Errorf("a conversion with no reply answered %v, want dark (§8.8's no-answer fallback)", m.canvasMode)
	}
	active, nomination := m.activeTheme, m.nomination

	late := deliverBackgroundReply(t, m, lightBg)

	if late.canvasMode != appearanceDarkCanvas {
		t.Errorf("a late reply moved the answer to %v; §8.8 resolves exactly once", late.canvasMode)
	}
	if late.activeTheme != active {
		t.Errorf("a late reply re-themed to %s, want the untouched %s", themeLabel(late.activeTheme), themeLabel(active))
	}
	if late.nomination != nomination {
		t.Error("a late reply moved the nomination")
	}
}

// TestCommitSlotLoad_ConversionDoesNotMoveStartupCanvasHex: it never moves the
// startup canvas hex on a conversion.
//
// §11.4's anchor is captured ONCE, at gate resolution, and
// RestoreTerminalBackground's canvas-echo guard compares against it on exit. This is
// the one path where a mistake re-sticks a colour in the user's terminal AFTER
// Portal exits, and tasks 4-2 / 4-5 / 8-9 guard it only against SWAPS — which a
// conversion is not.
//
// The fixture arrows OFF the constant before converting, so the anchor, the previewed
// palette and the newly-loaded one are three different values: a re-capture from
// either of the other two moves the hex, and the assertions name both.
func TestCommitSlotLoad_ConversionDoesNotMoveStartupCanvasHex(t *testing.T) {
	for _, tc := range []struct {
		name    string
		reply   tea.BackgroundColorMsg
		deliver bool
	}{
		{name: "a light terminal", reply: lightBg, deliver: true},
		{name: "a dark terminal", reply: darkBg, deliver: true},
		{name: "no reply at all", deliver: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := newConversionThemesDir(t)
			m, _, _ := newConversionPanelModel(t, dir, theme.RawKeys{Theme: conversionConstant})
			if tc.deliver {
				m = deliverBackgroundReply(t, m, tc.reply)
			}
			anchor := m.startupCanvasHex
			if anchor != conversionConstantValue {
				t.Fatalf("fixture: the startup canvas hex is %q, want the constant's %q", anchor, conversionConstantValue)
			}
			m = openConversionPanel(t, m)

			m, _ = convertToSlot(t, m, "nord", slotDarkPress)

			if m.startupCanvasHex != anchor {
				t.Errorf("the conversion moved the startup canvas hex to %q, want the byte-identical %q — §11.4's anchor is frozen at gate resolution", m.startupCanvasHex, anchor)
			}
			if m.startupCanvasHex == m.activeTheme.Canvas.Value {
				t.Error("the anchor now equals the PREVIEWED canvas, so this fixture cannot detect a re-capture from the active theme")
			}
			if m.startupCanvasHex == testLightTheme(t).Canvas.Value {
				t.Error("the anchor now equals the newly-LOADED slot's canvas, so this fixture cannot detect a re-capture from the load")
			}
		})
	}

	// The structural half: §11.4's anchor is captured inside syncResolvedMode, so a
	// conversion routing its answer through that function would re-anchor the guard
	// to a canvas the startup window never painted. The behavioural cases above
	// cover the values; this covers the ROUTE, which is what a later edit is most
	// likely to reach for when it wants "the mode set properly".
	t.Run("the classification does not route through syncResolvedMode", func(t *testing.T) {
		callers := themePanelSeamCallers(t, "syncResolvedMode")
		if !slices.Contains(callers, "New") {
			t.Fatalf("syncResolvedMode is called from %v; the scan found no known caller, so its absence proves nothing", callers)
		}
		if want := []string{"New", "Update", "armAppearanceDetection"}; !slices.Equal(callers, want) {
			t.Errorf("syncResolvedMode is called from %v, want exactly %v — §11.4's anchor is captured there, so the conversion records its answer directly", callers, want)
		}
	})
}

// TestCommitSlotLoad_BrokenBuiltinDegrades: it moves nothing when the load reports
// §7.6's fatal.
//
// The only error the seam can return is the should-never-happen state of a binary
// whose embedded set cannot supply a fallback, and the panel's shared policy
// (applyInForceTheme) is to DEGRADE rather than escalate: a settings surface must not
// become the route by which a broken binary quits Portal mid-session.
//
// Degrading has to mean NOTHING MOVES, which is why the return sits ahead of both
// assignments: a zero SlotResolution joined into the nomination would put an empty
// palette in a live slot, and lipgloss resolves that through its no-colour sentinel —
// a silently colourless render on the next close, with no error anywhere.
//
// The state is unreachable through a real loader (§7.6's build-time guarantee), so
// the seam is the stub that can produce it.
func TestCommitSlotLoad_BrokenBuiltinDegrades(t *testing.T) {
	rows := arrowValidRows(4)
	persisted, target := rows[0].Slug, rows[2].Slug
	persister := &fakeThemePersister{}
	deps := newArrowPanelDeps(t, rows, persisted)
	deps.ThemePersister = persister
	m := openCommitPanel(t, deps, PageSessions, persisted)
	seam, ok := m.themeEnumerator.(*stubThemeEnumerator)
	if !ok {
		t.Fatalf("fixture: the seam is %T, want the recording stub", m.themeEnumerator)
	}
	m = arrowToThemeRow(t, m, target)
	nomination, mode, active := m.nomination, m.canvasMode, m.activeTheme
	seam.err = errThemeResolveFatal

	m, _ = pressSlotKey(t, m, slotDarkPress)
	m, cmd := pressConfirmKey(t, m, confirmYes)

	if len(seam.slotLoads) != 1 {
		t.Fatalf("the conversion asked for %d slot load(s), want 1 — the degrade must be the seam's answer rather than a load that never ran", len(seam.slotLoads))
	}
	if m.nomination != nomination {
		t.Errorf("the fatal left the nomination %+v, want the untouched %+v — a degrade moves nothing", m.nomination, nomination)
	}
	if m.canvasMode != mode {
		t.Errorf("the fatal moved the light/dark answer to %v, want the untouched %v", m.canvasMode, mode)
	}
	if m.activeTheme != active {
		t.Errorf("the fatal rendered %s, want the untouched %s", themeLabel(m.activeTheme), themeLabel(active))
	}
	if cmd != nil {
		t.Errorf("the fatal scheduled %T; the panel degrades rather than quitting Portal mid-session", cmd)
	}
	if !m.themePanel.open {
		t.Error("the fatal closed the panel; `Esc` is the only way out (§9.2)")
	}
}
