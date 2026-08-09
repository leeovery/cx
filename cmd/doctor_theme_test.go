// Tests in this file seed PORTAL_* env vars and mutate package-level Cobra/DI
// state (doctorDeps, rootCmd), so they MUST NOT use t.Parallel.
//
// Concern-split from cmd/doctor_test.go, alongside cmd/doctor_summary_test.go
// and cmd/doctor_advisory_test.go: this file owns cmd/doctor_theme.go — the
// themes-directory scan that produces doctor's FIRST real advisories. The
// advisory line class itself (its trailing block, its exclusion from the exit
// code, the summary suffix) stays in doctor_advisory_test.go; what is pinned
// here is which lines the scan produces and what it leaves alone.
package cmd

import (
	"fmt"
	"go/ast"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
)

// themesDirWith writes each named file into a fresh temp directory and returns
// it. Names are FILENAMES rather than slugs, so a fixture can carry a
// non-`.theme` file the enumeration must ignore entirely.
func themesDirWith(t *testing.T, files map[string][]byte) string {
	t.Helper()

	dir := t.TempDir()
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	return dir
}

// themeAdvisoriesFor runs the scan over dir through the production entry point,
// with every other doctor seam left nil — the scan reads none of them.
func themeAdvisoriesFor(t *testing.T, dir string) []advisory {
	t.Helper()

	return collectThemeAdvisories(&DoctorDeps{ThemesDir: dir})
}

// advisoryLines is the rendered half of a scan result, for assertions that are
// about the copy rather than the identity fields.
func advisoryLines(advisories []advisory) []string {
	lines := make([]string, 0, len(advisories))
	for _, a := range advisories {
		lines = append(lines, a.line)
	}
	return lines
}

// requireOneAdvisory fails unless the scan produced exactly one advisory, and
// returns it. Cardinality is asserted before content everywhere in this file:
// "one file, one line" is half of what the scan promises, and a content-only
// assertion would pass against a second, duplicate line.
func requireOneAdvisory(t *testing.T, advisories []advisory) advisory {
	t.Helper()

	if len(advisories) != 1 {
		t.Fatalf("scan produced %d advisories, want exactly 1:\n  %s", len(advisories), strings.Join(advisoryLines(advisories), "\n  "))
	}
	return advisories[0]
}

// skipUnlessModeBitsDeny skips when the suite runs as root, where a mode-0000
// fixture denies nothing and an "unreadable" assertion would be about the wrong
// condition entirely.
func skipUnlessModeBitsDeny(t *testing.T) {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("root bypasses 0o000 permissions; the fixture would be readable")
	}
}

// TestThemeAdvisories_InvalidFileFrame: it reports an invalid theme file with
// its reason and detail.
//
// The pinned copy's generic frame is `⚠ theme <slug>: <reason> — <detail>`, and the detail
// is the loader's own, enumerating WITHIN the reason. The three cases are the three
// content reasons that read the file and produce a detail from Portal's own
// rules; `unreadable`'s OS-error detail has its own test below.
func TestThemeAdvisories_InvalidFileFrame(t *testing.T) {
	cases := []struct {
		name   string
		slug   string
		source func(*testing.T) []byte
		want   string
	}{
		{
			name:   "missing tokens names every absent token in §2.4 order",
			slug:   "mine",
			source: func(t *testing.T) []byte { return sourceMissingTokens(t, "text.primary", "bg.subtle") },
			want:   "⚠ theme mine: missing tokens — missing text.primary, bg.subtle",
		},
		{
			name: "bad colour names every offending pair in file order",
			slug: "nord-lee",
			source: func(t *testing.T) []byte {
				return sourceBadColours(t, themeOverride{"text.primary", "#GGGGGG"}, themeOverride{"canvas", "blue"})
			},
			want: "⚠ theme nord-lee: bad colour — text.primary = #GGGGGG, canvas = blue",
		},
		{
			name:   "bad syntax names the second occurrence's line",
			slug:   "dupes",
			source: func(t *testing.T) []byte { return sourceDuplicateKeyAt(t, 12, "text.primary") },
			want:   "⚠ theme dupes: bad syntax — line 12: duplicate key text.primary",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := themesDirWith(t, map[string][]byte{tc.slug + ".theme": tc.source(t)})

			got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
			if got.line != tc.want {
				t.Errorf("advisory line = %q; want %q", got.line, tc.want)
			}
		})
	}
}

// TestThemeAdvisories_UnreadableFileKeepsOSError: it reports an unreadable file
// with the OS error verbatim.
//
// The pinned copy: the OS error is the only thing distinguishing a permission denial from a
// dangling symlink, and doctor is where a verbatim system message belongs. The
// expectation is the error THE OS reports for the same read, so nothing about
// the message is restated in Go — and it must appear exactly once, since a
// double-prefixed line would still contain it.
func TestThemeAdvisories_UnreadableFileKeepsOSError(t *testing.T) {
	t.Run("a mode-0000 file", func(t *testing.T) {
		skipUnlessModeBitsDeny(t)

		dir := themesDirWith(t, map[string][]byte{"mine.theme": validThemeSource(t)})
		path := filepath.Join(dir, "mine.theme")
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatalf("chmod 0000 %s: %v", path, err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o644) })

		assertUnreadableAdvisory(t, dir, "mine", requireDeniedRead(t, path))
	})

	t.Run("a dangling symlink", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "mine.theme")
		if err := os.Symlink(filepath.Join(dir, "no-such-target"), path); err != nil {
			t.Fatalf("symlink %s: %v", path, err)
		}

		assertUnreadableAdvisory(t, dir, "mine", osReadError(t, path))
	})
}

