package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// IsPathArgument reports whether the argument looks like a filesystem path
// rather than a query string. An argument is treated as a path if it contains
// '/', or starts with '.' or '~'.
func IsPathArgument(arg string) bool {
	if arg == "" {
		return false
	}
	return strings.Contains(arg, "/") || arg[0] == '.' || arg[0] == '~'
}

// ResolvePath expands and validates a path argument. It expands tilde to the
// user's home directory, resolves relative paths to absolute, and validates
// that the result exists and is a directory. Returns the resolved absolute path.
func ResolvePath(arg string) (string, error) {
	expanded := expandTilde(arg)

	abs, err := filepath.Abs(expanded)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	info, err := os.Stat(abs)
	if err != nil {
		return "", fmt.Errorf("Directory not found: %s", abs) //nolint:staticcheck // user-facing message per spec
	}

	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", abs)
	}

	return abs, nil
}

// expandTilde replaces a leading ~ with the user's home directory.
func expandTilde(path string) string {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	return path
}
