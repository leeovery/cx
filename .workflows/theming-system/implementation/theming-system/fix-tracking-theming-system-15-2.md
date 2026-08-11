## Attempt 1

ISSUES:
- `internal/tui/theme_panel_message.go:311` — the byte-identity claim is FALSE at exactly the affordance. `themePanelMessageWraps` keys the truncate/wrap decision off `themePanelMinHeight`, which this task moved 11→8 / 12→9, so the wrap threshold moved with it. At content height 11 (usable dir) / 12 (unusable), panel width 24, with the confirm live, HEAD truncated the slot to one row and the working tree wraps it to two — a DIFFERENT FRAME AT A HEIGHT THAT RENDERED BEFORE.
  PROVEN, not theorised: the reviewer rendered HEAD and the working tree over 4320 states (heights 1–30 × widths {24,30} × dirUnusable × rows {1,3,12} × 4 message states × dark/light/colourless) and diffed — 36 cases differ at or above the affordance, all of them `msg=confirm*`, `w=24`, `h=affordance`. Concrete (h=11, usable, min width, colourless): HEAD rows 7–8 = `│   •••` / `│ clear constant nord? …`; working tree rows 7–8 = `│ clear constant nord?  ` / `│ y / n` — the panel also loses a body row.
  Any confirm wraps at this width (the shortest possible copy is 24 cells against a 22-cell inner width), so this is reachable, not theoretical, and it was NEVER VISUALLY GATED — the gated fixture is the new floor, and `theme-panel-confirm`'s tape is 31 rows.
  FIX: anchor the wrap threshold to the floor FOR THE SHAPE THE HEIGHT RENDERS, so the two thresholds coincide again inside each shape:
  ```go
  return height > themePanelFloorFor(themePanelHeaderShapeFor(height, p.union.DirUnusable).rows, themePanelKeymap(), p.union.DirUnusable)
  ```
  The reviewer applied exactly this in a scratch copy and re-ran the 4320-state diff: ZERO differences at or above the affordance, and the compact band still passes the full structural scan (block height, width, footer intact at every height).
  Also update the `themePanelMessageWraps` doc comment — the floor it compares against is now the floor for the header shape being drawn, not the panel's own floor — and extend `TestPanelGeometry_HeaderShapeFollowsTheHeight` with a `themePanelMinWidth` + confirm case asserting the slot truncates to one row AT the affordance and may wrap one row below it, so the axis that moved is the axis under test.
  ALTERNATIVE: keep the floor-anchored wrap and accept the affordance-height frame change as a deliberate visual change. Defensible in isolation (the whole message becomes readable) but it costs a list row at that height, contradicts the task's stated Outcome, and would need its own re-render and user gate — whereas the fix above costs one expression and keeps the newly-opened band's wrapping intact. The fix is recommended.
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- None. Every comment the diff introduced or touched checks out against the code, including both "widest footer row 16" figures, which the new footer test now measures.

NOTES:
- SCOPE GUARD HELD. The width band is genuinely untouched: the whole const block (lines 13–62), `themePanelWidthFor`, `themePanelFloor` and `themePanelInnerWidth` diff byte-identically against HEAD. `themePanelPreferredWidth`=30, `themePanelMinWidth`=24, `themePanelPreferredAffordance`=2×preferred, and the dimWidth-before-dimHeight ordering are all unchanged.
- The compact band is genuinely complete: 4320 rendered states machine-checked at or above the new floor — 0 problems. Exact block height, exact width on every row, rule row, label row, dir row when due, ≥1 list row, the message, and the full footer (`esc close` last) intact in every one.
- Both rewritten tests were rewritten because the task overturns their premise, verified at HEAD. `TestPanelChrome_FloorFollowsTheHeader` asserted the page-aligned floor this task exists to remove; the rewrite keeps that page-measured derivation and re-homes it as `chromeMeasuredAffordance`, then asserts both sides of the boundary. `messagePanelFloorTermHeight` 13→10 follows the fixture's own stated purpose, and its sibling assertions all still pass at the new height. Neither accommodates a mistake.
- Coverage gap that let this through: `TestPanelGeometry_HeaderShapeFollowsTheHeight` exercises the boundary only at `themePanelPreferredWidth` with an empty message slot — precisely the two axes on which the frame actually moved (min width + a wrapping message). It asserts anatomy, not identity.
- Non-blocking: parameter order is inconsistent across the three arithmetic helpers (`themePanelFloorFor(headerRows, entries, dirUnusable)` vs `themePanelChromeRows(headerRows, dirUnusable, messageRows, footer)` vs `themePanelMinHeight(entries, dirUnusable)`). Type-safe; worth aligning if the file is touched again.
- Non-blocking: `TestPanelGeometry_RendersAcrossTheCompactBand` names its subtests `msg=%v` on a type with no `String()`, so they read `msg=0/1/2`. A named case label would make a failure locate itself.
- Out of scope, pre-existing: `internal/capture/theme_panel_message_fixtures_test.go` still carries `§9.8` / `§14A` citations in failure strings adjacent to the touched constant.
