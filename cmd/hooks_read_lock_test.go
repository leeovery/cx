package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/hooks"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/tmux"
	"github.com/leeovery/portal/internal/transienttest"
)

func installHooksSink(t *testing.T) *logtest.Sink {
	t.Helper()
	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	return sink
}

func TestDoctorStaleHooksDegradedRead(t *testing.T) {
	t.Run("it keeps the stale-hooks check green under a degraded read", func(t *testing.T) {
		hooks.SetLockTimeoutForTest(t, 40*time.Millisecond)
		lister := &stubStaleSweepReader{rows: tokenRows(liveSeedA)}

		// The baseline must be a genuinely *locked* read: seedHooksJSON stages no
		// sidecar, so without this the baseline would itself degrade on ENOENT and
		// the comparison below would be degraded-against-degraded.
		unlockedStore, unlockedPath := seedHooksJSON(t, liveSeedA)
		transienttest.CreateHooksSidecar(t, unlockedPath)
		baseline, err := runDoctorDiagnosis(staleDeps(t.TempDir(), lister, unlockedStore, nil))
		if err != nil {
			t.Fatalf("runDoctorDiagnosis: %v", err)
		}
		want := findCheck(t, baseline, "stale hooks")
		wantUnhealthy := doctorUnhealthy(baseline)

		heldStore, heldPath := seedHooksJSON(t, liveSeedA)
		transienttest.HoldHooksSidecar(t, heldPath)

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
		transienttest.AssertDegradedRead(t, sink, "doctor")
	})

	t.Run("it leaves the config directory untouched across portal doctor", func(t *testing.T) {
		// A fresh install: the directory holds no hooks.json at all, so a read
		// that created either it or the sidecar would show up as a new entry.
		hooks.SetLockTimeoutForTest(t, 20*time.Millisecond)
		configDir := t.TempDir()
		store := hooks.NewStore(filepath.Join(configDir, "hooks.json"))
		before := dirListing(t, configDir)

		if _, err := runDoctorDiagnosis(staleDeps(t.TempDir(), &stubStaleSweepReader{rows: tokenRows(liveSeedA)}, store, nil)); err != nil {
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

		transienttest.HoldHooksSidecar(t, hooksFile)
		sink := installHooksSink(t)
		got := runHookList(t)

		if got != want {
			t.Errorf("output under a degraded read = %q, want %q", got, want)
		}
		transienttest.AssertDegradedRead(t, sink, "cli")
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
		transienttest.HoldHooksSidecar(t, path)

		// An empty live set stands the cycle down after the pre-read, so the
		// elapsed time measures that read alone rather than CleanStale's own
		// exclusive acquire behind it.
		lister := &stubStaleSweepReader{rows: tokenRows()}
		sink := installHooksSink(t)

		start := time.Now()
		err := sweepErr(lister, store, bootstrapLogger)
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
		transienttest.AssertDegradedRead(t, sink, "internal")
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
