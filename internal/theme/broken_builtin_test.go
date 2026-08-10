package theme_test

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

// TestBrokenBuiltinError_CopyIsPinned pins the pinned copy's fatal startup sentence
// verbatim, em dash included.
//
// The `want` strings below are TRANSCRIBED FROM THE SPECIFICATION and are
// deliberately not composed from the package's own format string — a test that
// built its expectation out of the thing under test would pass whatever the copy
// drifted to. That is the whole of the assertion: two independent statements of
// one sentence, so an edit to either side fails.
//
// Terse is right for a path the build-time guarantee makes unreachable,
// but it is still new user-facing copy, so it is pinned rather than left
// implicit — the one line a user gets instead of a Go panic trace.
func TestBrokenBuiltinError_CopyIsPinned(t *testing.T) {
	tests := []struct {
		name string
		slug string
		want string
	}{
		{
			name: "the dark fallback",
			slug: theme.DefaultDarkSlug,
			want: "built-in theme tokyo-night is missing or invalid — this binary is broken",
		},
		{
			name: "the light fallback",
			slug: theme.DefaultLightSlug,
			want: "built-in theme tokyo-night-day is missing or invalid — this binary is broken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := theme.BrokenBuiltinError(tt.slug)
			if err == nil {
				t.Fatalf("BrokenBuiltinError(%q) = nil, want §14A's pinned sentence", tt.slug)
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("BrokenBuiltinError(%q) =\n %q\nwant %q", tt.slug, got, tt.want)
			}
		})
	}
}

// TestFallback_MissingBuiltinIsFatal pins the build-time guarantee's one genuinely fatal state
// at the level the per-slot fallback rule leaves open: the MESSAGE.
//
// The theme a slot falls back to cannot be loaded from the embedded set, so
// there is nothing honest left to paint — the build-time guarantee removed the safety net
// beneath this point on purpose, rejecting "a compiled-in last-resort palette equal to
// Tokyo Night Dark" in favour of a build-time guarantee. What the user gets is
// therefore one printed line, and this is that line.
//
// Each row asserts BOTH halves the criterion asks for: the literal pinned sentence
// naming the fallback slug, and that the same text comes out of the exported
// single source. Either drifting fails — a reworded format string fails the
// literal, and an ad-hoc fmt.Errorf at the return site fails the single-source
// cross-check.
//
// The failure is fatal ONLY BECAUSE A FALLBACK IS WHAT IS MISSING. A nomination
// failing is ordinary and absorbed silently — see
// TestFallback_NominationFailureIsNotFatal, which drives the identical broken
// nomination against a healthy embedded set and gets a Resolution back.
func TestFallback_MissingBuiltinIsFatal(t *testing.T) {
	tests := []struct {
		name     string
		fallback string
		setting  theme.Setting
		want     string
	}{
		{
			name:     "a constant whose dark fallback is missing",
			fallback: theme.DefaultDarkSlug,
			setting:  theme.Setting{IsConstant: true, Constant: "gone"},
			want:     "built-in theme tokyo-night is missing or invalid — this binary is broken",
		},
		{
			name:     "a light slot whose light fallback is missing",
			fallback: theme.DefaultLightSlug,
			setting:  theme.Setting{Light: "gone-light", Dark: "nord"},
			want:     "built-in theme tokyo-night-day is missing or invalid — this binary is broken",
		},
		{
			name:     "a dark slot whose dark fallback is missing",
			fallback: theme.DefaultDarkSlug,
			setting:  theme.Setting{Light: "nord", Dark: "gone-dark"},
			want:     "built-in theme tokyo-night is missing or invalid — this binary is broken",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := nominationLoader()
			loader.BuiltinSource = withoutBuiltin(tt.fallback)

			got, err := loader.ResolveNomination(tt.setting, t.TempDir())

			if err == nil {
				t.Fatalf("ResolveNomination(%+v) = %+v, want §14A's fatal — the fallback itself did not resolve", tt.setting, got)
			}
			if err.Error() != tt.want {
				t.Errorf("message =\n %q\nwant %q", err.Error(), tt.want)
			}
			if single := theme.BrokenBuiltinError(tt.fallback).Error(); single != tt.want {
				t.Errorf("BrokenBuiltinError(%q) = %q, want %q — the sentence is single-sourced, so the returned error and the exported source must be the same copy", tt.fallback, single, tt.want)
			}
			requireZeroResolution(t, got)
		})
	}
}

