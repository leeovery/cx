package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// TestForEachBuiltinTheme_RunsTheShippedPair pins the palette iterator as the
// suite's enumeration of "both built-ins": two subtests, named dark then light,
// carrying the palettes internal/theme's shipped-default slugs resolve to.
//
// The subtest NAMES are asserted through t.Name() rather than trusted, because
// every call site inherits them — a rename here renames subtests across the
// whole render suite and breaks any -run filter aimed at them.
func TestForEachBuiltinTheme_RunsTheShippedPair(t *testing.T) {
	var gotNames []string
	var gotThemes []theme.Theme

	forEachBuiltinTheme(t, func(t *testing.T, th theme.Theme) {
		_, sub, _ := strings.Cut(t.Name(), "/")
		gotNames = append(gotNames, sub)
		gotThemes = append(gotThemes, th)
	})

	wantNames := []string{"dark", "light"}
	if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("subtest names = %v, want %v", gotNames, wantNames)
	}
	wantThemes := []theme.Theme{themetest.DefaultDark(t), themetest.DefaultLight(t)}
	for i, want := range wantThemes {
		if gotThemes[i] != want {
			t.Errorf("%s subtest ran against %s, want the %s built-in", wantNames[i], themeLabel(gotThemes[i]), wantNames[i])
		}
	}
}

// TestForEachCanvasMode_RunsBothModes is TestForEachBuiltinTheme_RunsTheShippedPair
// for the sites parameterised by the answer in force rather than by a palette.
func TestForEachCanvasMode_RunsBothModes(t *testing.T) {
	var gotNames []string
	var gotModes []theme.Member

	forEachCanvasMode(t, func(t *testing.T, m theme.Member) {
		_, sub, _ := strings.Cut(t.Name(), "/")
		gotNames = append(gotNames, sub)
		gotModes = append(gotModes, m)
	})

	wantNames := []string{"dark", "light"}
	if strings.Join(gotNames, ",") != strings.Join(wantNames, ",") {
		t.Fatalf("subtest names = %v, want %v", gotNames, wantNames)
	}
	wantModes := []theme.Member{theme.MemberDark, theme.MemberLight}
	for i, want := range wantModes {
		if gotModes[i] != want {
			t.Errorf("%s subtest ran against %v, want %v", wantNames[i], gotModes[i], want)
		}
	}
}

// twoBuiltinTableDeclarer is the file holding the pair's single declaration,
// which is the one place the guard below must not object to.
const twoBuiltinTableDeclarer = "theme_testing_test.go"

// twoBuiltinRow matches one row of a local dark/light table over the two shipped
// built-ins: a name field holding "dark" and a single value field holding the dark
// palette or the dark canvas mode, and NOTHING else in the row.
//
// The trailing brace is what keeps the rule to this table alone. A row carrying a
// further column — a colourless flag, a golden string, a third case — is a table
// over something else that happens to include the pair, which the iterators do not
// replace.
var twoBuiltinRow = regexp.MustCompile(`\{(?:\w+:\s*)?"dark",\s*(?:\w+:\s*)?(?:testDarkTheme\(t\)|theme\.MemberDark)\}`)

// TestNoLocalTwoBuiltinTable fails if any internal/tui test file spells out its
// own dark/light table over the two shipped built-ins.
//
// Copied per file, that table makes "which themes does the render suite run
// against" a fact restated once per site: a change to the shipped pair becomes an
// edit at every one of them, and a site left behind runs against a theme the rest
// of the suite has moved off with nothing to notice it.
func TestNoLocalTwoBuiltinTable(t *testing.T) {
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("read the internal/tui package dir: %v", err)
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, "_test.go") || name == twoBuiltinTableDeclarer {
			continue
		}
		src, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		for i, line := range strings.Split(string(src), "\n") {
			if twoBuiltinRow.MatchString(line) {
				t.Errorf("%s:%d declares a local two-built-in table; take the pair from forEachBuiltinTheme/forEachCanvasMode instead:\n\t%s",
					name, i+1, strings.TrimSpace(line))
			}
		}
	}
}
