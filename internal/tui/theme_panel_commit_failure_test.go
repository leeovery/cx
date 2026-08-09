package tui

import (
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// §9.13's FAILED COMMIT WRITE: the one genuinely dangerous state the picker idiom
// buys protection from — "applied but not persisted" — turned from SILENT into
// REPORTED.
//
// Four rules are individually easy and collectively easy to get wrong, and each is
// a named test below:
//
//   - THE THEME STAYS APPLIED and the `●` DOES NOT MOVE. The marker means "what is
//     persisted" and would be lying if it moved, which is what forces the failure
//     path to skip the key mutation AND the recompute.
//   - THE MESSAGE PERSISTS UNTIL THE NEXT KEYPRESS rather than timing out. It
//     reports a state the user must act on, and in a surface whose only other
//     feedback is the `●` deliberately NOT moving, a message that vanishes on its
//     own can be missed — so it takes none of the flash lifecycle.
//   - "OUTSTANDING" IS A STATE, NOT A MESSAGE. Arrowing away dismisses the message
//     and leaves the state, which is what stops the very next `Esc` reinstating the
//     silent revert §9.13 exists to close.
//   - A SUCCESSFUL COMMIT DISCHARGES IT, so a `d` that fails followed by an `l`
//     that succeeds reports nothing: the user is not told a theme was not saved
//     when it was.
//
// No t.Parallel() — the package-level mock convention makes parallelism unsafe
// across this package's tests.

// newFailedCommitModel is this file's standard fixture: a REAL loader over a REAL
// themes directory under an ADAPTIVE PAIR, the panel open, the cursor ARROWED OFF
// the row the open anchored on, and the persister primed to fail.
//
// The loader is real wherever rows and badges are asserted, for the reason the
// recompute suite's fixture is: a stub seam answers from declared values, so every
// row and every `●` would be a statement about the fixture rather than about the
// derivation a commit drives.
//
// The cursor is arrowed OFF the dark slot's own row because that is the only shape
// in which a mutation is observable at all: committing the slug the slot already
// holds leaves the keys and the badges byte-identical whether the write landed or
// not.
func newFailedCommitModel(t *testing.T) (Model, *fakeThemePersister) {
	t.Helper()

	m, persister := newCommitFailureFixture(t)
	persister.err = errThemeCommitFailed
	return m, persister
}

// newCommitFailureFixture is newFailedCommitModel with the persister still
// SUCCEEDING — the positive control every "nothing moved" assertion needs, and the
// starting point for the cases that fail once and then succeed.
func newCommitFailureFixture(t *testing.T) (Model, *fakeThemePersister) {
	t.Helper()

	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	writeThemeFileForTest(t, dir, "sunset.theme", "#202020")
	m, persister := newSlotPairPanelModel(t, dir, theme.DefaultLightSlug, "aurora")
	requireBadge(t, m, "aurora", theme.BadgeDark)
	requireBadge(t, m, theme.DefaultLightSlug, theme.BadgeLight)

	m = arrowToThemeRow(t, m, commitFailureTarget)
	return m, persister
}

// commitFailureTarget is the row the fixture's cursor is parked on — neither slot's
// slug, so a mirror that ran would move a key and a recompute that ran would move a
// `●`.
const commitFailureTarget = "sunset"

// requireCommitFailedMessage fails unless the panel is holding §9.13's line: the
// slot's VALUE, and the row the user reads it on.
//
// Both halves are asserted because they fail independently — a value installed
// without the slot rendering it reports nothing, and §14A's copy is pinned VERBATIM
// so a paraphrase at a call site is what the byte comparison catches.
func requireCommitFailedMessage(t *testing.T, m Model) {
	t.Helper()

	if got, want := m.themePanel.message, (themePanelMessage{Kind: themeMessageCommitFailed}); got != want {
		t.Errorf("the message slot holds %+v, want %+v — §9.13's line reports in §9.1's slot", got, want)
	}
	if got := themePanelMessageRow(m); got != messageTestFailedCopy {
		t.Errorf("the message slot reads %q, want §14A's %q", got, messageTestFailedCopy)
	}
}

// themePanelMessageRow is §9.1's message slot as a user reads it: the row directly
// above the vertical keymap footer, stripped of its SGR and of the border-and-gutter
// prefix every panel row opens with.
func themePanelMessageRow(m Model) string {
	lines := themePanelLines(renderRecomputePanel(m))
	row := lines[len(lines)-themePanelFooterHeight(themePanelFooterScope(m.themePanel.message))-1]
	return strings.TrimRight(strings.TrimPrefix(row, themePanelContentPrefix()), " ")
}

// badgeRows is every rendered panel row carrying a `●`, each prefixed with its row
// INDEX — so a marker that moved to another row and a row whose text changed under
// it both show up, which the badge map alone cannot say.
func badgeRows(m Model) []string {
	var rows []string
	for i, line := range themePanelLines(renderRecomputePanel(m)) {
		if strings.Contains(line, "●") {
			rows = append(rows, fmt.Sprintf("%d:%s", i, strings.TrimRight(line, " ")))
		}
	}
	return rows
}

// TestCommitFailure_MessageCopy: it renders the pinned message.
//
// §9.13: a failed write "reports in the panel's message slot (§9.1) — `⚠` plus a
// terse statement that the theme could not be saved, glyph-backed per Portal's
// convention". §14A pins that copy verbatim, and the `⚠` is part of the STRING so
// the report survives wherever the accent.attention hue does not.
func TestCommitFailure_MessageCopy(t *testing.T) {
	m, persister := newFailedCommitModel(t)

	m, cmd := pressSlotKey(t, m, slotDarkPress)

	requireSlotCommits(t, persister, slotCommit{slug: commitFailureTarget, slot: prefs.SlotDark})
	requireCommitFailedMessage(t, m)
	if !m.themePanel.open {
		t.Error("a failed commit closed the panel; `Esc` is the only way out (§9.2)")
	}
	if cmd != nil {
		t.Errorf("a failed commit scheduled %T, want nothing", cmd)
	}

	// The control: the same fixture with the write LANDING raises nothing at all, so
	// the line above is the failure rather than a slot that fills on every commit.
	landed, landedPersister := newCommitFailureFixture(t)
	landed, _ = pressSlotKey(t, landed, slotDarkPress)
	requireSlotCommits(t, landedPersister, slotCommit{slug: commitFailureTarget, slot: prefs.SlotDark})
	if got := landed.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("a successful commit raised the message %+v, want an empty slot", got)
	}
}

