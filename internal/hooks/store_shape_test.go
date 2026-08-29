package hooks_test

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/transienttest"
)

// seedHooksFile writes the raw on-disk map so a fixture can hold an arbitrary
// key, including the empty one.
func seedHooksFile(t *testing.T, contents string) (*hooks.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("seed hooks file: %v", err)
	}
	return hooks.NewStore(path), path
}

func TestCleanStaleShapeAwareness(t *testing.T) {
	liveKey := transienttest.ReapableHookKey(0)
	staleKey := transienttest.ReapableHookKey(1)
	retainedKey := transienttest.UnjudgeableHookKey(0)

	t.Run("it retains a non-token-shaped key absent from the live set", func(t *testing.T) {
		store, path := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"old"},%q:{"on-resume":"live"}}`, retainedKey, liveKey))

		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read seeded file: %v", err)
		}

		removed, err := store.CleanStale(enumerating(liveKey))
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if len(removed) != 0 {
			t.Fatalf("CleanStale removed %v, want nothing", removed)
		}

		persisted, err := store.Load("internal")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		predicted := hooks.StaleKeys(persisted, []string{liveKey})
		if len(predicted) != 0 {
			t.Errorf("StaleKeys reported %v, want nothing", predicted)
		}

		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file after clean: %v", err)
		}
		if string(after) != string(before) {
			t.Errorf("hooks.json changed:\n before %s\n after  %s", before, after)
		}
	})

	t.Run("it deletes a token-shaped key absent from the live set", func(t *testing.T) {
		store, _ := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"live"},%q:{"on-resume":"gone"}}`, liveKey, staleKey))

		removed, err := store.CleanStale(enumerating(liveKey))
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if !slices.Equal(removed, []string{staleKey}) {
			t.Fatalf("CleanStale removed %v, want [%s]", removed, staleKey)
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if _, ok := h[staleKey]; ok {
			t.Error("token-shaped absent key should have been removed")
		}
		if _, ok := h[liveKey]; !ok {
			t.Error("token-shaped live key should have been kept")
		}
	})

	t.Run("it deletes an empty key", func(t *testing.T) {
		store, _ := seedHooksFile(t, fmt.Sprintf(`{"":{"on-resume":"malformed"},%q:{"on-resume":"live"}}`, liveKey))

		removed, err := store.CleanStale(enumerating(liveKey))
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if !slices.Equal(removed, []string{""}) {
			t.Fatalf("CleanStale removed %v, want the empty key", removed)
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if _, ok := h[""]; ok {
			t.Error("empty key should have been removed")
		}
	})

	t.Run("it writes no file and emits no summary when every candidate is retained", func(t *testing.T) {
		store, path := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"a"},%q:{"on-resume":"b"}}`, transienttest.UnjudgeableHookKey(1), transienttest.UnjudgeableHookKey(2)))

		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read seeded file: %v", err)
		}

		sink := installCapture(t)
		removed, err := store.CleanStale(enumerating())
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if len(removed) != 0 {
			t.Fatalf("CleanStale removed %v, want nothing", removed)
		}

		for _, r := range sink.Records() {
			if r.Msg == "clean-stale" {
				t.Errorf("unexpected clean-stale record: %+v", r)
			}
		}

		after, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read file after clean: %v", err)
		}
		if string(after) != string(before) {
			t.Errorf("hooks.json changed:\n before %s\n after  %s", before, after)
		}
	})
}
