package tui

import (
	"errors"
	"go/ast"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// The picker idiom's `d` and `l`: they COMMIT ONE SLOT — `theme_dark`/`theme_light` = the
// cursor's slug, the constant cleared in the same atomic prefs write — and the
// panel STAYS OPEN.
//
// This is the panel's genuinely novel half (the reference frames records that assigning a
// theme to a light/dark slot from inside a picker was found in no surveyed tool), and
// three properties make it more than a second `Enter`:
//
//   - THE OTHER SLOT SURVIVES. A slot save leaves the other slot's raw value
//     exactly as it was, which is what makes `d` then `l` on one row produce
//     the row-rendering rule's `● both` in two keypresses — the likely path for a user wanting
//     "this theme everywhere" without realising `Enter` is the idiom for it.
//   - IT IS THE SHARPEST CASE OF "A COMMIT IS A WRITE, NOT A NAVIGATION".
//     Previewing a light theme in a dark terminal and pressing `l` writes the
//     light slot while the resolved-active theme is still the dark slot, so
//     absolutely nothing changes on screen and the only feedback is the badge.
//   - A CONSTANT MAKES IT INERT, DELIBERATELY. The picker idiom gates `d`/`l` over a constant
//     behind a confirm because a silent constant-clear is exactly the
//     loss that confirm exists to prevent, so the interim behaviour here writes
//     NOTHING rather than writing directly.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// slotDarkPress / slotLightPress are the picker idiom's two slot-commit presses.
var (
	slotDarkPress  = tea.KeyPressMsg{Code: 'd', Text: "d"}
	slotLightPress = tea.KeyPressMsg{Code: 'l', Text: "l"}
)

// pressSlotKey drives one slot-commit key through the live Update and returns the
// command alongside the model.
//
// The COMMAND is the observation a counter cannot make: a write deferred off the
// keypress would have to be scheduled as a tea.Cmd.
func pressSlotKey(t *testing.T, m Model, press tea.KeyPressMsg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(press)
	return updated.(Model), cmd
}

// requireSlotCommits fails unless the persister recorded exactly this sequence of
// SLOT commits — slug and slot together, and no constant commit at all.
//
// The pairing is the assertion: `d` writing the light slot, or `l` writing the
// cursor's neighbour, are both shapes a slug-only or slot-only recording would
// pass over.
func requireSlotCommits(t *testing.T, p *fakeThemePersister, want ...slotCommit) {
	t.Helper()

	if len(p.slotCommits) != len(want) {
		t.Fatalf("the persister recorded slot commits %v, want %v", p.slotCommits, want)
	}
	for i, commit := range want {
		if p.slotCommits[i] != commit {
			t.Fatalf("slot commit %d recorded %v, want %v", i+1, p.slotCommits[i], commit)
		}
	}
	if len(p.constants) != 0 {
		t.Errorf("a slot key committed the constant(s) %v; `d`/`l` write a SLOT and clear the constant in the SAME write (§9.2)", p.constants)
	}
}

// requireNoSlotLoad fails unless the recording fake behind m was asked for NO commit-time
// commit-time slot resolution at all.
//
// It is the SEAM-LEVEL statement of the construction-time load rule's "the load happens on the
// constant → adaptive transition and nowhere else": a `d`/`l` over an adaptive PAIR changes
// which slug a slot that is already live names, so both members are in hand and
// there is nothing to resolve. TestCommitSlotLoad_NonConvertingCommitIsSilent states
// the same rule from the other end — no `theme: loaded` line — and the two fail
// independently: a load that ran while emitting nothing passes there and fails here.
func requireNoSlotLoad(t *testing.T, m Model) {
	t.Helper()

	seam, ok := m.themeState.enumerator.(*fakeThemeEnumerator)
	if !ok {
		t.Fatalf("fixture: the seam is %T, want the recording fake — this is a statement about what the seam was ASKED", m.themeState.enumerator)
	}
	if len(seam.slotLoads) != 0 {
		t.Errorf("a non-converting commit asked the seam for slot load(s) %v, want none — §8.4 puts the load on the constant → adaptive transition and nowhere else", seam.slotLoads)
	}
}

// requirePairKeys fails unless the model's raw keys are the constant-or-pair rule's adaptive
// shape: these two slots, with the constant cleared.
func requirePairKeys(t *testing.T, m Model, light, dark string) {
	t.Helper()
	if got := m.themeState.keys; got != (theme.RawKeys{Light: light, Dark: dark}) {
		t.Errorf("themeKeys = %+v, want {Light:%s Dark:%s} — a slot commit clears the constant and leaves the OTHER slot alone (§8.2)", got, light, dark)
	}
}

// newSlotPairPanelModel is the SLOT fixture over a REAL loader and a REAL themes
// directory, under an adaptive pair: the light slot on lightSlug, the dark slot on
// darkSlug, and the dark one in force (the standing no-answer canvas, the appearance-gate
// rule).
//
// The loader is real wherever a test asserts ROWS or BADGES, for the reason the
// recompute fixture is: a stub seam answers Reassemble with a fixed union
// and Resolve with a fixed resolution, so every row and every `●` would be a
// statement about the fixture rather than about the derivation the commit drives.
func newSlotPairPanelModel(t *testing.T, dir, lightSlug, darkSlug string) (Model, *fakeThemePersister) {
	t.Helper()

	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Light: lightSlug, Dark: darkSlug})
	requireCursorOn(t, m, darkSlug)
	return m, persister
}

