package theme_test

import (
	"bytes"
	"go/ast"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/portalbintest"
	"github.com/leeovery/portal/internal/sourceguardtest"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

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
				t.Fatalf("BrokenBuiltinError(%q) = nil, want the pinned sentence", tt.slug)
			}
			if got := err.Error(); got != tt.want {
				t.Errorf("BrokenBuiltinError(%q) =\n %q\nwant %q", tt.slug, got, tt.want)
			}
		})
	}
}

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
				t.Fatalf("ResolveNomination(%+v) = %+v, want the pinned fatal — the fallback itself did not resolve", tt.setting, got)
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

func TestFallback_CorruptBuiltinIsFatal(t *testing.T) {
	const want = "built-in theme tokyo-night is missing or invalid — this binary is broken"

	loader := nominationLoader()
	loader.BuiltinSource = corruptBuiltin(theme.DefaultDarkSlug)
	setting := theme.Setting{IsConstant: true, Constant: "gone"}

	got, err := loader.ResolveNomination(setting, t.TempDir())

	if err == nil {
		t.Fatalf("ResolveNomination(%+v) = %+v, want the pinned fatal — the fallback parsed as invalid", setting, got)
	}
	if err.Error() != want {
		t.Errorf("message =\n %q\nwant %q — the sentence does not vary by reason", err.Error(), want)
	}
	requireZeroResolution(t, got)
}

func TestFallback_NeverPanics(t *testing.T) {
	loader := nominationLoader()
	loader.BuiltinSource = withoutBuiltin(theme.DefaultDarkSlug)

	_, err := loader.ResolveNomination(theme.Setting{IsConstant: true, Constant: "gone"}, t.TempDir())

	if err == nil {
		t.Fatal("ResolveNomination succeeded with its fallback removed, want an error — a returned error is the whole mechanism")
	}
}

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

func TestTheme_NoCompiledInFallbackPalette(t *testing.T) {
	for _, source := range sourceguardtest.ParsePackageSources(t, ".", false) {
		ast.Inspect(source.File, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || len(lit.Elts) == 0 {
				return true
			}
			if !isThemeTypeExpr(lit.Type) {
				return true
			}
			t.Errorf("%s:%d declares a populated Theme literal — there is no runtime last-resort palette beneath the built-in fallback; a build-time guarantee replaced that crutch", source.Path, source.Fset.Position(lit.Pos()).Line)
			return true
		})
	}
}

// Collections count too: a []Theme or map[K]Theme literal declares palettes just
// as a bare Theme literal does, and its elements elide the type entirely.
func isThemeTypeExpr(expr ast.Expr) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name == "Theme"
	case *ast.ArrayType:
		return isThemeTypeExpr(t.Elt)
	case *ast.MapType:
		return isThemeTypeExpr(t.Value)
	case *ast.StarExpr:
		return isThemeTypeExpr(t.X)
	}
	return false
}

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

var builtinSourceOwners = map[string]bool{
	filepath.Join("internal", "theme", "load.go"):     true,
	filepath.Join("internal", "theme", "builtins.go"): true,
}

func TestBuiltinSource_HasNoProductionCallSite(t *testing.T) {
	root, err := portalbintest.ProjectRoot()
	if err != nil {
		t.Fatalf("resolve project root: %v", err)
	}
	paths, err := sourceguardtest.GoSourceFiles(root)
	if err != nil {
		t.Fatalf("enumerate .go files: %v", err)
	}

	found := 0
	var scanPaths []string
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		if builtinSourceOwners[relToProjectRoot(t, root, path)] {
			found++
			continue
		}
		scanPaths = append(scanPaths, path)
	}

	for _, source := range sourceguardtest.ParseSources(t, scanPaths) {
		rel := relToProjectRoot(t, root, source.Path)
		ast.Inspect(source.File, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "BuiltinSource" {
				return true
			}
			t.Errorf("%s:%d references Loader.BuiltinSource — the seam is test-only; a production call site would redefine what \"built-in\" means on the very path the build-time guarantee covers", rel, source.Fset.Position(sel.Pos()).Line)
			return true
		})
	}

	if found != len(builtinSourceOwners) {
		t.Fatalf("found %d of the %d files that own BuiltinSource — the exemption list names a file that no longer exists, so the scan is not covering what it claims", found, len(builtinSourceOwners))
	}
}
