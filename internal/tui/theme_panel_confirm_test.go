package tui

import (
	"go/ast"
	"maps"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// §9.2's SLOT-FROM-CONSTANT CONFIRM: the one gate in the panel, and the only place
// a keypress the user was told is inert can silently cost them a setting they
// chose.
//
// On `"theme": "nord"`, pressing `l` clears the constant, the untouched dark slot
// falls back to the shipped default, and `Esc` in a dark terminal lands on
// `tokyo-night` rather than `nord`. §9.2 puts a confirm in front of exactly that,
// and three of its properties are what make the tests below more than a yes/no
// prompt:
//
//   - IT IS KEY-EXCLUSIVE WITHIN THE PANEL. An arrow that moved the cursor
//     mid-question would re-theme the screen while the user is answering about a
//     row that has just stopped being the previewed one — so every key but the
//     three answers and `Ctrl-C` is swallowed, and the swallow table below carries
//     a POSITIVE CONTROL per key: a key that does nothing anyway proves nothing.
//   - `Esc` CANCELS RATHER THAN CLOSES. The innermost thing resolves first, the
//     same nesting rule the panel already applies over multi-select (§9.7), so the
//     assertion is made against the CLOSE path's own observable effects rather than
//     against the `open` flag alone.
//   - NOTHING IS WRITTEN UNTIL `y`. A cancel leaves the preview, the cursor, the
//     badges and the row set exactly as they were, and a forced close (§9.8) drops
//     the question with no partial state behind it.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// The confirm's three resolving inputs, in every shape that can reach the dispatch —
// the ones a terminal actually delivers plus one defensive pin — and the two
// near-misses that must NOT resolve it.
//
// AN UPPERCASE ANSWER ALWAYS CARRIES ModShift, on BOTH paths. A legacy byte is
// decoded by parseUtf8 into the base rune `y` with the original in ShiftedCode, the
// text "Y" and ModShift set (decoder.go:1097-1102); under the Kitty keyboard protocol
// the same keypress reports that same base rune with shift on the modifier parameter
// and the text repopulated to "Y". confirmYesShift/confirmNoShift are therefore what a
// terminal ACTUALLY sends for `Y`/`N` — and why a matcher requiring `Mod == 0` would
// leave §9.2's uppercase answer dead on EVERY terminal, not on a subset of them.
//
// confirmYesUpper/confirmNoUpper ARE THE DEFENSIVE `Mod == 0` SHAPE, not a second
// terminal encoding: no path in this stack hands the dispatch an uppercase Code with
// an empty modifier field for a shift-typed letter. They stay because they pin the
// matcher's EqualFold independently of its modifier mask — a case-sensitive matcher
// kills them and leaves the shift rows to the mask.
//
// CAPS LOCK IS THE OTHER ENCODING OF THAT SAME ANSWER. The Kitty decoder reports it
// as the base rune carrying ModCapsLock with the shifted text, so a matcher
// forgiving shift ALONE rejects a caps-lock user's `y` — §9.2's answer key, dead for
// the one class of user most likely to be typing capitals.
//
// NUM LOCK RIDES ALONG ON A PLAIN LOWERCASE `y`. It is not another encoding of the
// uppercase answer — it is the ordinary one, wearing a modifier the decoder strips
// from a local copy before repopulating the text and never from the event it emits
// (see themeConfirmAnswer's note on decoder.go:1474-1478). So Portal sees
// {Text:"y", Mod:ModNumLock}, and a mask forgiving only shift and caps lock kills `y`
// outright for anyone typing with num lock on.
//
// `Ctrl-Y` AND `Alt-N` CARRY NO Text. bubbletea documents Key.Text as empty for
// "key combos with modifier keys", and ultraviolet enforces it on every path (the
// Esc-prefixed alt path clears it, control bytes decode to a bare Code plus ModCtrl,
// and the Kitty repopulation gate skips anything past shift and the locks). So these
// two press shapes are what a terminal ACTUALLY delivers, and what the swallow table
// pins is the dispatch refusing them — the matcher's modifier mask is the
// belt-and-braces half of that refusal rather than the load-bearing one.
var (
	confirmYes         = tea.KeyPressMsg{Code: 'y', Text: "y"}
	confirmYesUpper    = tea.KeyPressMsg{Code: 'Y', Text: "Y"}
	confirmYesShift    = tea.KeyPressMsg{Code: 'y', Text: "Y", Mod: tea.ModShift}
	confirmYesCapsLock = tea.KeyPressMsg{Code: 'y', Text: "Y", Mod: tea.ModCapsLock}
	confirmYesNumLock  = tea.KeyPressMsg{Code: 'y', Text: "y", Mod: tea.ModNumLock}
	confirmYesCtrl     = tea.KeyPressMsg{Code: 'y', Mod: tea.ModCtrl}
	confirmNo          = tea.KeyPressMsg{Code: 'n', Text: "n"}
	confirmNoUpper     = tea.KeyPressMsg{Code: 'N', Text: "N"}
	confirmNoShift     = tea.KeyPressMsg{Code: 'n', Text: "N", Mod: tea.ModShift}
	confirmNoAlt       = tea.KeyPressMsg{Code: 'n', Mod: tea.ModAlt}
	confirmEsc         = tea.KeyPressMsg{Code: tea.KeyEscape}
	confirmCtrlC       = tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}
)

// slotConfirmRows is the confirm fixture's union: six selectable rows, each with
// its own palette, so a cursor move is observable as a whole-frame colour change
// and the paging fixture below has three pages to move between.
func slotConfirmRows() []theme.Row {
	return arrowValidRows(6)
}

// newSlotConfirmModel opens the panel over a CONSTANT (rows[0]) with the cursor
// ARROWED OFF the persisted row, which is the only shape in which the pending
// slug and the constant being cleared are distinguishable strings.
//
// The persister is attached through the production option after the open, exactly
// as the recompute fixture attaches its own: the open itself must not write, so a
// seam wired before it would be recording an event this suite never wants to see.
func newSlotConfirmModel(t *testing.T) (Model, *fakeThemePersister) {
	t.Helper()
	return newSlotConfirmModelAt(t, arrowTermH)
}

// newSlotConfirmModelAt is newSlotConfirmModel at a chosen terminal HEIGHT — the
// one input the panel's page size is a function of, which the swallow table needs
// so `Ctrl+↑`/`Ctrl+↓` have somewhere to move.
func newSlotConfirmModelAt(t *testing.T, termH int) (Model, *fakeThemePersister) {
	t.Helper()

	rows := slotConfirmRows()
	m := newArrowPanelModelAt(t, rows, rows[0].Slug, termH)
	persister := &fakeThemePersister{}
	WithThemePersister(persister)(&m)
	// The swallow table's `k` control needs a killer to reach the confirm modal with
	// the panel closed; a nil one makes the key a no-op for a reason that has nothing
	// to do with the panel.
	m.sessionKiller = keymapParityKiller{}

	m = arrowToThemeRow(t, m, slotConfirmTarget())
	if m.themeKeys.Theme == slotConfirmTarget() {
		t.Fatalf("fixture: the constant and the cursor both name %q, so nothing distinguishes them", m.themeKeys.Theme)
	}
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Fatalf("fixture: the panel opened with the message %+v, want an empty slot", got)
	}
	return m, persister
}