// newSlotSplitPanelModel opens the panel over `opened` under an ADAPTIVE PAIR
// (light on opened[0], dark on opened[1], the dark one in force), with the seam's
// split-reassembly field primed to answer a recompute with `reassembled`.
//
// The two DIFFERENT unions are what make the recompute's own effect observable: a
// reassembly that ran replaces the panel's rows outright, so "the rows did not
// move" plus a zero reassembly count is the recompute genuinely not happening —
// which a real loader cannot show, since a failed write leaves the keys untouched
// and a recompute over untouched keys derives the identical union.
func newSlotSplitPanelModel(t *testing.T, opened, reassembled []theme.Row) (Model, *fakeThemeEnumerator, *fakeThemePersister) {
	t.Helper()

	light, dark := opened[0], opened[1]
	reassembly := themeRowsUnion(reassembled)
	enumerator := &fakeThemeEnumerator{
		enumeration: theme.Enumeration{DirPath: fixtureThemesDir},
		union:       themeRowsUnion(opened),
		reassembled: &reassembly,
		resolution: theme.Resolution{
			Nomination: theme.ConstantNomination(dark.Theme),
			Slots: []theme.SlotResolution{
				{Slot: theme.SlotLight, Requested: light.Slug, Resolved: light.Slug, Theme: light.Theme},
				{Slot: theme.SlotDark, Requested: dark.Slug, Resolved: dark.Slug, Theme: dark.Theme},
			},
		},
	}
	persister := &fakeThemePersister{}
	deps := Deps{
		Lister:          fakeLister{},
		Theme:           theme.ConstantNomination(dark.Theme),
		ThemeEnumerator: enumerator,
		ThemeKeys:       theme.RawKeys{Light: light.Slug, Dark: dark.Slug},
		ThemePersister:  persister,
	}
	return openCommitPanel(t, deps, PageSessions, dark.Slug), enumerator, persister
}

// requireBothBadge fails unless the labelled row carries the row-rendering rule's collapsed `●
// both` and it is the ONLY badge the panel renders.
//
// The rendered text is asserted alongside the item's badge because the two say
// different things: the map is what the recompute derived, the glyphs are what the
// user is told. A residual `● light` or `● dark` anywhere would mean the collapse
// happened in one place and not the other.
func requireBothBadge(t *testing.T, m Model, label string) {
	t.Helper()

	requireBadge(t, m, label, theme.BadgeBoth)
	rendered := ansi.Strip(renderRecomputePanel(m))
	if got := strings.Count(rendered, "● both"); got != 1 {
		t.Errorf("the panel renders %d `● both` badges, want exactly 1 — both slots naming one slug is ONE row (§9.5)\n%s", got, rendered)
	}
	if strings.Contains(rendered, "● light") || strings.Contains(rendered, "● dark") {
		t.Errorf("the panel still renders a slot badge beside `● both`\n%s", rendered)
	}
}

// TestPanelSlotCommit_DarkWritesTheDarkSlot: it writes the dark slot.
//
// The picker idiom: "`d` — **Commits the dark slot** — writes `theme_dark = <selection>`,
// clears the constant | stays open."
//
// The fixture ARROWS AWAY from the row the panel opened on, which is the only
// shape in which "the cursor's slug" and "the persisted slug" are
// distinguishable — with the cursor still on the dark slot's row the two are the
// same string and the test would say nothing about which one was taken.
func TestPanelSlotCommit_DarkWritesTheDarkSlot(t *testing.T) {
	rows := arrowValidRows(4)
	light, target := rows[0].Slug, rows[2].Slug
	m, persister := newCommitPairPanelModel(t, rows)

	m = arrowToThemeRow(t, m, target)
	m, cmd := pressSlotKey(t, m, slotDarkPress)

	requireSlotCommits(t, persister, slotCommit{slug: target, slot: prefs.SlotDark})
	requirePairKeys(t, m, light, target)
	requireNoSlotLoad(t, m)
	if !m.themePanel.open {
		t.Error("`d` closed the panel; `Esc` is the ONLY way out (§9.2)")
	}
	if cmd != nil {
		t.Errorf("`d` scheduled %T; the write lands on this keypress", cmd)
	}
}

// TestPanelSlotCommit_LightWritesTheLightSlot: it writes the light slot.
//
// The picker idiom: "`l` — **Commits the light slot** — writes `theme_light = <selection>`,
// clears the constant | stays open." The mirror of the dark case, and asserted
// separately rather than as a table row because the two keys are the one place a
// transposed slot argument would be invisible in either direction.
func TestPanelSlotCommit_LightWritesTheLightSlot(t *testing.T) {
	rows := arrowValidRows(4)
	dark, target := rows[1].Slug, rows[2].Slug
	m, persister := newCommitPairPanelModel(t, rows)

	m = arrowToThemeRow(t, m, target)
	m, cmd := pressSlotKey(t, m, slotLightPress)

	requireSlotCommits(t, persister, slotCommit{slug: target, slot: prefs.SlotLight})
	requirePairKeys(t, m, target, dark)
	requireNoSlotLoad(t, m)
	if !m.themePanel.open {
		t.Error("`l` closed the panel; `Esc` is the ONLY way out (§9.2)")
	}
	if cmd != nil {
		t.Errorf("`l` scheduled %T; the write lands on this keypress", cmd)
	}
}

