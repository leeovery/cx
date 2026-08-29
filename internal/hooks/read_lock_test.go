package hooks_test

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/logtest"
)

// holdSidecarShared takes the sidecar LOCK_SH from an independent open file
// description, modelling another reader. An exclusive acquire blocks against it
// to the bound; a shared one is granted immediately — which is what makes it
// the discriminator between the two read modes.
func holdSidecarShared(t *testing.T, hooksPath string) {
	t.Helper()
	f, err := os.OpenFile(hooksPath+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatalf("open sidecar: %v", err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		t.Fatalf("flock sidecar shared: %v", err)
	}
	t.Cleanup(func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	})
}

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
	createSidecar(t, path)
	return hooks.NewStore(path), path
}

// unlockedRecords returns the degradation breadcrumbs the sink captured.
func unlockedRecords(t *testing.T, sink *logtest.Sink) []logtest.Record {
	t.Helper()
	var out []logtest.Record
	for _, r := range sink.Records() {
		if r.Msg == "load-unlocked" {
			out = append(out, r)
		}
	}
	return out
}

// assertOneDegradedRecord pins the whole shape of a degraded read's single
// breadcrumb: DEBUG, op=load-unlocked, the lock error carried, and via naming
// the caller.
func assertOneDegradedRecord(t *testing.T, sink *logtest.Sink, wantVia string) {
	t.Helper()
	got := unlockedRecords(t, sink)
	if len(got) != 1 {
		t.Fatalf("got %d load-unlocked records, want exactly 1: %+v", len(got), got)
	}
	r := got[0]
	if r.Level != slog.LevelDebug {
		t.Errorf("level = %v, want DEBUG", r.Level)
	}
	if op := r.AttrString(t, "op"); op != "load-unlocked" {
		t.Errorf("op = %q, want load-unlocked", op)
	}
	if via := r.AttrString(t, "via"); via != wantVia {
		t.Errorf("via = %q, want %q", via, wantVia)
	}
	if errAttr := r.AttrString(t, "error"); errAttr == "" {
		t.Error("error attr is empty — the lock failure must be carried")
	}
}

func TestReadSharedLock(t *testing.T) {
	t.Run("it takes a shared lock for a read", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 2*time.Second)
		store, path := seedReadFixture(t, 2)
		holdSidecarShared(t, path)

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
		if got := unlockedRecords(t, sink); len(got) != 0 {
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
		if _, err := store.LoadSnapshot("internal"); err != nil {
			t.Fatalf("LoadSnapshot: %v", err)
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
		holdSidecarShared(t, path)

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
		if got := unlockedRecords(t, sink); len(got) != 0 {
			t.Fatalf("a concurrent read degraded: %+v", got)
		}
	})

	t.Run("it reads anyway when the lock cannot be taken", func(t *testing.T) {
		bound := 60 * time.Millisecond
		hooks.SetLockTimeoutForTest(t, bound)
		store, path := seedReadFixture(t, 3)
		holdSidecar(t, path)

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
		assertOneDegradedRecord(t, sink, "cli")
	})

	t.Run("it logs one DEBUG per degraded read", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 20*time.Millisecond)
		store, path := seedReadFixture(t, 42)
		holdSidecar(t, path)

		sink := installCapture(t)
		h, err := store.Load("cli")
		if err != nil {
			t.Fatalf("Load: %v", err)
		}
		if len(h) != 42 {
			t.Fatalf("got %d entries, want 42", len(h))
		}
		assertOneDegradedRecord(t, sink, "cli")
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
		assertOneDegradedRecord(t, sink, "cli")
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
		if got := unlockedRecords(t, sink); len(got) != 0 {
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
		holdSidecar(t, path)

		sink := installCapture(t)
		start := time.Now()
		h, err := store.LoadSnapshot("internal")
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("LoadSnapshot: %v", err)
		}
		if len(h) != 2 {
			t.Fatalf("got %d entries, want 2", len(h))
		}
		if elapsed < short {
			t.Errorf("LoadSnapshot returned after %v — it did not wait out the %v short bound", elapsed, short)
		}
		if elapsed >= time.Second {
			t.Fatalf("LoadSnapshot took %v — it waited at lockTimeout, not snapshotLockTimeout", elapsed)
		}
		assertOneDegradedRecord(t, sink, "internal")
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
		holdSidecar(t, path)

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
		holdSidecar(t, path)

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
		{"LoadSnapshot", "internal", func(s *hooks.Store) error { _, err := s.LoadSnapshot("internal"); return err }},
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
			holdSidecar(t, path)

			sink := installCapture(t)
			if err := tc.read(store); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			assertOneDegradedRecord(t, sink, tc.via)
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
		createSidecar(t, path)
		holdSidecar(t, path)

		sink := installCapture(t)
		cmd, ok, err := hooks.LookupOnResume(hooks.NewStore(path), "tok01")
		if err != nil {
			t.Fatalf("LookupOnResume returned an error under a held lock: %v", err)
		}
		if !ok || cmd != "claude --resume abc" {
			t.Fatalf("got (%q, %v), want the registered command — a busy lock must not drop a pane to a bare shell", cmd, ok)
		}
		assertOneDegradedRecord(t, sink, "hydrate")
	})

	t.Run("an empty key takes no lock and logs nothing", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, time.Second)
		store, path := seedReadFixture(t, 1)
		holdSidecar(t, path)

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
		if got := unlockedRecords(t, sink); len(got) != 0 {
			t.Fatalf("empty-key lookup emitted %+v", got)
		}
	})
}
