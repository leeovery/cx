package tui

import (
	"charm.land/bubbles/v2/list"
	"github.com/leeovery/portal/internal/theme"
)

// §9.2's COMMIT KEYS, and the first thing the panel writes. Browsing landed as
// its own checkpoint with `Enter`/`d`/`l` deliberately swallowed; this is where
// four decisions that read as small all bite.
//
// `Enter` DOES NOT CLOSE. A user who had just set both slots would press it to
// leave and thereby commit a constant, wiping the pair they just built — so `Esc`
// stays the only way out, there is no dual-purpose key, and the pair flow needs no
// special case. The accepted cost is that the common case ("pick one and go") is
// two keys rather than one.
//
// IT COMMITS THE CURSOR'S SLUG, NEVER THE PERSISTED ONE. The cursor is what is
// previewed and what the user is looking at, and §9.5 draws that split
// deliberately: `●` is what is SET, the cursor is what is PREVIEWED. Under §8.5's
// fallback the two differ by design.
//
// IT DOES NOT RE-THEME. A commit is a WRITE, NOT A NAVIGATION, so the frame is
// unchanged across the keypress and committing to a non-active slot (task 9-3)
// changes nothing on screen. applyInForceTheme states the shared degrade policy
// for every panel call site of Resolve and is explicit that a commit takes that
// policy WITHOUT taking the apply — routing one through it would swap the screen
// off the preview the user is still looking at.
//
// NOTHING ELSE IS WRITTEN — no tmux option, no file, no directory read, and no
// prefs read of this layer's own. The persister owns the read-modify-write (§8.9)
// and the path resolution; this layer holds the seam and nothing more.

// commitSelectedConstant is §9.2's `Enter`: it commits the row under the CURSOR
// as the constant theme.
//
// THE TARGET COMES FROM THE PANEL'S SELECTED ROW, never from m.themeKeys. Those
// two differ exactly where it matters — the user has arrowed away, or §8.5's
// fallback put the cursor somewhere other than the persisted slug — and committing
// the persisted one would write back the theme the user was trying to replace.
//
// THE GUARD IS DEFENSIVE, NOT A LIVE PATH. The arrows skip unselectable rows (task
// 8-9) and the open-time anchor lands on a selectable one (task 8-8), so a cursor
// on an unselectable row is structurally unreachable; an empty slug on a
// SELECTABLE row is a shape the union assembly cannot produce at all. Both write
// nothing rather than persisting a setting nothing can resolve — an unselectable
// row carries no palette to commit, and an empty theme name is not a name.
//
// The error is RETURNED rather than handled here: task 9-7 owns §9.13's message
// slot line, the outstanding-failure state, and the fact that the theme stays
// applied in memory.
func (m *Model) commitSelectedConstant() error {
	row, ok := selectedThemeRow(m.themePanel.list)
	if !ok || !row.Selectable() || row.Slug == "" {
		return nil
	}
	return m.commitConstant(row.Slug)
}

// commitConstant writes theme = slug and clears both slots (§8.2 mutual
// exclusion, performed on disk by prefs.SaveTheme in ONE atomic write) and
// mirrors that same rule on the model's construction-time raw keys.
//
// A NIL PERSISTER IS INERT, NOT FAILED. A fixture or `capturetool` model wires
// none (task 6-7), so a commit during a capture writes nowhere and mutates
// nothing — it is the ABSENCE OF A WRITER rather than a failed write, so it must
// never be routed into task 9-7's failure path: there is no report to make and no
// outstanding state to hold. It follows the modePersister nil-guard precedent
// exactly. Mutating the keys anyway would be worse than pointless: it would leave
// the panel claiming a constant nothing persisted, which `Esc` would then resolve
// to.
//
// THE MIRROR IS A MIRROR, NOT A RE-READ. §8.2's mutual exclusion is prefs'
// (SaveTheme clears both slots in the same atomic write); restating it here as an
// in-memory assignment is what keeps the panel's badges and rows in step with what
// was just persisted, with no second implementation of the rule and no read-back.
//
// AND IT IS APPLIED TO THE CONSTRUCTION-TIME SNAPSHOT (§8.4), never to the merged
// bytes the persister's read-modify-write just had in hand. That RMW necessarily
// holds another instance's writes, and re-deriving from them would make this
// panel jump to another instance's choices at the moment the user presses a key —
// the cross-instance sync §8.4 explicitly declines, arrived at through the write
// path instead of the open path. The residue is accepted and named there: this
// instance's `●` for a slot it is not acting on shows what it knows rather than
// what is on disk, until relaunch.
//
// ON ERROR NOTHING MOVES. The keys are untouched, so the `●` cannot move — §9.13's
// marker means "what is persisted" and would be lying if it moved — and the theme
// stays applied in memory. Without the mutation at all, task 8-10's close
// re-resolves stale keys and lands the user back on the theme they just replaced,
// which is exactly what the successful path exists to prevent.
//
// THE RECOMPUTE IS THE LAST STEP AND IS REACHED ONLY FROM HERE — past both early
// returns, and AFTER the mirror, since §9.2's re-derivation is a function of the
// keys this line has just set. Both returns above skipping it is the point: a
// failed write must not recompute (§9.13), and a nil persister wrote nothing for a
// recompute to render.
func (m *Model) commitConstant(slug string) error {
	if m.themePersister == nil {
		return nil
	}
	if err := m.themePersister.CommitTheme(slug); err != nil {
		return err
	}
	m.themeKeys = theme.RawKeys{Theme: slug}
	m.recomputeThemePanel()
	return nil
}

