package cmd

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"
	"unicode"

	"github.com/leeovery/portal/cmd/bootstrap"
	"github.com/leeovery/portal/internal/log"
	"github.com/leeovery/portal/internal/logtest"
	"github.com/leeovery/portal/internal/theme"
)

// themeExportRun is one `portal theme export` invocation's whole observable
// footprint: what it wrote to each stream, the error it returned, and how many
// times the bootstrap orchestrator ran.
//
// The bootstrap count rides along on EVERY run rather than living only in the
// exemption test, because "printing a file starts no tmux server" is a property
// of the command, not of one scenario.
type themeExportRun struct {
	stdout         []byte
	stderr         string
	err            error
	bootstrapCalls int
}

// execThemeExport runs `portal theme export <args...>` through the real root
// command with both streams captured and a recording orchestrator injected.
//
// The recorder is injected unconditionally so no test in this file can reach
// real tmux even if the command were to lose its skipTmuxCheck entry: a
// regression fails on the bootstrapCalls assertion rather than by dialling the
// poisoned socket.
func execThemeExport(t *testing.T, args ...string) themeExportRun {
	t.Helper()

	runner := &recordingRunner{}
	bootstrapDeps = &BootstrapDeps{Orchestrator: runner}
	t.Cleanup(func() { bootstrapDeps = nil })

	var stdout, stderr bytes.Buffer
	resetRootCmd()
	rootCmd.SetOut(&stdout)
	rootCmd.SetErr(&stderr)
	rootCmd.SetArgs(append([]string{"theme", "export"}, args...))
	err := rootCmd.Execute()

	return themeExportRun{
		stdout:         stdout.Bytes(),
		stderr:         stderr.String(),
		err:            err,
		bootstrapCalls: runner.calls,
	}
}

// requireCommentedSource fails unless the expected bytes actually carry the two
// things byte-equality is being used to prove: a `#` comment and a trailing
// newline.
//
// Without it, a comparison against a re-serialisation-shaped expectation would
// pass vacuously — the point of the verbatim contract is that comments and the
// final newline survive, so the fixture must have some to lose.
func requireCommentedSource(t *testing.T, source []byte) {
	t.Helper()

	if !bytes.Contains(source, []byte("#")) {
		t.Fatalf("fixture carries no # comment, so a verbatim assertion over it proves nothing:\n%s", source)
	}
	if !bytes.HasSuffix(source, []byte("\n")) {
		t.Fatalf("fixture has no trailing newline, so a verbatim assertion over it proves nothing:\n%s", source)
	}
}

// validThemeSource returns the shipped dark built-in's bytes.
//
// It serves two jobs, and they are the same bytes in each: contents for a
// drop-in fixture that is valid by construction — a built-in rather than a
// hand-written palette, so a fixture cannot drift out of validity as the token
// vocabulary evolves and no hex literal is restated in a Go test — and the
// expected output of exporting that built-in.
func validThemeSource(t *testing.T) []byte {
	t.Helper()

	source, found := theme.BuiltinBytes(theme.DefaultDarkSlug)
	if !found {
		t.Fatalf("BuiltinBytes(%q) reports not found", theme.DefaultDarkSlug)
	}
	return source
}

// useThemesDir points PORTAL_THEMES_DIR at a fresh empty directory and returns
// it. An empty-but-present directory is the unknown-slug state: the directory
// resolves and reads, and the composed filename simply is not in it.
func useThemesDir(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	t.Setenv("PORTAL_THEMES_DIR", dir)
	return dir
}

// seedThemesDir writes one drop-in into a fresh themes directory, points
// PORTAL_THEMES_DIR at it, and returns the directory.
func seedThemesDir(t *testing.T, slug string, source []byte) string {
	t.Helper()

	dir := useThemesDir(t)
	if err := os.WriteFile(filepath.Join(dir, slug+".theme"), source, 0o644); err != nil {
		t.Fatalf("seed %s.theme: %v", slug, err)
	}
	return dir
}

// TestThemeExport_IsBootstrapExempt: it is bootstrap-exempt.
//
// §12.1 puts export in skipTmuxCheck: printing a file must not start a tmux
// server, ensure the saver or run restore. Both halves are asserted — the
// allowlist entry, and the observable consequence that the ten-step
// orchestrator never runs — because the entry alone would not catch a command
// registered under a name the allowlist does not key on.
func TestThemeExport_IsBootstrapExempt(t *testing.T) {
	if !skipTmuxCheck["theme"] {
		t.Error(`skipTmuxCheck["theme"] = false; want true (printing a file starts no tmux server)`)
	}

	run := execThemeExport(t, theme.DefaultDarkSlug)

	if run.err != nil {
		t.Fatalf("theme export %s returned %v (stderr: %q)", theme.DefaultDarkSlug, run.err, run.stderr)
	}
	if run.bootstrapCalls != 0 {
		t.Errorf("orchestrator Run call count = %d, want 0", run.bootstrapCalls)
	}
}

// TestThemeExport_BuiltinBytesAreVerbatim: it writes a built-in's bytes
// verbatim.
//
// §12.1: the output is the FILE's bytes, comments included — never a
// re-serialisation of the parsed Theme, which would drop the attribution header
// and the eyeball-pin derivation notes that are the only surviving record of a
// judgement no test can re-derive.
//
// The loop is over BuiltinSlugs() rather than a named theme so a built-in added
// by a later PR is covered with no test edit, and it fails on an empty set so
// the suite cannot pass vacuously.
func TestThemeExport_BuiltinBytesAreVerbatim(t *testing.T) {
	slugs := theme.BuiltinSlugs()
	if len(slugs) == 0 {
		t.Fatal("BuiltinSlugs() is empty — every assertion below would be vacuous")
	}

	for _, slug := range slugs {
		t.Run(slug, func(t *testing.T) {
			want, found := theme.BuiltinBytes(slug)
			if !found {
				t.Fatalf("BuiltinBytes(%q) reports not found", slug)
			}
			requireCommentedSource(t, want)

			run := execThemeExport(t, slug)

			if run.err != nil {
				t.Fatalf("theme export %s returned %v (stderr: %q)", slug, run.err, run.stderr)
			}
			if !bytes.Equal(run.stdout, want) {
				t.Errorf("theme export %s wrote:\n%s\nwant the embedded bytes verbatim:\n%s", slug, run.stdout, want)
			}
		})
	}
}

