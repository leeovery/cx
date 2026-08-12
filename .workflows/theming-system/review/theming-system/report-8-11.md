TASK: 8-11 (tick-dd4c9c) — Panel Geometry — Degrade Between Preferred And Minimum, Refuse Below The Floor

ACCEPTANCE CRITERIA:
1. `themePanelWidthFor` returns preferred on a wide terminal, a staged/monotone shrink across the narrowing band, the minimum at the bottom, `ok=false` below it.
2. The panel OPENS at `themePanelWidthFor(contentW)` — first frame already narrowed inside the degraded band, no resize involved.
3. Open width and post-resize width agree for the same content width — one function, two callers, one table.
4. Height floor = header(2) + MEASURED footer + 1 list row + 1 message row, +1 when `DirUnusable`.
5. `themePanelFloor` reports `dimWidth` when both dimensions fail; its result is the single input to both the entry gate and the resize path.
6. A resize above the floor keeps the panel open and re-sizes its list; rendered height == content height, every row == panel width.
7. A resize re-points the delegate (no intervening arrow): label truncates with `…`, badge stays inside the inner edge, no row exceeds the inner width.
8. A resize above the floor does NOT re-lay-out the main screen (footer still cut mid-label, not reflowed).
9. Below the width floor → `terminal too narrow — theme picker closed`; below the height floor → `terminal too short — theme picker closed`.
10. The forced close routes through `closeThemePanel` — resolved persisted state rendered, enumeration discarded, identical to `Esc`.
11. The forced close writes nothing.
12. At the minimum height a set message renders on exactly one truncated line with one list row still rendered; at the minimum width above the floor it may take two rows.
13. A panel open at exactly the floor renders header, directory row (when unusable), one list row, the message row's budget and the footer, with no overflow.

STATUS: complete

SPEC CONTEXT: §9.8 "Geometry — degrade, don't refuse" (spec:1170-1188) pins a fixed preferred width of ~24–30 columns, a *staged* shrink toward a minimum ("consistent with §2.7's existing width steps"), refusal only when even the minimum panel cannot render, and "Exact thresholds are pinned at implementation". Height carries the same degrade-then-refuse rule with the floor composed as header + footer + one row + one message row; §9.5 (spec:1111) adds the `⚠ dir unreadable` row to that floor conditionally; §9.1 (spec:961) requires the message slot to truncate to one line at the minimum height while it may wrap at the minimum width. §9.8 also requires a forced close to take the `Esc` path exactly and a live confirm to be silently cancelled; §14A (spec:1812-1813) pins the two forced-close strings. §9.7 (spec:1159) makes the same floor an *entry* condition.

IMPLEMENTATION:
- Status: Implemented (with two mechanisms deliberately superseded by later plan tasks — see below).
- Location:
  - Ladder + floor: `internal/tui/theme_panel_geometry.go:97-135` (`themePanelWidthFor`, `themePanelMinHeight`, `themePanelFloorFor`, `themePanelChromeRows`, `themePanelFloor`, `themePanelDim`/`dimWidth`/`dimHeight`), constants at `:8-28`.
  - Open applies the ladder: `internal/tui/theme_panel.go:139-153` (`armThemePanel` → `themePanelWidthFor(m.contentWidth())`).
  - Entry gate consumes the same predicate: `internal/tui/theme_panel.go:96-104`, re-checked with the real `DirUnusable` at `:129`.
  - Resize: `internal/tui/theme_panel.go:261-281` (`resizeThemePanel`), invoked from `internal/tui/model.go:1467-1481`; delegate re-point rides `applyThemePanelListStyles` → `applyThemePanelCanvasMode` → `applyListCanvasMode(&list, m.themeRowDelegate(), …)` (`theme_panel.go:295-309`, `model.go:917-933`).
  - Forced close: `theme_panel.go:265-277` calls the same `closeThemePanel` (`:241-245`) and raises `themePanelForcedCloseFlash` (`:44-49`) from the pinned constants `:31-32`.
  - Message degradation: `themePanelMessageWraps` (`internal/tui/theme_panel_message.go:145-151`) drives both the budget (`themePanelListSize`) and the render (`theme_panel_render.go:22`), so wrap/truncate cannot disagree.
  - Exported terminal-end helpers used by the capture fixtures: `theme_panel_geometry.go:73-85`, consumed at `internal/capture/fixtures.go:492-493,590`.
