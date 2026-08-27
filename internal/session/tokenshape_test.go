package session_test

import (
	"testing"

	"github.com/leeovery/portal/internal/session"
)

func TestIsTokenShaped_AcceptsAGeneratedID(t *testing.T) {
	t.Run("it accepts a six-character alphanumeric key as token-shaped", func(t *testing.T) {
		for _, key := range []string{"abc123", "AbCdEf", "000000", "zZ9aQ0"} {
			if !session.IsTokenShaped(key) {
				t.Errorf("IsTokenShaped(%q) = false, want true", key)
			}
		}
	})

	t.Run("it accepts every id the generator mints", func(t *testing.T) {
		gen := session.NewNanoIDGenerator()
		for range 200 {
			id, err := gen()
			if err != nil {
				t.Fatalf("generate id: %v", err)
			}
			if !session.IsTokenShaped(id) {
				t.Fatalf("IsTokenShaped(%q) = false for a generated id, want true", id)
			}
		}
	})
}

func TestIsTokenShaped_Rejects(t *testing.T) {
	t.Run("it rejects a five-character and a seven-character key", func(t *testing.T) {
		for _, key := range []string{"abc12", "abc1234"} {
			if session.IsTokenShaped(key) {
				t.Errorf("IsTokenShaped(%q) = true, want false", key)
			}
		}
	})

	t.Run("it rejects a key carrying a character outside the alphabet", func(t *testing.T) {
		for _, key := range []string{"abc:23", "abc.23", "abc-23", "abc_23", "abc 23", "abc12!"} {
			if session.IsTokenShaped(key) {
				t.Errorf("IsTokenShaped(%q) = true, want false", key)
			}
		}
	})

	t.Run("it rejects an empty key as token-shaped", func(t *testing.T) {
		if session.IsTokenShaped("") {
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
			if session.IsTokenShaped(key) {
				t.Errorf("IsTokenShaped(%q) = true, want false", key)
			}
		}
	})

	t.Run("it rejects every old-format key shape", func(t *testing.T) {
		for _, key := range []string{"my-session:0.0", "abc123:12.3", "portal-abc123:0.1"} {
			if session.IsTokenShaped(key) {
				t.Errorf("IsTokenShaped(%q) = true, want false", key)
			}
		}
	})
}