// slotConfirmConstant / slotConfirmTarget are the fixture's persisted constant and
// the row the cursor is parked on — the two slugs every assertion below tells
// apart.
func slotConfirmConstant() string { return arrowSlug(0) }
func slotConfirmTarget() string   { return arrowSlug(2) }

// raiseSlotConfirmForTest drives a slot key through the live Update and fails
// unless it raised the confirm, so a test whose subject is the ANSWER does not
// silently start from an un-raised panel.
func raiseSlotConfirmForTest(t *testing.T, m Model, press tea.KeyPressMsg, slot prefs.ThemeSlot) Model {
	t.Helper()
	return raiseSlotConfirmForTestAt(t, m, press, themeSlotConfirm{slug: slotConfirmTarget(), slot: slot})
}

// raiseSlotConfirmForTestAt is raiseSlotConfirmForTest for a fixture whose pending
// assignment is not this file's standard one — the nil-persister model, which is
// built from the COMMIT fixture's rows rather than the confirm fixture's.
func raiseSlotConfirmForTestAt(t *testing.T, m Model, press tea.KeyPressMsg, pending themeSlotConfirm) Model {
	t.Helper()

	raised, cmd := pressSlotKey(t, m, press)
	if cmd != nil {
		t.Fatalf("the raise scheduled %T; the question is asked on this keypress", cmd)
	}
	requireConfirmLive(t, raised, pending)
	return raised
}

// pressConfirmKey drives one key through the live Update while the confirm is up.
func pressConfirmKey(t *testing.T, m Model, press tea.KeyPressMsg) (Model, tea.Cmd) {
	t.Helper()
	updated, cmd := m.Update(press)
	return updated.(Model), cmd
}

// requireConfirmLive fails unless the panel is holding §9.2's confirm for the
// pending assignment: the message slot naming the PERSISTED constant, the pending
// slug and typed slot recorded, and the panel still open.
func requireConfirmLive(t *testing.T, m Model, pending themeSlotConfirm) {
	t.Helper()

	want := themePanelMessage{Kind: themeMessageConfirm, Slug: m.themeKeys.Theme}
	if got := m.themePanel.message; got != want {
		t.Errorf("the message slot holds %+v, want %+v — the confirm names the CONSTANT being cleared (§9.2)", got, want)
	}
	if got := m.themePanel.pending; got != pending {
		t.Errorf("the panel holds the pending assignment %+v, want %+v", got, pending)
	}
	if !m.themePanel.open {
		t.Error("the confirm closed the panel; it is inline, not a modal, and `Esc` is the only close (§9.2)")
	}
}

// requireConfirmResolved fails unless the confirm is gone AND the slot is empty —
// the ordinary resolution, where nothing took the row the question vacated.
func requireConfirmResolved(t *testing.T, m Model) {
	t.Helper()

	requireConfirmGone(t, m)
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("the resolved confirm left the message %+v, want the slot empty", got)
	}
}

// requireConfirmGone fails unless the confirm itself is gone — the question and the
// pending assignment, which are raised and cleared as one act.
//
// It is separate from requireConfirmResolved because §9.13's failed-commit line
// legitimately takes the slot the question vacated: the confirm is resolved there
// too, and asserting an EMPTY slot would read that report as a confirm still
// standing.
func requireConfirmGone(t *testing.T, m Model) {
	t.Helper()

	if m.themePanel.confirming() {
		t.Errorf("the confirm is still live, holding %+v", m.themePanel.message)
	}
	if got := m.themePanel.pending; got != (themeSlotConfirm{}) {
		t.Errorf("the resolved confirm left the pending assignment %+v, want the zero value", got)
	}
	if !m.themePanel.open {
		t.Error("resolving the confirm closed the panel; only `Esc` with NO confirm live closes it (§9.2)")
	}
}

// slotConfirmPanelText is the panel as a user reads it: rendered at the height it
// is composited into, with every SGR sequence stripped.
func slotConfirmPanelText(m Model) string {
	return ansi.Strip(renderRecomputePanel(m))
}

// slotConfirmPanelCopy reduces the whole rendered panel to one whitespace-collapsed
// reading, so a footer row's fixed key column and pad-to-width collapse away and
// §14A's pinned phrases can be matched verbatim (themePanelFooterCopy's rule
// applied to the block).
func slotConfirmPanelCopy(m Model) string {
	return themePanelFooterCopy(slotConfirmPanelText(m))
}

// requireConfirmFooter fails unless the panel is rendering §9.2's NESTED CONFIRM
// SCOPE — `y confirm` / `n cancel` — and none of the standing scope's four rows.
//
// The standing footer advertises four keys of which NONE would act during a
// confirm, and §14.3 is firm that advertising a key that will not act is the dead
// end a proactive block exists to prevent.
func requireConfirmFooter(t *testing.T, m Model) {
	t.Helper()

	footer := slotConfirmPanelCopy(m)
	for _, want := range []string{"y confirm", "n cancel"} {
		if !strings.Contains(footer, want) {
			t.Errorf("the panel footer does not read %q while the confirm is live:\n%s", want, slotConfirmPanelText(m))
		}
	}
	for _, gone := range themePanelFooterPinnedRows() {
		if strings.Contains(footer, gone) {
			t.Errorf("the panel footer still advertises %q while the confirm is live; none of the standing keys would act (§9.2)\n%s", gone, slotConfirmPanelText(m))
		}
	}
}

// requireStandingFooter fails unless the panel is back on §14A's four-row footer
// with no trace of the confirm's substituted rows.
func requireStandingFooter(t *testing.T, m Model) {
	t.Helper()

	footer := slotConfirmPanelCopy(m)
	for _, want := range themePanelFooterPinnedRows() {
		if !strings.Contains(footer, want) {
			t.Errorf("the panel footer does not read %q once the confirm resolved:\n%s", want, slotConfirmPanelText(m))
		}
	}
	for _, gone := range []string{"y confirm", "n cancel"} {
		if strings.Contains(footer, gone) {
			t.Errorf("the panel footer still advertises the confirm's %q after it resolved\n%s", gone, slotConfirmPanelText(m))
		}
	}
}

