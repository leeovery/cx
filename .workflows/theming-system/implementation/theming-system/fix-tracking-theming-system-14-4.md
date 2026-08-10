## Attempt 1

ISSUES:
- Two package scans created by THIS feature were not converted, so the acceptance criterion "no test declares its own `os.ReadDir(".")` or `filepath.Glob("*.go")` package scan" is unmet: `internal/capture/theme_panel_message_fixtures_test.go:510` (`packageSourceFiles` — a byte-equivalent `os.ReadDir(".")` copy of the bodies this task deleted, added by theming-system-9-12) and `internal/tui/retired_token_guard_test.go:95` (`filepath.Glob(filepath.Join(dir, "*.go"))`, added by theming-system-14-2, the immediately preceding task in this cycle). The executor's report classifies `internal/capture` as "pre-existing"; it is not, and `retired_token_guard_test.go` is not mentioned at all. Neither can pass vacuously today (each carries its own empty-match fatal), so this is completeness of the single-sourcing, not a live blind spot.
  FIX: Convert both. The reviewer prototyped and validated exactly this in a sandbox copy (`go vet` clean, `./internal/capture` and `./internal/tui` green, and `TestNoRetiredTokenNameInComments` still fires on a canary comment naming `accent.violet`):
  (a) `internal/capture/theme_panel_message_fixtures_test.go` — replace the `packageSourceFiles` body with `paths, err := portalbintest.PackageGoFiles(".", false)` / `t.Fatalf("enumerate the internal/capture package sources: %v", err)` / `return paths`, and add the `portalbintest` import. The returned paths are bare filenames for `dir == "."` (`filepath.Join` cleans), which is what the existing `os.ReadFile(path)` consumer already expects. The test file is `package capture_test`, so the no-real-config import guard (which scans production sources) is untouched.
  (b) `internal/tui/retired_token_guard_test.go` — replace the glob + empty-match block with `matches, err := portalbintest.PackageGoFiles(dir, true)` / `t.Fatalf("enumerate the %s package sources: %v", dir, err)`. `includeTests=true` matches what the glob covered (it never skipped `_test.go`), and the joined-path strings are byte-identical for both `"."` and `"../capture"`. This also gives the `includeTests=true` arm its only production caller.
  Leave the genuinely pre-existing non-theming scans alone — converting them is scope creep into other features' guards: `internal/log/init_test.go:238`, `internal/resolver/log_free_test.go:24`, `internal/fileutil/atomic_classify_test.go:141`, `cmd/state_daemon_lock_pid_ordering_test.go:156`, `internal/tui/pagepreview_hermetic_test.go:164`, `internal/tui/sessions_grouped_reskin_test.go:307`.
  ALTERNATIVE: accept the narrower reading (the Do list names neither file) and record the two as a follow-up. Tradeoff: it leaves a literal duplicate of the just-deleted body inside the same feature's own guard family, which is precisely the drift this task exists to end, and the conversion is ~10 lines with proven-safe behaviour. Converting is recommended.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/tui/colour_literal_guard_test.go:38` — the change deleted the glob this line names; `centralisedColourSites` is now a directory enumeration.
  OLD: `// internal/tui (via the centralisedColourSites glob) and fails if any`
  NEW: `// internal/tui (via centralisedColourSites) and fails if any`

NOTES:
- Guard firing independently re-verified with fresh canary production files, and the reviewer found MORE guards firing than the executor claimed: `internal/theme` canary → 5 guards; `internal/tui` canary → 6 (executor claimed 4); `cmd` canary → 2. Every failure named the canary file with a correct line number, so shared-FileSet positions still resolve.
- Anti-vacuity is real and unswallowed: repointing the enumeration at `t.TempDir()` in all four front ends made every guard fatal rather than pass. All five production call sites `t.Fatalf` on error; none discards it.
- The two parse-mode changes are genuinely neutral: no `.Obj`, `.Scope`, `ast.Object`, `ast.Scope` or `Unresolved` reference exists anywhere in `cmd`, `internal/tui` or `internal/theme`.
- `includeTests bool` is a boolean parameter, which code-quality.md lists as an anti-pattern — but the task's Do item 1 mandates that exact signature, so it is not a finding.
- `packageRelFiles` sorts before comparing, so the doc comment's "in `os.ReadDir`'s filename order" claim is true but unpinned. Not worth a change.
- The new test file name `packagegofiles_test.go` follows the package's own `gosourcefiles_test.go` precedent rather than the generic one-test-file-per-source rule. The local convention is right here.
