// Tests in this file seed PORTAL_* env vars and mutate package-level Cobra/DI
// state (doctorDeps, bootstrapDeps, rootCmd), so they MUST NOT use t.Parallel.
//
// Concern-split from the four doctor suites that precede it: doctor_theme_test.go
// and doctor_persisted_theme_test.go own the two advisory PRODUCERS,
// doctor_theme_union_test.go owns the assembly between them, and
// doctor_advisory_test.go owns the line class itself. This file owns the ONE
// path none of them cross — `portal doctor --fix`, which renders TWO reports.
//
// What is pinned here is that the theme surface appears in BOTH of them and
// repairs NOTHING in either: doctor's "the theme scan runs on the `--fix` path
// too" together with its "read-only, with no `--fix` action". The two halves are
// one concern because they are the same decision seen from either side — doctor
// has a repair surface, so the reason the advisories may ride it is exactly the
// reason nothing may be repaired from them.
package cmd

import (
	"bytes"
	"encoding/json"
	"go/ast"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// requireTwoReports splits a `--fix` run's stdout into its two rendered reports,
// each running from a "Portal doctor:" header through the summary line that
// closes it. Whatever sits BETWEEN them — the repair breadcrumbs — belongs to
// neither report and is deliberately excluded, so an assertion about a report's
// trailing advisory block cannot be satisfied by a line the repairs printed.
func requireTwoReports(t *testing.T, out string) (pre, post string) {
	t.Helper()

	var reports []string
	var current []string
	inReport := false
	for line := range strings.SplitSeq(strings.TrimSuffix(out, "\n"), "\n") {
		if line == "Portal doctor:" {
			inReport = true
			current = nil
			continue
		}
		if !inReport {
			continue
		}
		current = append(current, line)
		if strings.Contains(line, " checks passed") {
			reports = append(reports, strings.Join(current, "\n"))
			inReport = false
		}
	}

	if len(reports) != 2 {
		t.Fatalf("stdout holds %d complete reports, want 2 (the initial diagnosis and the post-repair re-diagnosis):\n%s", len(reports), out)
	}
	return reports[0], reports[1]
}

// requireBothReportsEndWith fails unless each of a `--fix` run's two reports
// closes with the given block. Both are asserted with the SAME expectation
// wherever the checks half is unmoved by the repairs, because the claim under
// test is that the second render is as informative as the first.
func requireBothReportsEndWith(t *testing.T, out, want string) {
	t.Helper()

	pre, post := requireTwoReports(t, out)
	for i, report := range []string{pre, post} {
		label := "initial diagnosis"
		if i == 1 {
			label = "post-repair re-diagnosis"
		}
		if !strings.HasSuffix(report, want) {
			t.Errorf("the %s does not close with\n%s\ngot\nPortal doctor:\n%s", label, want, report)
		}
	}
}

// TestDoctorFix_AdvisoriesInBothPasses: it renders advisories in both --fix
// passes.
//
// Doctor's theme line: the theme scan runs on the `--fix` path too. Suppressing it there would
// make `--fix` a LESS informative diagnosis than the plain run — and the user who
// reaches for the repairing form is the one most likely to have something wrong,
// so they would be the one who could not see it.
func TestDoctorFix_AdvisoriesInBothPasses(t *testing.T) {
	deps := healthyDoctorDeps(t)
	deps.ThemesDir = themesDirWith(t, map[string][]byte{
		"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		"c-valid.theme":   validThemeSource(t),
	})

	outBuf, _, err := runDoctorFixCmd(t, deps)
	if err != nil {
		t.Fatalf("Execute err = %v; a broken drop-in must never drive the exit code", err)
	}

	requireBothReportsEndWith(t, outBuf.String(), ""+
		"  ⚠ theme a-missing: missing tokens — missing text.primary\n"+
		"  ⚠ theme b-colour: bad colour — canvas = blue\n"+
		"  7 checks passed · 2 advisories")
}

// TestDoctorFix_SuffixInBothSummaries: it suffixes both summaries with the
// advisory count.
//
// The block and the ` · <M>` suffix are one surface and must not half-arrive:
// a report carrying the lines but closing with a bare "<N> checks passed" would
// tell the user the run was clean two lines under the evidence that it was not.
//
// The subtest is the case the two summaries can be told apart in — a health
// check that `--fix` genuinely repairs, so the checks half MOVES between the
// passes while the advisory half does not. It is also the acceptance of doctor's
// exit-code rule on this path: the repaired run exits 0 with advisories standing
// in both reports.
func TestDoctorFix_SuffixInBothSummaries(t *testing.T) {
	deps := healthyDoctorDeps(t)
	deps.ThemesDir = themesDirWith(t, map[string][]byte{
		"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
	})

	outBuf, _, err := runDoctorFixCmd(t, deps)
	if err != nil {
		t.Fatalf("Execute err = %v; a broken drop-in must never drive the exit code", err)
	}

	out := outBuf.String()
	if n := strings.Count(out, "\n  7 checks passed · 2 advisories\n"); n != 2 {
		t.Errorf("suffixed summary count = %d; want 2 (one per report render):\n%s", n, out)
	}

	t.Run("the suffix rides both summary forms", func(t *testing.T) {
		dir := t.TempDir()
		seedHealthyStateDir(t, dir)
		// A persisted key absent from a NON-empty live set is genuinely stale (the
		// hazard guard does not defer), so the initial pass fails the stale-hooks
		// check and --fix prunes it before the second — which is what makes the two
		// summaries carry DIFFERENT checks halves and the same suffix.
		hookStore, _ := seedHooksJSON(t, "sessA:0.0")
		projectStore, _ := seedProjectsJSON(t, t.TempDir())
		deps := staleDeps(dir, fakeHookLister{keys: []string{"sessB:0.0"}}, hookStore, projectStore)
		deps.ThemesDir = themesDirWith(t, map[string][]byte{
			"a-missing.theme": sourceMissingTokens(t, "text.primary"),
			"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		})

		outBuf, _, err := runDoctorFixCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; want nil — the repaired catalog is healthy and an advisory never moves the exit code", err)
		}

		const block = "" +
			"  ⚠ theme a-missing: missing tokens — missing text.primary\n" +
			"  ⚠ theme b-colour: bad colour — canvas = blue\n"
		pre, post := requireTwoReports(t, outBuf.String())
		if want := block + "  6 of 7 checks passed · 2 advisories"; !strings.HasSuffix(pre, want) {
			t.Errorf("the initial diagnosis does not close with\n%s\ngot\nPortal doctor:\n%s", want, pre)
		}
		if want := block + "  7 checks passed · 2 advisories"; !strings.HasSuffix(post, want) {
			t.Errorf("the post-repair re-diagnosis does not close with\n%s\ngot\nPortal doctor:\n%s", want, post)
		}
	})
}

// TestDoctorFix_AdvisoryOnlyExitsZero: it exits zero when the only findings are
// advisories.
//
// The two halves of doctor's theme line on the one path that could break either: an advisory
// cannot drive the exit code, and it cannot summon a repair. A run whose sole
// findings are theme advisories must therefore exit 0 AND print no "Pruned …"
// line — because a `Pruned ` here could only mean doctor had decided to delete
// something of the user's.
func TestDoctorFix_AdvisoryOnlyExitsZero(t *testing.T) {
	requireDropInSlug(t, "gone")
	// A persisted advisory rides alongside the file lines so BOTH producers are
	// represented: neither may earn a repair, and neither may move the exit code.
	setPrefsFile(t, `{"theme":"gone"}`)

	deps := healthyDoctorDeps(t)
	deps.ThemesDir = themesDirWith(t, map[string][]byte{
		"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
	})

	outBuf, _, err := runDoctorFixCmd(t, deps)
	if err != nil {
		t.Fatalf("Execute err = %v; want nil — a run whose only findings are advisories is a healthy run", err)
	}

	out := outBuf.String()
	if strings.Contains(out, "Pruned ") {
		t.Errorf("--fix printed a prune breadcrumb over a theme-only finding; doctor can prune a stale hook entry, it cannot repair someone's colours:\n%s", out)
	}
	// Vacuity guard: the run really did have findings to report, so the clean exit
	// above is the advisory class being outside the exit code rather than a run
	// that found nothing.
	requireBothReportsEndWith(t, out, ""+
		"  ⚠ theme a-missing: missing tokens — missing text.primary\n"+
		"  ⚠ theme b-colour: bad colour — canvas = blue\n"+
		"  ⚠ theme gone does not resolve: not found\n"+
		"  7 checks passed · 3 advisories")
}

// fixThemeFixture seeds a themes directory holding one file per reason class
// beside a valid drop-in and a non-theme neighbour, plus a prefs.json, and
// points the production resolutions at both. It returns the config root to
// fingerprint and the prefs path to byte-compare. Both files live under that one
// root, so a snapshot of it covers the themes directory's own metadata, the
// space beside it, and any write to prefs.json — including one whose bytes come
// back identical.
//
// Every class is present because they are the ones a repair surface would be
// tempted by in different ways — a junk file to delete, a name to rewrite, a
// built-in collision to resolve — and a fixture holding only one of them would
// leave the others' absence of a repair unasserted.
//
// The prefs body carries an `appearance` key on purpose: it is the value the write-path
// ownership rule's one-shot translation acts on, over a file with no theme key and no marker —
// so it is a file the MIGRATING loader would genuinely rewrite. That is what makes
// "prefs.json is byte-identical" evidence that doctor read it through the
// NON-migrating variant, rather than evidence that this particular file had
// nothing to translate.
//
// syncPersistTranslation is what gives that evidence teeth, and it must not be
// dropped as redundant just because the passing run never reaches it: TestMain
// neutralises persistTranslation package-wide with a no-op, and the migrating
// loader performs its ENTIRE write inside that seam. Without the substitution a
// doctor rewired to loadPrefsStore would leave this fixture byte-identical too,
// and the assertions below would prove only that runDoctorFix writes no prefs of
// its own. Substituting the production body (minus the goroutine) puts the write
// back on the path, so the byte-compare fails the moment doctor's prefs read
// migrates.
func fixThemeFixture(t *testing.T) (root, prefsPath string) {
	t.Helper()

	syncPersistTranslation(t)
	requireBuiltinSlug(t, "nord")
	root = t.TempDir()
	themesDir := themesDirIn(t, root, map[string][]byte{
		"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		"c-valid.theme":   validThemeSource(t),
		"D_bad.theme":     validThemeSource(t),
		"nord.theme":      validThemeSource(t),
		"notes.txt":       []byte("not a theme file\n"),
	})
	t.Setenv("PORTAL_THEMES_DIR", themesDir)

	prefsPath = filepath.Join(root, "prefs.json")
	if err := os.WriteFile(prefsPath, []byte(`{"session_list_mode":"by-tag","appearance":"light"}`), 0o600); err != nil {
		t.Fatalf("seed prefs.json: %v", err)
	}
	t.Setenv("PORTAL_PREFS_FILE", prefsPath)
	return root, prefsPath
}

// TestDoctorFix_ThemeStateUntouched: it repairs no theme state.
//
// Doctor's theme line: "doctor can prune a stale hook entry; it cannot repair someone's
// colours". `--fix` is the ONE doctor path that writes, so it is the one where
// that claim has to be proven rather than assumed — and it is proven over the
// whole surface at once: every `.theme` file's bytes and mode, the directory's
// entry set (so nothing was deleted, renamed or seeded), and prefs.json.
func TestDoctorFix_ThemeStateUntouched(t *testing.T) {
	root, prefsPath := fixThemeFixture(t)
	before := treeFingerprint(t, root)
	// The fixture seeds prefs.json, the themes directory and a file per reason
	// class, so a snapshot this small means most of that never landed in it and
	// the comparison below would be vacuous.
	if len(before) < 3 {
		t.Fatalf("snapshot of the config root holds %d entries: %v", len(before), slices.Sorted(maps.Keys(before)))
	}
	prefsBefore, err := os.ReadFile(prefsPath)
	if err != nil {
		t.Fatalf("read prefs.json: %v", err)
	}

	outBuf, _, err := runDoctorFixCmd(t, healthyDoctorDeps(t))
	if err != nil {
		t.Fatalf("Execute err = %v; broken drop-ins must never drive the exit code", err)
	}
	// Vacuity guard: the run really did find theme problems, so "untouched" below
	// is a statement about a --fix that had something to be tempted by.
	if n := strings.Count(outBuf.String(), "⚠"); n != 8 {
		t.Fatalf("report carries %d advisory lines over the two renders, want 8 (4 per pass) — the untouched assertions would be about a scan that found nothing:\n%s", n, outBuf.String())
	}

	assertTreeUnchanged(t, root, before, "--fix changed the theme config tree")

	prefsAfter, err := os.ReadFile(prefsPath)
	if err != nil {
		t.Fatalf("re-read prefs.json: %v", err)
	}
	if !bytes.Equal(prefsBefore, prefsAfter) {
		t.Errorf("--fix rewrote prefs.json\nbefore: %s\nafter:  %s", prefsBefore, prefsAfter)
	}

	t.Run("it writes no theme_migrated marker", func(t *testing.T) {
		// The byte-compare above already covers this, but only incidentally — a
		// future prefs write that happened to be idempotent would satisfy it while
		// still recording the one-shot marker, which is the state that permanently
		// disarms the write-path ownership rule's translation for a user who never launched a TUI.
		if translateAppearance("light") == "" {
			t.Fatal("the fixture's appearance value translates to nothing, so a --fix that ran the migration would write no marker anyway and this assertion would be vacuous")
		}
		var decoded map[string]any
		if err := json.Unmarshal(prefsAfter, &decoded); err != nil {
			t.Fatalf("decode prefs.json: %v", err)
		}
		if got, ok := decoded["theme_migrated"]; ok {
			t.Errorf("prefs.json carries theme_migrated = %#v after --fix; doctor reads prefs through the NON-migrating variant on every path", got)
		}
	})

	t.Run("it creates no themes directory when there is none", func(t *testing.T) {
		// The directory-resolution rule: Portal never creates or seeds the themes directory — and
		// `--fix`, the one doctor path that writes, is the one that would be tempted to.
		absent := filepath.Join(t.TempDir(), "themes")
		t.Setenv("PORTAL_THEMES_DIR", absent)

		if _, _, err := runDoctorFixCmd(t, healthyDoctorDeps(t)); err != nil {
			t.Fatalf("Execute err = %v; an absent themes directory is silent, not unhealthy", err)
		}
		if _, err := os.Stat(absent); !os.IsNotExist(err) {
			t.Errorf("os.Stat(%s) = %v; --fix must never create the themes directory", absent, err)
		}
	})
}

// TestDoctorFix_ScanReRunForSecondPass: it re-runs the scan for the second
// render.
//
// The second report is a genuine RE-DIAGNOSIS of the same read-only condition,
// not a replay of the first — the same thing the second runDoctorDiagnosis call
// says about the check catalog. Reusing the first pass's slice would pair a stale
// advisory block with freshly-read check lines, so one report would describe two
// different moments.
//
// Both halves are needed and neither is sufficient. The behavioural half proves
// the function answers from CURRENT disk rather than from a cached first answer;
// the source guard proves the `--fix` branch actually calls it a second time,
// which no output assertion can distinguish from a reused variable, since nothing
// doctor does between the passes changes what the scan would say.
func TestDoctorFix_ScanReRunForSecondPass(t *testing.T) {
	t.Run("a second call reflects the directory's current state", func(t *testing.T) {
		dir := themesDirWith(t, map[string][]byte{
			"a-missing.theme": sourceMissingTokens(t, "text.primary"),
			"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		})
		deps := &DoctorDeps{ThemesDir: dir}

		requireAdvisoryLines(t, themeAdvisoryUnion(deps),
			"⚠ theme a-missing: missing tokens — missing text.primary",
			"⚠ theme b-colour: bad colour — canvas = blue",
		)

		// The user fixing a file mid-run is the only way the two passes can
		// legitimately differ, and it is what a re-run must be able to see.
		if err := os.Remove(filepath.Join(dir, "b-colour.theme")); err != nil {
			t.Fatalf("remove the broken drop-in: %v", err)
		}

		requireAdvisoryLines(t, themeAdvisoryUnion(deps),
			"⚠ theme a-missing: missing tokens — missing text.primary",
		)
	})

	t.Run("both renders are handed a freshly collected block", func(t *testing.T) {
		source := parsePackageFilesByName(t)["doctor.go"]
		if source == nil {
			t.Fatal("the cmd package declares no doctor.go — the guard has nothing to scan")
		}

		var renders []*ast.CallExpr
		ast.Inspect(source, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if ident, ok := call.Fun.(*ast.Ident); ok && ident.Name == "renderDoctorReport" {
				renders = append(renders, call)
			}
			return true
		})

		// Two call sites, in source order: the initial diagnosis and the post-repair
		// re-diagnosis. A third would mean an unexamined render.
		if len(renders) != 2 {
			t.Fatalf("doctor.go holds %d renderDoctorReport calls, want 2 (the plain/initial render and the post-repair one)", len(renders))
		}
		for i, call := range renders {
			if len(call.Args) != 3 {
				t.Fatalf("renderDoctorReport call %d takes %d arguments, want 3 — the guard is reading the wrong slot", i, len(call.Args))
			}
			arg, ok := call.Args[2].(*ast.CallExpr)
			if !ok {
				t.Errorf("renderDoctorReport call %d is handed %T as its advisories; want a collectThemeAdvisories(deps) call — a reused variable would make that render a replay", i, call.Args[2])
				continue
			}
			ident, ok := arg.Fun.(*ast.Ident)
			if !ok || ident.Name != "collectThemeAdvisories" {
				t.Errorf("renderDoctorReport call %d is handed a call to something other than collectThemeAdvisories", i)
			}
		}
	})
}

// TestDoctorFix_EmitsNoThemeRecords: it emits zero theme records across both
// passes.
//
// The `theme` log component: the component records where a theme is USED, never where one is
// DIAGNOSED — and `--fix` is the strongest case for it, being the only doctor
// path that diagnoses the same reject set TWICE. A loader logging here would
// write the largest possible WARN volume, doubled, on the surface that needs it
// least, and would put a write on the path whose whole claim is that it repairs
// no theme state.
func TestDoctorFix_EmitsNoThemeRecords(t *testing.T) {
	requireDropInSlug(t, "gone")
	requireBuiltinSlug(t, "nord")
	// Both producers reporting, over every reason class either can reach, so the
	// zero-record claim is made against the fullest emission the run can provoke.
	setPrefsFile(t, `{"theme":"gone"}`)
	deps := healthyDoctorDeps(t)
	deps.ThemesDir = themesDirWith(t, map[string][]byte{
		"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		"b-colour.theme":  sourceBadColours(t, themeOverride{"canvas", "blue"}),
		"c-syntax.theme":  sourceDuplicateKeyAt(t, 12, "text.primary"),
		"D_bad.theme":     validThemeSource(t),
		"nord.theme":      validThemeSource(t),
	})

	assertNoThemeRecords(t, func() {
		outBuf, _, err := runDoctorFixCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; broken drop-ins must never drive the exit code", err)
		}
		// Vacuity guard: six lines per pass over twelve, so the silence is about
		// a run that diagnosed a full reject set twice.
		if n := strings.Count(outBuf.String(), "⚠"); n != 12 {
			t.Fatalf("report carries %d advisory lines over the two renders, want 12 (6 per pass) — the zero-record assertion would be about the wrong run:\n%s", n, outBuf.String())
		}
	})
}

// TestDoctorFix_RemainsBootstrapExempt: it stays bootstrap-exempt.
//
// Doctor's exemption is what makes it a diagnosis rather than a repair: bootstrap
// would re-register hooks and respawn the daemon one step BEFORE doctor read
// their health, so the read-only check would heal its own subject. Adding the
// theme surface must not have quietly conscripted a server: it reads a directory
// and a config file, so it needs no tmux at all, and `--fix` — which runs the
// whole diagnosis twice — is where a regression would show up twice over.
func TestDoctorFix_RemainsBootstrapExempt(t *testing.T) {
	if !skipTmuxCheck["doctor"] {
		t.Error("skipTmuxCheck[\"doctor\"] = false; want true (Bootstrap Exemption)")
	}

	t.Run("a --fix run with a full theme surface runs no bootstrap", func(t *testing.T) {
		resetBootstrapOnce(t)
		runner := &recordingRunner{started: true}
		bootstrapDeps = &BootstrapDeps{Orchestrator: runner}
		t.Cleanup(func() { bootstrapDeps = nil })

		requireDropInSlug(t, "gone")
		setPrefsFile(t, `{"theme":"gone"}`)
		deps := healthyDoctorDeps(t)
		deps.ThemesDir = themesDirWith(t, map[string][]byte{
			"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		})

		outBuf, _, err := runDoctorFixCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; broken drop-ins must never drive the exit code", err)
		}
		if runner.calls != 0 {
			t.Errorf("the bootstrap orchestrator ran %d times on `doctor --fix`; doctor starts no server, ensures no saver and runs no restore", runner.calls)
		}
		// Vacuity guard: the theme surface really did run on both passes, so the
		// zero above is an exempt command that scanned rather than one that skipped.
		requireBothReportsEndWith(t, outBuf.String(), ""+
			"  ⚠ theme a-missing: missing tokens — missing text.primary\n"+
			"  ⚠ theme gone does not resolve: not found\n"+
			"  7 checks passed · 2 advisories")
	})

	t.Run("the orchestrator seam runs for a non-exempt command", func(t *testing.T) {
		// Without this the zero above could equally mean the seam is dead. A probe
		// command carries no exemption, so it must reach the orchestrator.
		resetBootstrapOnce(t)
		runner := &recordingRunner{started: true}
		bootstrapDeps = &BootstrapDeps{Orchestrator: runner}
		t.Cleanup(func() { bootstrapDeps = nil })

		probe := &cobra.Command{Use: "doctorfixprobe", RunE: func(*cobra.Command, []string) error { return nil }}
		rootCmd.AddCommand(probe)
		t.Cleanup(func() { rootCmd.RemoveCommand(probe) })

		resetRootCmd()
		rootCmd.SetArgs([]string{"doctorfixprobe"})
		if err := rootCmd.Execute(); err != nil {
			t.Fatalf("probe Execute err = %v", err)
		}
		if runner.calls != 1 {
			t.Fatalf("the bootstrap orchestrator ran %d times for a non-exempt command, want 1 — the exemption assertion above would be vacuous", runner.calls)
		}
	})
}

// TestDoctorFix_ExistingRepairsUnchanged: it leaves the existing repairs
// unchanged.
//
// The advisories are additive to `--fix`, not a change to it: the stale-hook
// prune, the stale-project prune and the log-retention sweep behave exactly as
// they did, and the exit stays driven solely by the post-repair health checks.
// The down-server arm is the one worth stating twice — the mass-deletion hazard
// guard is what stands between a rebooted server and a user's non-reconstructable
// on-resume commands, and it must still defer with the theme surface running
// alongside it.
func TestDoctorFix_ExistingRepairsUnchanged(t *testing.T) {
	t.Run("the two prunes and the log sweep still run", func(t *testing.T) {
		dir := t.TempDir()
		deps, hooksPath, projectsPath, liveDir, goneDir := seedStalePruneFixture(t, dir)
		deps.ThemesDir = themesDirWith(t, map[string][]byte{
			"a-missing.theme": sourceMissingTokens(t, "text.primary"),
		})
		staleLog := filepath.Join(dir, "portal.log.2000-01-01")
		if err := os.WriteFile(staleLog, []byte("old\n"), 0o600); err != nil {
			t.Fatalf("seed stale rotated log: %v", err)
		}

		outBuf, _, err := runDoctorFixCmd(t, deps)
		if err != nil {
			t.Fatalf("Execute err = %v; want nil (healthy post-repair)", err)
		}

		out := outBuf.String()
		assertStalePrunesApplied(t, hooksPath, projectsPath, liveDir, goneDir, out)

		if _, statErr := os.Stat(staleLog); !os.IsNotExist(statErr) {
			t.Errorf("stale rotated log not swept (stat err = %v); the log sweep did not run", statErr)
		}

		const advisory = "  ⚠ theme a-missing: missing tokens — missing text.primary\n"
		pre, post := requireTwoReports(t, out)
		if want := advisory + "  5 of 7 checks passed · 1 advisory"; !strings.HasSuffix(pre, want) {
			t.Errorf("the initial diagnosis does not close with\n%s\ngot\nPortal doctor:\n%s", want, pre)
		}
		if want := advisory + "  7 checks passed · 1 advisory"; !strings.HasSuffix(post, want) {
			t.Errorf("the post-repair re-diagnosis does not close with\n%s\ngot\nPortal doctor:\n%s", want, post)
		}
	})

	t.Run("the hazard guard still defers on a down server", func(t *testing.T) {
		dir := t.TempDir()
		hookStore, hooksPath := seedHooksJSON(t, "sessA:0.0")
		goneDir := filepath.Join(t.TempDir(), "gone")
		projectStore, projectsPath := seedProjectsJSON(t, goneDir)
		hooksBefore, err := os.ReadFile(hooksPath)
		if err != nil {
			t.Fatalf("read hooks.json: %v", err)
		}

		deps := &DoctorDeps{
			StateDir:      dir,
			ServerRunning: func() bool { return false },
			// A down server yields an empty live-pane enumeration.
			HookLister:   fakeHookLister{keys: []string{}},
			HookStore:    hookStore,
			ProjectStore: projectStore,
			Detector:     fakeTerminalDetector{},
			Resolve:      doctorUnsupportedResolve,
			ThemesDir: themesDirWith(t, map[string][]byte{
				"a-missing.theme": sourceMissingTokens(t, "text.primary"),
			}),
		}
		outBuf, _, execErr := runDoctorFixCmd(t, deps)
		if execErr != ErrDoctorUnhealthy {
			t.Fatalf("Execute err = %v; want ErrDoctorUnhealthy (server still down post-repair)", execErr)
		}

		hooksAfter, err := os.ReadFile(hooksPath)
		if err != nil {
			t.Fatalf("re-read hooks.json: %v", err)
		}
		if !bytes.Equal(hooksBefore, hooksAfter) {
			t.Errorf("hooks.json pruned on a down server (user commands must survive)\nbefore: %s\nafter:  %s", hooksBefore, hooksAfter)
		}
		projectsAfter, err := os.ReadFile(projectsPath)
		if err != nil {
			t.Fatalf("re-read projects.json: %v", err)
		}
		if strings.Contains(string(projectsAfter), goneDir) {
			t.Errorf("the filesystem-only stale-project prune did not run on a down server:\n%s", projectsAfter)
		}

		const advisory = "  ⚠ theme a-missing: missing tokens — missing text.primary\n"
		pre, post := requireTwoReports(t, outBuf.String())
		if want := advisory + "  2 of 6 checks passed · 1 advisory"; !strings.HasSuffix(pre, want) {
			t.Errorf("the initial diagnosis does not close with\n%s\ngot\nPortal doctor:\n%s", want, pre)
		}
		if want := advisory + "  3 of 6 checks passed · 1 advisory"; !strings.HasSuffix(post, want) {
			t.Errorf("the post-repair re-diagnosis does not close with\n%s\ngot\nPortal doctor:\n%s", want, post)
		}
	})
}
