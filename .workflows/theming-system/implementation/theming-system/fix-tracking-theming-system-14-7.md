## Attempt 1

ISSUES:
- `cmd/doctor_fix_theme_test.go:250` + `:267` — restructuring `fixThemeFixture` to stage themes via `themesDirWith` moved `prefs.json` out of the fingerprinted tree. At HEAD, `root` was the shared config root, so `treeFingerprint(t, root)` covered `prefs.json` for created/deleted/size/**mtime**/**ctime**/content AND any sibling entry appearing beside it. Now only the themes dir is fingerprinted, so the surviving prefs evidence is content-only.
  DEMONSTRATED, NOT THEORISED (reviewer ran these in a scratch copy): with `--fix` rewriting `prefs.json` with byte-identical content, HEAD's test FAILS with `prefs.json.MtimeNanos` + `CtimeNanos` deltas while the current test PASSES. With `--fix` leaking a `prefs.json.tmp-leak` sibling (the realistic `fileutil.AtomicWrite` failure mode), HEAD FAILS with `created` and the current test PASSES.
  The executor's framing is wrong twice over: `portaltest.Fingerprint` (`internal/portaltest/fingerprint.go:38`) has NO mode field at all (a chmod was only ever caught transitively via ctime), and what actually went is the "was prefs written *at all*" signal — precisely the hazard the test's own subtest comment names at `cmd/doctor_fix_theme_test.go:294-296` ("a future prefs write that happened to be idempotent would satisfy it"). Task step 7 said change no claim; this changed one.
  FIX: Keep `themesDirWith` (step 5 stands) and fingerprint the prefs directory as a second tree in `TestDoctorFix_ThemeStateUntouched`. `fixThemeFixture` already gives prefs its own dedicated `t.TempDir()`, so:
    - after `before := treeFingerprint(t, themesDir)` add `prefsDir := filepath.Dir(prefsPath)` and `prefsTreeBefore := treeFingerprint(t, prefsDir)`;
    - after the existing `assertTreeUnchanged(t, themesDir, before, "--fix changed the themes directory")` add `assertTreeUnchanged(t, prefsDir, prefsTreeBefore, "--fix wrote in the prefs directory")`.
    Keep the existing `bytes.Equal` compare — it is what renders the readable before/after body. Update `fixThemeFixture`'s doc comment (`cmd/doctor_fix_theme_test.go:209-212`) so "the prefs path to byte-compare" also states it is a dedicated directory the test fingerprints.
  ALTERNATIVE: revert step 5 for this fixture only — restore the `root`-rooted `os.Mkdir`+write loop so `themes/` and `prefs.json` share one fingerprinted root. Restores HEAD coverage exactly, but re-duplicates the write loop the task set out to remove and loses the staging-helper unification. The two-fingerprint fix is preferred.
  CONFIDENCE: high

NOTES:
- Everything else in the collapse is sound. Claim-by-claim enumeration against HEAD confirms nothing else was lost from either body; the shared assertion's failure wording is byte-identical to HEAD; `t.Helper()` puts the reported line at each call site and the FAIL header names the test.
- Non-vacuity confirmed for BOTH call sites by three mutations (break the pruned-hook breadcrumb, break the pruned-project breadcrumb, disable the stale-project prune) — each failed both tests at the shared assertion site.
- The failure-wording change `"--fix changed the theme config tree"` → `"--fix changed the themes directory"` is honest about the narrowed subject; with the fix above both subjects stay distinguishable.
- Non-blocking: `assertStalePrunesApplied(t, hooksPath, projectsPath, liveDir, goneDir, out)` is five same-typed string params and transposition-prone (a swapped `liveDir`/`goneDir` still compiles). This is the signature the task prescribed verbatim, and both call sites pass them in declaration order.
- No comment corrections — the new helper docs and the rewritten `fixThemeFixture` doc are accurate and carry no banned classes.

## Attempt 2