// TestThemeExport_DropInBytesAreVerbatim: it writes a drop-in's bytes verbatim.
//
// The slug domain is built-ins AND drop-ins (§12.1), which is what makes export
// a diagnosis tool — "show me the file Portal read" — rather than only an
// on-ramp. The no-trailing-newline case is the one that catches a Fprintln-
// shaped implementation: §12.1 needs no separate decision about the final
// newline precisely because whatever the file holds is what is written.
func TestThemeExport_DropInBytesAreVerbatim(t *testing.T) {
	t.Run("with a trailing newline", func(t *testing.T) {
		want := validThemeSource(t)
		requireCommentedSource(t, want)
		seedThemesDir(t, "nord-lee", want)

		run := execThemeExport(t, "nord-lee")

		if run.err != nil {
			t.Fatalf("theme export nord-lee returned %v (stderr: %q)", run.err, run.stderr)
		}
		if !bytes.Equal(run.stdout, want) {
			t.Errorf("theme export nord-lee wrote:\n%s\nwant the file's bytes verbatim:\n%s", run.stdout, want)
		}
	})

	t.Run("without a trailing newline", func(t *testing.T) {
		want := bytes.TrimRight(validThemeSource(t), "\n")
		if bytes.HasSuffix(want, []byte("\n")) {
			t.Fatal("fixture still ends in a newline — the no-newline case would be vacuous")
		}
		seedThemesDir(t, "nord-lee", want)

		run := execThemeExport(t, "nord-lee")

		if run.err != nil {
			t.Fatalf("theme export nord-lee returned %v (stderr: %q)", run.err, run.stderr)
		}
		if !bytes.Equal(run.stdout, want) {
			t.Errorf("theme export nord-lee wrote %d bytes ending %q, want the file's %d bytes ending %q — no newline may be added or stripped",
				len(run.stdout), lastBytes(run.stdout), len(want), lastBytes(want))
		}
	})
}

// lastBytes returns the tail of b, for failure messages about trailing bytes.
func lastBytes(b []byte) string {
	const tail = 24
	if len(b) > tail {
		return string(b[len(b)-tail:])
	}
	return string(b)
}

// themeKeyLines returns the built-in's `key = value` lines, comments and blanks
// dropped.
//
// Every fixture below is derived from them rather than hand-written, so no hex
// value is restated in Go and a fixture cannot drift out of validity as the
// token vocabulary evolves. It fails on a degenerate built-in so no derived
// fixture can be vacuous.
func themeKeyLines(t *testing.T) []string {
	t.Helper()

	var pairs []string
	for line := range strings.SplitSeq(string(validThemeSource(t)), "\n") {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		pairs = append(pairs, text)
	}
	if len(pairs) < 2 {
		t.Fatalf("built-in yielded %d key lines — the fixtures derived from them need at least two", len(pairs))
	}
	return pairs
}

// scrambledThemeSource builds a valid drop-in whose keys are in no canonical
// order and whose comments are interleaved between them.
//
// It is derived from a built-in — the key lines reversed, a comment and a blank
// line between each — so the fixture is guaranteed to parse and restates no hex
// value in Go. Reversal is deliberate: §2.7 makes file ordering carry nothing,
// so a re-serialising implementation would be free to emit its own order, and
// this is the fixture that catches it doing so.
func scrambledThemeSource(t *testing.T) []byte {
	t.Helper()

	pairs := slices.Clone(themeKeyLines(t))
	slices.Reverse(pairs)

	var b strings.Builder
	b.WriteString("# nord-lee — a hand-edited copy\n#\n# Its keys are in no canonical order and its notes sit between them.\n\n")
	for i, pair := range pairs {
		fmt.Fprintf(&b, "# note %d\n%s\n\n", i, pair)
	}
	return []byte(b.String())
}

// TestThemeExport_IsNotAReserialisation: it preserves comments and key order.
//
// §12.1's whole point: what is written is the SOURCE FILE, not a
// re-serialisation of the parsed Theme. A re-serialisation would drop every
// comment and impose its own key order — so a file that carries neither the
// canonical order nor decoration-free content is the one that tells the two
// implementations apart.
func TestThemeExport_IsNotAReserialisation(t *testing.T) {
	want := scrambledThemeSource(t)
	if bytes.Equal(want, validThemeSource(t)) {
		t.Fatal("the scrambled fixture equals the built-in — it would not distinguish a re-serialisation")
	}
	seedThemesDir(t, "nord-lee", want)

	run := execThemeExport(t, "nord-lee")

	if run.err != nil {
		t.Fatalf("theme export nord-lee returned %v (stderr: %q)", run.err, run.stderr)
	}
	if !bytes.Equal(run.stdout, want) {
		t.Errorf("theme export nord-lee wrote:\n%s\nwant the file's bytes verbatim (comments and key order intact):\n%s", run.stdout, want)
	}
}

