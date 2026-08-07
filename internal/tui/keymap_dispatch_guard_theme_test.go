package tui

import (
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// keymap_dispatch_guard_theme_test.go extends keymap_dispatch_guard_test.go's
// descriptor↔dispatch parity to the theme panel (§9.12) and to the nested confirm
// scope beneath it (§9.2) — the SECOND and THIRD places in the product a key label
// can go stale, both of them bespoke vertical footers rendered from descriptors that
// live outside the three page keymaps.
//
// The contract is the pages': every non-help descriptor entry has a probe the LIVE
// dispatch honours, and every probe's key appears in the descriptor. Both directions
// bite, which is why the panel scope is authored as all SIX keys rather than the four
// its footer shows — arrows and paging are dispatched inside the panel, so leaving
// them out of the descriptor is what breaks parity rather than what satisfies it.
//
// A PROBE MUST ASSERT THE BOUND EFFECT, NEVER MERE CONSUMPTION. That is the one way
// this guard could pass vacuously where the page guards cannot: the panel is
// key-exclusive while it is open (§9.7), so "the keypress was swallowed" is true of
// every key in the alphabet — including one whose dispatch arm has just been deleted.
// Every probe below therefore asserts a state change the dispatch is supposed to
// cause, and TestThemePanelParity_ProbesAssertEffects executes that rule against all
// of them.
//
// THE SEEDS TOUCH NOTHING REAL. A faked enumerator answers with hand-built rows so no
// themes directory is read, and a recording fake persister collects the commits so no
// `prefs.json` is written — the guard drives §9.2's four writing keys, which is
// precisely the set that would otherwise reach the user's own config.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe across
// this package's tests.

// themePanelGuardRows is the guard seed's union: four selectable rows and one the
// loader rejected.
//
// The INVALID row is not decoration: §9.4 requires a broken drop-in to be listed, so
// this is the shape a real themes directory reaches the moment anyone edits a file.
// It sorts last, leaving the pair the seed's slots name (rows[0] / rows[1]) and the
// rows the cursor steps and pages onto all selectable.
func themePanelGuardRows() []theme.Row {
	return append(arrowValidRows(4), arrowInvalidRow(arrowSlug(4)))
}

// themePanelGuardModel is the panel scope's seed — sessionsGuardModel's counterpart
// one surface in: a Sessions model with the §13.3 seam faked, a recording persister,
// and the panel already OPEN through the production `t` keypress rather than a
// hand-installed struct.
//
// THE SETTING IS AN ADAPTIVE PAIR, which is what keeps the `d`/`l` probes about the
// COMMIT: over a constant §9.2's gate makes those keys raise a confirm instead, and
// the probes would be asserting the question. The confirm gets its own seed and its
// own scope below.
//
// THE TERMINAL IS THE PAGINATING ONE (arrowPagingTermH), because the paging probe has
// to observe the page MOVE. At the standard fixture height every row fits one page,
// where `Ctrl+↓` routes perfectly and goes nowhere — indistinguishable from a deleted
// arm.
//
// The two preconditions are asserted rather than assumed for the same reason
// themeGuardModel checks `colourless`: a seed that quietly stopped opening the panel,
// or stopped paginating, would make the probes pass against nothing at all.
func themePanelGuardModel(t *testing.T) (Model, *fakeThemePersister) {
	t.Helper()

	rows := themePanelGuardRows()
	persister := &fakeThemePersister{}
	deps := commitPairPanelDeps(t, rows)
	deps.ThemePersister = persister

	m := Build(deps)
	if m.colourless {
		t.Fatal("the guard seed must not be colourless — §9.10 blocks `t`, and every probe would assert a refusal")
	}
	m.termWidth, m.termHeight = arrowTermW, arrowPagingTermH
	m.applySessions(closePanelSessions())

	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("the guard seed's panel did not open; every probe would assert against a closed panel")
	}
	requireCursorOn(t, m, rows[1].Slug)
	requireArrowPanelPageSize(t, m, arrowPagingPerPage)
	return m, persister
}

