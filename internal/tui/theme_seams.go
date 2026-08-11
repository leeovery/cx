package tui

import "github.com/leeovery/portal/internal/theme"

// ThemeSource is the seam through which the theme panel gets its rows AND
// resolves the persisted setting: it assembles the finished union, and it answers
// the per-slot resolution the panel's badges, applied theme and cursor seed all
// derive from. A fake must answer for both halves, not for listing alone.
//
// Open takes the raw persisted theme keys and returns the finished union, never
// a directory listing: every built-in, every `.theme` file, and every persisted
// slug resolving to neither, deduped one slug per row and each carrying its
// single reason. internal/theme owns that assembly, so this package never
// becomes another emitter of the `theme` log component.
//
// The themes directory is absent from the signature, as the state directory is
// from ScrollbackReader's: the production adapter closes over the resolved path,
// so the panel holds no path policy and a fixture — which fakes a Union
// wholesale — needs none.
//
// Reassemble re-derives the union from the enumeration Open retained, with
// changed persisted keys and no fresh I/O; Resolve re-resolves the persisted
// setting against that same enumeration, so the panel opens on the theme the
// enumeration says is live and a mid-session edit takes effect on open; and
// ResolveSlot resolves the slot a constant → adaptive conversion just made live,
// emitting the commit-time `theme: loaded` that Resolve does not.
//
// Every method takes the persisted keys as they stand — no collapsed setting and
// no defaulted slug. The tiebreak and the shipped-default substitution belong to
// internal/theme, so applying either here would let this package resolve against
// a setting derived differently from the one it lists and marks, and would let a
// fake answer from a third.
//
// None of the three reads the filesystem. A read would produce a further parse of
// the same slug, free to disagree with the row the user is looking at. Resolve's
// error is the broken-builtin fatal, which the panel degrades on rather than
// escalating (see Model.applyInForceTheme).
type ThemeSource interface {
	Open(keys theme.RawKeys) (theme.Enumeration, theme.Union)
	Reassemble(e theme.Enumeration, keys theme.RawKeys) theme.Union
	Resolve(e theme.Enumeration, keys theme.RawKeys) (theme.Resolution, error)
	ResolveSlot(e theme.Enumeration, slot theme.Slot, keys theme.RawKeys) (theme.SlotResolution, error)
}