// assertUnreadableAdvisory pins the one `unreadable` line the directory
// produces: the pinned copy's generic frame with the OS error as its detail, carried
// exactly once.
func assertUnreadableAdvisory(t *testing.T, dir, slug string, osErr error) {
	t.Helper()

	got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
	want := fmt.Sprintf("⚠ theme %s: unreadable — %s", slug, osErr)
	if got.line != want {
		t.Errorf("advisory line = %q; want %q", got.line, want)
	}
	if n := strings.Count(got.line, osErr.Error()); n != 1 {
		t.Errorf("advisory line = %q carries the OS error %d times, want exactly 1", got.line, n)
	}
}

// TestThemeAdvisories_ValidFileIsSilent: it produces no line for a valid file.
//
// The non-`.theme` neighbours are half the assertion: a file that was never a
// theme file did not fail to be one, so it earns no line either.
func TestThemeAdvisories_ValidFileIsSilent(t *testing.T) {
	dir := themesDirWith(t, map[string][]byte{
		"mine.theme":     validThemeSource(t),
		"nord-lee.theme": validThemeSource(t),
		"notes.txt":      []byte("text.primary = not a theme file\n"),
		"README.md":      []byte("# themes\n"),
	})

	if got := themeAdvisoriesFor(t, dir); len(got) != 0 {
		t.Errorf("scan produced %d advisories over a valid directory, want none:\n  %s", len(got), strings.Join(advisoryLines(got), "\n  "))
	}
}

// TestThemeAdvisories_AbsentDirectoryIsSilent: it is silent for an absent
// directory.
//
// The directory-resolution rule: zero drop-ins is not an error and Portal never creates or
// seeds the directory — so an absent one produces no line, no error and no log record, and
// is still absent afterwards.
func TestThemeAdvisories_AbsentDirectoryIsSilent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "themes")

	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)

	if got := themeAdvisoriesFor(t, dir); len(got) != 0 {
		t.Errorf("scan produced %d advisories over an absent directory, want none:\n  %s", len(got), strings.Join(advisoryLines(got), "\n  "))
	}
	if records := sink.Records(); len(records) != 0 {
		t.Errorf("scan emitted %d log records over an absent directory, want none: %+v", len(records), records)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Errorf("os.Stat(%s) = %v; the scan must never create the themes directory", dir, err)
	}
}

// TestThemeAdvisories_UnusableDirectoryLine: it reports an unusable directory as
// the only theme-file line.
//
// The directory-resolution rule's two unusable states get one pinned line each, carrying the
// path verbatim. Both fixtures hold a broken drop-in that WOULD produce a per-file
// line from a readable directory, so "the only line" is a real assertion: the
// enumeration returns no entries in this state and there is nothing else to
// report.
func TestThemeAdvisories_UnusableDirectoryLine(t *testing.T) {
	cases := []struct {
		name string
		make func(*testing.T) string
	}{
		{
			name: "a mode-0000 directory",
			make: func(t *testing.T) string {
				skipUnlessModeBitsDeny(t)
				dir := themesDirWith(t, map[string][]byte{"mine.theme": sourceMissingTokens(t, "canvas")})
				if err := os.Chmod(dir, 0o000); err != nil {
					t.Fatalf("chmod 0000 %s: %v", dir, err)
				}
				t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
				return dir
			},
		},
		{
			name: "a regular file where the directory belongs",
			make: func(t *testing.T) string {
				path := filepath.Join(t.TempDir(), "themes")
				if err := os.WriteFile(path, sourceMissingTokens(t, "canvas"), 0o644); err != nil {
					t.Fatalf("seed %s: %v", path, err)
				}
				return path
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := tc.make(t)

			got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
			if want := "⚠ themes directory unreadable: " + dir; got.line != want {
				t.Errorf("advisory line = %q; want %q", got.line, want)
			}
			if got.slug != "" {
				t.Errorf("directory advisory slug = %q; the line is about a path, not a theme", got.slug)
			}
			if got.fromPrefs {
				t.Error("directory advisory fromPrefs = true; the line comes from the directory, not from prefs.json")
			}
		})
	}
}

