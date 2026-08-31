package hooks_test

import (
	"fmt"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/nanoid"
)

// persistedKey is a hook key exactly as it stands in a hooks.json written by a
// shipped Portal — a literal, not a value derived from the id vocabulary, so a
// change to the pane-token width fails here on the data rather than only where
// a fixture rebuilds itself around the new width.
const persistedKey = "k3f9qz"

func TestPersistedKeyAuthoredAtThePaneTokenWidth(t *testing.T) {
	t.Run("it judges a literal six-character persisted key token-shaped", func(t *testing.T) {
		if !nanoid.IsTokenShaped(persistedKey) {
			t.Errorf("IsTokenShaped(%q) = false — a key already persisted in hooks.json stopped being judgeable, so the staleness rule now retains it forever", persistedKey)
		}
	})

	t.Run("it reaps a persisted key authored at the pane-token width when the pane is gone", func(t *testing.T) {
		store, _ := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"gone"}}`, persistedKey))

		removed, err := store.CleanStale(enumerating())
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if !slices.Equal(removed, []string{persistedKey}) {
			t.Fatalf("CleanStale removed %v, want [%s]", removed, persistedKey)
		}

		h, err := store.Load(hooks.ViaInternal)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if _, ok := h[persistedKey]; ok {
			t.Errorf("a persisted key absent from the live set survived the clean; file holds %v", keysOf(h))
		}
	})
}
