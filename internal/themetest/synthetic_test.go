package themetest_test

import (
	"fmt"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// TestSyntheticPalette_FillsEveryTokenInCanonicalOrder pins the three properties
// every guard probing with this palette rests on: every token in the vocabulary
// carries a value (a zero token can neither be diffed nor detected as stale), no
// two tokens share one (a swap that missed a site must be distinguishable), and
// the values are the ones the ramp defines, in the canonical table order.
func TestSyntheticPalette_FillsEveryTokenInCanonicalOrder(t *testing.T) {
	const red = 0x6E

	tokens := themetest.SyntheticPalette(t, red).All()

	names := theme.TokenNames()
	if len(tokens) != len(names) {
		t.Fatalf("palette enumerated %d tokens, the vocabulary has %d", len(tokens), len(names))
	}

	seen := make(map[string]string, len(tokens))
	for i, tok := range tokens {
		if tok.Name != names[i] {
			t.Errorf("token %d is %q, canonical order has %q", i, tok.Name, names[i])
		}
		want := fmt.Sprintf("#%02X%02X%02X", red, 0x80+i+1, 0xC8+i+1)
		if tok.Value != want {
			t.Errorf("token %q = %q, want %q", tok.Name, tok.Value, want)
		}
		if other, dup := seen[tok.Value]; dup {
			t.Errorf("token %q shares value %s with %q", tok.Name, tok.Value, other)
		}
		seen[tok.Value] = tok.Name
	}
}

// TestSyntheticPalette_ChannelsAreThreeDecimalDigits pins the substring property
// the "the stale colour is absent" half of every probe rests on: a rendered SGR
// core is fixed-width, so one token's run can never be a substring of another's.
func TestSyntheticPalette_ChannelsAreThreeDecimalDigits(t *testing.T) {
	for _, red := range []uint8{0x64, 0xAA, 0xFF} {
		for _, tok := range themetest.SyntheticPalette(t, red).All() {
			var r, g, b int
			if _, err := fmt.Sscanf(tok.Value, "#%02x%02x%02x", &r, &g, &b); err != nil {
				t.Fatalf("token %q value %q is not #RRGGBB: %v", tok.Name, tok.Value, err)
			}
			for channel, value := range map[string]int{"red": r, "green": g, "blue": b} {
				if value < 100 || value > 255 {
					t.Errorf("red %#x: token %q %s channel is %d, outside the three-digit 100–255 range", red, tok.Name, channel, value)
				}
			}
		}
	}
}

// TestSyntheticPair_SharesNoTokenValue pins what makes a pair usable as a
// before/after probe: no value survives the swap legitimately, so any theme-A
// value found in a theme-B frame is a site that never got the new theme.
func TestSyntheticPair_SharesNoTokenValue(t *testing.T) {
	a, b := themetest.SyntheticPair(t, 0x6E, 0xD2)

	inA := make(map[string]string, len(a.All()))
	for _, tok := range a.All() {
		inA[tok.Value] = tok.Name
	}
	for _, tok := range b.All() {
		if other, shared := inA[tok.Value]; shared {
			t.Errorf("token %q value %s also appears in the first palette as %q", tok.Name, tok.Value, other)
		}
	}
}
