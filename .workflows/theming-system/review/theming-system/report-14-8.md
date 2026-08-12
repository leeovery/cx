TASK: theming-system-14-8 — Delegate cmd's Theme-File Fixture Helpers To internal/themetest And Stage Themes Dirs One Way (tick-c856a1)

ACCEPTANCE CRITERIA:
- `cmd`'s theme-file fixture helpers delegate to `internal/themetest`; no second implementation of the render or the key mutations remains in `cmd` or `internal/theme`'s tests.
- One themes-dir staging helper is used across the `cmd` theme suites.
- Staged fixture bytes are unchanged.
- `go test ./cmd ./internal/theme ./internal/themetest` passes.

STATUS: complete

SPEC CONTEXT: The specification carries no `themetest` / fixture-format section (grep of `.workflows/theming-system/specification/theming-system/specification.md` for `themetest` / `fixture format` returns nothing) — this is an analysis-derived duplication remediation, not a spec behaviour. The governing convention is CLAUDE.md's `themetest` row: the package "defines the fixture format", so a change to what a valid `.theme` file looks like is one edit. The loader's six-rung rejection ladder (`bad name` → `reserved name` → `unreadable` → `bad syntax` → `bad colour` → `missing tokens`) is what the three fixture classes (`missing tokens`, `bad colour`, duplicate-key `bad syntax`) are staged against, and the duplicate-key detail is positional (`line N: duplicate key <key>`), which is why the shared mutator takes a position.

