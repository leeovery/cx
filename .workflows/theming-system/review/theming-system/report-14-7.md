TASK: theming-system-14-7 — Share The `doctor --fix` Stale-Prune Fixture And Assertions Between The Two Tests That Make The Claim (duplication, severity medium)

ACCEPTANCE CRITERIA:
- The stale-prune fixture and its five assertions exist once in `cmd`'s doctor test helpers; neither test restates them.
- Both tests still fail when a prune rule or a breadcrumb string changes.
- One themes-dir staging helper is used by `fixThemeFixture`; its inline write loop is gone.
- `go test ./cmd` passes.

STATUS: complete

SPEC CONTEXT:
Specification §doctor (lines 1454–1471) fixes the surface both tests defend: doctor's theme surface is **read-only with no `--fix` action** ("Doctor can prune a stale hook entry; it cannot repair someone's colours"), theme lines are advisory and do not drive the exit code, and the theme scan runs on the `--fix` path too, appearing in both the pre-repair and post-repair renders. `TestDoctorFix_ExistingRepairsUnchanged` exists to pin that adding the theme surface left the pre-existing `--fix` repairs (stale-hook prune, stale-project prune, log sweep) untouched — which is precisely why it had cloned the older test's fixture and assertions wholesale. This task is a test-hygiene remediation (phase 14 analysis cycle); it changes no production behaviour, and none was changed (commit 378adb10 touches only `cmd/doctor_test.go`, `cmd/doctor_fix_theme_test.go`, `cmd/doctor_theme_test.go` plus workflow bookkeeping).

IMPLEMENTATION:
- Status: Implemented (with one reviewer-mandated, justified deviation from Do-item 5)
- Location:
  - `cmd/doctor_test.go:850-864` — `seedStalePruneFixture(t, stateDir) (*DoctorDeps, hooksPath, projectsPath, liveDir, goneDir)`: `seedHealthyStateDir` + `seedHooksJSON("sessA:0.0")` + `seedProjectsJSON(liveDir, goneDir)` + `staleDeps(..., fakeHookLister{keys: []string{"sessB:0.0"}}, ...)`.
  - `cmd/doctor_test.go:866-894` — `assertStalePrunesApplied(t, hooksPath, projectsPath, liveDir, goneDir, out)`: all five assertion blocks, failure wording byte-identical to the pre-task originals (verified against the commit diff).
  - `cmd/doctor_test.go:1262-1288` — `TestDoctorFixPrunesStaleEntriesThenRediagnosesClean` re-pointed; retains only its own claims (two `Portal doctor:` renders, both post-repair checks clean, two summaries, trailing `7 checks passed`).
  - `cmd/doctor_fix_theme_test.go:378-410` — `TestDoctorFix_ExistingRepairsUnchanged` re-pointed; retains only its own subject (themes dir seed, stale rotated log + sweep assertion, the pre/post advisory suffixes).
  - `cmd/doctor_theme_test.go:23-42` — `themesDirWith` reduced to `themesDirIn(t, t.TempDir(), files)`; `themesDirIn` is the single staging implementation.
  - `cmd/doctor_fix_theme_test.go:161-183` — `fixThemeFixture` now stages via `themesDirIn(t, root, ...)`; its inline `os.Mkdir` + map-write loop is gone.