// TestThemeExport_BuiltinNeverReadsThemesDirectory: it resolves the embedded
// set before the themes directory.
//
// §8.4's ordering rule, which export is the fourth by-name resolver to inherit:
// a slug naming a built-in resolves to the built-in and never reads the themes
// directory at all.
//
// Three subtests, each closing what the one before it leaves open. An
// unreadable directory is the first observation; the second proves mode 0000
// actually denies this process (it would not, running as root), so the first is
// evidence of ordering rather than of a chmod that did nothing; and the third
// separates "never reads it" from "reads it, then falls back", which the first
// two cannot tell apart.
func TestThemeExport_BuiltinNeverReadsThemesDirectory(t *testing.T) {
	t.Run("a built-in resolves through an unreadable themes directory", func(t *testing.T) {
		unreadableThemesDir(t, "nord-lee")
		want := validThemeSource(t)

		run := execThemeExport(t, theme.DefaultDarkSlug)

		if run.err != nil {
			t.Fatalf("theme export %s returned %v — the embedded set must resolve before the themes directory", theme.DefaultDarkSlug, run.err)
		}
		if !bytes.Equal(run.stdout, want) {
			t.Errorf("theme export %s wrote:\n%s\nwant the embedded bytes verbatim:\n%s", theme.DefaultDarkSlug, run.stdout, want)
		}
	})

	t.Run("the directory really is unreadable", func(t *testing.T) {
		unreadableThemesDir(t, "nord-lee")

		run := execThemeExport(t, "nord-lee")

		if run.err == nil {
			t.Fatal("theme export nord-lee succeeded against a mode-0000 directory — the ordering assertion above would be vacuous")
		}
	})

	// The sharp discriminator. An unreadable directory alone does not separate
	// "never reads it" from "reads it, then falls back to the built-in" — both
	// end up printing the embedded bytes. Making the directory UNLOCATABLE does:
	// with no env var, no XDG_CONFIG_HOME and no HOME, themesDirPath cannot
	// answer at all, so any implementation that resolves the directory before
	// the embedded set surfaces that error instead of the theme.
	t.Run("a built-in resolves when the themes directory cannot be located", func(t *testing.T) {
		t.Setenv("PORTAL_THEMES_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")
		if _, err := themesDirPath(); err == nil {
			t.Fatal("themesDirPath resolved with no env var, no XDG_CONFIG_HOME and no HOME — this subtest would be vacuous")
		}
		want := validThemeSource(t)

		run := execThemeExport(t, theme.DefaultDarkSlug)

		if run.err != nil {
			t.Fatalf("theme export %s returned %v — a built-in must resolve without the themes directory being located at all", theme.DefaultDarkSlug, run.err)
		}
		if !bytes.Equal(run.stdout, want) {
			t.Errorf("theme export %s wrote:\n%s\nwant the embedded bytes verbatim:\n%s", theme.DefaultDarkSlug, run.stdout, want)
		}
	})
}

// TestThemeExport_ExactArgsOne: it takes exactly one slug.
//
// §12.1 fixes the arity at one, and an arity violation is deliberately OUTSIDE
// §14A's four refusal frames — it is a Cobra usage error, inheriting Portal's
// existing behaviour for arg-count errors. What matters here is that no output
// is produced: a redirect must never capture a usage complaint into the file
// the user is creating.
func TestThemeExport_ExactArgsOne(t *testing.T) {
	t.Run("the declared validator accepts one argument and nothing else", func(t *testing.T) {
		cases := []struct {
			name     string
			args     []string
			accepted bool
		}{
			{name: "zero", args: nil, accepted: false},
			{name: "one", args: []string{"tokyo-night"}, accepted: true},
			{name: "two", args: []string{"tokyo-night", "nord"}, accepted: false},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				err := themeExportCmd.Args(themeExportCmd, tc.args)
				if tc.accepted && err != nil {
					t.Errorf("Args(%v) = %v, want nil", tc.args, err)
				}
				if !tc.accepted && err == nil {
					t.Errorf("Args(%v) = nil, want a usage error", tc.args)
				}
			})
		}
	})

	t.Run("a wrong arity writes nothing to stdout", func(t *testing.T) {
		for _, args := range [][]string{{}, {"tokyo-night", "nord"}} {
			run := execThemeExport(t, args...)

			if run.err == nil {
				t.Errorf("theme export %v returned nil, want a usage error", args)
			}
			if len(run.stdout) != 0 {
				t.Errorf("theme export %v wrote %q to stdout, want nothing", args, run.stdout)
			}
		}
	})
}

// TestThemeExport_EmitsNoThemeEvents: it emits no theme log events.
//
// §12.3: the `theme` component records where a theme is USED, never where one
// is DIAGNOSED. Export is the user looking — its whole output is already the
// diagnostic on the screen they are reading — so the loader is handed
// log.Discard() and nothing reaches the log on any path, success or refusal.
//
// The last subtest keeps the rest honest: it proves the installed sink DOES
// capture a `theme` event when the loader is given a real component logger, so
// the zero-record assertions are evidence about export rather than about a
// deaf harness.
func TestThemeExport_EmitsNoThemeEvents(t *testing.T) {
	// seed is nil where the case needs no themes directory at all, which is the
	// state the built-in, unknown-slug and bad-name cases each want.
	cases := []struct {
		name string
		slug string
		seed func(*testing.T)
	}{
		{name: "a built-in", slug: theme.DefaultDarkSlug},
		{
			name: "a valid drop-in",
			slug: "nord-lee",
			seed: func(t *testing.T) { seedThemesDir(t, "nord-lee", validThemeSource(t)) },
		},
		{
			name: "an invalid drop-in",
			slug: "nord-lee",
			seed: func(t *testing.T) { seedThemesDir(t, "nord-lee", sourceMissingTokens(t, "text.primary")) },
		},
		{name: "an unknown slug", slug: "no-such-theme"},
		{name: "a slug failing the charset check", slug: "Nord"},
		{
			name: "an unreadable drop-in",
			slug: "mine",
			seed: func(t *testing.T) { unreadableThemeFile(t, "mine") },
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.seed != nil {
				tc.seed(t)
			}

			sink := &logtest.Sink{}
			log.SetTestHandler(t, sink)

			execThemeExport(t, tc.slug)

			if records := sink.Records(); len(records) != 0 {
				t.Errorf("theme export %s emitted %d log records, want none: %+v", tc.slug, len(records), records)
			}
		})
	}

	t.Run("the sink captures a theme event when one is emitted", func(t *testing.T) {
		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		theme.NewEventLogger(log.For("theme")).Rejected("mine", "", &theme.Rejection{Reason: theme.ReasonBadColour})

		if records := sink.Records(); len(records) != 1 {
			t.Fatalf("the capture harness recorded %d theme events, want 1 — the assertions above would be vacuous: %+v", len(records), records)
		}
	})
}

