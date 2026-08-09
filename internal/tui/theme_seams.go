package tui

import "github.com/leeovery/portal/internal/theme"

// ThemeEnumerator is the seam through which the theme panel gets its rows.
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
// None of the three reads the filesystem. A read would produce a further parse of
// the same slug, free to disagree with the row the user is looking at. Resolve's
// error is the broken-builtin fatal, which the panel degrades on rather than
// escalating (see Model.applyInForceTheme).
type ThemeEnumerator interface {
	Open(keys theme.RawKeys) (theme.Enumeration, theme.Union)
	Reassemble(e theme.Enumeration, keys theme.RawKeys) theme.Union
	Resolve(e theme.Enumeration, s theme.Setting) (theme.Resolution, error)
	ResolveSlot(e theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error)
}
