package tui

import (
	"charm.land/bubbles/v2/list"
	tea "charm.land/bubbletea/v2"
	"github.com/leeovery/portal/internal/theme"
)

// The panel's commit keys — the only thing the panel writes.
//
// `Enter` does not close. A user who had just set both slots would press it to
// leave and thereby commit a constant, wiping the pair they just built — so `Esc`
// stays the only way out and there is no dual-purpose key. The accepted cost is
// that the common case ("pick one and go") is two keys rather than one.
//
// A commit takes the cursor's slug, never the persisted one: `●` is what is set,
// the cursor is what is previewed, and under a fallback the two differ by design.
//
// It does not re-theme. A commit is a write, not a navigation, so the frame is
// unchanged across the keypress and committing to a non-active slot changes
// nothing on screen. applyInForceTheme states the shared degrade policy for every
// panel call site of Resolve; a commit takes that policy without taking the
// apply, which would swap the screen off the preview the user is still looking at.
//
// Nothing else is written — no tmux option, no file, no directory read, and no
// prefs read of this layer's own. The persister owns the read-modify-write and
// the path resolution; this layer holds the seam.

// commitSelectedConstant is `Enter`: it commits the row under the cursor as the
// constant theme.
//
// The target comes from the panel's selected row, never from m.themeState.keys.
// Those two differ exactly where it matters — the user has arrowed away, or a
// fallback put the cursor somewhere other than the persisted slug — and
// committing the persisted one would write back the theme the user was trying to
// replace.
//
// The guard is defensive rather than a live path: the arrows skip unselectable
// rows and the open-time anchor lands on a selectable one, and an empty slug on a
// selectable row is a shape the union assembly cannot produce. Both write nothing
// rather than persisting a setting nothing can resolve.
//
// The error is returned rather than handled here, and the report is not the
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

// committableThemeSlug is the slug a commit key acts on: the cursor's row, or the
// false return that writes nothing.
//
// It states the guard described on commitSelectedConstant once, for every key
// that needs it — `Enter`, `d`/`l` over a pair, and `d`/`l` over a constant, where
// the slug is what the confirm records as pending. A per-key copy of the
// condition is free to drift, and the confirm's copy is the one that matters: it
// decides what a question the user has already answered goes on to write.
func committableThemeSlug(l list.Model) (string, bool) {
	row, ok := selectedThemeRow(l)
	if !ok || !row.Selectable() || row.Slug == "" {
		return "", false
	}
	return row.Slug, true
}

// commitConstant writes theme = slug and clears both slots (mutual exclusion,
// performed on disk by prefs.SaveTheme in one atomic write) and mirrors that same
// rule on the model's construction-time raw keys.
//
// A nil persister is inert, not failed: a commit during a capture writes nowhere
// and mutates nothing, which is why applyCommitResult sits past this guard.
// Mutating the keys anyway would leave the panel claiming a constant nothing
// persisted, which `Esc` would then resolve to.
//
// The mirror is a mirror, not a re-read: applying prefs' own transformation to
// the keys in hand keeps the panel's badges and rows in step with what was just
// persisted, with no read-back. The rule itself is theme.RawKeys.WithConstant's —
// prefs is a leaf that cannot import the token layer, so the file's half and this
// one cannot be one mutator.
//
// It is applied to the construction-time snapshot, never to the merged bytes the
// persister's read-modify-write just had in hand: that RMW necessarily holds
// another instance's writes, and re-deriving from them would make this panel jump
// to another instance's choices at the moment the user presses a key.
//
// On error nothing moves. The keys are untouched, so the `●` cannot move, and the
// theme stays applied in memory. Without the mutation at all, the close
// re-resolves stale keys and lands the user back on the theme they just replaced.
//
// The result goes through applyCommitResult past the nil guard, which makes the
// report a statement about a write that was attempted rather than about the
// absence of a writer. The recompute is the last step, after the mirror, and both
// early returns skip it: a failed write must not recompute, and a nil persister
// wrote nothing for a recompute to render.
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

// applyCommitResult holds the whole failure semantics: what a write that did not
// land reports, and what a write that did land discharges.
//
// Every commit reaches it past the nil guard, so a capture model — which wires no
// persister — cannot report a failure by committing.
//
// On failure it mutates nothing else, and the caller's early return keeps that
// true: no key mutation, no recompute and no ApplyTheme, so the theme stays
// applied in memory while the `●` cannot move.
//
// The two lifetimes differ deliberately: the message persists only until the next
// keypress, while the state runs until a subsequent commit succeeds — see
// themeState.commitFailed for why splitting them makes the report survive an
// arrow.
//
// It does not log. The persister is the single emission site for
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