// TestSlotConfirm_RaisedByDAndLOverAConstant: it raises the confirm on a constant.
//
// §9.2: "**Assigning a slot while a constant is set asks for confirmation
// first.** This is the one place a keypress described as inert can silently cost
// the user a setting they chose." So the keypress writes NOTHING, records what it
// would write, names the constant it would clear, and swaps the footer to the
// question's own keys.
func TestSlotConfirm_RaisedByDAndLOverAConstant(t *testing.T) {
	for _, tc := range []struct {
		name  string
		press tea.KeyPressMsg
		slot  prefs.ThemeSlot
	}{
		{name: "d", press: slotDarkPress, slot: prefs.SlotDark},
		{name: "l", press: slotLightPress, slot: prefs.SlotLight},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, persister := newSlotConfirmModel(t)
			previewed := m.activeTheme
			index := m.themePanel.list.Index()

			m, cmd := pressSlotKey(t, m, tc.press)

			if len(persister.slugs) != 0 {
				t.Errorf("%q wrote %v while raising the confirm; nothing is written until `y` (§9.2)", tc.name, persister.slugs)
			}
			requireConfirmLive(t, m, themeSlotConfirm{slug: slotConfirmTarget(), slot: tc.slot})
			requireConstantKeys(t, m, slotConfirmConstant())
			requireConfirmFooter(t, m)
			if want := "clear constant " + slotConfirmConstant() + "?  y / n"; !strings.Contains(slotConfirmPanelText(m), want) {
				t.Errorf("the message slot does not read %q:\n%s", want, slotConfirmPanelText(m))
			}
			if got := m.themePanel.list.Index(); got != index {
				t.Errorf("raising the confirm moved the cursor to row %d, want it left on %d", got, index)
			}
			if m.activeTheme != previewed {
				t.Errorf("raising the confirm rendered canvas %s, want the previewed %s left alone", m.activeTheme.Canvas.Value, previewed.Canvas.Value)
			}
			if cmd != nil {
				t.Errorf("%q scheduled %T; the question is asked on this keypress", tc.name, cmd)
			}
		})
	}
}

// TestSlotConfirm_UnselectableRowAsksNothing: it asks nothing on a non-selectable
// row.
//
// The same DEFENSIVE guard `Enter` carries (task 9-2), at the site where it decides
// the most: the slug the raise records is what a question the user has ALREADY
// ANSWERED goes on to write. An unguarded raise here would ask "clear constant X?"
// about an empty slug and, on `y`, clear the constant AND write an empty slot value
// — the silent loss §9.2's confirm exists to prevent, arrived at THROUGH the
// confirm.
//
// STRUCTURALLY UNREACHABLE, exactly as it is for `Enter`: the arrows skip
// unselectable rows (task 8-9) and the open-time anchor lands on a selectable one
// (task 8-8). The cursor is therefore placed DIRECTLY, bypassing both.
func TestSlotConfirm_UnselectableRowAsksNothing(t *testing.T) {
	// requireNothingAsked fails unless the keypress left NO question on screen and
	// nothing pending for a later `y` to write.
	requireNothingAsked := func(t *testing.T, m Model, p *fakeThemePersister, rows []theme.Row) {
		t.Helper()

		if got := m.themePanel.message; got.Kind != themeMessageNone {
			t.Errorf("`d` raised the message %+v on a row it cannot commit; an empty slug is not a setting to ask about (§9.2)", got)
		}
		if got := m.themePanel.pending; got != (themeSlotConfirm{}) {
			t.Errorf("`d` recorded the pending assignment %+v, want the zero value — a `y` would go on to write it", got)
		}
		requireSlotCommits(t, p)
		requireConstantKeys(t, m, rows[0].Slug)
		if !m.themePanel.open {
			t.Error("`d` on a row it cannot commit closed the panel")
		}
	}

	t.Run("an unselectable row", func(t *testing.T) {
		rows := []theme.Row{arrowValidRow(arrowSlug(0), 0), arrowInvalidRow(arrowSlug(1))}
		m, persister := newCommitPanelModel(t, rows, rows[0].Slug)
		m.themePanel.list.Select(1)
		if themePanelCursorRow(t, m).Selectable() {
			t.Fatal("fixture: the cursor is on a selectable row, so there is no refusal to exercise")
		}

		m, cmd := pressSlotKey(t, m, slotDarkPress)

		requireNothingAsked(t, m, persister, rows)
		if cmd != nil {
			t.Errorf("`d` on an unselectable row scheduled %T, want nothing", cmd)
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

		m, _ = pressSlotKey(t, m, slotDarkPress)

		requireNothingAsked(t, m, persister, rows)
	})

	// Positive control: the same keypress on a SELECTABLE row DOES ask, so the two
	// refusals above are a guard rather than an unwired arm.
	t.Run("a selectable row does ask", func(t *testing.T) {
		rows := []theme.Row{arrowValidRow(arrowSlug(0), 0), arrowInvalidRow(arrowSlug(1))}
		m, persister := newCommitPanelModel(t, rows, rows[0].Slug)

		m, _ = pressSlotKey(t, m, slotDarkPress)

		requireConfirmLive(t, m, themeSlotConfirm{slug: rows[0].Slug, slot: prefs.SlotDark})
		requireSlotCommits(t, persister)
	})
}

// TestSlotConfirm_ConfirmsOnEitherCase: it commits on y and Y.
//
// §9.2: "`y` or `Y` confirms — the constant is cleared and the slot written, in one
// atomic prefs write." Both cases are asserted, and the uppercase one in every shape
// that can reach the dispatch (see the press vars): the ModShift shape BOTH terminal
// paths converge on, the caps-lock shape — dead on a matcher forgiving shift alone,
// for the very users most likely to be typing capitals — and the bare `Mod == 0`
// shape the mask is defensive against, which pins the EqualFold on its own. The
// num-lock row is the LOWERCASE answer with a stray lock modifier the decoder emits
// but does not mean, and it is here for the same reason as the caps-lock one: a lock
// key must never make an answer key dead.
func TestSlotConfirm_ConfirmsOnEitherCase(t *testing.T) {
	for _, tc := range []struct {
		name  string
		press tea.KeyPressMsg
	}{
		{name: "y", press: confirmYes},
		{name: "Y", press: confirmYesUpper},
		{name: "shift+y", press: confirmYesShift},
		{name: "capslock+y", press: confirmYesCapsLock},
		{name: "numlock+y", press: confirmYesNumLock},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, persister := newSlotConfirmModel(t)
			m = raiseSlotConfirmForTest(t, m, slotLightPress, prefs.SlotLight)

			m, cmd := pressConfirmKey(t, m, tc.press)

			requireSlotCommits(t, persister, slotCommit{slug: slotConfirmTarget(), slot: prefs.SlotLight})
			requirePairKeys(t, m, slotConfirmTarget(), "")
			requireConfirmResolved(t, m)
			requireStandingFooter(t, m)
			if cmd != nil {
				t.Errorf("the answer scheduled %T; the write lands on this keypress", cmd)
			}
		})
	}
}

// TestSlotConfirm_CancelsOnThreeInputs: it cancels on n, N and Esc.
//
// §9.2: "`n`, `N` or `Esc` cancels, leaving the panel open and nothing written."
// The uppercase shapes ride along for the same reason `Y`'s do — confirmNoShift is
// what a terminal sends, confirmNoUpper the defensive `Mod == 0` pin on EqualFold.
func TestSlotConfirm_CancelsOnThreeInputs(t *testing.T) {
	for _, tc := range []struct {
		name  string
		press tea.KeyPressMsg
	}{
		{name: "n", press: confirmNo},
		{name: "N", press: confirmNoUpper},
		{name: "shift+n", press: confirmNoShift},
		{name: "esc", press: confirmEsc},
	} {
		t.Run(tc.name, func(t *testing.T) {
			m, persister := newSlotConfirmModel(t)
			keys := m.themeKeys
			m = raiseSlotConfirmForTest(t, m, slotDarkPress, prefs.SlotDark)

			m, cmd := pressConfirmKey(t, m, tc.press)

			if len(persister.slugs) != 0 {
				t.Errorf("%q wrote %v; a cancel writes nothing (§9.2)", tc.name, persister.slugs)
			}
			if m.themeKeys != keys {
				t.Errorf("%q left keys %+v, want the untouched %+v", tc.name, m.themeKeys, keys)
			}
			requireConfirmResolved(t, m)
			requireStandingFooter(t, m)
			if cmd != nil {
				t.Errorf("%q scheduled %T, want nothing", tc.name, cmd)
			}
		})
	}
}

