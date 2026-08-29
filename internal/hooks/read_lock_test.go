package hooks_test

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/transienttest"
)

// seedReadFixture stages hooks.json holding entries keys plus the sidecar
// beside it, so a read has both a file to return and a lock to contend for.
func seedReadFixture(t *testing.T, entries int) (*hooks.Store, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "hooks.json")
	pairs := make([]string, 0, entries)
	for i := range entries {
		pairs = append(pairs, fmt.Sprintf(`%q:{"on-resume":"cmd%02d"}`, fmt.Sprintf("k%02d", i), i))
	}
	body := "{" + strings.Join(pairs, ",") + "}"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("seed hooks.json: %v", err)
	}
	transienttest.CreateHooksSidecar(t, path)
	return hooks.NewStore(path), path
}

func TestReadSharedLock(t *testing.T) {
	t.Run("it takes a shared lock for a read", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 2*time.Second)
		store, path := seedReadFixture(t, 2)
		transienttest.HoldHooksSidecarShared(t, path)

		sink := installCapture(t)
		start := time.Now()
		h, err := store.Load("cli")
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(h) != 2 {
			t.Fatalf("got %d entries, want 2", len(h))
		}
		if elapsed >= 500*time.Millisecond {
			t.Fatalf("Load took %v against another shared holder — it acquired exclusively", elapsed)
		}
		if got := transienttest.UnlockedRecords(t, sink); len(got) != 0 {
			t.Fatalf("read degraded despite a grantable shared lock: %+v", got)
		}
	})

	t.Run("it releases the shared lock before it returns", func(t *testing.T) {
		// Repeated reads would each leak a held fd, so an exclusive acquire
		// afterwards is granted only if every one of them released.
		hooks.SetLockTimeoutForTest(t, 2*time.Second)
		store, path := seedReadFixture(t, 1)

		for range 3 {
			if _, err := store.Load("cli"); err != nil {
				t.Fatalf("Load: %v", err)
			}
		}
		if _, err := store.List("cli"); err != nil {
			t.Fatalf("List: %v", err)
		}
		if _, err := store.Get("k00", "cli"); err != nil {
			t.Fatalf("Get: %v", err)
		}
		if _, err := store.CleanStale(abortingEnumeration(nil)); !errors.Is(err, errAbortEnumeration) {
			t.Fatalf("CleanStale snapshot read: %v", err)
		}
		if _, _, err := hooks.LookupOnResume(store, "k00"); err != nil {
			t.Fatalf("LookupOnResume: %v", err)
		}

		assertSidecarFree(t, path)

		// A mutation follows without contending with a read that already
		// returned — the sweep's pre-read must not make its own CleanStale wait.
		if err := store.Set("k99", "on-resume", "cmd99", "cli"); err != nil {
			t.Fatalf("mutation after reads: %v", err)
		}
	})

	t.Run("it lets two reads proceed concurrently", func(t *testing.T) {
		// A shared hold from a third fd is the discriminator: both reads are
		// granted alongside it, so neither can reach the (low) bound. An
		// exclusive read would block out against the hold and degrade.
		hooks.SetLockTimeoutForTest(t, 300*time.Millisecond)
		store, path := seedReadFixture(t, 4)
		transienttest.HoldHooksSidecarShared(t, path)

		sink := installCapture(t)
		var wg sync.WaitGroup
		elapsed := make([]time.Duration, 2)
		errs := make([]error, 2)
		counts := make([]int, 2)
		start := time.Now()
		for i := range 2 {
			wg.Go(func() {
				began := time.Now()
				h, err := store.Load("cli")
				elapsed[i] = time.Since(began)
				errs[i] = err
				counts[i] = len(h)
			})
		}
		wg.Wait()
		total := time.Since(start)

		for i := range 2 {
			if errs[i] != nil {
				t.Fatalf("read %d: %v", i, errs[i])
			}
			if counts[i] != 4 {
				t.Errorf("read %d saw %d entries, want 4", i, counts[i])
			}
			if elapsed[i] >= 300*time.Millisecond {
				t.Errorf("read %d took %v — it blocked to the bound", i, elapsed[i])
			}
		}
		if total >= 300*time.Millisecond {
			t.Errorf("two overlapping reads took %v — they serialised", total)
		}
		if got := transienttest.UnlockedRecords(t, sink); len(got) != 0 {
			t.Fatalf("a concurrent read degraded: %+v", got)
		}
	})

	t.Run("it reads anyway when the lock cannot be taken", func(t *testing.T) {
		bound := 60 * time.Millisecond
		hooks.SetLockTimeoutForTest(t, bound)
		store, path := seedReadFixture(t, 3)
		transienttest.HoldHooksSidecar(t, path)

		sink := installCapture(t)
		start := time.Now()
		h, err := store.Load("cli")
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("Load returned an error under a held exclusive lock: %v", err)
		}
		if len(h) != 3 {
			t.Fatalf("got %d entries, want 3 — the data must come back regardless", len(h))
		}
		if elapsed < bound {
			t.Errorf("Load returned after %v — it did not wait out the %v bound", elapsed, bound)
		}
		transienttest.AssertDegradedRead(t, sink, "cli")
	})

	t.Run("it logs one DEBUG per degraded read", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 20*time.Millisecond)
		store, path := seedReadFixture(t, 42)
		transienttest.HoldHooksSidecar(t, path)

		sink := installCapture(t)
		h, err := store.Load("cli")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(h) != 42 {
			t.Fatalf("got %d entries, want 42", len(h))
		}
		transienttest.AssertDegradedRead(t, sink, "cli")
	})

	t.Run("it degrades when the sidecar is absent", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 20*time.Millisecond)
		path := filepath.Join(t.TempDir(), "hooks.json")
		if err := os.WriteFile(path, []byte(`{"k0":{"on-resume":"cmd0"}}`), 0o600); err != nil {
			t.Fatalf("seed hooks.json: %v", err)
		}
		store := hooks.NewStore(path)

		sink := installCapture(t)
		h, err := store.Load("cli")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if h["k0"]["on-resume"] != "cmd0" {
			t.Fatalf("entries did not come back: %v", h)
		}
		if _, statErr := os.Stat(path + ".lock"); !os.IsNotExist(statErr) {
			t.Fatalf("the degraded read created the sidecar: %v", statErr)
		}
		transienttest.AssertDegradedRead(t, sink, "cli")
	})

	t.Run("it creates nothing when it reads", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 20*time.Millisecond)
		dir := filepath.Join(t.TempDir(), "absent")
		path := filepath.Join(dir, "hooks.json")

		h, err := hooks.NewStore(path).Load("cli")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(h) != 0 {
			t.Fatalf("got %d entries, want an empty map", len(h))
		}

		for _, p := range []string{dir, path, path + ".lock"} {
			if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
				t.Errorf("a read created %s: %v", p, statErr)
			}
		}
	})

	t.Run("it emits no load-unlocked record from inside a mutation", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "hooks.json")
		store := hooks.NewStore(path)

		sink := installCapture(t)
		if err := store.Set("k0", "on-resume", "cmd0", "cli"); err != nil {
			t.Fatalf("Set: %v", err)
		}
		if got := transienttest.UnlockedRecords(t, sink); len(got) != 0 {
			t.Fatalf("a mutation's non-locking load emitted %+v — it is exclusive, not degraded", got)
		}
	})
}

