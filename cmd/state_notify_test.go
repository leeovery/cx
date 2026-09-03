package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/state"
)

func TestStateNotify_CreatesSaveRequestedWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	if _, _, err := runRootCmd(t, "state", "notify"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	path := filepath.Join(dir, "save.requested")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("save.requested not created: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("save.requested size = %d, want 0", info.Size())
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("save.requested mode = %o, want 0600", perm)
	}
}

func TestStateNotify_BumpsMtimeWhenPresent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	if _, _, err := runRootCmd(t, "state", "notify"); err != nil {
		t.Fatalf("first notify: unexpected error: %v", err)
	}
	path := filepath.Join(dir, "save.requested")
	first, err := os.Stat(path)
	if err != nil {
		t.Fatalf("first stat: %v", err)
	}

	time.Sleep(10 * time.Millisecond)

	if _, _, err := runRootCmd(t, "state", "notify"); err != nil {
		t.Fatalf("second notify: unexpected error: %v", err)
	}
	second, err := os.Stat(path)
	if err != nil {
		t.Fatalf("second stat: %v", err)
	}

	if !second.ModTime().After(first.ModTime()) {
		t.Errorf("mtime did not advance: first=%v second=%v", first.ModTime(), second.ModTime())
	}
}

func TestStateNotify_CreatesStateDirWithMode0700WhenMissing(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "state-not-yet-created")
	t.Setenv("PORTAL_STATE_DIR", dir)

	if _, _, err := runRootCmd(t, "state", "notify"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("state dir not created: %v", err)
	}
	if !info.IsDir() {
		t.Fatalf("state path is not a directory")
	}
	if perm := info.Mode().Perm(); perm != 0o700 {
		t.Errorf("state dir mode = %o, want 0700", perm)
	}
}

func TestStateNotify_TruncatesExistingContent(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	path := filepath.Join(dir, "save.requested")
	if err := os.WriteFile(path, []byte("foo"), 0o600); err != nil {
		t.Fatalf("seed write: %v", err)
	}

	if _, _, err := runRootCmd(t, "state", "notify"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("save.requested size = %d, want 0 (truncated)", info.Size())
	}
}

func TestStateNotify_ExitsZeroOnSuccess(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	_, _, err := runRootCmd(t, "state", "notify")
	if err != nil {
		t.Fatalf("expected exit 0, got error: %v", err)
	}
}

func TestStateNotify_ExitsNonZeroWhenStateDirNotWritable(t *testing.T) {
	parent := t.TempDir()
	dir := filepath.Join(parent, "state")

	// Read+execute only so MkdirAll cannot create the state subdirectory.
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatalf("chmod parent: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	t.Setenv("PORTAL_STATE_DIR", dir)

	_, _, err := runRootCmd(t, "state", "notify")
	if err == nil {
		t.Fatal("expected non-zero exit when state dir is not writable, got nil")
	}
}

func TestStateNotify_DoesNotReadOrCreateOtherStateFiles(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	if _, _, err := runRootCmd(t, "state", "notify"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, "sessions.json")); !os.IsNotExist(err) {
		t.Errorf("sessions.json must not exist after notify; stat err = %v", err)
	}

	// scrollback/ may exist (EnsureDir creates it) but must be empty.
	scrollback := filepath.Join(dir, "scrollback")
	if entries, err := os.ReadDir(scrollback); err == nil {
		if len(entries) != 0 {
			t.Errorf("scrollback/ must be empty after notify, got %d entries", len(entries))
		}
	} else if !os.IsNotExist(err) {
		t.Errorf("unexpected scrollback stat error: %v", err)
	}
}

func TestStateNotify_DoesNotInvokeBootstrap(t *testing.T) {
	withBootstrapDeps(t, BootstrapDeps{Orchestrator: panicRunner{}})

	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("PersistentPreRunE invoked bootstrap: %v", r)
		}
	}()

	if _, _, err := runRootCmd(t, "state", "notify"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// save.requested is pre-created as a directory: OpenFile with
// O_WRONLY|O_CREATE|O_TRUNC against one fails with EISDIR.
func TestStateNotify_LogsWarnOnSaveRequestedCreateFailure(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PORTAL_STATE_DIR", dir)

	sink := logtest.Install(t)

	// Create the state dir here so the blocking directory can be planted at
	// save.requested before notify runs.
	if _, err := state.EnsureDir(); err != nil {
		t.Fatalf("EnsureDir: %v", err)
	}
	blockingPath := filepath.Join(dir, "save.requested")
	if err := os.Mkdir(blockingPath, 0o700); err != nil {
		t.Fatalf("mkdir blocking save.requested: %v", err)
	}

	_, _, err := runRootCmd(t, "state", "notify")
	if err == nil {
		t.Fatal("expected non-zero exit when save.requested cannot be created, got nil")
	}

	logged := sink.Body()
	if !strings.Contains(logged, "WARN") {
		t.Errorf("log missing WARN level entry: %q", logged)
	}
	if !strings.Contains(logged, "component="+"notify") {
		t.Errorf("log missing %q component: %q", "notify", logged)
	}
	if !strings.Contains(logged, "save.requested") {
		t.Errorf("log missing failing path 'save.requested': %q", logged)
	}
}