// TestThemeAdvisories_UnresolvedDirDegrades: it degrades when the themes
// directory cannot be resolved.
//
// The skip is scoped to the DIRECTORY SCAN, never to collectThemeAdvisories
// itself: an unresolved path yields no directory line and no per-file line, and
// the diagnosis still renders every check and its summary. The Execute half is
// what proves the degradation reaches the user as a complete report rather than
// as an aborted one.
func TestThemeAdvisories_UnresolvedDirDegrades(t *testing.T) {
	t.Run("an empty themes directory scans nothing", func(t *testing.T) {
		if got := themeAdvisoriesFor(t, ""); len(got) != 0 {
			t.Errorf("scan produced %d advisories over an unresolved path, want none:\n  %s", len(got), strings.Join(advisoryLines(got), "\n  "))
		}
	})

	t.Run("the diagnosis still renders in full", func(t *testing.T) {
		// resolveDoctorDeps' own best-effort themesDirPath() call is what is under
		// test here, so the deps carry no ThemesDir override and every input the
		// resolution reads is removed.
		deps := healthyDoctorDeps(t)
		unresolvableThemesDir(t)

		outBuf, _, err := runDoctorCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; an unresolvable themes directory must never abort the diagnosis", err)
		}

		out := outBuf.String()
		if strings.Contains(out, "⚠") {
			t.Errorf("report carries an advisory line with no themes directory to scan:\n%s", out)
		}
		if !strings.HasSuffix(out, "\n  7 checks passed\n") {
			t.Errorf("report does not close with the full all-passed summary:\n%s", out)
		}
	})
}

// TestThemeAdvisories_DetailIsVerbatim: it reuses the loader's detail verbatim.
//
// Nothing is re-derived, re-ordered, re-wrapped or double-prefixed here. The
// expectation is read off the SAME loader call the scan makes, so the assertion
// is that doctor carries the loader's string through rather than that it happens to
// render an equivalent one today.
func TestThemeAdvisories_DetailIsVerbatim(t *testing.T) {
	skipUnlessModeBitsDeny(t)

	sources := map[string][]byte{
		"missing.theme": sourceMissingTokens(t, "text.primary", "bg.subtle"),
		"colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		"syntax.theme":  sourceDuplicateKeyAt(t, 12, "text.primary"),
		"denied.theme":  validThemeSource(t),
	}
	dir := themesDirWith(t, sources)
	denied := filepath.Join(dir, "denied.theme")
	if err := os.Chmod(denied, 0o000); err != nil {
		t.Fatalf("chmod 0000 %s: %v", denied, err)
	}
	t.Cleanup(func() { _ = os.Chmod(denied, 0o644) })
	// Called for the vacuity guard alone — the OS error it returns is what the
	// loader re-reads below, so it is not restated here.
	_ = requireDeniedRead(t, denied)

	loader := theme.NewSilentLoader()
	byLine := map[string]string{}
	for _, a := range themeAdvisoriesFor(t, dir) {
		byLine[a.slug] = a.line
	}

	for _, name := range slices.Sorted(maps.Keys(sources)) {
		slug := strings.TrimSuffix(name, ".theme")
		t.Run(slug, func(t *testing.T) {
			_, rejection := loader.LoadFile(filepath.Join(dir, name))
			if rejection == nil {
				t.Fatalf("%s loaded cleanly — the verbatim assertion over it would be vacuous", name)
			}
			if rejection.Detail == "" {
				t.Fatalf("%s rejected with an empty detail — the assertion would be vacuous", name)
			}

			want := fmt.Sprintf("⚠ theme %s: %s — %s", slug, rejection.Reason, rejection.Detail)
			if got := byLine[slug]; got != want {
				t.Errorf("advisory line = %q; want %q — the loader's own reason and detail, carried through", got, want)
			}
		})
	}
}

// TestThemeAdvisories_OneReasonPerFile: it reports exactly one reason per file.
//
// The reason vocabulary: doctor enumerates WITHIN the reason and never across, so a file that
// is both bad-coloured and short of a token is `bad colour` and its line says
// nothing whatsoever about presence. The second half is the cardinality rule
// over a whole directory: every entry contributes at most one line.
func TestThemeAdvisories_OneReasonPerFile(t *testing.T) {
	t.Run("a doubly-broken file reports the ladder's first reason only", func(t *testing.T) {
		lines := slices.Clone(themeKeyLines(t))
		lines[themeLineIndex(t, lines, "canvas")] = "canvas = blue"
		lines = slices.Delete(lines, themeLineIndex(t, lines, "bg.subtle"), themeLineIndex(t, lines, "bg.subtle")+1)
		dir := themesDirWith(t, map[string][]byte{"mine.theme": themeSourceFromLines(lines)})

		got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
		if want := "⚠ theme mine: bad colour — canvas = blue"; got.line != want {
			t.Errorf("advisory line = %q; want %q", got.line, want)
		}
		if strings.Contains(got.line, "missing") {
			t.Errorf("advisory line = %q; a `bad colour` line must say nothing about presence", got.line)
		}
	})

	t.Run("a full reject set reports one line per file", func(t *testing.T) {
		dir := themesDirWith(t, map[string][]byte{
			"a-missing.theme": sourceMissingTokens(t, "text.primary"),
			"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
			"c-syntax.theme":  sourceDuplicateKeyAt(t, 12, "text.primary"),
			"d-valid.theme":   validThemeSource(t),
		})

		got := themeAdvisoriesFor(t, dir)
		want := []string{
			"⚠ theme a-missing: missing tokens — missing text.primary",
			"⚠ theme b-colour: bad colour — canvas = blue",
			"⚠ theme c-syntax: bad syntax — line 12: duplicate key text.primary",
		}
		if !slices.Equal(advisoryLines(got), want) {
			t.Errorf("advisory lines =\n  %s\nwant\n  %s", strings.Join(advisoryLines(got), "\n  "), strings.Join(want, "\n  "))
		}
	})
}

