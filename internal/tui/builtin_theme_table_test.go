package tui

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/sourceguardtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

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

const twoBuiltinTableDeclarer = "theme_testing_test.go"

var twoBuiltinRow = regexp.MustCompile(`\{(?:\w+:\s*)?"dark",\s*(?:\w+:\s*)?(?:testDarkTheme\(t\)|theme\.MemberDark)\}`)

func TestNoLocalTwoBuiltinTable(t *testing.T) {
	// The shared primitive errors on an empty match, so this cannot pass by having
	// stopped looking.
	paths, err := sourceguardtest.PackageGoFiles(".", true)
	if err != nil {
		t.Fatalf("enumerate the internal/tui package sources: %v", err)
	}
	for _, path := range paths {
		name := filepath.Base(path)
		if !strings.HasSuffix(name, "_test.go") || name == twoBuiltinTableDeclarer {
			continue
		}
		src, err := os.ReadFile(path)
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
