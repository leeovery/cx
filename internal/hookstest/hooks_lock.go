package hookstest

import (
	"log/slog"
	"os"
	"sync"
	"testing"

	"golang.org/x/sys/unix"

	"github.com/leeovery/portal/internal/logtest"
)

// CreateHooksSidecar pre-creates the lock file a mutation opens, so a fixture
// that denies writes to the directory still fails where it means to, and so a
// read under the fixture takes its shared lock rather than degrading on a
// missing sidecar the fixture never meant to model.
func CreateHooksSidecar(t *testing.T, hooksPath string) {
	t.Helper()
	if err := os.WriteFile(hooksPath+".lock", nil, 0o600); err != nil {
		t.Fatalf("hookstest.CreateHooksSidecar: create sidecar lock: %v", err)
	}
}

// HoldHooksSidecar takes the sidecar exclusively from an independent open file
// description, modelling a writer in another process: every read taken while it
// is held must degrade rather than fail, and every mutation must time out at the
// bound and write nothing. The returned release lets a caller free the lock
// mid-test and retry the operation that could not take it; it is idempotent and
// also runs on cleanup.
func HoldHooksSidecar(t *testing.T, hooksPath string) func() {
	t.Helper()
	f := openSidecar(t, hooksPath)
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Fatalf("hookstest.HoldHooksSidecar: flock sidecar: %v", err)
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

// HoldHooksSidecarShared takes the sidecar LOCK_SH from an independent open file
// description, modelling another reader. An exclusive acquire blocks against it
// to the bound; a shared one is granted immediately — which is what makes it the
// discriminator between the two read modes.
func HoldHooksSidecarShared(t *testing.T, hooksPath string) {
	t.Helper()
	f := openSidecar(t, hooksPath)
	if err := unix.Flock(int(f.Fd()), unix.LOCK_SH|unix.LOCK_NB); err != nil {
		t.Fatalf("hookstest.HoldHooksSidecarShared: flock sidecar shared: %v", err)
	}
	t.Cleanup(func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	})
}

// AssertSidecarFree proves the operation under test released its hold: an
// exclusive non-blocking acquire from a fresh open file description succeeds
// only when no other fd holds the sidecar. It creates the sidecar if it is
// absent, so it can never stand in for an assertion that none exists.
func AssertSidecarFree(t *testing.T, hooksPath string) {
	t.Helper()
	f := openSidecar(t, hooksPath)
	defer func() { _ = f.Close() }()
	if err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		t.Errorf("hookstest.AssertSidecarFree: sidecar is still held: %v", err)
		return
	}
	_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
}

func openSidecar(t *testing.T, hooksPath string) *os.File {
	t.Helper()
	f, err := os.OpenFile(hooksPath+".lock", os.O_RDWR|os.O_CREATE, 0o600)
	if err != nil {
		t.Fatalf("open sidecar: %v", err)
	}
	return f
}

// UnlockedRecords returns the degradation breadcrumbs the sink captured, so a
// test can assert a read did not degrade as well as that it did.
func UnlockedRecords(t *testing.T, sink *logtest.Sink) logtest.Records {
	t.Helper()
	return sink.RecordsWithMessage("load-unlocked")
}

// AssertDegradedRead pins the whole shape of a degraded read's single
// breadcrumb: exactly one record, DEBUG, op=load-unlocked, via naming the
// caller, and the lock error carried.
func AssertDegradedRead(t *testing.T, sink *logtest.Sink, wantVia string) {
	t.Helper()
	r := UnlockedRecords(t, sink).Only(t, "load-unlocked record")
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
