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

// builtinsDir is the embedded set's directory, relative to the package source
// directory go test runs the binary in.
const builtinsDir = "builtins"

// tokyoNightSlug is the first built-in's slug — the stem of its committed
// filename, which the filename-is-identity rule makes its identity.
const tokyoNightSlug = "tokyo-night"

// tokyoNightPath is the committed source of the first built-in.
var tokyoNightPath = builtinPath(tokyoNightSlug)

// TestTokyoNightFile_HasNineteenKeysAndNoBorderFooter pins the shipped file's
// SHAPE, as distinct from the palette its values name.
//
// Three things are asserted because three separate decisions land in this one
// file: it declares exactly the 19 keys of the closed vocabulary (the full-replacement
// rule — no partial built-in), it carries NO `border.footer`
// (the border consolidation merged the two border tokens into one, so the file is 19 keys and
// not 20), and it opens with a `#` header, which is the attribution the flat
// format was chosen to carry and which `portal theme export` ships
// verbatim to a user copying it.
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
		t.Errorf("%s declares border.footer — §2.2 consolidated the border tokens into one", tokyoNightPath)
	}

	got, want := slices.Sorted(slices.Values(keys)), slices.Sorted(slices.Values(theme.TokenNames()))
	if !slices.Equal(got, want) {
		t.Errorf("%s declares keys %v, want the closed vocabulary %v", tokyoNightPath, got, want)
	}
}

// TestBuiltinBytes_MatchesCommittedFile pins the embedded bytes as the
// committed file's bytes, verbatim.
//
// It is the guard on the export contract's byte-faithful export: `portal theme export` prints
// what the loader validated, comments and trailing newline included, so a user
// copying a built-in gets the attribution header rather than a re-serialisation
// stripped of it. A comparison against the file on disk is what makes that a
// fact about the SHIPPED file rather than about the embedding.
//
// The copy check is the second half of the same promise: the caller owns the
// bytes it is handed, so mutating them can corrupt neither the embedded set nor
// the next caller's read.
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

// TestBuiltinSlugs_DerivedAndSorted pins the set as DERIVED from the embedded
// filenames rather than restated in Go.
//
// The expectation is read off the committed directory, so the assertion is that
// the two agree — which is what makes "adding a built-in is adding a file"
// literally true: a new .theme file appears in the set, reserves its own slug
// and is validated by the build-time guarantee without a line of
// Go changing. A hand-written list would pass this test only until someone
// forgot to extend it, at which point the file would be embedded and invisible.
//
// The sort is asserted as a property of the function, not inherited from the
// directory read: stripping a common suffix can reorder two names (`a-.theme`
// sorts before `a.theme`, while the stems sort the other way round), so a
// caller relying on order needs the guarantee to live here.
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

// TestBuiltins_AreEnrolledInFloorChecks pins every embedded built-in into the
// contrast gate's auto-enumerated floor set.
//
// Enrolment is what makes each floor a statement about EVERY shipped palette
// rather than about one reference theme: no floor is relaxed per built-in, and
// every leg is measured against that theme's own canvas by the same code. A
// palette that silently stopped being measured is the failure the bundled tier
// exists to prevent — it would look fine and could read badly, most acutely on a
// light theme, whose floors only mean anything against its own near-white, and
// on a mid-dark canvas, where several legs clear by hundredths.
//
// The expectation is read off the COMMITTED directory rather than off the same
// enumeration the floor set is built from, which is what leaves the assertion
// able to fail: a .theme file that is committed but never reaches the floor set
// — an embed pattern narrowed to a subset, say — is named here. Reading the
// enumerated set on both sides would compare it against itself.
//
// It names no theme, so a built-in added to builtins/ is enrolled here with no
// test edit — the same property that makes adding a theme adding a file.
func TestBuiltins_AreEnrolledInFloorChecks(t *testing.T) {
	enrolled := embeddedThemes(t)

	for _, slug := range committedBuiltinSlugs(t) {
		if _, isEnrolled := enrolled[slug]; !isEnrolled {
			t.Errorf("the floor tests enrol %v, want %q among them", slices.Sorted(maps.Keys(enrolled)), slug)
		}
	}
}

// committedBuiltinSlugs reads the built-in slugs off the committed directory —
// the independent expectation BuiltinSlugs() is checked against.
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

