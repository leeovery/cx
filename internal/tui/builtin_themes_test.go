package tui

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// TestDefaultDarkTheme_SeedsTheShippedPaletteSilently pins both halves of the
// seed: it resolves the shipped dark built-in's palette, and it writes not one
// `theme` record while doing so.
//
// The silence is the point of the seed being a SEED: the component records where
// a theme is used, and a model priming itself with the shipped palette before any
// nomination is applied has used nothing the user chose.
//
// The emitting run over a staged directory is the non-vacuity control — the sink
// is the process handler, so it proves this component's records reach it and that
// the silence above is a genuine absence rather than an unwired sink.
func TestDefaultDarkTheme_SeedsTheShippedPaletteSilently(t *testing.T) {
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)

	seeded := defaultDarkTheme()

	if got := sink.Records(); len(got) != 0 {
		t.Errorf("the seed emitted %d record(s), want none:\n%s", len(got), sink.Body())
	}
	if want := themetest.DefaultDark(t); !slices.Equal(seeded.All(), want.All()) {
		t.Errorf("seeded palette = %+v, want the shipped dark built-in's %+v", seeded.All(), want.All())
	}

	dir := t.TempDir()
	writeThemeFileForTest(t, dir, "Bad_Name.theme", "#101010")
	if _, rejection := theme.NewLoader(theme.NewEventLogger(log.For("theme"))).Enumerate(dir); rejection != nil {
		t.Fatalf("the control directory was rejected: %v", rejection)
	}
	if len(sink.Records()) == 0 {
		t.Fatal("an emitting loader wrote nothing either — the silence above is a sink that was never wired")
	}
}
