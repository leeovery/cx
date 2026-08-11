package state

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"syscall"
)

// CreateFIFO removes anything already at path, so callers always get a fresh
// inode: a stale FIFO from a crashed bootstrap has no live reader, and reusing
// it would block the writer indefinitely. The chmod after mkfifo defends against
// a tight umask, and its error is deliberately ignored.
func CreateFIFO(path string) error {
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("create fifo %s: remove existing: %w", path, err)
	}
	if err := syscall.Mkfifo(path, 0o600); err != nil {
		return fmt.Errorf("create fifo %s: mkfifo: %w", path, err)
	}
	_ = os.Chmod(path, 0o600)
	return nil
}