// TestSlotConfirm_EscCancelsNotCloses: it does not close the panel on Esc.
//
// §9.2: "`Esc` cancels the confirm rather than closing the panel, because the
// innermost thing resolves first — the same nesting rule §9.7 applies to the panel
// over multi-select."
//
// The `open` flag alone is a weak statement (a close that reopened would satisfy
// it), so this is asserted against the CLOSE path's own observable effects: §5.8's
// retained enumeration, the union, the badge table, the sized list — and the
// PREVIEW, which the close discards in favour of the resolved persisted state. The
// enumeration is still retained, so the next `Esc` does the close it did not do.
func TestSlotConfirm_EscCancelsNotCloses(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Theme: "aurora"})
	requireCursorOn(t, m, "aurora")

	m = arrowToThemeRow(t, m, "nord")
	previewed := m.activeTheme
	if previewed == themePanelRowFor(t, m, "aurora").Row.Theme {
		t.Fatal("fixture: the previewed row paints the persisted theme, so a close would be invisible")
	}
	m, _ = pressSlotKey(t, m, slotDarkPress)
	requireConfirmLive(t, m, themeSlotConfirm{slug: "nord", slot: prefs.SlotDark})

	m, cmd := pressConfirmKey(t, m, confirmEsc)

	if !m.themePanel.open {
		t.Fatal("`Esc` closed the panel while the confirm was live; the innermost thing resolves first (§9.2)")
	}
	if got := m.themePanel.enumeration; len(got.Entries) == 0 || got.DirPath != dir {
		t.Errorf("the cancel discarded the enumeration (%d entries from %q), want the retained read of %q — that is the close's own effect (§5.8)", len(got.Entries), got.DirPath, dir)
	}
	if len(m.themePanel.union.Rows) == 0 || m.themePanel.badges == nil || len(m.themePanel.list.Items()) == 0 || m.themePanel.width == 0 {
		t.Errorf("the cancel discarded panel state %+v, want everything the close would have dropped left in place", m.themePanel)
	}
	if m.activeTheme != previewed {
		t.Errorf("the cancel rendered canvas %s, want the previewed %s — the close is what discards a preview (§9.2)", m.activeTheme.Canvas.Value, previewed.Canvas.Value)
	}
	if len(persister.slugs) != 0 {
		t.Errorf("the cancel wrote %v, want nothing", persister.slugs)
	}
	if cmd != nil {
		t.Errorf("the cancel scheduled %T, want nothing", cmd)
	}

	// The control: with no confirm live the SAME key takes the close it just
	// declined to take, so the assertions above are the nesting rule rather than an
	// `Esc` that stopped working.
	m = closeThemePanelForTest(t, m)
	if m.themePanel.open {
		t.Fatal("control: `Esc` with no confirm live did not close the panel")
	}
	if got := m.themePanel.enumeration; len(got.Entries) != 0 {
		t.Errorf("control: the close retained %d enumeration entries, want the zero value", len(got.Entries))
	}
	if m.activeTheme == previewed {
		t.Errorf("control: the close kept the previewed canvas %s, so the cancel's preview assertion says nothing", m.activeTheme.Canvas.Value)
	}
}

// TestSlotConfirm_CtrlCQuits: it keeps Ctrl-C live.
//
// §9.2: "`Ctrl-C` quits Portal, per §9.7. It is not a cancel; it stays live
// everywhere" — swallowing it would take away the user's exit key inside a
// settings surface. It is also not a write: the question was never answered.
func TestSlotConfirm_CtrlCQuits(t *testing.T) {
	m, persister := newSlotConfirmModel(t)
	m = raiseSlotConfirmForTest(t, m, slotDarkPress, prefs.SlotDark)

	quit, cmd := pressConfirmKey(t, m, confirmCtrlC)

	if !isQuitCmd(cmd) {
		t.Errorf("`Ctrl-C` scheduled %T while the confirm was live, want tea.Quit", cmd)
	}
	if len(persister.slugs) != 0 {
		t.Errorf("`Ctrl-C` wrote %v; it is not a confirm", persister.slugs)
	}
	if quit.themeKeys != m.themeKeys {
		t.Errorf("`Ctrl-C` left keys %+v, want the untouched %+v", quit.themeKeys, m.themeKeys)
	}
}