// TestCommitFailure_BadgeDoesNotMove: it does not move the marker.
//
// §9.13: a failed commit "does not move the `●` — the marker means 'what is
// persisted' and would be lying if it moved". That forces the failure path to skip
// BOTH the key mutation and §9.2's recompute, so the assertion is made on the
// RENDERED rows rather than on the badge map alone: the map is what was derived,
// the glyphs are what the user is told.
func TestCommitFailure_BadgeDoesNotMove(t *testing.T) {
	m, persister := newFailedCommitModel(t)
	keys := m.themeState.keys
	badges := maps.Clone(m.themePanel.badges)
	labels := themePanelRowLabels(m)
	rendered := badgeRows(m)

	m, _ = pressSlotKey(t, m, slotDarkPress)

	requireSlotCommits(t, persister, slotCommit{slug: commitFailureTarget, slot: prefs.SlotDark})
	if got := badgeRows(m); !slices.Equal(got, rendered) {
		t.Errorf("a failed commit rendered the markers on %v, want the untouched %v — the `●` means what is PERSISTED (§9.13)", got, rendered)
	}
	requireBadge(t, m, "aurora", theme.BadgeDark)
	requireBadge(t, m, theme.DefaultLightSlug, theme.BadgeLight)
	requireBadge(t, m, commitFailureTarget, theme.BadgeNone)
	if m.themeState.keys != keys {
		t.Errorf("a failed commit left keys %+v, want the untouched %+v — the badges derive from them", m.themeState.keys, keys)
	}
	if got := m.themePanel.badges; !maps.Equal(got, badges) {
		t.Errorf("a failed commit left badges %v, want the untouched %v", got, badges)
	}
	if got := themePanelRowLabels(m); !slices.Equal(got, labels) {
		t.Errorf("a failed commit left rows %v, want the untouched %v — only a SUCCESSFUL commit recomputes (§9.2)", got, labels)
	}

	// The control: the same fixture with the write LANDING does move the marker, so
	// the untouched rows above are the failure path rather than a badge that never
	// moves.
	landed, _ := newCommitFailureFixture(t)
	landed, _ = pressSlotKey(t, landed, slotDarkPress)
	requireBadge(t, landed, commitFailureTarget, theme.BadgeDark)
	if got := badgeRows(landed); slices.Equal(got, rendered) {
		t.Errorf("a successful commit rendered the markers on %v too; the comparison above proves nothing", got)
	}
}