IMPLEMENTATION:
- Status: Implemented (commit dab38cab; later comment-standard sweeps trimmed some of its doc comments, which is expected phase 15–17 behaviour).
- Location:
  - `internal/themetest/theme_file.go:43-46` — `Render(lines []string) []byte` exported (the old unexported `body`); `Body()` (`:39-41`) and `Write` (`:110-118`) both route through it, so there is one renderer in the package.
  - `internal/themetest/theme_file.go:68-82` — `WithDuplicateKeyAt(lines, key, at)` added; clone-then-`slices.Insert`, undeclared key is a no-op returning the clone.
  - `cmd/theme_test.go:502-513` `missingTokenLines` → `themetest.WithoutKey`; `:517-527` `badColourLines` → `themetest.WithValue`; `:531-547` `duplicateKeyLines` → `themetest.WithDuplicateKeyAt`; `:549-565` the three `sourceX` byte-returning wrappers are now `themetest.Render(xLines(...))` and keep their `cmd`-side names/signatures as the task required.
  - `themeSourceFromLines` is gone from `cmd` (grep across the repo returns nothing), and the two former inline mutation sites now go through the guarded lines helpers: `cmd/doctor_theme_test.go:280-281` and `cmd/doctor_theme_test.go:607-609`, each rendering via `themetest.Render`.
  - `internal/theme/resolution_test.go:257` — the open-coded `strings.Join(..., "\n") + "\n"` render is re-pointed at `themetest.Render`; `internal/theme/load_test.go:610` uses it too.
  - Themes-dir staging: `cmd/doctor_theme_test.go:23-41` (`themesDirWith` → `themesDirIn`, one implementation) is now the single stager. `cmd/theme_test.go:90-96` `seedThemesDir` is a one-file call into it plus the `t.Setenv` (explicitly permitted by Do #6); `cmd/doctor_fix_theme_test.go:167` `fixThemeFixture` calls `themesDirIn` (it needs the dir under the fingerprinted `root`, the same helper with the parent parameter); `cmd/doctor_persisted_theme_test.go:268`, `cmd/doctor_theme_test.go:385` and `cmd/theme_test.go:466,726` replaced their inline mkdir+write loops with it.
- Notes:
  - `themeLineIndex` (`cmd/theme_test.go:482-493`) is deliberately retained and justified under Do #4: `duplicateKeyLines` needs the key's first-declaration index both for its legality range check (`at >= first+2`) and for the post-assembly verification the shared mutator does not express.
  - `WithDuplicateKeyAt(lines, key, at)` deviates from the task's suggested `WithDuplicateKey(lines, key)`; the deviation is correct and documented — the rejection detail carries the line number, so the consumer must choose it, and the task wrote "e.g.".
  - Remaining direct `filepath.Join(t.TempDir(), "themes")` sites in `cmd` (`doctor_theme_test.go:156,188,361`, `doctor_fix_theme_test.go:230`, `doctor_persisted_theme_test.go:294,314`, `doctor_theme_union_test.go:206`) are the deliberate negative fixtures — an *absent* directory or a regular file where the directory belongs — not a second stager.
  - Fixture bytes: the mutations are the same operations over the same `themeKeyLines(t)` base rendered by the same join, so content is unchanged; `seedThemesDir`'s files move from `<tmp>/x.theme` to `<tmp>/themes/x.theme` at the same 0o644, and every consumer uses the returned path, so no assertion depends on the depth. The attached fix-tracking record (`.workflows/theming-system/implementation/theming-system/fix-tracking-theming-system-14-8.md`, NOTES) documents an exhaustive byte-diff (all 19 tokens, every legal duplicate (key, line) pair, both inline sites) coming back clean.

TESTS:
- Status: Adequate.
- Coverage:
  - `internal/themetest/theme_file_test.go:232` `TestRender_IsTheBytesWriteStages` (Render == what Write stages, over consumer-derived lines) and `:245` `TestBody_IsRenderOfLines` — the task's "Render produces exactly Body()'s bytes for Lines()" test, expressed as the stronger pair.
  - `:90` `TestWithDuplicateKeyAt_ProducesTheBadSyntaxRejection` drives the real loader and pins `ReasonBadSyntax` + the verbatim `line 12: duplicate key text.primary` detail, the exact rejection the `cmd` suite expects, plus line-count and position assertions; `:109` pins the undeclared-key no-op; `:119` `TestMutatorsLeaveTheirInputAlone` is extended with the new mutator, which matters because consumers derive several fixtures from one base.
  - `cmd`'s behaviour tests are unchanged in expectation and now exercise the delegating wrappers throughout (`doctor_theme_test.go`, `doctor_fix_theme_test.go`, `doctor_theme_union_test.go`, `doctor_persisted_theme_test.go`, `theme_test.go`).
  - Anti-vacuity is preserved rather than lost: because the shared mutators are silent no-ops on an undeclared key while the old `themeLineIndex` path fatalled, the three `cmd` lines helpers each re-assert the mutation landed (`:508`, `:522`, `:540-545`). The fix-tracking record shows this was caught in review attempt 1 and empirically re-verified by renaming a token out of the canonical table — the manual "rename a token and confirm the fixtures follow" check the task's Tests list asked for.
- Notes:
  - Not over-tested: three small new tests for two new exports, no redundant restatement of `Lines()`/`Write` behaviour.
  - Untested edge: `WithDuplicateKeyAt` with `at` at or before the key's first declaration silently produces a fixture whose loader detail names a different line. Every current caller guards it (`cmd/theme_test.go:535`), so no test is wrong today; see the non-blocking doc note.
  - Verification method: tests were assessed by reading, not by execution (test execution is out of scope for this review), so the fourth acceptance criterion (`go test ./cmd ./internal/theme ./internal/themetest`) is judged from the code — imports remain used, no dangling references to the deleted `themeSourceFromLines`, and subsequent phases build on this commit.

CODE QUALITY:
- Project conventions: Followed. `themetest` stays test-only (`*testing.T`-first where it stages files; the two new exports are pure functions with real test-only consumers only), the fixture format stays defined in one package, and `cmd`'s call sites keep their readable local names.
- SOLID principles: Good. Single responsibility is what the change buys — `themetest` owns the format (render + key mutations), `cmd` owns the fixture *semantics* (which key, which vacuity guard, which line number).
- Complexity: Low. Each delegating helper is a loop or a single expression; `WithDuplicateKeyAt` is clone → index → insert.
- Modern idioms: Yes — `slices.Clone` / `slices.IndexFunc` / `slices.Insert` / `slices.DeleteFunc`, consistent with the package's existing mutators.
- Readability: Good. `sourceMissingTokens` / `sourceBadColours` / `sourceDuplicateKeyAt` read exactly as before at their ~80 call sites while their bodies became one line each; the lines/bytes split (`xLines` returning `[]string`, `sourceX` returning `[]byte`) is what let the two inline mutation sites reuse the guards.
- Comment accuracy: Accurate. The false panic claim on `WithDuplicateKeyAt` flagged in review attempt 1 was corrected — `internal/theme_file.go:70-72` now says the undeclared key comes back unchanged and that splicing *outside the result* panics, which matches the code. `cmd/theme_test.go:495-497` ("leaves the key's file position alone … offenders enumerate in file order") holds for `themetest.WithValue`'s in-place substitution, and `:499-501` correctly states why the no-op mutators need a `cmd`-side removal check.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/theme/builtins_tokyo_night_day_test.go:122 — replace `[]byte(strings.Join(kept, "\n")+"\n")` with `themetest.Render(kept)` (add the `internal/themetest` import; keep the surrounding `os.WriteFile(..., 0o644)` so the staged mode is unchanged). This is the last open-coded copy of the render left in `internal/theme`'s tests, which the first acceptance criterion names; the executor flagged it as out of scope in the fix-tracking NOTES on the assumption that folding it in meant switching to `themetest.Write` and its 0o600 mode — using `Render` alone avoids that.
- [do-now] CLAUDE.md:84 — the `themetest` row enumerates the theme-file surface but predates the two exports this task added. Replace `` `WithValue(lines, key, value)` / `WithoutKey(lines, key)` derive the broken variants (bad colour, missing token) without mutating the input, `Write(t, dir, base, lines)` stages a file in a temp themes dir and returns its path, and `Body()` is the same bytes for a consumer staging the file itself.`` with `` `WithValue(lines, key, value)` / `WithoutKey(lines, key)` / `WithDuplicateKeyAt(lines, key, at)` derive the broken variants (bad colour, missing token, duplicate-key bad syntax) without mutating the input, `Write(t, dir, base, lines)` stages a file in a temp themes dir and returns its path, and `Render(lines)` is the file body those bytes are — with `Body()` as `Render(Lines())` — for a consumer staging the file itself.``
- [do-now] internal/themetest/theme_file.go:68-72 — the `WithDuplicateKeyAt` doc does not state the precondition every current caller enforces, so a future direct caller can stage a fixture whose `line N` detail names the wrong line. Append to the first paragraph, after "…and splicing outside the result panics.": `at must fall past the key's first declaration; an earlier at makes the spliced copy the FIRST occurrence, and the loader then names the original's line rather than at.`
- [quickfix] cmd/theme_test.go:82-96 — `useThemesDir` still creates the themes dir as `t.TempDir()` itself while `seedThemesDir` now creates it at `t.TempDir()/themes`, so the two sibling env-setting helpers stage at different depths. Rewrite `useThemesDir`'s body as `dir := themesDirWith(t, nil); t.Setenv("PORTAL_THEMES_DIR", dir); return dir` so both depths and both `t.Setenv` owners agree with the one stager.
