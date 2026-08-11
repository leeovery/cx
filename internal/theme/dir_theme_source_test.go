package theme_test

import (
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
)

func TestSlugForSlot_CollapsesTheRawKeysOntoOneSlot(t *testing.T) {
	requireDistinctDefaults(t)

	tests := []struct {
		name string
		keys theme.RawKeys
		slot theme.Slot
		want string
	}{
		{name: "a set light slot", keys: theme.RawKeys{Light: "nord"}, slot: theme.SlotLight, want: "nord"},
		{name: "an unset dark slot", keys: theme.RawKeys{Light: "nord"}, slot: theme.SlotDark, want: theme.DefaultDarkSlug},
		{name: "an unset light slot", keys: theme.RawKeys{Dark: "nord"}, slot: theme.SlotLight, want: theme.DefaultLightSlug},
		{name: "the constant wins the tiebreak", keys: theme.RawKeys{Theme: "nord", Light: "sunset"}, slot: theme.SlotConstant, want: "nord"},
		{name: "an unset constant", keys: theme.RawKeys{}, slot: theme.SlotConstant, want: ""},
		{name: "a control-stripped slug", keys: theme.RawKeys{Dark: "\x1b[31mnord"}, slot: theme.SlotDark, want: "nord"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := theme.SlugForSlot(tt.keys, tt.slot); got != tt.want {
				t.Errorf("SlugForSlot(%+v, %v) = %q, want %q", tt.keys, tt.slot, got, tt.want)
			}
		})
	}
}

// The slot is one the keys leave unset, so the recorded slug can only be the
// shipped default the collapse substitutes.
func TestDirThemeSource_LoadSlotRecordsTheCollapsedSlug(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	source := theme.DirThemeSource{
		Loader: theme.NewLoader(theme.NewEventLogger(logger)),
		Dir:    filepath.Join(t.TempDir(), "themes"),
	}
	keys := theme.RawKeys{Light: "nord"}
	enumeration, _ := source.Open(keys)

	if err := source.LoadSlot(enumeration, theme.SlotDark, keys); err != nil {
		t.Fatalf("LoadSlot: %v", err)
	}

	loaded := recordsNamed(sink, "loaded")
	if len(loaded) != 1 {
		t.Fatalf("LoadSlot emitted %d `theme: loaded` record(s), want exactly 1:\n%s", len(loaded), sink.Body())
	}
	if got := loaded[0].AttrString(t, "slug"); got != theme.DefaultDarkSlug {
		t.Errorf("`theme: loaded` carries slug=%q, want the unset slot's shipped default %q", got, theme.DefaultDarkSlug)
	}
	if got, want := loaded[0].AttrString(t, "slot"), "dark"; got != want {
		t.Errorf("`theme: loaded` carries slot=%q, want %q", got, want)
	}
}

func TestDirThemeSource_ResolveRecordsNoLoad(t *testing.T) {
	logger, sink := logtest.NewCaptureLogger(t)
	source := theme.DirThemeSource{
		Loader: theme.NewLoader(theme.NewEventLogger(logger)),
		Dir:    filepath.Join(t.TempDir(), "themes"),
	}
	keys := theme.RawKeys{Light: "nord"}
	enumeration, _ := source.Open(keys)

	if _, err := source.Resolve(enumeration, keys); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	if n := len(recordsNamed(sink, "loaded")); n != 0 {
		t.Errorf("Resolve emitted %d `theme: loaded` record(s), want none — recording the load is LoadSlot's job alone:\n%s", n, sink.Body())
	}
}