// TestCommitFailure_ThemeStaysApplied: it keeps the theme applied.
//
// §9.13 "keeps the theme applied in memory", which is precisely what recreates
// "applied but not persisted" — as a REPORTED state rather than a silent one. So
// the previewed palette is still in force and the frame's colours are still that
// palette's.
//
// THE FIXTURE IS THE STUB-BACKED PAIR, whose synthetic palettes carry a DISTINCT
// value per token. The real-loader fixture writes one colour across every token of
// a drop-in, which would make the message's own accent.attention run
// indistinguishable from the canvas and the colour comparison below vacuous.
func TestCommitFailure_ThemeStaysApplied(t *testing.T) {
	rows := arrowValidRows(4)
	m, persister := newCommitPairPanelModel(t, rows)
	m = arrowToThemeRow(t, m, rows[2].Slug)
	previewed := m.themeState.active
	if previewed != rows[2].Theme {
		t.Fatalf("fixture: the frame renders %s, want the arrowed-to row's palette", themeLabel(previewed))
	}
	before := frameColours(m.View().Content)
	persister.err = errThemeCommitFailed

	m, _ = pressSlotKey(t, m, slotDarkPress)

	requireSlotCommits(t, persister, slotCommit{slug: rows[2].Slug, slot: prefs.SlotDark})
	requireCommitFailedMessage(t, m)
	if m.themeState.active != previewed {
		t.Errorf("a failed commit rendered %s, want the previewed %s — §9.13 KEEPS the theme applied in memory", themeLabel(m.themeState.active), themeLabel(previewed))
	}

	frame := m.View().Content
	if seq := canvasSeq(t, previewed); !strings.Contains(frame, seq) {
		t.Errorf("the composed frame no longer paints the previewed canvas %q", seq)
	}
	after := frameColours(frame)
	if gone := colourDiff(before, after); len(gone) != 0 {
		t.Errorf("a failed commit dropped the colours %v from the frame", gone)
	}
	// The only colour it may ADD is the message slot's own accent.attention run:
	// §9.1 gives that line the token and NO band, and nothing else on the frame moved.
	// Its PRESENCE is asserted first, so the loop below is a statement about a frame
	// that genuinely gained the report rather than an empty walk.
	attention := tokenFgSeq(t, previewed.AccentAttention)
	carries := func(seq string) bool { return strings.Contains(seq, attention) }
	if !slices.ContainsFunc(after, carries) {
		t.Errorf("the frame paints no accent.attention run; §9.13's line renders in that token (§9.1)")
	}
	for _, seq := range colourDiff(after, before) {
		if !carries(seq) {
			t.Errorf("a failed commit added the colour %q to the frame; only §9.13's own line may appear", escSeq(seq))
		}
	}
}

// colourDiff returns the SGR sequences in a that b does not carry.
func colourDiff(a, b []string) []string {
	var only []string
	for _, seq := range a {
		if !slices.Contains(b, seq) {
			only = append(only, seq)
		}
	}
	return only
}

