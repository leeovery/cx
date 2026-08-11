package state_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"testing"

	"github.com/leeovery/portal/internal/state"
)

// Lstat, so a symlink reports its own inode rather than the target's.
func inodeOf(t *testing.T, path string) uint64 {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("info.Sys() is %T; want *syscall.Stat_t", info.Sys())
	}
	return uint64(st.Ino)
}

func assertIsFIFO(t *testing.T, path string, wantPerm os.FileMode) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat %s: %v", path, err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("%s: mode = %v; expected ModeNamedPipe to be set", path, info.Mode())
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("%s: still a symlink after CreateFIFO", path)
	}
	if got := info.Mode().Perm(); got != wantPerm {
		t.Errorf("%s perm = %o; want %o", path, got, wantPerm)
	}
}

func TestCreateFIFO_CreatesFreshFIFOWith0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "hydrate-work__0.1.fifo")

	if err := state.CreateFIFO(path); err != nil {
		t.Fatalf("CreateFIFO: %v", err)
	}

	assertIsFIFO(t, path, 0o600)
}

func TestCreateFIFO_VerifiesNamedPipeMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fifo")

	if err := state.CreateFIFO(path); err != nil {
		t.Fatalf("CreateFIFO: %v", err)
	}

	info, err := os.Lstat(path)
	if err != nil {
		t.Fatalf("lstat: %v", err)
	}
	if info.Mode()&os.ModeNamedPipe == 0 {
		t.Fatalf("ModeNamedPipe not set: mode = %v", info.Mode())
	}
}

func TestCreateFIFO_ReplacesStaleFIFOCleanly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fifo")

	if err := state.CreateFIFO(path); err != nil {
		t.Fatalf("first CreateFIFO: %v", err)
	}
	first := inodeOf(t, path)

	if err := state.CreateFIFO(path); err != nil {
		t.Fatalf("second CreateFIFO: %v", err)
	}
	second := inodeOf(t, path)

	if first == second {
		t.Fatalf("inode unchanged across recreate: %d", first)
	}
	assertIsFIFO(t, path, 0o600)
}

func TestCreateFIFO_RecreatesEvenWhenExistingFIFOIsAlreadyMode0600(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fifo")

	if err := syscall.Mkfifo(path, 0o600); err != nil {
		t.Fatalf("seed Mkfifo: %v", err)
	}
	first := inodeOf(t, path)

	if err := state.CreateFIFO(path); err != nil {
		t.Fatalf("CreateFIFO: %v", err)
	}
	second := inodeOf(t, path)

	if first == second {
		t.Fatalf("inode unchanged across recreate of existing 0600 FIFO: %d", first)
	}
	assertIsFIFO(t, path, 0o600)
}

func TestCreateFIFO_ReplacesRegularFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "fifo")

	if err := os.WriteFile(path, []byte("stale data"), 0o600); err != nil {
		t.Fatalf("seed WriteFile: %v", err)
	}

	if err := state.CreateFIFO(path); err != nil {
		t.Fatalf("CreateFIFO: %v", err)
	}

	assertIsFIFO(t, path, 0o600)
}

func TestCreateFIFO_ReplacesSymlinkWithFIFO(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	if err := os.WriteFile(target, []byte("target content"), 0o600); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	link := filepath.Join(dir, "fifo")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("seed symlink: %v", err)
	}

	if err := state.CreateFIFO(link); err != nil {
		t.Fatalf("CreateFIFO: %v", err)
	}

	assertIsFIFO(t, link, 0o600)

	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(data) != "target content" {
		t.Errorf("target content = %q; want %q", string(data), "target content")
	}
	info, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("lstat target: %v", err)
	}
	if !info.Mode().IsRegular() {
		t.Errorf("target is no longer a regular file: mode = %v", info.Mode())
	}
}

func TestCreateFIFO_ToleratesENOENTFromOSRemove(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "never-existed")

	if err := state.CreateFIFO(path); err != nil {
		t.Fatalf("CreateFIFO at fresh path: %v", err)
	}

	assertIsFIFO(t, path, 0o600)
}

func TestCreateFIFO_WrapsMkfifoErrorWithPathWhenParentMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent-subdir", "fifo")

	err := state.CreateFIFO(path)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, path) {
		t.Errorf("error %q does not contain path %q", msg, path)
	}
	if !strings.Contains(msg, "mkfifo") {
		t.Errorf("error %q does not mention mkfifo", msg)
	}
}

func TestCreateFIFO_PreservesPathErrorSoErrorsIsPermissionTraverses(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based EACCES setup is unix-specific")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses 0500 directory write protection")
	}

	parent := t.TempDir()
	path := filepath.Join(parent, "fifo")
	if err := os.WriteFile(path, []byte("seed"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatalf("chmod parent: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(parent, 0o700) })

	err := state.CreateFIFO(path)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected non-ENOENT error, got %v", err)
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("errors.Is(err, fs.ErrPermission) = false; *os.PathError dropped? err = %v", err)
	}
}

func TestCreateFIFO_WrapsRemoveErrorWithPath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("chmod-based EACCES setup is unix-specific")
	}
	if os.Geteuid() == 0 {
		t.Skip("root bypasses 0500 directory write protection")
	}

	parent := t.TempDir()
	path := filepath.Join(parent, "fifo")
	if err := os.WriteFile(path, []byte("seed"), 0o600); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	// Strip parent write permission so the unlink fails with EACCES.
	if err := os.Chmod(parent, 0o500); err != nil {
		t.Fatalf("chmod parent: %v", err)
	}
	t.Cleanup(func() {
		// Restore so t.TempDir's cleanup can succeed.
		_ = os.Chmod(parent, 0o700)
	})

	err := state.CreateFIFO(path)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("expected non-ENOENT error, got %v", err)
	}
	msg := err.Error()
	if !strings.Contains(msg, path) {
		t.Errorf("error %q does not contain path %q", msg, path)
	}
	if !strings.Contains(msg, "remove existing") {
		t.Errorf("error %q does not mention 'remove existing'", msg)
	}
}
