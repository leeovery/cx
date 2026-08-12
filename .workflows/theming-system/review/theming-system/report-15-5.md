TASK: theming-system-15-5 — Single-Source The Two-Built-In Subtest Table Across internal/tui's Render Suite (tick-7f13a0)

ACCEPTANCE CRITERIA:
1. Two iterators exist in one place; no `internal/tui` test declares its own `{"dark", …}, {"light", …}` two-built-in table.
2. Subtest names and the set of tests run are identical to today's.
3. Every assertion and failure message is unchanged.
4. The pair is read from `internal/theme`'s shipped-default slugs, not restated.
5. `go test ./internal/tui` passes.

STATUS: complete

SPEC CONTEXT:
This is an analysis-remediation task (Phase 15, cycle 5), not a spec-behaviour task — nothing in the specification governs test scaffolding shape. The relevant standing conventions are CLAUDE.md's: `internal/themetest` is the test-only accessor for parsed built-in palettes (`DefaultDark`/`DefaultLight` → `theme.DefaultDarkSlug`/`DefaultLightSlug`), `internal/theme`'s own `contrast_test.go` already auto-enumerates the embedded set for the same "one declaration of which themes we run against" reason, and the repo's ~20 unit-lane source guards share their Go-source scanning primitives (today `internal/sourceguardtest`, `internal/portalbintest` at the time of this commit) rather than hand-rolling walks.

