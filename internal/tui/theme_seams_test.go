package tui_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// fixtureThemeEnumerator is the §13.3 fixture shape: a ThemeEnumerator that
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
type fixtureThemeEnumerator struct {
	union      theme.Union
	resolution theme.Resolution
}

func (f fixtureThemeEnumerator) Open(theme.RawKeys) (theme.Enumeration, theme.Union) {
	return theme.Enumeration{}, f.union
}

func (f fixtureThemeEnumerator) Reassemble(theme.Enumeration, theme.RawKeys) theme.Union {
	return f.union
}

func (f fixtureThemeEnumerator) Resolve(theme.Enumeration, theme.Setting) (theme.Resolution, error) {
	return f.resolution, nil
}

func (f fixtureThemeEnumerator) ResolveSlot(_ theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error) {
	return theme.SlotResolution{Slot: slot, Requested: slug, Resolved: slug}, nil
}

// loaderThemeEnumerator is the shape the PRODUCTION adapter takes (task 8-7
// wires it): a theme.Loader plus the resolved themes directory, closed over so
// the seam's Open takes only the persisted keys — the same hiding
// ScrollbackReader applies to the state directory.
type loaderThemeEnumerator struct {
	loader    theme.Loader
	themesDir string
}

func (a loaderThemeEnumerator) Open(keys theme.RawKeys) (theme.Enumeration, theme.Union) {
	return a.loader.Open(a.themesDir, keys)
}

func (a loaderThemeEnumerator) Reassemble(e theme.Enumeration, keys theme.RawKeys) theme.Union {
	return a.loader.Reassemble(e, keys)
}

func (a loaderThemeEnumerator) Resolve(e theme.Enumeration, s theme.Setting) (theme.Resolution, error) {
	return a.loader.ResolveNominationFrom(e, s)
}

func (a loaderThemeEnumerator) ResolveSlot(e theme.Enumeration, slot theme.Slot, slug string) (theme.SlotResolution, error) {
	return a.loader.ResolveSlot(e, slot, slug)
}

func TestThemeEnumeratorIsSatisfiedByAFixtureFakeAndByTheLoader(t *testing.T) {
	// Compile-time assertions: the seam is satisfiable BOTH wholesale by a
	// fixture holding nothing (§13.3) and by a thin closure over
	// theme.Loader.Open / Reassemble. If either internal/theme signature drifts,
	// the second stops compiling here rather than in task 8-7's wiring.
	var _ tui.ThemeEnumerator = fixtureThemeEnumerator{}
	var _ tui.ThemeEnumerator = loaderThemeEnumerator{}
}

// TestThemeEnumeratorReturnsTheFinishedUnion pins §13.3's central claim about
// this seam: it returns the FINISHED §9.4 union, not a directory listing.
//
// The directory here does not exist, and the seam still answers with rows —
// every built-in, plus a row for the persisted slug that resolves to none of
// them, each already carrying its reason. Nothing in internal/tui merges,
// resolves, counts or SORTS anything, which is what keeps this package from
// becoming a fourth emitter of the `theme` component (§8.9 closes that set at
// three).
//
// The persisted row is found by identity rather than by position: it arrives in
// §9.5's order like every other row, which for `ghost` is among the built-ins
// rather than after them.
func TestThemeEnumeratorReturnsTheFinishedUnion(t *testing.T) {
	var enumerator tui.ThemeEnumerator = loaderThemeEnumerator{
		loader:    theme.NewLoader(theme.NewEventLogger(log.Discard())),
		themesDir: filepath.Join(t.TempDir(), "themes"),
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

// TestThemeEnumeratorReassemblesFromAFixtureUnion pins the seam's second method
// through the fixture route: §9.2's post-commit recompute and §5.8's `Esc`
// re-resolution both re-derive from a RETAINED enumeration, so a fake must be
// able to answer that call with no directory of any kind.
func TestThemeEnumeratorReassemblesFromAFixtureUnion(t *testing.T) {
	faked := theme.Union{
		Rows: []theme.Row{
			{Slug: "nord", Source: theme.SourceBuiltin},
			{Slug: "broken", Filename: "broken.theme", Source: theme.SourceFile, Rejection: &theme.Rejection{Reason: theme.ReasonBadColour}},
		},
		DirUnusable: true,
		Count:       2,
		Rejected:    1,
	}
	var enumerator tui.ThemeEnumerator = fixtureThemeEnumerator{union: faked}

	enumeration, opened := enumerator.Open(theme.RawKeys{})
	reassembled := enumerator.Reassemble(enumeration, theme.RawKeys{Theme: "nord"})

	for name, got := range map[string]theme.Union{"Open": opened, "Reassemble": reassembled} {
		if len(got.Rows) != len(faked.Rows) || got.Count != faked.Count || got.Rejected != faked.Rejected || !got.DirUnusable {
			t.Errorf("%s returned %+v, want the faked union %+v", name, got, faked)
		}
	}
}