// snapshotTree fingerprints every file under root as path -> mode + content
// hash, so a test can assert a command created, deleted or rewrote nothing.
func snapshotTree(t *testing.T, root string) map[string]string {
	t.Helper()

	tree := map[string]string{}
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		info, infoErr := entry.Info()
		if infoErr != nil {
			return infoErr
		}
		if entry.IsDir() {
			tree[rel] = fmt.Sprintf("dir %v", info.Mode())
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		tree[rel] = fmt.Sprintf("file %v %x", info.Mode(), sha256.Sum256(data))
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return tree
}

// TestThemeExport_ReadsNoPrefs: it never reads prefs.
//
// §10.5: export does not read prefs.json AT ALL. Its argument is a slug, so the
// theme setting never enters — side-effect-freedom by construction rather than
// by carve-out. Three independent proofs, because no one of them is sufficient
// alone: an unreadable prefs.json that would surface as an error, a source scan
// that catches a read whose error is swallowed, and a whole-tree fingerprint
// that catches a write.
func TestThemeExport_ReadsNoPrefs(t *testing.T) {
	t.Run("it succeeds against an unreadable prefs.json", func(t *testing.T) {
		prefsPath := filepath.Join(t.TempDir(), "prefs.json")
		before := []byte(`{"session_list_mode":"by-tag","appearance":"light"}`)
		if err := os.WriteFile(prefsPath, before, 0o600); err != nil {
			t.Fatalf("seed prefs.json: %v", err)
		}
		if err := os.Chmod(prefsPath, 0o000); err != nil {
			t.Fatalf("chmod 0000 prefs.json: %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(prefsPath, 0o600) })
		t.Setenv("PORTAL_PREFS_FILE", prefsPath)

		run := execThemeExport(t, theme.DefaultDarkSlug)

		if run.err != nil {
			t.Fatalf("theme export %s returned %v — prefs.json must never be read", theme.DefaultDarkSlug, run.err)
		}

		if err := os.Chmod(prefsPath, 0o600); err != nil {
			t.Fatalf("chmod 0600 prefs.json: %v", err)
		}
		after, err := os.ReadFile(prefsPath)
		if err != nil {
			t.Fatalf("read back prefs.json: %v", err)
		}
		if !bytes.Equal(after, before) {
			t.Errorf("prefs.json = %q, want it untouched: %q", after, before)
		}
	})

	t.Run("the export command names no prefs symbol", func(t *testing.T) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(".", "theme.go"), nil, 0)
		if err != nil {
			t.Fatalf("parse cmd/theme.go: %v", err)
		}

		banned := map[string]string{
			"prefsFilePath":  "resolves the prefs.json path",
			"loadPrefsStore": "opens the prefs store",
		}
		ast.Inspect(file, func(n ast.Node) bool {
			switch node := n.(type) {
			case *ast.ImportSpec:
				path, uerr := strconv.Unquote(node.Path.Value)
				if uerr == nil && strings.HasSuffix(path, "/internal/prefs") {
					t.Errorf("cmd/theme.go imports %s — export never reads prefs.json", path)
				}
			case *ast.Ident:
				if why, isBanned := banned[node.Name]; isBanned {
					t.Errorf("cmd/theme.go:%d names %s, which %s — export never reads prefs.json", fset.Position(node.Pos()).Line, node.Name, why)
				}
			}
			return true
		})
	})

	t.Run("it writes no file anywhere", func(t *testing.T) {
		root := t.TempDir()
		t.Setenv("HOME", root)
		t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "xdg"))
		t.Setenv("PORTAL_PREFS_FILE", filepath.Join(root, "prefs.json"))
		themes := filepath.Join(root, "themes")
		if err := os.MkdirAll(themes, 0o755); err != nil {
			t.Fatalf("create themes dir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(themes, "nord-lee.theme"), validThemeSource(t), 0o644); err != nil {
			t.Fatalf("seed nord-lee.theme: %v", err)
		}
		t.Setenv("PORTAL_THEMES_DIR", themes)

		before := snapshotTree(t, root)
		if len(before) == 0 {
			t.Fatal("snapshot of the config root is empty — the comparison below would be vacuous")
		}

		for _, slug := range []string{theme.DefaultDarkSlug, "nord-lee", "no-such-theme"} {
			execThemeExport(t, slug)
		}

		if after := snapshotTree(t, root); !maps.Equal(after, before) {
			t.Errorf("the config tree changed:\nbefore: %v\nafter:  %v", before, after)
		}
	})
}

