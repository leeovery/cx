package tui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/prefs"
	"github.com/leeovery/portal/internal/theme"
)

// The panel's COMMIT KEYS — the only thing the panel writes. Four decisions that
// read as small all bite here.
//
// `Enter` DOES NOT CLOSE. A user who had just set both slots would press it to
// leave and thereby commit a constant, wiping the pair they just built — so `Esc`
// stays the only way out, there is no dual-purpose key, and the pair flow needs no
// special case. The accepted cost is that the common case ("pick one and go") is
// two keys rather than one.
//
// IT COMMITS THE CURSOR'S SLUG, NEVER THE PERSISTED ONE. The cursor is what is
// previewed and what the user is looking at, and that split is deliberate: `●` is
// what is SET, the cursor is what is PREVIEWED. Under a fallback the two differ by
// design.
//
// IT DOES NOT RE-THEME. A commit is a WRITE, NOT A NAVIGATION, so the frame is
// unchanged across the keypress and committing to a non-active slot changes
// nothing on screen at all. applyInForceTheme states the shared degrade policy
// for every panel call site of Resolve and is explicit that a commit takes that
// policy WITHOUT taking the apply — routing one through it would swap the screen
// off the preview the user is still looking at.
//
// NOTHING ELSE IS WRITTEN — no tmux option, no file, no directory read, and no
// prefs read of this layer's own. The persister owns the read-modify-write and the
// path resolution; this layer holds the seam and nothing more.

// commitSelectedConstant is `Enter`: it commits the row under the CURSOR as the
// constant theme.
//
// THE TARGET COMES FROM THE PANEL'S SELECTED ROW, never from m.themeState.keys.
// Those two differ exactly where it matters — the user has arrowed away, or a fallback
// put the cursor somewhere other than the persisted slug — and committing
// the persisted one would write back the theme the user was trying to replace.
//
// THE GUARD IS DEFENSIVE, NOT A LIVE PATH. The arrows skip unselectable rows and
// the open-time anchor lands on a selectable one, so a cursor on an unselectable
// row is structurally unreachable; an empty slug on a
// SELECTABLE row is a shape the union assembly cannot produce at all. Both write
// nothing rather than persisting a setting nothing can resolve — an unselectable
// row carries no palette to commit, and an empty theme name is not a name.
//
// The error is RETURNED rather than handled here, and the REPORT is not the
// caller's job either way: the message-slot line and its outstanding-failure
// state are raised inside commitConstant, so a call site that discards the value
// still reports.
func (m *Model) commitSelectedConstant() error {
	slug, ok := committableThemeSlug(m.themePanel.list)
	if !ok {
		return nil
	}
	return m.commitConstant(slug)
}

// committableThemeSlug is the slug a commit key acts on: the CURSOR's row, or the
// false return that writes nothing.
//
// It states the guard described on commitSelectedConstant ONCE, for the three keys
// that need it — `Enter`, `d`/`l` over a pair, and `d`/`l` over a constant, where
// the slug is what the confirm records as PENDING. Three copies of one defensive
// condition is three places for it to drift apart, and the confirm's copy would be
// the one that matters: it decides what a question the user has already answered
// goes on to write.
func committableThemeSlug(l list.Model) (string, bool) {
	row, ok := selectedThemeRow(l)
	if !ok || !row.Selectable() || row.Slug == "" {
		return "", false
	}
	return row.Slug, true
}

