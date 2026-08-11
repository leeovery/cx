package tui

import (
	"github.com/leeovery/portal/internal/theme"
)

// Drives the degrade paths — a fatal Resolve, a resolved slug the union has no
// row for, a selectable row vanishing under the cursor — none of which a real
// loader can produce against a correctly built binary.
type fakeThemeSource struct {
	enumeration theme.Enumeration
	union       theme.Union

	// When non-nil, the different union the recompute's reassembly answers with,
	// isolating the recompute's own effect from the open's.
	reassembled *theme.Union

	resolution theme.Resolution

	// Returned by both Resolve, alongside the declared resolution, and LoadSlot.
	err error

	opens          int
	keys           []theme.RawKeys
	reassembles    int
	reassembleKeys []theme.RawKeys
	resolves       []theme.RawKeys
	slotLoads      []slotLoad
}

type slotLoad struct {
	slot theme.Slot
	slug string
}

func (e *fakeThemeSource) Open(keys theme.RawKeys) (theme.Enumeration, theme.Union) {
	e.opens++
	e.keys = append(e.keys, keys)
	return e.enumeration, e.union
}

func (e *fakeThemeSource) Reassemble(_ theme.Enumeration, keys theme.RawKeys) theme.Union {
	e.reassembles++
	e.reassembleKeys = append(e.reassembleKeys, keys)
	if e.reassembled != nil {
		return *e.reassembled
	}
	return e.union
}

func (e *fakeThemeSource) Resolve(_ theme.Enumeration, keys theme.RawKeys) (theme.Resolution, error) {
	e.resolves = append(e.resolves, keys)
	return e.resolution, e.err
}

// The slug is recorded rather than answered with, so a test can see which slot
// the commit asked for and on which slug.
func (e *fakeThemeSource) LoadSlot(_ theme.Enumeration, slot theme.Slot, keys theme.RawKeys) error {
	e.slotLoads = append(e.slotLoads, slotLoad{slot: slot, slug: theme.SlugForSlot(keys, slot)})
	return e.err
}