// themePanelDispatch is the panel's key routing as a VALUE, so the probe sets can be
// driven against a deliberately broken stand-in as well as against production.
//
// Without it the guard could never be guarded: a suite proving that deleting an arm
// makes a probe fail would have to delete the arm from updateThemePanel itself.
type themePanelDispatch func(Model, tea.KeyPressMsg) (tea.Model, tea.Cmd)

// livePanelDispatch is production — the routing every real probe drives. Both scopes
// share it: the confirm resolves inside updateThemePanel's own key-exclusive arm, so
// there is no second entry point to drive it through.
var livePanelDispatch themePanelDispatch = Model.updateThemePanel

// dispatchPanelKey drives one press through the given dispatch and returns the model
// it produced.
func dispatchPanelKey(t *testing.T, dispatch themePanelDispatch, m Model, press tea.KeyPressMsg) Model {
	t.Helper()

	updated, _ := dispatch(m, press)
	next, ok := updated.(Model)
	if !ok {
		t.Fatalf("the panel dispatch returned %T, want a Model", updated)
	}
	return next
}

// themePanelProbes is §9.12's panel scope as SIX probes, one per descriptor entry,
// each driving the live routing and asserting the effect that entry is bound to.
//
// NOT ONE OF THEM ASSERTS CONSUMPTION, and that is a requirement rather than a
// style: the panel owns the keyboard while it is open (§9.7), so "the key was
// swallowed" is true of every key in the alphabet and would be satisfied by a
// dispatch arm that had just been deleted. TestThemePanelParity_ProbesAssertEffects
// executes that rule against every probe below.
//
// ARROWS AND PAGING CARRY PROBES TOO, because the descriptor lists them (§9.12's
// "all six") and the guard's contract is parity in BOTH directions — a descriptor
// key with no probe is a failure, so omitting them is what breaks the check rather
// than what satisfies it. Their non-core flag governs the FOOTER, not this set.
//
// The dispatch is a PARAMETER so the same six probes can be driven against a
// deliberately broken stand-in; the live suites pass livePanelDispatch.
func themePanelProbes(dispatch themePanelDispatch) map[string]dispatchProbe {
	return map[string]dispatchProbe{
		// ↑↓ navigate — the panel's own cursor moves (routed by updateThemePanel
		// against the list's KeyMap, not by the list's Update alone).
		"↑↓": {press: arrowDown, honour: func(t *testing.T) bool {
			m, _ := themePanelGuardModel(t)
			before := m.themePanel.list.Index()
			return dispatchPanelKey(t, dispatch, m, arrowDown).themePanel.list.Index() != before
		}},
		// ^↑/↓ page — the paging bindings are live on the panel's list AND the
		// keypress moves the PAGE. The binding half alone would pass on a list whose
		// page never moves; the page half alone would pass on a one-row page, where
		// paging is indistinguishable from a step.
		"^↑/↓": {press: arrowPageDown, honour: func(t *testing.T) bool {
			m, _ := themePanelGuardModel(t)
			if len(m.themePanel.list.KeyMap.NextPage.Keys()) == 0 || len(m.themePanel.list.KeyMap.PrevPage.Keys()) == 0 {
				return false
			}
			before := m.themePanel.list.Paginator.Page
			return dispatchPanelKey(t, dispatch, m, arrowPageDown).themePanel.list.Paginator.Page != before
		}},
		// ⏎ set theme — the persister records the CURSOR's slug as a constant, and the
		// panel is still open. Both halves: §9.2 pins the write and the not-closing
		// equally, and `Enter` is precisely the key a dual-purpose exit would break.
		"⏎": {press: commitEnter, honour: func(t *testing.T) bool {
			m, persister := themePanelGuardModel(t)
			slug := themePanelCursorRow(t, m).Slug
			after := dispatchPanelKey(t, dispatch, m, commitEnter)
			return slices.Equal(persister.constants, []string{slug}) && after.themePanel.open
		}},
		// d set as dark — the persister records the cursor's slug against the TYPED
		// dark slot.
		"d": {press: slotDarkPress, honour: func(t *testing.T) bool {
			return themePanelSlotHonoured(t, dispatch, slotDarkPress, prefs.SlotDark)
		}},
		// l set as light — the same against the light slot. Asserted separately from
		// `d` because a transposed slot argument is invisible from either key alone.
		"l": {press: slotLightPress, honour: func(t *testing.T) bool {
			return themePanelSlotHonoured(t, dispatch, slotLightPress, prefs.SlotLight)
		}},
		// esc close — the panel CLOSES and the retained parse goes with it. This is
		// the close, never the confirm's cancel: the seed holds no confirm, and
		// TestThemePanelDispatch_EscMeansInnermostFirst pins the two apart.
		"esc": {press: keyEsc, honour: func(t *testing.T) bool {
			m, _ := themePanelGuardModel(t)
			after := dispatchPanelKey(t, dispatch, m, keyEsc)
			return !after.themePanel.open && !after.themePanel.confirming() && themePanelStateDropped(after)
		}},
	}
}