// commitConstant writes theme = slug and clears both slots (mutual exclusion,
// performed on disk by prefs.SaveTheme in ONE atomic write) and
// mirrors that same rule on the model's construction-time raw keys.
//
// A NIL PERSISTER IS INERT, NOT FAILED. A fixture or `capturetool` model wires
// none, so a commit during a capture writes nowhere and mutates nothing — it is the
// ABSENCE OF A WRITER rather than a failed write, so it must never be routed into
// the failure path, which is why applyCommitResult sits
// PAST this guard: there is no report to make and no outstanding state to hold. It
// follows the modePersister nil-guard precedent
// exactly. Mutating the keys anyway would be worse than pointless: it would leave
// the panel claiming a constant nothing persisted, which `Esc` would then resolve
// to.
//
// THE MIRROR IS A MIRROR, NOT A RE-READ. Mutual exclusion is prefs'
// (SaveTheme clears both slots in the same atomic write); applying the same
// transformation to the keys in hand is what keeps the panel's badges and rows in
// step with what was just persisted, with no read-back. The rule itself is
// theme.RawKeys.WithConstant's — prefs is a leaf that cannot import the token
// layer, so the file's half and this one cannot be one mutator, and stating the
// in-memory half anywhere but there would author it a second time.
//
// AND IT IS APPLIED TO THE CONSTRUCTION-TIME SNAPSHOT, never to the merged bytes
// the persister's read-modify-write just had in hand. That RMW necessarily holds
// another instance's writes, and re-deriving from them would make this panel jump
// to another instance's choices at the moment the user presses a key — the
// cross-instance sync Portal explicitly declines, arrived at through the write
// path instead of the open path. The residue is accepted and named there: this
// instance's `●` for a slot it is not acting on shows what it knows rather than
// what is on disk, until relaunch.
//
// ON ERROR NOTHING MOVES. The keys are untouched, so the `●` cannot move — the
// marker means "what is persisted" and would be lying if it moved — and the theme
// stays applied in memory. Without the mutation at all, the close re-resolves stale
// keys and lands the user back on the theme they just replaced,
// which is exactly what the successful path exists to prevent.
//
// THE RESULT GOES THROUGH applyCommitResult BEFORE EITHER BRANCH, and it sits PAST
// the nil guard deliberately: that is what makes the report a statement about a
// WRITE THAT WAS ATTEMPTED rather than about the absence of a writer.
//
// THE RECOMPUTE IS THE LAST STEP AND IS REACHED ONLY FROM HERE — past both early
// returns, and AFTER the mirror, since the re-derivation is a function of the
// keys this line has just set. Both returns above skipping it is the point: a
// failed write must not recompute, and a nil persister wrote nothing for a
// recompute to render.
func (m *Model) commitConstant(slug string) error {
	if m.themeState.persister == nil {
		return nil
	}
	err := m.themeState.persister.CommitTheme(slug)
	m.applyCommitResult(err)
	if err != nil {
		return err
	}
	m.themeState.keys = m.themeState.keys.WithConstant(slug)
	m.recomputeThemePanel()
	return nil
}

// applyCommitResult is the whole failure semantics, in ONE place: what a write
// that did not land reports, and what a write that did land discharges.
//
// IT IS REACHED FROM EVERY COMMIT AND ONLY PAST THE NIL GUARD — `Enter`'s constant,
// `d`/`l` over a pair, and the confirm's `y`, which routes through commitSlot. A nil
// persister returns before it, because that is the ABSENCE OF A WRITER rather than a
// failed write: there is nothing to report and nothing to hold outstanding, so a
// capture model cannot report a failure by COMMITTING (the capture seed declares
// that state directly instead, through the shared report below — a seed is not a
// write).
//
// ON FAILURE IT MUTATES NOTHING ELSE, and the caller's early return is what keeps
// that true: no key mutation, no recompute and no ApplyTheme, so the theme stays
// applied in memory while the `●` cannot move — nothing it derives from changed.
//
// THE TWO LIFETIMES ARE DIFFERENT AND BOTH ARE DELIBERATE. The MESSAGE persists only
// until the next keypress (updateThemePanel clears it), while the STATE runs until
// a subsequent commit succeeds — see themeState.commitFailed for why splitting them
// is what makes the report survive an arrow.
//
// IT DOES NOT LOG. The persister is the single emission site for
// `theme: commit failed`; logging here would double the event and add another
// package to the component's emitters.
func (m *Model) applyCommitResult(err error) {
	if err != nil {
		m.reportCommitFailure()
		return
	}
	m.clearThemePanelCommitFailed()
	m.themeState.commitFailed = false
}

// reportCommitFailure raises the report WHOLE: the message slot's line and the
// outstanding-failure state together.
//
// IT IS THE ONE SITE THAT PAIRS THEM, and the pairing is the point rather than
// tidiness, because their LIFETIMES DIFFER — the line until the next keypress, the
// state until a subsequent commit succeeds — so half a report is a reachable and
// silent wrong answer either way round. A line without the state is dismissed by
// the next arrow and never reaches the close, reinstating the silent revert the
// report exists to close; a state without the line reports nothing at the moment it
// happens, which is the surface the user is looking at.
//
// It is reached from a write that did not land (applyCommitResult) and from the
// capture seed, which DECLARES the state for a frame of it: a fixture wires no
// persister at all, so no capture could arrive here by committing, and the report
// is a designed surface that has to be capturable.
func (m *Model) reportCommitFailure() {
	m.raiseThemePanelCommitFailed()
	m.themeState.commitFailed = true
}

