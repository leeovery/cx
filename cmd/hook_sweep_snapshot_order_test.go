package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/tmux"
)

// sideEffectPaneLister runs during, then answers with rows — so a test can
// land a concurrent writer's mutation inside the enumeration, in the window
// between the sweep's snapshot and the token set that snapshot is weighed
// against.
type sideEffectPaneLister struct {
	rows   []tmux.PaneHookRow
	during func()
}

func (l *sideEffectPaneLister) ListAllPaneHookKeys() ([]tmux.PaneHookRow, error) {
	if l.during != nil {
		l.during()
	}
	return l.rows, nil
}

func (l *sideEffectPaneLister) TryGetServerOption(string) (string, bool, error) {
	return restoringOption(false, nil)
}

// The enumeration is a tmux read outside the lock, so it is always older than
// the mutation it feeds. Taking the snapshot before it is what keeps a
// registration that lands in that window out of the delete set.
func TestHookSweepSnapshotPrecedesEnumeration(t *testing.T) {
	t.Run("it retains an entry written during the pane enumeration", func(t *testing.T) {
		store, path := newTempHooksStore(t, fmt.Sprintf(`{%q: {"on-resume": "cmd-live"}}`, liveSeedA))

		lister := &sideEffectPaneLister{rows: tokenRows(liveSeedA), during: func() {
			if err := store.Set(reapableSeedA, "on-resume", "cmd-fresh", "cli"); err != nil {
				t.Errorf("register a hook during the enumeration: %v", err)
			}
		}}

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}

		postRun, err := store.Load("internal")
		if err != nil {
			t.Fatalf("store.Load post-run: %v", err)
		}
		if _, ok := postRun[reapableSeedA]; !ok {
			t.Errorf("an entry registered during the enumeration was reaped; file holds %v (%s)",
				keysOf(postRun), readFileBytes(t, path))
		}
		if _, ok := postRun[liveSeedA]; !ok {
			t.Errorf("the live entry was reaped; file holds %v", keysOf(postRun))
		}
	})

	t.Run("it holds no lock while enumerating", func(t *testing.T) {
		store, path := newTempHooksStore(t, fmt.Sprintf(`{%q: {"on-resume": "cmd-live"}}`, liveSeedA))

		probed := false
		lister := &sideEffectPaneLister{rows: tokenRows(liveSeedA), during: func() {
			probed = true
			f, err := os.OpenFile(path+".lock", os.O_RDWR, 0o600)
			if err != nil {
				t.Errorf("open sidecar during the enumeration: %v", err)
				return
			}
			defer func() { _ = f.Close() }()
			if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
				t.Errorf("sidecar is held during the enumeration: %v — a tmux read must sit outside the lock", err)
				return
			}
			_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		}}

		if err := runHookStaleCleanup(lister, store, nil, nil, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}
		if !probed {
			t.Fatal("the enumeration never ran — the probe proves nothing about the lock")
		}
	})

	t.Run("it feeds onRemoved exactly what was deleted", func(t *testing.T) {
		seed := fmt.Sprintf(`{
  %q: {"on-resume": "cmd-live"},
  %q: {"on-resume": "cmd-b"},
  %q: {"on-resume": "cmd-c"}
}`, liveSeedA, reapableSeedB, reapableSeedC)
		store, path := newTempHooksStore(t, seed)

		// Both mid-cycle windows at once, so the callback is measured against
		// what this sweep deleted rather than against a file delta the two
		// happen to agree on: another writer removes seed C — a key the
		// snapshot holds and the sweep would otherwise have reaped — while a
		// registration lands for seed D. C leaves the file by someone else's
		// hand and D never enters the snapshot, so neither may be named.
		lister := &sideEffectPaneLister{rows: tokenRows(liveSeedA), during: func() {
			if _, err := store.Remove(reapableSeedC, "on-resume", "cli"); err != nil {
				t.Errorf("remove a hook during the enumeration: %v", err)
			}
			if err := store.Set(reapableSeedD, "on-resume", "cmd-fresh", "cli"); err != nil {
				t.Errorf("register a hook during the enumeration: %v", err)
			}
		}}

		var reported []string
		if err := runHookStaleCleanup(lister, store, nil, func(key string) { reported = append(reported, key) }, nil); err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}
		slices.Sort(reported)

		want := []string{reapableSeedB}
		if !slices.Equal(reported, want) {
			t.Errorf("onRemoved reported %v, want only %v — the callback names this sweep's deletions, not a prediction over the file (file holds %s)",
				reported, want, readFileBytes(t, path))
		}
	})
}

// After the sweep's exclusive hold arrived, reaching CleanStale stopped being
// free: it creates the config directory and the sidecar. An install that has
// never registered a hook must not pay that every cycle.
func TestHookSweepTakesNoLockWithNothingPersisted(t *testing.T) {
	configRoot := t.TempDir()
	configDir := filepath.Join(configRoot, "portal")
	hooksPath := filepath.Join(configDir, "hooks.json")

	lister := &stubAllPaneLister{rows: tokenRows(liveSeedA)}
	if err := runHookStaleCleanup(lister, hooks.NewStore(hooksPath), nil, nil, nil); err != nil {
		t.Fatalf("runHookStaleCleanup: %v", err)
	}

	if entries := dirListing(t, configRoot); len(entries) != 0 {
		t.Errorf("the sweep created %v under a config root holding no hooks.json", entries)
	}
	if _, err := os.Stat(hooksPath + ".lock"); err == nil {
		t.Error("the sweep created the sidecar with nothing persisted")
	}
}
