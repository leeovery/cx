package transienttest_test

import (
	"testing"

	"github.com/leeovery/portal/internal/session"
	"github.com/leeovery/portal/internal/transienttest"
)

func TestHookKeySeedVocabulary(t *testing.T) {
	t.Run("a reapable key is token-shaped", func(t *testing.T) {
		for n := range 4 {
			if got := transienttest.ReapableHookKey(n); !session.IsTokenShaped(got) {
				t.Errorf("ReapableHookKey(%d) = %q, want a token-shaped key", n, got)
			}
		}
	})

	t.Run("an unjudgeable key is not token-shaped", func(t *testing.T) {
		for n := range 4 {
			if got := transienttest.UnjudgeableHookKey(n); session.IsTokenShaped(got) {
				t.Errorf("UnjudgeableHookKey(%d) = %q, want a key the staleness rule cannot judge", n, got)
			}
		}
	})

	t.Run("successive n give distinct keys", func(t *testing.T) {
		seen := map[string]int{}
		for n := range 8 {
			for _, got := range []string{transienttest.ReapableHookKey(n), transienttest.UnjudgeableHookKey(n)} {
				if prev, dup := seen[got]; dup {
					t.Errorf("key %q produced for both n=%d and n=%d", got, prev, n)
				}
				seen[got] = n
			}
		}
	})
}