// handleSlotCommitKey is `d` and `l` as the KEYS dispatch them, with the ONE
// decision that has to be made before anything is written.
//
// THE SETTING SHAPE IS GATED FIRST. Under the `theme`-wins rule a non-empty
// constant means the slots are not read at all, so writing a slot here would CLEAR
// THE CONSTANT as a side effect — and a confirm goes in front of exactly that: it
// is the one place a keypress described as inert can silently cost the user a
// setting they chose. So over a constant this keypress ASKS
// (theme_panel_confirm.go) and writes nothing; the write happens on `y`.
//
// THE SHAPE IS READ THROUGH themeSetting, NOT OFF THE RAW KEY. The two answer
// identically — the tiebreak is "a non-empty `theme` wins" and the model's keys
// are already control-stripped — but the tiebreak has ONE site, and what this
// keypress GATES on must not be able to disagree with what the panel LISTS,
// MARKS and RESOLVES, all three of which read it through that same helper.
//
// THE CURSOR'S SLUG IS RESOLVED ON THIS KEYPRESS ON BOTH PATHS, through the same
// guard, so the question names the row the user is looking at and an unselectable
// or slugless row asks nothing rather than asking about a setting nothing can
// resolve.
//
// THE ERROR IS DISCARDED HERE, exactly as `Enter`'s arm discards its own, and
// discarding it costs the user nothing: the message-slot line and its
// outstanding-failure state are raised inside the commit, so the value is a return
// rather than a report waiting to be delivered. It leaves nothing behind either
// way: on failure the raw keys are untouched, so the `●` cannot move. The confirmed
// path reads its own error for a different reason — it gates the newly-live-slot
// load on the write having landed (confirmSlotAssignment).
func (m Model) handleSlotCommitKey(slot prefs.ThemeSlot) (tea.Model, tea.Cmd) {
	if !m.themeSetting().IsConstant {
		_ = (&m).commitSelectedSlot(slot)
		return m, nil
	}
	if slug, ok := committableThemeSlug(m.themePanel.list); ok {
		(&m).raiseSlotConfirm(slug, slot)
	}
	return m, nil
}

// commitSelectedSlot is `d`/`l`: it commits the row under the CURSOR into
// ONE HALF of the adaptive pair.
//
// IT TAKES ITS TARGET EXACTLY AS commitSelectedConstant DOES — the selected row,
// with the same defensive guard — because the two keys differ in WHAT THEY WRITE
// and in nothing else. The guard is defensive for the same reasons stated there:
// the arrows skip unselectable rows and the open-time anchor lands on a
// selectable one, and a selectable row with an empty slug is a shape the union
// assembly cannot produce. Both write nothing rather than persisting a slot
// nothing can resolve.
//
// The error is RETURNED rather than handled here, for the same reason
// commitSelectedConstant returns its own: the report line is raised inside the
// commit rather than by whoever holds the value.
func (m *Model) commitSelectedSlot(slot prefs.ThemeSlot) error {
	slug, ok := committableThemeSlug(m.themePanel.list)
	if !ok {
		return nil
	}
	return m.commitSlot(slug, slot)
}

