package themetest_test

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

func TestBuiltin_ReturnsTheParsedPalette(t *testing.T) {
	const slug, canvas = "tokyo-night", "#0B0C14"

	got := themetest.Builtin(t, slug)

	if got.Canvas.Value != canvas {
		t.Errorf("Builtin(%q).Canvas = %q, want the embedded file's %q", slug, got.Canvas.Value, canvas)
	}
}

func TestDefaultDarkAndDefaultLight_ResolveTheShippedPair(t *testing.T) {
	dark, light := themetest.DefaultDark(t), themetest.DefaultLight(t)

	if want := themetest.Builtin(t, theme.DefaultDarkSlug); dark != want {
		t.Errorf("DefaultDark() = %+v, want the %q palette %+v", dark, theme.DefaultDarkSlug, want)
	}
	if want := themetest.Builtin(t, theme.DefaultLightSlug); light != want {
		t.Errorf("DefaultLight() = %+v, want the %q palette %+v", light, theme.DefaultLightSlug, want)
	}
	if dark.Canvas.Value == light.Canvas.Value {
		t.Errorf("the shipped pair share canvas %q; they must be distinguishable", dark.Canvas.Value)
	}
}
