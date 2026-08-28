package session_test

import (
	"testing"

	"github.com/leeovery/portal/internal/session"
)

func TestNewPaneToken(t *testing.T) {
	t.Run("it mints a token-shaped value", func(t *testing.T) {
		for range 200 {
			token, err := session.NewPaneToken()
			if err != nil {
				t.Fatalf("NewPaneToken: %v", err)
			}
			if !session.IsTokenShaped(token) {
				t.Fatalf("NewPaneToken() = %q, which is not token-shaped", token)
			}
		}
	})

	t.Run("it mints a distinct token per call", func(t *testing.T) {
		seen := map[string]struct{}{}
		for range 200 {
			token, err := session.NewPaneToken()
			if err != nil {
				t.Fatalf("NewPaneToken: %v", err)
			}
			if _, dup := seen[token]; dup {
				t.Fatalf("NewPaneToken minted %q twice in 200 calls", token)
			}
			seen[token] = struct{}{}
		}
	})
}