// TestPanelSlotCommit_OtherSlotSurvives: it leaves the other slot untouched.
//
// The property the row-rendering rule's `● both` rests on, and the one a naive "write the
// pair" implementation loses. It is asserted with the other slot naming a slug that
// resolves to NOTHING (`ghost`), which is the strongest form: prefs persists
// values verbatim, so a commit that re-derived the pair — or "helpfully" dropped
// an unresolvable slug — would show up here and nowhere else. The `ghost` row and
// its `● light` therefore survive the keypress intact, while the committed slot's
// badge moves.
func TestPanelSlotCommit_OtherSlotSurvives(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	m, persister := newSlotPairPanelModel(t, dir, "ghost", "aurora")
	requireRowLabels(t, m, "aurora", "ghost", "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	requireBadge(t, m, "ghost", theme.BadgeLight)

	m = arrowToThemeRow(t, m, "nord")
	m, _ = pressSlotKey(t, m, slotDarkPress)

	requireSlotCommits(t, persister, slotCommit{slug: "nord", slot: prefs.SlotDark})
	requirePairKeys(t, m, "ghost", "nord")
	requireRowLabels(t, m, "aurora", "ghost", "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	requireBadge(t, m, "ghost", theme.BadgeLight)
	requireBadge(t, m, "nord", theme.BadgeDark)
	requireBadge(t, m, "aurora", theme.BadgeNone)
}

// TestPanelSlotCommit_EmptyOtherSlotStaysEmpty: it leaves an UNSET other slot
// unset.
//
// The fresh-install shape of the same rule, and the one a real user reaches first:
// with no theme keys persisted at all, `d` produces `{Light:"" Dark:<slug>}`. The
// empty slot is carried through AS EMPTY rather than materialised into the shipped
// default it happens to resolve to — the shipped adaptive default's "an unset slot holds the
// shipped default" is a READ rule, and writing that default out would freeze today's
// default into the user's prefs, silently declining every later change to it. It is
// the same refusal commitSlot makes for an already-empty CONSTANT, which is
// likewise not special-cased in the write or in the mirror.
//
// The surviving `● light` is the control: the untouched slot resolves exactly as it
// did before the keypress, so leaving it empty is not a hole in the panel.
func TestPanelSlotCommit_EmptyOtherSlotStaysEmpty(t *testing.T) {
	dir := t.TempDir()
	// Sorted last, so it is reachable by `↓` from the row the open anchors on — and
	// it is neither shipped default, so the committed slug is distinguishable from
	// both.
	writeThemeFileForTest(t, dir, "zephyr.theme", "#101010")
	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{})
	requireRowLabels(t, m, "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug, "zephyr")
	requireCursorOn(t, m, theme.DefaultDarkSlug)
	requireBadge(t, m, theme.DefaultLightSlug, theme.BadgeLight)

	m = arrowToThemeRow(t, m, "zephyr")
	m, _ = pressSlotKey(t, m, slotDarkPress)

	requireSlotCommits(t, persister, slotCommit{slug: "zephyr", slot: prefs.SlotDark})
	requirePairKeys(t, m, "", "zephyr")
	requireBadge(t, m, "zephyr", theme.BadgeDark)
	requireBadge(t, m, theme.DefaultLightSlug, theme.BadgeLight)
	requireBadge(t, m, theme.DefaultDarkSlug, theme.BadgeNone)
}

// TestPanelSlotCommit_DThenLYieldsBoth: it produces the both badge in two
// keypresses.
//
// The row-rendering rule: "**When both slots name the same slug, that one row carries `●
// both`.** This is reachable in two keypresses (`d` then `l` on one row) and is a likely
// path — it is where a user lands wanting 'this theme everywhere' without
// realising `Enter` is the idiom for it."
//
// So the fixture presses exactly those two keys on one row, and the collapse falls
// out of the untouched-other-slot rule plus the post-commit recompute rather than out of
// any badge derivation on the commit path.
func TestPanelSlotCommit_DThenLYieldsBoth(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	writeThemeFileForTest(t, dir, "sunset.theme", "#202020")
	m, persister := newSlotPairPanelModel(t, dir, "sunset", "aurora")
	requireBadge(t, m, "sunset", theme.BadgeLight)
	requireBadge(t, m, "aurora", theme.BadgeDark)

	m = arrowToThemeRow(t, m, "nord")
	m, _ = pressSlotKey(t, m, slotDarkPress)
	requireBadge(t, m, "nord", theme.BadgeDark)
	requireBadge(t, m, "sunset", theme.BadgeLight)

	m, _ = pressSlotKey(t, m, slotLightPress)

	requireSlotCommits(t, persister,
		slotCommit{slug: "nord", slot: prefs.SlotDark},
		slotCommit{slug: "nord", slot: prefs.SlotLight},
	)
	requirePairKeys(t, m, "nord", "nord")
	requireBothBadge(t, m, "nord")
	requireBadge(t, m, "aurora", theme.BadgeNone)
	requireBadge(t, m, "sunset", theme.BadgeNone)
	if !m.themePanel.open {
		t.Error("the pair flow closed the panel; `Esc` is the ONLY way out (§9.2)")
	}
}

// TestPanelSlotCommit_ClearsTheConstantAtomically: it clears the constant in the
// same write.
//
// The constant-or-pair rule's mutual exclusion is enforced ON WRITE, and prefs.
// SaveThemeSlot performs both halves in ONE AtomicWrite. What the panel owes it
// is that the panel asks for it ONCE — no second call to clear the constant, and
// no merge re-implemented here — and that the in-memory mirror clears `Theme` the
// same way.
//
// The memory half is driven through commitSlot DIRECTLY, because the KEY is
// deliberately inert over a constant until the confirm resolves: this helper
// is what that confirm will drive on `y`, so the mirror is asserted where it is
// implemented rather than through a route that does not exist yet.
func TestPanelSlotCommit_ClearsTheConstantAtomically(t *testing.T) {
	t.Run("one call, not two", func(t *testing.T) {
		rows := arrowValidRows(4)
		m, persister := newCommitPairPanelModel(t, rows)

		m, _ = pressSlotKey(t, m, slotDarkPress)

		requireSlotCommits(t, persister, slotCommit{slug: rows[1].Slug, slot: prefs.SlotDark})
		if len(persister.slugs) != 1 {
			t.Errorf("the keypress made %d seam call(s) (%v), want the single atomic slot write", len(persister.slugs), persister.slugs)
		}
		if m.themeState.keys.Theme != "" {
			t.Errorf("themeKeys.Theme = %q after a slot commit, want it cleared (§8.2)", m.themeState.keys.Theme)
		}
	})

	t.Run("the constant is cleared in memory", func(t *testing.T) {
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
		// The hand-edited three-key shape the constant-or-pair rule makes legal and resolves as a
		// CONSTANT — the one state in which there is a constant for a slot commit
		// to clear, and in which the untouched slot is invisible until it does.
		keys := theme.RawKeys{Theme: "aurora", Light: "ghost", Dark: "aurora"}
		m, _, persister := newRecomputePanelModel(t, dir, keys)
		requireRowLabels(t, m, "aurora", "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug)

		if err := (&m).commitSlot("nord", prefs.SlotDark); err != nil {
			t.Fatalf("commitSlot(nord, dark): %v", err)
		}

		requireSlotCommits(t, persister, slotCommit{slug: "nord", slot: prefs.SlotDark})
		if len(persister.slugs) != 1 {
			t.Errorf("the commit made %d seam call(s) (%v), want one — the clear rides the SAME write (§8.2)", len(persister.slugs), persister.slugs)
		}
		requirePairKeys(t, m, "ghost", "nord")
		// The control: the recompute ran, so the cleared constant is a state this
		// instance genuinely moved to rather than one it merely reports.
		requireRowLabels(t, m, "aurora", "ghost", "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug)
		requireBadge(t, m, "ghost", theme.BadgeLight)
		requireBadge(t, m, "nord", theme.BadgeDark)
	})
}

// TestPanelSlotCommit_InertOverAConstant: it writes nothing while a constant is
// set.
//
// The picker idiom gives `d`/`l` over a constant a CONFIRM naming the constant that will be
// cleared, because it is "the one place a keypress described as inert can silently
// cost the user a setting they chose": on `"theme": "nord"`, pressing `l` clears
// the constant, the untouched dark slot falls back to the shipped default, and
// `Esc` in a dark terminal lands on `tokyo-night` rather than `nord`.
//
// So the KEYPRESS ITSELF WRITES NOTHING: it asks, and the write happens on `y`
// (theme_panel_confirm.go). What this file keeps asserting is the GATE at
// the `d`/`l` dispatch — no prefs call, no mutated keys, no re-theme, no close and
// no deferred write riding a tea.Cmd — while the question's own behaviour (its copy,
// its swapped footer, and the three inputs that resolve it) is covered by the
// confirm's suite.
//
// The POSITIVE CONTROL is the same keypress over an adaptive setting, which does
// write — so the absence above is the gate rather than an unwired key.
func TestPanelSlotCommit_InertOverAConstant(t *testing.T) {
	for _, tc := range []struct {
		name  string
		press tea.KeyPressMsg
		slot  prefs.ThemeSlot
	}{
		{name: "d", press: slotDarkPress, slot: prefs.SlotDark},
		{name: "l", press: slotLightPress, slot: prefs.SlotLight},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rows := arrowValidRows(4)
			persisted, target := rows[0].Slug, rows[2].Slug
			m, persister := newCommitPanelModel(t, rows, persisted)
			m = arrowToThemeRow(t, m, target)
			previewed := m.themeState.active

			m, cmd := pressSlotKey(t, m, tc.press)

			if len(persister.slugs) != 0 {
				t.Errorf("%q wrote %v over a constant; the write waits for the confirm's `y` (§9.2)", tc.name, persister.slugs)
			}
			requireConstantKeys(t, m, persisted)
			if !m.themePanel.open {
				t.Errorf("%q closed the panel over a constant", tc.name)
			}
			if got := m.themePanel.message; got.Kind != themeMessageConfirm {
				t.Errorf("%q left the message %+v; over a constant the keypress ASKS (§9.2)", tc.name, got)
			}
			if m.themeState.active != previewed {
				t.Errorf("%q rendered canvas %s, want the previewed %s left alone", tc.name, m.themeState.active.Canvas.Value, previewed.Canvas.Value)
			}
			if cmd != nil {
				t.Errorf("%q over a constant scheduled %T, want nothing", tc.name, cmd)
			}

			// Positive control: the SAME key over an ADAPTIVE setting writes, so the
			// inertness above is the picker idiom's gate rather than a dead arm.
			pair, pairPersister := newCommitPairPanelModel(t, rows)
			pair, _ = pressSlotKey(t, arrowToThemeRow(t, pair, target), tc.press)
			requireSlotCommits(t, pairPersister, slotCommit{slug: target, slot: tc.slot})
			if pair.themeState.keys.Theme != "" {
				t.Errorf("positive control: the adaptive commit left the constant %q set", pair.themeState.keys.Theme)
			}
		})
	}
}

// TestPanelSlotCommit_NonActiveSlotIsVisuallyInert: it changes nothing on screen.
//
// The picker idiom: "**Committing to a non-active slot changes nothing on screen.** Previewing
// a light theme in a dark terminal and pressing `l` writes the light slot, but the
// resolved-active theme is still the dark slot. A commit is a **write, not a
// navigation**."
//
// This is the sharpest case of that rule in the whole panel — the only feedback is
// the badge — so it is asserted in the three directions it can fail: the frame
// bytes, the palette across a commit that legitimately moves a badge, and what
// `Esc` resolves to afterwards.
func TestPanelSlotCommit_NonActiveSlotIsVisuallyInert(t *testing.T) {
	// The dark slot is in force (the appearance-gate rule's standing no-answer canvas), so every
	// subtest below previews a LIGHT theme in a dark terminal and commits the LIGHT
	// slot — the exact configuration the picker idiom names.
	newModel := func(t *testing.T) (Model, *fakeThemePersister) {
		t.Helper()
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
		writeThemeFileForTest(t, dir, "sunset.theme", "#202020")
		return newSlotPairPanelModel(t, dir, theme.DefaultLightSlug, "aurora")
	}

	t.Run("the frame is byte-identical across the keypress", func(t *testing.T) {
		m, persister := newModel(t)

		m = arrowToThemeRow(t, m, theme.DefaultLightSlug)
		previewed := m.themeState.active
		before := m.View().Content

		m, cmd := pressSlotKey(t, m, slotLightPress)

		requireSlotCommits(t, persister, slotCommit{slug: theme.DefaultLightSlug, slot: prefs.SlotLight})
		if m.themeState.active != previewed {
			t.Errorf("the commit rendered canvas %s, want the previewed %s left alone", m.themeState.active.Canvas.Value, previewed.Canvas.Value)
		}
		if got := m.View().Content; got != before {
			t.Errorf("the commit changed the composed frame\nbefore: %q\nafter:  %q", escSeq(before), escSeq(got))
		}
		if cmd != nil {
			t.Errorf("the commit scheduled %T, want nothing", cmd)
		}
	})

	// The badge-MOVING case, where a byte comparison cannot be made because the
	// panel legitimately re-renders: the palette is compared as a SET of SGR
	// sequences instead, and the moved badge is the control proving the recompute
	// ran. This is where the claim genuinely bites — the previewed row is NOT the
	// row the persisted setting resolves to, so a commit that re-resolved and
	// applied would flip the whole frame to the dark slot's palette.
	t.Run("a badge-moving commit leaves the palette alone", func(t *testing.T) {
		m, persister := newModel(t)

		m = arrowToThemeRow(t, m, "sunset")
		previewed := m.themeState.active
		if got := previewed.Canvas.Value; got != "#202020" {
			t.Fatalf("fixture: arrowing to `sunset` rendered canvas %s, want #202020", got)
		}
		colours := frameColours(m.View().Content)

		m, _ = pressSlotKey(t, m, slotLightPress)

		requireSlotCommits(t, persister, slotCommit{slug: "sunset", slot: prefs.SlotLight})
		requireBadge(t, m, "sunset", theme.BadgeLight)
		requireBadge(t, m, theme.DefaultLightSlug, theme.BadgeNone)
		if m.themeState.active != previewed {
			t.Errorf("the commit rendered canvas %s, want the previewed %s — the resolved-active theme is still the DARK slot (§9.2)", m.themeState.active.Canvas.Value, previewed.Canvas.Value)
		}
		if got := frameColours(m.View().Content); !slices.Equal(got, colours) {
			t.Errorf("the commit changed the frame's colours\nbefore: %v\nafter:  %v", colours, got)
		}
	})

	// What the write DID do, observed the only way a screen-inert write can be:
	// `Esc` resolves persisted state, and it still lands on the dark slot.
	t.Run("Esc still resolves the dark slot", func(t *testing.T) {
		m, persister := newModel(t)

		m, _ = pressSlotKey(t, arrowToThemeRow(t, m, "sunset"), slotLightPress)
		requireSlotCommits(t, persister, slotCommit{slug: "sunset", slot: prefs.SlotLight})

		m = closeThemePanelForTest(t, m)

		if m.themePanel.open {
			t.Fatal("Esc left the panel open")
		}
		if got := m.themeState.active.Canvas.Value; got != "#101010" {
			t.Errorf("the close rendered canvas %s, want the DARK slot's aurora #101010 — a light-slot commit does not change what is in force in a dark terminal (§9.2)", got)
		}
	})
}

// TestPanelSlotCommit_RepeatIsIdempotent: it is idempotent.
//
// The failed-commit rule: "A commit is always re-attemptable. The commit keys are
// unconditional writes, so pressing `d`/`l`/`Enter` again simply retries — no special
// retry affordance, and no state to clear first."
//
// Driven on the row ALREADY carrying that slot's badge, which is both the
// degenerate case (the cursor opens on the in-force slot's row, so it is one
// keypress away) and the one where a "don't write what is already set" shortcut
// would be tempting: prefs writes unconditionally, so the second press is the same
// call and the same resulting state.
func TestPanelSlotCommit_RepeatIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	m, persister := newSlotPairPanelModel(t, dir, theme.DefaultLightSlug, "aurora")
	requireBadge(t, m, "aurora", theme.BadgeDark)

	m, _ = pressSlotKey(t, m, slotDarkPress)
	once := m.View().Content
	onceKeys := m.themeState.keys
	onceBadges := maps.Clone(m.themePanel.badges)

	m, cmd := pressSlotKey(t, m, slotDarkPress)

	repeat := slotCommit{slug: "aurora", slot: prefs.SlotDark}
	requireSlotCommits(t, persister, repeat, repeat)
	requirePairKeys(t, m, theme.DefaultLightSlug, "aurora")
	if m.themeState.keys != onceKeys {
		t.Errorf("the second commit left keys %+v, want the first's %+v", m.themeState.keys, onceKeys)
	}
	if got := m.themePanel.badges; !maps.Equal(got, onceBadges) {
		t.Errorf("the second commit left badges %v, want the first's %v", got, onceBadges)
	}
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("the repeat commit raised the message %+v; there is no retry affordance and no state to clear first (§9.13)", got)
	}
	if cmd != nil {
		t.Errorf("the repeat commit scheduled %T, want nothing", cmd)
	}
	if got := m.View().Content; got != once {
		t.Errorf("the repeat commit changed the frame\nonce:  %q\ntwice: %q", escSeq(once), escSeq(got))
	}

	// The NO-ERROR half, which the keypress itself discards (the report is raised
	// inside the commit): a third identical commit through the helper reports success rather
	// than merely writing again.
	if err := (&m).commitSelectedSlot(prefs.SlotDark); err != nil {
		t.Errorf("a repeated commit returned %v, want nil — a commit is always re-attemptable (§9.13)", err)
	}
}

