TASK: theming-system-16-5 — Re-Home The Go Source-Guard AST Helpers Out Of The Binary-Build Test Package (tick-fdac23)

ACCEPTANCE CRITERIA:
- `internal/portalbintest` exports only `ProjectRoot`, `BuildPortalBinary` and `StagePortalBinary`, and its doc describes only those.
- The three source-scanning helpers and their tests live in the new package with their docs intact.
- Every guard consumer compiles against the new import path with no behavioural change.
- Every guard still fails when its forbidden construct is reintroduced.
- CLAUDE.md's inventory describes both packages accurately.
- `go test ./...` passes, `go test -tags integration -p 1 ./...` passes, and no test changes lane.

STATUS: complete

SPEC CONTEXT: No specification section governs this task — it is an architecture-remediation item raised by the plan's cycle-6 analysis phase (Phase 16), not a spec requirement (a grep of `specification.md` for `portalbintest` / `sourceguard` / `GoSourceFiles` returns nothing). The governing context is CLAUDE.md's test-helper package inventory and its rule that test-only helper packages are named for what they hold and must not be imported by production code. Note the later supersession: task 17-14 (`ea6d95e5`) renamed the package created here from `internal/sourceguard` to `internal/sourceguardtest` to match the repo's `*test` test-only naming convention, and two subsequent comment-standard chores (`578c7929`, `915e7fcb`) condensed the moved doc comments. Both are intentional later revisions; the task is judged against the current tree with those amendments in force.

IMPLEMENTATION:
- Status: Implemented (as amended by 17-14). Commit `d4a6abf3`, 28 files.
- Location:
  - `internal/sourceguardtest/gosourcefiles.go:13` (`GoSourceFiles` + unexported `excludedGuardDir:36`), `internal/sourceguardtest/packagegofiles.go:15` (`PackageGoFiles`), `internal/sourceguardtest/foreachfunccall.go:11` (`ForEachFuncCall`), `internal/sourceguardtest/doc.go:1` (package charter).
  - `internal/portalbintest/build.go:1-73` — doc trimmed to the build/stage surface; the file now holds exactly `ProjectRoot:17`, `BuildPortalBinary:37`, `StagePortalBinary:44` plus the unexported `buildPortalBinaryInto:61`. No source-scanning symbol survives (grep of `portalbintest.` repo-wide returns only those three names).
  - Consumers re-pointed: `internal/theme/{broken_builtin,leaf_guard,loader_construction_guard,slug_collapse_guard,resolve}_test.go`, `internal/tui/{colour_literal_guard,restore_source_guard,retired_token_guard,nomination,header_rule,modal_placement_consolidation,pagepreview_surface_audit,theme_panel_commit,theme_panel_confirm}_test.go`, `internal/prefs/appearance_api_guard_test.go`, `internal/capture/theme_panel_message_fixtures_test.go:327`, `cmd/{doctor_persisted_theme,open_theme_nomination,prefs_translation}_test.go`. No file anywhere still imports `internal/sourceguard` (the pre-17-14 path).
  - `internal/tui/pagepreview_surface_audit_test.go:201` — the new package is enrolled in the surface-audit allow-list (currently `"sourceguardtest"`), so the "no new package" audit stays honest rather than being silently widened.
  - `CLAUDE.md:85` — the shared test-helper row now reads `restoretest / tmuxtest / portalbintest / sourceguardtest / transienttest`; `portalbintest` is described as build/stage only ("build/stage is its whole surface, and it holds no source-scanning helpers") and `sourceguardtest` is described with its three helpers, its stdlib-only/untagged property, and the row-level "production code must not import these".
- Notes:
  - The move is genuinely a move: diffing the pre-move `internal/portalbintest/*.go` against the current `internal/sourceguardtest/*.go` shows the function bodies byte-identical — same walk, same `excludedGuardDir` exclusion set (dot-dirs, `vendor`, `node_modules`), same `PackageGoFiles` empty-match error, same `ForEachFuncCall` whole-`FuncDecl` traversal and whole-walk stop semantics. No helper was added, merged, widened or narrowed, and no guard's assertion, exclusion rule, parse mode or failure wording changed (Do items 1 and 7 respected).
  - Doc comments were carried over verbatim at `d4a6abf3`; the shorter forms in the tree today are the product of the later repo-wide comment-standard chores, not of this task.
  - `ProjectRoot` correctly stayed in `portalbintest` (Do item 4) and is still consumed by both families — `internal/log`, `cmd/capturetool`, `internal/tui`, `internal/theme`, `internal/prefs` guards call it for the repo root while scanning through `sourceguardtest`. That split is deliberate and documented, not drift.
  - `internal/log` and `cmd/capturetool` appear in Do item 5's consumer list but needed no change: they only ever used `ProjectRoot`, which did not move. Not an omission.
  - No production (non-`_test.go`) file imports `sourceguardtest`; the `*test` suffix carries the boundary at the import line, consistent with the other test-only helper packages.

