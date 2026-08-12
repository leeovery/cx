TASK: theming-system-4-2 — The Live Theme-Swap Entry Point And The Render-Swap-Render Harness Seam

ACCEPTANCE CRITERIA:
1. `Model.ApplyTheme` exists, sets the active theme and re-points every style through `applyCanvasMode`; the only mid-session swap path (no second setter).
2. A swap performs no `rebuildSessionList` and no lazy per-session pane read (counting `DirReader` records zero reads in every grouping mode).
3. A swap performs no file read (no loader call, no directory access).
4. `startupCanvasHex` unchanged after one swap and after fifty.
5. Swapping to the identical theme is a byte-identical no-op; repeated swaps idempotent per swap.
6. A colourless model stays colourless across a swap (no `38;2;`/`48;2;` in the post-swap frame).
7. `RenderSwapRender` constructs exactly one model; pre-swap state survives the swap.
8. The before-frame is non-empty, is not the pre-resolution blank frame, and carries theme A's canvas.
9. `ModelAt` drives every registered `tui.Build`-backed fixture to its captured state.
10. Both delegates and both lists' `bubbles/list`-owned styles re-point together on a swap.
11. `internal/capture` still performs no XDG lookup and no prefs read; the two import guards pass.

STATUS: complete

SPEC CONTEXT:
§13.4 (the swap-and-diff completeness guard) demands the swap be a *live mutation of one already-rendered model through the production swap path* — render under A, swap, render again — and explicitly names the vacuous-pass shape (two models, one per theme) that the fixture harness's one-shot-render design would otherwise produce. It requires `internal/capture` / `tui.Build` to expose a seam to drive that from a test, and excludes colourless fixtures. §11.1 fixes the swap as the `applyCanvasMode` restyle (O(1), no I/O, no list content touched) and forbids the `rebuildSessionList` path with its per-session tmux pane reads (~0.5s at ~38 sessions). §11.4 makes the retained startup canvas hex the anchor the exit-time OSC 11 canvas-echo guard rests on — it must not move on a swap. §13.3 keeps the harness free of config discovery and out of the shipped binary.

