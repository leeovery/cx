package tui

import (
	"github.com/leeovery/portal/internal/theme"
)

// The shared ThemeSource fake: it answers every seam method from declared
// values, touching no filesystem, and records what it was asked. It is how the
// degrade paths are driven at all — a fatal Resolve, a resolved slug the union
// has no row for, and a selectable row vanishing from under the cursor — none
// of which a real loader can produce against a correctly built binary.
type fakeThemeSource struct {
	enumeration theme.Enumeration
	union       theme.Union

	// When non-nil, the different union the recompute's reassembly answers
	// with, isolating the recompute's own effect from the open's.
	reassembled *theme.Union

	resolution theme.Resolution

	// Returned by both resolving methods: Resolve returns it alongside the
	// declared resolution, ResolveSlot with a zero SlotResolution.
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

// The resolution is returned alongside the error, not instead of it: an open
// that ignored the error would badge the resolved slug, repaint in its palette
// and move the cursor onto its row, each of which a degrade fixture asserts
// against. A badge-refresh fixture declares the opposite (a zero Resolution),
// since theme.Badges yields an empty map for its empty slot slice.
func (e *fakeThemeSource) Resolve(_ theme.Enumeration, keys theme.RawKeys) (theme.Resolution, error) {
	e.resolves = append(e.resolves, keys)
	return e.resolution, e.err
}

// The fallback branch is the live one on a conversion: a constant-resolution
// fixture declares only a SlotConstant record while the load asks for the
// opposite half. Answering from the nomination keeps a fake-driven conversion
// on a real palette — a zero Theme would render through lipgloss's no-colour
// sentinel.
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
