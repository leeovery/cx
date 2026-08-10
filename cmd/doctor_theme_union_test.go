// Tests in this file seed PORTAL_* env vars and mutate package-level Cobra/DI
// state (doctorDeps, rootCmd), so they MUST NOT use t.Parallel.
//
// Concern-split from cmd/doctor_theme_test.go and
// cmd/doctor_persisted_theme_test.go, which own doctor's two theme-advisory
// PRODUCERS — what is IN the themes directory, and what the user PICKED. This
// file owns what belongs to neither: the ASSEMBLY between them, which is doctor's
// one-slug-one-line union (a persisted line outranks the same slug's file line),
// the block's pinned region order, and <M> counted from the assembled set rather
// than from the producers' raw findings.
package cmd

import (
	"bytes"
	"fmt"
	"go/ast"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/theme"
)

// unionAdvisoriesFor runs doctor's WHOLE theme-advisory surface — both producers
// and the assembly between them — over a seeded prefs.json and themes directory,
// through the production entry point every test here must drive: the union is
// only observable where the two producers meet. It yields the assembled form,
// which still carries the identity the union turned on; collectThemeAdvisories
// drops it on the way to the renderer.
func unionAdvisoriesFor(t *testing.T, content, themesDir string) []themeAdvisory {
	t.Helper()

	return themeAdvisoryUnion(persistedThemeDeps(t, content, themesDir))
}

// A line-only advisory cannot reach the union: the assembly takes the
// theme-local record, so a producer of the renderer's advisory class has no way
// into the dedup — nor any identity field to leave unset. Widening the signature
// stops this file compiling.
var _ func([]themeAdvisory, []themeAdvisory) []themeAdvisory = assembleThemeAdvisories

