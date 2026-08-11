// Package themetest supports Portal's theme tests: it authors `.theme` fixture
// files, loads the embedded built-ins by slug, and builds the synthetic probe
// palettes a swap guard diffs between. It is the single definition of the
// fixture format, so a change to what the loader reads is one edit here.
// Lines() is a complete valid file and the WithX helpers derive the broken
// variants, each returning a fresh slice.
//
// Test-only: production code must not import this package.
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

// One mode for every fixture, so no test can depend on a permission difference
// that says nothing about what it is testing.
const fixtureMode = 0o600

// Lines renders a complete, valid theme file: one `key = value` line per
// token, in canonical order, each carrying a distinct value. The values are
// lower case, so a theme parsed from these lines proves the loader's
// canonicalisation rather than merely echoing the file.
func Lines() []string {
	names := theme.TokenNames()
	lines := make([]string, 0, len(names))
	for i, name := range names {
		lines = append(lines, fmt.Sprintf("%s = #abcd%02x", name, i+1))
	}
	return lines
}

// Body renders Lines() as one complete file body — the same bytes Write puts
// on disk.
func Body() []byte {
	return Render(Lines())
}

// Render renders lines as a file body: newline-separated, newline-terminated.
func Render(lines []string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
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

// WithDuplicateKeyAt returns the lines with a copy of the named key's line
// spliced in as the at-th line (1-based) of the result. Lines declaring the
// key nowhere are returned unchanged and the input is left untouched; the
// position is a parameter because the rejection detail names it. Splicing
// outside the result panics.
func WithDuplicateKeyAt(lines []string, key string, at int) []string {
	spliced := slices.Clone(lines)
	first := slices.IndexFunc(spliced, func(line string) bool {
		return strings.HasPrefix(line, key+" = ")
	})
	if first < 0 {
		return spliced
	}
	return slices.Insert(spliced, at-1, spliced[first])
}

// Write writes lines as a file named base inside dir and returns its path.
func Write(t *testing.T, dir, base string, lines []string) string {
	t.Helper()

	path := filepath.Join(dir, base)
	if err := os.WriteFile(path, Render(lines), fixtureMode); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}