// TestSlotConfirm_SwallowsEverythingElse: it swallows every other key.
//
// §9.2: "Every other key is swallowed — arrows, `Enter`, the other slot key, all of
// it. The confirm persists until one of the three above resolves it."
//
// EVERY ROW CARRIES A POSITIVE CONTROL, because a key that does nothing anyway
// proves nothing about a swallow: the panel already swallows the page's own keys
// (§9.7), so `k`, `x`, `m`, `/` and `?` are driven with the panel CLOSED and the
// panel's own keys with it OPEN and no confirm live. What is asserted of the live
// case is the whole frame, byte for byte, plus the confirm still standing — any
// effect a key could have had is a change to one of those.
//
// THE SECOND `l` IS DELIBERATELY IN THE TABLE ALONGSIDE `d`. A re-raise of the SAME
// key with the same cursor is state-identical by construction, so what that row
// pins is that it neither commits nor resolves the question; the OTHER slot key is
// where a re-raise is observable, since it would flip the pending slot.
func TestSlotConfirm_SwallowsEverythingElse(t *testing.T) {
	cursorMoved := func(before, after Model) bool {
		return after.themePanel.list.Index() != before.themePanel.list.Index()
	}
	openControl := func(effect func(before, after Model) bool) func(*testing.T, Model, tea.KeyPressMsg) bool {
		return func(t *testing.T, base Model, press tea.KeyPressMsg) bool {
			t.Helper()
			return effect(base, pressPanelKey(t, base, press))
		}
	}
	closedControl := func(effect func(before, after Model) bool) func(*testing.T, Model, tea.KeyPressMsg) bool {
		return func(t *testing.T, base Model, press tea.KeyPressMsg) bool {
			t.Helper()
			closed := closeThemePanelForTest(t, base)
			return effect(closed, pressPanelKey(t, closed, press))
		}
	}
	// answerControl is the control the two MODIFIED answer letters need: their
	// control is not the same key elsewhere but the UNMODIFIED letter here, which
	// does resolve the confirm. It is driven against a confirm raised on the base
	// model, which is the only state in which either letter means anything at all.
	answerControl := func(answer tea.KeyPressMsg, effect func(before, after Model) bool) func(*testing.T, Model, tea.KeyPressMsg) bool {
		return func(t *testing.T, base Model, _ tea.KeyPressMsg) bool {
			t.Helper()
			raised := raiseSlotConfirmForTest(t, base, slotLightPress, prefs.SlotLight)
			answered, _ := pressConfirmKey(t, raised, answer)
			return effect(raised, answered)
		}
	}

	for _, tc := range []struct {
		name     string
		press    tea.KeyPressMsg
		control  func(*testing.T, Model, tea.KeyPressMsg) bool
		controlS string
	}{
		{name: "↑", press: arrowUp, control: openControl(cursorMoved), controlS: "move the panel cursor"},
		{name: "↓", press: arrowDown, control: openControl(cursorMoved), controlS: "move the panel cursor"},
		{name: "ctrl+↑", press: arrowPageUp, control: openControl(cursorMoved), controlS: "page the panel cursor"},
		{name: "ctrl+↓", press: arrowPageDown, control: openControl(cursorMoved), controlS: "page the panel cursor"},
		{
			name:  "enter",
			press: commitEnter,
			control: openControl(func(before, after Model) bool {
				return after.themeKeys.Theme != before.themeKeys.Theme
			}),
			controlS: "commit the cursor's slug as the constant",
		},
		{
			name:  "d",
			press: slotDarkPress,
			control: openControl(func(_, after Model) bool {
				return after.themePanel.pending == themeSlotConfirm{slug: slotConfirmTarget(), slot: prefs.SlotDark}
			}),
			controlS: "raise the dark-slot confirm",
		},
		{
			name:  "l",
			press: slotLightPress,
			control: openControl(func(_, after Model) bool {
				return after.themePanel.pending == themeSlotConfirm{slug: slotConfirmTarget(), slot: prefs.SlotLight}
			}),
			controlS: "raise the light-slot confirm",
		},
		{
			name:  "ctrl+y",
			press: confirmYesCtrl,
			control: answerControl(confirmYes, func(before, after Model) bool {
				return after.themeKeys != before.themeKeys
			}),
			controlS: "resolve the confirm as a `y` when the letter carries no ctrl",
		},
		{
			name:  "alt+n",
			press: confirmNoAlt,
			control: answerControl(confirmNo, func(before, after Model) bool {
				return !after.themePanel.confirming() && after.themeKeys == before.themeKeys
			}),
			controlS: "resolve the confirm as an `n` when the letter carries no alt",
		},
		{
			name:  "t",
			press: tea.KeyPressMsg{Code: 't', Text: "t"},
			control: closedControl(func(_, after Model) bool {
				return after.themePanel.open
			}),
			controlS: "open the theme panel",
		},
		{
			name:  "?",
			press: tea.KeyPressMsg{Code: '?', Text: "?"},
			control: closedControl(func(_, after Model) bool {
				return after.modal == modalHelp
			}),
			controlS: "open the help modal",
		},
		{
			name:  "k",
			press: tea.KeyPressMsg{Code: 'k', Text: "k"},
			control: closedControl(func(_, after Model) bool {
				return after.modal == modalKillConfirm
			}),
			controlS: "open the kill confirm modal",
		},
		{
			name:  "x",
			press: tea.KeyPressMsg{Code: 'x', Text: "x"},
			control: closedControl(func(_, after Model) bool {
				return after.activePage == PageProjects
			}),
			controlS: "switch to the Projects page",
		},
		{
			name:  "m",
			press: tea.KeyPressMsg{Code: 'm', Text: "m"},
			control: closedControl(func(_, after Model) bool {
				return after.MultiSelectActive()
			}),
			controlS: "enter multi-select",
		},
		{
			name:  "/",
			press: tea.KeyPressMsg{Code: '/', Text: "/"},
			control: closedControl(func(_, after Model) bool {
				return after.sessionList.SettingFilter()
			}),
			controlS: "focus the filter",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The PAGING height is what gives `Ctrl+↑`/`Ctrl+↓` somewhere to move: at
			// the standard fixture height every row fits on one page and the two would
			// be inert for a reason that has nothing to do with the confirm.
			base, _ := newSlotConfirmModelAt(t, arrowPagingTermH)
			requireArrowPanelPageSize(t, base, arrowPagingPerPage)

			if !tc.control(t, base, tc.press) {
				t.Fatalf("precondition: %v does not %s with no confirm live, so a swallow proves nothing", tc.press, tc.controlS)
			}

			live, persister := newSlotConfirmModelAt(t, arrowPagingTermH)
			live = raiseSlotConfirmForTest(t, live, slotLightPress, prefs.SlotLight)
			frame := live.View().Content

			got, cmd := pressConfirmKey(t, live, tc.press)

			requireConfirmLive(t, got, themeSlotConfirm{slug: slotConfirmTarget(), slot: prefs.SlotLight})
			if len(persister.slugs) != 0 {
				t.Errorf("%v wrote %v while the confirm was live; only `y` writes (§9.2)", tc.press, persister.slugs)
			}
			if got.themeKeys != live.themeKeys {
				t.Errorf("%v left keys %+v, want the untouched %+v", tc.press, got.themeKeys, live.themeKeys)
			}
			if index, want := got.themePanel.list.Index(), live.themePanel.list.Index(); index != want {
				t.Errorf("%v moved the cursor to row %d, want it left on %d — an arrow mid-question would re-theme the screen behind the answer (§9.2)", tc.press, index, want)
			}
			if got.activeTheme != live.activeTheme {
				t.Errorf("%v rendered canvas %s, want the previewed %s left alone", tc.press, got.activeTheme.Canvas.Value, live.activeTheme.Canvas.Value)
			}
			if got.activePage != live.activePage || got.modal != modalNone || got.MultiSelectActive() || got.sessionList.SettingFilter() {
				t.Errorf("%v reached the page beneath the panel (page %d, modal %d, multi-select %v, filtering %v)",
					tc.press, got.activePage, got.modal, got.MultiSelectActive(), got.sessionList.SettingFilter())
			}
			if cmd != nil {
				t.Errorf("%v scheduled %T while the confirm was live, want nothing", tc.press, cmd)
			}
			if after := got.View().Content; after != frame {
				t.Errorf("%v changed the composed frame\nbefore: %q\nafter:  %q", tc.press, escSeq(frame), escSeq(after))
			}
		})
	}
}

