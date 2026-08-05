package capture

import (
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// fakeThemeEnumerator is §13.3's panel seam answered wholly from DECLARED
// VALUES: the fixture's faked union, its retained enumeration and its per-slot
// resolution records, with the palette threaded in from `--theme`.
//
// IT PERFORMS NO I/O ON ANY OF ITS THREE METHODS, and that is the requirement
// rather than a property it happens to have. §7.1's no-real-config import guard
// forbids internal/capture reaching config at all, so faking the seam wholesale
// is the ONLY way the harness renders a panel — an invalid-theme row, a
// persisted-but-missing slug, an unreadable themes directory — without a real
// themes directory to stage any of it in. It holds no theme.Loader, resolves no
// path and reads no file; this package's own file is scanned to keep it that way
// (TestFakeThemeEnumerator_NoIO).
//
// # THE PALETTE IS INJECTED, AND EVERY ANSWER REPORTS IT
//
// The fake is constructed with THE SAME theme.Theme the model's nomination
// carries — capturetool's resolved `--theme` value, or the palette task 4-2's
// ModelAt was handed — and it reports that palette as the resolution's
// nomination, as every SlotResolution.Theme, and on every selectable union row.
//
// That is not tidiness. Task 8-8's panel OPEN applies the theme its Resolve
// return names, through Model.ApplyTheme, BEFORE the cursor seed runs. Reporting
// the injected palette is what makes that apply a NO-OP. Three distinct failures
// follow from reporting anything else, and NONE OF THEM IS LOUD:
//
//   - A ZERO-VALUED resolution renders the whole panel through
//     lipgloss.Color("")'s no-colour sentinel — silently colourless, no compile
//     error, no failing assertion. It is exactly the hazard tui.New's own
//     built-in seed exists to close.
//   - A HARD-CODED BUILT-IN repaints the frame in a palette `--theme` did not
//     name, so `theme-panel-constant-previewing` would render in the persisted
//     constant `nord` while its doc comment and its `--theme` both say
//     `tokyo-night-day` — the one frame §9.14 calls the only reference that exists
//     for the slot half, contradicting the coherence rule it is captured under.
//   - Inside §13.4's swap-and-diff guard the same apply OVERWRITES the synthetic
//     theme ModelAt was handed, so a panel fixture contributes neither an A value
//     nor a B value: assertion 1 passes as a vacuous negative, assertion 2's union
//     balances, and the panel's own bubbles/list instance — §11.2's worst case of
//     the cached-style class — is covered by nothing while reading as covered.
//
// The ROWS carry it for a fourth reason, which belongs to the live view rather
// than to a still: §9.2's arrow-preview applies the cursor row's own palette, so
// rows left with a zero Theme would paint the frame colourless the moment anyone
// pressed `↓` in `go run ./cmd/capturetool`. A fake holds ONE palette, so every
// row reporting it is the honest answer — arrowing previews the same colours
// rather than none.
type fakeThemeEnumerator struct {
	// palette is the injected `--theme` value every answer reports.
	palette theme.Theme

	// enumeration is the fixture's declared retained parse (§5.8), handed back by
	// Open and never consulted.
	enumeration theme.Enumeration

	// union is the fixture's declared §9.4 row set, already repainted onto the
	// injected palette.
	union theme.Union

	// slots are the fixture's declared §9.5 badge source, already repainted onto
	// the injected palette. THIS RETURN IS THE PANEL'S BADGE SOURCE — task 8-8
	// retired the injected slot record — so a fixture declaring its slots anywhere
	// else renders no `●` on any row at all.
	slots []theme.SlotResolution
}

// Satisfying the seam is the whole point of the type, so a signature drift is a
// compile error here rather than a nil enumerator at the wiring site.
var _ tui.ThemeEnumerator = fakeThemeEnumerator{}

// newFakeThemeEnumerator binds a fixture's declared panel data to the palette
// the frame is being rendered at, repainting every answer onto it (see the type
// comment for why every one of them must report it).
func newFakeThemeEnumerator(palette theme.Theme, enumeration theme.Enumeration, union theme.Union, slots []theme.SlotResolution) fakeThemeEnumerator {
	return fakeThemeEnumerator{
		palette:     palette,
		enumeration: enumeration,
		union:       repaintUnion(union, palette),
		slots:       repaintSlots(slots, palette),
	}
}

// Open hands back the declared enumeration and union — the fixture's whole
// §9.4 list, with no directory read and no keys consulted.
func (f fakeThemeEnumerator) Open(theme.RawKeys) (theme.Enumeration, theme.Union) {
	return f.enumeration, f.union
}

// Reassemble hands back the same declared union. §9.2's post-commit recompute
// re-derives from the retained parse with changed prefs state; a fixture
// declares one list and holds it.
func (f fakeThemeEnumerator) Reassemble(theme.Enumeration, theme.RawKeys) theme.Union {
	return f.union
}

// Resolve reports the fixture's declared slots, every one of them carrying the
// injected palette, under a CONSTANT nomination of that same palette — the shape
// capturetool always passes (§13.3), so the frame is un-gated and byte-stable.
//
// The declared slots are what the badges come from, and the injected palette is
// what keeps task 8-8's open-time apply a no-op. See the type comment.
func (f fakeThemeEnumerator) Resolve(theme.Enumeration, theme.Setting) (theme.Resolution, error) {
	return theme.Resolution{
		Nomination: theme.ConstantNomination(f.palette),
		Slots:      f.slots,
	}, nil
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
// fields §9.5's badge table keys on — exactly as the fixture declared them.
func repaintSlots(slots []theme.SlotResolution, palette theme.Theme) []theme.SlotResolution {
	out := make([]theme.SlotResolution, len(slots))
	copy(out, slots)
	for i := range out {
		out[i].Theme = palette
	}
	return out
}
