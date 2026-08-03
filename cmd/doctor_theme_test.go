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
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
)

// themeOverride replaces one key's value in a fixture source, leaving the key
// and its FILE POSITION alone — which is what makes a `bad colour` fixture's
// offender order predictable, since offenders are enumerated in file order.
type themeOverride struct{ key, value string }

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

// themeLineIndex returns the index of the `key = …` line declaring key, failing
// when the built-in the fixtures derive from no longer declares it.
func themeLineIndex(t *testing.T, lines []string, key string) int {
	t.Helper()

	for i, line := range lines {
		declared, _, found := strings.Cut(line, "=")
		if found && strings.TrimSpace(declared) == key {
			return i
		}
	}
	t.Fatalf("the built-in declares no %q line, so a fixture derived from it would prove nothing:\n  %s", key, strings.Join(lines, "\n  "))
	return -1
}

// sourceMissingTokens is a `missing tokens` drop-in with the named keys' lines
// removed. Everything it still declares is well-formed, so it clears rungs 4
// and 5 and fails at the presence check.
func sourceMissingTokens(t *testing.T, keys ...string) []byte {
	t.Helper()

	lines := slices.Clone(themeKeyLines(t))
	for _, key := range keys {
		at := themeLineIndex(t, lines, key)
		lines = slices.Delete(lines, at, at+1)
	}
	return themeSourceFromLines(lines)
}

// sourceBadColours is a `bad colour` drop-in with the named keys' values
// replaced by ones that still lex as a well-formed `key = value` pair, so the
// file reaches rung 5 intact and fails on its values.
func sourceBadColours(t *testing.T, overrides ...themeOverride) []byte {
	t.Helper()

	lines := slices.Clone(themeKeyLines(t))
	for _, o := range overrides {
		lines[themeLineIndex(t, lines, o.key)] = o.key + " = " + o.value
	}
	return themeSourceFromLines(lines)
}

// sourceDuplicateKeyAt is a `bad syntax` drop-in in which the line declaring key
// is repeated on the given 1-based line, which §6.2's rung 4 refuses at that
// SECOND occurrence — the one the user has to delete.
//
// Both halves of a caller's expectation are verified against the ASSEMBLED
// source: that key really is declared first above the duplicate, and that the
// duplicate really lands on that line. So the `line N: duplicate key <key>`
// detail a test pins is a fact about the fixture rather than a coincidence of
// the built-in it was derived from.
func sourceDuplicateKeyAt(t *testing.T, line int, key string) []byte {
	t.Helper()

	lines := slices.Clone(themeKeyLines(t))
	first := themeLineIndex(t, lines, key)
	if line < first+2 || line > len(lines)+1 {
		t.Fatalf("line %d cannot carry a duplicate of %q, first declared on line %d of %d", line, key, first+1, len(lines))
	}

	assembled := slices.Insert(lines, line-1, lines[first])
	if got := themeLineIndex(t, assembled, key); got != first {
		t.Fatalf("assembled fixture declares %q first on line %d, want line %d", key, got+1, first+1)
	}
	if assembled[line-1] != lines[first] {
		t.Fatalf("assembled fixture carries %q on line %d, want the duplicate %q", assembled[line-1], line, lines[first])
	}
	return themeSourceFromLines(assembled)
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
// §14A's generic frame is `⚠ theme <slug>: <reason> — <detail>`, and the detail
// is Phase 1's own, enumerating WITHIN the reason. The three cases are the three
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
// §14A: the OS error is the only thing distinguishing a permission denial from a
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
// produces: §14A's generic frame with the OS error as its detail, carried
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
// §5.5: zero drop-ins is not an error and Portal never creates or seeds the
// directory — so an absent one produces no line, no error and no log record, and
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
// §5.5's two unusable states get one pinned line each, carrying the path
// verbatim. Both fixtures hold a broken drop-in that WOULD produce a per-file
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
// is that doctor carries Phase 1's string through rather than that it happens to
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

	loader := theme.NewLoader(theme.NewEventLogger(log.Discard()))
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
// §6.2: doctor enumerates WITHIN the reason and never across, so a file that is
// both bad-coloured and short of a token is `bad colour` and its line says
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
// because §12.2's one-slug-one-line union drops a file line only when its slug
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
// §12.3: the component records where a theme is USED, never where one is
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
// must be byte-identical afterwards, and prefs.json — which is task 7-5's seam,
// not this scan's — must be neither written nor created.
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
