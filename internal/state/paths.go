package state

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/leeovery/portal/internal/xdg"
)

const (
	sessionsJSONName  = "sessions.json"
	saveRequestedName = "save.requested"
	daemonPIDName     = "daemon.pid"
	daemonVersionName = "daemon.version"
	daemonLockName    = "daemon.lock"
	portalLogName     = "portal.log"
	portalLogOldName  = "portal.log.old"
	scrollbackSubdir  = "scrollback"
)

// Dir resolves the absolute path to Portal's state directory: $PORTAL_STATE_DIR
// verbatim when set, otherwise <XDG config base>/portal/state.
func Dir() (string, error) {
	if envPath := os.Getenv("PORTAL_STATE_DIR"); envPath != "" {
		return envPath, nil
	}

	base, err := xdg.ConfigBase()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "portal", "state"), nil
}

// EnsureDir creates the state directory and its scrollback subdirectory with
// mode 0700 and returns the state directory path. Existing directories are
// left in place.
func EnsureDir() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("failed to create state directory %s: %w", dir, err)
	}

	scrollback := filepath.Join(dir, scrollbackSubdir)
	if err := os.MkdirAll(scrollback, 0o700); err != nil {
		return "", fmt.Errorf("failed to create scrollback directory %s: %w", scrollback, err)
	}

	return dir, nil
}

// SessionsJSON returns the path to the structural index file.
func SessionsJSON(dir string) string { return filepath.Join(dir, sessionsJSONName) }

// SaveRequested returns the path to the dirty-flag file touched by
// `portal state notify`.
func SaveRequested(dir string) string { return filepath.Join(dir, saveRequestedName) }

// TouchSaveRequested creates-or-truncates the save.requested dirty flag under
// dir and bumps its mtime. The mtime bump is best-effort and its error is
// swallowed: the daemon's tick only checks the file's presence.
func TouchSaveRequested(dir string) error {
	path := SaveRequested(dir)
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("touch save.requested: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("touch save.requested: %w", err)
	}
	now := time.Now()
	_ = os.Chtimes(path, now, now)
	return nil
}

// DaemonPID returns the path to the daemon's PID file.
func DaemonPID(dir string) string { return filepath.Join(dir, daemonPIDName) }

// DaemonVersion returns the path to the daemon's version-marker file.
func DaemonVersion(dir string) string { return filepath.Join(dir, daemonVersionName) }

// DaemonLock returns the path to the daemon's advisory-lock file.
func DaemonLock(dir string) string { return filepath.Join(dir, daemonLockName) }

// PortalLog returns the path to the current portal log file.
func PortalLog(dir string) string { return filepath.Join(dir, portalLogName) }

// PortalLogOld returns the path to the previous (rotated) portal log file.
func PortalLogOld(dir string) string { return filepath.Join(dir, portalLogOldName) }

// ScrollbackDir returns the path to the directory holding per-pane
// scrollback `.bin` files.
func ScrollbackDir(dir string) string { return filepath.Join(dir, scrollbackSubdir) }

// ScrollbackFile returns the path to the scrollback `.bin` file for the
// given canonical paneKey.
func ScrollbackFile(dir, paneKey string) string {
	return filepath.Join(dir, scrollbackSubdir, paneKey+".bin")
}

// FIFOPath returns the hydration FIFO path for the given canonical paneKey.
func FIFOPath(dir, paneKey string) string {
	return filepath.Join(dir, "hydrate-"+paneKey+".fifo")
}

// PaneKeyFromFIFOPath recovers the paneKey from a hydration FIFO path or bare
// basename. An absent prefix or suffix is simply not trimmed.
func PaneKeyFromFIFOPath(path string) string {
	name := filepath.Base(path)
	name = strings.TrimSuffix(name, ".fifo")
	name = strings.TrimPrefix(name, "hydrate-")
	return name
}
