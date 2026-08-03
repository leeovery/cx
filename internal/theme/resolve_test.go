package theme_test

import (
	"errors"
	"go/ast"
	"io/fs"
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

// resolveLoader returns the production-shaped loader — every built-in slug
// reserved, the `theme` component really bound — with its events captured.
//
// The binding is a real log.For rather than a bare capture logger for the reason
// events_test.go gives: the component reaches the line from the INJECTED logger
// and from nowhere inside the package, so a test that faked the binding would be
// asserting about its own harness.
func resolveLoader(t *testing.T) (theme.Loader, *logtest.Sink) {
	t.Helper()

	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	return theme.NewLoader(theme.NewEventLogger(log.For(themeComponent))), sink
}

// requireNoThemeRecords fails the test unless the resolution emitted nothing at
// all. It is the assertion behind every silent outcome §5.5 names: an absent
// directory, an absent file, and a slug refused before any path was composed.
func requireNoThemeRecords(t *testing.T, sink *logtest.Sink) {
	t.Helper()

	if records := sink.Records(); len(records) != 0 {
		t.Errorf("emitted %d records, want none: %+v", len(records), records)
	}
}

// escapeTargetDir stages the fixture that catches a composed path: a themes
// directory nested one level down, with a valid theme sitting in its PARENT and
// further valid themes inside it under names the charset rule refuses.
//
// The theme in the PARENT is the one that discriminates. `../evil` joined onto
// the directory resolves to it, and it is a real, loadable theme — so an
// implementation that composed first and validated second would come back with a
// palette rather than a rejection, which makes the assertion evidence about
// ORDERING rather than about a file happening to be absent. That is the whole
// traversal §5.2's charset rule exists to stop.
//
// The files INSIDE the directory cannot discriminate, and they are staged
// anyway. A slug that fails the charset check composes a filename that fails the
// ladder's own name rung for the identical rule, so `bad name` is
// over-determined there — but their presence is what makes the fixture the state
// a user could really be in, rather than an empty directory in which every
// answer looks the same.
func escapeTargetDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	writeTheme(t, root, "evil.theme", themeLines())

	dir := filepath.Join(root, "themes")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	for _, base := range []string{"Nord.theme", "-nord.theme", "nord lee.theme", ".theme"} {
		writeTheme(t, dir, base, themeLines())
	}
	return dir
}

// TestResolveByName_CharsetCheckedBeforePathComposition pins §8.6: the nominated
// slug comes from a HAND-EDITABLE file and is used to locate a file by name on a
// path that deliberately never enumerates, so it is charset-validated BEFORE it
// can become a path component — which is what stops `../something` escaping the
// themes directory.
//
// The reason is `bad name`, never `not found` (§9.4). Each of §6.2's reasons has
// exactly one condition, and telling a user their file is missing when they
// typed an illegal name sends them looking in the wrong place.
//
// The traversal cases are what make this a test of ORDERING rather than of the
// charset rule alone: `../evil` joined onto the directory lands on a readable,
// valid theme, so an implementation that composed first and validated second
// would return that palette here instead of a rejection. The remaining cases
// pin the reason itself against the whole shape of the rule — case, spacing, a
// leading hyphen and the empty string.
func TestResolveByName_CharsetCheckedBeforePathComposition(t *testing.T) {
	slugs := []string{"../evil", "../../etc/passwd", "-nord", "Nord", "nord lee", ""}

	for _, slug := range slugs {
		t.Run(slug, func(t *testing.T) {
			loader, sink := resolveLoader(t)
			dir := escapeTargetDir(t)

			result, rejection := loader.ResolveByName(slug, dir)

			requireLoadRejection(t, result, rejection, theme.ReasonBadName, "")
			if rejection.BadNameCause != theme.BadNameSlug {
				t.Errorf("bad-name cause = %v, want BadNameSlug — a non-file input has no extension to fail", rejection.BadNameCause)
			}
			requireNoThemeRecords(t, sink)
		})
	}
}