// themeSourceFromLines renders a set of `key = value` lines as one theme file.
func themeSourceFromLines(lines []string) []byte {
	return []byte(strings.Join(lines, "\n") + "\n")
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

// themeOverride replaces one key's value in a fixture source, leaving the key
// and its FILE POSITION alone — which is what makes a `bad colour` fixture's
// offender order predictable, since offenders are enumerated in file order.
type themeOverride struct{ key, value string }

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

// unreadableThemeFile seeds a valid drop-in and then makes THE FILE unreadable,
// returning its path.
func unreadableThemeFile(t *testing.T, slug string) string {
	t.Helper()

	path := filepath.Join(seedThemesDir(t, slug, validThemeSource(t)), slug+".theme")
	if err := os.Chmod(path, 0o000); err != nil {
		t.Fatalf("chmod 0000 %s: %v", path, err)
	}
	t.Cleanup(func() { _ = os.Chmod(path, 0o644) })
	return path
}

// unreadableThemesDir seeds a valid drop-in and then makes THE DIRECTORY
// unreadable, returning the directory.
//
// The mode is restored on cleanup because t.TempDir's own RemoveAll cannot
// descend into a mode-0000 directory, and cleanups run last-registered-first.
func unreadableThemesDir(t *testing.T, slug string) string {
	t.Helper()

	dir := seedThemesDir(t, slug, validThemeSource(t))
	if err := os.Chmod(dir, 0o000); err != nil {
		t.Fatalf("chmod 0000 %s: %v", dir, err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	return dir
}

// osReadError returns the error the OS reports for reading path, and fails the
// test if the read SUCCEEDS — a fixture that is actually readable would make
// every `could not be read` assertion over it evidence about the wrong thing.
func osReadError(t *testing.T, path string) error {
	t.Helper()

	if _, err := os.ReadFile(path); err != nil {
		return err
	}
	t.Fatalf("reading %s succeeded — the unreadable fixture is readable, so the assertion over it would be vacuous", path)
	return nil
}

// requireDeniedRead is osReadError for the mode-0000 fixtures, with the
// root-user escape closed: mode 0000 denies nothing to root, and the read then
// fails (if at all) as ABSENT — which is the very distinction these tests exist
// to pin, so a denial that is really an absence must fail loudly rather than
// quietly assert the opposite frame.
func requireDeniedRead(t *testing.T, path string) error {
	t.Helper()

	err := osReadError(t, path)
	if os.IsNotExist(err) {
		t.Fatalf("reading %s reports %v — the fixture must be unreadable, not absent", path, err)
	}
	return err
}

// requireExportRefusal fails unless the run refused with EXACTLY §14A's pinned
// frame and wrote nothing to stdout.
//
// Both halves are the contract. The frame is compared whole rather than by
// substring because it IS the user's entire answer, and the four classes send
// them to four different places. The empty stdout is asserted alongside it
// because export is a pipe-into-a-file tool: a byte on the wrong stream lands
// inside the theme file the user just created.
func requireExportRefusal(t *testing.T, run themeExportRun, want string) {
	t.Helper()

	if run.err == nil {
		t.Fatalf("theme export succeeded, want the refusal %q", want)
	}
	if got := run.err.Error(); got != want {
		t.Errorf("refusal = %q, want §14A's frame %q", got, want)
	}
	if len(run.stdout) != 0 {
		t.Errorf("stdout = %q, want nothing — a redirect would capture the refusal into the user's theme file", run.stdout)
	}
}

// requireOrdinaryError fails unless err is what main.classify maps to exit 1
// with its message PRINTED.
//
// classify has exactly four arms, so ruling out three pins the fourth: a
// *bootstrap.FatalError exits 1 but suppresses stderr (Execute already printed
// it), a silent-exit sentinel exits 1 printing nothing, and a *UsageError exits
// 2. §12.1 fixes export's failure exit code at 1 for every class, with the
// reason string on stderr doing the discriminating — so an export refusal must
// be none of the three.
func requireOrdinaryError(t *testing.T, err error) {
	t.Helper()

	if err == nil {
		t.Fatal("theme export returned nil, want a refusal")
	}

	var fatal *bootstrap.FatalError
	if errors.As(err, &fatal) {
		t.Errorf("refusal %q is a *bootstrap.FatalError — classify would suppress its message", err)
	}
	if IsSilentExitError(err) {
		t.Errorf("refusal %q is a silent-exit sentinel — classify would print nothing, and the reason string is the whole answer", err)
	}
	var usage *UsageError
	if errors.As(err, &usage) {
		t.Errorf("refusal %q is a *UsageError — classify would exit 2, and §12.1 fixes every failure class at 1", err)
	}
}

// themeExportFailure is one of §14A's four refusal classes as a fixture: the
// state that provokes it, and the argument that reaches it.
type themeExportFailure struct {
	name string
	slug string
	seed func(t *testing.T)
}

// themeExportFailures enumerates the four classes ONCE, so the exit-code and
// empty-stdout guarantees are stated over the closed set rather than over
// whichever cases a later reader happened to copy.
func themeExportFailures() []themeExportFailure {
	return []themeExportFailure{
		{
			name: "an unknown slug",
			slug: "no-such-theme",
			seed: func(t *testing.T) { useThemesDir(t) },
		},
		{
			name: "an invalid drop-in",
			slug: "mine",
			seed: func(t *testing.T) { seedThemesDir(t, "mine", sourceMissingTokens(t, "text.primary")) },
		},
		{
			name: "a slug failing the charset check",
			slug: "Nord",
			seed: func(t *testing.T) { useThemesDir(t) },
		},
		{
			name: "an unreadable drop-in",
			slug: "mine",
			seed: func(t *testing.T) { unreadableThemeFile(t, "mine") },
		},
	}
}

// TestThemeExport_UnknownSlugFrame: it refuses an unknown slug with the pinned
// frame.
//
// §14A: `no theme named <slug>`. The reason BEHIND the case is §6.2's `not
// found`, but the label is deliberately not printed — the frame is a sentence
// about the name the user typed, which is the thing they have to go and fix.
func TestThemeExport_UnknownSlugFrame(t *testing.T) {
	t.Run("with an empty themes directory", func(t *testing.T) {
		useThemesDir(t)

		requireExportRefusal(t, execThemeExport(t, "nope"), "no theme named nope")
	})

	t.Run("with an absent themes directory", func(t *testing.T) {
		t.Setenv("PORTAL_THEMES_DIR", filepath.Join(t.TempDir(), "never-created"))

		requireExportRefusal(t, execThemeExport(t, "nope"), "no theme named nope")
	})
}

// TestThemeExport_InvalidDropInFrame: it refuses an invalid drop-in with its
// reason.
//
// §14A: `theme <slug> is not valid: <reason>`, where the reason is §6.2's terse
// label VERBATIM. The table is the three content reasons a by-name read can
// reach, and each fixture is derived from a built-in so a wrong fixture surfaces
// as the wrong reason in the message rather than passing quietly.
func TestThemeExport_InvalidDropInFrame(t *testing.T) {
	cases := []struct {
		name   string
		source func(*testing.T) []byte
		want   string
	}{
		{
			name:   "a duplicate key",
			source: func(t *testing.T) []byte { return sourceDuplicateKeyAt(t, 12, "text.primary") },
			want:   "theme mine is not valid: bad syntax",
		},
		{
			name:   "a bad hex",
			source: func(t *testing.T) []byte { return sourceBadColours(t, themeOverride{"canvas", "blue"}) },
			want:   "theme mine is not valid: bad colour",
		},
		{
			name:   "a missing token",
			source: func(t *testing.T) []byte { return sourceMissingTokens(t, "text.primary") },
			want:   "theme mine is not valid: missing tokens",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			seedThemesDir(t, "mine", tc.source(t))

			requireExportRefusal(t, execThemeExport(t, "mine"), tc.want)
		})
	}
}

// TestThemeExport_BadNameFrame: it refuses a charset-failing slug as bad name.
//
// §14A: `theme <slug> is not valid: bad name`, NOT `no theme named <slug>` —
// telling a user their file is missing when they typed an illegal name sends
// them looking in the wrong place (§12.1).
//
// The arguments arrive after `--`. A bare `-nord` never reaches the command at
// all: pflag claims it as a shorthand cluster and fails, which is Cobra's usage
// error and outside these four frames exactly as an arity violation is. The
// last subtest pins that boundary rather than leaving it to be discovered.
func TestThemeExport_BadNameFrame(t *testing.T) {
	t.Run("the argument is echoed back in the frame", func(t *testing.T) {
		cases := []struct {
			name string
			arg  string
			want string
		}{
			{name: "a traversal attempt", arg: "../evil", want: "theme ../evil is not valid: bad name"},
			{name: "a leading hyphen", arg: "-nord", want: "theme -nord is not valid: bad name"},
			{name: "the wrong case", arg: "Nord", want: "theme Nord is not valid: bad name"},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				useThemesDir(t)

				requireExportRefusal(t, execThemeExport(t, "--", tc.arg), tc.want)
			})
		}
	})

	// The sharp one: a themes directory nested one level down, with a valid
	// theme sitting in its PARENT. If the argument were ever joined onto the
	// directory, `../evil` would resolve to that file and its bytes would land
	// on stdout — which is the traversal §5.2's charset rule exists to stop.
	t.Run("no path is composed from the argument", func(t *testing.T) {
		root := t.TempDir()
		escaped := validThemeSource(t)
		if err := os.WriteFile(filepath.Join(root, "evil.theme"), escaped, 0o644); err != nil {
			t.Fatalf("seed evil.theme: %v", err)
		}
		nested := filepath.Join(root, "themes")
		if err := os.MkdirAll(nested, 0o755); err != nil {
			t.Fatalf("create nested themes dir: %v", err)
		}
		t.Setenv("PORTAL_THEMES_DIR", nested)

		run := execThemeExport(t, "--", "../evil")

		requireExportRefusal(t, run, "theme ../evil is not valid: bad name")
		if bytes.Contains(run.stdout, escaped) {
			t.Error("stdout carries the escape target's bytes — a path was composed from the argument")
		}
	})

	// The charset check runs ahead of the themes directory being LOCATED, not
	// merely ahead of it being read: with no env var, no XDG_CONFIG_HOME and no
	// HOME, any implementation that resolved the directory first would surface
	// that failure instead of the bad-name frame.
	t.Run("the charset check runs before the directory is located", func(t *testing.T) {
		t.Setenv("PORTAL_THEMES_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")
		if _, err := themesDirPath(); err == nil {
			t.Fatal("themesDirPath resolved with no env var, no XDG_CONFIG_HOME and no HOME — this subtest would be vacuous")
		}

		requireExportRefusal(t, execThemeExport(t, "Nord"), "theme Nord is not valid: bad name")
	})

	t.Run("a bare leading hyphen is Cobra's usage error, not a frame", func(t *testing.T) {
		useThemesDir(t)

		run := execThemeExport(t, "-nord")

		if run.err == nil {
			t.Error("theme export -nord returned nil, want pflag's shorthand-cluster error")
		}
		if len(run.stdout) != 0 {
			t.Errorf("stdout = %q, want nothing", run.stdout)
		}
	})
}