// TestPanelSlotCommit_TypedSlotOnly: it cannot mint a third slot.
//
// The slot is the TYPED prefs value threaded through the seam, never a
// caller-supplied key name — the structural half of the constant-or-pair rule's two-slot
// model, whose other half is prefs.SaveThemeSlot rejecting an out-of-range value before it
// writes anything. The type alone does most of the work (ThemeSlot is
// not a string, so `prefs.ThemeSlot("dark")` does not compile), which leaves
// exactly two shapes a scan has to close: a CONVERSION minting a value from an
// integer, and a THIRD constant appearing beside the two.
//
// The seam's parameter type is asserted alongside, because it is what makes the
// panel's call site incapable of naming a slot any other way.
func TestPanelSlotCommit_TypedSlotOnly(t *testing.T) {
	constants, conversions := themeSlotUsagesInPackage(t)

	if len(conversions) != 0 {
		t.Errorf("%v convert a value to prefs.ThemeSlot; the slot is the typed value threaded through the seam, never one minted here", conversions)
	}
	if want := []string{"SlotDark", "SlotLight"}; !slices.Equal(constants, want) {
		t.Errorf("the package names prefs slots %v, want exactly %v — a third slot must not be nameable", constants, want)
	}

	method, ok := reflect.TypeFor[ThemePersister]().MethodByName("CommitThemeSlot")
	if !ok {
		t.Fatal("ThemePersister declares no CommitThemeSlot")
	}
	if got, want := method.Type.In(1), reflect.TypeFor[prefs.ThemeSlot](); got != want {
		t.Errorf("CommitThemeSlot takes a %s, want the typed %s", got, want)
	}
}