// reportCommitFailure raises the report whole: the message slot's line and the
// outstanding-failure state together.
//
// The pairing matters because their lifetimes differ — the line until the next
// keypress, the state until a subsequent commit succeeds — so half a report is a
// reachable and silent wrong answer either way round. A line without the state is
// dismissed by the next arrow and never reaches the close, reinstating the silent
// revert the report exists to close; a state without the line reports nothing on
// the surface the user is looking at.
//
// It is reached from a write that did not land and from the capture seed, which
// declares the state for a frame of it.
func (m *Model) reportCommitFailure() {
	m.raiseThemePanelCommitFailed()
	m.themeState.commitFailed = true
}

// handleSlotCommitKey is `d` and `l` as the keys dispatch them, with the one
// decision that has to be made before anything is written.
//
// The setting shape is gated first. Under the `theme`-wins rule a non-empty
// constant means the slots are not read at all, so writing a slot here would
// clear the constant as a side effect — the case the confirm exists for. Over a
// constant this keypress asks (theme_panel_confirm.go) and writes nothing.
//
// The shape is read through themeSetting rather than off the raw key: the
// tiebreak has one site, and what this keypress gates on must not be able to
// disagree with what the panel lists, marks and resolves.
//
// The cursor's slug is resolved on both paths through the same guard, so the
// question names the row the user is looking at and an unselectable or slugless
// row asks nothing.
//
// The error is discarded here, as `Enter`'s arm discards its own: the report is
// raised inside the commit, and on failure the raw keys are untouched. The
// confirmed path reads its own error to gate the newly-live-slot load on the
// write having landed (confirmSlotAssignment).
func (m Model) handleSlotCommitKey(member theme.Member) (tea.Model, tea.Cmd) {
	if !m.themeSetting().IsConstant {
		_ = (&m).commitSelectedSlot(member)
		return m, nil
	}
	if slug, ok := committableThemeSlug(m.themePanel.list); ok {
		(&m).raiseSlotConfirm(slug, member)
	}
	return m, nil
}

// commitSelectedSlot is `d`/`l`: it commits the row under the cursor into one
// half of the adaptive pair.
//
// It takes its target exactly as commitSelectedConstant does — the selected row,
// with the same defensive guard — because the two keys differ only in what they
// write.
//
// The error is returned rather than handled here, for the same reason
// commitSelectedConstant returns its own: the report line is raised inside the
// commit rather than by whoever holds the value.
func (m *Model) commitSelectedSlot(member theme.Member) error {
	slug, ok := committableThemeSlug(m.themePanel.list)
	if !ok {
		return nil
	}
	return m.commitSlot(slug, member)
}

// commitSlot writes one slot and clears the constant (mutual exclusion,
// performed on disk by prefs.SaveThemeSlot in one atomic write) and mirrors that
// same rule on the model's construction-time raw keys.
//
// It is the mirror of commitConstant and carries every one of its rules, stated
// once there and holding here verbatim: the nil persister is inert rather than
// failed, the mirror is a mirror rather than a re-read, it is applied to the
// construction-time snapshot, on error nothing moves, the result goes through
// applyCommitResult past the nil guard, and the recompute is the last step past
// both early returns. What follows is only what a slot commit adds.
//
// The other slot is left exactly as it was, which is what makes a `● both` badge
// reachable in two keypresses (`d` then `l` on one row) — where a user lands
// wanting "this theme everywhere" without realising `Enter` is the idiom for it.
// It holds for a slot naming a slug that resolves to nothing just as for a live
// one: prefs persists values verbatim and this layer re-derives none of them.
//
// The half is theme.Member, the domain's two-valued light/dark type, so no path
// here can mint a third slot; the persister owns the single translation to what
// prefs.json is keyed by. The write is one call so no partial state is reachable,
// and clearing is writing the empty string, which `omitempty` renders as
// key-absent, so an already-empty constant is deliberately not special-cased.
//
// The mirror is theme.RawKeys.WithMember — the named half set to slug, the other
// half's raw value carried through untouched and the constant gone — as
// commitConstant's is WithConstant.
func (m *Model) commitSlot(slug string, member theme.Member) error {
	if m.themeState.persister == nil {
		return nil
	}
	err := m.themeState.persister.CommitThemeSlot(slug, member)
	m.applyCommitResult(err)
	if err != nil {
		return err
	}
	m.themeState.keys = m.themeState.keys.WithMember(member, slug)
	m.recomputeThemePanel()
	return nil
}

