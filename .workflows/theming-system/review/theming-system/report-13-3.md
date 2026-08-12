TASK: theming-system-13-3 — Single-Source The Repo-Wide Go Source-Guard Scaffolding (tick-899c3e, analysis-remediation, severity medium, source: duplication)

ACCEPTANCE CRITERIA:
- No hand-rolled `go.mod` walk remains in any test; both former copies call `portalbintest.ProjectRoot()`.
- One `.go` enumeration helper with one exclusion rule; all four guards consume it.
- The prefs guard's coverage matches the other three.
- `tui_test` performs one walk, not two.
- Every guard still fails when its forbidden symbol or import is reintroduced.
- `go test ./...` (unit lane) passes and no test moves lane.

STATUS: complete

SPEC CONTEXT: The specification governs the guards' *claims* (§13.4 swap-and-diff, §13.3 import guards, §11.4's "do not drop" canvas-echo guard, §8.8's appearance-API deletion, §7.6's built-in guarantee) but says nothing about the scaffolding that enumerates source files — this task is a plan-authored duplication remediation, judged against its own Do steps. The load-bearing spec-side constraint it must not disturb is that these guards are the enforcement layer for "this identifier/import must not exist anywhere" (§11.4 explicitly warns the `canvasHexFor` guard must not be dropped), so an enumeration that silently narrows would be a hole in a spec-mandated protection. CLAUDE.md's lane rule also applies: these guards are unit-lane, so the shared helper must carry no build tag.

IMPLEMENTATION:
- Status: Implemented (mechanism later relocated by tasks 16-5 / 17-14 — intentional supersession, not drift)
- Location:
  - `internal/sourceguardtest/gosourcefiles.go:13` — `GoSourceFiles(root)`, one `filepath.WalkDir`, one exclusion rule in `excludedGuardDir:36` (dot-directories, `vendor`, `node_modules`), stdlib-only, no build tag.
  - `internal/sourceguardtest/doc.go:1` — records why the primitives live outside `portalbintest` (which builds binaries) and that the package is unit-lane and test-only.
  - `internal/prefs/appearance_api_guard_test.go:29,37` — `portalbintest.ProjectRoot()` + `sourceguardtest.GoSourceFiles`.
  - `internal/theme/broken_builtin_test.go:255,262` — same; `themeRepoRoot`'s ~18-line `go.mod` walk deleted (commit cfb550a9).
  - `internal/tui/restore_source_guard_test.go:234` — `allGoFiles` is now a thin wrapper over `sourceguardtest.GoSourceFiles` with a zero-match fatal at :240.
  - `internal/tui/theme_source_guard_test.go:194` (`repoRoot` → `ProjectRoot`), `:203` (`forEachGoFile` now loops `allGoFiles(t, root)`, keeping its `parser.ImportsOnly` parse mode).
- Verification of each criterion:
  - Hand-rolled `go.mod` walks: `grep` over `**/*_test.go` finds none; the only `go.mod` stat is `portalbintest.ProjectRoot` itself (`internal/portalbintest/build.go:17`) plus its own test.
  - One enumeration, four consumers: confirmed, and adoption has since widened to `internal/theme/slug_collapse_guard_test.go:24` and `internal/theme/loader_construction_guard_test.go:25`.
  - Prefs coverage now matches: its old `.git`/`vendor`/`node_modules`-only skip is gone. Do-step 5's substantive check holds — the only `.go` files under a newly-excluded directory are `.claude/skills/golang-cli/assets/examples/*.go`, and a grep for every policed construct (`prefs.Appearance`, `parseAppearance`, `Appearance*`, `canvasHexFor`, `BuiltinSource`, `internal/tui/theme`) across `.claude`/`.github`/`.workflows` returns nothing, so no guard lost meaningful reach.
  - `tui_test` one walk: `forEachGoFile` and `allGoFiles` now share a single enumeration; the second *parse* mode is retained deliberately (imports-only vs full AST), which was the correct read of Do step 4.
  - Guard claims unchanged: the commit diff (cfb550a9) shows only root resolution and enumeration moving — forbidden-identifier lists, forbidden-import checks, exemption maps, the `builtinSourceOwners` completeness sentinel and the error copy are byte-identical apart from later chore-commit comment/§-citation stripping.
  - Lane: `sourceguardtest` and `portalbintest` carry no build tag; nothing moved lane.