// themeSlotUsagesInPackage walks the package's PRODUCTION sources and returns
// every prefs slot constant it names (sorted and deduplicated) alongside every
// file converting a value to prefs.ThemeSlot.
//
// The constants list is the scan's own positive control: it is asserted to hold
// both slots, so a scan looking for a shape that never appears would fail rather
// than pass an empty conversion list vacuously.
func themeSlotUsagesInPackage(t *testing.T) (constants, conversions []string) {
	t.Helper()

	for name, file := range parsePackageFilesByName(t) {
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.CallExpr:
				if isPrefsSelector(node.Fun, "ThemeSlot") {
					conversions = append(conversions, name)
				}
			case *ast.SelectorExpr:
				if isPrefsSelector(node, "") && strings.HasPrefix(node.Sel.Name, "Slot") {
					constants = append(constants, node.Sel.Name)
				}
			}
			return true
		})
	}
	slices.Sort(constants)
	slices.Sort(conversions)
	return slices.Compact(constants), conversions
}

// isPrefsSelector reports whether expr is `prefs.<sel>` — or any `prefs.*`
// selector when sel is empty.
func isPrefsSelector(expr ast.Expr, sel string) bool {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := selector.X.(*ast.Ident)
	return ok && pkg.Name == "prefs" && (sel == "" || selector.Sel.Name == sel)
}