- Notes:
  - AC1's "a value strictly between minimum and preferred" no longer holds, deliberately: task 13-11 (tick-416417) reconciled the band with §9.1/§9.8/§14A, replacing 8-11's proportional `contentW/2` clamp with the two-stage ladder (`>= 2*preferred → 30`, else `24`, refuse below 24) and moving the ends from 34/27 back to 30/24. The current shape is what §9.8 actually describes ("staged degradation") and what 13-11's own acceptance criteria demand ("the panel's width no longer changes on every resize step"). Judged against the amended intent, AC1 is met — `TestPanelGeometry_WidthLadder`'s "it takes two widths and nothing between them" sub-test pins monotonicity, the two stages, exactly one step, and the bottom of the ladder.
  - AC4's "header(2)" survived a round trip: task 8-17 grew the header to the page-aligned 5 rows and moved the floor with it; task 15-2 (tick-1eb62a) then split the header into two shapes so the blank alignment rows are a degradation step, not a floor cost — the floor is back on `themePanelCompactHeaderRows = 2` (`theme_panel_geometry.go:41,107-108`), i.e. exactly 8-11's stated arithmetic. The header shape is decided in one place (`themePanelHeaderShapeFor`) and consumed by the arithmetic, the renderer and the wrap rule.
  - Task 17-1 (tick-70fa0c) added the missing auto-clear tick to the forced-close flash (`theme_panel.go:270-276`); 8-11's own criteria did not require it and the arm is now consistent with every other theme flash.
  - The "Phase 9 hook, do not build it" instruction holds correctly by construction: `closeThemePanel` zeroes the whole `themePanel`, so a live confirm and its `pending` assignment vanish silently, and nothing has been written (verified by `theme_panel_confirm_test.go:780-793`).
  - `themePanelFloor` is genuinely the only decision site: no caller re-derives width or height arithmetic, and `themePanelChromeRows` is shared by the floor and the body budget so a new chrome component cannot reach one and miss the other.