// themePanelSlotHonoured is the shared body of the `d` and `l` probes: the cursor's
// slug written into the slot the key names, with the panel left open and no confirm
// raised.
//
// The seed is an ADAPTIVE PAIR precisely so no confirm intervenes — over a constant
// §9.2's gate would make these keys ask rather than write, and the probe would be
// asserting the question instead of the commit.
func themePanelSlotHonoured(t *testing.T, dispatch themePanelDispatch, press tea.KeyPressMsg, slot prefs.ThemeSlot) bool {
	t.Helper()

	m, persister := themePanelGuardModel(t)
	slug := themePanelCursorRow(t, m).Slug
	after := dispatchPanelKey(t, dispatch, m, press)
	return slices.Equal(persister.slotCommits, []slotCommit{{slug: slug, slot: slot}}) &&
		after.themePanel.open && !after.themePanel.confirming()
}

// themePanelStateDropped reports whether the panel's retained state went with the
// close — §5.8's one read, the union it produced and the list built from it, which
// closeThemePanel discards WHOLE.
func themePanelStateDropped(m Model) bool {
	return len(m.themePanel.enumeration.Entries) == 0 &&
		len(m.themePanel.union.Rows) == 0 &&
		len(m.themePanel.list.Items()) == 0 &&
		m.themePanel.width == 0
}

// themeConfirmProbes is §9.2's NESTED CONFIRM SCOPE as its own probe set — a SECOND
// scope beside the panel's, so the substituted footer and the arms it advertises
// cannot drift apart either.
//
// The descriptor carries the terse `y`/`n` glyphs and the dispatch honours both
// cases; TestThemeConfirmDispatch_UppercaseReachesTheSameArm drives `Y`/`N` through
// these same effect predicates so the uppercase half is not left unguarded by a
// lowercase-only probe.
func themeConfirmProbes(dispatch themePanelDispatch) map[string]dispatchProbe {
	return map[string]dispatchProbe{
		// y confirm — the pending assignment is written and the question comes down.
		"y": {press: confirmYes, honour: func(t *testing.T) bool {
			return themeConfirmYesHonoured(t, dispatch, confirmYes)
		}},
		// n cancel — nothing is written and the question comes down. The write half
		// alone would be true of a swallow, so the resolution is what this asserts.
		"n": {press: confirmNo, honour: func(t *testing.T) bool {
			return themeConfirmNoHonoured(t, dispatch, confirmNo)
		}},
	}
}

// themeConfirmYesHonoured is the `y` arm's bound effect: the pending assignment
// written, the confirm resolved, and the panel still open.
func themeConfirmYesHonoured(t *testing.T, dispatch themePanelDispatch, press tea.KeyPressMsg) bool {
	t.Helper()

	m, persister := themeConfirmGuardModel(t)
	after := dispatchPanelKey(t, dispatch, m, press)
	return slices.Equal(persister.slotCommits, []slotCommit{themeConfirmPending}) &&
		!after.themePanel.confirming() && after.themePanel.open
}

