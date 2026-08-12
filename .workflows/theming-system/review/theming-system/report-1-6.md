TASK: theming-system-1-6 — `themesDirPath` in `cmd/config.go`, with the directory injected into the loader

ACCEPTANCE CRITERIA:
1. `PORTAL_THEMES_DIR=/tmp/x` resolves to `/tmp/x` regardless of `XDG_CONFIG_HOME` or `HOME`.
2. `PORTAL_THEMES_DIR=""` with `XDG_CONFIG_HOME=/tmp/cfg` resolves to `/tmp/cfg/portal/themes`.
3. `XDG_CONFIG_HOME` unset with `HOME=/tmp/h` resolves to `/tmp/h/.config/portal/themes`.
4. A home-directory resolution failure returns an error, not an empty path or a panic.
5. Calling `themesDirPath()` any number of times creates nothing on disk.
6. `configFileComponents` unchanged, no routing through `configFilePath`, no migrate breadcrumb.
7. The guard test fails if `internal/theme` gains an `internal/xdg` import, a `PORTAL_THEMES_DIR` literal or a home-dir read.
8. `cmd`'s `TestMain` poisons `PORTAL_THEMES_DIR`; new tests set it with `t.Setenv`; no `t.Parallel()`.

STATUS: complete

SPEC CONTEXT:
§5.5 fixes the chain `PORTAL_THEMES_DIR` → `XDG_CONFIG_HOME/portal/themes/` → `~/.config/portal/themes/`, fixes the env-var name because `docs/theming.md` must print it, marks the `_DIR`/`_FILE` mechanical difference (directory vs file), and states there is no Application Support migration (the directory is new). The directory-state table makes **absent** the common, silent case — "Portal never creates or seeds it". §3.2 states `cmd/config.go` owns themes-directory resolution via a `themesDirPath` alongside `prefsFilePath`, and that **the loader takes the directory as an injected value and never resolves it**, which is what keeps the embedded built-in set reachable with no path at all (so `internal/capture`'s no-real-config import guard stays satisfiable). Directory-state behaviour (advisory/log on unreadable) belongs to task 1.7, not here.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - `cmd/config.go:191-205` — `themesDirPath()`; env-first with the `!= ""` empty-means-unset test (`:195`), else `xdg.ConfigBase()` joined with `portal/themes` (`:199-204`), error propagated verbatim with an empty path.
  - `cmd/testmain_isolation_test.go:19` — `PORTAL_THEMES_DIR` added to the package-wide poison set.
  - `internal/theme/leaf_guard_test.go:23-67` — `TestThemePackage_ResolvesNoPaths` (dep walk + AST scan).
  - Consumers all route through the single function: `cmd/theme_source.go:10`, `cmd/open.go:505`, `cmd/doctor.go:101`, `cmd/theme.go:56`. No second resolution site exists.
- Notes:
  - `configFileComponents` (`cmd/config.go:17-23`) is untouched — five entries, no themes member; `themesDirPath` does not call `configFilePath`, so `migrateConfigFile` is never reached and no breadcrumb can be emitted.
  - No `os.MkdirAll` / `os.Stat` / creation of any kind in the function — it returns a path and nothing more, matching §5.5's silent-absent contract.
  - The error path is exactly `xdg.ConfigBase`'s (`internal/xdg/xdg.go`), which already owns the `XDG_CONFIG_HOME`-empty-means-unset tolerance and wraps the `os.UserHomeDir` failure — no reimplementation, as the task required.
  - Injection side verified: `internal/theme` production sources contain no `os.Getenv` and no `os.UserHomeDir`; the only `os.Stat` calls (`internal/theme/enumerate.go:79,127`) act on the *injected* `Dir`, which is task 1.7's directory-state behaviour, not path resolution.
  - Callers consume the two-value contract correctly and each makes its own documented degradation choice — `cmd/theme_source.go` discards the error (`""` degrades to the embedded set), while `cmd/theme.go:69-75` folds `not found` + a resolution error into `unreadable` for export. Consistent with §3.2's "no path at all" reachability.
  - Later phases (11-17) touched the guard file only to re-home its AST helpers into `internal/sourceguardtest` and to add sibling guards (hex literals, no `init`); the mechanism of this task is unchanged and unsuperseded.

TESTS:
- Status: Adequate
- Coverage: `cmd/config_themes_test.go` implements all seven named cmd-side tests, one per acceptance criterion:
  - `TestThemesDirPath_EnvVarWins` (:10) sets all three vars — proves precedence, AC1.
  - `TestThemesDirPath_EmptyEnvFallsThrough` (:25) — AC2, the naive-`LookupEnv` failure mode.
  - `TestThemesDirPath_XDGConfigHome` (:40) — the XDG rung.
  - `TestThemesDirPath_HomeFallback` (:55) — AC3.
  - `TestThemesDirPath_HomeResolutionFailurePropagates` (:70) — AC4, asserting both `err != nil` *and* the empty path (so a partial path can't leak).
  - `TestThemesDirPath_NeverCreatesDirectory` (:84) — AC5, three calls on both rungs, asserting the resolved path *and* its parent stay absent (the parent assertion is what would catch a stray `MkdirAll(filepath.Dir(...))`).
  - `TestThemesDirPath_IsNotAConfigFilePathMember` (:125) — AC6 in two halves: `maps.Equal` against the literal expected `configFileComponents` map, and a behavioural half that seeds `Library/Application Support/portal/themes` under a temp `HOME` and asserts it is still there afterwards with zero records on a `logtest.Sink`. The seeded-old-dir assertion is the sharp one: a `configFilePath`-shaped implementation would visibly move it, and the empty-component wiring means the log assertion alone would not have caught that.
  - `internal/theme/leaf_guard_test.go:23` `TestThemePackage_ResolvesNoPaths` — AC7, `go list -deps` for the `internal/xdg` edge plus an AST pass flagging any `PORTAL_THEMES_DIR` string literal or `os.UserHomeDir` selector. Non-vacuous: `sourceguardtest.PackageGoFiles` errors on an empty match and the parse/`go list` failures are `t.Fatal`.
  - AC8: `cmd/testmain_isolation_test.go:19` poisons the var; every new test sets it via `t.Setenv`; no `t.Parallel()` in either new file.
- Notes:
  - Would fail if the feature broke: dropping the `!= ""` check breaks `_EmptyEnvFallsThrough`; adding an `os.MkdirAll` breaks `_NeverCreatesDirectory`; routing through `configFilePath` breaks both halves of `_IsNotAConfigFilePathMember`.
  - Not over-tested: no mocks, no seams invented for the test, the only setup is `t.Setenv` + `t.TempDir`. `_EmptyEnvFallsThrough` and `_XDGConfigHome` overlap on the same code path, but they pin two distinct plan criteria (empty-falls-through vs the XDG rung) and both were explicitly specified — keeping them is right.
  - `installMigrateCapture` is reused from `cmd/config_migrate_logging_test.go:13` rather than re-rolled.

CODE QUALITY:
- Project conventions: Followed. Sited immediately after `prefsFilePath` as specified; env-var literal passed inline exactly as the sibling `*FilePath` helpers do; leaf `internal/xdg` reused rather than re-implemented; the leading comment carries the *why* (not a `configFilePath` member; no macOS predecessor; returns a path and nothing more) with no process-artifact references — matching the repo's comment standard. CLAUDE.md's "Config path resolution" section already documents the deviation (Phase 10's job, done).
- SOLID principles: Good. Single responsibility — resolution only, no I/O, no judgement about directory state; the dependency-inversion point of the task (loader takes the directory injected) is structurally enforced by the guard rather than by convention.
- Complexity: Low. One branch plus one error propagation; 12 lines.
- Modern idioms: Yes. `strings.FieldsSeq` in the guard, `for range 3` in the test, `maps.Equal` for the map assertion.
- Readability: Good. Intent is legible from the comment without reading the callers.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] `internal/theme/leaf_guard_test.go:37-66` — widen the "reads neither the themes env var nor the home directory" scan so it also flags a bare `os.Getenv` call (or the `XDG_CONFIG_HOME` / `HOME` string literals) in `internal/theme`. As written the guard catches the `internal/xdg` import, the `PORTAL_THEMES_DIR` literal and `os.UserHomeDir`, but a hand-rolled `os.Getenv("XDG_CONFIG_HOME")` inside the package would satisfy all three checks and reintroduce exactly the path resolution the guard exists to forbid. Same AST pass, one extra `case`/condition.
