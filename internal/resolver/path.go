package resolver

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func IsPathArgument(arg string) bool {
	if arg == "" {
		return false
	}
	return strings.Contains(arg, "/") || arg[0] == '.' || arg[0] == '~'
}

// ResolvePath expands a tilde and returns the absolute path, erroring unless it
// exists and is a directory.
func ResolvePath(arg string) (string, error) {
	expanded := ExpandTilde(arg)

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

// NormalisePath expands a tilde and resolves to absolute. Unlike ResolvePath it
// does not check that the path exists.
func NormalisePath(path string) string {
	expanded := ExpandTilde(path)

	abs, err := filepath.Abs(expanded)
	if err != nil {
		return expanded
	}

	return abs
}

func ExpandTilde(path string) string {
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