// TestSlotConfirm_CancelIsInert: it leaves everything untouched on cancel.
//
// §9.2: a cancel leaves "the panel open and nothing written". The panel the user is
// left looking at must therefore be the panel they were looking at before they
// pressed the key — preview, cursor, badges and row set — which is asserted here as
// the composed frame byte for byte plus each of those four values.
//
// The RAISE is the control for the frame comparison: it genuinely changes the frame
// (message slot in, footer swapped), so "the frame came back" is a restoration
// rather than a keypress that never rendered anything.
func TestSlotConfirm_CancelIsInert(t *testing.T) {
	m, persister := newSlotConfirmModel(t)
	keys := m.themeKeys
	index := m.themePanel.list.Index()
	previewed := m.activeTheme
	badges := maps.Clone(m.themePanel.badges)
	labels := themePanelRowLabels(m)
	frame := m.View().Content

	raised := raiseSlotConfirmForTest(t, m, slotLightPress, prefs.SlotLight)
	if raised.View().Content == frame {
		t.Fatal("fixture: raising the confirm did not change the frame, so restoring it asserts nothing")
	}

	m, _ = pressConfirmKey(t, raised, confirmNo)

	if len(persister.slugs) != 0 {
		t.Errorf("the cancel wrote %v, want nothing", persister.slugs)
	}
	if m.themeKeys != keys {
		t.Errorf("the cancel left keys %+v, want the untouched %+v", m.themeKeys, keys)
	}
	if got := m.themePanel.list.Index(); got != index {
		t.Errorf("the cancel left the cursor on row %d, want %d", got, index)
	}
	if m.activeTheme != previewed {
		t.Errorf("the cancel rendered canvas %s, want the previewed %s", m.activeTheme.Canvas.Value, previewed.Canvas.Value)
	}
	if got := m.themePanel.badges; !maps.Equal(got, badges) {
		t.Errorf("the cancel left badges %v, want the untouched %v", got, badges)
	}
	if got := themePanelRowLabels(m); !slices.Equal(got, labels) {
		t.Errorf("the cancel left rows %v, want the untouched %v", got, labels)
	}
	if got := m.View().Content; got != frame {
		t.Errorf("the cancel did not restore the pre-raise frame\nbefore: %q\nafter:  %q", escSeq(frame), escSeq(got))
	}
}

// TestSlotConfirm_AtomicConstantClearPlusSlot: it clears the constant and writes
// the slot atomically.
//
// §9.2: "the constant is cleared and the slot written, in ONE atomic prefs write",
// which task 6-2 pinned inside prefs.SaveThemeSlot. What the confirm owes it is
// that the panel asks for it ONCE — no second call to clear the constant — and that
// the recompute then moves the badges: the constant's bare `●` is gone and the
// committed slot's `●` is on the row the cursor was parked on.
//
// The fixture runs over a REAL loader and a REAL directory, for task 9-2's reason:
// a stub seam answering with a fixed union would make every row and every `●` a
// statement about the fixture rather than about the derivation the commit drives.
func TestSlotConfirm_AtomicConstantClearPlusSlot(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Theme: "aurora"})
	requireBadge(t, m, "aurora", theme.BadgeConstant)
	requireBadgeText(t, m, 1, 0, 0)

	m = arrowToThemeRow(t, m, "nord")
	m, _ = pressSlotKey(t, m, slotLightPress)
	requireConfirmLive(t, m, themeSlotConfirm{slug: "nord", slot: prefs.SlotLight})

	m, _ = pressConfirmKey(t, m, confirmYes)

	requireSlotCommits(t, persister, slotCommit{slug: "nord", slot: prefs.SlotLight})
	if len(persister.slugs) != 1 {
		t.Errorf("the answer made %d seam call(s) (%v), want the single atomic slot write (§8.2)", len(persister.slugs), persister.slugs)
	}
	requirePairKeys(t, m, "nord", "")
	requireConfirmResolved(t, m)
	requireStandingFooter(t, m)
	requireBadge(t, m, "nord", theme.BadgeLight)
	requireBadge(t, m, "aurora", theme.BadgeNone)
	// The dark slot was never set, so §9.5's third badge row puts its `●` on the
	// shipped default — and the constant's bare `●` is gone from the panel entirely.
	requireBadge(t, m, theme.DefaultDarkSlug, theme.BadgeDark)
	requireBadgeText(t, m, 0, 1, 1)

	// §9.3's other half is a FILE rather than an answer: the slot the user did not
	// assign was never loaded at construction and becomes live on this keypress.
	// That load is task 9-6's, reached from the one arm that can create the state.
	if got := themePanelSeamCallers(t, "loadNewlyLiveSlot"); !slices.Equal(got, []string{"confirmSlotAssignment"}) {
		t.Errorf("the newly-live-slot seam is called from %v, want exactly [confirmSlotAssignment] — §8.4's load has one route in", got)
	}
}

// TestSlotConfirm_FailedCommitKeepsTheConstant: it keeps the constant in memory
// when the write fails.
//
// §9.13: a failed commit "does not move the `●` — the marker means 'what is
// persisted' and would be lying if it moved". The mechanism is that nothing is
// mutated: the constant is cleared in the WRITE (§8.2), and this write did not
// land, so the panel still marks it with the bare `●`.
//
// The confirm itself resolves EITHER WAY — the question has been answered, so the
// footer comes back — and §9.13's line then takes the slot the question vacated.
// That ordering is what makes §9.1's two contenders mutually exclusive on this path:
// the confirm gates the write, so by the time a write can fail it has already
// resolved.
func TestSlotConfirm_FailedCommitKeepsTheConstant(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Theme: "aurora"})
	keys := m.themeKeys
	badges := maps.Clone(m.themePanel.badges)
	labels := themePanelRowLabels(m)

	m = arrowToThemeRow(t, m, "nord")
	previewed := m.activeTheme
	persister.err = errThemeCommitFailed
	m, _ = pressSlotKey(t, m, slotDarkPress)

	m, cmd := pressConfirmKey(t, m, confirmYes)

	requireSlotCommits(t, persister, slotCommit{slug: "nord", slot: prefs.SlotDark})
	if m.themeKeys != keys {
		t.Errorf("a failed commit left keys %+v, want the untouched %+v — §8.2 clears the constant in the WRITE, and this write did not land", m.themeKeys, keys)
	}
	if got := m.themePanel.badges; !maps.Equal(got, badges) {
		t.Errorf("a failed commit left badges %v, want the untouched %v — a failed commit does not move the `●` (§9.13)", got, badges)
	}
	requireBadge(t, m, "aurora", theme.BadgeConstant)
	if got := themePanelRowLabels(m); !slices.Equal(got, labels) {
		t.Errorf("a failed commit left rows %v, want the untouched %v — only a SUCCESSFUL commit recomputes (§9.2)", got, labels)
	}
	requireConfirmGone(t, m)
	requireStandingFooter(t, m)
	if m.activeTheme != previewed {
		t.Errorf("a failed commit rendered canvas %s, want the previewed %s — §9.13 KEEPS the theme applied in memory", m.activeTheme.Canvas.Value, previewed.Canvas.Value)
	}
	if cmd != nil {
		t.Errorf("a failed commit scheduled %T, want nothing", cmd)
	}

	// The REPORT itself, which is the confirm's whole share of §9.13: the question is
	// down, the standing footer is back, and the failed-commit line is what the slot
	// now holds.
	requireCommitFailedMessage(t, m)
	if !m.themeCommitFailed {
		t.Error("a failed confirmed commit left no outstanding failure; the state runs until a commit SUCCEEDS (§9.13)")
	}

	// And it is the SHARED path's report rather than a second copy of the semantics:
	// the confirm writes through commitSlot, which is one of the two routes into the
	// handler that owns them.
	if got := themePanelSeamCallers(t, "applyCommitResult"); !slices.Equal(got, []string{"commitConstant", "commitSlot"}) {
		t.Errorf("§9.13's result handler is called from %v, want exactly [commitConstant commitSlot] — the failure semantics live in one place", got)
	}

	// The control: the same fixture with the write landing DOES clear the constant
	// and move the `●`, so the untouched state above is the failure path rather than
	// a confirm that never committed.
	landed, _, _ := newRecomputePanelModel(t, dir, theme.RawKeys{Theme: "aurora"})
	landed = arrowToThemeRow(t, landed, "nord")
	landed, _ = pressSlotKey(t, landed, slotDarkPress)
	landed, _ = pressConfirmKey(t, landed, confirmYes)
	requirePairKeys(t, landed, "", "nord")
	requireBadge(t, landed, "aurora", theme.BadgeNone)
}

