package tui

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// The slot is one the keys leave unset, so an implementation reaching past the
// shared collapse answers an empty slug rather than the shipped default.
func TestThemeSourceImplementations_LoadTheSameSlug(t *testing.T) {
	keys := theme.RawKeys{Light: "nord"}
	want := theme.SlugForSlot(keys, theme.SlotDark)
	if want != theme.DefaultDarkSlug {
		t.Fatalf("fixture: the unset dark slot collapses to %q, want the shipped default %q", want, theme.DefaultDarkSlug)
	}

	dir := newConversionThemesDir(t)
	loader, sink := themeOpenTestLoader(t)
	production := countingEnumeratorOver(loader, dir)
	enumeration, _ := production.Open(keys)
	if err := production.LoadSlot(enumeration, theme.SlotDark, keys); err != nil {
		t.Fatalf("the production adapter's LoadSlot: %v", err)
	}
	loaded := themeEventRecords(sink, "loaded")
	if len(loaded) != 1 {
		t.Fatalf("the production adapter emitted %d `theme: loaded` line(s), want exactly 1\n%s", len(loaded), sink.Body())
	}
	if got := loaded[0].AttrString(t, "slug"); got != want {
		t.Errorf("the production adapter loaded %q, want the collapsed %q", got, want)
	}

	fake := &fakeThemeSource{}
	if err := fake.LoadSlot(theme.Enumeration{}, theme.SlotDark, keys); err != nil {
		t.Fatalf("the fake's LoadSlot: %v", err)
	}
	if len(fake.slotLoads) != 1 {
		t.Fatalf("the fake recorded %d slot load(s), want 1", len(fake.slotLoads))
	}
	if got := fake.slotLoads[0].slug; got != want {
		t.Errorf("the fake loaded %q, want the collapsed %q", got, want)
	}
}
