package project

import (
	"path/filepath"

	"github.com/leeovery/portal/internal/resolver"
)

// CanonicalDirKey reduces a directory path to the canonical lookup key. Every
// path compared against a stored Project.Path must be reduced this way or a
// session silently drops out of its group; an unresolvable path still yields a
// stable key. resolver.NormalisePath is not used: it deliberately does not
// evaluate symlinks.
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