// TestCommitFailure_MessageClearsOnNextKeyAndFallsThrough: it persists until the
// next keypress.
//
// §9.13: the line "persists until the next keypress rather than timing out like a
// transient flash". The clear is the shape the main screen's actionable-key clear
// already uses — ONE KEY, ONE INTENT — so the arrow that takes the message down also
// moves the cursor and re-themes the frame, rather than being swallowed as a
// dismissal.
//
// The keypress that RAISES the message is unaffected, because the clear runs ahead of
// the dispatch and the raise happens inside it — asserted here by the message being
// live after the failing keypress at all.
func TestCommitFailure_MessageClearsOnNextKeyAndFallsThrough(t *testing.T) {
	m, _ := newFailedCommitModel(t)

	m, _ = pressSlotKey(t, m, slotDarkPress)
	requireCommitFailedMessage(t, m)
	index := m.themePanel.list.Index()
	previewed := m.themeState.active
	body := m.themePanel.list.Height()

	m = pressPanelKey(t, m, arrowDown)

	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("the next keypress left the message %+v, want the slot empty (§9.13)", got)
	}
	if strings.Contains(themePanelMessageRow(m), messageTestFailedCopy) {
		t.Errorf("the panel still renders §9.13's line after the next keypress:\n%s", renderRecomputePanel(m))
	}
	if got := m.themePanel.list.Index(); got == index {
		t.Errorf("the arrow left the cursor on row %d; the keystroke continues to its normal handler — one key, one intent", got)
	}
	if m.themeState.active == previewed {
		t.Errorf("the arrow left %s painting the frame; it moved the cursor, so it previews the new row (§9.2)", themeLabel(previewed))
	}
	if got := m.themePanel.list.Height(); got != body+1 {
		t.Errorf("the list body is %d rows once the message cleared, want the row back (%d) — the slot is not reserved when empty (§9.1)", got, body+1)
	}
}

// TestCommitFailure_MessageSurvivesWindowSize: it survives a non-key event.
//
// The lifetime is "until the next KEYPRESS", so an event that is not one leaves the
// report standing — the same rule the main screen's flash follows, where a
// WindowSizeMsg, a focus change or a session refresh never counts as the user
// acknowledging anything.
//
// The resize is ABOVE §9.8's floor, so the panel degrades in place rather than
// force-closing: the message has to survive the panel being re-sized, which is the
// path that re-derives the slot's own vertical budget.
func TestCommitFailure_MessageSurvivesWindowSize(t *testing.T) {
	m, _ := newFailedCommitModel(t)

	m, _ = pressSlotKey(t, m, slotDarkPress)
	requireCommitFailedMessage(t, m)

	updated, cmd := m.Update(tea.WindowSizeMsg{Width: arrowTermW - 4, Height: arrowTermH - 2})
	m = updated.(Model)

	if !m.themePanel.open {
		t.Fatal("the resize force-closed the panel; the fixture must stay above §9.8's floor for this to assert anything")
	}
	requireCommitFailedMessage(t, m)
	if !m.themeState.commitFailed {
		t.Error("the resize discharged the outstanding failure; only a successful commit does (§9.13)")
	}
	if cmd != nil {
		t.Errorf("the resize scheduled %T, want nothing", cmd)
	}
}

// TestCommitFailure_MessageHasNoTickLifecycle: it does not auto-clear on a timer.
//
// §9.13 is explicit that the line does NOT time out like a transient flash: it
// reports a state the user must act on, and in a surface whose only other feedback is
// the `●` deliberately not moving, a message that vanishes on its own can be missed.
//
// So the failure path takes NONE of the flash machinery — no flashTickCmd, no
// generation bump, no main-screen flash text — and the clock is advanced the only way
// it can be observed in a Bubble Tea model: by delivering the tick the 3s timer would
// have fired.
func TestCommitFailure_MessageHasNoTickLifecycle(t *testing.T) {
	m, _ := newFailedCommitModel(t)
	gen := m.flashGen

	m, cmd := pressSlotKey(t, m, slotDarkPress)

	requireCommitFailedMessage(t, m)
	if cmd != nil {
		t.Errorf("a failed commit scheduled %T; §9.13's line takes no tick lifecycle", cmd)
	}
	if m.flashGen != gen {
		t.Errorf("a failed commit bumped the flash generation to %d, want the untouched %d — it raises no flash", m.flashGen, gen)
	}
	if m.flashText != "" {
		t.Errorf("a failed commit set the main-screen flash %q; §9.13 reports in the PANEL's slot", m.flashText)
	}

	// The clock, advanced past flashAutoClearDuration: the tick a flash would have
	// scheduled fires, and the panel's own message is untouched by it.
	updated, tickCmd := m.Update(flashTickMsg{Gen: m.flashGen})
	m = updated.(Model)

	requireCommitFailedMessage(t, m)
	if tickCmd != nil {
		t.Errorf("the tick scheduled %T, want nothing", tickCmd)
	}
	if !m.themeState.commitFailed {
		t.Error("the tick discharged the outstanding failure; only a successful commit does (§9.13)")
	}
}