- Notes:
  - **Do-item 5 deviation is justified, not drift.** The task said "have `fixThemeFixture` call `themesDirWith`". Doing so literally would have moved `prefs.json` out of the fingerprinted tree and, per the fix-tracking record (`fix-tracking-theming-system-14-7.md`, attempts 1 and 2 with a five-probe matrix), silently dropped four coverage axes from `TestDoctorFix_ThemeStateUntouched` — including the material one (a `--fix` that chmods the themes directory readable, "unreadable" being one of doctor's own advisory reason classes). The landed shape (parent-taking `themesDirIn` core, `themesDirWith` a one-line delegate) restores byte-for-byte the pre-task coverage while still satisfying "one staging helper, no repeated write loop". Task 14-8 later converged `TestThemeAdvisories_ScanIsReadOnly` (`cmd/doctor_theme_test.go:385`) onto the same `themesDirIn`, so the fixture family now agrees.
  - `themesDirWith`'s **returned path shape changed** (`<tmp>` → `<tmp>/themes`) across ~40 call sites. Checked and inert: `deps.ThemesDir` is only ever fed to `loader.OpenEnumeration` (`cmd/doctor_theme.go:53`), the path is never rendered into doctor output, and no cmd test derives a parent/sibling from a `themesDirWith` result (`cmd/config_themes_test.go:101` is the only `filepath.Dir` on a themes path and it resolves `themesDirPath()`, not the fixture).
  - Do-item 6 honoured: the down-server / mass-deletion-hazard arm (`cmd/doctor_fix_theme_test.go:412-433`) is untouched by this commit and keeps its own `downServerDeferFixture` / `assertDownServerDeferral` pair (later shared by task 16-3).
  - Do-item 7 honoured: no production file in the commit; no failure wording that carries information was altered.
  - Later commits (`a4bc7bd5`, `915e7fcb` comment sweeps; `dab38cab` 14-8) revised the doc comments this task wrote. Current state is consistent — judged against the amended intent, not the task's original comment text.

TESTS:
- Status: Adequate (this task's deliverable *is* test code; the claim it must preserve is non-vacuity)
- Coverage:
  - Both consumers reach the shared assertion: `cmd/doctor_test.go:1271` and `cmd/doctor_fix_theme_test.go:396`, each after a `t.Fatalf` guard on the Execute error, each passing the fixture's returns in declaration order.
  - Non-vacuity holds by construction. A changed breadcrumb string fails both tests at the one `strings.Contains` site (`cmd/doctor_test.go:888,891`). A disabled prune fails both tests too — via the post-repair exit code (the re-diagnosis reports the surviving stale entry, so `runDoctorFixCmd` returns non-nil and the `t.Fatalf` fires) and, for the hook prune, via the on-disk read-back at `cmd/doctor_test.go:873`. The fix-tracking record documents the reviewer running exactly these probes (reworded pruned-project breadcrumb, disabled stale-project prune, disabled stale-hook prune) with both tests failing on each.
  - The fixture's discriminating detail survives the move: the non-empty `fakeHookLister{keys: ["sessB:0.0"]}` both makes `sessA:0.0` stale and keeps `runHookStaleCleanup`'s mass-deletion hazard guard (`cmd/doctor.go:290-294`) from deferring — the inline comment at `cmd/doctor_test.go:859-860` states this and is accurate against `doctor.go`.
  - `assertStalePrunesApplied`'s `proj1` expectation is correct: `seedProjectsJSON` names records `"proj"+i` and `goneDir` is index 1 in the shared fixture (the down-server fixture seeds only `goneDir`, hence its separate `proj0` assertion at `cmd/doctor_test.go:1334` — not residual duplication, a different claim).
  - `t.Helper()` on both helpers keeps the reported line at the call site, so a failure still names which test lost the claim.
  - Compile surface checked: `maps`/`slices` imports added at `cmd/doctor_fix_theme_test.go:7,10` are used at line 189; `staleDeps`, `seedHealthyStateDir`, `seedHooksJSON`, `seedProjectsJSON` all retain other callers, so nothing is orphaned.
- Notes:
  - One assertion beyond the Do list landed: the snapshot vacuity floor at `cmd/doctor_fix_theme_test.go:188-190`. It is not scope creep — the reviewer's attempt-2 note explicitly asked for it ("neither snapshot has a vacuity guard on `len(before)`, unlike both sibling fingerprint tests") — and it strengthens rather than changes a claim. Its threshold is loose (see non-blocking note).
  - No over-testing introduced: the collapse strictly removes assertions, and each test's residual body maps 1:1 to a distinct claim.
  - I did not execute the suite (assessment is by reading, per my constraints).

CODE QUALITY:
- Project conventions: Followed. Helpers live in `cmd`'s existing doctor test-helper cluster beside `seedHooksJSON`/`seedProjectsJSON`/`staleDeps`; `t.Helper()` on both; no `t.Parallel()`; no new mocking; the fixture keeps taking `stateDir` from the call site so the theme test can seed its stale rotated log into the same dir.
- SOLID principles: Good — `seedStalePruneFixture` (arrange) and `assertStalePrunesApplied` (assert) are separated rather than fused into one do-everything helper, so each test keeps ownership of the act step and of its own extra claims.
- Complexity: Low. Both helpers are straight-line; `themesDirIn` is six lines with the delegate collapsing `themesDirWith` to one.
- Modern idioms: Yes — `slices.Sorted(maps.Keys(...))` in the new vacuity guard's failure message.
- Readability: Good. The one surviving inline comment explains the non-obvious dual role of the live-pane set; the rest is self-documenting.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/doctor_test.go:850,866 — `seedStalePruneFixture` returns four same-typed strings and `assertStalePrunesApplied` takes five, so a transposed `liveDir`/`goneDir` still compiles and inverts the "live project survives" claim into a false pass. Replace both with a small `stalePruneFixture` struct (`deps, hooksPath, projectsPath, liveDir, goneDir`) returned by the seeder and taken by the asserter, updating the two call sites (`cmd/doctor_test.go:1263,1271`, `cmd/doctor_fix_theme_test.go:381,396`). The five-string signature is what the task prescribed verbatim, so this is a follow-on improvement, not a deviation.
- [quickfix] cmd/doctor_fix_theme_test.go:188 — the new vacuity floor `len(before) < 3` is far below what `fixThemeFixture` actually seeds (the `themes` dir entry + 6 files + `prefs.json` = 8 entries, since `portaltest.SnapshotStateDir` records directories and excludes only the root itself), so a snapshot taken over a mostly-wrong root would still clear it. Tighten it to the seeded count — either lift the file map into a named var and assert `len(before) < len(files)+2`, matching the sibling guard's idiom at `cmd/doctor_theme_test.go:393`, or have `fixThemeFixture` return the seeded entry count.
