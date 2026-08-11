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

func (f fixtureThemeSource) Resolve(theme.Enumeration, theme.RawKeys) (theme.Resolution, error) {
	return f.resolution, nil
}

func (f fixtureThemeSource) ResolveSlot(_ theme.Enumeration, slot theme.Slot, keys theme.RawKeys) (theme.SlotResolution, error) {
	setting, _ := theme.ResolveSetting(keys)
	slug := setting.Slug(slot)
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

// TestThemeSourceResolvesFromTheRawKeys pins the collapse as the SEAM's, not the
// caller's: the adapter is handed prefs.json's keys exactly as they are persisted
// — all three at once, which a hand-edited file may legally carry — and applies
// the tiebreak itself.
//
// The keys rather than a collapsed setting is what keeps the rule behind the
// seam. A caller collapsing first is free to answer from a setting derived
// differently from the one it lists and marks.
func TestThemeSourceResolvesFromTheRawKeys(t *testing.T) {
	var source tui.ThemeSource = theme.DirThemeSource{
		Loader: theme.NewSilentLoader(),
		Dir:    filepath.Join(t.TempDir(), "themes"),
	}

	enumeration, _ := source.Open(theme.RawKeys{})
	resolution, err := source.Resolve(enumeration, theme.RawKeys{Theme: "nord", Light: "tokyo-night-day", Dark: "tokyo-night"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if len(resolution.Slots) != 1 {
		t.Fatalf("the resolution holds %d slot(s), want 1 — a non-empty `theme` wins and the slots are not read", len(resolution.Slots))
	}
	if got := resolution.Slots[0]; got.Slot != theme.SlotConstant || got.Requested != "nord" {
		t.Errorf("the resolved slot is %+v, want the constant %q", got, "nord")
	}
	if !resolution.Nomination.IsConstant() {
		t.Errorf("the nomination is adaptive, want the constant shape the winning `theme` key names")
	}
}

// TestThemeSourceResolvesASlotFromTheRawKeys pins the OTHER rule the seam now
// owns: the shipped-default substitution for a slot the user never set.
//
// The unset slot resolves the shipped default and reports no fallback, which is
// the distinction that would be lost if a caller handed over a raw empty slug: an
// untouched slot would be reported as a fallback of a slug nobody set.
func TestThemeSourceResolvesASlotFromTheRawKeys(t *testing.T) {
	var source tui.ThemeSource = theme.DirThemeSource{
		Loader: theme.NewSilentLoader(),
		Dir:    filepath.Join(t.TempDir(), "themes"),
	}

	enumeration, _ := source.Open(theme.RawKeys{})
	slot, err := source.ResolveSlot(enumeration, theme.SlotDark, theme.RawKeys{Light: "nord"})
	if err != nil {
		t.Fatalf("ResolveSlot: %v", err)
	}

	if slot.Requested != theme.DefaultDarkSlug {
		t.Errorf("the unset dark slot requested %q, want the shipped default %q", slot.Requested, theme.DefaultDarkSlug)
	}
	if slot.Resolved != theme.DefaultDarkSlug || slot.FellBack {
		t.Errorf("the unset dark slot resolved %q (fell back: %v), want %q with no fallback", slot.Resolved, slot.FellBack, theme.DefaultDarkSlug)
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