// recomputeThemePanel is §9.2's POST-COMMIT RECOMPUTE: the panel re-rendered
// against the state THIS INSTANCE has just created.
//
// IT RUNS ON A SUCCESSFUL COMMIT AND NOWHERE ELSE — every commit path calls it
// (this one, task 9-3's slot commit, task 9-5's confirmed commit) and nothing
// else does. A FAILED write must not reach it: the badges are derived from the
// raw keys, which a failed write leaves untouched, so recomputing would move the
// `●` off a theme that is still what is persisted — §9.13's "a failed commit does
// not move the `●`" is the rule that forbids exactly that.
//
// THE ROW SET MOVES, NOT ONLY THE BADGES, and in BOTH DIRECTIONS. `Enter` clears
// both slots, so a `not found` or charset-rejected row that existed ONLY because a
// slot named it loses its reason to exist and DISAPPEARS; `d`/`l` on a constant
// makes the other slot live (§8.2), so a slug with no file and no built-in gains
// the row §9.4 requires and one APPEARS — the open-time union never minted it,
// because a `theme`-wins file's slots are not read at all.
//
// THE INPUTS ARE THE RETAINED ENUMERATION AND THIS INSTANCE'S OWN MUTATED KEYS,
// and each half is a refusal:
//
//   - NOT THE MERGED BYTES. The persister's read-modify-write (§8.9) necessarily
//     holds another instance's writes, and re-deriving from them would make rows
//     and badges jump to another instance's choices at the moment the user presses
//     a key — the cross-instance sync §8.4 declines, arrived at through the write
//     path instead of the open path.
//   - NOT A FRESH DIRECTORY READ. §5.8 pins enumeration to panel OPEN, and a
//     commit changes prefs rather than the directory; a read here would also mint a
//     THIRD parse of the same slug — neither construction's nor the panel's — that
//     can disagree with the row the user is looking at.
//
// ACCEPTED RESIDUE (§8.9): after a concurrent commit elsewhere, this panel's `●`
// for the OTHER instance's slot shows what this instance knows rather than what is
// on disk, until relaunch. That is the same per-instance staleness every prefs
// field already carries under last-write-wins, and it is confined to a slot the
// user is not acting on.
//
// THE ORDER IS LOAD-BEARING at both ends. The identity is captured FIRST, before
// the union moves under it; the cursor is re-anchored LAST, from that identity and
// never from an index — an index silently breaks §9.2's invariant the moment the
// reassembly inserts a row ABOVE the cursor, leaving the screen previewing one
// theme while the cursor sits on another. Anchoring inherits the helper's
// clamp-to-first-selectable degradation for the case where the previewed row is no
// longer in the union at all.
//
// ORDERING IS THE ASSEMBLER'S. theme.Reassemble sorts inside itself (§9.5), so an
// inserted row lands in its alphabetical place with no second comparator here and
// no caller-side sort.
//
// IT EMITS NOTHING. The shared retained-enumeration resolver emits no
// `theme: loaded` (§12.3); its `theme: fallback applied` WARN may still fire and is
// deduplicated per process on slug+reason, so a persistently broken persisted slug
// does not produce one per keypress. §12.3's commit-time `theme: loaded` is wired
// onto the commit entry point deliberately, not onto this body.
//
// THE SEAM IS ALWAYS LIVE HERE, for the same reason `Esc`'s close is: this runs
// only from a commit, a commit only fires while themePanel.open, and `open` is set
// in exactly one place (armThemePanel) reachable only through openThemePanel's nil
// guard.
func (m *Model) recomputeThemePanel() {
	previewed := previewedThemeIdentity(m.themePanel.list)

	m.themePanel.union = m.themeEnumerator.Reassemble(m.themePanel.enumeration, m.themeKeys)
	m.refreshThemePanelBadges()

	// The list INSTANCE is kept and its items replaced: §11.1 rules a rebuild out
	// as the expensive path, and its `bubbles/list`-owned styles are RE-POINTED by
	// task 8-9's restyle rather than reassigned here — a commit does not change the
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

// refreshThemePanelBadges re-derives §9.5's `●` table from a re-resolution of the
// mutated setting against the RETAINED enumeration.
//
// THE RE-RESOLUTION IS FOR THE BADGES ALONE. It never selects a new active member
// and never calls Model.ApplyTheme: a commit is a WRITE, NOT A NAVIGATION (§9.2),
// so routing it through applyInForceTheme would swap the screen off the preview the
// user is still looking at. What it DOES take from that function is its degrade
// policy, which is stated there once for all three panel call sites.
//
// ON A NON-NIL ERROR THE EXISTING TABLE STANDS, and that is the whole point of not
// deriving from a zero Resolution: theme.Badges returns an EMPTY map for an empty
// slice, so a discarded error would wipe every `●` off the panel at the exact moment
// the user committed one — the marker lying in the direction §9.13's "a failed
// commit does not move the `●`" rule exists to forbid. The caller's other steps
// still run on that path: they read the mutated keys and the retained enumeration,
// not the resolution, so the rows still re-derive, re-sort and re-anchor.
func (m *Model) refreshThemePanelBadges() {
	resolution, err := m.themeEnumerator.Resolve(m.themePanel.enumeration, m.themeSetting())
	if err != nil {
		return
	}
	m.themePanel.badges = theme.Badges(resolution.Slots)
}