// TestPanelSlotCommit_FailedWriteLeavesKeysAlone: it mutates nothing on failure.
//
// The failed-commit rule: a failed commit "does not move the `●` — the marker means 'what is
// persisted' and would be lying if it moved". The badges are derived from the raw
// keys, so the mechanism is that nothing is mutated and the recompute is never
// reached. The whole key struct is compared, so the OTHER slot's raw value riding
// through untouched is covered alongside the committed one.
//
// BOTH SUBTESTS COMMIT A SLUG THE TARGET SLOT DOES NOT ALREADY HOLD, which is what
// makes the mirror a NON-IDENTITY on these keys and the untouched-keys assertion a
// real one: an implementation that mirrored first and wrote afterwards moves the
// committed slot's raw value — and, over a constant, clears the constant — off a
// write that never landed. Committing the slug the slot already holds leaves those
// keys byte-identical either way, and would assert nothing.
//
// The error is RETURNED rather than swallowed, and the panel REPORTS it: the failed-commit
// rule's message-slot line is raised from the value while everything the `●` derives from
// is left exactly as it was. What this file asserts is the untouched half; the
// report's own behaviour — its copy, its lifetime and the outstanding state behind
// it — is covered by the failure suite.
func TestPanelSlotCommit_FailedWriteLeavesKeysAlone(t *testing.T) {
	opened := arrowValidRows(4)
	reassembled := arrowValidRows(2)

	t.Run("the keypress mutates nothing", func(t *testing.T) {
		m, enumerator, persister := newSlotSplitPanelModel(t, opened, reassembled)
		// Arrowed OFF the dark slot's own row before the keypress, per the
		// non-identity rule above: the fixture opens on the slug the dark slot
		// already holds.
		m = arrowToThemeRow(t, m, opened[2].Slug)
		keys := m.themeState.keys
		badges := maps.Clone(m.themePanel.badges)
		previewed := m.themeState.active
		persister.err = errThemeCommitFailed

		m, cmd := pressSlotKey(t, m, slotDarkPress)

		requireSlotCommits(t, persister, slotCommit{slug: opened[2].Slug, slot: prefs.SlotDark})
		if m.themeState.keys != keys {
			t.Errorf("a failed slot commit left keys %+v, want the untouched %+v", m.themeState.keys, keys)
		}
		requireRowLabels(t, m, arrowSlug(0), arrowSlug(1), arrowSlug(2), arrowSlug(3))
		if got := m.themePanel.badges; !maps.Equal(got, badges) {
			t.Errorf("the failed commit left badges %v, want the untouched %v — a failed commit does not move the `●` (§9.13)", got, badges)
		}
		if enumerator.reassembles != 0 {
			t.Errorf("the failed commit ran %d reassemblies, want 0 — only a SUCCESSFUL commit recomputes (§9.2)", enumerator.reassembles)
		}
		if !m.themePanel.open {
			t.Error("a failed slot commit closed the panel; `Esc` is the only way out (§9.2)")
		}
		if m.themeState.active != previewed {
			t.Errorf("a failed commit rendered canvas %s, want the previewed %s — §9.13 KEEPS the theme applied in memory", m.themeState.active.Canvas.Value, previewed.Canvas.Value)
		}
		if got := m.themePanel.message; got.Kind != themeMessageCommitFailed {
			t.Errorf("a failed slot commit left the message %+v, want §9.13's failed-commit line reported in the slot", got)
		}
		if cmd != nil {
			t.Errorf("a failed slot commit scheduled %T, want nothing", cmd)
		}
	})

	// The helper's own return, driven over the shape that carries a CONSTANT for the
	// failed write to have wrongly cleared — the confirm's half: it calls this helper on
	// `y`, and on failure the constant is not cleared in memory, so the badges still
	// show it. The KEY cannot reach here over a constant (that is 9-5's confirm), so
	// the helper is called directly, exactly as the successful clear is.
	t.Run("the helper returns the error and leaves the constant set", func(t *testing.T) {
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
		// The hand-edited three-key shape the constant-or-pair rule makes legal and resolves as a
		// CONSTANT.
		keys := theme.RawKeys{Theme: "aurora", Light: "ghost", Dark: "aurora"}
		m, _, persister := newRecomputePanelModel(t, dir, keys)
		requireBadge(t, m, "aurora", theme.BadgeConstant)
		badges := maps.Clone(m.themePanel.badges)
		m = arrowToThemeRow(t, m, "nord")
		persister.err = errThemeCommitFailed

		if err := (&m).commitSelectedSlot(prefs.SlotDark); !errors.Is(err, errThemeCommitFailed) {
			t.Errorf("commitSelectedSlot returned %v, want the persister's error — the caller reads the outcome from it", err)
		}

		requireSlotCommits(t, persister, slotCommit{slug: "nord", slot: prefs.SlotDark})
		if m.themeState.keys != keys {
			t.Errorf("a failed slot commit left keys %+v, want the untouched %+v — §8.2 clears the constant in the WRITE, and this write did not land", m.themeState.keys, keys)
		}
		if got := m.themePanel.badges; !maps.Equal(got, badges) {
			t.Errorf("the failed commit left badges %v, want the untouched %v — the constant is still set, so the panel still marks it (§9.13)", got, badges)
		}
		requireBadge(t, m, "aurora", theme.BadgeConstant)
	})

	// Positive control: the same fixture with the write succeeding DOES recompute,
	// so the untouched rows above are the failure path rather than an unwired one.
	t.Run("the same fixture recomputes when the write lands", func(t *testing.T) {
		m, enumerator, _ := newSlotSplitPanelModel(t, opened, reassembled)

		m, _ = pressSlotKey(t, m, slotDarkPress)

		requireRowLabels(t, m, arrowSlug(0), arrowSlug(1))
		if enumerator.reassembles != 1 {
			t.Fatalf("a successful slot commit ran %d reassemblies, want 1", enumerator.reassembles)
		}
	})
}