TESTS:
- Status: Adequate.
- Coverage: The three moved test files (`internal/sourceguardtest/gosourcefiles_test.go`, `packagegofiles_test.go`, `foreachfunccall_test.go`) landed with every assertion intact — a line-by-line diff against their pre-move versions shows only the package clause, the import path and the call qualifier changed (plus the later comment-standard strip of the test doc comments). Retained cases: enumerate-under-root incl. `_test.go` sources, the dot-dir/`vendor`/`node_modules` exclusion (with the `vendorish`/`node_modulesish` near-miss directories that stop the exclusion being a prefix match), missing-root error, package-local production-only vs. with-tests enumeration, missing-dir error, the anti-vacuity empty-match error, call-visiting at depth (nested args, closures, methods), the bodyless-declaration no-panic case, and `false` ending the whole walk rather than the current declaration. Their local fixtures (`writeFiles`, `relFiles`, `packageFixture`, `parseSource`, `callName`) moved with them; none is orphaned in `portalbintest`, whose `build_test.go` still covers only `ProjectRoot` and `StagePortalBinary`.
- Notes:
  - Lane is unchanged: no file in `internal/sourceguardtest` carries a `//go:build` tag and the package imports stdlib only (`go/ast`, `io/fs`, `os`, `path/filepath`, `strings`, `fmt`), so every guard it serves stays in the unit lane. The commit added no build tag to any consumer.
  - The "reintroduce a forbidden construct behind each guard class and confirm it still fails" step is a manual, revert-after check I cannot replay by reading; its risk is structurally nil here, since the helper bodies are byte-identical to the pre-move versions and each guard's assertion text is untouched, so a guard's pass/fail behaviour can only have changed if the helper changed — it did not.
  - Not over-tested: no test was added for the move itself (correctly — a move needs no new test), and no duplicate coverage was left behind in `portalbintest`.

CODE QUALITY:
- Project conventions: Followed. Test-only package named with the `*test` suffix per CLAUDE.md's inventory convention (post-17-14); stdlib-only and untagged so unit-lane guards can import it; no `t.Parallel()` in the moved tests; the package doc states the test-only boundary in the same shape as the sibling helper docs; CLAUDE.md's inventory was updated in the same change rather than left to rot.
- SOLID principles: Good. This is precisely a single-responsibility repair — `portalbintest` is back to "build and stage the portal binary", `sourceguardtest` holds "scan the repo's Go source", and each package doc now matches its contents. Neither package gained a dependency on the other.
- Complexity: Low. Pure relocation; no control flow changed.
- Modern idioms: Yes — `filepath.WalkDir`/`fs.SkipDir`, `os.ReadDir`, `ast.Inspect`, wrapped errors with `%w`, `slices.Equal`/`slices.Sort` in the tests.
- Readability: Good. Filenames match the single exported helper each holds; the surface-audit allow-list entry explains why the new package is exempt rather than just listing it.
- Issues: One doc-rationale sentence overstates its case — see the first non-blocking note.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/sourceguardtest/doc.go:10-12 — the closing rationale reads "These primitives stay here rather than folding back into portalbintest: guards across several packages share them, and portalbintest builds binaries, which a package serving unit-lane guards must not drag in." The "must not drag in" claim is falsified by the repo's own history: before this task every one of these guards imported `portalbintest` and ran in the unit lane without issue — importing it costs nothing at compile time (`os/exec` + `testing`), the `go build` only happens when a caller invokes it. Replace the final clause so the reason is the boundary rather than a cost that does not exist: "These primitives stay here rather than folding back into portalbintest: guards across several packages share them, and portalbintest is the helper that builds and stages the portal binary — a unit-lane guard should reach a source walk without importing the binary-build package."
- [idea] internal/log/discard_guard_test.go:24 and internal/log/migration_guard_test.go:27 — these two guards still hand-roll their own `filepath.WalkDir` over the repo root (skipping only `.git`, `vendor`, `node_modules`) while importing `portalbintest` for `ProjectRoot`, so they are the last repo-wide guards outside `sourceguardtest.GoSourceFiles`. Routing them through it would finish the single-source story, but it is not a mechanical swap: `GoSourceFiles` excludes *all* dot-directories and returns test sources too, so the log guards' own `_test.go` filter and exclusion set would have to be re-decided — a scope call this task explicitly forbade ("do not change which directories any guard walks"), hence an idea for a follow-up rather than an edit here.
