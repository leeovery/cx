TASK: theming-system-13-11 — Reconcile the panel width ladder with the specified column band (commit `e6d03898`)

ACCEPTANCE CRITERIA:
1. The width ladder and the specification state the same band; no comment contradicts §9.1/§9.8/§14A.
2. The ladder is staged (preferred, then minimum) rather than proportional — the panel's width no longer changes on every resize step across the mid range.
3. Every pinned panel string still fits, or truncates by its stated rule, at the minimum width.
4. The panel refuses only below the minimum, and the refusing path still returns a clamped width.
5. The visual gate is passed on the re-rendered narrow and minimum-height frames.
6. `go test ./internal/tui ./internal/capture` passes.

STATUS: complete

SPEC CONTEXT:
`.workflows/theming-system/specification/theming-system/specification.md` states the band in four places, all still reading 24–30: §9.8 "A fixed preferred width of ~24–30 columns … A fixed width is predictable to lay out against" (:1172); §9.5's row-composition priority "The elements compete for a fixed ~24–30 columns" (:1098); §9.1's table row for the failed-commit message "would read as heavy inside a 24–30 column panel" (:978) and its trade-off paragraph (:943); §14A "in the panel the wording is a layout constraint as much as a copy choice — it has to fit 24–30 columns" (:1792). The spec was NOT amended, so step 8's escape hatch was not taken — the fix-tracking record confirms the user passed the visual gate on the narrower band ("The visual gate was PASSED by the user: they kept the narrower 24–30 band").

IMPLEMENTATION:
- Status: Implemented (mechanism later re-homed by a subsequent phase, intent intact)
- Location:
  - `internal/tui/theme_panel_geometry.go:11-16` — `themePanelPreferredWidth = 30`, `themePanelMinWidth = 24`, `themePanelPreferredAffordance = 2 * themePanelPreferredWidth`.
  - `internal/tui/theme_panel_geometry.go:97-102` — `themePanelWidthFor`: preferred when `contentW >= 60`, else the minimum, with `ok = contentW >= 24`; `w` clamped to the minimum on the refusing path.
  - `internal/tui/theme_panel_geometry.go:76-78` — `ThemePanelMinWidthTerminal()` (= 63), the exported derivation the capture fixtures now key off instead of literals.
  - `internal/tui/theme_panel_geometry.go:127-135` — `themePanelFloor` still gates entry/resize width-first; `internal/tui/theme_panel.go:140` and `:278` take `w` and ignore `ok`, as the contract requires.
  - `internal/tui/theme_row.go:206` — `themeRowReason` charges the label `max(width, themeRowLabelFloor)` before deciding the reason fits; `internal/tui/theme_row.go:19-22` documents the floor as a guaranteed share ("Anything charged after the label must charge at least this much, or the row renders wider than its width").
  - Fixtures/tapes: `internal/capture/fixtures.go:492` and `:590` declare the narrow and min-height frames at `tui.ThemePanelMinWidthTerminal()`.