// wantTokyoNightTokens is the shipped Tokyo Night dark table — the values MV carried as its
// Dark variants — in canonical table order and in the hex-only value rule's canonical upper
// case.
//
// It is a deliberate second copy of the shipped file's values, and the only one
// in Go: these 19 hexes are what Portal looks like out of the box, so changing
// one is a change to the product and has to be made twice. Two of them
// (`canvas`, `bg.selection`) are written lower case in the file and appear here
// upper case, which is the canonicalisation the OSC 11 re-emission rule's background diffing
// and the exit-time restore rule's retained startup canvas hex both compare against.
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

// TestLoadBuiltin_TokyoNightValuesAreUppercaseCanonical pins the palette the
// dark built-in ships, and the case it ships it in.
//
// The whole 19-token slice is asserted in canonical table order, so a value edited in the
// file, a key wired to the wrong role, or a token quietly dropped all surface
// here rather than in a screenshot.
//
// The case half is not cosmetic. The file writes `canvas` and `bg.selection`
// lower case — verbatim as MV held them — and the parser canonicalises to upper,
// which is what lets the OSC 11 re-emission rule's background diffing and the exit-time
// restore rule's retained startup canvas hex compare hex strings at all: a theme written
// `#0b0c14` must not fail to match one written `#0B0C14`.
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

	// The two values the file writes lower case, read back off their fields —
	// the canonicalisation this test is named for, stated where it is legible.
	if want := "#0B0C14"; got.Theme.Canvas.Value != want {
		t.Errorf("Canvas.Value = %q, want the upper-case canonical %q", got.Theme.Canvas.Value, want)
	}
	if want := "#28243A"; got.Theme.BgSelection.Value != want {
		t.Errorf("BgSelection.Value = %q, want the upper-case canonical %q", got.Theme.BgSelection.Value, want)
	}
}

// TestLoadBuiltin_UsesTheSharedParsePath is the pin on the built-in-is-a-file rule's central
// claim: a built-in IS a theme file, parsed by the same loader as a stranger's drop-in.
//
// The proof is that the two callers cannot be told apart by behaviour. The
// embedded bytes are written out under an ordinary filename in an ordinary
// directory and loaded through LoadFile, and the palette that comes back is the
// palette LoadBuiltin returns — so there is no second parse path a format bug
// could hide behind, and no built-in enjoying a validity rule a user's file does
// not get.
//
// The slugs deliberately DIFFER, which is the other half of the same shape: the
// palette comes from the contents and the identity from the name, so the same
// bytes are `tokyo-night` embedded and `tokyo-night-copy` on disk. That is
// exactly the "copy a built-in and edit it" workflow the embedding decision
// exists for.
//
// Source is asserted on both, because export prints what the loader
// validated rather than re-reading the file: the bytes on the Result are the
// bytes that were parsed, comments and trailing newline included, on either
// route in.
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

// LoadBuiltin's signature, asserted at COMPILE time: one slug in, no directory
// anywhere on it.
//
// This is the structural half of "the built-in lookup touches no filesystem".
// A directory parameter would be the first step to config discovery inside
// internal/theme, which internal/capture's no-real-config import guard and
// the built-in fallback both depend on never happening — and unlike a runtime
// assertion, this one cannot be satisfied by a function that simply chose not to
// read the path it was given.
var _ func(string) (theme.Result, *theme.Rejection, bool) = theme.Loader{}.LoadBuiltin

// TestLoadBuiltin_UnknownSlugIsNotFound pins the outcome by-name
// resolution falls through on: not-found, with NO rejection.
//
// The distinction is the whole reason found is a separate return. A rejection
// would mean "this built-in is broken" — the thing the build-time guarantee's test
// makes impossible — whereas an unknown slug means only that the answer lies in the
// themes directory instead, which is where the resolver looks next. Collapsing
// the two would turn every user's own theme name into a reported failure of the
// built-in set.
//
// PORTAL_THEMES_DIR is pointed at a path that does not exist for the duration.
// If the lookup ever grew a filesystem leg, the env var is the route it would
// most plausibly take, and this makes taking it fatal rather than invisible.
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

// firstNonBlankLine returns the file's first line carrying anything at all.
func firstNonBlankLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

// themeFileKeys returns the key of every `key = value` line in a theme file's
// text, in file order.
//
// It is a deliberately independent reading of the file rather than a call into
// the lexer: this is the guard on what the shipped file DECLARES, so parsing it
// with the same code the file is meant to satisfy would make the assertion
// circular. Any line that is neither blank, a comment nor a pair fails the test
// outright.
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
