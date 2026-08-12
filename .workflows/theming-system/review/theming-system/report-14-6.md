TASK: theming-system-14-6 — Single-Source The Prefs Absent-Path Case Table And Its "Nothing Was Created" Assertion (Phase 14: Analysis Cycle 4 remediation; severity medium, source: duplication)

ACCEPTANCE CRITERIA:
- One absent-path case table and one "nothing was created" assertion exist in `internal/prefs`; the duplicated bodies are gone.
- All three tests range over the same table.
- Each test keeps its own outcome assertion (two decline, one creates).
- `go test ./internal/prefs` passes.

STATUS: complete

SPEC CONTEXT:
The shared table encodes the inputs behind two opposing spec contracts on the prefs write path.
- §8.9 (specification.md:922-927): "An absent file and an unusable one are different conditions, and only the second aborts" — `prefs.json` absent means "nothing to merge and nothing to lose", so the ordinary `Save` **creates** the file; an abort here "would be permanent — nothing else creates the file either". Spec line 927 explicitly notes a suite built only around merging would not catch an abort-on-absent implementation, which is what `TestSave_CreatesAbsentFile` guards.
- §8.1 / marker semantics (specification.md:741): the `theme_migrated` marker is "**Not written when `prefs.json` does not exist**" — creating the file purely to record a marker would be a new side effect on a path the feature otherwise leaves alone. `SaveMigrationMarker` and `SaveTranslation` therefore decline on absent, and declining is not an error.
So the same absent-path inputs decide create-versus-decline across three savers. Stating them once (this task) is exactly what keeps the two halves of the §8.9/§8.1 discrimination judged on identical shapes.

