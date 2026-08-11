package resolver

import (
	"path/filepath"
	"strings"
)

// A leading '[' opens a character class; a lone ']' cannot start one, so it is
// deliberately absent.
const globMeta = "*?["

// HasGlobMeta reports whether s contains a glob metacharacter; such a target is
// session-domain by construction, never run through the path, alias or zoxide
// chain.
func HasGlobMeta(s string) bool {
	return strings.ContainsAny(s, globMeta)
}

// MatchGlob returns the subset of names matching pattern, in the order given. A
// malformed pattern matches nothing rather than erroring, so a bad glob hard-fails
// at the caller.
func MatchGlob(pattern string, names []string) []string {
	matches := []string{}
	for _, name := range names {
		if ok, err := filepath.Match(pattern, name); err == nil && ok {
			matches = append(matches, name)
		}
	}
	return matches
}
