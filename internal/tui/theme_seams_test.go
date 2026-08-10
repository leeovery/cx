package tui_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// fixtureThemeSource is the harness contract fixture shape: a ThemeSource that
// answers with a HAND-BUILT union, holding no loader, naming no directory and
// touching no filesystem.
//
// It is what lets internal/capture render a panel — including the invalid rows
// and the unusable-directory chrome — under the no-real-config import guard that
// forbids it reaching a real themes directory at all.
//
// Its Resolve is declared the same way: a hand-built theme.Resolution, which is
// where a fixture states its `●` slots and the palette the panel opens on now that
// the injected slot record is retired.
type fixtureThemeSource struct {
	union      theme.Union
	resolution theme.Resolution
}

func (f fixtureThemeSource) Open(theme.RawKeys) (theme.Enumeration, theme.Union) {
	return theme.Enumeration{}, f.union
}

func (f fixtureThemeSource) Reassemble(theme.Enumeration, theme.RawKeys) theme.Union {
	return f.union
}

func (f fixtureThemeSource) Resolve(theme.Enumeration, theme.Setting) (theme.Resolution, error) {
	return f.resolution, nil
}

func (f fixtureThemeSource) ResolveSlot(_ theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error) {
	return theme.SlotResolution{Slot: slot, Requested: slug, Resolved: slug}, nil
}

func TestThemeSourceIsSatisfiedByAFixtureFakeAndByTheExportedAdapter(t *testing.T) {
	// Compile-time assertions: the seam is satisfiable BOTH wholesale by a
	// fixture holding nothing and by theme.DirThemeSource, the exported
	// adapter production wires (cmd's newThemeSource returns it). A drift in
	// either signature stops compiling here rather than at the wiring site — and
	// because the adapter asserted is production's own rather than a
	// re-implementation of it, this cannot pass while production fails.
	var _ tui.ThemeSource = fixtureThemeSource{}
	var _ tui.ThemeSource = theme.DirThemeSource{}
}

// TestThemeSourceReturnsTheFinishedUnion pins the harness contract's central claim about
// this seam: it returns the FINISHED union, not a directory listing.
//
// The directory here does not exist, and the seam still answers with rows —
// every built-in, plus a row for the persisted slug that resolves to none of
// them, each already carrying its reason. Nothing in internal/tui merges,
// resolves, counts or SORTS anything, which is what keeps this package from
// becoming a fourth emitter of the `theme` component (the concurrent-write rule closes that
// set at three).
//
// The persisted row is found by identity rather than by position: it arrives in
// the row-rendering rule's order like every other row, which for `ghost` is among the
// built-ins rather than after them.
func TestThemeSourceReturnsTheFinishedUnion(t *testing.T) {
	var enumerator tui.ThemeSource = theme.DirThemeSource{
		Loader: theme.NewSilentLoader(),
		Dir:    filepath.Join(t.TempDir(), "themes"),
	}

	_, union := enumerator.Open(theme.RawKeys{Theme: "ghost"})

	if union.Count != len(union.Rows) {
		t.Errorf("union count = %d, want len(Rows) = %d", union.Count, len(union.Rows))
	}
	if want := len(theme.BuiltinSlugs()) + 1; len(union.Rows) != want {
		t.Fatalf("union has %d rows, want %d — the built-ins plus the unresolvable persisted slug", len(union.Rows), want)
	}
	at := slices.IndexFunc(union.Rows, func(row theme.Row) bool { return row.Slug == "ghost" })
	if at < 0 {
		t.Fatalf("union rows = %+v, want one for the unresolvable persisted slug %q", union.Rows, "ghost")
	}
	persisted := union.Rows[at]
	if persisted.Source != theme.SourcePersisted {
		t.Errorf("the %q row is a %v, want %v", "ghost", persisted.Source, theme.SourcePersisted)
	}
	if persisted.Rejection == nil || persisted.Rejection.Reason != theme.ReasonNotFound {
		t.Errorf("persisted row rejection = %v, want %q — the reason arrives with the row", persisted.Rejection, theme.ReasonNotFound)
	}
	if union.Rejected != 1 {
		t.Errorf("union rejected = %d, want 1 — the counts arrive assembled too", union.Rejected)
	}
}

// TestThemeSourceReassemblesFromAFixtureUnion pins the seam's second method
// through the fixture route: the picker idiom's post-commit recompute and the re-read-on-open
// rule's `Esc` re-resolution both re-derive from a RETAINED enumeration, so a fake must be
// able to answer that call with no directory of any kind.
func TestThemeSourceReassemblesFromAFixtureUnion(t *testing.T) {
	faked := theme.Union{
		Rows: []theme.Row{
			{Slug: "nord", Source: theme.SourceBuiltin},
			{Slug: "broken", Filename: "broken.theme", Source: theme.SourceFile, Rejection: &theme.Rejection{Reason: theme.ReasonBadColour}},
		},
		DirUnusable: true,
		Count:       2,
		Rejected:    1,
	}
	var enumerator tui.ThemeSource = fixtureThemeSource{union: faked}

	enumeration, opened := enumerator.Open(theme.RawKeys{})
	reassembled := enumerator.Reassemble(enumeration, theme.RawKeys{Theme: "nord"})

	for name, got := range map[string]theme.Union{"Open": opened, "Reassemble": reassembled} {
		if len(got.Rows) != len(faked.Rows) || got.Count != faked.Count || got.Rejected != faked.Rejected || !got.DirUnusable {
			t.Errorf("%s returned %+v, want the faked union %+v", name, got, faked)
		}
	}
}