func TestReadSharedLockBoundSelection(t *testing.T) {
	t.Run("it degrades the sweep pre-read at the short bound", func(t *testing.T) {
		short := 60 * time.Millisecond
		hooks.SetSnapshotLockTimeoutForTest(t, short)
		hooks.SetLockTimeoutForTest(t, 5*time.Second)
		store, path := seedReadFixture(t, 2)
		transienttest.HoldHooksSidecar(t, path)

		sink := installCapture(t)
		var entries int
		start := time.Now()
		// Aborted at the enumeration, so the elapsed time measures the snapshot
		// read alone rather than the exclusive acquire that would follow it
		// against the same held sidecar.
		_, err := store.CleanStale(abortingEnumeration(&entries))
		elapsed := time.Since(start)

		if !errors.Is(err, errAbortEnumeration) {
			t.Fatalf("CleanStale: %v", err)
		}
		if entries != 2 {
			t.Fatalf("the snapshot held %d entries, want 2", entries)
		}
		if elapsed < short {
			t.Errorf("the snapshot read returned after %v — it did not wait out the %v short bound", elapsed, short)
		}
		if elapsed >= time.Second {
			t.Fatalf("the snapshot read took %v — it waited at lockTimeout, not snapshotLockTimeout", elapsed)
		}
		transienttest.AssertDegradedRead(t, sink, "internal")
	})

	t.Run("it takes the bound from the parameter, not from via", func(t *testing.T) {
		// via=internal identifies the sweep's pre-read uniquely today, so a
		// via-driven branch would pass every other case here. An ordinary read
		// carrying that same value must still wait at lockTimeout.
		short := time.Millisecond
		hooks.SetSnapshotLockTimeoutForTest(t, short)
		bound := 120 * time.Millisecond
		hooks.SetLockTimeoutForTest(t, bound)
		store, path := seedReadFixture(t, 1)
		transienttest.HoldHooksSidecar(t, path)

		start := time.Now()
		if _, err := store.Load("internal"); err != nil {
			t.Fatalf("Load: %v", err)
		}
		if elapsed := time.Since(start); elapsed < bound {
			t.Errorf("Load(\"internal\") returned after %v — the bound was chosen from via, not from the parameter", elapsed)
		}
	})

	t.Run("it waits at the ordinary bound for every other read", func(t *testing.T) {
		hooks.SetSnapshotLockTimeoutForTest(t, time.Millisecond)
		bound := 120 * time.Millisecond
		hooks.SetLockTimeoutForTest(t, bound)
		store, path := seedReadFixture(t, 1)
		transienttest.HoldHooksSidecar(t, path)

		reads := map[string]func() error{
			"Load": func() error { _, err := store.Load("cli"); return err },
			"List": func() error { _, err := store.List("cli"); return err },
			"Get":  func() error { _, err := store.Get("k00", "cli"); return err },
			"LookupOnResume": func() error {
				_, _, err := hooks.LookupOnResume(store, "k00")
				return err
			},
		}
		for name, read := range reads {
			t.Run(name, func(t *testing.T) {
				start := time.Now()
				if err := read(); err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if elapsed := time.Since(start); elapsed < bound {
					t.Errorf("%s returned after %v — it did not wait at lockTimeout (%v)", name, elapsed, bound)
				}
			})
		}
	})
}

