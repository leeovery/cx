package tui

import "github.com/leeovery/portal/internal/theme"

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
func (m *Model) commitConstant(slug string) error {
	if m.themePersister == nil {
		return nil
	}
	if err := m.themePersister.CommitTheme(slug); err != nil {
		return err
	}
	m.themeKeys = theme.RawKeys{Theme: slug}
	return nil
}
