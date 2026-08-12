TASK: theming-system-7-7 — The `--fix` Path Carries The Advisories And Repairs Nothing

ACCEPTANCE CRITERIA:
- A `--fix` run over a directory holding two broken themes prints both advisories in both reports, with ` · 2 advisories` on both summaries.
- A `--fix` run whose only findings are theme advisories exits 0 and prints no `Pruned ` line.
- A `--fix` run with a failing health check that `--fix` repairs still exits 0, and both reports carry their own advisory block and suffix.
- `runDoctorFix` contains no theme step: after the run every `.theme` file, the directory's entry set and its modes, and `prefs.json` are byte-identical.
- `theme_migrated` is not written by any `--fix` run.
- The whole `--fix` Execute produces zero `theme` records through a `logtest.Sink`.
- `doctor` is still in `skipTmuxCheck`; the theme surface starts no tmux server and touches no saver or restore path.
- The second render's advisories are recomputed — a themes-directory change between the two calls is reflected in the second result.
- The existing stale-hook / stale-project prunes and the log sweep are behaviourally unchanged, hazard guard included.

STATUS: complete

SPEC CONTEXT:
§12.2 (specification.md:1454, :1456, :1471) governs this task directly: doctor's theme lines are "Read-only, with no `--fix` action. Doctor can prune a stale hook entry; it cannot repair someone's colours"; the theme class is advisory and does not drive the exit code; and "**The theme scan runs on the `--fix` path too**, and its advisories and the `· <M> advisories` suffix appear there. `--fix` re-diagnoses after repairs and the theme lines are read-only in both passes". §12.2 also pins the non-migrating prefs read so a doctor run never triggers the one-shot `appearance` translation, and §5.5 pins that Portal never creates or seeds the themes directory — the constraint that bites hardest on `--fix`, the one doctor path that writes.

IMPLEMENTATION:
- Status: Implemented (comments later condensed by the phase 12/16/17 comment-audit commits `0a471f95`, `a4bc7bd5`, `915e7fcb`; mechanism intact and unchanged since `ef84ee5e`).
- Location:
  - `cmd/doctor.go:176` — the second render is handed `collectThemeAdvisories(deps)`, a fresh call, not the first pass's slice (`cmd/doctor.go:155`).
  - `cmd/doctor.go:174-175` — the comment states why it is re-collected ("the whole second report must describe one moment").
  - `cmd/doctor.go:184-193` — `runDoctorFix` still calls exactly the three pre-existing repairs (`pruneDoctorStaleHooks`, `pruneDoctorStaleProjects`, `sweepDoctorLogs`); its doc-comment carries the no-theme-step rationale ("Themes get no repair step: every available one would destroy user-authored content").
  - `cmd/doctor.go:165-166, :177-179` — exit stays `doctorUnhealthy(postResults)`; advisories reach `doctorSummaryLine` (`cmd/doctor.go:504-514`) which only appends the suffix and never touches `doctorCheckCounts`.
  - `cmd/doctor_theme.go:35-56` — `collectThemeAdvisories`/`themeAdvisoryUnion` open a fresh `loader.OpenEnumeration(deps.ThemesDir)` per call and read prefs through `deps.PrefsStore.LoadThemeKeys()`, which re-reads the file every call (`internal/prefs/store.go:191`, `:125`) — no caching on either side, so the second pass genuinely re-diagnoses.
  - `cmd/doctor.go:96-99` — production prefs come from `loadPrefsStoreNoMigrate()`, so the `--fix` path inherits the non-migrating read; nothing on the fix branch re-routes it.
- Notes: no theme step, no directory creation, no seeding and no `.theme`/prefs write was added anywhere on the fix branch — verified by reading `runDoctorFix` and its three callees end to end, and confirmed by the commit diff (`git show ef84ee5e -- cmd/doctor.go` touched only the render argument and two comments). The loader is `theme.NewSilentLoader()` = `NewLoader(NewEventLogger(log.Discard()))` (`internal/theme/load.go:43-45`), which is what makes the zero-`theme`-records claim structural rather than incidental.

