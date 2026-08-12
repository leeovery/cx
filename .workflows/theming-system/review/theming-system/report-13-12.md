TASK: theming-system-13-12 — Restore The Bold On The Panel Cursor Row (tick-9279eb)

ACCEPTANCE CRITERIA:
- The panel's cursor row reproduces all three elements of the shipped selection treatment: `▌`, tint, bold label.
- Only the label is bolded; trailing segments are unchanged.
- Unselected rows are unchanged.
- NO_COLOR behaviour matches the Sessions delegate's for the selected name.
- The colour-literal guard and the swap-and-diff completeness guard stay green.
- `go test ./internal/tui ./internal/capture` passes.

STATUS: complete

SPEC CONTEXT:
Specification §9.1 (specification.md:950) pins the theme slide-over's cursor row as "the shipped selection treatment (`▌` + tint + white bold name), so the panel's list reads as the same kind of list as Sessions", and the §9.1 token table (:970) restates it: "the shipped selection treatment — `bg.selection` tint, `accent.primary` `▌`, `text.on-selection` name". Before this task the panel reproduced the bar, the tint and the token but not the weight, and the `rowToken` comment asserted the opposite of the spec's requirement ("Panel rows carry no non-colour attribute of their own"). The spec also constrains the panel to render every surface through an existing token with no raw hex (§2.1 colour-literal guard, §13.4 swap-and-diff guard) — this change adds no colour, only a non-colour attribute, so neither guard's surface is touched.

IMPLEMENTATION:
- Status: Implemented
- Location:
  - `internal/tui/theme_row.go:161-168` — new `themeRowDelegate.labelStyle`, which passes the shared `nameBase` bold base into `rowTokenStyle` only when the row is selected.
  - `internal/tui/theme_row.go:69` — `renderRow` routes the LABEL run through `labelStyle`; the trailing loop (`:73-76`) and the cursor column (`:140-142`) still go through `rowToken`, which keeps the zero base.
  - `internal/tui/session_item.go:17` — `nameBase` is the shared style, reused verbatim rather than re-declared, so the two delegates cannot drift into two weights.
  - Commit `96e24dbe`, which also re-rendered `testdata/vhs/theme-panel-adaptive-pair.png` and `testdata/vhs/theme-panel-constant-previewing.png` (both PNG blobs changed in the commit — a verified fresh write, not a stale capture).
- Notes:
  - All four "Do" items landed. The misleading comment at the old `rowToken` (do-item 3) was replaced in the task commit and then trimmed further by a later comment-pruning task in phases 14–17; at HEAD `rowToken` carries no comment at all and `labelStyle` carries "Bold is the label's alone and, as a non-colour attribute, survives NO_COLOR." That is the amended-intent outcome, not drift: the false assertion is gone and the NO_COLOR decision (do-item 4) is still stated in-source.
  - NO_COLOR parity is structural, not incidental: `rowTokenStyle` returns `base` unchanged when `colourless` (session_item.go:166-169), so the panel's bold survives the carve-out exactly as the Sessions selected name does (`row_style_helpers_test.go:139` golden shows the colourless Sessions row rendering `\x1b[1malpha\x1b[m`).
  - Visual check performed against both re-rendered frames. `theme-panel-adaptive-pair.png`: the `nord` cursor row carries `▌` + tint + a visibly heavier label than its neighbours, and its `● dark` badge is the same weight as the unselected `● light` badge. `theme-panel-constant-previewing.png` (light): `tokyo-night-day` cursor row likewise bold, unselected labels unchanged.
  - No new colour literal is introduced anywhere, so the glob-based `colour_literal_guard_test.go` is untouched; the swap-and-diff guard scans rendered colour, and bold is theme-invariant, so `TestThemeRow_NoCachedStyles` (which asserts a theme swap changes colour but not text) still holds.
  - Pre-existing colour assertions survive the added SGR parameter by construction: `tokenFgSeq` returns the bare parameter run (`theme_testing_test.go:279-296`), so `themeRowRunAfter(out, "38;2;…")` still matches inside the now `1;38;2;…`-prefixed sequence at `theme_row_test.go:475`. No stale expectation builds a selected panel label with a zero base (single hit, `theme_row_test.go:630`, which is the new parity test itself).