// TestCommitFailure_StateOutlivesTheMessage: it keeps the failure outstanding after
// the message is dismissed.
//
// §9.13: "'outstanding' is a state, not a message... arrowing away does not [clear
// it]: that dismisses the MESSAGE while leaving the state, which is what stops the
// very next `Esc` reinstating the silent revert this section exists to close."
//
// The three keys below are the ones that take the message down without writing
// anything: an arrow, a page, and a confirm raised and cancelled. None of them is a
// commit, so none of them may discharge the state.
func TestCommitFailure_StateOutlivesTheMessage(t *testing.T) {
	t.Run("an arrow", func(t *testing.T) {
		m, _ := newFailedCommitModel(t)
		m, _ = pressSlotKey(t, m, slotDarkPress)

		m = pressPanelKey(t, m, arrowDown)

		requireOutstandingWithNoMessage(t, m)
	})

	t.Run("a page", func(t *testing.T) {
		rows := arrowValidRows(30)
		m, persister := newCommitPairPanelModel(t, rows)
		if got := m.themePanel.list.Paginator.TotalPages; got < 2 {
			t.Fatalf("fixture: the panel's list is %d page(s), want it paginating so `Ctrl+↓` has somewhere to go", got)
		}
		persister.err = errThemeCommitFailed
		m, _ = pressSlotKey(t, m, slotDarkPress)
		requireCommitFailedMessage(t, m)
		page := m.themePanel.list.Paginator.Page

		m = pressPanelKey(t, m, arrowPageDown)

		if got := m.themePanel.list.Paginator.Page; got == page {
			t.Errorf("the page key left the list on page %d; the keystroke continues to its normal handler", got)
		}
		requireOutstandingWithNoMessage(t, m)
	})

	t.Run("a confirm raised and cancelled", func(t *testing.T) {
		dir := t.TempDir()
		writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
		m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Theme: "aurora"})
		m = arrowToThemeRow(t, m, "nord")
		persister.err = errThemeCommitFailed

		m, cmd := pressCommitKey(t, m)
		requireCommitFailedMessage(t, m)
		if cmd != nil {
			t.Errorf("a failed `Enter` scheduled %T; §9.13's line takes no tick lifecycle on any commit key", cmd)
		}

		// The raise takes the message down and puts the QUESTION in its place — §9.1's
		// two contenders, one slot — and the cancel writes nothing at all.
		m, _ = pressSlotKey(t, m, slotDarkPress)
		requireConfirmLive(t, m, themeSlotConfirm{slug: "nord", slot: prefs.SlotDark})
		if !m.themeState.commitFailed {
			t.Error("raising the confirm discharged the outstanding failure")
		}

		m, _ = pressConfirmKey(t, m, confirmNo)

		requireConfirmResolved(t, m)
		requireOutstandingWithNoMessage(t, m)
		requireConstantKeys(t, m, "aurora")
	})
}

// requireOutstandingWithNoMessage fails unless §9.13's split holds: the message gone,
// the state still outstanding.
func requireOutstandingWithNoMessage(t *testing.T, m Model) {
	t.Helper()

	if got := m.themePanel.message; got.Kind == themeMessageCommitFailed {
		t.Errorf("the message slot still holds %+v; it persists only until the next keypress (§9.13)", got)
	}
	if !m.themeState.commitFailed {
		t.Error("the failure is no longer outstanding; dismissing the MESSAGE must leave the STATE, or the very next `Esc` reinstates the silent revert (§9.13)")
	}
}

