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
	"github.com/leeovery/portal/internal/sourceguard"
	"github.com/leeovery/portal/internal/theme"
	"github.com/leeovery/portal/internal/themetest"
)

func resolveLoader(t *testing.T) (theme.Loader, *logtest.Sink) {
	t.Helper()

	sink := &logtest.Sink{}
	log.SetTestHandler(t, sink)
	return theme.NewLoader(theme.NewEventLogger(log.For(themeComponent))), sink
}

func requireNoThemeRecords(t *testing.T, sink *logtest.Sink) {
	t.Helper()

	if records := sink.Records(); len(records) != 0 {
		t.Errorf("emitted %d records, want none: %+v", len(records), records)
	}
}

func escapeTargetDir(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	themetest.Write(t, root, "evil.theme", themetest.Lines())

	dir := filepath.Join(root, "themes")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	for _, base := range []string{"Nord.theme", "-nord.theme", "nord lee.theme", ".theme"} {
		themetest.Write(t, dir, base, themetest.Lines())
	}
	return dir
}

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

func requireBuiltins(t *testing.T) []string {
	t.Helper()

	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("BuiltinSlugs() is empty — every assertion over the embedded set would be vacuous")
	}
	return slugs
}

func TestResolveByName_BuiltinNeverReadsDirectory(t *testing.T) {
	t.Run("with an unreadable themes directory", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := themesDirWithOneTheme(t)
		_ = themetest.DenyDir(t, dir)

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
			themetest.Write(t, dir, slug+".theme", themetest.WithoutKey(themetest.Lines(), "bg.subtle"))

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

func TestResolveByName_DropInResolves(t *testing.T) {
	loader, sink := resolveLoader(t)
	dir := t.TempDir()
	path := themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
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
		path := themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())
		osErr := themetest.DenyRead(t, path)

		result, rejection := loader.ResolveByName("nord-lee", dir)

		requireLoadRejection(t, result, rejection, theme.ReasonUnreadable, osErr.Error())
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
		requireNoThemeRecords(t, sink)
	})
}

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
		themetest.Write(t, cwd, "nord-lee.theme", themetest.Lines())
		t.Chdir(cwd)

		result, rejection := loader.ResolveByName("nord-lee", "")

		requireLoadRejection(t, result, rejection, theme.ReasonNotFound, "")
		requireNoThemeRecords(t, sink)
	})
}

func TestResolveByName_UnusableDirectoryIsUnreadable(t *testing.T) {
	cases := []struct {
		name       string
		stage      func(t *testing.T) string
		wantDetail func(t *testing.T, dir string) string
	}{
		{
			name: "an unreadable directory",
			stage: func(t *testing.T) string {
				dir := themesDirWithOneTheme(t)
				_ = themetest.DenyDir(t, dir)
				return dir
			},
			wantDetail: func(t *testing.T, dir string) string {
				return osReadError(t, filepath.Join(dir, "nord-lee.theme")).Error()
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

func TestResolveByName_DirectoryUnusableIsDeduped(t *testing.T) {
	t.Run("five successive resolutions emit one record", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := themesDirWithOneTheme(t)
		_ = themetest.DenyDir(t, dir)

		for range 5 {
			if _, rejection := loader.ResolveByName("nord-lee", dir); rejection == nil {
				t.Fatal("ResolveByName succeeded against an unreadable directory — the dedup assertion would be vacuous")
			}
		}

		requireDirectoryUnusableRecord(t, sink, dir)
	})

	t.Run("enumeration and the by-name read do not double up", func(t *testing.T) {
		loader, sink := resolveLoader(t)
		dir := themesDirWithOneTheme(t)
		_ = themetest.DenyDir(t, dir)

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
		dir := themesDirWithOneTheme(t)
		_ = themetest.DenyDir(t, dir)

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

func TestResolveByName_ContentReasonsPassThrough(t *testing.T) {
	cases := []struct {
		name       string
		lines      []string
		wantReason theme.Reason
	}{
		{name: "a duplicate key", lines: append(themetest.Lines(), "text.primary = #010203"), wantReason: theme.ReasonBadSyntax},
		{name: "a bad hex", lines: themetest.LinesWithCanvas("blue"), wantReason: theme.ReasonBadColour},
		{name: "a missing token", lines: themetest.WithoutKey(themetest.Lines(), "bg.subtle"), wantReason: theme.ReasonMissingTokens},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			loader, sink := resolveLoader(t)
			dir := t.TempDir()
			path := themetest.Write(t, dir, "nord-lee.theme", tc.lines)
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

func TestResolveByName_UnreachableReasonsAreUnreachable(t *testing.T) {
	t.Run("a colliding drop-in never reports reserved name", func(t *testing.T) {
		loader, _ := resolveLoader(t)
		dir := t.TempDir()

		for _, slug := range requireBuiltins(t) {
			t.Run(slug, func(t *testing.T) {
				path := themetest.Write(t, dir, slug+".theme", themetest.Lines())
				if _, rejection := loader.LoadFile(path); rejection == nil || rejection.Reason != theme.ReasonReservedName {
					t.Fatalf("LoadFile(%s) = %v, want %q — the collision this fixture stages is not real", path, rejection, theme.ReasonReservedName)
				}

				result, rejection := loader.ResolveByName(slug, dir)

				requireBuiltinResolved(t, result, rejection, slug)
			})
		}
	})

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

	t.Run("a valid absent slug is not found, never bad name", func(t *testing.T) {
		loader, _ := resolveLoader(t)

		result, rejection := loader.ResolveByName("nord-", t.TempDir())

		requireLoadRejection(t, result, rejection, theme.ReasonNotFound, "")
	})
}

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
			t.Errorf("ResolveByName reaches %d os.ReadFile call sites, want exactly 1 (one file read per nominated theme)", got)
		}
		if got := reads["os.ReadDir"]; got != 0 {
			t.Errorf("ResolveByName reaches %d os.ReadDir call sites, want 0 — enumeration belongs to panel open", got)
		}
	})
}

// Drops the read bit but keeps the search bit, so a known filename opens while
// the directory cannot be listed. Mode bits do not deny root, so the fixture is
// impossible there and the test skips; the mode is restored on cleanup because
// t.TempDir's RemoveAll has to list the directory.
func searchOnlyDir(t *testing.T) string {
	t.Helper()

	if os.Geteuid() == 0 {
		t.Skip("running as root: mode bits do not deny, so an unlistable directory cannot be staged")
	}

	dir := filepath.Join(t.TempDir(), "themes")
	if err := os.Mkdir(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	themetest.Write(t, dir, "nord-lee.theme", themetest.Lines())

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

type themeCallNode struct {
	stdlib []string
	local  []string
}

func themeCallGraph(t *testing.T) map[string]themeCallNode {
	t.Helper()

	graph := map[string]themeCallNode{}
	for _, source := range parseThemeSources(t) {
		imports := importedPackageNames(source.File)
		sourceguard.ForEachFuncCall(source.File, func(funcName string, call *ast.CallExpr) bool {
			node := graph[funcName]
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
			graph[funcName] = node
			return true
		})
	}
	return graph
}

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

// Fails if the read succeeds, so no expectation is derived from a fixture that
// is not in the state the case describes.
func osReadError(t *testing.T, path string) error {
	t.Helper()

	_, err := os.ReadFile(path)
	if err == nil {
		t.Fatalf("reading %s succeeded — the assertion over the failed read would be vacuous", path)
	}
	return err
}
