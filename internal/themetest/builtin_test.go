package themetest_test

import (
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// TestBuiltin_ReturnsTheParsedPalette pins what every consumer takes the helper
// for: the palette the named built-in's committed file parses to, in the
// loader's canonical upper case — not a restatement of it, and not a zero Theme.
//
// It asserts on ONE distinguishing token rather than a whole-struct literal: the
// canvas is the token no two built-ins can share, so it says WHICH theme came
// back without pinning the other eighteen values a built-in is free to retune.
func TestBuiltin_ReturnsTheParsedPalette(t *testing.T) {
	const slug, canvas = "tokyo-night", "#0B0C14"

	got := themetest.Builtin(t, slug)

	if got.Canvas.Value != canvas {
		t.Errorf("Builtin(%q).Canvas = %q, want the embedded file's %q", slug, got.Canvas.Value, canvas)
	}
}

// TestDefaultDarkAndDefaultLight_ResolveTheShippedPair pins the two wrappers as
// the shipped slugs' palettes, and as DIFFERENT palettes — a pair that collapsed
// to one theme would let a light/dark assertion pass on the wrong canvas.
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