// requireBuiltinResolved fails the test unless the resolution returned the
// EMBEDDED theme for slug — its slug, its parsed palette and its bytes verbatim.
//
// The bytes are what make it a claim about which of two candidates answered:
// a shadowing drop-in and the built-in it collides with are both valid themes,
// so only the source distinguishes them.
func requireBuiltinResolved(t *testing.T, result theme.Result, rejection *theme.Rejection, slug string) {
	t.Helper()

	if rejection != nil {
		t.Fatalf("ResolveByName(%q) = %v, want the embedded theme", slug, rejection)
	}
	if result.Slug != slug {
		t.Errorf("resolved slug = %q, want %q", result.Slug, slug)
	}

	want, found := theme.BuiltinBytes(slug)
	if !found {
		t.Fatalf("BuiltinBytes(%q) reports not found", slug)
	}
	if string(result.Source) != string(want) {
		t.Errorf("resolved source =\n%s\nwant the embedded bytes:\n%s", result.Source, want)
	}
	if result.Theme == (theme.Theme{}) {
		t.Error("resolved theme is the zero Theme, want the embedded palette")
	}
}

// requireBuiltins fails the test if the embedded set is empty, and returns it.
// Every assertion looping over the built-ins would otherwise pass vacuously.
func requireBuiltins(t *testing.T) []string {
	t.Helper()

	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("BuiltinSlugs() is empty — every assertion over the embedded set would be vacuous")
	}
	return slugs
}

// TestResolveByName_BuiltinNeverReadsDirectory pins §8.4's ordering rule: the
// EMBEDDED SET FIRST, then the themes directory. A nominated slug that names a
// built-in resolves to the built-in and never reads the themes directory at all.
//
// This ordering IS §5.4's no-shadowing safety property on the by-name path.
// Construction does not enumerate, so there is no collision to DETECT there —
// and construction is where the fallback resolves, which is the exact thing
// no-shadowing exists to protect.
//
// The first three fixtures pin the OUTCOME across the directory states a user
// can be in: unreadable, absent, and holding a BROKEN file under the built-in's
// own name — §5.4's danger case in the flesh, since a `tokyo-night.theme` with a
// token missing is exactly what must not become the thing Portal falls back to.
//
// None of the three can pin the ORDERING, and it matters to say so rather than
// to assume otherwise. An unreadable directory STATS PERFECTLY WELL, so its
// silence says nothing about whether it was stat'd; an absent one is silent by
// decision; and a file under a built-in's own name is refused by the ladder's
// reserved-name rung BEFORE it is opened. In all three, "never looked" and
// "looked, then preferred the built-in" answer identically.
//
// The fourth is the one that discriminates, being the only built-in-slug fixture
// whose directory state has an OBSERVABLE side effect: a regular file where the
// directory belongs is §5.5's unusable state, which emits `theme: directory
// unusable` the moment anything stats it. The SILENCE is the evidence that
// nothing did. Without it a resolver that tried the directory first and fell
// through to the embedded set on any rejection passes this whole file — and
// §8.4's "never reads the themes directory at all" is a COST property (§5.7's
// construction budget, and internal/capture's no-real-config import guard), so
// it would regress invisibly rather than loudly.
func TestResolveByName_BuiltinNeverReadsDirectory(t *testing.T) {
	t.Run("with an unreadable themes directory", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := unreadableDir(t)

		for _, slug := range requireBuiltins(t) {
			result, rejection := loader.ResolveByName(slug, dir)

			requireBuiltinResolved(t, result, rejection, slug)
		}
		requireNoThemeRecords(t, sink)
	})

	t.Run("with an absent themes directory", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := filepath.Join(t.TempDir(), "themes")

		for _, slug := range requireBuiltins(t) {
			result, rejection := loader.ResolveByName(slug, dir)

			requireBuiltinResolved(t, result, rejection, slug)
		}
		requireNoThemeRecords(t, sink)
	})

	t.Run("with a shadowing broken file under the built-in's own name", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := t.TempDir()

		for _, slug := range requireBuiltins(t) {
			writeTheme(t, dir, slug+".theme", withoutKey(themeLines(), "bg.subtle"))

			result, rejection := loader.ResolveByName(slug, dir)

			requireBuiltinResolved(t, result, rejection, slug)
		}
		requireNoThemeRecords(t, sink)
	})

	t.Run("with a regular file where the themes directory belongs", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := writeFile(t, t.TempDir(), "themes", "this is not a directory\n")

		for _, slug := range requireBuiltins(t) {
			result, rejection := loader.ResolveByName(slug, dir)

			requireBuiltinResolved(t, result, rejection, slug)
		}
		requireNoThemeRecords(t, sink)
	})
}

