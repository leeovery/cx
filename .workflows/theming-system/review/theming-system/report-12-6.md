TASK: theming-system-12-6 — Extract The Two Repeated Theme-Panel Model-Construction Sequences (Phase 12, Analysis Cycle 2; severity medium, source: duplication)

ACCEPTANCE CRITERIA:
1. The `Build` → dims → seed → size → assert-region → press → assert-open sequence appears in exactly one function in `package tui`.
2. The `ResolveSetting` → `ResolveNomination` → Fatal → `New(...)` sequence appears in exactly one function.
3. Every former constructor is reduced to its Deps, its call, and only the assertions unique to it.
4. No test expectation, dimension or session-seed value changes.

STATUS: complete

SPEC CONTEXT: Phase 12 is an analysis-remediation cycle, not a spec requirement — the task is a test-side duplication extraction with no behavioural surface. The fixtures it touches exercise spec §9.8 (panel geometry: preferred/stepped width, content-region-derived list height) and §8.8 (the light/dark gate resolving exactly once, which is why the dir-backed fixtures pin the answer via `WithCanvasMode` rather than detecting it). Both are preserved verbatim by the extraction.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - Shared helpers: `internal/tui/theme_testing_test.go:19` (`openPanelForTest`), `:24` (`openPanelForTestWithSessions`), `:48` (`newDirBackedPanelModel`), `:55` (`newDirBackedPanelModelOver`).
  - Reduced open-sequence call sites: `internal/tui/theme_panel_geometry_test.go:304-308` (`newGeometryPanelModel`), `internal/tui/theme_panel_chrome_test.go:20-35` (`newChromePanelModel`), `internal/tui/theme_panel_arrow_test.go:104-110` (`newArrowPanelModelAt`), `internal/tui/theme_panel_behaviour_test.go:61-74` (`behaviourPanelAt`), `internal/tui/theme_panel_commit_test.go:25-38` (`openCommitPanel`).
  - Reduced resolution call sites: `internal/tui/theme_panel_cursor_test.go:12-16` (`themeCursorModel`), `internal/tui/theme_panel_close_test.go:19-25` (`newClosePanelModel`).
  - Commit: `0bb1a639` (10 files, +106/−122).
