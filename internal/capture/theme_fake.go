package capture

import (
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// Every answer must report the injected `--theme` palette: the panel's open
// and arrow-preview apply whatever the seam reports, so anything else silently
// repaints the frame off `--theme`. No method performs I/O.
type fakeThemeSource struct {
	palette     theme.Theme
	enumeration theme.Enumeration
	union       theme.Union

	// The panel's badge source — slots declared anywhere else render no badge.
	slots []theme.SlotResolution
}

var _ tui.ThemeSource = fakeThemeSource{}

func newFakeThemeSource(palette theme.Theme, enumeration theme.Enumeration, union theme.Union, slots []theme.SlotResolution) fakeThemeSource {
	return fakeThemeSource{
		palette:     palette,
		enumeration: enumeration,
		union:       repaintUnion(union, palette),
		slots:       repaintSlots(slots, palette),
	}
}

func (f fakeThemeSource) Open(theme.RawKeys) (theme.Enumeration, theme.Union) {
	return f.enumeration, f.union
}

// A fixture declares one list and holds it — the recompute inputs are ignored.
func (f fakeThemeSource) Reassemble(theme.Enumeration, theme.RawKeys) theme.Union {
	return f.union
}

// A constant nomination of the injected palette keeps the frame un-gated and
// the panel's open-time apply a no-op.
func (f fakeThemeSource) Resolve(theme.Enumeration, theme.RawKeys) (theme.Resolution, error) {
	return theme.Resolution{
		Nomination: theme.ConstantNomination(f.palette),
		Slots:      f.slots,
	}, nil
}

// A fixture has nowhere to record a load, and the error-only return carries no
// palette, so the frame stays whatever the answering methods reported.
func (f fakeThemeSource) LoadSlot(theme.Enumeration, theme.Slot, theme.RawKeys) error {
	return nil
}

// Rejected rows are left alone deliberately: a rejected row carrying a palette
// is a shape the real assembly cannot produce.
func repaintUnion(union theme.Union, palette theme.Theme) theme.Union {
	rows := make([]theme.Row, len(union.Rows))
	copy(rows, union.Rows)
	for i := range rows {
		if rows[i].Selectable() {
			rows[i].Theme = palette
		}
	}
	union.Rows = rows
	return union
}

func repaintSlots(slots []theme.SlotResolution, palette theme.Theme) []theme.SlotResolution {
	out := make([]theme.SlotResolution, len(slots))
	copy(out, slots)
	for i := range out {
		out[i].Theme = palette
	}
	return out
}
