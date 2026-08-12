package themetest

import (
	"fmt"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// Per-token ramps: token i takes green base+i and blue base+i, so values within
// one palette are unique and every component stays three decimal digits.
const (
	syntheticGreenBase = 0x80
	syntheticBlueBase  = 0xC8
)

// The lowest red channel that still renders as three decimal digits.
const syntheticRedFloor = 0x64

// SyntheticPalette builds a whole palette from a fixed red channel: every token
// carries a value, unique within the palette, and two palettes built from
// different reds share none. Shipped palettes are unusable as swap probes — a
// token both themes value identically renders the same either side, so the probe
// passes whether or not the site updated. Three decimal digits per channel keeps
// a rendered SGR core fixed-width, so one token's core can never be a substring
// of another's and let the "stale value is absent" half pass vacuously.
func SyntheticPalette(t *testing.T, red uint8) theme.Theme {
	t.Helper()

	if red < syntheticRedFloor {
		t.Fatalf("synthetic red %#x renders as fewer than three decimal digits; use %#x or above", red, syntheticRedFloor)
	}

	v := func(i int) theme.Token {
		return theme.Token{Value: fmt.Sprintf("#%02X%02X%02X", red, syntheticGreenBase+i, syntheticBlueBase+i)}
	}
	palette := theme.Theme{
		TextPrimary:      v(1),
		TextSecondary:    v(2),
		TextTertiary:     v(3),
		TextMuted:        v(4),
		TextSubtle:       v(5),
		TextFaint:        v(6),
		TextOnSelection:  v(7),
		AccentPrimary:    v(8),
		AccentKey:        v(9),
		AccentMode:       v(10),
		AccentAttention:  v(11),
		StatePositive:    v(12),
		StateDestructive: v(13),
		Canvas:           v(14),
		BgSelection:      v(15),
		BgAttention:      v(16),
		BgSubtle:         v(17),
		Border:           v(18),
		TextOnAttention:  v(19),
	}

	for _, tok := range palette.All() {
		if tok.Value == "" {
			t.Fatalf("synthetic palette left token %q empty; add it to the builder", tok.Name)
		}
	}
	return palette
}

// SyntheticPair builds two palettes that share no token value — the before and
// after a swap probe diffs. Equal reds are fatal: an identical pair would make
// every "the stale value is gone" assertion pass vacuously.
func SyntheticPair(t *testing.T, redA, redB uint8) (a, b theme.Theme) {
	t.Helper()

	if redA == redB {
		t.Fatalf("synthetic pair needs two different reds, both are %#x", redA)
	}
	return SyntheticPalette(t, redA), SyntheticPalette(t, redB)
}