TESTS:
- Status: Adequate
- Coverage: All five test items from the task are present in `internal/tui/theme_row_test.go`:
  - `TestThemeRow_CursorRowLabelIsBold` (:586) — selected label bold, unselected label not, across both built-in themes.
  - `TestThemeRow_CursorRowBoldsOnlyTheLabel` (:606) — the `●` badge and the `⚠`/reason runs are not bolded on the cursor row.
  - `TestThemeRow_CursorRowLabelStyleMatchesSessionName` (:623) — byte parity of the rendered style against `SessionDelegate.rowToken(nameBase, text.on-selection, true)` across both themes × colourless, which is the anti-drift pin.
  - `TestThemeRow_ColourlessCursorRowKeepsBoldWithoutHue` (:639) — NO_COLOR cursor row keeps bold and emits no colour; the unselected colourless row still emits no escape at all.
  - Visual check of the two re-rendered frames (PNG bytes changed in the commit).
  - The pre-existing `TestThemeRow_ColourlessIsGlyphBacked` (:516) was correctly relaxed from "no escape sequence at all" to the new `assertThemeRowHasNoColour` walker (:534), which bans every SGR parameter except `1` — a tightening in substance (it still forbids all hue and all background) rather than a loosening.
- Notes:
  - Not over-tested: each test pins one distinct property, and the trailing-segment test sensibly runs one theme only (weight is theme-independent), while the colour-sensitive ones loop both.
  - One thin gap: nothing asserts the `▌` cursor column itself stays unbolded. It is structurally safe (`cursorColumn` → `rowToken` → zero base) but is the one element of the three-part treatment with no weight assertion.
  - `themeRowRunIsBold` (:581) detects bold by a raw `slices.Contains(params, "1")` over the whole opening SGR parameter list. The truecolor runs are `38;2;R;G;B` / `48;2;R;G;B`, so a token whose R, G or B channel is exactly `1` would read as bold. No shipped built-in token has a channel of 1 (verified against `tokyo-night.theme` and the light pair's selection-path tokens), so the tests are sound today; the fragility is bounded to the fixture themes but the positive assertion is the one that could false-pass.

CODE QUALITY:
- Project conventions: Followed. Reuses the shared `nameBase`/`rowTokenStyle` seam rather than re-deriving a bold style, keeps every colour behind a token, adds no package-level theme state, and leaves the one-line-per-row pagination invariant untouched (the change is a style, not a run).
- SOLID principles: Good. `labelStyle` is a single-responsibility split of what was one overloaded `rowToken` call site; the delegate's other two callers (cursor column, trailing segments) keep the simpler helper, which is exactly the "split the call sites" instruction rather than bolding everything.
- Complexity: Low. One branch, no new state, no new width arithmetic.
- Modern idioms: Yes. `lipgloss.Style{}` zero value as the neutral base matches the file's existing idiom.
- Readability: Good. The surviving comment states the rule and the NO_COLOR consequence in one line; the earlier false claim is gone rather than merely softened.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_row_test.go:606 — in `TestThemeRow_CursorRowBoldsOnlyTheLabel`, add one assertion that the cursor column is not bolded: `if themeRowRunIsBold(t, out, selectorBar) { t.Errorf("the cursor row bolded its ▌ selector bar: %q", escSeq(out)) }`. It pins the last unasserted element of the three-part selection treatment; the `▌` run's opening SGR carries only `accent.primary` over `bg.selection` (no channel equal to 1 in either built-in), so it passes as written.
- [quickfix] internal/tui/theme_row_test.go:581 — make `themeRowRunIsBold` skip truecolor/indexed parameter runs before testing for `"1"`: walk the parameter list and, on `38`/`48`, jump past the following `2;R;G;B` or `5;N` group (the same walk `applySGR` already does in `canvas_cell_background_test.go:16-54`). As written a token with a channel of exactly `1` would be read as bold, which would silently false-pass the positive assertion.
- [idea] internal/tui/theme_row.go:162 — decide whether the panel's UNSELECTED labels should also carry `nameBase`. The Sessions delegate bolds its name on every row (`session_item.go:251,256`; the goldens at `row_style_helpers_test.go:136` show the unselected `bravo` bold), so the panel's non-cursor rows still read a step lighter than Sessions' non-cursor rows even though §9.1's stated reason for the treatment is that "the panel's list reads as the same kind of list as Sessions". This task explicitly scoped itself to the cursor row ("Unselected rows are unchanged"), so this is not a deviation — it is the residual parity question that scope leaves open, and answering it needs a design call, not an edit.