// TestFallback_CorruptBuiltinIsFatal asserts the message DOES NOT VARY BY
// REASON: a fallback file that is present and unparseable produces the same
// sentence an absent one does.
//
// The per-slot fallback rule's "one not-loadable path serves every cause" applies to the
// fallback as much as to the nomination. The user cannot act on the distinction either way —
// the binary is broken, and which of the two ways it is broken changes nothing
// they can do — so a second sentence would be copy with no reader.
func TestFallback_CorruptBuiltinIsFatal(t *testing.T) {
	const want = "built-in theme tokyo-night is missing or invalid — this binary is broken"

	loader := nominationLoader()
	loader.BuiltinSource = corruptBuiltin(theme.DefaultDarkSlug)
	setting := theme.Setting{IsConstant: true, Constant: "gone"}

	got, err := loader.ResolveNomination(setting, t.TempDir())

	if err == nil {
		t.Fatalf("ResolveNomination(%+v) = %+v, want §14A's fatal — the fallback parsed as invalid", setting, got)
	}
	if err.Error() != want {
		t.Errorf("message =\n %q\nwant %q — the sentence does not vary by reason", err.Error(), want)
	}
	requireZeroResolution(t, got)
}

// TestFallback_NeverPanics asserts the escalation is an ORDINARY ERROR RETURNED
// UP THE NORMAL PATH rather than a panic.
//
// It RECOVERS NOTHING, deliberately: there is no deferred recover here, so a
// panic on this path takes the test binary down and reports as a failure with
// the trace attached. A recover would turn the very outcome under test into a
// passing assertion.
//
// main.go's panic-recovering exit and its `process: panic` lifecycle marker stay
// the backstop for a GENUINE PROGRAMMING FAULT, not the designed route for a
// binary whose embedded set is broken. The source half of the claim — that
// nothing in the package panics, exits or calls log.Fatal — is
// TestEmbeddedRejection_HasNoFatalPathInThePackage.
func TestFallback_NeverPanics(t *testing.T) {
	loader := nominationLoader()
	loader.BuiltinSource = withoutBuiltin(theme.DefaultDarkSlug)

	_, err := loader.ResolveNomination(theme.Setting{IsConstant: true, Constant: "gone"}, t.TempDir())

	if err == nil {
		t.Fatal("ResolveNomination succeeded with its fallback removed, want an error — a returned error is the whole mechanism")
	}
}

// TestFallback_NominationFailureIsNotFatal draws the line the fatal depends on:
// a NOMINATION that will not load is ordinary and is absorbed silently by the
// fallback; only a FALLBACK that will not load is fatal.
//
// The same broken setting drives both, and the only difference is whether the
// embedded set can still supply the fallback. Without this pairing, "the
// resolution returned an error" would be evidence for nothing in particular —
// the two states are one `if` apart in resolveSlot, and confusing them would
// make every typo in prefs.json a fatal.
func TestFallback_NominationFailureIsNotFatal(t *testing.T) {
	setting := theme.Setting{IsConstant: true, Constant: "gone"}

	got, err := nominationLoader().ResolveNomination(setting, t.TempDir())
	if err != nil {
		t.Fatalf("ResolveNomination(%+v) = %v, want a fallback — only a broken FALLBACK is fatal", setting, err)
	}

	if len(got.Slots) != 1 {
		t.Fatalf("Slots = %+v, want exactly one record for a constant", got.Slots)
	}
	slot := got.Slots[0]
	if !slot.FellBack {
		t.Error("FellBack = false, want true — the nomination did not load")
	}
	if slot.Requested != "gone" {
		t.Errorf("Requested = %q, want %q — the persisted name is kept", slot.Requested, "gone")
	}
	if slot.Resolved != theme.DefaultDarkSlug {
		t.Errorf("Resolved = %q, want %q", slot.Resolved, theme.DefaultDarkSlug)
	}
	if want := themetest.Builtin(t, theme.DefaultDarkSlug); slot.Theme != want {
		t.Error("the slot's palette is not the dark default's — a fallback paints the fallback's theme")
	}
	if got.Nomination.Constant() != slot.Theme {
		t.Error("the nomination does not carry the slot's palette — a fallen-back resolution is still a complete one")
	}
}