// TestCommitFailure_SuccessDischargesTheState: it clears the state on a later
// successful commit.
//
// §9.13: "because a successful retry clears it, a `d` that fails followed by an `l`
// that succeeds raises no flash — the user is not told a theme was not saved when it
// was."
//
// The two keys are DIFFERENT on purpose: the state is about the SETTING rather than
// about the key that failed, so a success on either commit key discharges it. The
// landed write's own effects are asserted alongside, because a discharge that came
// from a write which did not happen would be the same bug in the other direction.
func TestCommitFailure_SuccessDischargesTheState(t *testing.T) {
	m, persister := newFailedCommitModel(t)

	m, _ = pressSlotKey(t, m, slotDarkPress)
	requireCommitFailedMessage(t, m)
	if !m.themeState.commitFailed {
		t.Fatal("the failed commit left nothing outstanding, so the discharge below proves nothing")
	}
	gen := m.flashGen
	persister.err = nil

	m, cmd := pressSlotKey(t, m, slotLightPress)

	requireSlotCommits(t, persister,
		slotCommit{slug: commitFailureTarget, slot: prefs.SlotDark},
		slotCommit{slug: commitFailureTarget, slot: prefs.SlotLight},
	)
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("the successful commit left the message %+v, want the slot empty (§9.13)", got)
	}
	if m.themeState.commitFailed {
		t.Error("the successful commit left the failure outstanding; a `d` that fails followed by an `l` that succeeds reports nothing (§9.13)")
	}
	if m.flashText != "" || m.flashGen != gen {
		t.Errorf("the successful commit raised the flash %q (gen %d, want %d); there is nothing left to report", m.flashText, m.flashGen, gen)
	}
	// The write genuinely landed: the keys mirrored it and the recompute moved the `●`.
	requirePairKeys(t, m, commitFailureTarget, "aurora")
	requireBadge(t, m, commitFailureTarget, theme.BadgeLight)
	requireBadge(t, m, theme.DefaultLightSlug, theme.BadgeNone)
	if cmd != nil {
		t.Errorf("the successful commit scheduled %T, want nothing", cmd)
	}
}

// TestCommitFailure_RetryIsJustPressingAgain: it retries on the same key.
//
// §9.13: "a commit is always re-attemptable. The commit keys are unconditional
// writes (§9.2), so pressing `d`/`l`/`Enter` again simply retries — no special retry
// affordance, and no state to clear first."
//
// So the second press is another write rather than a no-op guarded by the
// outstanding state, and the third — with the persister healthy — lands and clears
// everything. The RECORDED CALLS are the assertion: a retry that never reached the
// seam would leave the message looking identical.
func TestCommitFailure_RetryIsJustPressingAgain(t *testing.T) {
	m, persister := newFailedCommitModel(t)
	failed := slotCommit{slug: commitFailureTarget, slot: prefs.SlotDark}

	m, _ = pressSlotKey(t, m, slotDarkPress)
	requireCommitFailedMessage(t, m)

	m, _ = pressSlotKey(t, m, slotDarkPress)

	requireSlotCommits(t, persister, failed, failed)
	requireCommitFailedMessage(t, m)
	if !m.themeState.commitFailed {
		t.Error("the retry discharged the outstanding failure; it failed again")
	}

	persister.err = nil
	m, _ = pressSlotKey(t, m, slotDarkPress)

	requireSlotCommits(t, persister, failed, failed, failed)
	if got := m.themePanel.message; got.Kind != themeMessageNone {
		t.Errorf("the successful retry left the message %+v, want the slot empty", got)
	}
	if m.themeState.commitFailed {
		t.Error("the successful retry left the failure outstanding (§9.13)")
	}
	requirePairKeys(t, m, theme.DefaultLightSlug, commitFailureTarget)
	requireBadge(t, m, commitFailureTarget, theme.BadgeDark)
}

