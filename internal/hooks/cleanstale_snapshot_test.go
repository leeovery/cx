package hooks_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/transienttest"
)

// enumerating answers tokens as the live set whatever the snapshot holds: the
// ordinary case, where nothing lands between the two reads.
func enumerating(tokens ...string) func(hooks.Snapshot) ([]string, error) {
	return func(hooks.Snapshot) ([]string, error) { return tokens, nil }
}

// The snapshot is the file as CleanStale found it before the enumeration ran.
// It exists to keep a key the enumeration never saw out of the delete set,
// because such a key was written after the live set was read — so that read
// never had the chance to protect it.
func TestCleanStaleSnapshotNarrowing(t *testing.T) {
	liveKey := transienttest.ReapableHookKey(0)
	staleKey := transienttest.ReapableHookKey(1)
	lateKey := transienttest.ReapableHookKey(2)
	unjudgeableKey := transienttest.UnjudgeableHookKey(0)

	t.Run("it deletes a key present in the file, in the snapshot and absent from the live set", func(t *testing.T) {
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
			t.Error("a key in the file, in the snapshot and absent from the live set survived")
		}
		if _, ok := h[liveKey]; !ok {
			t.Error("the live key was reaped")
		}
	})

	t.Run("it retains a key written after the snapshot", func(t *testing.T) {
		store, path := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"gone"}}`, staleKey))

		// The registration that lands while the enumeration runs: token-shaped,
		// absent from the live set, and therefore reapable on shape alone.
		removed, err := store.CleanStale(func(hooks.Snapshot) ([]string, error) {
			if err := store.Set(lateKey, "on-resume", "fresh", "cli"); err != nil {
				return nil, fmt.Errorf("seed the late registration: %w", err)
			}
			return nil, nil
		})
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

	t.Run("it hands the enumeration the file as it stood before it ran", func(t *testing.T) {
		store, _ := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"gone"}}`, staleKey))

		var seen []string
		if _, err := store.CleanStale(func(snapshot hooks.Snapshot) ([]string, error) {
			if err := store.Set(lateKey, "on-resume", "fresh", "cli"); err != nil {
				return nil, fmt.Errorf("seed the late registration: %w", err)
			}
			seen = keysOf(snapshot)
			return nil, nil
		}); err != nil {
			t.Fatalf("CleanStale: %v", err)
		}

		if !slices.Equal(seen, []string{staleKey}) {
			t.Errorf("the enumeration was handed %v, want [%s] — the snapshot was read after it, not before", seen, staleKey)
		}
	})

	t.Run("it holds no lock while the enumeration runs", func(t *testing.T) {
		store, path := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"gone"}}`, staleKey))
		// A mutation stages the sidecar, which no read creates.
		if err := store.Set(liveKey, "on-resume", "live", "cli"); err != nil {
			t.Fatalf("stage the sidecar: %v", err)
		}

		probed := false
		if _, err := store.CleanStale(func(hooks.Snapshot) ([]string, error) {
			probed = true
			f, err := os.OpenFile(path+".lock", os.O_RDWR, 0o600)
			if err != nil {
				return nil, fmt.Errorf("open sidecar: %w", err)
			}
			defer func() { _ = f.Close() }()
			if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
				return nil, fmt.Errorf("sidecar is held during the enumeration: %w", err)
			}
			return []string{liveKey}, unix.Flock(int(f.Fd()), unix.LOCK_UN)
		}); err != nil {
			t.Fatalf("CleanStale: %v", err)
		}
		if !probed {
			t.Fatal("the enumeration never ran — the probe proves nothing about the lock")
		}
	})

	t.Run("an enumeration error aborts the clean untouched", func(t *testing.T) {
		store, path := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"gone"}}`, staleKey))
		// A mutation stages the sidecar, so the clean's own pre-read has a lock
		// to take and nothing it says can be mistaken for the deletion's lines.
		if err := store.Set(liveKey, "on-resume", "live", "cli"); err != nil {
			t.Fatalf("stage the sidecar: %v", err)
		}
		before := string(readFileBytes(t, path))

		sentinel := errors.New("nothing to enumerate")
		sink := installCapture(t)

		removed, err := store.CleanStale(func(hooks.Snapshot) ([]string, error) { return nil, sentinel })
		if !errors.Is(err, sentinel) {
			t.Fatalf("CleanStale error = %v, want the enumeration's own error", err)
		}
		if len(removed) != 0 {
			t.Errorf("removed = %v, want none", removed)
		}
		if after := string(readFileBytes(t, path)); after != before {
			t.Errorf("hooks.json changed:\n before %s\n after  %s", before, after)
		}
		if recs := sink.Records(); len(recs) != 0 {
			t.Errorf("an aborted clean emitted %d records, want 0: %+v", len(recs), recs)
		}
	})

	t.Run("it derives the delete set from the file under the lock, not from the snapshot", func(t *testing.T) {
		store, path := seedHooksFile(t, fmt.Sprintf(`{%q:{"on-resume":"gone"},%q:{"on-resume":"also-gone"}}`, staleKey, lateKey))

		// Another writer removes a key the snapshot holds and the clean would
		// otherwise have reaped, so it must not be named as removed.
		removed, err := store.CleanStale(func(hooks.Snapshot) ([]string, error) {
			if _, err := store.Remove(lateKey, "on-resume", "cli"); err != nil {
				return nil, fmt.Errorf("remove during the enumeration: %w", err)
			}
			return nil, nil
		})
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

		removed, err := store.CleanStale(enumerating())
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
			t.Error("the empty key survived")
		}
	})

	t.Run("an unreadable file aborts before the enumeration", func(t *testing.T) {
		// A directory at the hooks.json path is what makes the read fail:
		// malformed JSON decodes to an empty map instead of erroring.
		path := seedHooksDirectory(t)
		enumerated := false

		removed, err := hooks.NewStore(path).CleanStale(func(hooks.Snapshot) ([]string, error) {
			enumerated = true
			return nil, nil
		})
		if !errors.Is(err, hooks.ErrSnapshotRead) {
			t.Fatalf("CleanStale error = %v, want errors.Is ErrSnapshotRead", err)
		}
		if enumerated {
			t.Error("the enumeration ran despite an unreadable file — it was never judgeable")
		}
		if len(removed) != 0 {
			t.Errorf("removed = %v, want none", removed)
		}
	})
}

// seedHooksDirectory stages a directory where hooks.json belongs, so every read
// of it fails.
func seedHooksDirectory(t *testing.T) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir bogus hooks path: %v", err)
	}
	return path
}

func keysOf(h map[string]map[string]string) []string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