// TestResolveByName_DropInResolves pins the ordinary success on the other half
// of §8.4's order: a slug naming no built-in composes `<themesDir>/<slug>.theme`
// and loads it through the same §6.2 ladder every other route runs.
//
// All three fields of the Result are asserted. The slug is the one asked for,
// the palette is the file's parsed contents, and the source is its bytes
// VERBATIM — the last being what `portal theme export` writes to stdout, so a
// resolver that re-read or re-serialised would surface here rather than in the
// command.
func TestResolveByName_DropInResolves(t *testing.T) {
	loader, sink := resolveLoader(t)
	dir := t.TempDir()
	path := writeTheme(t, dir, "nord-lee.theme", themeLines())
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back %s: %v", path, err)
	}

	result, rejection := loader.ResolveByName("nord-lee", dir)

	if rejection != nil {
		t.Fatalf("ResolveByName(nord-lee) = %v, want the drop-in", rejection)
	}
	if result.Slug != "nord-lee" {
		t.Errorf("resolved slug = %q, want %q", result.Slug, "nord-lee")
	}
	if tokens, wantTokens := result.Theme.All(), wantThemeTokens(); !slices.Equal(tokens, wantTokens) {
		t.Errorf("resolved theme = %+v, want %+v", tokens, wantTokens)
	}
	if string(result.Source) != string(want) {
		t.Errorf("resolved source =\n%s\nwant the file's bytes verbatim:\n%s", result.Source, want)
	}
	requireNoThemeRecords(t, sink)
}

// TestResolveByName_AbsentFileIsNotFound pins §5.5's absent-versus-unusable
// discrimination on the FILE, the same one §12.1 draws for export: `not found`
// sends the user to check the filename, `unreadable` sends them to check
// permissions — so reporting the wrong one sends them looking in the wrong
// place.
//
// The four fixtures are four ways a composed path fails to yield bytes, and only
// the first of them is an absence. An absent file is `not found` and is SILENT,
// because a directory that simply does not hold the file is not a
// misconfiguration. An unreadable one is `unreadable` with the OS error, since
// permissions is the actual problem.
//
// The dangling symlink is the sharp case for the REASON: the read fails with
// exactly the errno an absent file's does, so an implementation deciding from
// the READ ERROR alone would call a link that plainly exists "not found". §5.6
// puts a dangling link under `unreadable` — it is a read that failed, not a name
// that is free — and only the name's own existence tells the two apart.
//
// The name the OS itself refuses is the sharp case for the EMISSION. The
// directory is perfectly healthy in all four, so all four are silent — but this
// one is the only one where the lookup FAILS rather than answering, which is the
// state §5.5's `theme: directory unusable` record is reachable from. It fires
// for a denied lookup and for nothing else, so a resolver reading every lookup
// failure as the directory's fault is caught here rather than in the panel,
// where the symptom would be a real unusable-directory sighting silently
// swallowed by the `path`+`reason` dedup.
func TestResolveByName_AbsentFileIsNotFound(t *testing.T) {
	t.Run("an absent file", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := t.TempDir()

		result, rejection := loader.ResolveByName("nord-lee", dir)

		requireLoadRejection(t, result, rejection, theme.ReasonNotFound, "")
		requireNoThemeRecords(t, sink)
	})

	t.Run("an unreadable file", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := t.TempDir()
		path := writeUnreadableTheme(t, dir, "nord-lee.theme")
		osErr := requireDeniedThemeRead(t, path)

		result, rejection := loader.ResolveByName("nord-lee", dir)

		requireLoadRejection(t, result, rejection, theme.ReasonUnreadable, osErr.Error())
		// The DIRECTORY is fine here — one file inside it is not. Attributing
		// this denial to the directory would put a `directory unusable` record
		// on a perfectly good directory and, worse, take up the dedup key the
		// panel's enumeration needs for a real one.
		requireNoThemeRecords(t, sink)
	})

	t.Run("a dangling symlink", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := t.TempDir()
		path := writeDanglingThemeLink(t, dir, "nord-lee.theme")
		_, osErr := os.ReadFile(path)
		if !os.IsNotExist(osErr) {
			t.Fatalf("reading the dangling link reports %v, want a not-exist error — this case would not exercise the distinction", osErr)
		}

		result, rejection := loader.ResolveByName("nord-lee", dir)

		requireLoadRejection(t, result, rejection, theme.ReasonUnreadable, osErr.Error())
		requireNoThemeRecords(t, sink)
	})

	t.Run("a name the OS itself refuses", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := t.TempDir()
		slug := strings.Repeat("x", 300)
		osErr := requireUnnameableThemeRead(t, filepath.Join(dir, slug+".theme"))

		result, rejection := loader.ResolveByName(slug, dir)

		requireLoadRejection(t, result, rejection, theme.ReasonUnreadable, osErr.Error())
		// The DIRECTORY is healthy here — the NAME is what the OS refuses. §5.5
		// defines its record for a directory that is unreadable or is not a
		// directory, and this is neither, so blaming the directory would put a
		// spec-defined WARN on a state the spec defines none for AND consume the
		// `path`+`reason` dedup slot a real sighting from the panel's enumeration
		// needs in the same process.
		requireNoThemeRecords(t, sink)
	})
}

