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

	// Returned by both resolvers: Resolve alongside the declared resolution,
	// ResolveSlot with a zero SlotResolution.
	err error

	opens       int
	keys        []theme.RawKeys
	reassembles int
	resolves    []theme.RawKeys
	slotLoads   []slotLoad
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

func (e *fakeThemeSource) Reassemble(theme.Enumeration, theme.RawKeys) theme.Union {
	e.reassembles++
	if e.reassembled != nil {
		return *e.reassembled
	}
	return e.union
}

func (e *fakeThemeSource) Resolve(_ theme.Enumeration, keys theme.RawKeys) (theme.Resolution, error) {
	e.resolves = append(e.resolves, keys)
	return e.resolution, e.err
}

// The fallback branch is the live one on a conversion. Answering from the
// nomination keeps a fake-driven conversion on a real palette — a zero Theme
// would render through lipgloss's no-colour sentinel.
func (e *fakeThemeSource) ResolveSlot(_ theme.Enumeration, slot theme.Slot, keys theme.RawKeys) (theme.SlotResolution, error) {
	setting, _ := theme.ResolveSetting(keys)
	slug := setting.Slug(slot)
	e.slotLoads = append(e.slotLoads, slotLoad{slot: slot, slug: slug})
	if e.err != nil {
		return theme.SlotResolution{}, e.err
	}
	for _, declared := range e.resolution.Slots {
		if declared.Slot == slot {
			return declared, nil
		}
	}
	return theme.SlotResolution{
		Slot:      slot,
		Requested: slug,
		Resolved:  slug,
		Theme:     e.resolution.Nomination.Select(memberForSlot(slot)),
	}, nil
}
