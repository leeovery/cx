package tui

import (
	"github.com/leeovery/portal/internal/theme"
)

// fakeThemeSource is the shared ThemeSource fake behind the panel suites: it
// answers every seam method from DECLARED VALUES, touching no filesystem and holding
// no loader, and the cases those suites differ on are FIELDS on it rather than second
// types.
//
// It is how the degrade paths are driven at all, none of which a real loader can
// produce against a correctly built binary: a `Resolve` returning the build-time guarantee's
// fatal, a resolved slug the union has no row for, and a selectable row vanishing from under
// the cursor.
//
// It also RECORDS what it was asked, which is how a suite driven off declared
// values observes the ASK itself rather than only its effect.
type fakeThemeSource struct {
	// enumeration is the parse Open hands back for the panel to retain, so an
	// assertion on the retained value has a declared one to compare against.
	enumeration theme.Enumeration

	// union is the row set Open answers with — and the one Reassemble answers with
	// too, unless reassembled declares otherwise.
	union theme.Union

	// reassembled, when non-nil, is the DIFFERENT union the recompute's reassembly
	// answers with, which is how the recompute's own effect is isolated: whatever
	// the panel lists after a commit came from the REASSEMBLY, and whatever it lists
	// without one came from the open. Nil is the ordinary fixture, where the one
	// declared union answers both calls.
	reassembled *theme.Union

	// resolution is what both resolving methods answer from: the fixture's `●`
	// slots, the palette the panel opens on, and the nomination a slot load falls
	// back to.
	resolution theme.Resolution

	// err is the error both resolving methods return. Resolve returns it ALONGSIDE
	// the declared resolution rather than instead of it — see Resolve. ResolveSlot
	// returns it with a ZERO SlotResolution, so a degrade has nothing to render.
	err error

	// opens counts the Open calls, which is what pins the open cadence — the read is
	// per open, and a refusal that decides before reading has run none at all.
	opens int

	// keys records the RawKeys every Open was handed, which is what pins the panel
	// reading prefs from the construction-time snapshot rather than from disk.
	keys []theme.RawKeys

	// reassembles counts the Reassemble calls, which is what makes "only a
	// SUCCESSFUL commit recomputes" observable: a reassembly that ran replaces the
	// panel's rows outright, so unchanged rows plus a zero count is the recompute
	// genuinely not happening.
	reassembles int

	// settings records every setting Resolve was asked to re-resolve, which is what
	// pins the open's and the close's resolution counts — an arrow previews from the
	// parse already in hand and must resolve nothing.
	settings []theme.Setting

	// slotLoads records every commit-time slot resolution the seam was asked
	// for. Two cases read it: requireNoSlotLoad, for a NON-CONVERTING commit asking
	// for none at all, and the build-time guarantee degrade case, for a conversion whose no-op
	// has to be the seam's ANSWER rather than a load that never ran.
	slotLoads []slotLoad
}

// slotLoad is one recorded ResolveSlot call.
type slotLoad struct {
	slot theme.Slot
	slug string
}

// Open records the ask and answers with the declared parse and union.
func (e *fakeThemeSource) Open(keys theme.RawKeys) (theme.Enumeration, theme.Union) {
	e.opens++
	e.keys = append(e.keys, keys)
	return e.enumeration, e.union
}

// Reassemble counts the ask and answers with the split reassembly where the fixture
// declared one, else with the single union it declared.
func (e *fakeThemeSource) Reassemble(theme.Enumeration, theme.RawKeys) theme.Union {
	e.reassembles++
	if e.reassembled != nil {
		return *e.reassembled
	}
	return e.union
}

// Resolve records the ask and answers with the declared resolution AND the declared
// error TOGETHER, which is what lets each degrade fixture state the shape its own
// assertions need.
//
// A POPULATED resolution alongside the error is what keeps an OPEN's degrade
// assertions from being vacuous: an open that ignored the error would badge the
// resolved slug, repaint the screen in its palette and move the cursor onto its row,
// and each of those is asserted against. The loader's own shape is the opposite —
// the build-time guarantee returns the ZERO Resolution alongside the fatal, precisely so there
// is nothing to be tempted to render — and that is what a BADGE-REFRESH fixture
// declares instead, since theme.Badges yields an empty map for the empty slot slice
// a zero Resolution carries, so a refresh that ignored the error would wipe every
// `●` off the panel.
func (e *fakeThemeSource) Resolve(_ theme.Enumeration, setting theme.Setting) (theme.Resolution, error) {
	e.settings = append(e.settings, setting)
	return e.resolution, e.err
}

// ResolveSlot records the ask and answers with the declared resolution's record for
// that slot — or, for a slot the fixture declared no record for, with the declared
// NOMINATION's member for it.
//
// The fallback branch is the live one on a conversion: a fixture whose resolution is
// a constant (newArrowPanelDeps) declares only a SlotConstant record, and the
// construction-time load rule's load asks for the opposite light/dark slot. Answering it from
// the nomination is what keeps a fake-driven conversion joining a palette the fixture chose —
// a zero Theme in a live nomination slot renders through lipgloss's no-colour sentinel, which
// is the silent colourless render New's dark seed exists to keep out of this package.
func (e *fakeThemeSource) ResolveSlot(_ theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error) {
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
