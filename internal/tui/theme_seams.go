package tui

import "github.com/leeovery/portal/internal/theme"

// Every method takes the raw persisted keys — the tiebreak and shipped-default
// substitution belong to internal/theme, so this package never emits the `theme`
// log component. No method reads the filesystem: a further parse of the same slug
// could disagree with the row on screen. ResolveSlot emits the commit-time
// `theme: loaded` that Resolve does not; Resolve's error is the broken-builtin fatal.
type ThemeSource interface {
	Open(keys theme.RawKeys) (theme.Enumeration, theme.Union)
	Reassemble(e theme.Enumeration, keys theme.RawKeys) theme.Union
	Resolve(e theme.Enumeration, keys theme.RawKeys) (theme.Resolution, error)
	ResolveSlot(e theme.Enumeration, slot theme.Slot, keys theme.RawKeys) (theme.SlotResolution, error)
}
