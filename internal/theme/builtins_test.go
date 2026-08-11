package theme_test

import (
	"bytes"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

const builtinsDir = "builtins"

const tokyoNightSlug = "tokyo-night"

var tokyoNightPath = builtinPath(tokyoNightSlug)

func TestTokyoNightFile_HasNineteenKeysAndNoBorderFooter(t *testing.T) {
	text := readBuiltinFile(t, tokyoNightSlug)
	if first := firstNonBlankLine(text); !strings.HasPrefix(first, "#") {
		t.Errorf("%s opens with %q, want a # header comment naming the palette and its source", tokyoNightPath, first)
	}

	keys := themeFileKeys(t, text)
	if want := len(theme.TokenNames()); len(keys) != want {
		t.Errorf("%s declares %d keys, want %d", tokyoNightPath, len(keys), want)
	}
	if slices.Contains(keys, "border.footer") {
		t.Errorf("%s declares border.footer — the border tokens are consolidated into one", tokyoNightPath)
	}

	got, want := slices.Sorted(slices.Values(keys)), slices.Sorted(slices.Values(theme.TokenNames()))
	if !slices.Equal(got, want) {
		t.Errorf("%s declares keys %v, want the closed vocabulary %v", tokyoNightPath, got, want)
	}
}

func TestBuiltinBytes_MatchesCommittedFile(t *testing.T) {
	committed := []byte(readBuiltinFile(t, tokyoNightSlug))

	t.Run("byte-identical to the committed file", func(t *testing.T) {
		got, found := theme.BuiltinBytes(tokyoNightSlug)

		if !found {
			t.Fatalf("BuiltinBytes(%q) reported not found, want the embedded file", tokyoNightSlug)
		}
		if !bytes.Equal(got, committed) {
			t.Errorf("BuiltinBytes(%q) =\n%s\nwant the committed %s:\n%s", tokyoNightSlug, got, tokyoNightPath, committed)
		}
	})

	t.Run("returns a fresh copy the caller may mutate", func(t *testing.T) {
		first, _ := theme.BuiltinBytes(tokyoNightSlug)
		if len(first) == 0 {
			t.Fatalf("BuiltinBytes(%q) returned no bytes", tokyoNightSlug)
		}
		first[0] = 'X'

		second, _ := theme.BuiltinBytes(tokyoNightSlug)

		if !bytes.Equal(second, committed) {
			t.Errorf("a mutation of one caller's bytes reached the next read: got\n%s\nwant\n%s", second, committed)
		}
	})

	t.Run("unknown slug yields no bytes", func(t *testing.T) {
		got, found := theme.BuiltinBytes("no-such-theme")

		if found {
			t.Errorf("BuiltinBytes(%q) reported found, want not found", "no-such-theme")
		}
		if got != nil {
			t.Errorf("BuiltinBytes(%q) = %q, want nil alongside not-found", "no-such-theme", got)
		}
	})
}

func TestBuiltinSlugs_DerivedAndSorted(t *testing.T) {
	got := theme.BuiltinSlugs()

	if want := committedBuiltinSlugs(t); !slices.Equal(got, want) {
		t.Errorf("BuiltinSlugs() = %v, want the committed set %v", got, want)
	}
	if !slices.IsSorted(got) {
		t.Errorf("BuiltinSlugs() = %v, want them sorted", got)
	}
	if !slices.Contains(got, tokyoNightSlug) {
		t.Errorf("BuiltinSlugs() = %v, want it to contain %q", got, tokyoNightSlug)
	}
}

func TestBuiltins_AreEnrolledInFloorChecks(t *testing.T) {
	enrolled := embeddedThemes(t)

	for _, slug := range committedBuiltinSlugs(t) {
		if _, isEnrolled := enrolled[slug]; !isEnrolled {
			t.Errorf("the floor tests enrol %v, want %q among them", slices.Sorted(maps.Keys(enrolled)), slug)
		}
	}
}

func committedBuiltinSlugs(t *testing.T) []string {
	t.Helper()

	entries, err := os.ReadDir(builtinsDir)
	if err != nil {
		t.Fatalf("read %s: %v", builtinsDir, err)
	}

	slugs := []string{}
	for _, entry := range entries {
		stem, isTheme := strings.CutSuffix(entry.Name(), ".theme")
		if !isTheme {
			continue
		}
		slugs = append(slugs, stem)
	}
	if len(slugs) == 0 {
		t.Fatalf("%s holds no .theme files — the expectation would be vacuous", builtinsDir)
	}

	slices.Sort(slugs)
	return slugs
}

var wantTokyoNightTokens = []theme.Token{
	{Name: "text.primary", Value: "#C0CAF5"},
	{Name: "text.secondary", Value: "#A9B1D6"},
	{Name: "text.tertiary", Value: "#828BB8"},
	{Name: "text.muted", Value: "#737AA2"},
	{Name: "text.subtle", Value: "#535C86"},
	{Name: "text.faint", Value: "#3B4261"},
	{Name: "text.on-selection", Value: "#FFFFFF"},
	{Name: "accent.primary", Value: "#BB9AF7"},
	{Name: "accent.key", Value: "#7AA2F7"},
	{Name: "accent.mode", Value: "#7DCFFF"},
	{Name: "accent.attention", Value: "#FF9E64"},
	{Name: "state.positive", Value: "#9ECE6A"},
	{Name: "state.destructive", Value: "#F7768E"},
	{Name: "canvas", Value: "#0B0C14"},
	{Name: "bg.selection", Value: "#28243A"},
	{Name: "bg.attention", Value: "#241B10"},
	{Name: "bg.subtle", Value: "#26283A"},
	{Name: "border", Value: "#292E42"},
	{Name: "text.on-attention", Value: "#E8C9A0"},
}

func TestLoadBuiltin_TokyoNightValuesAreUppercaseCanonical(t *testing.T) {
	got, rejection, found := theme.Loader{}.LoadBuiltin(tokyoNightSlug)

	if rejection != nil {
		t.Fatalf("LoadBuiltin(%q) rejected the embedded file: %v", tokyoNightSlug, rejection)
	}
	if !found {
		t.Fatalf("LoadBuiltin(%q) reported not found, want the embedded built-in", tokyoNightSlug)
	}
	if got.Slug != tokyoNightSlug {
		t.Errorf("slug = %q, want %q", got.Slug, tokyoNightSlug)
	}
	if tokens := got.Theme.All(); !slices.Equal(tokens, wantTokyoNightTokens) {
		t.Errorf("theme = %+v, want %+v", tokens, wantTokyoNightTokens)
	}

	if want := "#0B0C14"; got.Theme.Canvas.Value != want {
		t.Errorf("Canvas.Value = %q, want the upper-case canonical %q", got.Theme.Canvas.Value, want)
	}
	if want := "#28243A"; got.Theme.BgSelection.Value != want {
		t.Errorf("BgSelection.Value = %q, want the upper-case canonical %q", got.Theme.BgSelection.Value, want)
	}
}

func TestLoadBuiltin_UsesTheSharedParsePath(t *testing.T) {
	committed := []byte(readBuiltinFile(t, tokyoNightSlug))

	embedded, rejection, found := theme.Loader{}.LoadBuiltin(tokyoNightSlug)
	if rejection != nil || !found {
		t.Fatalf("LoadBuiltin(%q) = (rejection %v, found %t), want the embedded built-in", tokyoNightSlug, rejection, found)
	}

	copyPath := filepath.Join(t.TempDir(), tokyoNightSlug+"-copy.theme")
	if err := os.WriteFile(copyPath, committed, 0o644); err != nil {
		t.Fatalf("write %s: %v", copyPath, err)
	}

	dropIn, rejection := theme.Loader{}.LoadFile(copyPath)
	if rejection != nil {
		t.Fatalf("LoadFile(%q) rejected a copy of the shipped built-in: %v", copyPath, rejection)
	}

	if !slices.Equal(embedded.Theme.All(), dropIn.Theme.All()) {
		t.Errorf("LoadBuiltin() parsed %+v but LoadFile() parsed %+v from the same bytes — there are two parse paths", embedded.Theme.All(), dropIn.Theme.All())
	}
	if want := tokyoNightSlug + "-copy"; dropIn.Slug != want {
		t.Errorf("LoadFile() slug = %q, want %q — the identity comes from the filename, not the contents", dropIn.Slug, want)
	}
	if !bytes.Equal(embedded.Source, committed) {
		t.Errorf("LoadBuiltin() Source =\n%s\nwant the parsed bytes:\n%s", embedded.Source, committed)
	}
	if !bytes.Equal(dropIn.Source, committed) {
		t.Errorf("LoadFile() Source =\n%s\nwant the parsed bytes:\n%s", dropIn.Source, committed)
	}
}

var _ func(string) (theme.Result, *theme.Rejection, bool) = theme.Loader{}.LoadBuiltin

func TestLoadBuiltin_UnknownSlugIsNotFound(t *testing.T) {
	t.Setenv("PORTAL_THEMES_DIR", filepath.Join(t.TempDir(), "no-such-directory"))

	got, rejection, found := theme.Loader{}.LoadBuiltin("no-such-theme")

	if found {
		t.Errorf("LoadBuiltin(%q) reported found, want not found", "no-such-theme")
	}
	if rejection != nil {
		t.Errorf("LoadBuiltin(%q) rejected with %v, want no rejection — an unknown slug is not a broken built-in", "no-such-theme", rejection)
	}
	if !reflect.DeepEqual(got, theme.Result{}) {
		t.Errorf("LoadBuiltin(%q) = %+v, want the zero Result", "no-such-theme", got)
	}
}

func firstNonBlankLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func themeFileKeys(t *testing.T, text string) []string {
	t.Helper()

	keys := []string{}
	for index, raw := range strings.Split(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, _, separated := strings.Cut(line, "=")
		if !separated {
			t.Fatalf("line %d is neither blank, a comment nor a key = value pair: %q", index+1, raw)
		}
		keys = append(keys, strings.TrimSpace(key))
	}
	return keys
}
