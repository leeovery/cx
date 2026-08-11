package theme_test

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

const brokenCanvasValue = "blue"

func TestLoadFile_ReservedSlugRejected(t *testing.T) {
	path := themetest.Write(t, t.TempDir(), tokyoNightSlug+".theme", themetest.Lines())

	got, rejection := productionLoader().LoadFile(path)

	requireLoadRejection(t, got, rejection, theme.ReasonReservedName, "")
}

func TestLoadFile_ReservedDecidedBeforeRead(t *testing.T) {
	tests := []struct {
		name string
		make func(t *testing.T, dir string) string
	}{
		{
			name: "no file at all",
			make: func(t *testing.T, dir string) string {
				return filepath.Join(dir, tokyoNightSlug+".theme")
			},
		},
		{
			name: "an unreadable file",
			make: func(t *testing.T, dir string) string {
				return writeUnreadableTheme(t, dir, tokyoNightSlug+".theme")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.make(t, t.TempDir())
			if _, err := os.ReadFile(path); err == nil {
				t.Fatalf("os.ReadFile(%q) succeeded — the fixture is readable, so the verdict would prove no ordering", path)
			}

			got, rejection := productionLoader().LoadFile(path)

			requireLoadRejection(t, got, rejection, theme.ReasonReservedName, "")
			if rejection.Err != nil {
				t.Errorf("rejection carries Err %v, want nil — the file was never opened", rejection.Err)
			}
		})
	}
}

func TestLoadFile_MixedCaseFilenameIsBadNameNotReserved(t *testing.T) {
	loader := productionLoader()

	for _, slug := range requireBuiltinSlugs(t) {
		for _, variant := range caseVariants(t, slug) {
			t.Run(variant.base, func(t *testing.T) {
				path := themetest.Write(t, t.TempDir(), variant.base, themetest.Lines())

				got, rejection := loader.LoadFile(path)

				requireLoadRejection(t, got, rejection, theme.ReasonBadName, "")
				if rejection.BadNameCause != variant.cause {
					t.Errorf("rejection cause = %v, want %v", rejection.BadNameCause, variant.cause)
				}

				derived, nameRejection := theme.SlugFromFilename(variant.base)
				if nameRejection == nil {
					t.Errorf("SlugFromFilename(%q) = %q, want no slug — a case variant must never be normalised into the reserved %q", variant.base, derived, slug)
				}
				if derived != "" {
					t.Errorf("SlugFromFilename(%q) = %q alongside a rejection, want the empty slug", variant.base, derived)
				}
			})
		}
	}
}

func TestReservedSet_CoversEveryBuiltinSlug(t *testing.T) {
	loader := productionLoader()

	for _, slug := range requireBuiltinSlugs(t) {
		t.Run(slug, func(t *testing.T) {
			path := themetest.Write(t, t.TempDir(), slug+".theme", themetest.Lines())

			got, rejection := loader.LoadFile(path)

			requireLoadRejection(t, got, rejection, theme.ReasonReservedName, "")
		})
	}
}