// TestResolution_NoStartupEagerValidation is the build-time guarantee's NEGATIVE assertion:
// nothing walks the embedded set.
//
// Validation is not startup-eager by decision — the build-time guarantee's test
// already proves the set, and re-proving it on every launch would put a parse of every
// built-in on the one cold path this feature otherwise adds no cost to. The
// checkable form is a counting byte source: a resolution reads EXACTLY the
// built-ins it was asked about, one under a constant and two under a pair, and
// never the set.
//
// The count is of BYTE READS — parses — and not of name enumerations. NewLoader
// derives the reserved-slug rule's reserved-slug set from the embedded FILENAMES, which reads
// no file's contents and is what the counting source therefore cannot see.
//
// The third case is the sharp one: a nomination that is NOT a built-in still
// costs exactly one built-in read (the miss that sends it to the themes
// directory), never a sweep of the set looking for it.
//
// The remaining half of the claim — zero reads on the exec path — is
// cmd's TestOpenExecPath_DoesNoThemeWork, which guards that `portal open
// <target>` constructs no TUI and reaches no theme call site at all.
func TestResolution_NoStartupEagerValidation(t *testing.T) {
	dir := t.TempDir()
	themetest.Write(t, dir, "mine.theme", themetest.Lines())

	tests := []struct {
		name    string
		setting theme.Setting
		want    []string
	}{
		{
			name:    "a constant reads one built-in",
			setting: theme.Setting{IsConstant: true, Constant: "nord"},
			want:    []string{"nord"},
		},
		{
			name:    "a pair reads two",
			setting: theme.Setting{Light: theme.DefaultLightSlug, Dark: theme.DefaultDarkSlug},
			want:    []string{theme.DefaultLightSlug, theme.DefaultDarkSlug},
		},
		{
			name:    "a drop-in nomination still reads only its own slug",
			setting: theme.Setting{IsConstant: true, Constant: "mine"},
			want:    []string{"mine"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var asked []string
			loader := nominationLoader()
			loader.BuiltinSource = func(slug string) ([]byte, bool) {
				asked = append(asked, slug)
				return theme.BuiltinBytes(slug)
			}

			if _, err := loader.ResolveNomination(tt.setting, dir); err != nil {
				t.Fatalf("ResolveNomination(%+v) = %v", tt.setting, err)
			}

			if !slices.Equal(asked, tt.want) {
				t.Errorf("built-ins read = %v, want %v — nothing walks the embedded set", asked, tt.want)
			}
		})
	}
}

// TestTheme_NoCompiledInFallbackPalette asserts no production file in
// internal/theme declares a populated Theme.
//
// The build-time guarantee rejected "a compiled-in last-resort palette equal to Tokyo Night
// Dark" in those words — a build-time guarantee beats a runtime crutch — so the fatal
// path above must never grow one later. The temptation sits exactly where the
// error is returned: the one place that discovers the embedded set cannot supply
// a fallback is also the one place a hardcoded palette would look like mercy
// rather than a second, unspecified source of colour values.
//
// It pairs with TestThemePackage_DeclaresNoHexLiterals, and neither subsumes the
// other: that one forbids the VALUES, this one forbids the STRUCTURE. A palette
// assembled from constants declared elsewhere would clear the hex guard and be
// caught here.
//
// A ZERO Theme{} is not a palette and is allowed — it is what every rejection
// returns alongside its reason, and returning one is how the package says "there
// is nothing to paint".
func TestTheme_NoCompiledInFallbackPalette(t *testing.T) {
	for _, source := range parseThemeSources(t) {
		ast.Inspect(source.File, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || len(lit.Elts) == 0 {
				return true
			}
			if !isThemeTypeExpr(lit.Type) {
				return true
			}
			t.Errorf("%s:%d declares a populated Theme literal — there is no runtime last-resort palette beneath the built-in fallback; §7.6 replaced that crutch with a build-time guarantee", source.Name, source.Fset.Position(lit.Pos()).Line)
			return true
		})
	}
}