IMPLEMENTATION:
- Status: Implemented (with two later-phase supersessions, both intended)
- Location:
  - `internal/tui/model.go:893-900` — `ApplyTheme` (set `themeState.active`, then `applyCanvasMode`), with the three prohibitions documented on it.
  - `internal/tui/model.go:905-913` — `applyCanvasMode`, the shared re-point (filter input, preview palette, sessions list + delegate, projects list + delegate, and — added by Phase 8 — the theme panel's list).
  - `internal/capture/harness.go:19-43` — `ModelAt`: `tui.Build(f.Deps(th))` under the constant nomination, then a fixed `Update` order (WindowSize → SessionsMsg → ProjectsLoadedMsg → seeded loading events → seeded fatal → `captureKeys`), no tea program, no `Init`, `BootstrapCompleteMsg`/`LoadingMinElapsedMsg` deliberately never sent.
  - `internal/capture/harness.go:70-80` — `RenderSwapRender`: one `ModelAt`, `View().Content`, `ApplyTheme(b)` on that same model, `View().Content`.
  - `internal/capture/harness.go:82-87` — `Colourless()` reading `f.Deps(...).NoColor`.
  - `internal/capture/fixtures.go:61-63` — the `captureKeys` field; populated at `:221` (`sessions-empty`, `x`), `:680` (`projects`, `x`), `:717` (`preview-screen`, `Space`), and — by Phase 8 — `:445/:464/:550/:575/:635` (`t`, plus `x`+`t` and a Ctrl+↓ page for the panel fixtures).
- Notes:
  - AC1: `m.themeState.active` is written in exactly two places repo-wide (`model.go:861` in `syncResolvedMode`, the once-per-run gate resolution, and `model.go:898` in `ApplyTheme`). There is no second mid-session setter, and the field is unexported.
  - AC10 is superseded as intended: `applyCanvasMode` now also re-points the panel's third list (`theme_panel.go:304-309`), added when Phase 8 introduced the slide-over. The task's "no panel/third-list instance is referenced" clause was a statement about what existed at the time, not a prohibition; `restyle_repoint_test.go:242-280` covers all three surfaces.
  - AC11 holds: `harness.go` imports only `bubbletea`, `internal/theme`, `internal/tui`; the capture package touches `internal/prefs` for the `SessionListMode` enum only (no store read) and reaches `internal/xdg` only transitively through `internal/tui`. `TestPortalBinaryDoesNotImportCapture` / `TestCaptureToolDoesImportCapture` (`cmd/capturetool/import_guard_test.go`) are intact and the second one keeps the first non-vacuous.
  - The seam is load-bearing rather than decorative: §13.4's guard (`theme_swap_guard_test.go`), the panel render fixtures (`theme_panel_fixture_render_test.go:245`, `theme_panel_remaining_fixtures_test.go:44,256`) and the colourless exclusion (`excludeColourless`, `theme_swap_guard_test.go:91-93`) all consume it.
  - The three later-phase amendments to this task's code are all coherent: `Deps` takes the palette (so the faked `ThemeSource` and the nomination cannot disagree), `RenderSize`/`PinRenderSize` were added by 17-11, and comment wording was rewritten by the 11-3/comment-audit passes.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/tui/apply_theme_test.go` — every named micro-acceptance test is present and non-vacuous: `TestApplyTheme_RestylesWithoutRebuild` (zero `DirReader` reads across a swap *and* across the following render, in flat/by-project/by-tag, with a positive control that fires `rebuildSessionList` to prove the counter can move, and a deliberate cache re-arm at `:47-51` so "zero reads" cannot be true for the wrong reason); `TestApplyTheme_PerformsNoFileRead` (seven counting seams aggregated, with a positive control asserting all seven counters increment, plus a reflective guard that no `theme.Loader` is reachable from `Model`); `TestApplyTheme_DoesNotMoveStartupCanvasHex` (1 and 50 swaps, with a setup assertion pinning the pre-swap value); `TestApplyTheme_SameThemeIsANoOp`; `TestApplyTheme_RepeatedSwapsAreIdempotent`; `TestApplyTheme_ColourlessStaysColourless` (with a coloured positive control so the negative assertions cannot pass vacuously).
  - `internal/capture/swap_harness_test.go` — `TestModelAt_ReachesCapturedState` is a table over all 27 build-backed fixtures with a completeness sub-test that diffs the table against `FixtureNames()` minus the swatch, so a new fixture fails the test rather than going silently uncovered; each row asserts distinguishing present/absent content (and `ActivePage()` where the page constant is exported). `TestRenderSwapRender_ARenderPopulatesCachesBeforeSwap` asserts non-empty, not-the-blank-frame, fixture content present, A's canvas present, B's canvas absent in the before-frame and present in the after-frame. `TestRenderSwapRender_MutatesASingleModel` carries both halves: behavioural (the `projects` fixture is still on the Projects page after the swap) and structural (AST count over the real `harness.go` body: exactly 1 `ModelAt`, 0 `tui.Build`, exactly 1 `ApplyTheme`). The structural half is the one that actually catches the vacuous shape §13.4 names, since a correct restyle makes one model's frame indistinguishable from two — a good call.
  - `internal/capture/fixture_colourless_test.go` — both polarities plus a registry-wide agreement sweep between `Colourless()` and `Deps().NoColor`.
  - `internal/tui/restyle_repoint_test.go` — drives `ApplyTheme` and asserts every `bubbles/list`-owned cached style on both page lists (help, both pagination dot *styles* and the `Paginator`'s snapshotted dot *strings*, NoItems, TitleBar, PaginationStyle), both filter inputs, both delegates, the preview chrome, and a whole-palette stale-run scan across all three surfaces including the panel — each with a setup assertion that the pre-swap colour was present, so a clean post-swap scan can't be an unpainted surface.
- Notes:
  - Not under-tested: every prohibition is asserted rather than only documented, and nearly every assertion carries a positive control or setup pin. The absent-page assertion for `preview-screen` is content-based rather than page-based only because `pagePreview` is unexported — acceptable, and the three asserted strings are specific to the overlay.
  - Not over-tested: the seven counting seams in `TestApplyTheme_PerformsNoFileRead` are the widest piece of scaffolding here, but "a swap reads nothing" is exactly a claim over the whole seam set, and the aggregate counter keeps the assertion one line.
  - The ThemeSource seam (added by Phase 8, after this task) is not in that counting set, but the equivalent claim on the real production caller is covered by `internal/tui/theme_panel_arrow_test.go:375` `TestPanelArrow_NoFileReadPerKeystroke` — no gap.
  - Tests were assessed by reading, not by execution.

CODE QUALITY:
- Project conventions: Followed. Seams are small interfaces injected through `tui.Deps`; the harness stays test-only-by-construction (`internal/capture` is excluded from the shipped binary by the import guard); no `t.Parallel()`; no logging added to `internal/capture`; `internal/prefs` remains a leaf (only its enum is referenced).
- SOLID principles: Good. `ApplyTheme` is a two-line composition over the pre-existing `applyCanvasMode`, so the swap path has one reason to change; `ModelAt` / `RenderSwapRender` / `Colourless` are each one responsibility, and `RenderSwapRender` is expressed in terms of `ModelAt` rather than duplicating the drive sequence.
- Complexity: Low. `ApplyTheme` is 2 statements; `ModelAt` is a straight-line drive with two bounded loops and no branching beyond the seeded-fatal guard.
- Modern idioms: Yes — `reflect.Type.Fields()` iterator (Go 1.26), `range over int`, `slices.DeleteFunc`, generic `excludeColourless[F colourReporter]` in the consuming guard.
- Readability: Good. Comments explain the non-obvious (why `BootstrapCompleteMsg` is never sent, why exactly one model, why `Colourless` reads `Deps` rather than a name list).
- Issues: One comment-accuracy wobble in `harness.go` (see notes); one inherited-and-inert `captureKeys` entry.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/capture/harness.go:70-74 — the `RenderSwapRender` doc comment says "cached styles are assigned at construction … the A-render populates those caches", which is self-contradictory and falsified by the code: `Model.View` has a value receiver (`internal/tui/model.go:2697`), so the A-render mutates only a copy and populates nothing — the caches are populated by `tui.Build` and the `ModelAt` `Update` chain, both of which run under A. Replace the last clause so the rationale stays true: "// RenderSwapRender renders the fixture under theme a, swaps it live to theme b\n// through the production Model.ApplyTheme, and renders again. One model,\n// deliberately: cached styles are assigned when the model is built and driven\n// under a, so two models would each render correctly while live swap was broken.\n// The a-render is what proves the frame under a is the fixture's own."
- [quickfix] internal/capture/fixtures.go:685-690 — `projectsCommandPendingFixture` derives from `projectsFixture` and so inherits `captureKeys: {'x'}`, but a pending command already forces the Projects page and `x` is an explicit no-op while `commandPending` (`internal/tui/model.go:1734-1737`), so the key does nothing. Add `fx.captureKeys = nil` beside the existing overrides, so the declared key script is what the fixture actually needs rather than dead input a reader has to trace to discover is inert.
- [idea] cmd/capturetool/main.go:168-185 — `captureKeys` is replayed only by `ModelAt`; the live `capturetool --fixture <name>` path builds the model and hands it to `tea.NewProgram` without them, so the fixtures that need a key to reach their captured state (`projects`, `sessions-empty`, `preview-screen`, and every `theme-panel-*`) open on a different screen than the harness renders, and the user has to know which key to press. Decide whether capturetool should feed the fixture's declared script at start-up (which needs an exported accessor on `Fixture`, since `captureKeys` is unexported) or whether the manual keypress is the intended interactive behaviour.
