package hookstest_test

import (
	"testing"

	"github.com/leeovery/portal/internal/hookstest"
	"github.com/leeovery/portal/internal/nanoid"
)

func TestHookKeySeedVocabulary(t *testing.T) {
	t.Run("a reapable key is token-shaped", func(t *testing.T) {
		for n := range 4 {
			if got := hookstest.ReapableHookKey(n); !nanoid.IsTokenShaped(got) {
				t.Errorf("ReapableHookKey(%d) = %q, want a token-shaped key", n, got)
			}
		}
	})

	t.Run("it authors a reapable key at the pane-token width", func(t *testing.T) {
		token, err := nanoid.NewPaneTokenGenerator()()
		if err != nil {
			t.Fatalf("mint a pane token: %v", err)
		}
		for n := range 4 {
			if got := hookstest.ReapableHookKey(n); len(got) != len(token) {
				t.Errorf("ReapableHookKey(%d) = %q (%d bytes), want the pane-token width of %d", n, got, len(got), len(token))
			}
		}
	})

	t.Run("an unjudgeable key is not token-shaped", func(t *testing.T) {
		for n := range 4 {
			if got := hookstest.UnjudgeableHookKey(n); nanoid.IsTokenShaped(got) {
				t.Errorf("UnjudgeableHookKey(%d) = %q, want a key the staleness rule cannot judge", n, got)
			}
		}
	})

	t.Run("successive n give distinct keys", func(t *testing.T) {
		seen := map[string]int{}
		for n := range 8 {
			for _, got := range []string{hookstest.ReapableHookKey(n), hookstest.UnjudgeableHookKey(n)} {
				if prev, dup := seen[got]; dup {
					t.Errorf("key %q produced for both n=%d and n=%d", got, prev, n)
				}
				seen[got] = n
			}
		}
	})
}