// commitSlot writes ONE SLOT and clears the constant (mutual exclusion,
// performed on disk by prefs.SaveThemeSlot in ONE atomic write) and mirrors that
// same rule on the model's construction-time raw keys.
//
// IT IS THE MIRROR OF commitConstant AND CARRIES EVERY ONE OF ITS RULES — the
// nil persister is INERT rather than failed, the mirror is a MIRROR rather than a
// re-read, it is applied to the CONSTRUCTION-TIME SNAPSHOT rather than to the
// merged bytes the persister's read-modify-write just had in hand, ON ERROR
// NOTHING MOVES, the result goes through applyCommitResult past that nil guard and
// before either branch, and the recompute is the last step and is reached only past
// both early returns. Those six are stated once, on commitConstant, and hold here
// verbatim; what follows is only what a SLOT commit adds.
//
// THE OTHER SLOT IS LEFT EXACTLY AS IT WAS, and that untouched-other-slot rule is
// not incidental: it is what makes a `● both` badge reachable in two keypresses
// (`d` then `l` on one row), a LIKELY path — where a user lands
// wanting "this theme everywhere" without realising `Enter` is the idiom for it.
// It holds for a slot holding a slug that resolves to nothing just as it does for
// a live one: prefs persists values verbatim and this layer re-derives none of
// them.
//
// THE SLOT IS THE TYPED prefs VALUE, threaded through the seam and never a
// caller-supplied key name, so no path here can mint a third slot — the panel
// names the two constants and nothing else.
//
// THE WRITE IS ONE CALL. Setting the slot and clearing the constant ride a single
// CommitThemeSlot precisely so no partial state is reachable and this layer
// re-implements no merge; a second call to clear the constant would open the
// window the one-atomic-write rule closes. Clearing is writing the EMPTY STRING,
// which `omitempty` renders as key-absent (an unset slot holds the shipped
// default, and a hand-edited file stays clean) — so an
// already-empty constant is deliberately NOT special-cased, in the write or in
// the mirror.
func (m *Model) commitSlot(slug string, slot prefs.ThemeSlot) error {
	if m.themeState.persister == nil {
		return nil
	}
	err := m.themeState.persister.CommitThemeSlot(slug, slot)
	m.applyCommitResult(err)
	if err != nil {
		return err
	}
	m.themeState.keys = mirrorThemeSlot(m.themeState.keys, slug, slot)
	m.recomputeThemePanel()
	return nil
}

// mirrorThemeSlot is the slot direction of the same mutual exclusion, over the
// keys in hand: slug in the named slot, the OTHER slot's raw value carried
// through untouched, and the constant gone.
//
// THE RULE IS theme.RawKeys.WithMember'S, not a second copy of it — exactly as
// commitConstant's clear is WithConstant's. What is left here is the TYPE
// TRANSLATION and nothing more: the keypress threads the typed prefs slot, which
// is what the write takes, while the keys name their half in the token layer's
// own two-valued vocabulary. It is the same prefs-slot-to-Member translation
// oppositeThemeMember makes, minus its flip to the other half.
//
// ANYTHING THAT IS NOT THE LIGHT SLOT IS THE DARK ONE. An out-of-range slot is
// rejected by the store BEFORE this is reached — its guard returns an error and
// commitSlot has already returned on it — and the only values this package names
// are the two constants.
func mirrorThemeSlot(keys theme.RawKeys, slug string, slot prefs.ThemeSlot) theme.RawKeys {
	if slot == prefs.SlotLight {
		return keys.WithMember(theme.MemberLight, slug)
	}
	return keys.WithMember(theme.MemberDark, slug)
}

