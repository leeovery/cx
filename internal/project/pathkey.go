package project

import (
	"path/filepath"

	"github.com/leeovery/portal/internal/resolver"
)

// CanonicalDirKey reduces a directory path to the canonical lookup key: tilde
// expanded, made absolute, symlinks evaluated, cleaned. Every path compared
// against a stored Project.Path must be reduced this way or a session silently
// drops out of its group. A path that cannot be resolved on disk still yields a
// stable key.
//
// It is built from lower-level primitives rather than resolver.NormalisePath,
// which deliberately does not evaluate symlinks.
func CanonicalDirKey(path string) string {
	expanded := resolver.ExpandTilde(path)

	abs, err := filepath.Abs(expanded)
	if err != nil {
		return filepath.Clean(expanded)
	}

	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return filepath.Clean(abs)
	}

	return filepath.Clean(resolved)
}
