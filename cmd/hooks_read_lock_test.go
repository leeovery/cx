package cmd

import (
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"golang.org/x/sys/unix"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/tmux"
)

// holdHooksSidecar takes the sidecar exclusively from an independent open file
// description, modelling a writer in another process: every read taken while it
// is held must degrade rather than fail, and every mutation must time out at the
// bound and write nothing. The returned release lets a caller free the lock
// mid-test and retry the operation that could not take it.
func holdHooksSidecar(t *testing.T, hooksPath string) func() {
	t.Helper()
	f, err := os.OpenFile(hooksPath+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatalf("open sidecar: %v", err)
	}
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatalf("flock sidecar: %v", err)
	}
	var once sync.Once
	release := func() {
		once.Do(func() {
			_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
			_ = f.Close()
		})
	}
	t.Cleanup(release)
	return release
}

func installHooksSink(t *testing.T) *logtest.Sink {
	t.Helper()
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	return sink
}

// assertDegradedReadVia pins the single breadcrumb a degraded read leaves and
// the caller it names.
func assertDegradedReadVia(t *testing.T, sink *logtest.Sink, wantVia string) {
	t.Helper()
	var got []logtest.Record
	for _, r := range sink.Records() {
		if r.Msg == "load-unlocked" {
			got = append(got, r)
		}
	}
	if len(got) != 1 {
		t.Fatalf("got %d load-unlocked records, want exactly 1: %+v", len(got), got)
	}
	if got[0].Level != slog.LevelDebug {
		t.Errorf("level = %v, want DEBUG", got[0].Level)
	}
	if op := got[0].AttrString(t, "op"); op != "load-unlocked" {
		t.Errorf("op = %q, want load-unlocked", op)
	}
	if via := got[0].AttrString(t, "via"); via != wantVia {
		t.Errorf("via = %q, want %q", via, wantVia)
	}
	if got[0].AttrString(t, "error") == "" {
		t.Error("error attr is empty — the lock failure must be carried")
	}
}

func TestDoctorStaleHooksDegradedRead(t *testing.T) {
	t.Run("it keeps the stale-hooks check green under a degraded read", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)
		lister := fakeHookLister{rows: tokenRows(liveSeedA)}

		// The baseline must be a genuinely *locked* read: seedHooksJSON stages no
		// sidecar, so without this the baseline would itself degrade on ENOENT and
		// the comparison below would be degraded-against-degraded.
		unlockedStore, unlockedPath := seedHooksJSON(t, liveSeedA)
		if err := os.WriteFile(unlockedPath+".lock", nil, 0o600); err != nil {
			t.Fatalf("create sidecar: %v", err)
		}
		baseline, err := runDoctorDiagnosis(staleDeps(t.TempDir(), lister, unlockedStore, nil))
		if err != nil {
			t.Fatalf("runDoctorDiagnosis: %v", err)
		}
		want := findCheck(t, baseline, "stale hooks")
		wantUnhealthy := doctorUnhealthy(baseline)

		heldStore, heldPath := seedHooksJSON(t, liveSeedA)
		holdHooksSidecar(t, heldPath)

		sink := installHooksSink(t)
		degraded, err := runDoctorDiagnosis(staleDeps(t.TempDir(), lister, heldStore, nil))
		if err != nil {
			t.Fatalf("runDoctorDiagnosis under a held lock: %v", err)
		}
		got := findCheck(t, degraded, "stale hooks")

		if got.status != want.status {
			t.Errorf("status = %v under a degraded read, want %v", got.status, want.status)
		}
		if got.detail != want.detail {
			t.Errorf("detail = %q under a degraded read, want %q", got.detail, want.detail)
		}
		if doctorUnhealthy(degraded) != wantUnhealthy {
			t.Errorf("doctorUnhealthy = %v under a degraded read, want %v", doctorUnhealthy(degraded), wantUnhealthy)
		}
		assertDegradedReadVia(t, sink, "doctor")
	})

	t.Run("it leaves the config directory untouched across portal doctor", func(t *testing.T) {
		// A fresh install: the directory holds no hooks.json at all, so a read
		// that created either it or the sidecar would show up as a new entry.
		hooks.SetLockTimeoutForTest(t, 20*time.Millisecond)
		configDir := t.TempDir()
		store := hooks.NewStore(filepath.Join(configDir, "hooks.json"))
		before := dirListing(t, configDir)

		if _, err := runDoctorDiagnosis(staleDeps(t.TempDir(), fakeHookLister{rows: tokenRows(liveSeedA)}, store, nil)); err != nil {
			t.Fatalf("runDoctorDiagnosis: %v", err)
		}

		if after := dirListing(t, configDir); !reflect.DeepEqual(before, after) {
			t.Errorf("config directory changed across a read-only doctor: before=%v after=%v", before, after)
		}
	})
}

