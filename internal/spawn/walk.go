package spawn

import (
	"errors"
	"fmt"
	"os/exec"
	"path"
	"strconv"
	"strings"

	"github.com/leeovery/portal/internal/log"
)

// ProcessWalker reads one hop of a process tree: a pid's parent pid and its
// executable command, a full path on macOS.
type ProcessWalker interface {
	ProcessInfo(pid int) (ppid int, command string, err error)
}

// BundleReader reads a macOS `.app` bundle directory's CFBundleIdentifier and a
// friendly display name.
type BundleReader interface {
	Read(appPath string) (bundleID, name string, err error)
}

// ErrDetectTransient marks a transient terminal-detection failure — a `ps` or
// `defaults read` error mid-walk. It is distinct from the clean "no host-local
// terminal" outcome, which is a NULL Identity with a nil error. The underlying
// cause stays reachable through the chain.
var ErrDetectTransient = errors.New("terminal detection transient failure")

// Bounds the ancestry walk so a cyclic or pathologically long process tree
// cannot hang detection. Hitting the bound is a clean NULL, not an error.
const maxWalkHops = 32

const appBundleSuffix = ".app"

// Exhausting the ancestry — reaching the root, a repeated pid, or the hop bound
// — is a clean NULL with a nil error. Only a failed `ps` or `defaults` read is an
// error.
func walkToBundle(startPID int, walker ProcessWalker, reader BundleReader) (Identity, error) {
	seen := make(map[int]bool)
	pid := startPID

	for range maxWalkHops {
		if seen[pid] {
			return Identity{}, nil
		}
		seen[pid] = true

		ppid, command, err := walker.ProcessInfo(pid)
		if err != nil {
			return Identity{}, transient(fmt.Sprintf("read process info for pid %d", pid), err)
		}

		if appPath, ok := appBundlePath(command); ok {
			bundleID, name, rerr := reader.Read(appPath)
			if rerr != nil {
				return Identity{}, transient(fmt.Sprintf("read bundle info for %s", appPath), rerr)
			}
			return NewIdentity(bundleID, name), nil
		}

		if ppid <= 1 {
			return Identity{}, nil
		}
		pid = ppid
	}

	return Identity{}, nil
}

// Wrapping twice keeps both the sentinel and the underlying cause reachable via
// errors.Is.
func transient(what string, cause error) error {
	return fmt.Errorf("%s: %w: %w", what, ErrDetectTransient, cause)
}

func appBundlePath(command string) (string, bool) {
	marker := appBundleSuffix + "/"
	idx := strings.Index(command, marker)
	if idx < 0 {
		return "", false
	}
	return command[:idx+len(appBundleSuffix)], true
}

type realProcessWalker struct{}

var _ ProcessWalker = realProcessWalker{}

func (realProcessWalker) ProcessInfo(pid int) (int, string, error) {
	cmd := exec.Command("ps", "-o", "ppid=,comm=", "-p", strconv.Itoa(pid))
	out, err := log.CombinedOutputWithContext(cmd)
	if err != nil {
		return 0, "", err
	}
	return parsePSProcessInfo(string(out))
}

// `ps -o comm=` yields a full path that may itself contain spaces, so only the
// leading whitespace-delimited ppid field is split off; the rest is the command.
func parsePSProcessInfo(out string) (int, string, error) {
	line := strings.TrimSpace(out)
	if line == "" {
		return 0, "", errors.New("empty ps output")
	}

	idx := strings.IndexAny(line, " \t")
	if idx < 0 {
		return 0, "", fmt.Errorf("malformed ps output %q", line)
	}

	ppid, err := strconv.Atoi(line[:idx])
	if err != nil {
		return 0, "", fmt.Errorf("parse ppid from %q: %w", line, err)
	}
	command := strings.TrimSpace(line[idx:])
	if command == "" {
		return 0, "", fmt.Errorf("empty command in ps output %q", line)
	}
	return ppid, command, nil
}

type realBundleReader struct{}

var _ BundleReader = realBundleReader{}

// CFBundleIdentifier is required; CFBundleName is best-effort, falling back to
// the `.app` basename.
func (realBundleReader) Read(appPath string) (string, string, error) {
	plist := path.Join(appPath, "Contents", "Info.plist")

	bundleID, err := readDefault(plist, "CFBundleIdentifier")
	if err != nil {
		return "", "", err
	}

	name, err := readDefault(plist, "CFBundleName")
	if err != nil || name == "" {
		name = appBasename(appPath)
	}
	return bundleID, name, nil
}

func readDefault(plist, key string) (string, error) {
	cmd := exec.Command("defaults", "read", plist, key)
	out, err := log.CombinedOutputWithContext(cmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func appBasename(appPath string) string {
	return strings.TrimSuffix(path.Base(appPath), appBundleSuffix)
}
