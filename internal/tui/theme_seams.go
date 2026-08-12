package tui

import "github.com/leeovery/portal/internal/theme"

// Every method takes the raw persisted keys — the tiebreak and shipped-default
// substitution belong to internal/theme, so this package never emits the `theme`
// log component. Open performs the one directory read; the others must not — a
// further parse of the same slug could disagree with the row on screen. LoadSlot
// resolves the named slot and
// records the load — the commit-time `theme: loaded` emission Resolve
// deliberately does not make. Resolve's error is the broken-builtin fatal.
type ThemeSource interface {
	Open(keys theme.RawKeys) (theme.Enumeration, theme.Union)
	Reassemble(e theme.Enumeration, keys theme.RawKeys) theme.Union
	Resolve(e theme.Enumeration, keys theme.RawKeys) (theme.Resolution, error)
	LoadSlot(e theme.Enumeration, slot theme.Slot, keys theme.RawKeys) error
}
