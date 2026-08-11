package themetest

import (
	"os"
	"testing"
)

// DenyRead stages an existing file as unreadable for the rest of the test and
// returns the OS error a read of it now fails with, so a caller asserting
// "denied, not absent" asserts on the returned value rather than by re-reading.
// The prior mode is restored on cleanup: a path left at mode 0000 blocks both
// a later read and a temp root's teardown.
//
// Where the mode bits deny nothing — the process is root, or the filesystem
// ignores them — the fixture is impossible, so the test skips rather than
// asserting vacuously.
func DenyRead(t *testing.T, path string) error {
	t.Helper()

	return deny(t, path, func() error {
		_, err := os.ReadFile(path)
		return err
	})
}

// DenyDir stages an existing directory as unreadable for the rest of the test
// and returns the OS error a read of it now fails with. It carries DenyRead's
// contract — the returned denial, the restored mode, and the same root policy —
// over a directory.
func DenyDir(t *testing.T, dir string) error {
	t.Helper()

	return deny(t, dir, func() error {
		_, err := os.ReadDir(dir)
		return err
	})
}

func deny(t *testing.T, path string, read func() error) error {
	t.Helper()

	restoreMode(t, path)
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod 0000 %s: %v", path, err)
	}

	err := read()
	switch {
	case err == nil:
		t.Skipf("reading %s succeeds at mode 0000; the mode bits deny nothing here, so the fixture is impossible", path)
	case os.IsNotExist(err):
		t.Fatalf("reading %s reports %v; the fixture must exist to be staged unreadable", path, err)
	}
	return err
}

func restoreMode(t *testing.T, path string) {
	t.Helper()

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat %s: %v", path, err)
	}
	mode := info.Mode().Perm()
	t.Cleanup(func() {
		if err := os.Chmod(path, mode); err != nil {
			t.Errorf("restore mode %#o on %s: %v", mode, path, err)
		}
	})
}