// TestThemeExport_UnreadableFrame: it refuses an unreadable file or directory
// with the OS error.
//
// §14A keeps this frame SEPARATE from `is not valid` on purpose: nothing was
// read, so "is not valid" would describe a judgement that was never made. The
// expected OS error is obtained by performing the same read from the test, so
// the assertion pins "verbatim" without hard-coding a platform's wording.
func TestThemeExport_UnreadableFrame(t *testing.T) {
	t.Run("an unreadable file", func(t *testing.T) {
		osErr := requireDeniedRead(t, unreadableThemeFile(t, "mine"))

		requireExportRefusal(t, execThemeExport(t, "mine"), "theme mine could not be read: "+osErr.Error())
	})

	t.Run("an unreadable directory", func(t *testing.T) {
		dir := unreadableThemesDir(t, "mine")
		osErr := requireDeniedRead(t, filepath.Join(dir, "mine.theme"))

		requireExportRefusal(t, execThemeExport(t, "mine"), "theme mine could not be read: "+osErr.Error())
	})

	// A themes directory that cannot even be LOCATED lands in this same frame
	// (§5.5): from the user's side it is the identical fact — the theme could
	// not be read, and here is the system's reason. Surfacing the resolution
	// error raw instead would put a fifth, unpinned sentence on a surface §14A
	// closes at four, which is exactly what this subtest makes impossible.
	//
	// The expectation is derived from themesDirPath's own error rather than
	// spelling out `$HOME is not defined`, for the same reason the two subtests
	// above source their wording from the OS: the frame's contract is that the
	// system's reason rides through verbatim, not that it reads any particular
	// way on any particular platform.
	t.Run("a themes directory that cannot be located", func(t *testing.T) {
		t.Setenv("PORTAL_THEMES_DIR", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", "")
		_, err := themesDirPath()
		if err == nil {
			t.Fatal("themesDirPath resolved with no env var, no XDG_CONFIG_HOME and no HOME — this subtest would be vacuous")
		}

		requireExportRefusal(t, execThemeExport(t, "nord-lee"), "theme nord-lee could not be read: "+err.Error())
	})
}

// TestThemeExport_AbsentIsNotUnreadable: it distinguishes absent from
// unreadable.
//
// §12.1's row, inherited from §5.5: `not found` sends the user to check the
// filename, `unreadable` sends them to check permissions — and against a themes
// directory they cannot read, permissions is the actual problem. Printing "no
// theme named nord-lee" about a file that plainly exists is the misdirection
// this test exists to make impossible.
//
// The dangling symlink is the case that separates "the read failed with ENOENT"
// from "there is nothing at this name": the read reports ENOENT either way, and
// only the name's own existence tells them apart.
func TestThemeExport_AbsentIsNotUnreadable(t *testing.T) {
	t.Run("an absent file is no theme named", func(t *testing.T) {
		useThemesDir(t)

		requireExportRefusal(t, execThemeExport(t, "nope"), "no theme named nope")
	})

	t.Run("an unreadable directory is could not be read", func(t *testing.T) {
		dir := unreadableThemesDir(t, "mine")
		osErr := requireDeniedRead(t, filepath.Join(dir, "nope.theme"))

		requireExportRefusal(t, execThemeExport(t, "nope"), "theme nope could not be read: "+osErr.Error())
	})

	t.Run("a dangling symlink is could not be read", func(t *testing.T) {
		dir := useThemesDir(t)
		path := filepath.Join(dir, "mine.theme")
		if err := os.Symlink(filepath.Join(dir, "no-such-target"), path); err != nil {
			t.Fatalf("symlink %s: %v", path, err)
		}
		osErr := osReadError(t, path)
		if !os.IsNotExist(osErr) {
			t.Fatalf("reading the dangling link reports %v, want a not-exist error — this case would not exercise the distinction", osErr)
		}

		requireExportRefusal(t, execThemeExport(t, "mine"), "theme mine could not be read: "+osErr.Error())
	})
}

// TestThemeExport_ArgumentIsControlStripped: it control-strips the echoed
// argument.
//
// §9.5 extends the prefs rule to the CLI argument, at the point export READS it
// — export never reads prefs (§10.5), so it is not covered by that rule and
// needs its own site. §14A echoes the argument back on stderr, and an argument
// can carry a pasted escape exactly as a prefs value can.
//
// Both halves are asserted: the whole frame, so the stripped value is the one
// the user is shown, and the absence of any control character, so a pasted
// newline cannot split one refusal into two lines with the second looking like a
// message Portal never wrote.
func TestThemeExport_ArgumentIsControlStripped(t *testing.T) {
	cases := []struct {
		name string
		arg  string
		want string
	}{
		{name: "a pasted newline", arg: "no\npe", want: "no theme named nope"},
		{name: "a pasted tab", arg: "no\tpe", want: "no theme named nope"},
		{name: "a pasted ANSI escape", arg: "\x1b[31mnope\x1b[0m", want: "no theme named nope"},
		{name: "a trailing newline on a charset failure", arg: "Nord\n", want: "theme Nord is not valid: bad name"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			useThemesDir(t)

			run := execThemeExport(t, "--", tc.arg)

			requireExportRefusal(t, run, tc.want)
			for _, r := range run.err.Error() {
				if unicode.IsControl(r) {
					t.Fatalf("refusal %q carries the control character %q — the message must stay one line of ordinary text", run.err, r)
				}
			}
		})
	}
}