// requireAdvisoryLines fails unless the advisories rendered are exactly these
// lines, in this order. Order is asserted everywhere in this file rather than
// membership: half of what the assembly promises is that a report over an
// unchanged directory reads the same way twice.
func requireAdvisoryLines(t *testing.T, got []themeAdvisory, want ...string) {
	t.Helper()

	lines := advisoryLines(got)
	if !slices.Equal(lines, want) {
		t.Fatalf("advisory lines =\n  %s\nwant\n  %s", strings.Join(lines, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestThemeAdvisoryUnion_PersistedLineWins: it drops the file line when a
// persisted line covers the same slug.
//
// Doctor's theme line, over the most likely failure in the whole feature — the user's
// persisted theme IS the invalid file. Two producers detect it independently, and without
// the union the report would carry two lines for one problem, so <M> would count
// DETECTIONS while the union rule's panel counted the one row it renders. The persisted
// line is the survivor because it carries strictly more: the reason AND which
// slot is affected.
//
// Both the pinned copy renderings of that line are covered, since the parenthetical is the
// only thing that differs between them and the dedup must not depend on it.
func TestThemeAdvisoryUnion_PersistedLineWins(t *testing.T) {
	requireDropInSlug(t, "nord-lee")

	cases := []struct{ name, prefs, want string }{
		{
			name:  "a slot names the invalid file",
			prefs: `{"theme_dark":"nord-lee"}`,
			want:  "⚠ theme nord-lee (dark) does not resolve: bad colour",
		},
		{
			name:  "a constant names the invalid file",
			prefs: `{"theme":"nord-lee"}`,
			want:  "⚠ theme nord-lee does not resolve: bad colour",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := themesDirWith(t, map[string][]byte{"nord-lee.theme": sourceBadColours(t, themeOverride{"canvas", "blue"})})

			// Vacuity guard: the scan really does produce a file line for this slug,
			// so its absence below is the dedup dropping it rather than a producer
			// that found nothing to drop.
			const fileLine = "⚠ theme nord-lee: bad colour — canvas = blue"
			requireAdvisoryLines(t, scanThemesDirectory(theme.NewSilentLoader(), dir), fileLine)

			got := requireOneAdvisory(t, unionAdvisoriesFor(t, tc.prefs, dir))
			if got.line != tc.want {
				t.Errorf("advisory line = %q; want %q — the file line %q is the one that must be dropped", got.line, tc.want, fileLine)
			}
			if !got.fromPrefs {
				t.Error("advisory fromPrefs = false; the surviving line is the persisted one")
			}
		})
	}

	t.Run("the report carries the one line", func(t *testing.T) {
		t.Setenv("PORTAL_THEMES_DIR", themesDirWith(t, map[string][]byte{
			"nord-lee.theme": sourceBadColours(t, themeOverride{"canvas", "blue"}),
		}))
		setPrefsFile(t, `{"theme_dark":"nord-lee"}`)

		outBuf, _, err := runDoctorCmd(t, healthyDoctorDeps(t))
		if err != nil {
			t.Fatalf("Execute err = %v; a theme advisory must never drive the exit code", err)
		}

		out := outBuf.String()
		want := "" +
			"  ⚠ theme nord-lee (dark) does not resolve: bad colour\n" +
			"  7 checks passed · 1 advisory\n"
		if !strings.HasSuffix(out, want) {
			t.Errorf("report does not close with the single union line and its summary:\n%s", out)
		}
		if strings.Contains(out, "⚠ theme nord-lee: bad colour") {
			t.Errorf("report carries the dropped file line as well:\n%s", out)
		}
	})

	t.Run("the rank is the fromPrefs field, not the argument position", func(t *testing.T) {
		// The union's rank is DECLARED on the themeAdvisory record and must be read
		// from there, not inferred from which producer's slice a line arrived in —
		// otherwise the declared identity is unread, and a test asserting
		// `fromPrefs: true` on a persisted line would be asserting a field nothing
		// consults. Only a hand-built value can separate the two, since the real
		// producer always sets it.
		file := themeAdvisory{line: "⚠ theme nord-lee: bad colour — canvas = blue", slug: "nord-lee"}
		unranked := themeAdvisory{line: "⚠ theme nord-lee does not resolve: bad colour", slug: "nord-lee"}

		requireAdvisoryLines(t, assembleThemeAdvisories([]themeAdvisory{file}, []themeAdvisory{unranked}), file.line, unranked.line)
	})
}

// TestThemeAdvisoryUnion_BadNameFileNeverCollides: it keeps both lines for a
// bad-name file.
//
// The reason vocabulary's rung 1 yields no usable identity, so a `bad name` row carries NO
// slug — and the union's non-empty-slug guard is what turns that into a structural
// non-collision rather than a coincidence: such a row can never match a persisted
// slug, so both lines legitimately stand and both count toward <M>.
func TestThemeAdvisoryUnion_BadNameFileNeverCollides(t *testing.T) {
	requireDropInSlug(t, "nord-lee")
	dir := themesDirWith(t, map[string][]byte{"Nord.theme": validThemeSource(t)})

	got := unionAdvisoriesFor(t, `{"theme":"nord-lee"}`, dir)
	requireAdvisoryLines(t, got,
		"⚠ theme file Nord.theme: slug must be lowercase letters, digits and hyphens",
		"⚠ theme nord-lee does not resolve: not found",
	)
	if got[0].slug != "" {
		t.Errorf("file advisory slug = %q; the two lines stand together because a `bad name` row carries none", got[0].slug)
	}

	t.Run("a persisted line with no slug covers nothing", func(t *testing.T) {
		// The guard is stated on BOTH sides of the drop — a file line with no slug
		// is never dropped, and a persisted line with no slug never covers — and
		// neither is reachable from the producers (every nomination carries a
		// non-empty slug). A hand-built pair is the only way to pin the defence, and
		// without it the empty string would be an ordinary key matching every
		// bad-name row and the directory line at once.
		directory := themeAdvisory{line: "⚠ themes directory unreadable: /themes"}
		badName := themeAdvisory{line: "⚠ theme file Nord.theme: slug must be lowercase letters, digits and hyphens"}
		slugless := themeAdvisory{line: "⚠ theme  does not resolve: not found", fromPrefs: true}

		requireAdvisoryLines(t, assembleThemeAdvisories([]themeAdvisory{directory, badName}, []themeAdvisory{slugless}),
			directory.line, badName.line, slugless.line)
	})
}

// TestThemeAdvisoryUnion_ReservedNameResolvesToBuiltin: it keeps the
// reserved-name file line and produces no persisted line.
//
// The second structural non-collision. A persisted slug naming a `reserved name`
// file resolves to the BUILT-IN at ResolveByName's step 2 — the reserved-slug rule's
// no-shadowing property — so the persisted producer emits nothing for it and there is nothing
// to dedup against. The file keeps its own line, which is right: that collision is
// the entire content of the reason.
func TestThemeAdvisoryUnion_ReservedNameResolvesToBuiltin(t *testing.T) {
	requireBuiltinSlug(t, "nord")
	dir := themesDirWith(t, map[string][]byte{"nord.theme": validThemeSource(t)})

	got := requireOneAdvisory(t, unionAdvisoriesFor(t, `{"theme":"nord"}`, dir))

	if want := reservedNameLine("nord.theme", "nord"); got.line != want {
		t.Errorf("advisory line = %q; want %q", got.line, want)
	}
	if got.fromPrefs {
		t.Error("advisory fromPrefs = true; the surviving line is the file's — the persisted slug resolved to the built-in")
	}
	if strings.Contains(got.line, "does not resolve") {
		t.Errorf("advisory line = %q; a slug that resolves to the built-in earns no persisted line at all", got.line)
	}
}

// TestThemeAdvisoryUnion_ValidPersistedFileIsSilent: it produces neither line for
// a persisted valid file.
//
// The ordinary healthy state, and the one that keeps the union honest in the
// other direction: the dedup drops a line because a BETTER one covers it, never
// because a slug was persisted. With nothing wrong there is nothing to report from
// either producer.
func TestThemeAdvisoryUnion_ValidPersistedFileIsSilent(t *testing.T) {
	requireDropInSlug(t, "nord-lee")
	dir := themesDirWith(t, map[string][]byte{
		"nord-lee.theme": validThemeSource(t),
		"notes.txt":      []byte("not a theme file\n"),
	})

	requireNoAdvisories(t, unionAdvisoriesFor(t, `{"theme":"nord-lee"}`, dir))
}

// TestThemeAdvisoryUnion_BothSlotsStayOneLine: it renders one both line rather
// than two.
//
// The row-rendering rule's `● both` state is reachable in two keypresses, so one slug naming
// both slots of a broken file is ordinary rather than exotic — and it is the case where
// a naive union would render THREE lines for one problem (two slots plus the
// file). The pinned copy pins one line in every case: the persisted producer collapses the
// pair, and the union drops the file line under it.
func TestThemeAdvisoryUnion_BothSlotsStayOneLine(t *testing.T) {
	requireDropInSlug(t, "nord-lee")
	dir := themesDirWith(t, map[string][]byte{"nord-lee.theme": sourceBadColours(t, themeOverride{"canvas", "blue"})})

	got := requireOneAdvisory(t, unionAdvisoriesFor(t, `{"theme_light":"nord-lee","theme_dark":"nord-lee"}`, dir))

	if want := "⚠ theme nord-lee (both) does not resolve: bad colour"; got.line != want {
		t.Errorf("advisory line = %q; want %q", got.line, want)
	}
}

// TestThemeAdvisoryUnion_DirectoryLineIsNeverDeduped: it never dedups the
// directory line against a slug.
//
// The directory line is DIRECTORY-LEVEL: it reports a path, carries no slug, and
// says nothing about any particular theme — so no persisted slug can cover it,
// whatever it names. It is also the condition that explains the absence of every
// file line beneath it, which is why the pinned order puts it first.
//
// The fixture is the state where the two lines are most alike: an unusable
// directory makes the persisted slug `unreadable` too, so both lines describe the
// same permissions problem, and both are still owed — one names the directory, the
// other names what the user set and what became of it.
func TestThemeAdvisoryUnion_DirectoryLineIsNeverDeduped(t *testing.T) {
	requireDropInSlug(t, "nord-lee")
	dir := filepath.Join(t.TempDir(), "themes")
	if err := os.WriteFile(dir, []byte("not a directory\n"), 0o644); err != nil {
		t.Fatalf("seed %s: %v", dir, err)
	}

	requireAdvisoryLines(t, unionAdvisoriesFor(t, `{"theme":"nord-lee"}`, dir),
		"⚠ themes directory unreadable: "+dir,
		"⚠ theme nord-lee does not resolve: unreadable",
	)
}

// TestThemeAdvisoryUnion_HandsTheRendererLinesOnly: it converts the assembled
// block to the renderer's line-only class, and a line from elsewhere passes
// through beside it untouched.
//
// The conversion is where the union's identity stops: what the renderer receives
// is what it prints. A line contributed by any other producer therefore neither
// drops a theme line nor is dropped by one, whatever slug its copy happens to
// name — it never carried the identity the dedup reads.
func TestThemeAdvisoryUnion_HandsTheRendererLinesOnly(t *testing.T) {
	requireDropInSlug(t, "nord-lee")
	dir := themesDirWith(t, map[string][]byte{"nord-lee.theme": sourceBadColours(t, themeOverride{"canvas", "blue"})})
	deps := persistedThemeDeps(t, `{"theme_dark":"nord-lee"}`, dir)

	themeBlock := collectThemeAdvisories(deps)
	if want := []advisory{{line: "⚠ theme nord-lee (dark) does not resolve: bad colour"}}; !slices.Equal(themeBlock, want) {
		t.Fatalf("collected block = %+v; want %+v", themeBlock, want)
	}

	elsewhere := advisory{line: "⚠ theme nord-lee: reported by another producer"}
	lines := renderedLines(t, mixedCatalog(), append(themeBlock, elsewhere))

	want := []string{
		"  " + themeBlock[0].line,
		"  " + elsewhere.line,
		"  1 of 2 checks passed · 2 advisories",
	}
	if got := lines[len(lines)-3:]; !slices.Equal(got, want) {
		t.Errorf("report closes with\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// everyRegionFixture seeds the one fixture in which every ordering rule the
// assembly owns is observable at once: file lines in the enumeration's order
// (`Nord.theme` sorts above the lowercase names, so the order is provably
// os.ReadDir's rather than any tidier one), one of them dropped under a persisted
// line, a valid file contributing nothing, and both persisted slots in the constant-or-pair
// rule's key order. It returns the deps and the block they must assemble to.
func everyRegionFixture(t *testing.T) (*DoctorDeps, []string) {
	t.Helper()

	requireDropInSlug(t, "b-colour")
	requireDropInSlug(t, "gone")

	dir := themesDirWith(t, map[string][]byte{
		"Nord.theme":      validThemeSource(t),
		"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		"c-syntax.theme":  sourceDuplicateKeyAt(t, 12, "text.primary"),
		"d-valid.theme":   validThemeSource(t),
	})
	return persistedThemeDeps(t, `{"theme_light":"b-colour","theme_dark":"gone"}`, dir), []string{
		"⚠ theme file Nord.theme: slug must be lowercase letters, digits and hyphens",
		"⚠ theme a-missing: missing tokens — missing text.primary",
		"⚠ theme c-syntax: bad syntax — line 12: duplicate key text.primary",
		"⚠ theme b-colour (light) does not resolve: bad colour",
		"⚠ theme gone (dark) does not resolve: not found",
	}
}

// TestThemeAdvisoryUnion_OrderIsDeterministic: it renders the block in a pinned
// order.
//
// A report whose line order can shift between runs is not testable and reads as
// noise, so the block's sequence is a decision rather than whatever the producers
// happened to append: the directory line, then the file lines, then the persisted
// lines — outermost to innermost, the container, then its contents, then the
// setting that points into it.
func TestThemeAdvisoryUnion_OrderIsDeterministic(t *testing.T) {
	t.Run("the regions appear in the pinned order", func(t *testing.T) {
		deps, want := everyRegionFixture(t)

		got := themeAdvisoryUnion(deps)
		requireAdvisoryLines(t, got, want...)

		// The literal lines above already pin the sequence; this states WHY it is
		// the sequence — the first region boundary is the producers', not a
		// coincidence of how these particular slugs happen to sort.
		for i, a := range got {
			if wantFromPrefs := i >= 3; a.fromPrefs != wantFromPrefs {
				t.Errorf("advisory[%d] (%q) fromPrefs = %t; want %t — every file line precedes every persisted line", i, a.line, a.fromPrefs, wantFromPrefs)
			}
		}
	})

	t.Run("repeat runs over an unchanged directory and prefs are byte-identical", func(t *testing.T) {
		deps, want := everyRegionFixture(t)

		var first string
		for run := range 10 {
			var buf bytes.Buffer
			renderDoctorReport(&buf, mixedCatalog(), collectThemeAdvisories(deps))

			if run == 0 {
				requireAdvisoryLines(t, themeAdvisoryUnion(deps), want...)
				first = buf.String()
				continue
			}
			if got := buf.String(); got != first {
				t.Fatalf("run %d rendered a different report\n got: %q\nwant: %q", run, got, first)
			}
		}
	})

	t.Run("file lines follow the enumeration, not the creation order", func(t *testing.T) {
		// Seeded in reverse-alphabetical order, so a scan that reported files as it
		// found them — or as a map yielded them — could not produce the order below.
		dir := t.TempDir()
		for _, name := range []string{"d-fourth.theme", "c-third.theme", "b-second.theme", "a-first.theme"} {
			if err := os.WriteFile(filepath.Join(dir, name), sourceMissingTokens(t, "text.primary"), 0o644); err != nil {
				t.Fatalf("seed %s: %v", name, err)
			}
		}

		requireAdvisoryLines(t, unionAdvisoriesFor(t, `{}`, dir),
			"⚠ theme a-first: missing tokens — missing text.primary",
			"⚠ theme b-second: missing tokens — missing text.primary",
			"⚠ theme c-third: missing tokens — missing text.primary",
			"⚠ theme d-fourth: missing tokens — missing text.primary",
		)
	})

	t.Run("the assembly iterates no map", func(t *testing.T) {
		// The behavioural half above cannot be trusted alone: Go randomises map
		// iteration per range, so a two-element map would agree with itself often
		// enough for a repeat-run comparison to pass by luck. Declaring no map type
		// anywhere in the assembly is what makes the determinism structural — there
		// is nothing that COULD be iterated in a random order.
		assembly := []string{"collectThemeAdvisories", "themeAdvisoryUnion", "assembleThemeAdvisories", "persistedSlugs"}

		source := parsePackageFilesByName(t)["doctor_theme.go"]
		if source == nil {
			t.Fatal("the cmd package declares no doctor_theme.go — the guard has nothing to scan")
		}

		found := map[string]bool{}
		for _, decl := range source.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !slices.Contains(assembly, fn.Name.Name) {
				continue
			}
			found[fn.Name.Name] = true
			ast.Inspect(fn, func(n ast.Node) bool {
				if _, ok := n.(*ast.MapType); ok {
					t.Errorf("%s declares a map; the assembly's order must not depend on how one is walked", fn.Name.Name)
				}
				return true
			})
		}
		for _, name := range assembly {
			if !found[name] {
				t.Errorf("doctor_theme.go declares no %s — the guard scanned nothing", name)
			}
		}
	})
}

// TestThemeAdvisoryUnion_CountMatchesRenderedLines: it counts M from the final
// line set.
//
// Doctor's theme line: <M> counts LINES, so it counts problems rather than detections — which
// only holds if it is taken from the assembled slice rather than from either
// producer's raw finding count. The property is asserted as an identity between
// the summary's count and the advisory lines actually rendered beside it, over
// every shape the union can take.
func TestThemeAdvisoryUnion_CountMatchesRenderedLines(t *testing.T) {
	requireDropInSlug(t, "nord-lee")

	cases := []struct {
		name  string
		prefs string
		files map[string][]byte
		want  []string
	}{
		{
			name:  "nothing to report",
			prefs: `{}`,
			files: map[string][]byte{"nord-lee.theme": validThemeSource(t)},
		},
		{
			name:  "a persisted line covering a file line",
			prefs: `{"theme_dark":"nord-lee"}`,
			files: map[string][]byte{"nord-lee.theme": sourceBadColours(t, themeOverride{"canvas", "blue"})},
			want:  []string{"⚠ theme nord-lee (dark) does not resolve: bad colour"},
		},
		{
			name:  "two lines sharing the empty slug",
			prefs: `{}`,
			files: map[string][]byte{"A-Nord.theme": validThemeSource(t), "B_Nord.theme": validThemeSource(t)},
			want: []string{
				"⚠ theme file A-Nord.theme: slug must be lowercase letters, digits and hyphens",
				"⚠ theme file B_Nord.theme: slug must be lowercase letters, digits and hyphens",
			},
		},
		{
			name:  "a file line and a persisted line for different slugs",
			prefs: `{"theme":"gone"}`,
			files: map[string][]byte{"nord-lee.theme": sourceBadColours(t, themeOverride{"canvas", "blue"})},
			want: []string{
				"⚠ theme nord-lee: bad colour — canvas = blue",
				"⚠ theme gone does not resolve: not found",
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			deps := persistedThemeDeps(t, tc.prefs, themesDirWith(t, tc.files))
			advisories := themeAdvisoryUnion(deps)
			requireAdvisoryLines(t, advisories, tc.want...)

			lines := renderedLines(t, mixedCatalog(), collectThemeAdvisories(deps))

			var rendered []string
			for _, line := range lines {
				if strings.HasPrefix(line, "  ⚠") {
					rendered = append(rendered, line)
				}
			}
			if len(rendered) != len(advisories) {
				t.Fatalf("report rendered %d advisory lines from %d advisories:\n%s", len(rendered), len(advisories), strings.Join(lines, "\n"))
			}

			// mixedCatalog is 1-of-2, so the summary's checks half is fixed and the
			// suffix is the only thing under test. It must count the lines beside it
			// — never the two producers' raw findings, and never their distinct slugs.
			want := "  1 of 2 checks passed"
			switch len(rendered) {
			case 0:
				// The pinned copy suppresses the suffix entirely at M == 0.
			case 1:
				want += " · 1 advisory"
			default:
				want += fmt.Sprintf(" · %d advisories", len(rendered))
			}
			if got := lines[len(lines)-1]; got != want {
				t.Errorf("rendered summary = %q; want %q", got, want)
			}
		})
	}

	t.Run("two lines sharing a slug are counted twice", func(t *testing.T) {
		// The one same-slug pair the union can actually produce, and the reason it
		// matters: <M> counted as DISTINCT SLUGS rather than as lines would read 1
		// here — a report claiming one problem while printing two — and every other
		// fixture in the suite would stay green, since no other pair shares a slug.
		deps := persistedThemeDeps(t, `{}`, themesDirWith(t, map[string][]byte{
			"A-Nord.theme": validThemeSource(t),
			"B_Nord.theme": validThemeSource(t),
		}))
		advisories := themeAdvisoryUnion(deps)
		if len(advisories) != 2 {
			t.Fatalf("union produced %d advisories, want 2:\n  %s", len(advisories), strings.Join(advisoryLines(advisories), "\n  "))
		}
		for i, a := range advisories {
			if a.slug != "" {
				t.Fatalf("advisory[%d].slug = %q; both lines must carry the same (empty) slug for this to be the collision case", i, a.slug)
			}
		}

		if got, want := doctorSummaryLine(mixedCatalog(), collectThemeAdvisories(deps)), "1 of 2 checks passed · 2 advisories"; got != want {
			t.Errorf("doctorSummaryLine = %q; want %q — <M> counts lines, not distinct slugs", got, want)
		}
	})
}
