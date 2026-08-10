package themetest

import (
	"fmt"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// syntheticGreenBase / syntheticBlueBase are the per-token ramps: token i takes
// green base+i and blue base+i, so the values within one palette are unique and
// every component stays three decimal digits.
const (
	syntheticGreenBase = 0x80 // 128
	syntheticBlueBase  = 0xC8 // 200
)

// syntheticRedFloor is the lowest red channel that still renders as three
// decimal digits.
const syntheticRedFloor = 0x64 // 100

// SyntheticPalette builds a whole palette from a fixed red channel: every token
// in the vocabulary carries a value, unique within the palette, and two palettes
// built from different reds share none.
//
// SHIPPED PALETTES ARE DELIBERATELY NOT USED as probes. Two shipped themes fail
// two ways: a hex both palettes happen to set identically survives a swap
// LEGITIMATELY, so a probe fails permanently for a non-bug; and — worse, because
// it is silent — a token with the same value either side renders identically
// before and after, so the probe cannot tell whether that site updated. It
// passes whether or not it did, and the site is uncovered with no signal.
//
// Every channel is THREE decimal digits (red at or above syntheticRedFloor,
// green and blue from the ramps), so a rendered SGR core is fixed-width
// `38;2;RRR;GGG;BBB` and one token's core can never be a substring of another's
// — which would otherwise let the "the stale value is absent" half of a probe's
// assertions pass vacuously. A red below the floor is fatal rather than
// silently narrow.
//
// theme.Theme is an ordinary struct, so a palette built here needs no loader, no
// file and no embedded set — which is what keeps probes independent of anything
// done to the shipped themes. It is also why the completeness assertion below
// earns its place: a token added to the vocabulary would otherwise leave this
// literal compiling with a zero-valued field, and a probe reading a zero token
// can neither diff it nor detect a stale value. The assertion turns that into a
// loud failure, naming the token, in every guard that probes with this palette.
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

	tokens := palette.All()
	if len(tokens) != len(theme.TokenNames()) {
		t.Fatalf("synthetic palette enumerated %d tokens, the vocabulary has %d", len(tokens), len(theme.TokenNames()))
	}
	for _, tok := range tokens {
		if tok.Value == "" {
			t.Fatalf("synthetic palette left token %q empty; add it to the builder", tok.Name)
		}
	}
	return palette
}

// SyntheticPair builds two palettes that share no token value — the before and
// after a swap probe diffs against each other.
//
// Equal reds are fatal: the pair would be identical, and every "the stale value
// is gone" assertion made with it would pass vacuously.
func SyntheticPair(t *testing.T, redA, redB uint8) (a, b theme.Theme) {
	t.Helper()

	if redA == redB {
		t.Fatalf("synthetic pair needs two different reds, both are %#x", redA)
	}
	return SyntheticPalette(t, redA), SyntheticPalette(t, redB)
}