// requireUnnameableThemeRead returns the error the OS reports for reading path,
// and fails unless it is neither an absence nor a denial — the third class of
// lookup failure, which the resolver must attribute to neither a free name nor
// an unusable directory.
//
// A slug is an identity with NO LENGTH BOUND (§5.2 fixes none deliberately), so
// an over-long one is a charset-valid slug the OS still cannot look up. A
// platform whose name limit exceeds the fixture would report an absence instead,
// which is a different case entirely — it cannot stage this one, so the test
// skips rather than asserting the opposite reason.
func requireUnnameableThemeRead(t *testing.T, path string) error {
	t.Helper()

	_, err := os.ReadFile(path)
	if err == nil {
		t.Fatalf("reading %s succeeded — the fixture is nameable, so the assertion over it would be vacuous", path)
	}
	if os.IsNotExist(err) {
		t.Skipf("reading %s reports %v: this platform's name limit accommodates the fixture, so a name it refuses cannot be staged", path, err)
	}
	if errors.Is(err, fs.ErrPermission) {
		t.Fatalf("reading %s reports %v — the fixture must be unnameable, not denied", path, err)
	}
	return err
}

// TestResolveByName_AbsentOrEmptyDirectoryIsNotFound pins §5.5's first row on
// the by-name path: an ABSENT themes directory is the common case, so it is
// completely silent and simply yields `not found`. Portal never creates or seeds
// the directory, and zero drop-ins is not an error.
//
// An EMPTY injected directory string behaves identically, and it is not a
// hypothetical: task 5-7 degrades an unresolvable themes-directory path to the
// empty string precisely so a built-in still resolves and a drop-in slug falls
// back. It must compose no path at all — which is what the second subtest's
// working directory proves, since `filepath.Join("", "nord-lee.theme")` yields a
// RELATIVE path that would resolve against exactly that directory.
func TestResolveByName_AbsentOrEmptyDirectoryIsNotFound(t *testing.T) {
	t.Run("an absent directory", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := filepath.Join(t.TempDir(), "themes")

		result, rejection := loader.ResolveByName("nord-lee", dir)

		requireLoadRejection(t, result, rejection, theme.ReasonNotFound, "")
		requireNoThemeRecords(t, sink)
	})

	t.Run("an empty directory string composes no path", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		cwd := t.TempDir()
		writeTheme(t, cwd, "nord-lee.theme", themeLines())
		t.Chdir(cwd)

		result, rejection := loader.ResolveByName("nord-lee", "")

		requireLoadRejection(t, result, rejection, theme.ReasonNotFound, "")
		requireNoThemeRecords(t, sink)
	})
}