// TestThemeAdvisories_FileLinesCarryTheirSlug: it carries the slug identity for
// the union.
//
// Every file line carries `slug` and `fromPrefs: false` ALONGSIDE its line,
// because doctor's one-slug-one-line union drops a file line only when its slug
// is non-empty and matches a persisted line's. A producer setting `line` alone
// would silently defeat the dedup and make <M> count detections rather than
// problems.
func TestThemeAdvisories_FileLinesCarryTheirSlug(t *testing.T) {
	dir := themesDirWith(t, map[string][]byte{
		"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		"c-syntax.theme":  sourceDuplicateKeyAt(t, 12, "text.primary"),
	})

	got := themeAdvisoriesFor(t, dir)
	if len(got) != 3 {
		t.Fatalf("scan produced %d advisories, want 3:\n  %s", len(got), strings.Join(advisoryLines(got), "\n  "))
	}

	wantSlugs := []string{"a-missing", "b-colour", "c-syntax"}
	for i, a := range got {
		if a.slug != wantSlugs[i] {
			t.Errorf("advisory[%d].slug = %q; want %q", i, a.slug, wantSlugs[i])
		}
		if a.fromPrefs {
			t.Errorf("advisory[%d].fromPrefs = true; a file line never comes from prefs.json", i)
		}
		if !strings.Contains(a.line, wantSlugs[i]) {
			t.Errorf("advisory[%d].line = %q; want it to name the slug %q", i, a.line, wantSlugs[i])
		}
	}
}

// TestThemeAdvisories_EmitsNoThemeRecords: it emits zero theme log records.
//
// The `theme` log component: the component records where a theme is USED, never where one is
// DIAGNOSED. Doctor is the user looking, its whole output is already the
// diagnostic, and it is the run most likely to hit a full reject set — so the
// loader is handed log.Discard() and the largest possible WARN volume is never
// written on the surface needing it least.
//
// The last subtest keeps the rest honest: it proves the installed sink DOES
// capture a `theme` event, so the zero-record assertions are evidence about the
// scan rather than about a deaf harness.
func TestThemeAdvisories_EmitsNoThemeRecords(t *testing.T) {
	t.Run("a full reject set writes nothing", func(t *testing.T) {
		skipUnlessModeBitsDeny(t)

		dir := themesDirWith(t, map[string][]byte{
			"a-missing.theme": sourceMissingTokens(t, "text.primary"),
			"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
			"c-syntax.theme":  sourceDuplicateKeyAt(t, 12, "text.primary"),
			"d-denied.theme":  validThemeSource(t),
			"e-valid.theme":   validThemeSource(t),
		})
		denied := filepath.Join(dir, "d-denied.theme")
		if err := os.Chmod(denied, 0o000); err != nil {
			t.Fatalf("chmod 0000 %s: %v", denied, err)
		}
		t.Cleanup(func() { _ = os.Chmod(denied, 0o644) })
		// Called for the vacuity guard alone: a readable fixture would leave the
		// reject set one short and the record count arguing about the wrong run.
		_ = requireDeniedRead(t, denied)

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		if got := themeAdvisoriesFor(t, dir); len(got) != 4 {
			t.Fatalf("scan produced %d advisories, want 4 — the zero-record assertion needs a full reject set:\n  %s", len(got), strings.Join(advisoryLines(got), "\n  "))
		}
		if records := sink.Records(); len(records) != 0 {
			t.Errorf("scan emitted %d log records, want none: %+v", len(records), records)
		}
	})

	t.Run("an unusable directory writes nothing", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "themes")
		if err := os.WriteFile(path, []byte("not a directory\n"), 0o644); err != nil {
			t.Fatalf("seed %s: %v", path, err)
		}

		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		requireOneAdvisory(t, themeAdvisoriesFor(t, path))
		if records := sink.Records(); len(records) != 0 {
			t.Errorf("scan emitted %d log records over an unusable directory, want none: %+v", len(records), records)
		}
	})

	t.Run("the sink captures a theme event when one is emitted", func(t *testing.T) {
		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		theme.NewEventLogger(log.For("theme")).Rejected("mine", "", &theme.Rejection{Reason: theme.ReasonBadColour})

		if records := sink.Records(); len(records) != 1 {
			t.Fatalf("the capture harness recorded %d theme events, want 1 — the assertions above would be vacuous: %+v", len(records), records)
		}
	})
}

