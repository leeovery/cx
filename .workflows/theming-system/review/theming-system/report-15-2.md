TASK: 15-2 — Stop The Panel's Blank Page-Alignment Rows From Raising The Height Refuse Threshold (tick-1eb62a)

ACCEPTANCE CRITERIA:
- At every content height that affords the page-aligned header today, `renderThemePanel` output is byte-identical to today's (dark, light and colourless).
- `themePanelMinHeight` equals header content rows + footer rows + one message row + one body row, plus the directory row when unusable — §9.8's composition, with no blank alignment rows counted.
- Across the previously-refusing band the panel opens and renders a complete block: rule row, label row, at least one list row, the message slot and the FULL footer, all within the height, nothing truncated.
- `themePanelFloor` still returns `dimWidth` before `dimHeight`, and the width ladder's constants and thresholds are unchanged.
- The header shape is decided in exactly one place; no renderer or arithmetic re-derives it.
- Both in-source references to the widest footer row state 16.
- `go test ./internal/tui` passes and the panel capture fixtures re-render unchanged.

STATUS: complete

SPEC CONTEXT:
- §9.1 (specification.md:954): "The label is `Themes` … followed by a one-row `border` rule … The header therefore costs **two rows**, which is what §9.8's minimum-height rule (header + footer + one row) resolves against."
- §9.8 (specification.md:1170-1183): "degrade, don't refuse"; "Minimum height … refuse with a flash only when **header + footer + one row + one message row** cannot fit"; resize degrades in place, closing only below the render floor.
- §9.5 (specification.md:1111): the `⚠ dir unreadable` row is a conditional addition to that floor.
- §9.1 (specification.md:961): at the minimum height the message slot truncates to one line rather than wrapping — the axis the fix-tracking cycle caught as the real byte-identity risk.

IMPLEMENTATION:
- Status: Implemented (commit 315e0e51; later comment-sweep commits e3fa1503 / 915e7fcb and 17-8 / 17-11 touched the same files and are the current shape).
- Location:
  - `internal/tui/theme_panel_geometry.go:30-71` — new `themePanelHeaderShape` value type (`ruleRow` / `labelRow` / `rows` + `borderFrom()`), `themePanelCompactHeaderShape()` (rule 0, label 1, 2 rows), `themePanelPageAlignedHeaderShape()` (measured off `headerBand` / `renderHeaderBlock` / `sectionHeaderBlockRows`), and the single predicate `themePanelHeaderShapeFor(height, dirUnusable)`.
  - `internal/tui/theme_panel_geometry.go:107-122` — `themePanelMinHeight` now delegates to `themePanelFloorFor(themePanelCompactHeaderRows, …)`; `themePanelChromeRows` takes `headerRows` as a parameter so floor and body budget share one arithmetic.
  - `internal/tui/theme_panel_geometry.go:146-155` — `themePanelListSize` passes `themePanelHeaderRows(height, …)`, following the same decision.
  - `internal/tui/theme_panel_render.go:14-41` — `renderThemePanel` resolves the shape once and hands it to both `themePanelHeaderBlock` (which indexes `shape.ruleRow` / `shape.labelRow` into a `shape.rows` block) and `themePanelBlock` (via `header.borderFrom()`), replacing the removed `themePanelBorderFromRow`.
  - `internal/tui/theme_panel_message.go:142-151` — `themePanelMessageWraps` re-anchored to the floor *for the shape being drawn*.
  - `internal/capture/theme_panel_message_fixtures_test.go:29` / `internal/capture/fixtures.go:493` — the min-height fixture now derives its terminal height from `tui.ThemePanelFloorTerminalHeight()` (13 → 10 at the time of this task, later made self-tracking by 17-11).
