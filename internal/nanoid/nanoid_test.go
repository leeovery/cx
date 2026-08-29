package nanoid_test

import (
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/nanoid"
)

func TestAlphabet_MatchesExpectedCharset(t *testing.T) {
	// The absence of "-" is load-bearing for the "<batch>-<token>" marker split.
	const want = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	if nanoid.Alphabet != want {
		t.Errorf("Alphabet = %q, want %q", nanoid.Alphabet, want)
	}
	for _, forbidden := range []rune{'.', ':', '-', ' '} {
		if strings.ContainsRune(nanoid.Alphabet, forbidden) {
			t.Errorf("Alphabet contains forbidden rune %q; the option-name-safe marker scheme requires its absence", forbidden)
		}
	}
}

func TestNewGenerator(t *testing.T) {
	t.Run("it mints a distinct id per call", func(t *testing.T) {
		gen := nanoid.NewGenerator()
		seen := map[string]struct{}{}
		for range 200 {
			id, err := gen()
			if err != nil {
				t.Fatalf("generate id: %v", err)
			}
			if _, dup := seen[id]; dup {
				t.Fatalf("generator minted %q twice in 200 calls", id)
			}
			seen[id] = struct{}{}
		}
	})
}

func TestIsTokenShaped_AcceptsAGeneratedID(t *testing.T) {
	t.Run("it accepts a six-character alphanumeric key as token-shaped", func(t *testing.T) {
		for _, key := range []string{"abc123", "AbCdEf", "000000", "zZ9aQ0"} {
			if !nanoid.IsTokenShaped(key) {
				t.Errorf("IsTokenShaped(%q) = false, want true", key)
			}
		}
	})

	t.Run("it accepts every id the generator mints", func(t *testing.T) {
		gen := nanoid.NewGenerator()
		for range 200 {
			id, err := gen()
			if err != nil {
				t.Fatalf("generate id: %v", err)
			}
			if !nanoid.IsTokenShaped(id) {
				t.Fatalf("IsTokenShaped(%q) = false for a generated id, want true", id)
			}
		}
	})
}

func TestIsTokenShaped_Rejects(t *testing.T) {
	t.Run("it rejects a five-character and a seven-character key", func(t *testing.T) {
		for _, key := range []string{"abc12", "abc1234"} {
			if nanoid.IsTokenShaped(key) {
				t.Errorf("IsTokenShaped(%q) = true, want false", key)
			}
		}
	})

	t.Run("it rejects a key carrying a character outside the alphabet", func(t *testing.T) {
		for _, key := range []string{"abc:23", "abc.23", "abc-23", "abc_23", "abc 23", "abc12!"} {
			if nanoid.IsTokenShaped(key) {
				t.Errorf("IsTokenShaped(%q) = true, want false", key)
			}
		}
	})

	t.Run("it rejects an empty key as token-shaped", func(t *testing.T) {
		if nanoid.IsTokenShaped("") {
			t.Error(`IsTokenShaped("") = true, want false`)
		}
	})

	t.Run("it rejects a multi-byte input", func(t *testing.T) {
		sixBytes := "éàî"    // three 2-byte runes
		sixRunes := "日本語のうた" // six 3-byte runes
		if len(sixBytes) != 6 {
			t.Fatalf("fixture %q is %d bytes, want 6", sixBytes, len(sixBytes))
		}
		if len([]rune(sixRunes)) != 6 {
			t.Fatalf("fixture %q is %d runes, want 6", sixRunes, len([]rune(sixRunes)))
		}
		for _, key := range []string{sixBytes, sixRunes} {
			if nanoid.IsTokenShaped(key) {
				t.Errorf("IsTokenShaped(%q) = true, want false", key)
			}
		}
	})

	t.Run("it rejects every old-format key shape", func(t *testing.T) {
		for _, key := range []string{"my-session:0.0", "abc123:12.3", "portal-abc123:0.1"} {
			if nanoid.IsTokenShaped(key) {
				t.Errorf("IsTokenShaped(%q) = true, want false", key)
			}
		}
	})
}
