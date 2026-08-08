// Package themetest authors `.theme` fixture files for Portal's theme tests.
//
// It is the single definition of the fixture format every consumer stages files
// in — the loader's `key = value` shape, the canonical token order, and the one
// mode fixture files are written with — so a change to what the loader reads is
// one edit rather than a hunt across the packages that test against it.
//
// The helpers compose: Lines() is a complete, valid file, and WithValue /
// WithoutKey derive the broken variants from it (a bad colour, a missing token),
// each returning a fresh slice so one base can seed several fixtures.
//
// Test-only: production code MUST NOT import this package (mirroring the
// precedent for logtest / spawntest / transienttest). Enforcement is contributor
// discipline plus the *testing.T parameter on Write.
package themetest

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// fixtureMode is the mode this package writes fixture theme files with. One
// mode, so no test can come to depend on a permission difference that says
// nothing about what it is testing.
const fixtureMode = 0o600

// Lines renders a complete, valid theme file: one `key = value` line per token,
// in the canonical table order, each carrying a distinct value.
//
// The names come from theme.TokenNames() rather than a restatement, so a
// vocabulary change moves every fixture with it. The values are LOWER case, so a
// theme parsed from these lines proves the loader's canonicalisation rather than
// merely echoing the file.
func Lines() []string {
	names := theme.TokenNames()
	lines := make([]string, 0, len(names))
	for i, name := range names {
		lines = append(lines, fmt.Sprintf("%s = #abcd%02x", name, i+1))
	}
	return lines
}

// WithValue returns the lines with the named key's value replaced, in the same
// file order. The input is left untouched.
func WithValue(lines []string, key, value string) []string {
	replaced := slices.Clone(lines)
	for i, line := range replaced {
		if strings.HasPrefix(line, key+" = ") {
			replaced[i] = key + " = " + value
		}
	}
	return replaced
}

// WithoutKey returns the lines with the named key's line removed — a file that
// never declared it. The input is left untouched.
func WithoutKey(lines []string, key string) []string {
	return slices.DeleteFunc(slices.Clone(lines), func(line string) bool {
		return strings.HasPrefix(line, key+" = ")
	})
}

// Write writes lines as a file named base inside dir and returns its path.
func Write(t *testing.T, dir, base string, lines []string) string {
	t.Helper()

	path := filepath.Join(dir, base)
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")+"\n"), fixtureMode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