- Notes:
  - Arithmetic checks out against §9.8: compact floor = 2 header + 4 footer + 1 message + 1 body = 8 (9 with the directory row), i.e. `ThemePanelFloorTerminalHeight()` = 10 terminal rows. The previous 11/12 floor is gone.
  - Byte-identity above the affordance holds because the affordance is *the old floor computed with the page-aligned header*: at and above it the shape, `borderFrom` and the wrap threshold all resolve exactly as before. The one place this could have broken — `themePanelMessageWraps`, which used to key off `themePanelMinHeight` and would have started wrapping the confirm at the affordance — was caught in review cycle 1 (`fix-tracking-theming-system-15-2.md:4-14`, proven by a 4320-state HEAD-vs-tree render diff) and fixed by anchoring the threshold to the shape's own floor. The final code carries that fix and the matching pinned test.
  - Scope guard held: the const block (`themePanelPreferredWidth` 30, `themePanelMinWidth` 24, `themePanelPreferredAffordance`, `themePanelBorderWidth`, `themePanelGutterWidth`), `themePanelWidthFor` and `themePanelInnerWidth` are byte-identical to pre-task; only the height path moved.
  - Single decision confirmed by grep: `themePanelHeaderShapeFor` has exactly three production call sites (render, `themePanelHeaderRows` → `themePanelListSize`, `themePanelMessageWraps`), and the removed helpers (`themePanelHeaderRuleRow`, `themePanelHeaderLabelRow`, `themePanelBorderFromRow`) have zero remaining references anywhere in the tree.
  - Deliberate shape stability: the predicate is asked with `themePanelKeymap()` (standing scope), not the live footer scope, so raising/resolving a confirm never reflows the header. Correct — but see note 2 below, the rationale is no longer stated at that site.
  - Overflow re-checked by hand across both shapes: with the wrap threshold shape-anchored, `height − chrome ≥ 1` holds at and above the floor for every (dirUnusable × message × footer-scope) combination, so `themePanelBlock`'s defensive `break` never cuts the footer on a gated height.
  - "Fixtures re-render unchanged" is met in intent, not literally: `theme-panel-min-height-message` is *defined* as "the terminal that lands exactly on the floor", so lowering the floor necessarily moved that frame (13 → 10 rows). The fixture's stated purpose is preserved and 17-11 later made the height derive from `ThemePanelFloorTerminalHeight()` so it can no longer drift.
  - Deviation from §9.1 (rule-above-label rather than label-then-rule) is recorded in-source at `themePanelHeaderBlock` as a deliberate, not-to-be-flipped decision, as instructed; the later comment sweep dropped the literal "the design states" phrasing, consistent with the repo's no-spec-citation comment standard.