// themeConfirmNoHonoured is the `n` arm's bound effect: the confirm resolved with
// the panel still open and NOTHING written.
func themeConfirmNoHonoured(t *testing.T, dispatch themePanelDispatch, press tea.KeyPressMsg) bool {
	t.Helper()

	m, persister := themeConfirmGuardModel(t)
	after := dispatchPanelKey(t, dispatch, m, press)
	return len(persister.slugs) == 0 && !after.themePanel.confirming() && after.themePanel.open
}

// themeConfirmPending is the assignment the seed below leaves outstanding: the row
// its cursor is parked on, into the slot its raise named. `y` writes exactly this and
// the other two answers write nothing, so the pending value and the raise that
// produces it are declared together rather than restated at each predicate.
var themeConfirmPending = slotCommit{slug: slotConfirmTarget(), slot: prefs.SlotLight}

// themeConfirmGuardModel is the confirm scope's seed: a CONSTANT setting with the
// cursor arrowed off the persisted row, and `l` pressed to raise §9.2's question.
//
// The constant is the whole condition — it is the only setting shape under which
// `d`/`l` ask rather than write — and the raise goes through the live dispatch, so a
// scope whose question stopped being raised fails here rather than leaving its two
// probes asserting against a panel that answers nothing.
func themeConfirmGuardModel(t *testing.T) (Model, *fakeThemePersister) {
	t.Helper()

	m, persister := newSlotConfirmModel(t)
	return raiseSlotConfirmForTest(t, m, slotLightPress, themeConfirmPending.slot), persister
}

// TestThemePanelDescriptorDispatchParity: it keeps the panel descriptor and
// dispatch in parity.
//
// The panel renders a bespoke VERTICAL footer from a descriptor of its own — the
// second place in the product a key label can go stale, and the drift class this
// guard exists to make a test failure rather than a bug report. The check is the
// SAME two-way one the three pages take: every non-help descriptor entry has a probe
// the live dispatch honours, and every probe's key appears in the descriptor.
func TestThemePanelDescriptorDispatchParity(t *testing.T) {
	assertDescriptorDispatchParity(t, "theme panel", themePanelKeymap(), themePanelProbes(livePanelDispatch))
}

// TestThemeConfirmDescriptorDispatchParity: it keeps the confirm descriptor and
// dispatch in parity.
//
// The confirm SUBSTITUTES its own footer while it is live (§9.2), which is the third
// such place, so it takes the same check as its own scope rather than riding on the
// panel's.
func TestThemeConfirmDescriptorDispatchParity(t *testing.T) {
	assertDescriptorDispatchParity(t, "theme confirm", themePanelConfirmKeymap(), themeConfirmProbes(livePanelDispatch))
}

// themePanelArmRemoved is the live routing with ONE arm deleted: the presses it
// names fall through to the swallow-everything default, which is exactly what
// removing a `case` from updateThemePanel leaves behind.
//
// It WRAPS the production dispatch rather than restating it, so every surviving arm
// is the real one and the guard-of-the-guard cannot pass against a stub that has
// quietly stopped resembling the thing it stands in for.
func themePanelArmRemoved(presses ...tea.KeyPressMsg) themePanelDispatch {
	return func(m Model, msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
		if slices.Contains(presses, msg) {
			return m, nil
		}
		return m.updateThemePanel(msg)
	}
}

