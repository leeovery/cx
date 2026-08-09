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

// brokenCanvasValue is the typo'd hex the reserved-slug rule tells its story with — a value
// that is not a `#RRGGBB` at all, so a file carrying it is `bad colour` the moment
// anything reads it.
const brokenCanvasValue = "blue"

// TestLoadFile_ReservedSlugRejected pins the reserved-slug rule on the PRODUCTION loader: a
// drop-in whose slug collides with a built-in is rejected `reserved name`, even
// when there is nothing else wrong with it whatsoever.
//
// This is the wiring test, and it is deliberately distinct from the loader's
// rung test (TestLoadFile_ReservedNameDecidedFromSlugAlone), which drives the
// rung through an injected synthetic set: that one passes whether or not
// NewLoader populates the seam, so it can say nothing about whether the safety
// property actually EXISTS in a Portal that has been built.
func TestLoadFile_ReservedSlugRejected(t *testing.T) {
	path := themetest.Write(t, t.TempDir(), tokyoNightSlug+".theme", themetest.Lines())

	got, rejection := productionLoader().LoadFile(path)

	requireLoadRejection(t, got, rejection, theme.ReasonReservedName, "")
}

// TestLoadFile_ReservedDecidedBeforeRead pins rung 2's POSITION against the
// production set: the verdict comes from the slug alone, so a colliding file is
// never opened and can never report `unreadable` instead.
//
// Both fixtures are files a read would certainly fail on, which is what makes
// the reserved verdict evidence of ordering rather than a coincidence — and the
// ordering is what makes the guarantee unconditional. A reserved check placed
// after the read would leave a permission-denied `tokyo-night.theme` reported as
// a mere read failure, which is the shape of an answer that invites a "so fix
// the permissions and it will work" reading of a name that will never work.
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

// TestLoadFile_MixedCaseFilenameIsBadNameNotReserved pins the half of the
// no-shadowing property that lives in the slug charset rule's reject-never-normalise rule: a
// mis-cased filename is `bad name` at rung 1 and never reaches the reserved
// rung, so it yields NO slug rather than a quietly corrected one.
//
// That is what keeps the reserved check EXACT STRING EQUALITY and keeps
// `Nord.theme` beside a built-in `nord` safe on a case-insensitive macOS
// filesystem — the filesystem a user is most likely to type it on. Were the
// stem lowercased anywhere on this path, the file would become a collision the
// ladder had already waved through, and a `reserved name` verdict here would be
// the symptom.
//
// The variants are derived from the embedded set rather than named, so a
// built-in added later is covered with no edit here.
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

// TestReservedSet_CoversEveryBuiltinSlug pins the set as DERIVED: every member
// of BuiltinSlugs() reserves, and the test NAMES no theme.
//
// A built-in added by a later PR is therefore covered here without this file
// changing — which is the property that keeps the reserved-slug rule's guarantee true as the
// set grows. A hand-maintained reserved list, or a test that spelled the slugs out,
// would both pass right up until the day someone added a built-in and forgot,
// at which point the new theme would ship shadowable.
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

// TestNoShadowing_BrokenDropInCannotReplaceBuiltin is the safety property
// itself, stated end to end: the built-in Portal falls back to can never
// be the user's broken file.
//
// The fixture is the reserved-slug rule's own story — `tokyo-night.theme` dropped into the
// themes directory with a typo'd hex — driven through the two calls that would have to
// agree for the property to hold: the enumerating path REPORTS it, and the
// built-in lookup is UNAFFECTED by it. Without the reserved rung the first
// would hand the panel a selectable palette from that file and the second would
// be the only thing standing between a typo and an unusable Portal.
//
// The reported reason is asserted exactly: `bad colour` would mean the ladder
// read the file, which is the ordering half of the same guarantee.
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

// TestLoadFile_RenamedCopyIsAccepted pins the reserved-slug rule's published workaround — copy
// `nord` to `nord-lee.theme` — as a working escape hatch, for every built-in.
//
// It is the other face of exact string equality: a slug that merely CONTAINS a
// reserved one is not a collision, so the two-second rename really is the whole
// answer, and a user who wants a built-in with one value changed is never stuck.
// The palette is compared against the built-in's own, so the copy is proved to
// be the same theme under a different identity rather than merely something
// that parsed.
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
				t.Fatalf("LoadFile(%q) rejected §5.4's published workaround: %v", path, rejection)
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

// productionLoader returns the Loader the PRODUCTION constructor builds — the
// one carrying the derived reserved set — with a silenced event seam, since
// nothing in this file asserts on a log line.
func productionLoader() theme.Loader {
	return theme.NewLoader(theme.NewEventLogger(nil))
}

// requireBuiltinSlugs returns the embedded set, failing the test if it is empty
// so a loop over it can never assert vacuously.
func requireBuiltinSlugs(t *testing.T) []string {
	t.Helper()

	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("BuiltinSlugs() is empty — every loop over it would assert nothing")
	}
	return slugs
}

// caseVariant is one mis-cased filename plus the `bad name` cause it must be
// rejected for.
type caseVariant struct {
	base  string
	cause theme.BadNameCause
}

// caseVariants renders the ways a built-in's filename is most plausibly typed
// with the wrong case: a shouted stem, a capitalised stem, and a shouted
// extension.
//
// A stem that does not change under upper-casing — a slug of digits alone —
// contributes no variant, because it would stage the canonical filename and
// assert `bad name` of a name that is perfectly good.
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
		variants = append(variants, caseVariant{base: stem + ".theme", cause: theme.BadNameSlug})
	}
	return append(variants, caseVariant{base: slug + ".THEME", cause: theme.BadNameExtension})
}