// TestThemeAdvisories_ScanIsReadOnly: it writes nothing and reads no prefs.
//
// Doctor heals nothing on the read-only path, and there is deliberately no
// repair for a user's broken palette. The themes directory and every file in it
// must be byte-identical afterwards, and prefs.json — which is the persisted-theme line's
// seam, not this scan's — must be neither written nor created.
func TestThemeAdvisories_ScanIsReadOnly(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "themes")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	files := map[string][]byte{
		"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		"c-syntax.theme":  sourceDuplicateKeyAt(t, 12, "text.primary"),
		"d-valid.theme":   validThemeSource(t),
		"notes.txt":       []byte("not a theme file\n"),
	}
	for name, data := range files {
		if err := os.WriteFile(filepath.Join(dir, name), data, 0o644); err != nil {
			t.Fatalf("seed %s: %v", name, err)
		}
	}
	prefsPath := filepath.Join(root, "prefs.json")
	if err := os.WriteFile(prefsPath, []byte(`{"session_list_mode":"by-tag","appearance":"light"}`), 0o600); err != nil {
		t.Fatalf("seed prefs.json: %v", err)
	}
	t.Setenv("PORTAL_PREFS_FILE", prefsPath)

	before := snapshotTree(t, root)
	if len(before) < len(files) {
		t.Fatalf("snapshot holds %d entries over %d seeded files — the comparison would be vacuous", len(before), len(files))
	}

	if got := themeAdvisoriesFor(t, dir); len(got) != 3 {
		t.Fatalf("scan produced %d advisories, want 3:\n  %s", len(got), strings.Join(advisoryLines(got), "\n  "))
	}

	if after := snapshotTree(t, root); !maps.Equal(after, before) {
		t.Errorf("the config tree changed:\nbefore: %v\nafter:  %v", before, after)
	}

	t.Run("it creates no prefs.json when there is none", func(t *testing.T) {
		absent := filepath.Join(t.TempDir(), "prefs.json")
		t.Setenv("PORTAL_PREFS_FILE", absent)

		themeAdvisoriesFor(t, dir)

		if _, err := os.Stat(absent); !os.IsNotExist(err) {
			t.Errorf("os.Stat(%s) = %v; the scan must never create prefs.json", absent, err)
		}
	})
}