// TestThemePanelParity_DetectsARemovedArm: it fails on a missing dispatch arm.
//
// The guard's own guard, in both directions of the parity contract. A suite that
// passes today says nothing about tomorrow unless deleting a dispatch arm — or
// advertising a key nothing dispatches — is shown to BREAK it, and the failure has to
// name the key that went: a check reporting six violations for one deleted arm sends
// the reader looking in five wrong places.
func TestThemePanelParity_DetectsARemovedArm(t *testing.T) {
	t.Run("a removed dispatch arm", func(t *testing.T) {
		for _, tc := range []struct {
			key     string
			presses []tea.KeyPressMsg
		}{
			{key: "↑↓", presses: []tea.KeyPressMsg{arrowUp, arrowDown}},
			{key: "^↑/↓", presses: []tea.KeyPressMsg{arrowPageUp, arrowPageDown}},
			{key: "⏎", presses: []tea.KeyPressMsg{commitEnter}},
			{key: "d", presses: []tea.KeyPressMsg{slotDarkPress}},
			{key: "l", presses: []tea.KeyPressMsg{slotLightPress}},
			{key: "esc", presses: []tea.KeyPressMsg{keyEsc}},
		} {
			t.Run(tc.key, func(t *testing.T) {
				probes := themePanelProbes(themePanelArmRemoved(tc.presses...))

				if probes[tc.key].honour(t) {
					t.Errorf("the %q probe passes with that arm deleted; it is not tied to the dispatch at all", tc.key)
				}
				requireDriftNaming(t, descriptorDispatchDrift(t, themePanelKeymap(), probes), tc.key)
			})
		}
	})

	t.Run("an unprobed descriptor entry", func(t *testing.T) {
		seventh := append(themePanelKeymap(), keymapEntry{Key: "z", Action: "zoom", Core: true})

		requireDriftNaming(t, descriptorDispatchDrift(t, seventh, themePanelProbes(livePanelDispatch)), "z")
	})

	t.Run("an unadvertised probe", func(t *testing.T) {
		probes := themePanelProbes(livePanelDispatch)
		probes["z"] = probes["esc"]

		requireDriftNaming(t, descriptorDispatchDrift(t, themePanelKeymap(), probes), "z")
	})
}

// requireDriftNaming fails unless the parity check reported EXACTLY ONE violation
// and it names key.
//
// The count is half the assertion: one broken binding must produce one report, or the
// guard tells a reader which key to look at by listing all of them.
func requireDriftNaming(t *testing.T, drift []string, key string) {
	t.Helper()

	if len(drift) != 1 {
		t.Fatalf("the parity check reported %d violations %v, want exactly the one naming %q", len(drift), drift, key)
	}
	if !strings.Contains(drift[0], strconv.Quote(key)) {
		t.Errorf("the parity check reported %q, want a violation naming %q", drift[0], key)
	}
}

// TestThemePanelParity_ProbesAssertEffects: it probes a bound effect rather than
// consumption.
//
// THE PANEL IS KEY-EXCLUSIVE (§9.7), so a probe asserting only that the key was
// SWALLOWED passes for every key in the alphabet — including one whose dispatch arm
// has just been deleted, which is the whole failure this guard exists to catch. Every
// probe therefore has to assert a state change the dispatch is supposed to CAUSE, and
// this is that requirement executed: driven against a dispatch that swallows
// everything and changes nothing, not one probe may pass.
func TestThemePanelParity_ProbesAssertEffects(t *testing.T) {
	swallow := themePanelDispatch(func(m Model, _ tea.KeyPressMsg) (tea.Model, tea.Cmd) { return m, nil })

	for _, scope := range []struct {
		name   string
		probes map[string]dispatchProbe
	}{
		{name: "theme panel", probes: themePanelProbes(swallow)},
		{name: "theme confirm", probes: themeConfirmProbes(swallow)},
	} {
		for _, key := range slices.Sorted(maps.Keys(scope.probes)) {
			t.Run(scope.name+" "+key, func(t *testing.T) {
				if scope.probes[key].honour(t) {
					t.Errorf("the %s probe for %q passes against a dispatch that swallows every key and changes nothing; it asserts CONSUMPTION rather than the bound effect, so a deleted dispatch arm would still satisfy it", scope.name, key)
				}
			})
		}
	}
}