// isThemeTypeExpr reports whether the composite literal's type is Theme, in
// either of the two spellings a package-local literal can take: `Theme{…}` and
// `&Theme{…}` (whose inner literal carries the same Ident type).
func isThemeTypeExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "Theme"
}

// TestBuiltinSource_DefaultsToTheEmbeddedSet asserts the nil field — the shape
// every production caller carries — reads the embedded set.
//
// It is the half TestBuiltinSource_HasNoProductionCallSite depends on and
// cannot state: that guard proves nothing in production ever SETS the field,
// which is only safe because the unset field already means "the embedded set".
// Together they are the criterion in full — the seam costs a call site nothing,
// because there is no call site.
//
// The bytes are compared, not just the palette: LoadBuiltin's Source is what
// `portal theme export` writes, so a default that parsed the right values from
// somewhere else would still be the wrong default.
func TestBuiltinSource_DefaultsToTheEmbeddedSet(t *testing.T) {
	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("BuiltinSlugs() is empty — the assertion below would be vacuous")
	}

	loader := nominationLoader()
	if loader.BuiltinSource != nil {
		t.Fatal("the production loader carries a BuiltinSource — the field's zero value is the production one, and NewLoader must not populate it")
	}

	for _, slug := range slugs {
		result, rejection, found := loader.LoadBuiltin(slug)
		if !found || rejection != nil {
			t.Fatalf("LoadBuiltin(%q) = (found %t, %v), want the embedded palette", slug, found, rejection)
		}
		want, ok := theme.BuiltinBytes(slug)
		if !ok {
			t.Fatalf("BuiltinBytes(%q) reports not found — the embedded set does not carry a slug it enumerated", slug)
		}
		if !bytes.Equal(result.Source, want) {
			t.Errorf("LoadBuiltin(%q) parsed %d bytes, want the embedded file's %d — a nil BuiltinSource reads BuiltinBytes", slug, len(result.Source), len(want))
		}
	}
}

// builtinSourceOwners are the only two production files allowed to name
// Loader.BuiltinSource: the one that declares the field and the one that reads
// it.
var builtinSourceOwners = map[string]bool{
	filepath.Join("internal", "theme", "load.go"):     true,
	filepath.Join("internal", "theme", "builtins.go"): true,
}

// TestBuiltinSource_HasNoProductionCallSite asserts the injectable built-in byte
// source is reached by TESTS ONLY.
//
// The seam exists BECAUSE the path it stages is unreachable: a correctly built
// binary cannot reach the build-time guarantee's broken-fallback state, and an unreachable
// fatal with no test is a path nobody has ever run. It costs production one nil check
// per built-in read and nothing else.
//
// What it must never become is a production knob. The build-time guarantee's whole guarantee
// is that "built-in" means the embedded set — the files the build-time test parses and
// validates — so a production call site wiring this field would redefine the
// term on the exact path the guarantee is about, and every assertion resting on
// it would still pass. The scan is repo-wide rather than package-local because
// the field is exported: a `loader.BuiltinSource = …` in cmd is the shape this
// forbids, and it is invisible from inside internal/theme.
func TestBuiltinSource_HasNoProductionCallSite(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	fset := token.NewFileSet()
	found := 0

	paths, err := portalbintest.GoSourceFiles(root)
	if err != nil {
		t.Fatalf("enumerate .go files: %v", err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			t.Fatalf("relativise %s: %v", path, relErr)
		}
		if builtinSourceOwners[rel] {
			found++
			continue
		}

		file, parseErr := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if parseErr != nil {
			t.Fatalf("parse %s: %v", rel, parseErr)
		}
		ast.Inspect(file, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "BuiltinSource" {
				return true
			}
			t.Errorf("%s:%d references Loader.BuiltinSource — the seam is test-only; a production call site would redefine what \"built-in\" means on the very path §7.6's build-time guarantee covers", rel, fset.Position(sel.Pos()).Line)
			return true
		})
	}

	if found != len(builtinSourceOwners) {
		t.Fatalf("found %d of the %d files that own BuiltinSource — the exemption list names a file that no longer exists, so the scan is not covering what it claims", found, len(builtinSourceOwners))
	}
}