- Criterion 1 — met. The full ritual (dims via `geometryTerm` → seed → `applySessionListSize` + `applyProjectListSize` → both content-region Fatal guards → `pressThemeKey` → open Fatal) exists only in `openPanelForTestWithSessions`; `openPanelForTest` is a one-line defaulted wrapper over it, which is the idiomatic Go rendering of the task's "pass session names as a parameter rather than fork the helper". `Build` itself is deliberately outside the helper (Do-step 1 specifies `m Model` in, so each site keeps its own Deps).
- Criterion 2 — met as stated. The `New(...)`-shaped sequence now exists only at `theme_testing_test.go:58-69` (verified: `WithThemeNomination(resolution.Nomination)` has exactly one occurrence in the package). The related resolve-then-Fatal *step* does still repeat in three `Build(Deps{…})`-shaped sites (`theme_testing_test.go:93-97`, `theme_panel_commit_load_test.go:60-64`, `theme_panel_confirm_test.go:735-739`) — but two of those predate the task and were never in its named scope (it named `themeCursorModel`, `newClosePanelModel` and `behaviourNomination`), and the third was introduced by a *later* task in this plan (14-5). Recorded as a non-blocking note, not drift.
- Criterion 3 — met. Each former constructor is now Deps + call + its own unique assertion: chrome keeps the `themePanelPreferredWidth` Fatal, arrow keeps `requireCursorOn`, commit keeps the projects seed / page / cursor slug, behaviour keeps its persister return.
- Criterion 4 — met. `closePanelSessions()` (`theme_panel_close_test.go:33`) is byte-identical to the alpha/bravo/charlie literal the geometry and arrow constructors inlined, so no seed changed. Dimensions round-trip exactly: sites pass `arrowTermW-2*Hinset, arrowTermH-2*Vinset` (100−4, 28−2 = 96, 26) and `geometryTerm` re-inflates to the original `arrowTermW, arrowTermH`; the geometry/chrome sites already spoke in content units. No assertion text or value was edited.
- Notes:
  - Two sites gained setup they did not previously run: `newArrowPanelModelAt` now sizes both lists and asserts the content region (it previously sized neither); `behaviourPanelAt` now also sizes the project list and asserts the region. This is prescribed by the task's own Do-step 1 (one helper that sizes both lists and asserts the region), and both additions are strictly stronger preconditions on the fixture, not changes to what any test asserts. The arrow file's comment was correspondingly amended at the time ("Nothing here sizes the PANEL's list") — that comment has since been removed wholesale by the topic's later comment-standard sweeps (`e3fa1503`, `915e7fcb`), so nothing stale survives.
  - `openCommitPanel` reordered its setup (projects/page first, then the helper's dims/seed/size/press). Safe: `applySessions` (`model.go:1089`) touches only `m.sessions`, the multi-select prune and `rebuildSessionList` — it neither resets `activePage` nor perturbs `projectList` selection.
  - Do-step 4 was honoured: `behaviourNomination` (`theme_panel_behaviour_test.go:76`) was left un-shared and the commit added the one-line reason (it resolves against a declared enumeration, not a loader over a directory, and narrows to the in-force slot). That note was later stripped by the topic's comment sweeps; the function's divergence is still self-evident from its `*behaviourEnumerator` parameter, so this is not a stale-comment defect.

TESTS:
- Status: Adequate
- Coverage: This is a pure test-side move, so the correct coverage is the existing panel suite exercising the helpers — `openPanelForTest`/`openPanelForTestWithSessions` are on the path of the geometry, chrome, arrow, behaviour and commit fixtures, and `newDirBackedPanelModel(Over)` on the cursor and close ones. A regression in either helper fails those suites loudly rather than silently.
- The task's micro-acceptance ("the shared helpers Fatal, not skip or silently continue") is satisfied structurally: `theme_testing_test.go:33`, `:36`, `:41` and `:61` are all `t.Fatalf`/`t.Fatal`, and every helper calls `t.Helper()` so a failure points at the caller. The "invert one precondition locally" step is a transient manual check by construction — correctly, nothing was persisted for it.
- The two content-region guards are not tautological despite `geometryTerm` being their inverse: `termDims` substitutes fallback dims for a non-positive value and `insetRegion` (`model.go:2760`) returns the dimension unchanged when `dim <= 2*inset`, so a degenerate region is genuinely caught.
- Not over-tested: no new test was added, and none is warranted — a test-helper extraction verified by an unchanged suite is the right shape.
- Notes: I assessed adequacy by reading; the suite was not executed (out of scope for this review).

CODE QUALITY:
- Project conventions: Followed. Helpers live in the package's shared theme test-helper file, take `*testing.T` first, call `t.Helper()`, use no `t.Parallel()` (per the package-level-mock rule in CLAUDE.md), and follow the file's existing `…ForTest` / `new…Model` naming.
- SOLID principles: Good. Each helper does one thing; the `…WithSessions` / `…Over` pairs parameterise the single axis that genuinely varies (session set, panel loader) instead of forking.
- Complexity: Low. Both helpers are straight-line; call sites shrank from ~15 lines to 1-2.
- Modern idioms: Yes.
- Readability: Good, with one wrinkle — call sites state the region as `arrowTermW-2*Hinset, arrowTermH-2*Vinset` and the helper immediately re-inflates it through `geometryTerm`, so the reader has to run the arithmetic twice to see the dims are unchanged.
- Comment accuracy: Verified. `theme_testing_test.go:17-18` (dims assigned directly, not via `tea.WindowSizeMsg`) matches `:27`; `:46-47` (light/dark pinned, not detected) matches `WithCanvasMode(mode)` at `:68`; `:53-54` (construction resolves through a loader of its own) matches the separate `theme.NewSilentLoader()` at `:59` against the caller-supplied `panelLoader` at `:63`. No process-artifact references remain.
- Security / performance: N/A (test-only, no I/O added).
- Issues: none.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_testing_test.go:111-116 — `requireCommitDoesNoOtherIO` re-inlines the open ritual (dims → `applySessions(closePanelSessions())` → `pressThemeKey` → open Fatal) in the very file that owns the extracted helper. Replace those six lines with `m = openPanelForTest(t, m, arrowTermW-2*Hinset, arrowTermH-2*Vinset)`; the `stores.reset()` on the following line absorbs the helper's extra list-sizing calls, so the counters it asserts are unaffected.
- [quickfix] internal/tui/theme_testing_test.go:58-62, internal/tui/theme_testing_test.go:93-97, internal/tui/theme_panel_commit_load_test.go:60-64, internal/tui/theme_panel_confirm_test.go:735-739 — the `ResolveSetting` → `NewSilentLoader().ResolveNomination` → Fatal step is written out four times. Extract `constructionNomination(t *testing.T, keys theme.RawKeys, dir string) theme.Nomination` into the shared helper file and call it from all four (the three `Build(Deps{…})` sites keep their own Deps).
- [quickfix] internal/tui/theme_panel_arrow_test.go:107, internal/tui/theme_panel_commit_test.go:35 — replace the inline `arrowTermW-2*Hinset, arrowTermH-2*Vinset` arithmetic with `arrowContentW`/`arrowContentH` constants declared beside `arrowTermW`/`arrowTermH` in `theme_panel_arrow_test.go:25-26`, so the call sites read in the same units the helper takes.
- [quickfix] internal/tui/theme_testing_test.go:58,66 — `theme.ResolveSetting` returns the stripped keys as its second value and the helper discards them, handing the unstripped `keys` to `WithThemeKeys`; production wires the stripped ones (`cmd/open.go:501` and its `raw`). Change to `setting, raw := theme.ResolveSetting(keys)` and pass `WithThemeKeys(raw)` so the single-sourced fixture mirrors production. (Pre-existing behaviour carried through the extraction, not introduced by it.)
