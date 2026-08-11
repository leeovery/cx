package themetest_test

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/leeovery/portal/internal/themetest"
)

func TestDenyRead_MakesTheFileUnreadable(t *testing.T) {
	path := writeModedFile(t, filepath.Join(t.TempDir(), "nord-lee.theme"), 0o640)

	err := themetest.DenyRead(t, path)

	requireDenial(t, err)
	if _, readErr := os.ReadFile(path); readErr == nil {
		t.Fatalf("os.ReadFile(%q) succeeded while the file is staged unreadable", path)
	}
}

func TestDenyRead_ReturnsTheDenialItself(t *testing.T) {
	path := writeModedFile(t, filepath.Join(t.TempDir(), "nord-lee.theme"), 0o640)

	err := themetest.DenyRead(t, path)

	_, readErr := os.ReadFile(path)
	if err.Error() != readErr.Error() {
		t.Errorf("DenyRead returned %v, want the error a read of the staged file reports: %v", err, readErr)
	}
}

func TestDenyRead_RestoresThePriorModeOnCleanup(t *testing.T) {
	path := writeModedFile(t, filepath.Join(t.TempDir(), "nord-lee.theme"), 0o640)

	t.Run("staged", func(t *testing.T) { _ = themetest.DenyRead(t, path) })

	if got := modeOf(t, path); got != 0o640 {
		t.Errorf("mode after cleanup = %#o, want the prior %#o", got, 0o640)
	}
}

func TestDenyDir_MakesTheDirectoryUnreadable(t *testing.T) {
	dir := makeModedDir(t, filepath.Join(t.TempDir(), "themes"), 0o750)
	themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())

	err := themetest.DenyDir(t, dir)

	requireDenial(t, err)
	if _, readErr := os.ReadDir(dir); readErr == nil {
		t.Fatalf("os.ReadDir(%q) succeeded while the directory is staged unreadable", dir)
	}
}

func TestDenyDir_ReturnsTheDenialItself(t *testing.T) {
	dir := makeModedDir(t, filepath.Join(t.TempDir(), "themes"), 0o750)

	err := themetest.DenyDir(t, dir)

	_, readErr := os.ReadDir(dir)
	if err.Error() != readErr.Error() {
		t.Errorf("DenyDir returned %v, want the error a read of the staged directory reports: %v", err, readErr)
	}
}

func TestDenyDir_RestoresThePriorModeOnCleanup(t *testing.T) {
	dir := makeModedDir(t, filepath.Join(t.TempDir(), "themes"), 0o750)

	t.Run("staged", func(t *testing.T) { _ = themetest.DenyDir(t, dir) })

	if got := modeOf(t, dir); got != 0o750 {
		t.Errorf("mode after cleanup = %#o, want the prior %#o", got, 0o750)
	}
}

func requireDenial(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("the helper returned no error, want the denial the staged path now reports")
	}
	if !errors.Is(err, fs.ErrPermission) {
		t.Errorf("returned error = %v, want a permission error", err)
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("returned error = %v, want the denial rather than a not-exist error", err)
	}
}

func writeModedFile(t *testing.T, path string, mode fs.FileMode) string {
	t.Helper()

	if err := os.WriteFile(path, []byte("payload\n"), mode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func makeModedDir(t *testing.T, dir string, mode fs.FileMode) string {
	t.Helper()

	if err := os.Mkdir(dir, mode); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	return dir
}

func modeOf(t *testing.T, path string) fs.FileMode {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	return info.Mode().Perm()
}