IMPLEMENTATION:
- Status: Implemented (commit bb4294ba, 17 test files rewritten + 1 new guard file; comments later stripped by the package-wide pass e3fa1503, which is the project's own standard, not drift)
- Location:
  - internal/tui/theme_testing_test.go:315-350 — `builtinThemeCase` / `builtinThemeCases(t)` / `forEachBuiltinTheme(t, fn)` / `forEachCanvasMode(t, fn)`, sited beside the existing `testDarkTheme`/`testLightTheme`.
  - internal/tui/builtin_theme_table_test.go:14-83 — the two iterator-contract tests plus `TestNoLocalTwoBuiltinTable`, the drift guard.
  - Palette sites rewritten as closures: footer_test.go:55, header_test.go:15 and :163, help_modal_frame_test.go:17, help_modal_test.go:288, multi_select_banner_test.go:17, pagination_dots_test.go:47, panel_test.go:12, projects_footer_test.go:55, projects_header_test.go:15, section_header_test.go:18 and :203, unsupported_banner_test.go:19, theme_panel_open_test.go:405 (a 15th site beyond the 14 the task enumerated — same table, correctly folded in).
  - modal_footer_test.go:176 takes `builtinThemeCases(t)` rather than the closure form, because its loop runs *inside* per-call-site subtests and has no per-mode `t.Run` to preserve. Correct call: converting it to `forEachBuiltinTheme` would have added subtest levels and violated criterion 2.
  - Mode sites rewritten: canvas_cell_background_test.go:108, canvas_paint_test.go:41, help_modal_test.go:322, modal_blank_screen_test.go:79.
- Notes:
  - Criterion 4 holds transitively and genuinely: `testDarkTheme` → `themetest.DefaultDark` → `Builtin(t, theme.DefaultDarkSlug)`; no slug is restated in `internal/tui`.
  - Criterion 1 holds and is now structurally enforced rather than trusted: `TestNoLocalTwoBuiltinTable` scans every `_test.go` in the package (exempting only the declarer) for the two-field row shape, and the trailing-brace anchor in the regex correctly spares wider tables (`command_pending_band_test.go:302`, `destructive_confirm_test.go:34/56/87/117` carry colourless + golden columns and are legitimately out of scope per Do #6).
  - Criterion 3: I diffed every removed line against every added line under whitespace/`tc.th`→`th` normalisation. The only non-boilerplate residue is canvas_cell_background_test.go:125,134, where the `mode %s` argument moved from the table's `tc.name` to `themeLabel(th)`. `themeLabel` returns exactly `"dark"`/`"light"` for the two built-in canvases, so the rendered failure text is identical; it is evaluated lazily inside the `t.Fatalf`/`t.Errorf` argument list, so the extra built-in load only happens on failure. No assertion, golden, token check or message string changed anywhere else.
  - Criterion 2: subtest names are preserved by construction (the iterators run `t.Run("dark"/"light")` themselves) and are additionally pinned by assertion — `TestForEachBuiltinTheme_RunsTheShippedPair` reads them back out of `t.Name()` rather than trusting them, which is the right call given every call site inherits them and `-run` filters target them.

TESTS:
- Status: Adequate
- Coverage: Three new tests, each earning its place. `TestForEachBuiltinTheme_RunsTheShippedPair` pins both halves of the contract the 15 call sites inherit (subtest names, and that the palettes are the shipped defaults — compared against `themetest.DefaultDark/Light`, so it fails if the iterator drifts off the shipped pair). `TestForEachCanvasMode_RunsBothModes` does the same for the 4 mode sites. `TestNoLocalTwoBuiltinTable` is the durability guard that stops the duplication re-accreting — without it the consolidation would decay silently, which is exactly the failure mode the task exists to fix.
- Notes:
  - Not over-tested: the two iterator tests look like tests of two-line functions, but what they actually pin is the inherited subtest naming and the pair's provenance — both cross-file contracts, not implementation detail.
  - Not under-tested for the task's own risk surface. The task's suggested verification steps (compare `go test -v` names before/after; break one palette and confirm the `light` subtest fails; point the iterator at one theme and watch the count drop) are one-shot manual checks; the committed equivalents (name read-back, palette identity comparison) cover the durable half.
  - Judged by reading only — no suite execution, per my remit. Nothing in the rewrite can change pass/fail: every assertion is byte-identical, every closure body is the former subtest body verbatim, and no site lost or gained a case.
  - The guard's own coverage has one blind spot (line-based matching, see notes below) — it catches the shape that existed, not every shape that could.

CODE QUALITY:
- Project conventions: Followed, with one deviation (the guard hand-rolls the package-file walk instead of using the shared enumerator that already existed one phase earlier — note 1).
- SOLID principles: Good. The iterator owns enumeration; the body owns assertion; `builtinThemeCases` is exposed separately for the one site that needs the cases without the subtest wrapper, rather than contorting that site to fit the closure form.
- Complexity: Low. Both iterators are a loop and a `t.Run`; the conversions strictly removed a nesting level (~120 lines of table boilerplate gone, net -494/+512 including the new guard).
- Modern idioms: Yes. Closure-taking `forEach*` helpers with `t.Helper()`, subtest-scoped `t` shadowing the outer one (so a body cannot accidentally report against the parent), `strings.Cut` for the name read-back.
- Readability: Good. The helpers sit beside `testDarkTheme`/`testLightTheme` where a reader looking for "how do I run against both built-ins" will already be. Naming (`forEachBuiltinTheme` / `forEachCanvasMode`) states the distinction — palette vs the light/dark answer — that the two tables actually encoded.
- Comment accuracy: Current files carry no comments on this code; the explanatory comments the task shipped were removed wholesale by the later `chore(comments): strip internal/tui to the code-quality standard` pass, which stripped every sibling guard (`retired_token_guard_test.go`, `colour_literal_guard_test.go`) to zero comments too. That is the project's standard applied uniformly, not this task's omission, so I am not raising it — with one consequence recorded in note 3: the regex's trailing-brace subtlety is now load-bearing and unexplained.
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/builtin_theme_table_test.go:62-75 — replace the hand-rolled `os.ReadDir(".")` + `IsDir()`/`.go` filtering with the shared per-package enumerator `sourceguardtest.PackageGoFiles(".", true)` (keeping the `_test.go` and `twoBuiltinTableDeclarer` filters), dropping the now-unused `os` import. The primitive already existed when this landed (as `portalbintest.PackageGoFiles`, single-sourced by phase 14's tick-5ab18d) and three sibling guards in this same package already used it — this guard is the one that re-hand-rolled it, and because it landed after the sweep, no later phase caught it. Beyond convention, `PackageGoFiles` errors on an empty match, so the guard cannot pass by having stopped looking.
- [quickfix] internal/tui/burst_preflight_abort_test.go:103 — `for _, th := range []theme.Theme{testDarkTheme(t), testLightTheme(t)}` still spells out the built-in pair; it has no `t.Run`, so the guard's row regex does not see it. Rewrite as `for _, tc := range builtinThemeCases(t)` and use `tc.th` in the body — no subtest is added, so test names and the set of tests run are unchanged.
- [idea] internal/tui/builtin_theme_table_test.go:60 — the guard matches a single source line, so a row split across lines (`{\n\tname: "dark",\n\tth: testDarkTheme(t),\n}`) or written value-first reintroduces the table undetected. Decide whether to swap the regex for an AST walk of composite literals (the package already parses Go source in `retired_token_guard_test.go`), or to accept line-shape matching as sufficient for a discipline aid.
