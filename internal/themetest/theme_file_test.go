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

func TestWithValue_ProducesTheBadColourRejection(t *testing.T) {
	path := themetest.Write(t, t.TempDir(), "nord-lee.theme", themetest.WithValue(themetest.Lines(), "canvas", "blue"))

	_, rejection := theme.Loader{}.LoadFile(path)

	requireReason(t, rejection, theme.ReasonBadColour)
}

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

func TestWithDuplicateKeyAt_ProducesTheBadSyntaxRejection(t *testing.T) {
	const at = 12
	lines := themetest.WithDuplicateKeyAt(themetest.Lines(), "text.primary", at)
	path := themetest.Write(t, t.TempDir(), "nord-lee.theme", lines)

	_, rejection := theme.Loader{}.LoadFile(path)

	requireReason(t, rejection, theme.ReasonBadSyntax)
	if want := "line 12: duplicate key text.primary"; rejection.Detail != want {
		t.Errorf("rejection detail = %q, want %q", rejection.Detail, want)
	}
	if got, want := len(lines), len(themetest.Lines())+1; got != want {
		t.Errorf("WithDuplicateKeyAt returned %d lines, want %d — it must add exactly the one line", got, want)
	}
	if names := tokenNamesFrom(t, lines); names[at-1] != "text.primary" {
		t.Errorf("line %d declares %q, want the duplicated text.primary", at, names[at-1])
	}
}

func TestWithDuplicateKeyAt_LeavesAnUndeclaredKeyAlone(t *testing.T) {
	lines := themetest.Lines()

	got := themetest.WithDuplicateKeyAt(lines, "no.such.token", 3)

	if !slices.Equal(got, lines) {
		t.Errorf("duplicating an undeclared key changed the lines:\n got %v\nwant %v", got, lines)
	}
}

func TestMutatorsLeaveTheirInputAlone(t *testing.T) {
	base := themetest.Lines()
	before := slices.Clone(base)

	themetest.WithValue(base, "canvas", "blue")
	themetest.WithoutKey(base, "bg.subtle")
	themetest.WithDuplicateKeyAt(base, "text.primary", 12)

	if !slices.Equal(base, before) {
		t.Errorf("the mutators wrote through to their input:\n got %v\nwant %v", base, before)
	}
}

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

func TestRender_IsTheBytesWriteStages(t *testing.T) {
	lines := themetest.WithValue(themetest.WithoutKey(themetest.Lines(), "bg.subtle"), "canvas", "blue")
	path := themetest.Write(t, t.TempDir(), "nord-lee.theme", lines)
	staged, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read the written fixture: %v", err)
	}

	if got := string(themetest.Render(lines)); got != string(staged) {
		t.Errorf("Render() =\n%q\nwant what Write stages:\n%q", got, staged)
	}
}

func TestBody_IsRenderOfLines(t *testing.T) {
	if got, want := string(themetest.Body()), string(themetest.Render(themetest.Lines())); got != want {
		t.Errorf("Body() =\n%q\nwant Render(Lines()):\n%q", got, want)
	}
}

func requireReason(t *testing.T, rejection *theme.Rejection, want theme.Reason) {
	t.Helper()

	if rejection == nil {
		t.Fatalf("the loader accepted the fixture, want the %q rejection", want)
	}
	if rejection.Reason != want {
		t.Errorf("rejection reason = %q (%s), want %q", rejection.Reason, rejection.Detail, want)
	}
}

func wantTokensFrom(lines []string) []theme.Token {
	tokens := make([]theme.Token, 0, len(lines))
	for _, line := range lines {
		name, value, _ := strings.Cut(line, " = ")
		tokens = append(tokens, theme.Token{Name: name, Value: strings.ToUpper(value)})
	}
	return tokens
}

func tokenNamesFrom(t *testing.T, lines []string) []string {
	t.Helper()

	names := make([]string, 0, len(lines))
	for _, line := range lines {
		name, _ := splitLine(t, line)
		names = append(names, name)
	}
	return names
}

func splitLine(t *testing.T, line string) (string, string) {
	t.Helper()

	name, value, found := strings.Cut(line, " = ")
	if !found {
		t.Fatalf("fixture line %q is not a `key = value` pair", line)
	}
	return name, value
}