// TestThemeExport_AllFailuresExitOne: it exits one for every failure class.
//
// §12.1: "Failure exit code: 1 for every failure class… the reason string on
// stderr is what discriminates, and distinguishing unknown-slug from
// invalid-file numerically buys nothing scriptable."
func TestThemeExport_AllFailuresExitOne(t *testing.T) {
	for _, tc := range themeExportFailures() {
		t.Run(tc.name, func(t *testing.T) {
			tc.seed(t)

			requireOrdinaryError(t, execThemeExport(t, tc.slug).err)
		})
	}
}

// TestThemeExport_StdoutIsEmptyOnFailure: it writes nothing to stdout on
// failure.
//
// The published workflow is `portal theme export nord > …/nord-lee.theme`, so a
// byte on the wrong stream is a byte inside the file the user just created.
// Nothing is written before validation succeeds.
//
// The last subtest keeps the rest honest: a command that wrote to stdout on NO
// path would pass every assertion above.
func TestThemeExport_StdoutIsEmptyOnFailure(t *testing.T) {
	for _, tc := range themeExportFailures() {
		t.Run(tc.name, func(t *testing.T) {
			tc.seed(t)

			run := execThemeExport(t, tc.slug)

			if run.err == nil {
				t.Fatalf("theme export %s succeeded, want a refusal", tc.slug)
			}
			if len(run.stdout) != 0 {
				t.Errorf("stdout = %q, want nothing", run.stdout)
			}
		})
	}

	t.Run("the success path does write to stdout", func(t *testing.T) {
		run := execThemeExport(t, theme.DefaultDarkSlug)

		if run.err != nil {
			t.Fatalf("theme export %s returned %v", theme.DefaultDarkSlug, run.err)
		}
		if len(run.stdout) == 0 {
			t.Fatal("the success path wrote nothing — the empty-stdout assertions above would be vacuous")
		}
	})
}