func TestReadSharedLockVia(t *testing.T) {
	// Every read names its caller in the degradation breadcrumb, and the
	// in-package hydrate read names itself without a caller-supplied value.
	cases := []struct {
		name string
		via  string
		read func(*hooks.Store) error
	}{
		{"Load", "cli", func(s *hooks.Store) error { _, err := s.Load("cli"); return err }},
		{"List", "doctor", func(s *hooks.Store) error { _, err := s.List("doctor"); return err }},
		{"Get", "internal", func(s *hooks.Store) error { _, err := s.Get("k00", "internal"); return err }},
		{"CleanStale snapshot", "internal", func(s *hooks.Store) error {
			if _, err := s.CleanStale(abortingEnumeration(nil)); !errors.Is(err, errAbortEnumeration) {
				return err
			}
			return nil
		}},
		{"LookupOnResume", "hydrate", func(s *hooks.Store) error {
			_, _, err := hooks.LookupOnResume(s, "k00")
			return err
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			hooks.SetLockTimeoutForTest(t, 20*time.Millisecond)
			hooks.SetSnapshotLockTimeoutForTest(t, 20*time.Millisecond)
			store, path := seedReadFixture(t, 1)
			transienttest.HoldHooksSidecar(t, path)

			sink := installCapture(t)
			if err := tc.read(store); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			transienttest.AssertDegradedRead(t, sink, tc.via)
		})
	}
}

func TestLookupOnResumeUnderHeldLock(t *testing.T) {
	t.Run("it returns the hook for a hydrating pane while a mutation holds the lock", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)
		path := filepath.Join(t.TempDir(), "hooks.json")
		if err := os.WriteFile(path, []byte(`{"tok01":{"on-resume":"claude --resume abc"}}`), 0o600); err != nil {
			t.Fatalf("seed hooks.json: %v", err)
		}
		transienttest.CreateHooksSidecar(t, path)
		transienttest.HoldHooksSidecar(t, path)

		sink := installCapture(t)
		cmd, ok, err := hooks.LookupOnResume(hooks.NewStore(path), "tok01")
		if err != nil {
			t.Fatalf("LookupOnResume returned an error under a held lock: %v", err)
		}
		if !ok || cmd != "claude --resume abc" {
			t.Fatalf("got (%q, %v), want the registered command — a busy lock must not drop a pane to a bare shell", cmd, ok)
		}
		transienttest.AssertDegradedRead(t, sink, "hydrate")
	})

	t.Run("an empty key takes no lock and logs nothing", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, time.Second)
		store, path := seedReadFixture(t, 1)
		transienttest.HoldHooksSidecar(t, path)

		sink := installCapture(t)
		start := time.Now()
		cmd, ok, err := hooks.LookupOnResume(store, "")
		elapsed := time.Since(start)

		if err != nil || ok || cmd != "" {
			t.Fatalf("got (%q, %v, %v), want the empty-key early return", cmd, ok, err)
		}
		if elapsed >= 100*time.Millisecond {
			t.Errorf("empty-key lookup took %v — it reached the acquire", elapsed)
		}
		if got := transienttest.UnlockedRecords(t, sink); len(got) != 0 {
			t.Fatalf("empty-key lookup emitted %+v", got)
		}
	})
}

// errAbortEnumeration ends a clean at its enumeration, so a fixture measures
// the snapshot read that precedes it and nothing after.
var errAbortEnumeration = errors.New("abort the enumeration")

// abortingEnumeration records the snapshot's size, when entries is non-nil, and
// aborts.
func abortingEnumeration(entries *int) func(hooks.Snapshot) ([]string, error) {
	return func(snapshot hooks.Snapshot) ([]string, error) {
		if entries != nil {
			*entries = len(snapshot)
		}
		return nil, errAbortEnumeration
	}
}
