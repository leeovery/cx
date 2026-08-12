TASK: theming-system-13-14 — Add `internal/themetest` To CLAUDE.md's Internal-Packages Inventory (tick-c72c72, severity low, source: standards)

ACCEPTANCE CRITERIA:
1. CLAUDE.md's internal-packages table has a `themetest` row whose stated exports match the package's actual public API exactly.
2. The production-must-not-import rule is written down for `themetest` as it is for every other test-only helper.
3. The row names the package's consumers.
4. No other CLAUDE.md content changes.

STATUS: complete

SPEC CONTEXT:
Spec §12.6 (`specification.md:1559-1571`) makes CLAUDE.md this feature's own responsibility — "CLAUDE.md is what an implementing agent reads first" — and enumerates seven pre-feature entries the feature must correct. `themetest` is not one of the seven (the package is new), so this task is a standards-driven addition on top of §12.6's table: every other test-only helper package in the inventory (`portaltest`, `logtest`, `spawntest`, `restoretest`/`tmuxtest`/`portalbintest`/`sourceguardtest`/`transienttest`) carries the "production code must not import" rule in writing, and without a row the new package's test-only contract rested only on the `*testing.T`-first signature convention.

IMPLEMENTATION:
- Status: Implemented (as authored); the stated export list has since drifted behind the package — see NON-BLOCKING NOTES.
- Location: `/Users/leeovery/Code/portal/CLAUDE.md:84` (the `themetest` row); commit `b95fd990`.
- Verification against each criterion:
  1. **Exports at delivery — exact match.** `git show b95fd990:internal/themetest/{builtin.go,theme_file.go}` exports exactly `Builtin`, `DefaultDark`, `DefaultLight`, `Lines`, `Body`, `WithValue`, `WithoutKey`, `Write` — the eight the row names, no more and no fewer. Criterion 1 held at the commit.
  2. **Rule written down — yes, verbatim sibling wording.** The row opens "Test-only — production code must not import." — byte-identical to the `logtest` and `spawntest` rows' opening.
  3. **Consumers named — yes, and more accurately than the task text asked.** The row names `cmd`, `cmd/capturetool`, `internal/theme`, `internal/tui`, `internal/capture`; the task's Do step 5 asked for four of those, and `internal/theme` was a genuine fifth consumer at the time.
  4. **Nothing else changed.** `git show b95fd990 -- CLAUDE.md` is a single `+` line at table position 84; the only other files in the commit are `.tick/tasks.jsonl` and the workflow manifest (bookkeeping).
- Placement: sits between the `spawntest` row and the grouped `restoretest / tmuxtest / portalbintest / sourceguardtest / transienttest` row — i.e. with the other test-only helpers, as Do step 2 required.
- Content fidelity of the claims it does make (re-read against the current source): `Builtin` does report "not found in the embedded set" and "was rejected" as two distinct `t.Fatalf`s (`internal/themetest/builtin.go:16-22`); `Lines()` does emit one `key = value` line per `theme.TokenNames()` entry in order (`theme_file.go:28-35`); `WithValue`/`WithoutKey` do `slices.Clone` before mutating, so "without mutating the input" is true (`theme_file.go:50-66`); `Write` returns the joined path (`theme_file.go:110-118`); `Body()` is literally `Render(Lines())`, so "the same bytes" is true (`theme_file.go:39-41`). No claim in the row is falsified by the code it describes.
- Post-delivery drift (caused by *later* plan tasks, not by this one): `14-1` (`f0f1d37b`) added `SyntheticPalette`/`SyntheticPair`, `14-8` (`dab38cab`) added `Render`/`WithDuplicateKeyAt`/`MonochromeLines`, `17-5` (`3d3081cf`) added `LinesWithCanvas`/`WriteWithCanvas`/`DenyRead`/`DenyDir` and made `internal/prefs` a consumer. None updated the row. Measured at HEAD the row is a strict subset of the API (8 of 17 exports) and its "Two halves" framing now describes four roles. This is documentation staleness introduced downstream of the task, not a defect in the task's own delivery — recorded as a `[do-now]` with replacement text rather than a blocking issue.