// TestThemeExport_UsesSharedByNameResolver: it shares one resolver with
// construction.
//
// §8.4's by-name ordering — charset check, embedded set, then the themes
// directory — lives in ONE place, `theme.Loader.ResolveByName`, so export and
// TUI construction cannot drift about what a slug means. Export's inline
// sequence was the second implementation, and a second implementation of an
// ordering rule is a rule that will eventually be two rules.
//
// Two halves, and both are needed. The structural one is the actual claim: cmd
// composes no path, checks no charset and reaches neither loader entry point of
// its own, so there is nothing left here to drift. The behavioural one is that
// §14A's four frames are unchanged by the re-pointing — the shared resolver
// returns §6.2 reasons, and every frame is still mapped from one.
func TestThemeExport_UsesSharedByNameResolver(t *testing.T) {
	t.Run("the four §14A frames are unchanged", func(t *testing.T) {
		t.Run("an unknown slug", func(t *testing.T) {
			useThemesDir(t)

			requireExportRefusal(t, execThemeExport(t, "no-such-theme"), "no theme named no-such-theme")
		})

		t.Run("an invalid drop-in", func(t *testing.T) {
			seedThemesDir(t, "mine", sourceMissingTokens(t, "text.primary"))

			requireExportRefusal(t, execThemeExport(t, "mine"), "theme mine is not valid: missing tokens")
		})

		t.Run("a slug failing the charset check", func(t *testing.T) {
			useThemesDir(t)

			requireExportRefusal(t, execThemeExport(t, "Nord"), "theme Nord is not valid: bad name")
		})

		t.Run("an unreadable drop-in", func(t *testing.T) {
			osErr := requireDeniedRead(t, unreadableThemeFile(t, "mine"))

			requireExportRefusal(t, execThemeExport(t, "mine"), "theme mine could not be read: "+osErr.Error())
		})

		t.Run("a valid drop-in still resolves", func(t *testing.T) {
			want := validThemeSource(t)
			seedThemesDir(t, "nord-lee", want)

			run := execThemeExport(t, "nord-lee")

			if run.err != nil {
				t.Fatalf("theme export nord-lee returned %v (stderr: %q)", run.err, run.stderr)
			}
			if !bytes.Equal(run.stdout, want) {
				t.Errorf("theme export nord-lee wrote:\n%s\nwant the file's bytes verbatim:\n%s", run.stdout, want)
			}
		})
	})

	// The structural half. Each banned symbol is one step of the ordering that
	// now belongs to internal/theme: a second copy here could accept a slug the
	// resolver refuses, look in the themes directory before the embedded set, or
	// draw the absent-versus-unreadable line differently — the three ways two
	// by-name resolvers diverge.
	t.Run("the export command re-implements no step of the ordering", func(t *testing.T) {
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, filepath.Join(".", "theme.go"), nil, 0)
		if err != nil {
			t.Fatalf("parse cmd/theme.go: %v", err)
		}

		banned := map[string]string{
			"ValidSlug":   "re-runs the charset check",
			"LoadBuiltin": "consults the embedded set",
			"LoadFile":    "reads the themes directory itself",
			"Join":        "composes a path from the slug",
			"Lstat":       "draws the absent-versus-unreadable line itself",
		}
		ast.Inspect(file, func(n ast.Node) bool {
			ident, isIdent := n.(*ast.Ident)
			if !isIdent {
				return true
			}
			if why, isBanned := banned[ident.Name]; isBanned {
				t.Errorf("cmd/theme.go:%d names %s, which %s — §8.4's ordering lives in theme.Loader.ResolveByName alone", fset.Position(ident.Pos()).Line, ident.Name, why)
			}
			return true
		})
	})

	t.Run("it still emits no theme records", func(t *testing.T) {
		unreadableThemesDir(t, "mine")
		sink := &logtest.Sink{}
		log.SetTestHandler(t, sink)

		execThemeExport(t, "mine")

		if records := sink.Records(); len(records) != 0 {
			t.Errorf("theme export emitted %d log records, want none — including the shared resolver's directory-unusable line: %+v", len(records), records)
		}
	})
}

// TestThemeExport_ReservedAndFilenameReasonsAreUnreachable: it can never report
// reserved name or a filename bad name.
//
// Two of §6.2's reasons cannot arise on export's by-name path, and the
// impossibility is pinned rather than assumed — an unreachable arm is only
// unreachable while the thing that makes it so still holds.
//
//   - `reserved name` is decided from a slug that collides with a built-in, and
//     a built-in slug resolves to the BUILT-IN first (§8.4), so the file is never
//     opened and the rung is never reached.
//   - A filename `bad name` is decided from the composed base name, which is
//     always `<valid-slug>.theme` — the argument cleared §5.2's charset rule
//     before any path was composed, and the extension is a constant.
func TestThemeExport_ReservedAndFilenameReasonsAreUnreachable(t *testing.T) {
	t.Run("a colliding drop-in never reports reserved name", func(t *testing.T) {
		slugs := theme.BuiltinSlugs()
		if len(slugs) == 0 {
			t.Fatal("BuiltinSlugs() is empty — every assertion below would be vacuous")
		}

		for _, slug := range slugs {
			t.Run(slug, func(t *testing.T) {
				shadow := scrambledThemeSource(t)
				seedThemesDir(t, slug, shadow)
				want, found := theme.BuiltinBytes(slug)
				if !found {
					t.Fatalf("BuiltinBytes(%q) reports not found", slug)
				}
				if bytes.Equal(shadow, want) {
					t.Fatal("the shadowing file equals the built-in — it would not distinguish the two")
				}

				run := execThemeExport(t, slug)

				if run.err != nil {
					t.Fatalf("theme export %s returned %v, want the built-in's bytes — a colliding file must never be reached", slug, run.err)
				}
				if !bytes.Equal(run.stdout, want) {
					t.Errorf("theme export %s wrote the drop-in's bytes, want the embedded ones", slug)
				}
			})
		}
	})

	// The structural half: every argument that survives the charset check
	// composes a base name the filename rules accept, so LoadFile's first rung
	// cannot fire from this path whatever the user typed.
	//
	// The extension is restated here because the composition itself belongs to
	// theme.Loader.ResolveByName now — cmd holds no path-building code of its
	// own, which is what TestThemeExport_UsesSharedByNameResolver enforces.
	const themeFileExtension = ".theme"

	t.Run("a composed filename always clears the filename rules", func(t *testing.T) {
		slugs := []string{"a", "0", "nord", "nord-lee", "nord-", "n0rd-2", strings.Repeat("x", 200)}

		for _, slug := range slugs {
			t.Run(slug, func(t *testing.T) {
				if !theme.ValidSlug(slug) {
					t.Fatalf("ValidSlug(%q) = false — the fixture is not a slug export would compose a path from", slug)
				}

				got, rejection := theme.SlugFromFilename(slug + themeFileExtension)

				if rejection != nil {
					t.Fatalf("SlugFromFilename(%q) = %v, want no rejection — the composed filename is always <valid-slug>.theme", slug+themeFileExtension, rejection)
				}
				if got != slug {
					t.Errorf("SlugFromFilename(%q) = %q, want %q", slug+themeFileExtension, got, slug)
				}
			})
		}
	})

	// And the observable half: a valid-but-absent slug takes the unknown-slug
	// frame, never a bad-name one.
	t.Run("a valid absent slug is unknown, never bad name", func(t *testing.T) {
		useThemesDir(t)

		requireExportRefusal(t, execThemeExport(t, "nord-"), "no theme named nord-")
	})
}