// recomputeThemePanel is the POST-COMMIT RECOMPUTE: the panel re-rendered
// against the state THIS INSTANCE has just created.
//
// IT RUNS ON A SUCCESSFUL COMMIT AND NOWHERE ELSE — the constant commit, the slot
// commit and the confirmed commit all call it, and nothing else does. A FAILED
// write must not reach it: the badges are derived from the raw keys, which a failed
// write leaves untouched, so recomputing would move the `●` off a theme that is
// still what is persisted — a failed commit must never move the marker.
//
// THE ROW SET MOVES, NOT ONLY THE BADGES, and in BOTH DIRECTIONS. `Enter` clears
// both slots, so a `not found` or charset-rejected row that existed ONLY because a
// slot named it loses its reason to exist and DISAPPEARS; `d`/`l` on a constant
// makes the other slot live, so a slug with no file and no built-in gains the row
// the union owes it and one APPEARS — the open-time union never minted it,
// because a `theme`-wins file's slots are not read at all.
//
// THE INPUTS ARE THE RETAINED ENUMERATION AND THIS INSTANCE'S OWN MUTATED KEYS,
// and each half is a refusal:
//
//   - NOT THE MERGED BYTES. The persister's read-modify-write necessarily holds
//     another instance's writes, and re-deriving from them would make rows and
//     badges jump to another instance's choices at the moment the user presses a
//     key — the cross-instance sync Portal declines, arrived at through the write
//     path instead of the open path.
//   - NOT A FRESH DIRECTORY READ. Enumeration is pinned to panel OPEN, and a
//     commit changes prefs rather than the directory; a read here would also mint a
//     THIRD parse of the same slug — neither construction's nor the panel's — that
//     can disagree with the row the user is looking at.
//
// ACCEPTED RESIDUE: after a concurrent commit elsewhere, this panel's `●`
// for the OTHER instance's slot shows what this instance knows rather than what is
// on disk, until relaunch. That is the same per-instance staleness every prefs
// field already carries under last-write-wins, and it is confined to a slot the
// user is not acting on.
//
// THE ORDER IS LOAD-BEARING at both ends. The identity is captured FIRST, before
// the union moves under it; the cursor is re-anchored LAST, from that identity and
// never from an index — an index silently breaks the cursor invariant the moment the
// reassembly inserts a row ABOVE the cursor, leaving the screen previewing one
// theme while the cursor sits on another. Anchoring inherits the helper's
// clamp-to-first-selectable degradation for the case where the previewed row is no
// longer in the union at all.
//
// ORDERING IS THE ASSEMBLER'S. theme.Reassemble sorts inside itself, so an
// inserted row lands in its alphabetical place with no second comparator here and
// no caller-side sort.
//
// IT EMITS NOTHING. The shared retained-enumeration resolver emits no
// `theme: loaded`; its `theme: fallback applied` WARN may still fire and is
// deduplicated per process on slug+reason, so a persistently broken persisted slug
// does not produce one per keypress. The commit-time `theme: loaded` is wired
// onto the commit entry point deliberately, not onto this body.
//
// THE SEAM IS ALWAYS LIVE HERE, for the same reason `Esc`'s close is: this runs
// only from a commit, a commit only fires while themePanel.open, and `open` is set
// in exactly one place (armThemePanel) reachable only through openThemePanel's nil
// guard.
func (m *Model) recomputeThemePanel() {
	previewed := previewedThemeIdentity(m.themePanel.list)

	m.themePanel.union = m.themeState.enumerator.Reassemble(m.themePanel.enumeration, m.themeState.keys)
	m.refreshThemePanelBadges()

	// The list INSTANCE is kept and its items replaced: a rebuild is the expensive
	// path, and its `bubbles/list`-owned styles are RE-POINTED by the restyle path
	// rather than reassigned here — a commit does not change the
	// theme, so there is nothing to re-point. The returned command is always nil:
	// the panel disables filtering (newThemePanelList), which is the only condition
	// under which SetItems schedules anything.
	_ = m.themePanel.list.SetItems(m.themePanel.rowItems())

	m.anchorThemePanelCursor(previewed)
}

// previewedThemeIdentity is the IDENTITY of the row the panel is previewing — the
// value anchorThemePanelCursor matches on, which is Row.SortKey: the slug wherever
// one exists, else the filename, else the raw persisted string.
//
// The empty string is the anchor's own no-op, and it covers the two shapes that
// have no identity to preserve: a cursor on nothing at all, and the union shape
// that carries no key. Both leave the cursor exactly where SetItems left it.
func previewedThemeIdentity(l list.Model) string {
	row, ok := selectedThemeRow(l)
	if !ok {
		return ""
	}
	return row.SortKey()
}

// refreshThemePanelBadges re-derives the `●` table from a re-resolution of the
// mutated setting against the RETAINED enumeration.
//
// THE RE-RESOLUTION IS FOR THE BADGES ALONE. It never selects a new active member
// and never calls Model.ApplyTheme: a commit is a WRITE, NOT A NAVIGATION,
// so routing it through applyInForceTheme would swap the screen off the preview the
// user is still looking at. What it DOES take from that function is its degrade
// policy, which is stated there once for all three panel call sites.
//
// ON A NON-NIL ERROR THE EXISTING TABLE STANDS, and that is the whole point of not
// deriving from a zero Resolution: theme.Badges returns an EMPTY map for an empty
// slice, so a discarded error would wipe every `●` off the panel at the exact moment
// the user committed one — the marker lying in exactly the direction the
// failed-commit rule exists to forbid. The caller's other steps
// still run on that path: they read the mutated keys and the retained enumeration,
// not the resolution, so the rows still re-derive, re-sort and re-anchor.
func (m *Model) refreshThemePanelBadges() {
	resolution, err := m.themeState.enumerator.Resolve(m.themePanel.enumeration, m.themeSetting())
	if err != nil {
		return
	}
	m.themePanel.badges = theme.Badges(resolution.Slots)
}