// TestSlotConfirm_NilPersisterIsInert: it tolerates a nil persister, and reaches
// no newly-live-slot load.
//
// A fixture or `capturetool` model carries NO persister (task 6-7), so `y` during a
// capture writes nowhere. It is the ABSENCE OF A WRITER rather than a failed write
// — the same rule commitConstant/commitSlot state — so the confirm still comes down
// and nothing else moves at all.
//
// AND IT MUST NOT REACH §9.3'S OTHER HALF. commitSlot returns nil for BOTH a landed
// write and this early return, and the nil-persister one returns BEFORE the mirror
// and the recompute. §8.4's load runs on mirrored keys, off which it decides which
// slug the opposite slot now names; run here it would read keys still holding the
// CONSTANT — whose slot slugs are both EMPTY — resolve §8.5's fallback for one of
// them, and report a commit-time `theme: loaded` (§12.3) for a write that never
// happened.
//
// THAT REFUSAL IS ASSERTED BEHAVIOURALLY, not structurally. While the seam was an
// empty no-op no observation could tell a call that happened from one that did not,
// so the guard was pinned by an AST scan for the early return; now that the seam has
// a body the emitted line and the joined nomination are both observable, and the
// scan is retired rather than kept alongside — it pinned one SHAPE of the guard,
// while this pins the property the guard exists for.
func TestSlotConfirm_NilPersisterIsInert(t *testing.T) {
	t.Run("the answer writes and moves nothing", func(t *testing.T) {
		rows := arrowValidRows(4)
		persisted, target := rows[0].Slug, rows[2].Slug

		m := openCommitPanel(t, newArrowPanelDeps(t, rows, persisted), PageSessions, persisted)
		if m.themePersister != nil {
			t.Fatalf("fixture: the model holds persister %#v, want none", m.themePersister)
		}
		m = arrowToThemeRow(t, m, target)
		before := m.View().Content

		raised := raiseSlotConfirmForTestAt(t, m, slotDarkPress, themeSlotConfirm{slug: target, slot: prefs.SlotDark})
		answered, cmd := pressConfirmKey(t, raised, confirmYes)

		requireConstantKeys(t, answered, persisted)
		requireConfirmResolved(t, answered)
		requireStandingFooter(t, answered)
		if cmd != nil {
			t.Errorf("`y` over a nil persister scheduled %T, want nothing", cmd)
		}
		if got := answered.View().Content; got != before {
			t.Errorf("`y` over a nil persister did not restore the pre-raise frame\nbefore: %q\nafter:  %q", escSeq(before), escSeq(got))
		}
	})

	t.Run("the newly-live-slot load is behind the persister", func(t *testing.T) {
		dir := newConversionThemesDir(t)
		loader, sink := themeOpenTestLoader(t)
		keys := theme.RawKeys{Theme: conversionConstant}
		setting, _ := theme.ResolveSetting(keys.Theme, keys.Light, keys.Dark)
		resolution, err := theme.NewLoader(nil).ResolveNomination(setting, dir)
		if err != nil {
			t.Fatalf("construction-time resolution of %+v: %v", setting, err)
		}
		m := Build(Deps{
			Lister:          fakeLister{},
			Theme:           resolution.Nomination,
			ThemeEnumerator: &realThemeEnumerator{loader: loader, dir: dir},
			ThemeKeys:       keys,
		})
		if m.themePersister != nil {
			t.Fatalf("fixture: the model holds persister %#v, want none", m.themePersister)
		}
		m.termWidth, m.termHeight = arrowTermW, arrowTermH
		m.applySessions(closePanelSessions())
		m = openConversionPanel(t, m)
		nomination := m.nomination

		m, _ = convertToSlot(t, m, "nord", slotDarkPress)

		if got := themeEventRecords(sink, "loaded"); len(got) != 0 {
			t.Errorf("`y` over a nil persister emitted %d `theme: loaded` line(s), want none — no write landed for a load to follow\n%s", len(got), sink.Body())
		}
		if got := themeEventRecords(sink, "fallback applied"); len(got) != 0 {
			t.Errorf("`y` over a nil persister emitted %d `theme: fallback applied` line(s), want none — the un-mirrored keys' slots are both empty, which is the shape a load run here would resolve\n%s", len(got), sink.Body())
		}
		if m.nomination != nomination || !m.nomination.IsConstant() {
			t.Errorf("`y` over a nil persister left the nomination %+v, want the untouched constant", m.nomination)
		}
		requireConstantKeys(t, m, conversionConstant)
	})
}

// TestSlotConfirm_ForcedCloseCancels: it is cancelled silently by a forced close.
//
// §9.8: "A live slot-from-constant confirm is silently cancelled by a forced close.
// Nothing has been written at that point (§9.2), so there is no partial state to
// leave behind — but it is stated because the confirm is otherwise specified as
// resolvable only by a keypress."
//
// So the close takes task 8-10's path exactly (everything the panel retained is
// discarded, the resolved persisted state is rendered), the pending assignment goes
// with it, nothing is written, and the flash is the geometry event's own §14A copy —
// the confirm raises none of its own.
func TestSlotConfirm_ForcedCloseCancels(t *testing.T) {
	m := newGeometryPanelModel(t, geometryWideW, geometryContentH)
	persister := &fakeThemePersister{}
	WithThemePersister(persister)(&m)
	m, _ = pressSlotKey(t, m, slotDarkPress)
	if got := m.themePanel.message; got.Kind != themeMessageConfirm {
		t.Fatalf("fixture: `d` left the message %+v, want the confirm live over a constant", got)
	}
	keys := m.themeKeys

	contentW, contentH := geometryBelowHeightFloor()
	m = resizeForTest(t, m, contentW, contentH)

	requireForcedClose(t, m, specShortClosedFlash)
	if got := m.themePanel.pending; got != (themeSlotConfirm{}) {
		t.Errorf("the forced close retained the pending assignment %+v, want it cancelled with the panel", got)
	}
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("the forced close retained the message %+v, want the slot empty", got)
	}
	if len(persister.slugs) != 0 {
		t.Errorf("the forced close wrote %v; nothing has been written at that point (§9.8)", persister.slugs)
	}
	if m.themeKeys != keys {
		t.Errorf("the forced close left keys %+v, want the untouched %+v", m.themeKeys, keys)
	}
}