IMPLEMENTATION:
- Status: Implemented (matches the task's Do list step for step)
- Location:
  - `internal/prefs/store_write_path_test.go:83-101` — `assertNothingCreated(t, dir, path)`, placed beside `assertUntouched` (line 74) as instructed: `os.Stat` / `errors.Is(os.ErrNotExist)`, the `os.ReadDir(dir)` empty-tree check, and `assertNoTempFiles(t, dir)`.
  - `internal/prefs/store_write_path_test.go:152-166` — `absentPathCase` struct (`name`, `rel []string`), the `path(dir)` join method, and `absentPathCases()` returning the two documented shapes, placed beside `undecodablePrefsCases()` (line 138) as instructed.
  - `internal/prefs/store_write_path_test.go:168-182` — `TestSave_CreatesAbsentFile` re-pointed at the table, keeping its opposite outcome assertion (`assertWrittenValue(... "session_list_mode", "by-tag")` + `assertNoTempFiles`) untouched.
  - `internal/prefs/migration_marker_test.go:313-326` — `TestSaveMigrationMarker_DoesNotCreateAbsentFile` reduced to the table + its own "declining to write is not a failure" fatal + `assertNothingCreated`.
  - `internal/prefs/translation_saver_test.go:110-127` — `TestSaveTranslation_DoesNotCreateAbsentFile` likewise, retaining the extra `persisted == false` assertion inside the loop body as instructed.
- Commit: `c709fa7c` (+57/-57 across the three test files). The diff removes exactly the two duplicated ~28-line bodies and the third copy of the case table; nothing else in those tests moved.
- Notes:
  - All three files are `package prefs` (white-box), so the shared helpers are reachable from every consumer — the package-split hazard called out at `store_write_path_test.go:14-15` (`theme_keys_test.go` is `package prefs_test`) does not apply here.
  - Imports stay consistent: `translation_saver_test.go` correctly dropped `os` (no longer used there); `migration_marker_test.go` kept `os`/`errors`/`filepath`, which are still used at lines 346-350, 429 and 445-447. No unused-import compile break.
  - Do-step 5 (add no new claim) was respected — no new decline-on-absent test was invented for any other saver.
  - Superseded, not drift: the task's Do-step 1 asked for the table to be documented, and commit `c709fa7c` added that doc comment. The later package-wide sweep `fee1927d` ("strip internal/prefs to the code-quality standard", comment-only, 864 → 122 comment lines) removed it along with every other comment on unexported test scaffolding. Judged against the amended intent, this is a deliberate later decision, not a regression of this task.

TESTS:
- Status: Adequate (this task *is* test scaffolding; the assertions it carries are unchanged in strength)
- Coverage:
  - Both absent-path shapes (file absent; parent directory absent too) now run against all three savers from one declaration — structurally, adding a case to `absentPathCases()` reaches `Save`, `SaveMigrationMarker` and `SaveTranslation` with no other edit, which is the task's stated first mutation check.
  - The decline half is still fully asserted: file absent, temp dir wholly empty (nothing created above the file either), no `.atomic-` leftovers, and for `SaveTranslation` `persisted == false`.
  - The create half (`TestSave_CreatesAbsentFile`) still decodes the written file and asserts the value, so a saver that flipped from create to decline fails only that test — the second mutation check holds by construction.
  - The nested-path case is meaningful in both directions: for the decline tests the empty-`ReadDir(dir)` assertion proves the `sub/nested` tree was not created; for the create test `assertNoTempFiles(filepath.Dir(path))` still inspects the leaf directory the write had to make.
- Notes:
  - No information was lost from failure messages (Do-step 6). The translation copy's "the translation must create nothing" became the shared "the save must create nothing", but the failing saver is still named by the enclosing test (`TestSaveTranslation_DoesNotCreateAbsentFile/...`), and the shared message is strictly richer — it now prints the directory path and the names of the offending entries rather than only a count, and `assertNothingCreated`'s `os.Stat` error names the path.
  - Not over-tested: the helper asserts one contract ("nothing was created") in three complementary ways rather than repeating one; the only redundancy is the trailing `assertNoTempFiles` (see non-blocking notes).
  - Not run: per verifier constraints the suite was not executed. Compile-level review found no breakage (package, imports, helper signatures and call sites all line up), so `go test ./internal/prefs` is expected to pass unchanged.

CODE QUALITY:
- Project conventions: Followed. Helpers take `*testing.T` first and call `t.Helper()` (`assertNothingCreated:85`), matching the package's existing `assertUntouched`/`assertNoTempFiles` shape and the repo's `thelper` expectation. Table-driven subtests with `t.Run(c.name, ...)`; no `t.Parallel()` (correct — CLAUDE.md prohibits it). The case table is a function returning a fresh slice, mirroring `undecodablePrefsCases()`, so no shared mutable package state.
- SOLID principles: Good. `absentPathCase.path` is a one-line method on the data it belongs to, keeping the `filepath.Join(append([]string{dir}, c.rel...)...)` incantation stated once; the table states inputs, the helper states the outcome, and each test keeps its own call under test — clean separation of fixture from assertion.
- Complexity: Low. Helper is straight-line; no branching beyond the two guard checks and the names loop.
- Modern idioms: Yes. `errors.Is(err, os.ErrNotExist)` rather than `os.IsNotExist`; `make([]string, 0, len(entries))` pre-sized; `t.TempDir()` for isolation.
- Readability: Good. `absentPathCases()` / `assertNothingCreated` read as intent at the three call sites, and both consumer tests are now four lines of substance each.
- Comment accuracy: N/A on the current tree — the later comment sweep left no comments in the changed region, so there is nothing that can be false. The comments introduced by this task (now removed) contained no task ids or phase references.
- Security / performance: N/A (test-only scaffolding, temp dirs).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/prefs/store_write_path_test.go:100 — the trailing `assertNoTempFiles(t, dir)` inside `assertNothingCreated` is subsumed by the empty-tree check three lines above it: a `.atomic-` leftover in `dir` already makes `len(entries) != 0` fail and is already printed by name, so this call can only ever emit a second error for a directory the stronger check has just failed. Delete the call (the `assertNoTempFiles` helper stays, used by `assertUntouched` and `TestSave_CreatesAbsentFile`).
- [quickfix] internal/prefs/store_write_path_test.go:326-339 and internal/prefs/theme_savers_test.go:371-383 — two remaining hand-rolled copies of the same `os.Stat` / `errors.Is(os.ErrNotExist)` + `assertNoTempFiles(t, dir)` pair (`TestMutate_DecliningMutatorWritesNothing`'s "an absent file is not created" subtest, and the invalid-slot subtest of the theme savers). Both are single-shape, not the two-case table, so they are outside this task's scope-by-design (Do-step 5, "adds no claim"), but re-pointing each at `assertNothingCreated(t, dir, path)` would remove the last of the duplication. Note it strengthens both sites with the empty-tree claim — that is the only reason it is a follow-up rather than part of this task.