ISSUES:
- `cmd/doctor_fix_theme_test.go:242,252,269-271,287-288` — the prefs axis is now fully restored (both prior probes fail correctly), but the two-disjoint-trees shape leaves THREE further regressions the reviewer's first-round fix did not cover. Moving prefs.json out of the themes dir's parent turned one config-root fingerprint into two disjoint sub-trees, and `portaltest.SnapshotStateDir` EXCLUDES THE ROOT ENTRY ITSELF (`internal/portaltest/fingerprint.go:78-82`). So the themes directory's own metadata and its sibling space stopped being covered.

  Probe matrix for `TestDoctorFix_ThemeStateUntouched` (PASS = regression slips through), run against both HEAD and current:

  | probe (what `--fix` was made to do) | HEAD | current |
  |---|---|---|
  | rewrite prefs.json with identical bytes | FAIL ✓ | FAIL ✓ |
  | leak a `prefs.json.tmp-leak` sibling | FAIL ✓ | FAIL ✓ |
  | `chmod` the themes directory itself | FAIL ✓ | **PASS ✗** |
  | create-then-delete a file inside the themes dir | FAIL ✓ | **PASS ✗** |
  | write a `<themesDir>.bak` sibling of the themes dir | FAIL ✓ | **PASS ✗** |

  The chmod case is the material one — "unreadable" is one of doctor's own advisory reason classes, so a `--fix` tempted to chmod the themes directory readable is exactly the shape this test exists to forbid, and it is now unasserted.

  FIX: restore the single config-root tree while keeping ONE staging implementation. Add a parent-taking core to `cmd/doctor_theme_test.go:32` — e.g. `themesDirIn(t, parent string, files map[string][]byte) string` creating and seeding `<parent>/themes`, with `themesDirWith(t, files)` reduced to `themesDirIn(t, t.TempDir(), files)`. Then have `fixThemeFixture` take `root := t.TempDir()`, stage via `themesDirIn(t, root, …)`, put prefs.json back at `root/prefs.json`, return `(root, themesDir, prefsPath)` or just `(root, prefsPath)`, and go back to ONE `treeFingerprint(t, root)` + ONE `assertTreeUnchanged(t, root, before, "--fix changed the theme config tree")`.
  That is byte-for-byte HEAD's coverage, keeps step 5 satisfied (no repeated map-write loop), and re-converges the three sibling fixtures on one staging path rather than adding a second — compatible with Task 8's "stage themes dirs one way".
  Re-run all five probes above and confirm all five FAIL.

  ALTERNATIVE: keep the two trees and instead make cmd's `treeFingerprint` (`cmd/theme_test.go:473`) include the root entry's own Lstat metadata under a synthetic key. Smaller edit and it lifts every fingerprint call site at once, but it restores only two of the three (chmod, create-then-delete) — a sibling write beside the themes dir stays uncovered — and it changes a helper shared with two other tests. The first fix is preferred: exact equivalence, and it reduces fixture divergence rather than adding a special case.
  CONFIDENCE: medium

COMMENT_CORRECTIONS:
- None standing, BUT: `fixThemeFixture`'s doc sentence "prefs.json sits alone in a dedicated directory…" is currently TRUE and verified by probe. Applying the fix above makes it FALSE — it must be rewritten in the same edit.

NOTES:
- A convention divergence the fix also closes: the two adjacent read-only fingerprint tests (`cmd/doctor_theme_test.go:509` `TestThemeAdvisories_ScanIsReadOnly`, `cmd/theme_test.go:586`) both use the config-root idiom (one root holding `themes/` + prefs.json, fingerprinted once). `fixThemeFixture` currently uses two disjoint trees, so the three sibling fixtures no longer agree on how a "config root untouched" claim is staged.
- Everything else re-verified and sound: the shared fixture and assertions are still non-vacuous for BOTH call sites (three independent probes — reword the pruned-project breadcrumb, disable the stale-project prune, disable the stale-hook prune — each fails both tests, the breadcrumb probe failing from the single shared assertion site). Failure wording byte-identical. The down-server / mass-deletion-hazard arm is byte-identical to HEAD.
- Worth adding whichever layout lands: neither snapshot in `TestDoctorFix_ThemeStateUntouched` has a vacuity guard on `len(before)`, unlike both sibling fingerprint tests (`cmd/doctor_theme_test.go:533`, `cmd/theme_test.go:600`, which `t.Fatal` on an empty/short snapshot). HEAD had none either, so this is not drift — but the split doubled the number of snapshots that could silently be taken over the wrong path.
- Inert, no action: the ordering change inside `TestDoctorFix_ExistingRepairsUnchanged` (stale rotated log now seeded after the fixture) is safe — `seedStalePruneFixture` writes nothing into stateDir beyond `seedHealthyStateDir`.
- Out of scope, untouched by the diff: the pre-existing claim-about-tests comment at `cmd/doctor_test.go:1373`.