// TestResolveByName_UnusableDirectoryIsUnreadable pins §5.5's other row: a
// themes directory that cannot be read, or a regular file sitting where one
// belongs, is a genuine MISCONFIGURATION — reason `unreadable`, never `not
// found`, because permissions is what the user has to go and fix.
//
// It also pins the log record §5.5 requires from THIS path. The entry is
// emitted by the panel's enumeration and by the construction-time by-name read
// alike, and emitting it from construction too is what gives a user who never
// opens the theme panel a record at all.
//
// The directory holds a perfectly good `nord-lee.theme` in the first case, so
// the verdict is load-bearing rather than incidental: the file is right there
// and the directory's own state is what makes it unreachable.
//
// The two fixtures fail at different syscalls, which is why the expected detail
// is sourced from the OS per case rather than restated: a regular file is caught
// by the directory stat, while an unreadable directory STATS PERFECTLY WELL —
// stat needs no permission on the directory itself — and denies the composed
// path instead. Both are the same misconfiguration to the user, so both carry
// the same reason and the same record.
func TestResolveByName_UnusableDirectoryIsUnreadable(t *testing.T) {
	cases := []struct {
		name       string
		stage      func(t *testing.T) string
		wantDetail func(t *testing.T, dir string) string
	}{
		{
			name:  "an unreadable directory",
			stage: unreadableDir,
			wantDetail: func(t *testing.T, dir string) string {
				return requireDeniedThemeRead(t, filepath.Join(dir, "nord-lee.theme")).Error()
			},
		},
		{
			name: "a regular file where the directory belongs",
			stage: func(t *testing.T) string {
				return writeFile(t, t.TempDir(), "themes", "this is not a directory\n")
			},
			wantDetail: func(t *testing.T, dir string) string {
				_, err := os.ReadDir(dir)
				if err == nil {
					t.Fatalf("os.ReadDir(%q) succeeded — the fixture is not a regular file", dir)
				}
				return err.Error()
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loader, sink := resolveLoader(t)
			dir := tc.stage(t)
			if _, err := os.ReadDir(dir); err == nil {
				t.Fatalf("os.ReadDir(%q) succeeded — the fixture is usable, so the assertion over it would be vacuous", dir)
			}
			wantDetail := tc.wantDetail(t, dir)

			result, rejection := loader.ResolveByName("nord-lee", dir)

			requireLoadRejection(t, result, rejection, theme.ReasonUnreadable, wantDetail)
			if rejection.Err == nil {
				t.Error("rejection carries no Err, want the OS error structured alongside the detail")
			}
			requireDirectoryUnusableRecord(t, sink, dir)
		})
	}
}

// requireDirectoryUnusableRecord fails the test unless exactly one `theme:
// directory unusable` record was emitted for dir, carrying §12.3's `path` and
// `reason` attrs.
func requireDirectoryUnusableRecord(t *testing.T, sink *logtest.Sink, dir string) {
	t.Helper()

	records := sink.Records()
	if len(records) != 1 {
		t.Fatalf("emitted %d records, want exactly 1 directory-unusable record: %+v", len(records), records)
	}
	record := records[0]
	if record.Msg != "directory unusable" {
		t.Errorf("record message = %q, want %q", record.Msg, "directory unusable")
	}
	if got := record.AttrString(t, "path"); got != dir {
		t.Errorf("record path = %q, want %q", got, dir)
	}
	if got := record.AttrString(t, "reason"); got != string(theme.ReasonUnreadable) {
		t.Errorf("record reason = %q, want %q", got, theme.ReasonUnreadable)
	}
}

// TestResolveByName_DirectoryUnusableIsDeduped pins §12.3's per-process dedup on
// the construction-time path, and §5.5's reason for it: the record is emitted by
// the panel's enumeration AND by this by-name read, "deduplicated per process so
// the two never double up".
//
// The first subtest is the by-name path on its own — an adaptive pair resolves
// two slugs at construction and the panel re-enumerates on every open (§5.8), so
// without dedup a broken directory would turn a forensic trail into a running
// commentary.
//
// The second is the one §5.5 is actually written about: the two surfaces sharing
// ONE loader, which is how production wires them. It is also what pins the
// emission identity — the by-name read reports the record against the DIRECTORY,
// not the composed file path, or the dedup key would differ from enumeration's
// and the two would double up after all.
//
// The third keeps the dedup honest about where it lives: state on the INJECTED
// logger, not package state in the leaf, so a second process — modelled here by a
// second event logger — still gets its own record.
func TestResolveByName_DirectoryUnusableIsDeduped(t *testing.T) {
	t.Run("five successive resolutions emit one record", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := unreadableDir(t)

		for range 5 {
			if _, rejection := loader.ResolveByName("nord-lee", dir); rejection == nil {
				t.Fatal("ResolveByName succeeded against an unreadable directory — the dedup assertion would be vacuous")
			}
		}

		requireDirectoryUnusableRecord(t, sink, dir)
	})

	t.Run("enumeration and the by-name read do not double up", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := unreadableDir(t)

		if _, rejection := loader.Enumerate(dir); rejection == nil {
			t.Fatal("Enumerate accepted an unreadable directory — the dedup assertion would be vacuous")
		}
		if _, rejection := loader.ResolveByName("nord-lee", dir); rejection == nil {
			t.Fatal("ResolveByName succeeded against an unreadable directory — the dedup assertion would be vacuous")
		}

		requireDirectoryUnusableRecord(t, sink, dir)
	})

	t.Run("a second event logger emits its own record", func(t *testing.T) {
		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)
		dir := unreadableDir(t)

		for range 2 {
			loader := theme.NewLoader(theme.NewEventLogger(log.For(themeComponent)))
			if _, rejection := loader.ResolveByName("nord-lee", dir); rejection == nil {
				t.Fatal("ResolveByName succeeded against an unreadable directory — the assertion would be vacuous")
			}
		}

		if records := sink.Records(); len(records) != 2 {
			t.Errorf("emitted %d records across two event loggers, want 2 — the dedup state lives on the injected logger: %+v", len(records), records)
		}
	})
}