// recomputeThemePanel is the post-commit recompute: the panel re-rendered against
// the state this instance has just created.
//
// A commit that did not land must not recompute. The badges and the nomination
// are derived from the raw keys, which a failed write leaves untouched, so
// recomputing would move the `●` off a theme that is still what is persisted and
// load the palettes of a setting nothing wrote.
//
// The row set moves, not only the badges, and in both directions. `Enter` clears
// both slots, so a `not found` or charset-rejected row that existed only because
// a slot named it disappears; `d`/`l` on a constant makes the other slot live, so
// a slug with no file and no built-in gains the row the union owes it, which the
// open-time union never minted because a `theme`-wins file's slots are not read.
//
// The inputs are the retained enumeration and this instance's own mutated keys,
// and each half is a refusal:
//
//   - Not the merged bytes. The persister's read-modify-write necessarily holds
//     another instance's writes, and re-deriving from them would make rows and
//     badges jump to another instance's choices at the moment the user presses a
//     key.
//   - Not a fresh directory read. A commit changes prefs rather than the
//     directory, and a read here would mint a third parse of the same slug, free
//     to disagree with the row the user is looking at.
//
// Accepted residue: after a concurrent commit elsewhere, this panel's `●` for the
// other instance's slot shows what this instance knows rather than what is on
// disk, until relaunch — the same per-instance staleness every prefs field
// carries under last-write-wins, confined to a slot the user is not acting on.
//
// Order matters at both ends. The identity is captured first, before the union
// moves under it; the cursor is re-anchored last, from that identity and never
// from an index — an index silently breaks the cursor invariant the moment the
// reassembly inserts a row above the cursor. theme.Reassemble sorts inside
// itself, so an inserted row lands in its alphabetical place with no second
// comparator here.
//
// It emits nothing. The shared retained-enumeration resolver emits no
// `theme: loaded`; its `theme: fallback applied` WARN may still fire and is
// deduplicated per process on slug+reason, so a persistently broken persisted
// slug does not produce one per keypress.
//
// The theme source seam is dereferenced without a nil guard: a commit is reachable
// only while themePanel.open, which armThemePanel sets past openThemePanel's nil
// guard.
func (m *Model) recomputeThemePanel() {
	previewed := previewedThemeIdentity(m.themePanel.list)

	m.themePanel.union = m.themeState.source.Reassemble(m.themePanel.enumeration, m.themeState.keys)
	m.applyCommittedSetting()

	// The list instance is kept and its items replaced: a rebuild is the expensive
	// path, and a commit does not change the theme, so there is nothing to
	// re-point. The returned command is always nil: the panel disables filtering,
	// which is the only condition under which SetItems schedules anything.
	_ = m.themePanel.list.SetItems(m.themePanel.rowItems())

	m.anchorThemePanelCursor(previewed)
}

// previewedThemeIdentity is the identity of the row the panel is previewing — the
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

// applyCommittedSetting re-resolves the mutated setting against the retained
// enumeration and takes both of its products: the `●` table and the loaded
// nomination the setting now names.
//
// The nomination rides the badges' own resolution so the two cannot disagree
// about what is set, and so the nomination tracks the persisted setting through
// one site rather than one per kind of commit. It stays a description of what is
// persisted — the palette in force is themeState.active, which this function
// leaves alone.
//
// The re-resolution never selects a new active member and never calls
// Model.ApplyTheme, because a commit is a write rather than a navigation and
// routing it through applyInForceTheme would swap the screen off the preview the
// user is still looking at. It does take that function's degrade policy, stated
// there once for all panel call sites.
//
// On a non-nil error nothing moves rather than either value being derived from a
// zero Resolution: theme.Badges returns an empty map for an empty slice and a zero
// Nomination is the "nothing was injected" sentinel, so a discarded error would
// wipe every `●` off the panel and drop the loaded palettes at the moment the user
// committed one. The caller's other steps still run on that path — they read the
// mutated keys and the retained enumeration, not the resolution.
func (m *Model) applyCommittedSetting() {
	resolution, err := m.themeState.source.Resolve(m.themePanel.enumeration, m.themeSetting())
	if err != nil {
		return
	}
	m.themePanel.badges = theme.Badges(resolution.Slots)
	m.themeState.nomination = resolution.Nomination
}