TESTS:
- Status: Adequate (documentation-only task; the task's own "Tests" section prescribes read-backs, not new test code).
- Coverage:
  - *Export read-back*: performed against `internal/themetest/*.go` — see above; exact at the commit, subset at HEAD.
  - *No production importer*: verified. Every file importing `github.com/leeovery/portal/internal/themetest` is a `_test.go` file — 54 files across `cmd`, `cmd/capturetool`, `internal/capture`, `internal/prefs`, `internal/theme`, `internal/tui` and the package's own tests. Zero non-test importers.
  - *Suite unchanged*: the commit touches no `.go` file, so `go test ./...` is unaffected by construction (not executed here — reading only).
- Notes: No test is warranted for a CLAUDE.md row, and none should be added. Worth noting the contract has no mechanical guard — the repo has ~20 source guards driven by `sourceguardtest`, but none asserts "no non-`_test.go` file imports a `*test` helper package"; that gap predates this task and is out of its scope.

CODE QUALITY:
- Project conventions: Followed. The row matches the inventory's house style — package name in backticks, single-cell prose, exported identifiers in backticks with their signatures (`Builtin(t, slug)`, `Write(t, dir, base, lines)`), and the sibling rows' exact test-only sentence. The two-halves framing mirrors how the grouped `restoretest / …` row separates its members' responsibilities.
- SOLID principles: N/A (no code).
- Complexity: Low — one added table line, no restructuring.
- Modern idioms: N/A.
- Readability: Good. The row explains *why* each helper exists rather than restating its signature (e.g. `Builtin`'s two failure classes are called out as deliberately separate, which is the non-obvious part of `builtin.go`), which is the level the rest of the inventory writes at.
- Issues: None in the delivered line. The row's "Two halves." sentence is the only clause that HEAD falsifies, and only because later tasks added two more roles.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `/Users/leeovery/Code/portal/CLAUDE.md:84` — the `themetest` row now names 8 of the package's 17 exports and says "Two halves" where there are four roles; `internal/prefs` is also an unnamed consumer. Replace the row body with: "Test-only — production code must not import. Four roles. The **built-in accessors** hand back a parsed `theme.Theme` for a slug — `Builtin(t, slug)` (fatal, with \"no such built-in\" and \"the shipped file is broken\" reported as separate failures) plus `DefaultDark(t)` / `DefaultLight(t)` for the shipped pair. The **theme-file helpers** author `.theme` fixtures against the loader's format: `Lines()` is a complete valid file (one `key = value` line per token, in `theme.TokenNames()` order), `MonochromeLines(value)` / `LinesWithCanvas(canvas)` are the whole-palette and canvas-only variants, `WithValue` / `WithoutKey` / `WithDuplicateKeyAt` derive the broken variants (bad colour, missing token, duplicate key) without mutating the input, `Write(t, dir, base, lines)` / `WriteWithCanvas(t, dir, base, canvas)` stage a file in a temp themes dir and return its path, and `Body()` / `Render(lines)` are the same bytes for a consumer staging the file itself. The **synthetic probe palettes** `SyntheticPalette(t, red)` / `SyntheticPair(t, redA, redB)` build the value-disjoint before/after the swap-and-diff guard diffs. The **denial fixtures** `DenyRead(t, path)` / `DenyDir(t, dir)` stage an existing file or directory at mode 0000 for the rest of the test, returning the OS error and restoring the prior mode on cleanup (skipping where the mode bits deny nothing). Consumed from `cmd`, `cmd/capturetool`, `internal/capture`, `internal/prefs`, `internal/theme` and `internal/tui` test files".
- [do-now] `/Users/leeovery/Code/portal/internal/themetest/theme_file.go:1-7` — the package doc comment lists three responsibilities (fixture files, built-ins by slug, synthetic probe palettes) and omits the denial fixtures added in `deny.go`. Extend the first sentence to "…, builds the synthetic probe palettes a swap guard diffs between, and stages a path as unreadable."
- [quickfix] no file — no mechanical guard enforces the "production code must not import" contract for the `*test` helper packages (`themetest`, `logtest`, `spawntest`, `portaltest`, `restoretest`, `tmuxtest`, `spawntest`, `transienttest`, `sourceguardtest`); it rests on the `*testing.T`-first convention plus the newly-written prose, and `themetest`'s `Lines`/`Body`/`Render`/`WithValue`/`WithoutKey`/`WithDuplicateKeyAt`/`MonochromeLines`/`LinesWithCanvas` take no `*testing.T`, so nothing structurally stops a production import of them. Add a unit-lane source guard driven by `sourceguardtest.GoSourceFiles` asserting no non-`_test.go` file imports a package under `internal/` whose name ends in `test`. (Pre-existing repo-wide gap, surfaced by this task rather than caused by it.)