// TestResolveByName_ContentReasonsPassThrough pins that a drop-in reached BY
// NAME is judged by exactly the ladder a drop-in reached by enumeration is
// (§6.2): the three content reasons come back from LoadFile untouched, detail
// and all.
//
// The expectation is LoadFile's own answer for the same file rather than a
// restated string, which is the whole claim being made — not "the reason is
// `bad syntax`" but "nothing on the by-name path re-derives, re-orders or
// re-frames what the ladder said". A resolver that rebuilt the rejection would
// pass a reason-only assertion and fail this one.
func TestResolveByName_ContentReasonsPassThrough(t *testing.T) {
	cases := []struct {
		name       string
		lines      []string
		wantReason theme.Reason
	}{
		{name: "a duplicate key", lines: append(themeLines(), "text.primary = #010203"), wantReason: theme.ReasonBadSyntax},
		{name: "a bad hex", lines: withValue(themeLines(), "canvas", "blue"), wantReason: theme.ReasonBadColour},
		{name: "a missing token", lines: withoutKey(themeLines(), "bg.subtle"), wantReason: theme.ReasonMissingTokens},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loader, sink := resolveLoader(t)
			dir := t.TempDir()
			path := writeTheme(t, dir, "nord-lee.theme", tc.lines)
			_, want := loader.LoadFile(path)
			if want == nil {
				t.Fatalf("LoadFile(%s) accepted the fixture, want %q", path, tc.wantReason)
			}
			if want.Reason != tc.wantReason {
				t.Fatalf("LoadFile(%s) = %q, want %q — the fixture reaches the wrong rung", path, want.Reason, tc.wantReason)
			}

			result, rejection := loader.ResolveByName("nord-lee", dir)

			requireLoadRejection(t, result, rejection, want.Reason, want.Detail)
			if rejection.Line != want.Line {
				t.Errorf("rejection line = %d, want LoadFile's %d", rejection.Line, want.Line)
			}
			requireNoThemeRecords(t, sink)
		})
	}
}