// TestCommitFailure_ConfirmDrivenFailure: it lands identically on a confirmed
// commit.
//
// The confirm has ALREADY RESOLVED by the time the write runs (§9.2 — it gates the
// write), so the failure lands in a slot the question vacated and the footer is
// already back on the standing scope. The constant is not cleared in memory either,
// so §9.5's bare `●` is still on it: §9.13's "a failed commit does not move the `●`"
// falls out of the keys being untouched rather than out of a rule the confirm states
// for itself.
func TestCommitFailure_ConfirmDrivenFailure(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Theme: "aurora"})
	requireBadge(t, m, "aurora", theme.BadgeConstant)
	labels := themePanelRowLabels(m)
	badges := maps.Clone(m.themePanel.badges)

	m = arrowToThemeRow(t, m, "nord")
	previewed := m.themeState.active
	persister.err = errThemeCommitFailed
	m, _ = pressSlotKey(t, m, slotDarkPress)
	requireConfirmLive(t, m, themeSlotConfirm{slug: "nord", slot: prefs.SlotDark})

	m, cmd := pressConfirmKey(t, m, confirmYes)

	requireSlotCommits(t, persister, slotCommit{slug: "nord", slot: prefs.SlotDark})
	requireCommitFailedMessage(t, m)
	if !m.themeState.commitFailed {
		t.Error("a failed confirmed commit left nothing outstanding (§9.13)")
	}
	requireConfirmGone(t, m)
	requireStandingFooter(t, m)
	// The constant is still in the in-memory keys, so the panel still marks it — and
	// it is the ONLY `●` on the panel, which is what a cleared constant would not be.
	requireConstantKeys(t, m, "aurora")
	requireBadge(t, m, "aurora", theme.BadgeConstant)
	requireBadgeText(t, m, 1, 0, 0)
	if got := m.themePanel.badges; !maps.Equal(got, badges) {
		t.Errorf("a failed confirmed commit left badges %v, want the untouched %v", got, badges)
	}
	if got := themePanelRowLabels(m); !slices.Equal(got, labels) {
		t.Errorf("a failed confirmed commit left rows %v, want the untouched %v — only a SUCCESSFUL commit recomputes (§9.2)", got, labels)
	}
	if m.themeState.active != previewed {
		t.Errorf("a failed confirmed commit rendered %s, want the previewed %s left alone", themeLabel(m.themeState.active), themeLabel(previewed))
	}
	if cmd != nil {
		t.Errorf("a failed confirmed commit scheduled %T, want nothing", cmd)
	}
}

// TestCommitFailure_NeverLiveWithTheConfirm: it never co-renders with the confirm.
//
// §9.1's slot is a single-slot arbiter whose two contenders "can never be live at
// once because a confirm resolves before any write happens". This drives that
// through the one path where both are in play — a confirmed commit that fails, then
// a second `d` over the constant the failure left standing — and asserts it at the
// RENDER surface in both directions, which is where a slot that drew both would show
// up.
func TestCommitFailure_NeverLiveWithTheConfirm(t *testing.T) {
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")
	m, _, persister := newRecomputePanelModel(t, dir, theme.RawKeys{Theme: "aurora"})
	m = arrowToThemeRow(t, m, "nord")
	persister.err = errThemeCommitFailed

	m, _ = pressSlotKey(t, m, slotDarkPress)
	requireOnlySlotContender(t, m, "clear constant aurora?  y / n")

	m, _ = pressConfirmKey(t, m, confirmYes)
	requireOnlySlotContender(t, m, messageTestFailedCopy)

	// The other direction: the constant survived the failed write, so `d` asks again —
	// and the question takes the slot the report was occupying.
	m, _ = pressSlotKey(t, m, slotDarkPress)

	requireOnlySlotContender(t, m, "clear constant aurora?  y / n")
	requireConfirmLive(t, m, themeSlotConfirm{slug: "nord", slot: prefs.SlotDark})
	if !m.themeState.commitFailed {
		t.Error("raising the confirm discharged the outstanding failure; only a successful commit does (§9.13)")
	}
}