TESTS:
- Status: Adequate.
- Coverage:
  - `TestPanelGeometry_HeightFloorArithmetic` (theme_panel_geometry_test.go:108-136) pins `themePanelMinHeight` to the §9.8 composition using a test-local `wantPanelHeaderRows = 2` written out rather than read from production, for both the usable and unusable-directory cases, plus the shorter confirm footer.
  - `TestPanelGeometry_RendersAcrossTheCompactBand` (:833) renders every height from the new floor up to the affordance × {usable, unusable dir} × {empty, confirm, commit-failed} at minimum width, asserting exact block height, the label directly beneath the rule, a real list row, and every footer row present and uncut — the newly-opened band is covered end to end.
  - `TestPanelGeometry_HeaderShapeFollowsTheHeight` (:888) asserts the page-aligned frame at the affordance, +1 and +6 (rule in the page's rule lane, label on the page's section-header row, every other header row blank, first list row where the page's first session row is), the compact frame one row below, and — the subtest added by the review cycle — that the message slot still truncates at the affordance and only wraps below it.
  - `TestPanelChrome_FloorFollowsTheHeader` (theme_panel_chrome_test.go:297) derives the floor and the affordance independently from the page's own rendered chrome and asserts the header cost either side of the boundary, so the arithmetic is pinned against measured page chrome rather than production constants.
  - `TestThemePanelFooter_WidestRowIsMeasured` (theme_panel_footer_test.go:181) pins the widest rendered footer row at 16 and asserts it clears the minimum inner width — the corrected figure is now a measurement, not a reader's claim.
  - `themePanelFloor` precedence: the table case "both fail" (theme_panel_geometry_test.go:221) asserts `dimWidth`, and `TestPanelGeometry_TerminalEnds` (:77) pins the shortest opening terminal and the one-row-shorter refusal.
  - `panelHeaderRowsOf(m)` replaced the old fixed-header index in the behaviour/entry/geometry helpers, so those tests follow the shape rather than hardcoding it.
- Notes:
  - No over-testing of consequence. Footer-intact assertions recur across four tests, but each carries a distinct axis (list-overshoot clamp at preferred width; floor anatomy at minimum width; page-measured floor derivation; the whole new band × chrome states), so the overlap is deliberate rather than redundant.
  - Judged by reading only, per the review protocol — no suite was executed.

CODE QUALITY:
- Project conventions: Followed. No `t.Parallel()`, no new logging, no colour literals, geometry stays measurement-derived (`lipgloss.Height` off the real renderers at zero width on the colourless path) rather than literal, matching the discipline already applied to `themePanelDirRowHeight` / `themePanelFooterHeight`.
- SOLID principles: Good. The header shape is a small value type owning its own `borderFrom()`; the decision lives in one predicate that both the arithmetic and the renderer consume, so the "reserved rows are the rows that render" invariant is structural rather than conventional.
- Complexity: Low. One added branch; `themePanelFloorFor` factors the floor so the same sum is asked with two header costs instead of being restated.
- Modern idioms: Yes (`max`, `for i := range n` in the new test, value receivers on the shape).
- Readability: Good. Naming (`themePanelCompactHeaderShape` / `themePanelPageAlignedHeaderShape` / `themePanelHeaderShapeFor`) states the two shapes plainly.
- Issues: None blocking. Two comment/ergonomics nits below.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_panel_render.go:28-34 — `themePanelHeaderBlock`'s doc describes only the page-aligned shape ("sits above the label so it shares the page's rule lane … `Themes` lands on the `Sessions` section-header row"), which the compact shape it now also renders falsifies (rule at row 0, label at row 1, sharing neither lane). Replace the first sentence with: "// Two shapes, one renderer: the block indexes the shape it is handed, so the reserved rows are the rows filled. In the page-aligned shape the rule sits in the page's own rule lane and the label on the page's section-header row, with the remaining rows blank; in the compact shape the two close up at the top. The rule spans the border column and always sits above the label — do not reorder or notch it: a full-height border makes the panel read as a second column rather than a layer, and with the rule in the page's lane `Themes` lands on the `Sessions` section-header row, which is the alignment the panel is meant to keep." Keep the remaining paragraphs unchanged.
- [do-now] internal/tui/theme_panel_geometry.go:59-61 — the predicate hardcodes `themePanelKeymap()` and the reason (the confirm's shorter footer would flip the header shape mid-session and reflow it) is only stated two functions away, on `themePanelMinHeight`. Append to the existing comment: "// The standing keymap scope, never the live one: the confirm's shorter footer would flip the shape as it raises and resolves, reflowing the header under the user."
- [quickfix] internal/tui/theme_panel_geometry.go:107-122 — parameter order is inconsistent across the three arithmetic helpers: `themePanelFloorFor(headerRows, entries, dirUnusable)`, `themePanelChromeRows(headerRows, dirUnusable, messageRows, footer)`, `themePanelMinHeight(entries, dirUnusable)`. All three are `int`/`bool`/`[]keymapEntry` so a transposition is type-caught only by luck; align them on one order (e.g. `headerRows, dirUnusable, …, entries`) and update the four call sites plus the three tests that call them directly.
- [quickfix] internal/tui/theme_panel_geometry_test.go:839-843 — the compact-band subtests name their message state with `message.Kind`, an unstringered int, so failures read `dir=false/msg=1/h=9`. Iterate a labelled slice (`[]struct{name string; message themePanelMessage}{{"empty", {}}, {"confirm", geometryMessage()}, {"commit-failed", {Kind: themeMessageCommitFailed}}}`) and use `tc.name` so a failure locates itself.
