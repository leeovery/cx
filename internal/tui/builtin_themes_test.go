package tui

import (
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

func TestDefaultDarkTheme_SeedsTheShippedPaletteSilently(t *testing.T) {
	sink := logtest.Install(t)

	seeded := defaultDarkTheme()

	if got := sink.Records(); len(got) != 0 {
		t.Errorf("the seed emitted %d record(s), want none:\n%s", len(got), sink.Body())
	}
	if want := themetest.DefaultDark(t); !slices.Equal(seeded.All(), want.All()) {
		t.Errorf("seeded palette = %+v, want the shipped dark built-in's %+v", seeded.All(), want.All())
	}

	dir := t.TempDir()
	themetest.Write(t, dir, "Bad_Name.theme", themetest.MonochromeLines("#101010"))
	if _, rejection := theme.NewLoader(theme.NewEventLogger(log.For("theme"))).Enumerate(dir); rejection != nil {
		t.Fatalf("the control directory was rejected: %v", rejection)
	}
	if len(sink.Records()) == 0 {
		t.Fatal("an emitting loader wrote nothing either — the silence above is a sink that was never wired")
	}
}