TESTS:
- Status: Adequate.
- Coverage: `internal/tui/theme_panel_geometry_test.go` carries every test the task names, under the named behaviours:
  - `TestPanelGeometry_WidthLadder` (table + the two-stage/monotonicity sweep), `_WidthFloor` (every width below the minimum refuses AND returns the clamped `w`), `_TerminalEnds` (the exported helpers are the widest stepping-down / shortest opening terminal, with the off-by-one on each side).
  - `_OpenUsesTheWidthLadder` — opens in the degraded band with no resize, asserted on the composed frame (`requireRenderedPanelWidth` checks the panel block's row count == content height, every row == panel width, and that those rows appear at the right column of `m.View()`).
  - `_OpenAndResizeWidthsAgree` — one table over 7 content widths asserting both the panel width and the list's `WxH` agree between the open path and the resize path.
  - `_HeightFloorArithmetic` — pins the composition against a written-out header constant, proves `themePanelFooterHeight` is the rendered footer's height, and re-runs the floor under a deliberately shorter footer so "measured, never a literal" is actually exercised. `_DirRowRaisesTheFloor` pins the +1 with a fixture check that the row measures 1.
  - `_FloorReportsWidthFirst` — including "both fail → dimWidth"; its "the resize path decides by this predicate alone" sub-test drives six regions through the real `Update(tea.WindowSizeMsg)` and asserts open/closed and flash purely from `themePanelFloor`'s answer. `theme_panel_entry_test.go:323-374` (`TestPanelEntry_SameFloorAsResize`) drives entry and resize through the same six regions and the same predicate — AC5's "one seam" is met across both callers.
  - `_ResizeDegradesInPlace` (narrow, widen, and a taller terminal re-deriving `PerPage`, with `requirePanelListMatchesTheRenderCopy` catching a stale paginator), `_ResizeRepointsTheDelegate` (no intervening arrow; fixture guards prove the label is untruncated before, then assert `…`, the trailing badge, and that no row exceeds the panel width), `_ResizeDoesNotReflowTheBase` (compares against a no-panel control and fails the fixture if the border lands on a word boundary, so the cut-mid-label case is genuinely exercised).
  - `_ResizeBelowWidthFloorClosesWithFlash` / `_ResizeBelowHeightFloorClosesWithFlash` — copy asserted against verbatim strings, not the production constants (`:559-562`), with the constants separately pinned; the height case also asserts the band is visible in the rendered frame.
  - `_ForcedCloseIsTheEscPath` — compares the forced close's full `View().Content` against the `Esc` path's after normalising the flash, plus the resolved-theme and discarded-enumeration assertions. `_ForcedCloseWritesNothing` uses counting persisters with a positive control.
  - `_MessageTruncatesAtFloorHeight` — both halves of AC12 (one truncated line at the floor with a list row still present and the `y / n` tail dropped; two wrapped rows one row above, reassembled from the two lines rather than substring-matched). `_RendersAtTheFloor` covers AC13 across `dirUnusable × message` with a guard that the floor does not silently afford the page-aligned header.
  - Edge cases from the spec are covered elsewhere in the suite rather than duplicated here: the silent confirm cancellation (`theme_panel_confirm_test.go:780-793`), the flash auto-clear and generation guard (`theme_panel_close_report_test.go:145-167`), paging/invalid-row skip (`theme_panel_arrow_test.go`, `theme_panel_open_test.go`).
- Notes: No under-testing found. Fixture-invalidating guards (`t.Fatalf("fixture: …")`) are used throughout, which is what keeps these from being tests that pass by testing nothing. Slight overlap exists between `_RendersAtTheFloor` and `_RendersAcrossTheCompactBand` at the floor height itself, but the two pin different invariants (§9.8's floor composition vs task 15-2's compact-band regression) and the overlap is one height — not worth collapsing.

CODE QUALITY:
- Project conventions: Followed. No raw hex at call sites (every colour goes through a token), the geometry file is stdlib + lipgloss + `internal/theme`, no `t.Parallel()`, no test executes a binary or touches tmux, comments carry no spec-section/task-id citations (the Phase 12/16 comment sweeps hold here).
- SOLID principles: Good. One predicate with one reason to change (`themePanelFloor`), the chrome sum shared by the floor and the body budget, the width ladder a pure function of content width consumed by both the open and the resize path, and the header shape decided once and consumed by three sites.
- Complexity: Low. Every function in `theme_panel_geometry.go` is a single expression or a two-branch guard; `resizeThemePanel` is the only place with real branching and its ordering constraint (read `commitFailed` before the close discharges it) is stated in-source at the line that depends on it.
- Modern idioms: Yes — `max`, multiple-return `themePanelListSize` feeding `SetSize` directly, value receivers for the shape struct.
- Readability: Good. Comments consistently state the *reason* rather than restating the code (e.g. why `w` is clamped on the refusing path, why the message row is in the floor, why the delegate must be re-pointed rather than only re-sized).
- Issues: One comment states a false equality (see the first non-blocking note). Nothing else.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_panel_geometry.go:20 — `// Matches the page's Hinset; charged once in themePanelBlock.` sits on `themePanelGutterWidth = 1` while `Hinset = 2` (internal/tui/model.go:2736), so the claim reads false; it is the border *plus* the gutter that equals Hinset. Replace with: `// Border plus this gutter sets panel content Hinset cells in from the panel's edge, matching the page; charged once in themePanelBlock.`
- [quickfix] internal/tui/theme_panel_geometry.go:50 — `themePanelPageAlignedHeaderShape` re-renders two chrome blocks (`renderHeaderBlock`, `headerBand`) on every call purely to measure heights, and it is reached ~3× per frame (`themePanelHeaderShapeFor` from `renderThemePanel`, from `themePanelListSize`, and from `themePanelMessageWraps`). Its inputs are constant, so wrap it in `sync.OnceValue` and call through the memoised func.
- [quickfix] internal/tui/theme_panel_geometry.go:107 — `themePanelMinHeight(entries []keymapEntry, …)`'s parameter is passed `themePanelKeymap()` at every production call site (`:84`, `:131`); it exists only so a test can substitute a shorter footer, and the doc comment has to warn "Pass the standing keymap scope, never the live one" as a result. Drop the parameter (call `themePanelFloorFor(themePanelCompactHeaderRows, themePanelKeymap(), dirUnusable)` inside) and have `TestPanelGeometry_HeightFloorArithmetic`'s shorter-footer assertion drive `themePanelFloorFor` directly — the invariant becomes structural and ~20 test call sites shorten.
- [quickfix] internal/tui/theme_panel_geometry_test.go:662 — `TestPanelGeometry_ForcedCloseWritesNothing` proves the seams are untouched but not the filesystem, while the sibling open/close proofs route through the shared `requireNoPrefsOrThemesWrite` (internal/tui/theme_testing_test.go:153). Add a `requireNoPrefsOrThemesWrite` call to the forced-close test (verb `"the forced close"`, act = open → arrow → resize below the floor) so the one close path the user cannot retry from is covered by the same prefs/themes-directory proof as the others.
