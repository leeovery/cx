package log

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
)

var symlinkFunc = os.Symlink

// Pid-scoped so two portal processes swinging concurrently cannot collide.
func pidSymlinkTmp(stateDir string, pid int) string {
	return filepath.Join(stateDir, portalLogName+"."+strconv.Itoa(pid)+".symlink.tmp")
}

const legacyOldName = portalLogName + ".old"

// migrationGuard frees the portal.log name so the next swing can claim it as a
// symlink. It must run before swingSymlink, never aborts the reopen, and must
// Lstat rather than Stat. After the first swing portal.log is itself the
// migration marker, so the guard no-ops forever and no separate flag is needed.
func migrationGuard(stateDir string) error {
	link := symlinkPath(stateDir)

	info, err := os.Lstat(link)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	// A regular file: pre-migration debris, along with any .old sibling.
	_ = os.Remove(link)
	_ = os.Remove(filepath.Join(stateDir, legacyOldName))
	return nil
}

// swingSymlink atomically re-points portal.log at target, which must be a bare
// day-file basename: storing it relative keeps the link valid across a move.
func swingSymlink(stateDir, target string) error {
	return swingSymlinkAs(stateDir, target, os.Getpid())
}

func swingSymlinkAs(stateDir, target string, pid int) error {
	link := symlinkPath(stateDir)
	pidTmp := pidSymlinkTmp(stateDir, pid)

	// Reclaim a same-pid temp leaked by a crash between Symlink and Rename.
	if err := os.Remove(pidTmp); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale symlink temp %s: %w", pidTmp, err)
	}

	if err := symlinkFunc(target, pidTmp); err != nil {
		return fmt.Errorf("create symlink temp %s -> %s: %w", pidTmp, target, err)
	}

	if err := os.Rename(pidTmp, link); err != nil {
		return fmt.Errorf("rename symlink temp %s -> %s: %w", pidTmp, link, err)
	}
	return nil
}