// TestPanelSlotCommit_NilPersisterIsInert: it tolerates a nil persister.
//
// A fixture or `capturetool` model carries NO persister, so a slot
// commit during a capture writes nowhere. It is the ABSENCE OF A WRITER rather
// than a failed write — it raises no message and no outstanding-failure state
// — which is why it mutates nothing either: there is nothing on disk
// for the in-memory keys to mirror, and a panel claiming a slot nothing persisted
// is what `Esc` would then resolve to.
func TestPanelSlotCommit_NilPersisterIsInert(t *testing.T) {
	rows := arrowValidRows(4)
	target := rows[2].Slug

	m := openCommitPanel(t, commitPairPanelDeps(t, rows), PageSessions, rows[1].Slug)
	if m.themeState.persister != nil {
		t.Fatalf("fixture: the model holds persister %#v, want none", m.themeState.persister)
	}
	m = arrowToThemeRow(t, m, target)
	keys := m.themeState.keys
	before := m.View().Content

	m, cmd := pressSlotKey(t, m, slotDarkPress)

	if m.themeState.keys != keys {
		t.Errorf("`d` over a nil persister left keys %+v, want the untouched %+v", m.themeState.keys, keys)
	}
	if !m.themePanel.open {
		t.Error("`d` closed the panel over a nil persister")
	}
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("a nil persister raised the message %+v; it is the absence of a WRITER, not a failed write", got)
	}
	if cmd != nil {
		t.Errorf("`d` over a nil persister scheduled %T, want nothing", cmd)
	}
	if got := m.View().Content; got != before {
		t.Errorf("`d` over a nil persister changed the frame\nbefore: %q\nafter:  %q", escSeq(before), escSeq(got))
	}

	// Positive control: the SAME fixture with a persister wired does write and does
	// mutate, so the untouched keys above are the nil seam rather than a dead arm.
	wired, persister := newCommitPairPanelModel(t, rows)
	wired, _ = pressSlotKey(t, arrowToThemeRow(t, wired, target), slotDarkPress)
	requireSlotCommits(t, persister, slotCommit{slug: target, slot: prefs.SlotDark})
	requirePairKeys(t, wired, rows[0].Slug, target)
}