// TestThemeAdvisories_ReachTheDoctorReport pins the production wiring: the scan
// supplies the report's advisory block on the real `portal doctor` path, its
// lines trail the whole check catalog, the summary counts them — and none of it
// touches the exit code, which stays 0 over an all-passing catalog.
//
// Both arms of the ThemesDir seam are pinned, because they fail differently. The
// body drives the doctorDeps OVERRIDE, which is what every other test in this
// file scans through; the subtest drives resolveDoctorDeps' own best-effort
// themesDirPath() call with NO override, which is the one line connecting this
// feature to a real user's themes directory — delete it and an override-only
// suite stays green while doctor silently scans nothing in production.
func TestThemeAdvisories_ReachTheDoctorReport(t *testing.T) {
	deps := healthyDoctorDeps(t)
	deps.ThemesDir = themesDirWith(t, map[string][]byte{
		"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		"c-valid.theme":   validThemeSource(t),
	})

	outBuf, _, err := runDoctorCmd(t, deps)
	if err != nil {
		t.Fatalf("Execute err = %v; a broken drop-in must never drive the exit code", err)
	}

	out := outBuf.String()
	want := "" +
		"  ⚠ theme a-missing: missing tokens — missing text.primary\n" +
		"  ⚠ theme b-colour: bad colour — canvas = blue\n" +
		"  7 checks passed · 2 advisories\n"
	if !strings.HasSuffix(out, want) {
		t.Errorf("report does not close with the advisory block and its summary:\n%s", out)
	}

	t.Run("the resolved themes directory is scanned with no override", func(t *testing.T) {
		t.Setenv("PORTAL_THEMES_DIR", themesDirWith(t, map[string][]byte{
			"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		}))

		// No ThemesDir on the deps: the field can only arrive through the
		// production resolution, so the advisory below is proof that doctor reads
		// the directory the user's environment actually names.
		outBuf, _, err := runDoctorCmd(t, healthyDoctorDeps(t))
		if err != nil {
			t.Fatalf("Execute err = %v; a broken drop-in must never drive the exit code", err)
		}

		out := outBuf.String()
		want := "" +
			"  ⚠ theme a-missing: missing tokens — missing text.primary\n" +
			"  7 checks passed · 1 advisory\n"
		if !strings.HasSuffix(out, want) {
			t.Errorf("report does not close with the resolved directory's advisory:\n%s", out)
		}
	})
}

// requireBuiltinSlug fails unless slug names one of the embedded built-ins.
//
// It is the vacuity guard every deliberately-colliding fixture needs: a
// `nord.theme` beside no built-in `nord` is an ordinary valid drop-in producing
// no line at all, so a reserved-name assertion over it would be evidence about
// nothing. The one test that must name no theme derives its slugs instead.
func requireBuiltinSlug(t *testing.T, slug string) {
	t.Helper()

	if !slices.Contains(theme.BuiltinSlugs(), slug) {
		t.Fatalf("%q is not a built-in slug (the embedded set is %v) — a fixture colliding with it would prove nothing", slug, theme.BuiltinSlugs())
	}
}

// reservedNameLine composes the pinned copy's reserved-name line for a filename and the
// slug it collided on.
//
// It exists for the one test that derives its fixtures from the embedded set and
// so must name no theme. Every test that CAN name one states the pinned copy
// verbatim instead, so the composition below is never the suite's only statement
// of it.
func reservedNameLine(filename, slug string) string {
	return "⚠ theme file " + filename + ": " + slug + " is a built-in — rename it (e.g. " + slug + "-mine.theme)"
}

// TestThemeAdvisories_BadNameSlugFrame: it renders the slug-cause bad-name
// frame.
//
// The pinned copy composes this line from the FILENAME rather than from a slug, because a
// `bad name` file has none — and the differing frame (`⚠ theme file <filename>:`
// versus `⚠ theme <slug>:`) is what carries the reason vocabulary's input class while the
// reason class itself stays single.
//
// Every fixture holds VALID contents, so the name is the only thing wrong with
// it and the line cannot be a content reason wearing the wrong frame. The cases
// are the slug charset rule — an uppercase letter, an underscore, a space — plus
// two of the edges its anchoring closes: a leading hyphen, and the empty stem of
// a bare `.theme`.
func TestThemeAdvisories_BadNameSlugFrame(t *testing.T) {
	cases := []struct{ name, filename string }{
		{name: "an uppercase stem", filename: "Nord.theme"},
		{name: "an underscore", filename: "nord_lee.theme"},
		{name: "a space", filename: "nord lee.theme"},
		{name: "a leading hyphen", filename: "-nord.theme"},
		{name: "an empty stem", filename: ".theme"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := themesDirWith(t, map[string][]byte{tc.filename: validThemeSource(t)})

			got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
			want := "⚠ theme file " + tc.filename + ": slug must be lowercase letters, digits and hyphens"
			if got.line != want {
				t.Errorf("advisory line = %q; want %q", got.line, want)
			}
		})
	}
}

// TestThemeAdvisories_BadNameExtensionFrame: it renders the extension-cause
// bad-name frame.
//
// A DISTINCT message from the slug cause, off the loader's BadNameCause: with a
// wrong-cased extension the slug portion is already legal, so telling the user
// to fix their slug would send them to correct the one thing that is fine —
// exactly the misdirection the pinned copy discriminates against here.
//
// These files are visible at all only because enumeration matches the extension
// case-insensitively before rejecting it, which is what keeps a `nord.THEME`
// from being silently absent on the case-insensitive filesystem where it is most
// likely to be typed. Each case gets its own directory: the two names fold
// together there.
func TestThemeAdvisories_BadNameExtensionFrame(t *testing.T) {
	cases := []struct{ name, filename string }{
		{name: "a shouted extension", filename: "nord.THEME"},
		{name: "a title-cased extension", filename: "nord.Theme"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := themesDirWith(t, map[string][]byte{tc.filename: validThemeSource(t)})

			got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
			want := "⚠ theme file " + tc.filename + ": extension must be lowercase .theme"
			if got.line != want {
				t.Errorf("advisory line = %q; want %q", got.line, want)
			}
		})
	}
}

// TestThemeAdvisories_FilenameReasonsLabelledByFilename: it labels a
// filename-reason row by filename.
//
// The two frames are asserted against each other over ONE directory holding
// both classes, which is the only place their split is observable: line by line,
// every filename-reason row (`bad name` either cause, `reserved name`) reads
// `⚠ theme file <filename>: …` and every content-reason row reads `⚠ theme
// <slug>: …`. Neither frame may appear on the other's row.
//
// The reserved row is labelled by filename despite having a perfectly valid slug
// because that slug is IDENTICAL to the built-in's: labelling by slug would print
// the same name twice with no way to tell which row is the user's file.
func TestThemeAdvisories_FilenameReasonsLabelledByFilename(t *testing.T) {
	requireBuiltinSlug(t, "nord")
	dir := themesDirWith(t, map[string][]byte{
		"a-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		"b_bad.theme":     validThemeSource(t),
		"c-missing.theme": sourceMissingTokens(t, "text.primary"),
		"d-ext.THEME":     validThemeSource(t),
		"nord.theme":      validThemeSource(t),
	})

	got := advisoryLines(themeAdvisoriesFor(t, dir))
	want := []string{
		"⚠ theme a-colour: bad colour — canvas = blue",
		"⚠ theme file b_bad.theme: slug must be lowercase letters, digits and hyphens",
		"⚠ theme c-missing: missing tokens — missing text.primary",
		"⚠ theme file d-ext.THEME: extension must be lowercase .theme",
		"⚠ theme file nord.theme: nord is a built-in — rename it (e.g. nord-mine.theme)",
	}
	if !slices.Equal(got, want) {
		t.Errorf("advisory lines =\n  %s\nwant\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// TestThemeAdvisories_ReservedNameFrame: it renders the reserved-name line
// naming the conflict and the fix.
//
// The pinned copy gives this reason a whole line of its own rather than the generic
// `<reason> — <detail>` frame, and the `(e.g. <slug>-mine.theme)` suffix is the
// point of it: the reserved-slug rule's workaround is a two-second rename, and printing the
// rename is what makes it self-documenting rather than merely short. Its terse `reserved
// name` label stays the panel's business, where there is no width for either.
//
// The fixture's contents are BROKEN, so the line is also proof the reason is
// settled at the reason vocabulary's rung 2 — above everything that reads the file.
func TestThemeAdvisories_ReservedNameFrame(t *testing.T) {
	requireBuiltinSlug(t, "nord")
	dir := themesDirWith(t, map[string][]byte{"nord.theme": sourceMissingTokens(t, "text.primary")})

	got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
	want := "⚠ theme file nord.theme: nord is a built-in — rename it (e.g. nord-mine.theme)"
	if got.line != want {
		t.Errorf("advisory line = %q; want %q", got.line, want)
	}
	if strings.Contains(got.line, string(theme.ReasonReservedName)) {
		t.Errorf("advisory line = %q carries the terse reason label; §14A's line names the conflict and the fix INSTEAD of following the generic `<reason> — <detail>` frame", got.line)
	}
	if got.slug != "nord" {
		t.Errorf("advisory slug = %q; want %q — a reserved-name entry has a valid slug, and it is what collided", got.slug, "nord")
	}
	if got.fromPrefs {
		t.Error("advisory fromPrefs = true; a file line never comes from prefs.json")
	}
}

// TestThemeAdvisories_ReservedSetIsTheEmbeddedSet: it derives the reserved set
// from the embedded built-ins.
//
// The set doctor reports against is the embedded set ITSELF (the reserved-slug rule via
// builtinSlugSet), never a list restated here — so a built-in added by a later PR
// is covered with no edit to doctor and no edit to this test. That is asserted by
// seeding one colliding file per member and NAMING NO THEME: a hardcoded slug
// would pass right up until someone forgot to extend it.
func TestThemeAdvisories_ReservedSetIsTheEmbeddedSet(t *testing.T) {
	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("the embedded set is empty — every assertion below would be vacuous")
	}

	files := map[string][]byte{}
	for _, slug := range slugs {
		files[slug+".theme"] = validThemeSource(t)
	}
	dir := themesDirWith(t, files)

	got := themeAdvisoriesFor(t, dir)
	if len(got) != len(slugs) {
		t.Fatalf("scan produced %d advisories over %d built-in collisions, want one each:\n  %s", len(got), len(slugs), strings.Join(advisoryLines(got), "\n  "))
	}

	lines := advisoryLines(got)
	for _, slug := range slugs {
		if want := reservedNameLine(slug+".theme", slug); !slices.Contains(lines, want) {
			t.Errorf("scan produced no reserved-name line for the built-in %q:\n  %s\nwant it to carry\n  %s", slug, strings.Join(lines, "\n  "), want)
		}
	}
}

// TestThemeAdvisories_BadNameNeverReportsContent: it never reports a bad-name
// file's contents.
//
// The reason vocabulary's rung 1 is decided from the filename BEFORE the file is opened, so a
// `bad name` file can never also report `unreadable` or anything about what is
// inside it. The fixture is broken at every rung below it that a file can be put
// into at once — a mode denying the read outright, a duplicate key and a bad hex
// — and still owes exactly one line, the filename one.
func TestThemeAdvisories_BadNameNeverReportsContent(t *testing.T) {
	skipUnlessModeBitsDeny(t)

	lines := slices.Clone(themeKeyLines(t))
	lines[themeLineIndex(t, lines, "canvas")] = "canvas = blue"
	lines = append(lines, lines[themeLineIndex(t, lines, "text.primary")])

	dir := themesDirWith(t, map[string][]byte{"Bad_Name.theme": themeSourceFromLines(lines)})
	path := filepath.Join(dir, "Bad_Name.theme")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod 0000 %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	// Called for the vacuity guard alone: a readable fixture would leave
	// `unreadable` unreachable and the assertion arguing about the wrong run.
	_ = requireDeniedRead(t, path)

	got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
	want := "⚠ theme file Bad_Name.theme: slug must be lowercase letters, digits and hyphens"
	if got.line != want {
		t.Errorf("advisory line = %q; want %q", got.line, want)
	}
	for _, reason := range []theme.Reason{theme.ReasonUnreadable, theme.ReasonBadSyntax, theme.ReasonBadColour, theme.ReasonMissingTokens} {
		if strings.Contains(got.line, string(reason)) {
			t.Errorf("advisory line = %q reports %q; the filename is decided before the file is opened, so a `bad name` file can never report on its contents", got.line, reason)
		}
	}
}

// TestThemeAdvisories_BadNameCarriesNoSlug: it leaves a bad-name advisory
// without a slug.
//
// A `bad name` file yields NO usable identity — the enumeration leaves Entry.Slug empty
// exactly there — and doctor's row inherits that emptiness rather than inventing
// a label for the union to match on. It is what makes doctor's one-slug-one-line
// dedup structural for this class: a bad-name row can never collide with a
// persisted slug, because it carries none.
func TestThemeAdvisories_BadNameCarriesNoSlug(t *testing.T) {
	cases := []struct{ name, filename string }{
		{name: "the slug cause", filename: "Nord.theme"},
		{name: "the extension cause", filename: "nord.THEME"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := themesDirWith(t, map[string][]byte{tc.filename: validThemeSource(t)})

			got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
			if got.slug != "" {
				t.Errorf("advisory slug = %q; a `bad name` file yields no slug at all, and the union must never dedup a persisted slug against one", got.slug)
			}
			if got.fromPrefs {
				t.Error("advisory fromPrefs = true; a file line never comes from prefs.json")
			}
		})
	}
}

// TestThemeAdvisories_BadNameSlugIsStatedNotCopied: it states the bad-name row's
// empty slug rather than copying the entry's.
//
// Every `bad name` entry the ENUMERATION can produce already carries an empty
// Slug (the enumeration leaves it empty exactly there), so on a directory fixture
// "stated" and "copied" are indistinguishable — and the claim that the emptiness
// follows from the REASON rather than from an upstream field happening to be
// empty would rest on nothing. A hand-built entry separates them: it names a
// slug, the reason vocabulary's rung 1 says a bad-name file yields no usable identity, and the
// row must take the reason's answer. That is what makes doctor's dedup structural for
// this class — a bad-name row can never collide with a persisted slug.
func TestThemeAdvisories_BadNameSlugIsStatedNotCopied(t *testing.T) {
	entry := theme.Entry{
		Filename:  "Bad_Name.theme",
		Slug:      "bad-name",
		Rejection: &theme.Rejection{Reason: theme.ReasonBadName, BadNameCause: theme.BadNameSlug},
	}

	got, reported := themeFileAdvisory(entry)
	if !reported {
		t.Fatal("themeFileAdvisory reported nothing for a `bad name` entry")
	}
	if got.slug != "" {
		t.Errorf("advisory slug = %q; want %q — the emptiness is a consequence of the reason, not a copy of the entry's field", got.slug, "")
	}
	if want := "⚠ theme file Bad_Name.theme: slug must be lowercase letters, digits and hyphens"; got.line != want {
		t.Errorf("advisory line = %q; want %q", got.line, want)
	}
}

// TestThemeAdvisories_NotFoundIsNeverAFileLine: it produces no line for a `not
// found` entry.
//
// The reason vocabulary's seventh reason is the one this producer does NOT own — it applies to
// a persisted slug with no file, which is the other producer's line — and the
// switch skips it VISIBLY rather than sweeping it into the generic frame, where
// it would render `⚠ theme gone: not found — ` with an empty detail. The
// enumeration cannot produce such an entry (every entry came from a file that
// exists), so only a hand-built one can pin the arm.
func TestThemeAdvisories_NotFoundIsNeverAFileLine(t *testing.T) {
	entry := theme.Entry{
		Filename:  "gone.theme",
		Slug:      "gone",
		Rejection: &theme.Rejection{Reason: theme.ReasonNotFound},
	}

	if got, reported := themeFileAdvisory(entry); reported {
		t.Errorf("themeFileAdvisory reported %q for a `not found` entry; the persisted producer owns that reason", got.line)
	}
}

// TestThemeAdvisories_ReservedNameDecidedBeforeRead: it reports a reserved-name
// file whose contents are valid.
//
// The reason vocabulary's rung 2 is decided from the SLUG alone, before any read, so the
// contents bear on it not at all in either direction: a perfectly valid palette is still
// refused (it would otherwise shadow the built-in Portal falls back to), and one
// that cannot be read at all is still `reserved name` rather than `unreadable`.
func TestThemeAdvisories_ReservedNameDecidedBeforeRead(t *testing.T) {
	requireBuiltinSlug(t, "nord")
	want := reservedNameLine("nord.theme", "nord")

	t.Run("contents that are perfectly valid", func(t *testing.T) {
		dir := themesDirWith(t, map[string][]byte{"nord.theme": validThemeSource(t)})

		got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
		if got.line != want {
			t.Errorf("advisory line = %q; want %q", got.line, want)
		}
	})

	t.Run("contents that cannot be read at all", func(t *testing.T) {
		skipUnlessModeBitsDeny(t)

		dir := themesDirWith(t, map[string][]byte{"nord.theme": validThemeSource(t)})
		path := filepath.Join(dir, "nord.theme")
		if err := os.Chmod(path, 0o000); err != nil {
			t.Fatalf("chmod 0000 %s: %v", path, err)
		}
		t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
		// Called for the vacuity guard alone — an actually-readable fixture would
		// prove nothing about the rung's position.
		_ = requireDeniedRead(t, path)

		got := requireOneAdvisory(t, themeAdvisoriesFor(t, dir))
		if got.line != want {
			t.Errorf("advisory line = %q; want %q", got.line, want)
		}
	})
}

// TestThemeAdvisories_FilenameFramesAreSingleSourced: it states each filename
// frame exactly once.
//
// The copy is single-sourced following the convention spawn.UnsupportedNoopMessage
// set, and these three frames need it more than most: doctor grows a SECOND
// theme-advisory producer for persisted slugs, whose union renders into the same
// block, so a second rendering of "is a built-in — rename it" would be invisible
// at review and would drift silently.
//
// The scan is the cmd package's production sources, which is where every pinned copy
// frame is composed — doctor's report and `portal theme export`'s stderr alike.
func TestThemeAdvisories_FilenameFramesAreSingleSourced(t *testing.T) {
	fragments := []string{
		"slug must be lowercase letters, digits and hyphens",
		"extension must be lowercase .theme",
		"is a built-in — rename it",
	}

	for _, fragment := range fragments {
		t.Run(fragment, func(t *testing.T) {
			sites := cmdLiteralSites(t, fragment)

			if want := map[string]int{"doctor_theme.go": 1}; !maps.Equal(sites, want) {
				t.Errorf("the literal %q is declared at %v; want %v — one const in the file that owns doctor's theme copy", fragment, sites, want)
			}
		})
	}
}

// cmdLiteralSites counts, per production file of the cmd package, the string
// literals containing fragment.
func cmdLiteralSites(t *testing.T, fragment string) map[string]int {
	t.Helper()

	sites := map[string]int{}
	for name, file := range parseCmdFiles(t) {
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value, err := strconv.Unquote(lit.Value)
			if err == nil && strings.Contains(value, fragment) {
				sites[name]++
			}
			return true
		})
	}
	return sites
}