// TestThemePanelDispatch_EscMeansInnermostFirst: it distinguishes panel close from
// confirm cancel.
//
// `Esc` is the one genuinely OVERLOADED key across the two scopes — §9.2 gives it the
// panel's close and the confirm's cancel, resolved innermost-first — so a probe that
// did not tell the two apart would prove nothing about either. The panel scope's
// `esc` probe asserts the CLOSE; this asserts both meanings side by side, which is
// what makes that probe's silence about the cancel a division of labour rather than a
// gap.
func TestThemePanelDispatch_EscMeansInnermostFirst(t *testing.T) {
	t.Run("with no confirm live it closes the panel", func(t *testing.T) {
		m, persister := themePanelGuardModel(t)
		if m.themePanel.confirming() {
			t.Fatal("fixture: the panel seed is already asking a question, so `Esc` would take the cancel")
		}

		after := dispatchPanelKey(t, livePanelDispatch, m, keyEsc)

		if after.themePanel.open {
			t.Error("`Esc` with no confirm live left the panel open; it is the ONLY way out (§9.2)")
		}
		if !themePanelStateDropped(after) {
			t.Errorf("the close retained panel state %+v, want the whole struct dropped so the next open re-reads (§5.8)", after.themePanel)
		}
		if len(persister.slugs) != 0 {
			t.Errorf("the close wrote %v; every write is an explicit commit key (§9.2)", persister.slugs)
		}
	})

	t.Run("with the confirm live it cancels and leaves the panel open", func(t *testing.T) {
		m, persister := themeConfirmGuardModel(t)

		after := dispatchPanelKey(t, livePanelDispatch, m, keyEsc)

		if !after.themePanel.open {
			t.Fatal("`Esc` closed the panel while the confirm was live; the innermost thing resolves first (§9.2)")
		}
		if after.themePanel.confirming() {
			t.Error("`Esc` left the confirm standing; it is one of the three inputs that resolve it (§9.2)")
		}
		if themePanelStateDropped(after) {
			t.Errorf("the cancel dropped the panel state %+v, want everything the CLOSE would have discarded left in place", after.themePanel)
		}
		if len(persister.slugs) != 0 {
			t.Errorf("the cancel wrote %v; nothing is written until `y` (§9.2)", persister.slugs)
		}
	})
}

// TestThemeConfirmDispatch_UppercaseReachesTheSameArm: it honours both cases of the
// confirm keys.
//
// The descriptor advertises the TERSE `y`/`n` glyphs — the footer form — while §9.2
// answers on either case, so the parity probes are pinned to the lowercase press and
// the uppercase half would sit outside the guard entirely. It is driven through the
// SAME effect predicates those probes use, so a `Y` reaching a different arm than `y`
// could not satisfy it.
//
// Both shapes an uppercase answer arrives in are covered: the ModShift press every
// terminal actually sends, and the bare `Mod == 0` press the matcher's case-fold is
// defensive against.
func TestThemeConfirmDispatch_UppercaseReachesTheSameArm(t *testing.T) {
	for _, tc := range []struct {
		name     string
		press    tea.KeyPressMsg
		honoured func(*testing.T, themePanelDispatch, tea.KeyPressMsg) bool
	}{
		{name: "shift+y", press: confirmYesShift, honoured: themeConfirmYesHonoured},
		{name: "Y", press: confirmYesUpper, honoured: themeConfirmYesHonoured},
		{name: "shift+n", press: confirmNoShift, honoured: themeConfirmNoHonoured},
		{name: "N", press: confirmNoUpper, honoured: themeConfirmNoHonoured},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if !tc.honoured(t, livePanelDispatch, tc.press) {
				t.Errorf("%q does not reach the arm its lowercase form does; the descriptor carries ONE glyph for both cases (§9.2)", tc.name)
			}
		})
	}
}

