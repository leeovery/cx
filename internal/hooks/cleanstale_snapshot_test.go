package hooks_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"slices"
	"testing"

	"github.com/leeovery/portal/internal/transienttest"
)

// The snapshot is a caller's older view of the file. It exists to keep a key
// the caller never saw out of the delete set, because such a key was written
// after the live enumeration the caller derived its token set from — so that
// enumeration never had the chance to protect it.
func TestCleanStaleSnapshotNarrowing(t *testing.T) {
	liveKey := transienttest.ReapableHookKey(0)
	staleKey := transienttest.ReapableHookKey(1)
	lateKey := transienttest.ReapableHookKey(2)
	unjudgeableKey := transienttest.UnjudgeableHookKey(0)

	t.Run("it deletes a key present in the file, in the snapshot and absent from the live set", func(t *testing.T) {
		store, _ := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"live"},%q:{"on-resume":"gone"}}`, liveKey, staleKey))

		removed, err := store.CleanStale([]string{liveKey}, []string{liveKey, staleKey})
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
			t.Error("a key in the file, in the snapshot and absent from the live set survived")
		}
		if _, ok := h[liveKey]; !ok {
			t.Error("the live key was reaped")
		}
	})

	t.Run("it retains a key the snapshot did not hold", func(t *testing.T) {
		store, path := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"gone"}}`, staleKey))

		snapshot := []string{staleKey}

		// The registration that lands after the snapshot: token-shaped, absent
		// from the live set, and therefore reapable on shape alone.
		if err := store.Set(lateKey, "on-resume", "fresh", "cli"); err != nil {
			t.Fatalf("seed the late registration: %v", err)
		}

		removed, err := store.CleanStale(nil, snapshot)
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
		if _, ok := h[lateKey]; !ok {
			t.Errorf("a key the snapshot did not hold was deleted; file now holds %v (%s)", keysOf(h), string(readFileBytes(t, path)))
		}
	})

	t.Run("it derives the delete set from the file under the lock, not from the snapshot", func(t *testing.T) {
		store, path := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"gone"}}`, staleKey))

		// The snapshot describes a key another writer has since removed.
		snapshot := []string{staleKey, lateKey}

		removed, err := store.CleanStale(nil, snapshot)
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if !slices.Equal(removed, []string{staleKey}) {
			t.Fatalf("CleanStale removed %v, want only [%s] — a key no longer in the file was reported as removed", removed, staleKey)
		}

		h, err := store.Load("internal")
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if len(h) != 0 {
			t.Errorf("file holds %v, want nothing (%s)", keysOf(h), string(readFileBytes(t, path)))
		}
	})

	t.Run("it still retains a non-token-shaped key", func(t *testing.T) {
		store, path := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"old"}}`, unjudgeableKey))
		before := string(readFileBytes(t, path))

		removed, err := store.CleanStale(nil, []string{unjudgeableKey})
		if err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if len(removed) != 0 {
			t.Fatalf("CleanStale removed %v, want nothing", removed)
		}
		if after := string(readFileBytes(t, path)); after != before {
			t.Errorf("hooks.json changed:\n before %s\n after  %s", before, after)
		}
	})

	t.Run("it still deletes an empty key present in both the file and the snapshot", func(t *testing.T) {
		store, _ := seedHooksFile(t, fmt.Sprintf(`{"":{"on-resume":"malformed"},%q:{"on-resume":"live"}}`, liveKey))

		removed, err := store.CleanStale([]string{liveKey}, []string{"", liveKey})
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
			t.Error("the empty key survived")
		}
	})
}

func keysOf(h map[string]map[string]string) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// snapshotAll is a caller's pre-read of the whole file: the key set a sweep
// holds when it reaches CleanStale having found nothing else to narrow it by.
// It reads the file directly rather than through the store, so staging a
// snapshot leaves no record in a test that is asserting on the store's own.
func snapshotAll(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		t.Fatalf("read %s: %v", path, err)
	}
	var h map[string]map[string]string
	if err := json.Unmarshal(data, &h); err != nil {
		t.Fatalf("decode %s: %v", path, err)
	}
	return keysOf(h)
}
