package themetest_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// These tests pin the fixture format against the REAL loader rather than against
// a restatement of it: every consumer stages files with these helpers and asserts
// on what the loader made of them, so a fixture that stopped being accepted — or
// stopped being rejected for the reason its consumer named — would fail far from
// here, in tests about something else entirely.
//
// No t.Parallel(), matching the project-wide rule.

// TestLines_ProducesAFileTheLoaderAccepts is the positive half: a file built from
// Lines() alone loads, and the 19 values arrive as written (in the loader's
// canonical upper case).
func TestLines_ProducesAFileTheLoaderAccepts(t *testing.T) {
	lines := themetest.Lines()
	path := themetest.Write(t, t.TempDir(), "nord-lee.theme", lines)

	got, rejection := theme.Loader{}.LoadFile(path)

	if rejection != nil {
		t.Fatalf("LoadFile(%q) rejected the fixture: %v", path, rejection)
	}
	if want := "nord-lee"; got.Slug != want {
		t.Errorf("slug = %q, want %q", got.Slug, want)
	}
	if tokens, want := got.Theme.All(), wantTokensFrom(lines); !slices.Equal(tokens, want) {
		t.Errorf("theme = %+v, want %+v", tokens, want)
	}
}

// TestLines_GivesEveryTokenADistinctValue pins the half of Lines()' contract the
// loader cannot: consumers tell two fixture files apart by ONE token's value, and
// a fixture whose tokens shared a value could not say which token a rejection
// named.
func TestLines_GivesEveryTokenADistinctValue(t *testing.T) {
	lines := themetest.Lines()

	if got, want := len(lines), len(theme.TokenNames()); got != want {
		t.Fatalf("Lines() returned %d lines, want one per token (%d)", got, want)
	}

	seen := make(map[string]string, len(lines))
	for _, line := range lines {
		name, value := splitLine(t, line)
		if first, dup := seen[value]; dup {
			t.Errorf("%s and %s both carry %q; every token must be distinguishable by its value", first, name, value)
		}
		seen[value] = name
	}
}

// TestWithValue_SubstitutesTheNamedTokenInPlace pins the substituter's accepting
// path: the file still loads, the named token carries the new value, and the
// order the loader reads is unchanged.
func TestWithValue_SubstitutesTheNamedTokenInPlace(t *testing.T) {
	const canvas = "#1A2B3C"
	lines := themetest.WithValue(themetest.Lines(), "canvas", canvas)
	path := themetest.Write(t, t.TempDir(), "nord-lee.theme", lines)

	got, rejection := theme.Loader{}.LoadFile(path)

	if rejection != nil {
		t.Fatalf("LoadFile(%q) rejected the substituted fixture: %v", path, rejection)
	}
	if got.Theme.Canvas.Value != canvas {
		t.Errorf("canvas = %q, want the substituted %q", got.Theme.Canvas.Value, canvas)
	}
	if names, want := tokenNamesFrom(t, lines), theme.TokenNames(); !slices.Equal(names, want) {
		t.Errorf("substituting reordered the file:\n got %v\nwant %v", names, want)
	}
}