// TestThemePanelKeymap_NoRightAlignedEntry: it adds no right-aligned entry.
//
// The `?` help allow-list is the parity check's ONE exception and it is derived from
// the descriptor's own RightAligned flag, so an entry carrying that flag is silently
// exempted from needing a probe. Neither theme scope may have one: their footers are
// VERTICAL (§9.1), with no right anchor to pin an entry to, so the allow-list has
// nothing to exclude here and must not widen to admit anything.
func TestThemePanelKeymap_NoRightAlignedEntry(t *testing.T) {
	for _, scope := range themeKeymapScopes() {
		for _, e := range scope.entries {
			if e.RightAligned {
				t.Errorf("the %s scope's %q entry is RightAligned; the vertical footer has no right anchor, and the flag would exempt the key from the dispatch guard", scope.name, e.Key)
			}
		}
	}

	// The control: the flag is a LIVE carve-out on the horizontal page footers, so the
	// assertion above is about these scopes rather than about a flag nothing uses.
	for _, page := range []struct {
		name    string
		entries []keymapEntry
	}{
		{name: "sessions", entries: sessionsKeymap()},
		{name: "projects", entries: projectsKeymap()},
	} {
		if got := countRightAligned(page.entries); got != 1 {
			t.Errorf("control: the %s descriptor carries %d RightAligned entries, want the single `?` help hint", page.name, got)
		}
	}
}

// countRightAligned counts a descriptor's right-anchored entries.
func countRightAligned(entries []keymapEntry) int {
	n := 0
	for _, e := range entries {
		if e.RightAligned {
			n++
		}
	}
	return n
}

// TestThemePanelKeymap_NoHelpEntry: it binds no help key.
//
// §9.12: `?` does nothing inside the panel. There is no panel help modal — a help
// body over a non-blanking overlay would stack the shape §9.6 rejects — so the scope
// exists to drive the vertical footer and the guard, and `?` earns neither a
// descriptor entry nor a probe.
//
// `Ctrl-C` is the opposite omission and belongs beside it: it IS dispatched in both
// scopes and deliberately unadvertised, being the global quit §9.7 keeps live
// everywhere rather than a scope binding.
func TestThemePanelKeymap_NoHelpEntry(t *testing.T) {
	t.Run("neither scope advertises or probes ?", func(t *testing.T) {
		for _, scope := range themeKeymapScopes() {
			for _, e := range scope.entries {
				if e.Key == "?" || e.HelpKey == "?" {
					t.Errorf("the %s scope advertises a `?` entry; `?` does nothing inside the panel (§9.12)", scope.name)
				}
			}
			if _, probed := scope.probes["?"]; probed {
				t.Errorf("the %s scope carries a `?` probe; the key is swallowed, so there is no bound effect to assert", scope.name)
			}
		}
	})

	t.Run("the live dispatch swallows ?", func(t *testing.T) {
		m, persister := themePanelGuardModel(t)
		before := m.themePanel.list.Index()

		after := dispatchPanelKey(t, livePanelDispatch, m, tea.KeyPressMsg{Code: '?', Text: "?"})

		if !after.themePanel.open || after.modal != modalNone {
			t.Errorf("`?` inside the panel left open=%t modal=%v, want the panel standing with no modal (§9.12)", after.themePanel.open, after.modal)
		}
		if got := after.themePanel.list.Index(); got != before {
			t.Errorf("`?` moved the panel cursor to row %d, want it left on %d", got, before)
		}
		if len(persister.slugs) != 0 {
			t.Errorf("`?` wrote %v, want nothing", persister.slugs)
		}
	})

	t.Run("neither scope advertises the global quit", func(t *testing.T) {
		m, _ := themePanelGuardModel(t)
		if _, cmd := livePanelDispatch(m, confirmCtrlC); !isQuitCmd(cmd) {
			t.Fatal("control: `Ctrl-C` no longer quits inside the panel, so the descriptors' silence about it says nothing")
		}

		for _, scope := range themeKeymapScopes() {
			for _, e := range scope.entries {
				if slices.Contains(ctrlCGlyphs, e.Key) {
					t.Errorf("the %s scope advertises %q; `Ctrl-C` is the global quit that stays live everywhere, not a scope binding (§9.7)", scope.name, e.Key)
				}
			}
		}
	})
}

// ctrlCGlyphs are the forms the global quit would be written in if a scope ever
// advertised it.
var ctrlCGlyphs = []string{"^c", "^C", "ctrl+c", "ctrl-c", "Ctrl-C"}