- Notes: The task text said "delete `repoRoot`"; the implementation kept the name as an 8-line `t.Helper()` wrapper around `ProjectRoot()` used from three call sites in `tui_test`. That satisfies the criterion as written (no hand-rolled walk survives) and is the DRY choice for a package with multiple guards — not drift. The helper landing in `internal/sourceguardtest` rather than `internal/portalbintest` is later-task supersession (16-5 re-homed it, 17-14 renamed it to the `*test` convention) and is the better home: it keeps unit-lane guards off a package that shells out to `go build`.

TESTS:
- Status: Adequate
- Coverage: `internal/sourceguardtest/gosourcefiles_test.go` covers the three things that matter for a helper whose failure mode is silence — `.go`-only enumeration including `_test.go` (:12), the exclusion rule with near-miss directory names (`internal/vendorish`, `internal/node_modulesish`) so a naive `strings.Contains` implementation would fail (:33), and a hard error on a missing root (:57). The guards themselves remain the behavioural tests for the forbidden constructs. Two of the three consuming packages also carry an anti-vacuum sentinel: `restore_source_guard_test.go:240` fatals on zero files, `broken_builtin_test.go:294` fatals if the exemption list stops matching real files.
- Notes: Not over-tested — three focused cases, one shared `relFiles`/`writeFiles` fixture, no mocking. The task's prescribed verification (temporarily reintroduce each forbidden construct; drop a scratch `.go` under an excluded dir) is manual and correctly left uncommitted. The one gap is the prefs guard, which has no zero-paths sentinel (see notes below); it is the only one of the four that could pass vacuously if enumeration ever returned nothing.

CODE QUALITY:
- Project conventions: Followed. Test-only package, `*test` name suffix per CLAUDE.md's helper-package convention, verified no production importer; stdlib-only and untagged so unit-lane guards keep running; `t.Helper()` on every test wrapper; no `t.Parallel()`.
- SOLID principles: Good. `GoSourceFiles` does one thing and returns paths, leaving the parse mode to each caller — the exact seam that let `tui_test` share one walk while keeping two parse strategies.
- Complexity: Low. One walk closure, one predicate, no nesting beyond two levels; the guards' bodies got simpler (early `continue` replacing `return nil` inside a closure, `t.Fatalf` at the point of failure instead of error tunnelling out of `WalkDir`).
- Modern idioms: Yes — `fs.SkipDir`, `%w` wrapping, `slices.Equal` in the tests.
- Readability: Good. The one stated exclusion rule sits beside the walk with its rationale, which is what the task asked for.
- Issues: The exclusion comment's rationale is loose in one clause (see the do-now note); no correctness issue.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/sourceguardtest/gosourcefiles.go:11-12 — the comment's rationale "Go's own tooling ignores them" is true of dot-directories and `vendor` but not of `node_modules` (the toolchain has no rule for it), and Do step 5 asked for the excluded-directory Go sources to be recorded here. Replace the doc comment body with: "// GoSourceFiles returns every .go file under root, test sources included, as\n// paths joined onto root. Dot-directories, vendor and node_modules are skipped —\n// one rule, shared by every repo-wide guard, so none can be narrower than its\n// siblings by accident. The only .go files this excludes today are the vendored\n// skill examples under .claude/skills; they are not part of the module and hold\n// none of the constructs the guards police."
- [quickfix] internal/prefs/appearance_api_guard_test.go:37 — the guard has no anti-vacuum sentinel, so a zero-length enumeration would make it pass silently; its two siblings both have one. After the `GoSourceFiles` call add `if len(paths) == 0 { t.Fatalf("enumerated no .go files under %s", root) }`, mirroring internal/tui/restore_source_guard_test.go:240.
- [quickfix] internal/sourceguardtest/gosourcefiles.go:19-23 — the exclusion predicate is applied to the root entry itself, so `GoSourceFiles` on a module root whose own basename starts with "." (e.g. a checkout at `~/.portal`) returns zero files with a nil error rather than walking it. Skip the check for the root entry (`if path != root && excludedGuardDir(entry.Name())`) and add a case to `TestGoSourceFiles_SkipsExcludedDirectories` covering a dot-prefixed root.
- [quickfix] internal/log/discard_guard_test.go:24 and internal/log/migration_guard_test.go:27 — two further repo-wide source guards (pre-existing, outside this task's named four) still hand-roll the walk with the narrower `.git`/`vendor`/`node_modules` exclusion, so the task's "one decision in one place" outcome is not yet repo-wide. Replace both `filepath.WalkDir` closures with `sourceguardtest.GoSourceFiles(root)` + the existing per-file filtering, as the four converted guards do.