// TestResolveByName_UnreachableReasonsAreUnreachable pins the two of §6.2's
// reasons that cannot arise on this path. An unreachable arm is only unreachable
// while the thing that makes it so still holds, so the impossibility is asserted
// rather than assumed.
//
//   - `reserved name` is decided from a slug that collides with a built-in, and a
//     built-in slug resolves to the BUILT-IN first (§8.4) — so the file is never
//     opened and the rung is never reached. That is §5.4's no-shadowing property
//     stated as an outcome.
//   - A filename `bad name` is decided from the composed base name, which is
//     always `<valid-slug>.theme`: the slug cleared §5.2's charset rule before any
//     path was composed, and the extension is a constant.
func TestResolveByName_UnreachableReasonsAreUnreachable(t *testing.T) {
	t.Run("a colliding drop-in never reports reserved name", func(t *testing.T) {
		loader, _ := resolveLoader(t)
		dir := t.TempDir()

		for _, slug := range requireBuiltins(t) {
			t.Run(slug, func(t *testing.T) {
				path := writeTheme(t, dir, slug+".theme", themeLines())
				if _, rejection := loader.LoadFile(path); rejection == nil || rejection.Reason != theme.ReasonReservedName {
					t.Fatalf("LoadFile(%s) = %v, want %q — the collision this fixture stages is not real", path, rejection, theme.ReasonReservedName)
				}

				result, rejection := loader.ResolveByName(slug, dir)

				requireBuiltinResolved(t, result, rejection, slug)
			})
		}
	})

	// The structural half: every slug that survives the charset check composes a
	// base name the filename rules accept, so the ladder's first rung cannot fire
	// from this path whatever was persisted.
	t.Run("a composed filename always clears the filename rules", func(t *testing.T) {
		for _, slug := range []string{"a", "0", "nord-lee", "nord-", "n0rd-2", strings.Repeat("x", 200)} {
			t.Run(slug, func(t *testing.T) {
				if !theme.ValidSlug(slug) {
					t.Fatalf("ValidSlug(%q) = false — the fixture is not a slug a path would be composed from", slug)
				}

				got, rejection := theme.SlugFromFilename(slug + ".theme")

				if rejection != nil {
					t.Fatalf("SlugFromFilename(%q) = %v, want no rejection — the composed filename is always <valid-slug>.theme", slug+".theme", rejection)
				}
				if got != slug {
					t.Errorf("SlugFromFilename(%q) = %q, want %q", slug+".theme", got, slug)
				}
			})
		}
	})

	// And the observable half: a valid-but-absent slug is `not found`, never a
	// bad-name one.
	t.Run("a valid absent slug is not found, never bad name", func(t *testing.T) {
		loader, _ := resolveLoader(t)

		result, rejection := loader.ResolveByName("nord-", t.TempDir())

		requireLoadRejection(t, result, rejection, theme.ReasonNotFound, "")
	})
}

// TestResolveByName_NoReadDirAndSingleRead pins §5.7's construction budget: "at
// construction, Portal loads only the nominated themes BY NAME — one file read
// for a constant, two for an adaptive pair. No enumeration." A ReadDir here
// would pay the startup scan §5.7 explicitly rejects, on the cold path, in a
// process that may never open the theme panel at all.
//
// The behavioural half uses the one directory state that tells listing and
// naming apart: SEARCH-ONLY (mode 0111) permits opening a file whose name you
// already know and denies listing outright. A resolver that enumerated could not
// resolve here, so the drop-in coming back IS the proof.
//
// The structural half is a call-graph walk, because "exactly one read" is a
// property of every path through the resolver rather than of one fixture — no
// arrangement of files on disk can demonstrate that a second read did not
// happen.
func TestResolveByName_NoReadDirAndSingleRead(t *testing.T) {
	t.Run("it resolves through a directory that cannot be listed", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := searchOnlyDir(t)

		result, rejection := loader.ResolveByName("nord-lee", dir)

		if rejection != nil {
			t.Fatalf("ResolveByName(nord-lee) = %v — a by-name read needs no listing, so a search-only directory must resolve", rejection)
		}
		if result.Slug != "nord-lee" {
			t.Errorf("resolved slug = %q, want %q", result.Slug, "nord-lee")
		}
		requireNoThemeRecords(t, sink)
	})

	t.Run("it reaches exactly one file read and no directory listing", func(t *testing.T) {
		reads := osCallsReachableFrom(t, "ResolveByName")

		if got := reads["os.ReadFile"]; got != 1 {
			t.Errorf("ResolveByName reaches %d os.ReadFile call sites, want exactly 1 (§5.7: one file read per nominated theme)", got)
		}
		if got := reads["os.ReadDir"]; got != 0 {
			t.Errorf("ResolveByName reaches %d os.ReadDir call sites, want 0 — enumeration belongs to panel open", got)
		}
	})
}