- Notes:
  - The commit modified `internal/tui/theme_panel.go`; a later phase split the geometry into `theme_panel_geometry.go` and pruned the long comments. That is the amended mechanism, not drift — the constants, the ladder shape and the return contract survived the move unchanged.
  - Comment reconciliation is complete: `grep` finds no "27–34"/"27-34" anywhere outside the historical `.workflows` analysis records — not in Go source, not in the `testdata/vhs` tapes, not in docs. The surviving band restatements (`internal/tui/theme_panel_message.go:97`, `internal/capture/theme_panel_message_fixtures_test.go`, the tapes) all read 24–30, and `CLAUDE.md` records "24 or 30 columns wide (`themePanelWidthFor`'s two-stage ladder)".
  - Step 4's re-check found a REAL regression the task itself introduced and fixed it: at inner 22 an unbadged ≤3-cell slug with `missing tokens` composed to 23 cells, overflowing the panel's declared width. The `max(label, floor)` charge in `themeRowReason` is the correct fix (the reason is §9.5's first-dropped element) and is a no-op for labels ≥ 4 cells, so no existing expectation moved. Worth noting the old `theme_row.go` comment claiming "the panel refuses to open at all at those widths" was falsified by the narrowing and was corrected rather than left standing.
  - AC1 ✓ (code 24/30 = spec 24–30, no contradicting comment). AC2 ✓ (two stages; a resize crossing 63→64 terminal columns is the only width change). AC4 ✓ (`ok` false only below 24; `w` clamped to 24 on that path).
  - AC3 ✓ by arithmetic through the production renderers: inner width at the minimum is 22 (`themePanelInnerWidth(24)`); `⚠ dir unreadable` = 16; `⚠ couldn't save theme` = 21; the widest footer row `l set as light` = 16 (3-cell key column + gap + 12); the confirm's fixed cost is 23, so it exceeds 22 and degrades by its stated rule — wrapping to two rows above the height floor, truncating to one at it, with the slug (never the `?  y / n` tail) taking the cut and never below `themeRowLabelFloor`.
  - AC5 ✓ by inspection of the re-rendered frames: `testdata/vhs/theme-panel-narrow.png` shows the 24-column panel with `tokyo-night…` cut, both badges intact and one line per row; `theme-panel-min-height-message.png` shows `⚠ couldn't save theme` unwrapped at the minimum; `theme-panel-confirm.png` shows the two-row wrap the spec anticipates, with `y / n` intact. The fix-tracking file records the user's explicit gate pass.
  - AC6: judged by reading only (no test execution). Every helper the changed tests reference exists (`insetRegion`, `panelSlugRow`, `labelTruncates`, `panelUnionSlugs`), the derived width constants type-check as `var` where they call functions, and the arithmetic each assertion pins matches the production constants.

TESTS:
- Status: Adequate
- Coverage:
  - Width-for-content-region table — `internal/tui/theme_panel_geometry_test.go:14-63`: wide, exactly at the affordance, one column below it, mid range and exactly at the floor, plus a 200→24 sweep asserting monotonicity, both-stages-only and `steps == 1` ("the ladder is staged, not proportional"). This is the assertion that would fail if the proportional rule came back.
  - Refusal + clamp — `theme_panel_geometry_test.go:90-100` sweeps every width below the minimum down to 0, asserting `!ok` AND the clamped `w`.
  - Terminal ends — `theme_panel_geometry_test.go:65-88` pins `ThemePanelMinWidthTerminal()` as the WIDEST terminal that steps down (one column wider takes the preferred width), so the exported derivation the fixtures consume cannot drift silently.
  - Decided-band literals — `internal/tui/theme_panel_chrome_test.go` `TestPanelChrome_LadderEnds` asserts 30/24 against literals, which is right for a decided (not derived) band; its former subtests were deleted because they duplicated the geometry suite or asserted the deleted rule.
  - Minimum-width copy — `theme_panel_message_test.go:156-268` (wrap costs two rows, truncation at the height floor, slug truncation to the floor with the leading phrase and `? y / n` intact, and every rendered line exactly 24 cells); `theme_panel_test.go:336-365` (`⚠ dir unreadable` untruncated at the minimum); `theme_panel_footer_test.go:194` (widest footer row ≤ the minimum inner width). §14A's exact copy including the double space is still byte-compared at the preferred width (`TestPanelMessage_ConfirmPinnedCopy`), so the capture-side relaxation to word comparison at the wrapping width loses no pin.
  - Row composition at the minimum — `theme_row_test.go:236` `TestThemeRow_ShortLabelDropsTheReasonRatherThanOverflow` renders through the real delegate and asserts exactly `themeRowTestMinWidth` cells, the reason dropped, the `⚠` kept and the label kept: precisely the regression found in attempt 1. The delegate's two test widths are now derived from `themePanelInnerWidth(...)` rather than restated, so a future ladder move reaches them.
  - Capture lane — `TestPanelFixture_NarrowRendersTheStepDown` measures the ladder's two ends off renders (never literals) and asserts the narrow frame equals the minimum; `TestPanelFixture_ConfirmWrapsAtMinWidth` pins the two-row wrap, the one-row render at the preferred width, and that the wrapped rows are charged to the list body; `TestPanelFixture_MinHeightMessageTruncates` covers the floor.
- Notes: Not over-tested — the commit deleted three subtests of the old `TestPanelChrome_WiderLadder` rather than porting them, and the fix-tracking record justifies each deletion against a surviving assertion. The one genuinely weakened assertion (the capture-side confirm comparison) is compensated by the untouched byte-exact pin in `internal/tui`.

CODE QUALITY:
- Project conventions: Followed. No `t.Parallel()`; no new logging; the exported `ThemePanelMinWidthTerminal` / `ThemePanelFloorTerminalHeight` are consumed by non-test code (`internal/capture/fixtures.go`, `cmd/capturetool`), so exporting them is legitimate rather than a test-only leak.
- SOLID principles: Good. The ladder stays a single pure function with one caller-facing contract; the floor gate remains the only entry/resize decision point, so the "entry and resize must consume this answer" invariant is preserved.
- Complexity: Low. The proportional `min(max(...))` became two branches; the reason-fit predicate gained one `max`.
- Modern idioms: Yes — builtin `max`, derived-not-restated test constants, `strings.CutSuffix` in the new capture helper.
- Readability: Good. Every constant carries a one-line rationale (`themePanelPreferredAffordance`'s "twice the preferred width, so at least half the previewed page stays visible" is arithmetically true at every point of the ladder: page ≥ panel for all contentW ≥ 24).
- Comment accuracy: Verified. No comment in the changed code is falsified by it — the "refuses to open at all at those widths" claim the narrowing falsified was corrected in the same commit, the "d set as dark is the widest footer row" miscount was corrected to `l set as light`, and the "only observable check on the ladder" claims that the moved message fixtures falsified were rewritten. No comment references task ids or plan phases.
- Security / performance: N/A — pure integer layout arithmetic on a render path.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/capture/theme_panel_message_fixtures_test.go:27 — `messagePanelTermWidth = 54` is now a bare literal (its ladder derivation was pruned with the surrounding comments), while the sibling narrow and min-height fixtures derive their terminal from `tui.ThemePanelMinWidthTerminal()`. Restore a one-line comment above it: "54 columns leaves 50 content columns — below twice the preferred width, so the ladder steps down to the minimum."
- [do-now] internal/capture/theme_panel_remaining_fixtures_test.go:32 — same for `minimumPanelTermWidth = 28`, which is the panel's unexported 24-column minimum plus `2*tui.Hinset` with nothing on the line saying so. Add: "the narrowest terminal that still renders a panel — the 24-column minimum plus the page gutter."
- [quickfix] internal/capture/panel_frame_test.go:115-141 — `panelSlugRow` collects every row whose truncated label is a prefix of the requested slug, then fatals with "renders on N panel lines — §9.5 puts every row on exactly one delegate line" when more than one matches. Two rows cut to different prefixes of the same slug (e.g. `tokyo-nigh…` and `tokyo-night…` against `tokyo-night-day`) would trip that message while the one-line invariant is intact. Unreachable with the current fixture unions; make the truncated branch keep only the longest matching prefix so the fatal can only ever mean what it says.