// themeKeymapScopes pairs each theme scope's descriptor with its probe set, so the
// negatives above are stated once over both rather than twice over one.
func themeKeymapScopes() []struct {
	name    string
	entries []keymapEntry
	probes  map[string]dispatchProbe
} {
	return []struct {
		name    string
		entries []keymapEntry
		probes  map[string]dispatchProbe
	}{
		{name: "theme panel", entries: themePanelKeymap(), probes: themePanelProbes(livePanelDispatch)},
		{name: "theme confirm", entries: themePanelConfirmKeymap(), probes: themeConfirmProbes(livePanelDispatch)},
	}
}

// TestKeymapGuard_PageDescriptorsUnchanged: it leaves the page descriptors
// untouched.
//
// The theme scopes are NEW SCOPES BESIDE the three pages, never an extension of
// them: nothing about the panel's six keys or the confirm's two may reach
// sessionsKeymap / projectsKeymap / previewKeymap, and their probe sets are pinned by
// the parity check itself — direction 1 demands a probe for every non-help entry and
// direction 2 rejects a probe the descriptor omits, so those probe keys ARE the keys
// pinned below.
//
// The golden is the KEY ORDER AND THE FLAGS: what the dispatch guard and the footer
// read. The Action / HelpAction copy is pinned by the footer and help suites, so
// restating it here would duplicate their goldens rather than add a statement.
func TestKeymapGuard_PageDescriptorsUnchanged(t *testing.T) {
	for _, tc := range []struct {
		page string
		got  []keymapEntry
		want []string
	}{
		{page: "sessions", got: sessionsKeymap(), want: []string{
			"↑↓ core=false right=false destructive=false",
			"^↑/↓ core=false right=false destructive=false",
			"⏎ core=true right=false destructive=false",
			"/ core=true right=false destructive=false",
			"␣ core=true right=false destructive=false",
			"s core=true right=false destructive=false",
			"n core=false right=false destructive=false",
			"r core=false right=false destructive=false",
			"k core=false right=false destructive=true",
			"q core=false right=false destructive=false",
			"x core=true right=false destructive=false",
			"t core=true right=false destructive=false",
			"m core=true right=false destructive=false",
			"? core=true right=true destructive=false",
		}},
		{page: "projects", got: projectsKeymap(), want: []string{
			"↑↓ core=false right=false destructive=false",
			"^↑/↓ core=false right=false destructive=false",
			"⏎ core=true right=false destructive=false",
			"x core=true right=false destructive=false",
			"e core=true right=false destructive=false",
			"/ core=true right=false destructive=false",
			"t core=true right=false destructive=false",
			"d core=false right=false destructive=true",
			"n core=false right=false destructive=false",
			"q core=false right=false destructive=false",
			"esc core=false right=false destructive=false",
			"? core=true right=true destructive=false",
		}},
		{page: "preview", got: previewKeymap(), want: []string{
			"↑/↓ core=false right=false destructive=false",
			"^↑/↓ core=false right=false destructive=false",
			"Home/End core=false right=false destructive=false",
			"←→ core=true right=false destructive=false",
			"⇥ core=true right=false destructive=false",
			"⏎ core=true right=false destructive=false",
			"␣ core=true right=false destructive=false",
		}},
	} {
		t.Run(tc.page, func(t *testing.T) {
			if got := descriptorShape(tc.got); !slices.Equal(got, tc.want) {
				t.Errorf("the %s descriptor's keys/flags changed:\ngot  %v\nwant %v", tc.page, got, tc.want)
			}
		})
	}
}

// descriptorShape reduces a descriptor to the ordered keys and the flags the footer
// and the dispatch guard read.
func descriptorShape(entries []keymapEntry) []string {
	shapes := make([]string, 0, len(entries))
	for _, e := range entries {
		shapes = append(shapes, fmt.Sprintf("%s core=%t right=%t destructive=%t", e.Key, e.Core, e.RightAligned, e.Destructive))
	}
	return shapes
}