// searchOnlyDir stages a themes directory holding one valid theme and then
// removes its READ bit while leaving its SEARCH bit, so a file whose name is
// already known can be opened and the directory cannot be listed.
//
// It is the fixture that separates the two operations, which no permission-free
// arrangement can. Mode bits do not deny root, so the fixture is impossible
// there and the test skips rather than asserting something the platform will not
// do; the mode is restored on cleanup because t.TempDir's own RemoveAll has to
// list the directory to empty it.
func searchOnlyDir(t *testing.T) string {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("running as root: mode bits do not deny, so an unlistable directory cannot be staged")
	}

	dir := filepath.Join(t.TempDir(), "themes")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	writeTheme(t, dir, "nord-lee.theme", themeLines())

	if err := os.Chmod(dir, 0o111); err != nil {
		t.Fatalf("chmod %s: %v", dir, err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(dir, 0o755); err != nil {
			t.Errorf("restore mode on %s: %v", dir, err)
		}
	})

	if _, err := os.ReadDir(dir); err == nil {
		t.Fatalf("os.ReadDir(%q) succeeded — the fixture is listable, so the assertion over it would be vacuous", dir)
	}
	return dir
}

// osCallsReachableFrom counts the stdlib `os` call sites reachable from the
// named function, by walking the package's own call graph.
//
// The graph is keyed by FUNCTION NAME with no type resolution, so two methods
// sharing a name are treated as one node. That over-approximates reachability,
// which is the safe direction for a guard: it can only ever attribute MORE calls
// to the resolver than it truly makes, so a passing count is a real bound.
//
// It fails on an unknown root, and its callers assert a non-zero read count, so
// neither a renamed function nor a graph that resolved nothing can let it pass
// vacuously.
func osCallsReachableFrom(t *testing.T, root string) map[string]int {
	t.Helper()

	graph := themeCallGraph(t)
	if _, known := graph[root]; !known {
		t.Fatalf("no function named %q in internal/theme — the walk below would prove nothing", root)
	}

	counts := map[string]int{}
	seen := map[string]bool{}
	queue := []string{root}
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if seen[name] {
			continue
		}
		seen[name] = true

		node, known := graph[name]
		if !known {
			continue
		}
		for _, call := range node.stdlib {
			counts[call]++
		}
		queue = append(queue, node.local...)
	}
	return counts
}

// themeCallNode is one function's outgoing edges: the stdlib calls it makes
// itself, and the names it hands off to inside the package.
type themeCallNode struct {
	stdlib []string
	local  []string
}

// themeCallGraph builds the package's call graph from source.
//
// A call is stdlib when its receiver names an IMPORTED PACKAGE in that file, and
// local otherwise — which covers both a bare `helper()` and a method call like
// `l.LoadFile(path)`, the latter keyed by the method name alone.
func themeCallGraph(t *testing.T) map[string]themeCallNode {
	t.Helper()

	graph := map[string]themeCallNode{}
	for _, source := range parseThemeSources(t) {
		imports := importedPackageNames(source.File)
		for _, decl := range source.File.Decls {
			fn, isFunc := decl.(*ast.FuncDecl)
			if !isFunc || fn.Body == nil {
				continue
			}
			node := graph[fn.Name.Name]
			ast.Inspect(fn.Body, func(n ast.Node) bool {
				call, isCall := n.(*ast.CallExpr)
				if !isCall {
					return true
				}
				switch fun := call.Fun.(type) {
				case *ast.Ident:
					node.local = append(node.local, fun.Name)
				case *ast.SelectorExpr:
					if pkg, isIdent := fun.X.(*ast.Ident); isIdent && imports[pkg.Name] {
						node.stdlib = append(node.stdlib, pkg.Name+"."+fun.Sel.Name)
					} else {
						node.local = append(node.local, fun.Sel.Name)
					}
				}
				return true
			})
			graph[fn.Name.Name] = node
		}
	}
	return graph
}

// importedPackageNames returns the names a file's imports are referred to by:
// the alias where one is written, the path's last element otherwise.
func importedPackageNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		name := path[strings.LastIndex(path, "/")+1:]
		if spec.Name != nil {
			name = spec.Name.Name
		}
		names[name] = true
	}
	return names
}

// requireDeniedThemeRead returns the error the OS reports for reading path, and
// fails unless it is a DENIAL rather than an absence — the very distinction the
// assertions over it draw, so a fixture that is really absent must fail loudly
// instead of quietly asserting the opposite reason.
func requireDeniedThemeRead(t *testing.T, path string) error {
	t.Helper()

	_, err := os.ReadFile(path)
	if err == nil {
		t.Fatalf("reading %s succeeded — the unreadable fixture is readable, so the assertion over it would be vacuous", path)
	}
	if os.IsNotExist(err) {
		t.Fatalf("reading %s reports %v — the fixture must be unreadable, not absent", path, err)
	}
	return err
}