func TestHookListDegradedRead(t *testing.T) {
	t.Run("it lists hooks under a degraded read", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)
		_, hooksFile := hooksFileInTempDir(t)
		writeHooksJSON(t, hooksFile, map[string]map[string]string{
			"aaa111": {"on-resume": "npm start"},
		})
		hooksDeps = &HooksDeps{PaneLister: &recordingPaneHookLister{rows: []tmux.PaneHookRow{
			{Token: "aaa111", Location: "proj:0.0"},
		}}}
		t.Cleanup(func() { hooksDeps = nil })

		want := runHookList(t)

		holdHooksSidecar(t, hooksFile)
		sink := installHooksSink(t)
		got := runHookList(t)

		if got != want {
			t.Errorf("output under a degraded read = %q, want %q", got, want)
		}
		assertDegradedReadVia(t, sink, "cli")
	})

	t.Run("it takes no tmux read and creates nothing on a fresh install", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 20*time.Millisecond)
		configDir, _ := hooksFileInTempDir(t)
		hooksDeps = &HooksDeps{PaneLister: &loudPaneHookLister{t: t}}
		t.Cleanup(func() { hooksDeps = nil })

		before := dirListing(t, configDir)
		if got := runHookList(t); got != "" {
			t.Errorf("output = %q, want empty", got)
		}
		if after := dirListing(t, configDir); !reflect.DeepEqual(before, after) {
			t.Errorf("config directory changed across hook list: before=%v after=%v", before, after)
		}
	})
}

func TestSweepPreReadBound(t *testing.T) {
	t.Run("it degrades the sweep pre-read at the short bound", func(t *testing.T) {
		short := 60 * time.Millisecond
		hooks.SetSnapshotLockTimeoutForTest(t, short)
		hooks.SetLockTimeoutForTest(t, 5*time.Second)

		store, path := newTempHooksStore(t, `{"`+reapableSeedA+`": {"on-resume": "cmd-a"}}`)
		holdHooksSidecar(t, path)

		// An empty live set stands the cycle down after the pre-read, so the
		// elapsed time measures that read alone rather than CleanStale's own
		// exclusive acquire behind it.
		lister := &stubAllPaneLister{rows: tokenRows()}
		sink := installHooksSink(t)

		start := time.Now()
		err := runHookStaleCleanup(lister, store, bootstrapLogger, nil, nil)
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("runHookStaleCleanup: %v", err)
		}
		if elapsed >= time.Second {
			t.Fatalf("the pre-read spent %v under a held lock — it waited at lockTimeout, not snapshotLockTimeout (%v)", elapsed, short)
		}
		if elapsed < short {
			t.Errorf("the sweep returned after %v — the pre-read did not wait out the %v short bound", elapsed, short)
		}
		assertDegradedReadVia(t, sink, "internal")
	})
}

// dirListing is the directory's entry names, so a test can assert a read-only
// command added nothing to it.
func dirListing(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir %s: %v", dir, err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}