TESTS:
- Status: Adequate (all eight planned tests present in `cmd/doctor_fix_theme_test.go`, unit lane, no `t.Parallel()`, no daemon/binary spawn — correct lane).
- Coverage:
  - `TestDoctorFix_AdvisoriesInBothPasses` (:63) — two broken drop-ins plus a valid one that stays silent; asserts both reports close with the same two-line block + `7 checks passed · 2 advisories`. Covers AC1.
  - `TestDoctorFix_SuffixInBothSummaries` (:82) — suffixed-summary count == 2, plus a subtest where a stale hook makes the two summaries differ (`6 of 7` → `7`) while both carry ` · 2 advisories`. Covers AC1/AC3.
  - `TestDoctorFix_AdvisoryOnlyExitsZero` (:130) — nil error, no `Pruned ` substring anywhere in stdout, file + persisted lines in both reports. Covers AC2.
  - `TestDoctorFix_ThemeStateUntouched` (:185) — `portaltest` tree fingerprint of the whole config root before/after (`treeFingerprint`/`assertTreeUnchanged`, `cmd/theme_test.go:366-382`) plus a `prefs.json` byte-compare, gated by a pre-assertion that the run actually found 8 advisory lines (4 per pass) so the untouched claim is not about an empty scan. Subtests add the `theme_migrated`-absent check and the "creates no themes directory when there is none" §5.5 check. Covers AC4/AC5.
  - `TestDoctorFix_ScanReRunForSecondPass` (:242) — behavioural (call `themeAdvisoryUnion`, delete a broken file, call again, second result reflects disk) plus an AST guard over `doctor.go` requiring exactly two `renderDoctorReport` calls each handed a `collectThemeAdvisories` *call* rather than an identifier. Covers AC8 both ways as the task asked.
  - `TestDoctorFix_EmitsNoThemeRecords` (:302) — `logtest.Sink` over the whole Execute via `assertNoThemeRecords`, with a 12-line (6 per pass) precondition. Covers AC6.
  - `TestDoctorFix_RemainsBootstrapExempt` (:326) — `skipTmuxCheck["doctor"]` plus an orchestrator call-count of 0 over a full theme surface, and a probe subtest proving the seam does fire for a non-exempt command. Covers AC7.
  - `TestDoctorFix_ExistingRepairsUnchanged` (:378) — both prunes + the log sweep still applied (via the shared `assertStalePrunesApplied`), and the down-server hazard-guard deferral still defers (`assertDownServerDeferral`), each with the advisory block asserted on both reports. Covers AC9.
- Notes:
  - Anti-vacuity discipline is unusually strong and worth calling out: `fixThemeFixture` (:161) installs `syncPersistTranslation` because `TestMain` no-ops `persistTranslation` package-wide (`cmd/testmain_isolation_test.go:24`) — without it the byte-identical assertion would pass even against a doctor rewired to the migrating read; `assertNoThemeRecords` (`cmd/theme_test.go:384`) proves the capture harness can see a `theme` record before trusting the zero; the advisory-line counts precede the untouched/zero-record assertions; and the bootstrap-exempt test proves the orchestrator seam is live.
  - AC4's "modes" clause is covered indirectly: `portaltest.Fingerprint` (`internal/portaltest/fingerprint.go:22-31`) records no permission-bit field, but it records `CtimeNanos` and `DiffFingerprints` compares it (`:179-181`), so a `chmod` surfaces as a `ctime` delta. Adequate; an explicit mode field would be over-testing for this criterion.
  - Later remediation phases (13-5, 13-8, 14-4, 14-7, 16-3) refactored this file onto shared fixtures/helpers, shrinking it from 659 to 435 lines. I checked the eight planned test names all survive with their assertions intact — this is dedup, not coverage loss.
  - One mild redundancy between two of the planned tests (see notes).

CODE QUALITY:
- Project conventions: Followed. Unit-lane placement is correct (no daemon, no built binary); the command body is driven through the package-level `doctorDeps` seam with `t.Cleanup` restore (`runDoctorFixCmd`, `cmd/doctor_test.go:1191`) and `isolateTerminalsFile`, matching the cmd DI pattern; no `t.Parallel()`; the source guard reuses the shared `parsePackageFilesByName` enumeration rather than hand-rolling a walk.
- SOLID principles: Good. The change is a single call-site edit at the composition root; the read-only collector stays ignorant of which pass it serves, and the repair function stays ignorant of themes.
- Complexity: Low. No new branches, no new state; `RunE` reads linearly.
- Modern idioms: Yes (`strings.SplitSeq` in the report splitter, `maps.Keys`/`slices.Sorted` in the snapshot precondition).
- Readability: Good. The two surviving comments (`cmd/doctor.go:174-175`, `:184-186`, `cmd/doctor_theme.go:35-37`) each state a decision the code cannot state itself and are accurate against the code — I checked each claim, including the `TestMain`/`syncPersistTranslation` claim at `cmd/doctor_fix_theme_test.go:156-160` and the "excludes repair breadcrumbs" claim at `:17-19`. No process-artifact references, no restated code.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] cmd/doctor.go:187,167 — `runDoctorFix` returns `error` but every path returns nil (all three callees are `func(...)` with no return, each swallowing its own failure by design), so the caller's `if err := runDoctorFix(cmd, deps); err != nil { return err }` at :167 is unreachable. Drop the return value and call it as a statement. Pre-existing — this task correctly added nothing to the function — so it is out of this task's scope but sits directly in the code it touched.
- [quickfix] cmd/doctor_fix_theme_test.go:82-97 — the top-level body of `TestDoctorFix_SuffixInBothSummaries` (same two-broken-theme fixture, asserting the suffixed summary appears twice) restates what `TestDoctorFix_AdvisoriesInBothPasses` (:63) already pins via `requireBothReportsEndWith`, which includes the suffix. The distinct value is the nested "the suffix rides both summary forms" subtest (`6 of 7` vs `7`). Promote that subtest's body to be the test body and delete the duplicated top-level fixture and count assertion, keeping the plan-named test.
