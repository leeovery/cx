package tui_test

import (
	"path/filepath"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/tui"
)

// Answers from hand-built values, holding no loader and touching no filesystem
// — the shape internal/capture needs under its no-real-config import guard.
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
	// Compile-time assertions: a signature drift stops compiling here rather
	// than at the wiring site.
	var _ tui.ThemeSource = fixtureThemeSource{}
	var _ tui.ThemeSource = theme.DirThemeSource{}
}

// The directory deliberately does not exist: the seam still answers with the
// finished union rather than a directory listing.
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

// All three keys at once, which a hand-edited prefs.json may legally carry: the
// tiebreak is the seam's, not the caller's.
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