// TestPanelSlotCommit_EnterAfterSlotNeedsNoConfirm: it needs no confirm for a
// following `Enter`.
//
// The picker idiom: "**The reverse direction needs no confirm.** `Enter` on a theme while a
// pair is set clears both slots — but `Enter` visibly does what it says." The pair
// this one clears is the one the user has just built with `d`, which is the
// sharpest form of the asymmetry: the slot key that CREATED the pair is gated
// behind a confirm over a constant, and the key that destroys the pair is not
// gated at all.
//
// So the constant lands on that single keypress, with nothing in the panel layout's message
// slot and the panel's standing footer untouched.
func TestPanelSlotCommit_EnterAfterSlotNeedsNoConfirm(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	m, persister := newSlotPairPanelModel(t, dir, theme.DefaultLightSlug, "aurora")

	m, _ = pressSlotKey(t, m, slotDarkPress)
	requireSlotCommits(t, persister, slotCommit{slug: "aurora", slot: prefs.SlotDark})
	requirePairKeys(t, m, theme.DefaultLightSlug, "aurora")
	footer := renderThemePanelFooter(themePanelKeymap(), themePanelInnerWidth(m.themePanel.width), m.themeState.active, m.colourless)

	m, cmd := pressCommitKey(t, m)

	requireConstantKeys(t, m, "aurora")
	if len(persister.constants) != 1 || persister.constants[0] != "aurora" {
		t.Errorf("`Enter` after a slot commit recorded constants %v, want exactly [aurora]", persister.constants)
	}
	requireBadge(t, m, "aurora", theme.BadgeConstant)
	requireBadge(t, m, theme.DefaultLightSlug, theme.BadgeNone)
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("`Enter` after a slot commit raised the message %+v; the reverse direction needs no confirm (§9.2)", got)
	}
	if cmd != nil {
		t.Errorf("`Enter` scheduled %T; the write lands on this keypress rather than awaiting an answer", cmd)
	}
	if !m.themePanel.open {
		t.Error("`Enter` closed the panel")
	}
	after := renderThemePanelFooter(themePanelKeymap(), themePanelInnerWidth(m.themePanel.width), m.themeState.active, m.colourless)
	if after != footer {
		t.Errorf("`Enter` swapped the panel footer\nbefore: %q\nafter:  %q", escSeq(footer), escSeq(after))
	}
}

// TestPanelSlotCommit_NoOtherIO: it reads nothing and writes nothing else.
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
func TestPanelSlotCommit_NoOtherIO(t *testing.T) {
	configDir := t.TempDir()
	t.Setenv("PORTAL_PREFS_FILE", filepath.Join(configDir, "prefs.json"))
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "sunset.theme", "#101010")

	stores := newCountingStores()
	// The theme persister is deliberately the RECORDING fake rather than the
	// counted one: it is the single write `d` is allowed to make, so the counted set
	// covers everything else and this records what the one write was.
	persister := &fakeThemePersister{}
	loader := theme.NewLoader(nil)
	enumerator := countingEnumeratorOver(loader, dir)
	keys := theme.RawKeys{Light: theme.DefaultLightSlug, Dark: "sunset"}
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

	m, cmd := pressSlotKey(t, m, slotDarkPress)

	requireSlotCommits(t, persister, slotCommit{slug: "sunset", slot: prefs.SlotDark})
	if got := stores.calls(); got != 0 {
		t.Errorf("the slot commit made %d file-touching seam call(s) — the prefs write is the only one (project store %d, project editor %d, alias editor %d, mode persister %d, theme persister %d, scrollback %d, lister %d)",
			got, stores.projectStore.calls, stores.projectEditor.calls, stores.aliasEditor.calls,
			stores.modePersister.calls, stores.themePersister.calls, stores.scrollback.calls, stores.lister.calls)
	}
	if enumerator.opens != opens {
		t.Errorf("the slot commit ran %d enumerations in total, want the open's %d — a commit re-derives from the retained parse and never re-reads the directory (§8.4)", enumerator.opens, opens)
	}
	if cmd != nil {
		t.Errorf("the slot commit scheduled %T; a deferred write is the one shape the counters above cannot see", cmd)
	}
	if entries, err := os.ReadDir(configDir); err != nil || len(entries) != 0 {
		t.Errorf("the slot commit left %d entries in the config directory (err %v), want none — the model writes through the seam and touches no path of its own", len(entries), err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	if len(entries) != 1 || entries[0].Name() != "sunset.theme" {
		t.Errorf("the themes directory holds %d entries after a commit, want only the seeded drop-in", len(entries))
	}
}
