## Attempt 1

ISSUES:
- Four call sites still compose the canvas fixture inline, failing the criterion `themetest.WithValue(themetest.Lines(), "canvas", …)` appears at no call site outside `internal/themetest`: `/Users/leeovery/Code/portal/internal/theme/load_test.go:432`, `/Users/leeovery/Code/portal/internal/theme/load_test.go:56`, `/Users/leeovery/Code/portal/internal/theme/resolve_test.go:401`, `/Users/leeovery/Code/portal/internal/theme/resolution_test.go:458`. The report's justification ("they do not write a file, so `WriteWithCanvas` cannot express them") is factually wrong for `load_test.go:432`, which DOES write a file: `lines := themetest.WithValue(themetest.Lines(), "canvas", "blue")` / `return themetest.Write(t, dir, "nord-lee.theme", lines), "canvas = blue"`. Four instances of one identical expression is past the Rule of Three, and this is the exact duplication the task targets.
  FIX: Migrate `load_test.go:432` to the existing helper — `return themetest.WriteWithCanvas(t, dir, "nord-lee.theme", "blue"), "canvas = blue"`. For the three genuinely lines-level sites, add `func LinesWithCanvas(canvas string) []string { return WithValue(Lines(), "canvas", canvas) }` to `/Users/leeovery/Code/portal/internal/themetest/theme_file.go`, re-express `WriteWithCanvas` as `Write(t, dir, base, LinesWithCanvas(canvas))` so the two cannot drift, and route `resolve_test.go:401` (`lines: themetest.LinesWithCanvas("blue")`), `resolution_test.go:458` (`dirWith("nord-lee.theme", themetest.LinesWithCanvas("blue"))`) and `load_test.go:56` (`lines := themetest.LinesWithCanvas("blue")`, second `WithValue` chain unchanged) through it. Cover `LinesWithCanvas` in `theme_file_test.go` alongside the existing `WriteWithCanvas` case.
  ALTERNATIVE: Record the criterion as an accepted deviation and leave the three lines-level sites composing inline, migrating only `load_test.go:432`. Cheaper by one helper, but leaves a grep-checkable criterion permanently failing and three copies of the composition to drift. The reviewer recommends the fix — it is ~5 lines and completes the task's stated outcome.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `/Users/leeovery/Code/portal/internal/themetest/deny.go:14-17` — cardinality claim the code contradicts: 20 `os.Geteuid()` root policies survive elsewhere in the tree, including one in `internal/theme` itself (`resolve_test.go:508`), so "the whole tree's" and "no call site decides this again" are both false and are the class of claim ordinary additive change falsifies.
  OLD:
  ```
  // The whole tree's root policy lives here: where the mode bits deny nothing —
  // the process is root, or the filesystem ignores them — the fixture is
  // impossible and the test skips rather than asserting vacuously. No call site
  // decides this again.
  ```
  NEW:
  ```
  // Where the mode bits deny nothing — the process is root, or the filesystem
  // ignores them — the fixture is impossible, so the test skips rather than
  // asserting vacuously.
  ```
- `/Users/leeovery/Code/portal/internal/themetest/deny.go:29` — "the one root policy" refers to a uniqueness the corrected `DenyRead` doc no longer asserts; the point is that it is the same policy, not that it is the only one.
  OLD: `// contract — the returned denial, the restored mode, and the one root policy —`
  NEW: `// contract — the returned denial, the restored mode, and the same root policy —`
- `/Users/leeovery/Code/portal/cmd/theme_source_test.go:191-192` — restates the function's own doc comment two lines above it (`// The poisoned (mode 0000) directory makes any read loud: an unusable directory / // earns a `theme: directory unusable` WARN, where an absent one is silent.`).
  OLD:
  ```
  	// An existing but unreadable themes directory earns a `theme: directory
  	// unusable` WARN, where an absent one is silent.
  ```
  NEW: (delete)

NOTES:
- `/Users/leeovery/Code/portal/internal/theme/enumerate_test.go:50-52` re-reads the directory to confirm it is unreadable immediately after `themetest.DenyDir`, which already guarantees that (it skips otherwise). Harmless, but it is the "assert by re-reading the path" pattern the task's step 3 set out to retire. Same shape, milder, at `cmd/theme_test.go:768` and `internal/theme/resolve_test.go:288`, where `osReadError` reads a file under an already-denied directory — that one is genuinely needed (chmod cannot reach a file inside a mode-0000 dir).
- Two byte-identical `osReadError` helpers now exist, in `/Users/leeovery/Code/portal/cmd/theme_test.go:568` and `/Users/leeovery/Code/portal/internal/theme/resolve_test.go:615`. Both serve the read-inside-a-denied-directory case. Consolidating them into `themetest` (e.g. a `ReadError` companion to the deny pair) would finish the job, but it was outside the Do list and is not worth a fix round on its own.
- `internal/themetest`'s package doc (`/Users/leeovery/Code/portal/internal/themetest/theme_file.go:1-7`) enumerates three capabilities and describes the package as supporting "Portal's theme tests". It now also owns the tree's deny-read fixture and is consumed by `internal/prefs`'s read-policy tests. The doc line itself was not touched by this diff — worth folding into whichever change lands next in that file.
- Deliberate behaviour change, correctly executed and correctly reported: `cmd` sites that previously fatalled when a mode-0000 read succeeded now skip, matching the single prescribed policy. No site loses meaning, because `DenyRead`/`DenyDir` verify the denial before returning, so a chmod that did not take can no longer produce a vacuous pass.
- `deny`'s `os.IsNotExist` branch (`deny.go:52`) is effectively unreachable — `restoreMode` already `t.Fatal`s on a failed `os.Stat` of the same path, and a dangling symlink fails that stat first. Defensive and harmless; the `deny_test.go` not-exist assertion is correspondingly a contract pin rather than a live check.
- ARCHITECTURE tension (flagged, not a defect): the deny helpers are generic filesystem fixtures with no theme content, so hosting them in `themetest` means the shared root policy is only reachable by packages willing to import the theme fixture package. That is why `internal/state`, `internal/project` and `internal/spawn` keep their own guards and the "one root policy for the whole tree" outcome cannot be completed from here. The task directed this placement.
- Two acceptance criteria are unreachable as written and the reviewer accepted the executor's judgement on both: the `rg 'Chmod\(.*0o000' --glob '*_test.go'` criterion cannot return `internal/themetest` at all (`deny.go` is not a `_test.go` file), and "exactly one root policy exists in the tree" is false for non-theme fixtures (a mode-`0o111` search-only fixture at `internal/theme/resolve_test.go:508` that `DenyDir` cannot express).