// requireOnlySlotContender fails unless the rendered panel carries want in §9.1's
// message slot and no trace of the other contender anywhere.
func requireOnlySlotContender(t *testing.T, m Model, want string) {
	t.Helper()

	if got := themePanelMessageRow(m); got != want {
		t.Errorf("the message slot reads %q, want %q", got, want)
	}
	other := messageTestFailedCopy
	if want == other {
		other = "clear constant"
	}
	if rendered := slotConfirmPanelText(m); strings.Contains(rendered, other) {
		t.Errorf("the panel renders %q alongside %q; §9.1's two contenders are never live at once\n%s", other, want, rendered)
	}
}

// TestCommitFailure_PanelEmitsNoThemeRecord: it logs nothing from the panel.
//
// §12.3's `theme: commit failed` is the PERSISTER's line and §8.9 closes the `theme`
// component's emitters at three — loader, translation, persister. A panel that logged
// its own report would double the event and make internal/tui a fourth emitter, so
// the failure path writes to the screen and nowhere else.
//
// The sink is the PROCESS handler and the loader emits through the component, so the
// open's own `theme: enumerated` is the non-vacuity control: the sink genuinely
// receives this component's records, and the failing keypress adds none.
func TestCommitFailure_PanelEmitsNoThemeRecord(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "aurora.theme", "#101010")

	m, persister := newLoadPanelModel(t, dir, theme.RawKeys{Light: theme.DefaultLightSlug, Dark: "aurora"}, theme.NewLoader(theme.NewEventLogger(log.For("theme"))))
	m = pressThemeKey(t, m)
	if !m.themePanel.open {
		t.Fatal("fixture: the panel did not open")
	}
	if len(sink.Records()) == 0 {
		t.Fatal("the open emitted nothing at all, so the silence below is a sink that was never wired")
	}
	opened := len(sink.Records())
	m = arrowToThemeRow(t, m, "nord")
	persister.err = errThemeCommitFailed

	m, _ = pressSlotKey(t, m, slotDarkPress)

	requireSlotCommits(t, persister, slotCommit{slug: "nord", slot: prefs.SlotDark})
	requireCommitFailedMessage(t, m)
	if got := sink.Records(); len(got) != opened {
		t.Errorf("the failure path emitted %d record(s):\n%s\nthe persister is the SINGLE site for `theme: commit failed` (§8.9)", len(got)-opened, sink.Body())
	}
}

// TestCommitFailure_NilPersisterRaisesNothing: it treats a nil persister as inert.
//
// A fixture or `capturetool` model wires none, so a commit during a capture writes
// nowhere. That is the ABSENCE OF A WRITER rather than a failed
// write: there is no report to make and no outstanding state to hold, and the
// discrimination is kept explicit so a capture model can never enter the reported
// state — which would put §14A's copy on a frame no user could have caused.
func TestCommitFailure_NilPersisterRaisesNothing(t *testing.T) {
	rows := arrowValidRows(4)
	m := openCommitPanel(t, commitPairPanelDeps(t, rows), PageSessions, rows[1].Slug)
	if m.themeState.persister != nil {
		t.Fatalf("fixture: the model holds persister %#v, want none", m.themeState.persister)
	}
	m = arrowToThemeRow(t, m, rows[2].Slug)
	before := m.View().Content

	for _, tc := range []struct {
		name string
		run  func(*testing.T, Model) Model
	}{
		{name: "Enter", run: func(t *testing.T, m Model) Model { m, _ = pressCommitKey(t, m); return m }},
		{name: "d", run: func(t *testing.T, m Model) Model { m, _ = pressSlotKey(t, m, slotDarkPress); return m }},
		{name: "l", run: func(t *testing.T, m Model) Model { m, _ = pressSlotKey(t, m, slotLightPress); return m }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pressed := tc.run(t, m)

			if got := pressed.themePanel.message; got.Kind != themeMessageNone {
				t.Errorf("a nil persister raised the message %+v; it is the absence of a WRITER, not a failed write", got)
			}
			if pressed.themeState.commitFailed {
				t.Error("a nil persister left a commit failure outstanding; nothing was written and nothing failed")
			}
			if got := pressed.View().Content; got != before {
				t.Errorf("%q over a nil persister changed the frame\nbefore: %q\nafter:  %q", tc.name, escSeq(before), escSeq(got))
			}
		})
	}
}