func TestNoShadowing_BrokenDropInCannotReplaceBuiltin(t *testing.T) {
	embedded, found := theme.BuiltinBytes(tokyoNightSlug)
	if !found {
		t.Fatalf("BuiltinBytes(%q) reported not found — there is no fallback to protect", tokyoNightSlug)
	}

	dir := t.TempDir()
	shadowPath := themetest.Write(t, dir, tokyoNightSlug+".theme", themetest.WithValue(themetest.Lines(), "canvas", brokenCanvasValue))
	shadowBytes, err := os.ReadFile(shadowPath)
	if err != nil {
		t.Fatalf("read %s: %v", shadowPath, err)
	}
	if bytes.Equal(shadowBytes, embedded) {
		t.Fatalf("the staged drop-in is byte-identical to the embedded built-in — it shadows nothing")
	}
	loader := productionLoader()

	entries, rejection := loader.Enumerate(dir)
	if rejection != nil {
		t.Fatalf("Enumerate(%q) reported the directory unusable: %v", dir, rejection)
	}
	if len(entries) != 1 {
		t.Fatalf("Enumerate(%q) returned %d entries, want the one staged drop-in: %+v", dir, len(entries), entries)
	}

	entry := entries[0]
	if entry.Slug != tokyoNightSlug {
		t.Errorf("entry slug = %q, want %q — a rejected file is still listed under its name", entry.Slug, tokyoNightSlug)
	}
	if entry.Rejection == nil {
		t.Fatalf("Enumerate(%q) accepted a drop-in shadowing the built-in %q", dir, tokyoNightSlug)
	}
	if entry.Rejection.Reason != theme.ReasonReservedName {
		t.Errorf("entry reason = %q, want %q — %q would mean the ladder read the file", entry.Rejection.Reason, theme.ReasonReservedName, theme.ReasonBadColour)
	}
	if entry.Theme != (theme.Theme{}) {
		t.Errorf("entry carries the palette %+v alongside a rejection, want the zero Theme", entry.Theme)
	}

	builtin, builtinRejection, builtinFound := loader.LoadBuiltin(tokyoNightSlug)
	if !builtinFound {
		t.Fatalf("LoadBuiltin(%q) reported not found, want the embedded built-in", tokyoNightSlug)
	}
	if builtinRejection != nil {
		t.Fatalf("LoadBuiltin(%q) rejected the embedded built-in: %v", tokyoNightSlug, builtinRejection)
	}
	if !bytes.Equal(builtin.Source, embedded) {
		t.Errorf("LoadBuiltin(%q) Source =\n%s\nwant the embedded bytes:\n%s", tokyoNightSlug, builtin.Source, embedded)
	}
	if bytes.Equal(builtin.Source, shadowBytes) {
		t.Errorf("LoadBuiltin(%q) parsed the drop-in's bytes — the shadowing file became the fallback", tokyoNightSlug)
	}
}

func TestLoadFile_RenamedCopyIsAccepted(t *testing.T) {
	loader := productionLoader()

	for _, slug := range requireBuiltinSlugs(t) {
		t.Run(slug, func(t *testing.T) {
			builtin, rejection, found := loader.LoadBuiltin(slug)
			if rejection != nil || !found {
				t.Fatalf("LoadBuiltin(%q) = (rejection %v, found %t), want the embedded built-in", slug, rejection, found)
			}

			renamed := slug + "-lee"
			path := filepath.Join(t.TempDir(), renamed+".theme")
			if err := os.WriteFile(path, builtin.Source, 0o644); err != nil {
				t.Fatalf("write %s: %v", path, err)
			}

			got, rejection := loader.LoadFile(path)

			if rejection != nil {
				t.Fatalf("LoadFile(%q) rejected the published rename workaround: %v", path, rejection)
			}
			if got.Slug != renamed {
				t.Errorf("slug = %q, want %q", got.Slug, renamed)
			}
			if !slices.Equal(got.Theme.All(), builtin.Theme.All()) {
				t.Errorf("theme = %+v, want the built-in's %+v", got.Theme.All(), builtin.Theme.All())
			}
		})
	}
}

func productionLoader() theme.Loader {
	return theme.NewLoader(theme.NewEventLogger(nil))
}

func requireBuiltinSlugs(t *testing.T) []string {
	t.Helper()

	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("BuiltinSlugs() is empty — every loop over it would assert nothing")
	}
	return slugs
}

type caseVariant struct {
	base  string
	cause theme.BadNameCause
}

func caseVariants(t *testing.T, slug string) []caseVariant {
	t.Helper()

	if slug == "" {
		t.Fatal("a built-in slug is empty — it has no case to vary")
	}

	variants := []caseVariant{}
	for _, stem := range []string{strings.ToUpper(slug), strings.ToUpper(slug[:1]) + slug[1:]} {
		if stem == slug {
			continue
		}
		variants = append(variants,
			caseVariant{base: stem + ".theme", cause: theme.BadNameSlug},
			caseVariant{base: stem + ".THEME", cause: theme.BadNameSlug},
		)
	}
	return append(variants, caseVariant{base: slug + ".THEME", cause: theme.BadNameExtension})
}
