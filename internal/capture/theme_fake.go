package capture

import (
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// fakeThemeSource is the panel's theme seam answered wholly from DECLARED
// VALUES: the fixture's faked union, its retained enumeration and its per-slot
// resolution records, with the palette threaded in from `--theme`.
//
// # THE PALETTE IS INJECTED, AND EVERY ANSWER REPORTS IT
//
// The fake is constructed with THE SAME theme.Theme the model's nomination
// carries — capturetool's resolved `--theme` value, or the palette the fixture
// constructor was handed — and it reports that palette as the resolution's
// nomination, as every SlotResolution.Theme, and on every selectable union row.
//
// That is not tidiness. The panel's OPEN applies the theme its Resolve return
// names, through Model.ApplyTheme, BEFORE the cursor seed runs. Reporting the
// injected palette is what makes that apply a NO-OP. Three distinct failures
// follow from reporting anything else, and NONE OF THEM IS LOUD:
//
//   - A ZERO-VALUED resolution renders the whole panel through
//     lipgloss.Color("")'s no-colour sentinel — silently colourless, no compile
//     error, nothing to notice. It is exactly the hazard tui.New's own built-in
//     seed exists to close.
//   - A HARD-CODED BUILT-IN repaints the frame in a palette `--theme` did not
//     name, so a fixture would render in a palette its own doc comment and its
//     `--theme` both contradict — and the panel frames are the only visual
//     reference the light/dark slot half has.
//   - Inside the swap-and-diff completeness guard the same apply OVERWRITES the
//     synthetic theme the model was handed, so a panel fixture contributes
//     neither an A value nor a B value: the negative assertion passes vacuously,
//     the union balances, and the panel's own bubbles/list instance — the worst
//     case of the cached-style class — is covered by nothing while reading as
//     covered.
//
// The ROWS carry it for a fourth reason, which belongs to the live view rather
// than to a still: the arrow-preview applies the cursor row's own palette, so
// rows left with a zero Theme would paint the frame colourless the moment anyone
// pressed `↓` in `go run ./cmd/capturetool`. A fake holds ONE palette, so every
// row reporting it is the honest answer — arrowing previews the same colours
// rather than none.
//
// # IT PERFORMS NO I/O ON ANY OF ITS FOUR METHODS
//
// That is the requirement rather than a property it happens to have. The
// no-real-config import guard forbids internal/capture reaching config at all, so
// faking the seam wholesale is the ONLY way the harness renders a panel — an
// invalid-theme row, a persisted-but-missing slug, an unreadable themes directory
// — without a real themes directory to stage any of it in. It holds no
// theme.Loader, resolves no path and reads no file, and this package's own source
// is scanned to keep it that way.
type fakeThemeSource struct {
	// palette is the injected `--theme` value every answer reports.
	palette theme.Theme

	// enumeration is the fixture's declared retained parse, handed back by Open
	// and never consulted.
	enumeration theme.Enumeration

	// union is the fixture's declared row set, already repainted onto the
	// injected palette.
	union theme.Union

	// slots are the fixture's declared badge source, already repainted onto the
	// injected palette. THIS RETURN IS THE PANEL'S BADGE SOURCE, so a fixture
	// declaring its slots anywhere else renders no `●` on any row at all.
	slots []theme.SlotResolution
}

// Satisfying the seam is the whole point of the type, so a signature drift is a
// compile error here rather than a nil seam at the wiring site.
var _ tui.ThemeSource = fakeThemeSource{}

// newFakeThemeSource binds a fixture's declared panel data to the palette
// the frame is being rendered at, repainting every answer onto it (see the type
// comment for why every one of them must report it).
func newFakeThemeSource(palette theme.Theme, enumeration theme.Enumeration, union theme.Union, slots []theme.SlotResolution) fakeThemeSource {
	return fakeThemeSource{
		palette:     palette,
		enumeration: enumeration,
		union:       repaintUnion(union, palette),
		slots:       repaintSlots(slots, palette),
	}
}

// Open hands back the declared enumeration and union — the fixture's whole list,
// with no directory read and no keys consulted.
func (f fakeThemeSource) Open(theme.RawKeys) (theme.Enumeration, theme.Union) {
	return f.enumeration, f.union
}

// Reassemble hands back the same declared union. The post-commit recompute
// re-derives from the retained parse with changed prefs state; a fixture
// declares one list and holds it.
func (f fakeThemeSource) Reassemble(theme.Enumeration, theme.RawKeys) theme.Union {
	return f.union
}

// Resolve reports the fixture's declared slots, every one of them carrying the
// injected palette, under a CONSTANT nomination of that same palette — the shape
// capturetool always passes, so the frame is un-gated and byte-stable.
//
// The declared slots are what the badges come from, and the injected palette is
// what keeps the panel's open-time apply a no-op. See the type comment.
func (f fakeThemeSource) Resolve(theme.Enumeration, theme.Setting) (theme.Resolution, error) {
	return theme.Resolution{
		Nomination: theme.ConstantNomination(f.palette),
		Slots:      f.slots,
	}, nil
}

// ResolveSlot answers the commit-time load with the injected palette under the
// slot and slug it was asked for — the fake's one palette, reported as every other
// answer reports it (see the type comment).
//
// It is unreachable in a capture: a fixture wires NO theme persister, so the
// conversion this serves returns before the load. It is implemented honestly
// anyway, because a fake answering a fourth method differently from its other
// three is exactly the divergence the injected palette exists to prevent.
func (f fakeThemeSource) ResolveSlot(_ theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error) {
	return theme.SlotResolution{Slot: slot, Requested: slug, Resolved: slug, Theme: f.palette}, nil
}

// repaintUnion returns the union with every SELECTABLE row's palette replaced by
// the injected one, leaving the row identities, sources and rejections exactly as
// the fixture declared them.
//
// Rejected rows are left alone deliberately: theme.Row populates Theme IFF
// Rejection is nil, and a rejected row that came back carrying a palette would be
// a shape the real assembly cannot produce.
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

// repaintSlots returns the slot records with every palette replaced by the
// injected one, leaving each record's slot, requested and resolved slugs — the
// fields the badge table keys on — exactly as the fixture declared them.
func repaintSlots(slots []theme.SlotResolution, palette theme.Theme) []theme.SlotResolution {
	out := make([]theme.SlotResolution, len(slots))
	copy(out, slots)
	for i := range out {
		out[i].Theme = palette
	}
	return out
}
