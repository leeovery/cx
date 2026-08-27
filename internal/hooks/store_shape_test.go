package hooks_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/hooks"
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
	t.Run("it retains a non-token-shaped key absent from the live set", func(t *testing.T) {
		store, path := seedHooksFile(t, `{"my-session:0.1":{"on-resume":"old"},"aBc123":{"on-resume":"live"}}`)

		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read seeded file: %v", err)
		}

		removed, err := store.CleanStale([]string{"aBc123"})
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if len(removed) != 0 {
			t.Fatalf("CleanStale removed %v, want nothing", removed)
		}

		persisted, err := store.Load()
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		predicted := hooks.StaleKeys(persisted, []string{"aBc123"})
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
		store, _ := seedHooksFile(t, `{"aBc123":{"on-resume":"live"},"Zx9Q0p":{"on-resume":"gone"}}`)

		removed, err := store.CleanStale([]string{"aBc123"})
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if !slices.Equal(removed, []string{"Zx9Q0p"}) {
			t.Fatalf("CleanStale removed %v, want [Zx9Q0p]", removed)
		}

		h, err := store.Load()
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if _, ok := h["Zx9Q0p"]; ok {
			t.Error("token-shaped absent key should have been removed")
		}
		if _, ok := h["aBc123"]; !ok {
			t.Error("token-shaped live key should have been kept")
		}
	})

	t.Run("it deletes an empty key", func(t *testing.T) {
		store, _ := seedHooksFile(t, `{"":{"on-resume":"malformed"},"aBc123":{"on-resume":"live"}}`)

		removed, err := store.CleanStale([]string{"aBc123"})
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if !slices.Equal(removed, []string{""}) {
			t.Fatalf("CleanStale removed %v, want the empty key", removed)
		}

		h, err := store.Load()
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if _, ok := h[""]; ok {
			t.Error("empty key should have been removed")
		}
	})

	t.Run("it writes no file and emits no summary when every candidate is retained", func(t *testing.T) {
		store, path := seedHooksFile(t, `{"my-session:0.1":{"on-resume":"a"},"other:1.2":{"on-resume":"b"}}`)

		before, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read seeded file: %v", err)
		}

		sink := installCapture(t)
		removed, err := store.CleanStale(nil)
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
