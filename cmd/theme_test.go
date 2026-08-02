package cmd

import (
	"bytes"
	"crypto/sha256"
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

// seedThemesDir writes one drop-in into a fresh themes directory, points
// PORTAL_THEMES_DIR at it, and returns the directory.
func seedThemesDir(t *testing.T, slug string, source []byte) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, slug+".theme"), source, 0o644); err != nil {
		t.Fatalf("seed %s.theme: %v", slug, err)
	}
	t.Setenv("PORTAL_THEMES_DIR", dir)
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

	var pairs []string
	for line := range strings.SplitSeq(string(validThemeSource(t)), "\n") {
		text := strings.TrimSpace(line)
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		pairs = append(pairs, text)
	}
	if len(pairs) < 2 {
		t.Fatalf("built-in yielded %d key lines — a scrambled-order fixture needs at least two", len(pairs))
	}
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
	seedUnreadableThemesDir := func(t *testing.T) {
		t.Helper()
		dir := seedThemesDir(t, "nord-lee", validThemeSource(t))
		if err := os.Chmod(dir, 0o000); err != nil {
			t.Fatalf("chmod 0000 %s: %v", dir, err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	}

	t.Run("a built-in resolves through an unreadable themes directory", func(t *testing.T) {
		seedUnreadableThemesDir(t)
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
		seedUnreadableThemesDir(t)

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
	declaresNothing := func(*testing.T) []byte { return []byte("# a file that declares nothing\n") }

	// source is nil where the case seeds no drop-in at all, which is the state
	// the built-in, unknown-slug and bad-name cases each need.
	cases := []struct {
		name   string
		slug   string
		source func(*testing.T) []byte
	}{
		{name: "a built-in", slug: theme.DefaultDarkSlug},
		{name: "a valid drop-in", slug: "nord-lee", source: validThemeSource},
		{name: "an invalid drop-in", slug: "nord-lee", source: declaresNothing},
		{name: "an unknown slug", slug: "no-such-theme"},
		{name: "a slug failing the charset check", slug: "Nord"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.source != nil {
				seedThemesDir(t, tc.slug, tc.source(t))
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
