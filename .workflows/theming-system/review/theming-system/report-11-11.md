TASK: theming-system-11-11 (tick-3ccb13) — De-Duplicate cmd's Invalid-Theme Fixture Builders

ACCEPTANCE CRITERIA:
- Package `cmd` declares one builder per rejection class (missing tokens / bad colours / duplicate key), each parameterised.
- The three hardcoded builders are deleted.
- Doctor and export tests assert the same rejection reasons on the same fixture shapes as before.

STATUS: complete

SPEC CONTEXT:
The fixtures under review exercise §6.2's fixed-order rejection ladder (`bad name` → `reserved name` → `unreadable` → `bad syntax` → `bad colour` → `missing tokens`, short-circuiting at the first failure; spec lines 449-452) and §4.2's lexical rules (spec line 264: a duplicated key is `bad syntax`, "the duplicate check is lexical and runs before any key is classified or compared"). Spec line 1862 gives the exact detail shape a duplicate must render — `line 12: duplicate key text.primary`, naming the SECOND occurrence's line. The consolidated `sourceDuplicateKeyAt(t, 12, "text.primary")` call sites now reproduce that worked example literally, which is a small correctness bonus over the superseded `duplicateKeySource` (which appended the first key at the end of the file, an arbitrary line number).

IMPLEMENTATION:
- Status: Implemented (commit 3febba39; later refined by phase-17 tasks — see Notes)
- Location:
  - `cmd/theme_test.go:497` (`themeOverride`), `:502` (`missingTokenLines`), `:517` (`badColourLines`), `:531` (`duplicateKeyLines`), `:549` (`sourceMissingTokens`), `:555` (`sourceBadColours`), `:561` (`sourceDuplicateKeyAt`), `:482` (`themeLineIndex`) — one builder per rejection class, all parameterised, all sited beside the line-mutating primitives they compose.
  - Rewritten export call sites: `cmd/theme_test.go:337`, `:631`, `:670`, `:675`, `:680`, `:906`.
  - Doctor call sites untouched by the move: `cmd/doctor_theme_test.go:79/86/93/…`, `cmd/doctor_theme_union_test.go`, `cmd/doctor_fix_theme_test.go`.
- Notes:
  - The hardcoded trio is gone: a repo-wide grep for `missingTokenSource` / `badColourSource` / `duplicateKeySource` returns nothing.
  - The duplicated helpers that rode along with the trio are also single-sourced: `themeOverride` and `themeLineIndex` were declared in BOTH files before the change and are now declared once, in `cmd/theme_test.go`. Both `slices` and `strings` remain in use in `cmd/doctor_theme_test.go` after the removals (lines 259/64 etc.), so no dangling import was left behind.
  - Later supersession (not drift): phase-17 work replaced the in-file `themeSourceFromLines` and the hand-rolled slice mutation with the shared `themetest.Render` / `WithoutKey` / `WithValue` / `WithDuplicateKeyAt` primitives, and split each builder into a `…Lines` verifier + a `source…` byte wrapper. The task's outcome ("one builder per rejection class in `cmd`, parameterised, beside the primitives it composes") holds in the current tree with that stronger factoring.
  - Fixture validity is sound after the parameterisation swap: `themeKeyLines` derives from `theme.DefaultDarkSlug`'s embedded file, whose key lines run `text.primary` (index 0) … `text.on-attention` (index 18), so `sourceDuplicateKeyAt(t, 12, "text.primary")` satisfies the builder's own `at >= first+2 && at <= len(lines)+1` guard, and `canvas` (index 13) exists for the `bad colour` overrides.

TESTS:
- Status: Adequate
- Coverage: This is a test-helper de-duplication, so the verification is that the consuming suites' assertions are unchanged. They are:
  - `cmd/theme_test.go:662` `TestThemeExport_InvalidDropInFrame` still pins the same three refusal strings (`… is not valid: bad syntax` / `bad colour` / `missing tokens`).
  - `cmd/doctor_theme_test.go:69` `TestThemeAdvisories_InvalidFileFrame` still pins the full advisory details, including `line 12: duplicate key text.primary` and the file-order offender list `text.primary = #GGGGGG, canvas = blue` — the assertions most sensitive to a fixture-shape regression, and untouched by the commit.
  - The builders are self-verifying rather than trusting the built-in: `missingTokenLines` fails if the key was never declared, `badColourLines` fails if the substitution did not land, and `duplicateKeyLines` re-reads the ASSEMBLED source to prove both the first-occurrence index and the duplicate's line, so a pinned `line N` detail is a fact about the fixture rather than a coincidence. The underlying mutators additionally carry their own rejection-class tests in `internal/themetest/theme_file_test.go:67/75/90`.
- Notes: The export-side fixtures changed parameterisation (bad colour now breaks `canvas` rather than the first key; the duplicate now lands on line 12 rather than the file's end). Rejection CLASS and every assertion are identical, and the new parameters match both the doctor-side fixtures and the spec's worked example — so the AC's "same fixture shapes" is met at the level it is asserted at. Not over-tested: no new test was added for a pure de-duplication, which is correct.

CODE QUALITY:
- Project conventions: Followed. Test-only helpers stay in `_test.go` files in `cmd`; shared file-authoring primitives live in `internal/themetest` (test-only package, not imported by production code); no `t.Parallel()`; `t.Helper()` on every helper.
- SOLID principles: Good. Each builder now has one reason to change; the parameterised trio composes the line mutators rather than restating slice surgery.
- Complexity: Low. Each `source…` builder is a one-line composition over a verified `…Lines` mutator.
- Modern idioms: Yes — `slices.Clone`/`Contains`, `strings.Cut`, variadic parameterisation.
- Readability: Good. Comments explain WHY each fixture is constructed the way it is (why `themeOverride` preserves file position, why `duplicateKeyLines` re-verifies the assembled source) rather than restating the code, and no comment references a task id or phase. Comment claims check out against the code: `badColourLines`'s "the replacements still lex as well-formed pairs" holds (`key = value` substitution in place), and `missingTokenLines`'s "the shared mutators leave an undeclared key alone rather than failing" matches `themetest.WithoutKey`'s behaviour, which is exactly why the length check is there.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [idea] `cmd/doctor_theme_enumeration_test.go:14` — a later task (17-7) added a fourth missing-token fixture in `cmd` built ad hoc from `themetest.WithoutKey(themetest.Lines(), "bg.subtle")` instead of the shared `sourceMissingTokens`, so `cmd` now has two origins for invalid-theme fixtures (built-in-derived vs themetest-authored). Decide whether that test should route through the shared builder (it asserts no rejection reason and needs `themetest.Write`'s lines-shaped input, so the ad-hoc form is defensible) or whether the single-builder-per-class invariant this task established should be restated to cover it.