// TestSlotConfirm_NotRaisedByEnter: it is not raised by Enter.
//
// §9.2: "**The reverse direction needs no confirm.** `Enter` on a theme while a
// pair is set clears both slots — but `Enter` visibly does what it says: you get
// the theme you are looking at, and it is the theme already previewing behind the
// panel. The asymmetry is the point: the confirm guards the case where the RESOLVED
// theme changes as a side effect of a write the user was told is inert."
//
// Asserted over BOTH settings, because `Enter` is unconditional: over a pair (the
// direction §9.2 names) and over a constant (where the neighbouring `d`/`l` DO ask).
func TestSlotConfirm_NotRaisedByEnter(t *testing.T) {
	t.Run("over a pair", func(t *testing.T) {
		rows := arrowValidRows(4)
		m, persister := newCommitPairPanelModel(t, rows)

		m, _ = pressCommitKey(t, arrowToThemeRow(t, m, rows[2].Slug))

		requireCommitted(t, persister, rows[2].Slug)
		requireConfirmResolved(t, m)
		requireStandingFooter(t, m)
	})

	t.Run("over a constant", func(t *testing.T) {
		m, persister := newSlotConfirmModel(t)

		m, _ = pressCommitKey(t, m)

		requireCommitted(t, persister, slotConfirmTarget())
		requireConstantKeys(t, m, slotConfirmTarget())
		requireConfirmResolved(t, m)
		requireStandingFooter(t, m)
	})
}

// TestSlotConfirm_HandEditedFileNamesTheConstant: it names the constant on a
// theme-wins file.
//
// §8.2: a hand-edited file may carry all three keys, and `theme` wins — so the
// slots are not read at all and the panel shows one bare `●`. "The one visible
// consequence: on such a file, `d`/`l` clears the constant and the OTHER stale
// hand-edited slot becomes live in the same keypress. The §9.2 confirm names the
// constant being cleared, which is the change the user initiated; the stale slot
// surfacing is then plainly visible in the panel's badges the moment the confirm
// resolves."
//
// So the copy is NOT extended to mention the stale slot, and the badge that appears
// on `y` is what tells the user about it.
func TestSlotConfirm_HandEditedFileNamesTheConstant(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	keys := theme.RawKeys{Theme: "aurora", Light: "ghost", Dark: "aurora"}
	m, _, persister := newRecomputePanelModel(t, dir, keys)
	requireRowLabels(t, m, "aurora", "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	requireBadge(t, m, "aurora", theme.BadgeConstant)

	m = arrowToThemeRow(t, m, "nord")
	m, _ = pressSlotKey(t, m, slotDarkPress)

	requireConfirmLive(t, m, themeSlotConfirm{slug: "nord", slot: prefs.SlotDark})
	rendered := slotConfirmPanelText(m)
	if want := "clear constant aurora?  y / n"; !strings.Contains(rendered, want) {
		t.Errorf("the confirm does not read %q — it names the CONSTANT being cleared (§8.2)\n%s", want, rendered)
	}
	if strings.Contains(rendered, "ghost") {
		t.Errorf("the confirm mentions the stale light slug; the copy is not extended for it (§8.2)\n%s", rendered)
	}

	m, _ = pressConfirmKey(t, m, confirmYes)

	requireSlotCommits(t, persister, slotCommit{slug: "nord", slot: prefs.SlotDark})
	requirePairKeys(t, m, "ghost", "nord")
	// The stale light slot is live the moment the confirm resolves: §9.4 gives its
	// unresolvable slug a row of its own, and §9.5 puts the `● light` on it.
	requireRowLabels(t, m, "aurora", "ghost", "nord", theme.DefaultDarkSlug, theme.DefaultLightSlug)
	requireBadge(t, m, "ghost", theme.BadgeLight)
	requireBadge(t, m, "nord", theme.BadgeDark)
	requireBadge(t, m, "aurora", theme.BadgeNone)
	if got := slotConfirmPanelText(m); !strings.Contains(got, "● light") {
		t.Errorf("the panel does not render the stale slot's `● light` badge:\n%s", got)
	}
}

// TestSlotConfirm_ResizesTheListForTheSwappedLayout: it re-sizes the list when the
// message slot changes.
//
// The message slot and §9.2's nested confirm scope BOTH move the panel's vertical
// budget — the slot costs a row, the two-row confirm footer hands two back — and
// renderThemePanel sizes its per-frame copy from the message it is handed. A model
// whose list keeps the PRE-MESSAGE page derives `Ctrl+↑`/`Ctrl+↓` from a page the
// screen is not showing, which no rendered frame reveals (the same class
// resizeThemePanel re-sizes for, and the same one the main screen's
// resyncPageLayouts answers on every band raise/clear).
//
// It is asserted at the WRITERS rather than at this call site, so §9.13's
// failed-commit line — which persists with arrows LIVE — inherits it.
func TestSlotConfirm_ResizesTheListForTheSwappedLayout(t *testing.T) {
	m, _ := newSlotConfirmModelAt(t, arrowPagingTermH)
	requireArrowPanelPageSize(t, m, arrowPagingPerPage)
	requireThemePanelListSized(t, m, "on open")
	before := m.themePanel.list.Height()

	raised := raiseSlotConfirmForTest(t, m, slotDarkPress, prefs.SlotDark)

	requireThemePanelListSized(t, raised, "with the confirm live")
	if got := raised.themePanel.list.Height(); got == before {
		t.Fatalf("fixture: the list body is %d rows either side of the raise, so the re-size asserts nothing", got)
	}

	cancelled, _ := pressConfirmKey(t, raised, confirmNo)

	requireThemePanelListSized(t, cancelled, "once the confirm resolved")
	if got := cancelled.themePanel.list.Height(); got != before {
		t.Errorf("the resolved confirm left the list body at %d rows, want the pre-raise %d", got, before)
	}
}

// requireThemePanelListSized fails unless the MODEL's list is sized to the same
// body the renderer sizes its per-frame copy to — the page the user scrolls and the
// page the model pages through being one page.
func requireThemePanelListSized(t *testing.T, m Model, when string) {
	t.Helper()

	width, rows := themePanelListSize(m.themePanel, m.contentHeight())
	if got := m.themePanel.list.Height(); got != rows {
		t.Errorf("%s the model's list is %d rows tall and the rendered frame sizes its copy to %d", when, got, rows)
	}
	if got := m.themePanel.list.Width(); got != width {
		t.Errorf("%s the model's list is %d cells wide and the rendered frame sizes its copy to %d", when, got, width)
	}
}

// themePanelSeamCallers returns the name of every production function containing a
// call to the named function, sorted and deduplicated — the structural half of "the
// seam has one route in", which a no-op body cannot state behaviourally.
func themePanelSeamCallers(t *testing.T, name string) []string {
	t.Helper()

	var callers []string
	for _, file := range parsePackageFilesByName(t) {
		for _, decl := range file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == name {
					callers = append(callers, fn.Name.Name)
				}
				return true
			})
		}
	}
	slices.Sort(callers)
	return slices.Compact(callers)
}