// TestWithValue_ProducesTheBadColourRejection is the class every consumer's
// broken-file fixture depends on: a value the loader cannot parse as a colour
// fails on the bad-colour rung, not on a rung above it.
func TestWithValue_ProducesTheBadColourRejection(t *testing.T) {
	path := themetest.Write(t, t.TempDir(), "nord-lee.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))

	_, rejection := theme.Loader{}.LoadFile(path)

	requireReason(t, rejection, theme.ReasonBadColour)
}

// TestWithoutKey_ProducesTheMissingTokenRejection is the other class consumers
// name: a file that never declared a token fails the presence check.
func TestWithoutKey_ProducesTheMissingTokenRejection(t *testing.T) {
	lines := themetest.WithoutKey(themetest.Lines(), "bg.subtle")
	path := themetest.Write(t, t.TempDir(), "nord-lee.theme", lines)

	_, rejection := theme.Loader{}.LoadFile(path)

	requireReason(t, rejection, theme.ReasonMissingTokens)
	if slices.Contains(tokenNamesFrom(t, lines), "bg.subtle") {
		t.Errorf("WithoutKey left a bg.subtle line behind: %v", lines)
	}
	if got, want := len(lines), len(theme.TokenNames())-1; got != want {
		t.Errorf("WithoutKey returned %d lines, want %d — it must remove exactly the one key", got, want)
	}
}

// TestMutatorsLeaveTheirInputAlone pins the copy-on-write both mutators promise.
// Consumers derive several fixtures from one base slice, so a mutator that wrote
// through would silently corrupt every later file built from it.
func TestMutatorsLeaveTheirInputAlone(t *testing.T) {
	base := themetest.Lines()
	before := slices.Clone(base)

	themetest.WithValue(base, "canvas", "blue")
	themetest.WithoutKey(base, "bg.subtle")

	if !slices.Equal(base, before) {
		t.Errorf("the mutators wrote through to their input:\n got %v\nwant %v", base, before)
	}
}

// TestWrite_WritesTheNamedFileAtTheOneFixtureMode pins the writer's two
// observable outputs: where the file lands, and the single mode it writes.
func TestWrite_WritesTheNamedFileAtTheOneFixtureMode(t *testing.T) {
	dir := t.TempDir()

	path := themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())

	if want := filepath.Join(dir, "nord-lee.theme"); path != want {
		t.Errorf("Write returned %q, want %q", path, want)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat the written fixture: %v", err)
	}
	if got, want := info.Mode().Perm(), fs.FileMode(0o600); got != want {
		t.Errorf("fixture mode = %v, want %v", got, want)
	}
}

// TestBody_IsTheBytesWriteStages pins the one thing a consumer staging its own
// file needs: Body() is byte-for-byte what Write would have put on disk.
//
// A consumer that writes the bytes itself — a decoy drop-in in a directory it
// then asserts about — otherwise hand-rolls the join, and a change to the file
// shape moves the writer while leaving that copy behind.
func TestBody_IsTheBytesWriteStages(t *testing.T) {
	path := themetest.Write(t, t.TempDir(), "nord-lee.theme", themetest.Lines())
	staged, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the written fixture: %v", err)
	}

	if got := string(themetest.Body()); got != string(staged) {
		t.Errorf("Body() =\n%q\nwant what Write stages:\n%q", got, staged)
	}
}

// requireReason fails unless the rejection carries exactly want.
func requireReason(t *testing.T, rejection *theme.Rejection, want theme.Reason) {
	t.Helper()

	if rejection == nil {
		t.Fatalf("the loader accepted the fixture, want the %q rejection", want)
	}
	if rejection.Reason != want {
		t.Errorf("rejection reason = %q (%s), want %q", rejection.Reason, rejection.Detail, want)
	}
}

// wantTokensFrom renders the tokens the given fixture lines must parse into: the
// same values in the loader's canonical upper case, in file order.
func wantTokensFrom(lines []string) []theme.Token {
	tokens := make([]theme.Token, 0, len(lines))
	for _, line := range lines {
		name, value, _ := strings.Cut(line, " = ")
		tokens = append(tokens, theme.Token{Name: name, Value: strings.ToUpper(value)})
	}
	return tokens
}

// tokenNamesFrom lists the keys the fixture lines declare, in file order.
func tokenNamesFrom(t *testing.T, lines []string) []string {
	t.Helper()

	names := make([]string, 0, len(lines))
	for _, line := range lines {
		name, _ := splitLine(t, line)
		names = append(names, name)
	}
	return names
}

// splitLine parses one fixture line into its key and value.
func splitLine(t *testing.T, line string) (string, string) {
	t.Helper()

	name, value, found := strings.Cut(line, " = ")
	if !found {
		t.Fatalf("fixture line %q is not a `key = value` pair", line)
	}
	return name, value
}
