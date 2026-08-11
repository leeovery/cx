package state

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/leeovery/portal/internal/fileutil"
)

// ErrPIDFileAbsent reports that daemon.pid does not exist, as distinct from an
// I/O or parse failure reading it.
var ErrPIDFileAbsent = errors.New("daemon.pid absent")

// ErrVersionFileAbsent reports that daemon.version does not exist, as distinct
// from an I/O failure reading it.
var ErrVersionFileAbsent = errors.New("daemon.version absent")

// WritePIDFile atomically writes pid to daemon.pid inside dir. Plain
// AtomicWrite is deliberate — the pid is non-sensitive, so the umask defence
// applied to sessions.json and scrollback is not wanted here.
func WritePIDFile(dir string, pid int) error {
	content := strconv.Itoa(pid) + "\n"
	return fileutil.AtomicWrite(DaemonPID(dir), []byte(content))
}

// ReadPIDFile reads daemon.pid from dir and returns the parsed pid. A missing
// file gives ErrPIDFileAbsent; every other failure is wrapped, with a zero pid.
func ReadPIDFile(dir string) (int, error) {
	data, err := readDaemonFile(DaemonPID(dir), ErrPIDFileAbsent)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, fmt.Errorf("parse daemon.pid: %w", err)
	}
	return pid, nil
}

// IsProcessAlive reports whether pid names an existing process, probing with
// kill(pid, 0). A process we lack permission to signal still counts as alive;
// pid <= 0 and every other error count as dead.
func IsProcessAlive(pid int) bool {
	if pid <= 0 {
		return false
	}

	err := syscall.Kill(pid, 0)
	if err == nil {
		return true
	}
	if errors.Is(err, syscall.EPERM) {
		return true
	}
	if errors.Is(err, syscall.ESRCH) {
		return false
	}
	return false
}

// DaemonAlive reports whether dir holds a daemon.pid naming a live process; a
// missing or unparseable file counts as not alive.
func DaemonAlive(dir string) bool {
	pid, err := ReadPIDFile(dir)
	if err != nil {
		return false
	}
	return IsProcessAlive(pid)
}

// WriteVersionFile atomically writes version to daemon.version inside dir.
// Plain AtomicWrite is deliberate, as in WritePIDFile.
func WriteVersionFile(dir, version string, logger *slog.Logger) error {
	logger = loggerOrDiscard(logger)
	path := DaemonVersion(dir)
	// Order matters: the breadcrumb precedes the write, so a write failure still
	// leaves a record of the intent.
	logger.Debug("daemon.version write", "path", path)
	return fileutil.AtomicWrite(path, []byte(version+"\n"))
}

// ReadVersionFile returns the trimmed contents of daemon.version in dir. A
// missing file gives ErrVersionFileAbsent, while an empty file gives ("", nil)
// — a recorded blank version is distinguishable from none.
func ReadVersionFile(dir string) (string, error) {
	data, err := readDaemonFile(DaemonVersion(dir), ErrVersionFileAbsent)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func readDaemonFile(path string, absentSentinel error) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, absentSentinel
		}
		return nil, fmt.Errorf("read %s: %w", filepath.Base(path), err)
	}
	return data, nil
}
