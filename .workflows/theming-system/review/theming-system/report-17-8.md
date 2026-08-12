TASK: theming-system-17-8 — Render The Panel's Rule Row Through The Header's Shared Renderer

ACCEPTANCE CRITERIA:
- `strings.Repeat(headerRuleGlyph, …)` appears in exactly one function in `internal/tui`.
- The page header and the panel header render byte-identical rule rows for the same width, theme and colourless flag.
- The panel's rendered frame is unchanged (rule row above the label, spanning the border column, at both header shapes).
- The rule-then-label rationale is stated in-source at `themePanelHeaderBlock`.

STATUS: complete

SPEC CONTEXT:
§9.1 (specification.md:954) pins the panel header as the `Themes` label "rendered in `accent.mode` … followed by a one-row `border` rule, matching the Sessions section-header idiom minus the count", costing two rows. The two halves of that sentence disagree: "followed by a rule" reads label-then-rule, while "matching the Sessions section-header idiom" requires the rule to sit in the page's rule lane with `Themes` on the `Sessions ··· N` row — i.e. rule-then-label. The implementation follows the idiom clause; the task's job was to record that resolution in-source so it cannot be "fixed" back, and to stop the panel hand-rolling the page's rule.

IMPLEMENTATION:
- Status: Implemented (all four Do steps, exactly as specified)
- Location:
  - `internal/tui/header.go:68-73` — new `ruleOfWidth(w int, th theme.Theme, colourless bool) string` = `headerStyle(th.Border, th, colourless).Render(strings.Repeat(headerRuleGlyph, max(w, 0)))`, with the shared-lane comment above it.
  - `internal/tui/header.go:75-77` — `headerSeparatorRule` reduced to `ruleOfWidth(headerWidthOrFallback(width), th, colourless)`; the page's zero-width fallback (`headerFallbackWidth` = 80, `header.go:33-38`) is preserved exactly and is applied before the renderer, so the page path is behaviour-identical (`headerWidthOrFallback` always returns > 0, making the renderer's `max(w, 0)` a no-op there).
  - `internal/tui/theme_panel_render.go:37` — `rows[shape.ruleRow] = ruleOfWidth(width, th, colourless)`; the panel passes its own width and never routes through the fallback, and the clamp it previously carried inline now lives in the renderer.
  - `internal/tui/theme_panel_render.go:28-34` — comment extended with the §9.1 resolution in plain language ("Rule-then-label is deliberate and must not be flipped to label-then-rule: with the rule in the page's lane, `Themes` lands on the `Sessions` section-header row…") plus the "width is the panel's own — it never takes the page's fallback" note.
- Notes:
  - AC1 verified by grep: the only production-source occurrence of `strings.Repeat(headerRuleGlyph, …)` in `internal/tui` is `header.go:71` inside `ruleOfWidth`; every other hit is in `_test.go` files. `footerTopRule` (`footer.go:135-137`) repeats a *different* glyph (`footerRuleGlyph = "▔"` vs `headerRuleGlyph = "▁"`), so it is correctly out of the shared renderer's scope and the header.go comment is scoped to the two rules that genuinely share the lane.
  - AC2 holds for width > 0 (the only widths at which the two are meant to agree); at width ≤ 0 they diverge by design — the page falls back to 80, the panel renders its own width. The comment at `theme_panel_render.go:33-34` records that split; the tests encode both halves.
  - AC3: the rule still lands at `shape.ruleRow` with `borderFrom() = ruleRow + 1` (`theme_panel_geometry.go:36-39`), so it continues to span the border column; `themePanelBlock` pads pre-border rows to full `width` (`theme_panel_render.go:71-74`), and the rule is already `width` wide, so `themePanelPadRow` is a no-op on it. Byte output is unchanged from the pre-task code (identical style + identical `max(width, 0)` glyph run).
  - No drift from the task, and nothing later in the plan supersedes this mechanism.

TESTS:
- Status: Adequate
- Coverage (`internal/tui/header_rule_test.go`, new):
  - `TestPanelRule_RendersThroughThePageRuleRenderer` (:19-31) — panel rule vs `headerSeparatorRule` at widths 24/30/80 × colourless {false, true}. Guards the shared-lane property behaviourally: a re-inlined panel rule that diverges in glyph, token or style fails here.
  - `TestPanelRule_TakesNoPageFallbackWidth` (:33-44) — widths 0/1/7 assert both `lipgloss.Width(row) == width` and `ansi.Strip(row) == strings.Repeat(headerRuleGlyph, width)`. This is the load-bearing one: swapping the panel to `headerSeparatorRule` would render 80 cells at width 0 and fail. Covers the AC's explicit width-0 case.
  - `TestRuleGlyphRun_HasASingleRenderer` (:48-63) — AST source guard for AC1. Correctly scoped to production sources: `parsePackageFilesByName` (`nomination_test.go:252-273`) calls `sourceguardtest.PackageGoFiles(".", false)`, which excludes `_test.go` (`packagegofiles.go:26-28`) and errors on an empty enumeration, so the guard cannot pass by having stopped looking. Uses the shared `sourceguardtest.ForEachFuncCall` primitive rather than a private AST walk, matching the repo's ~20 other source guards; the visitor always returns true, so the walk is not truncated at the first hit.
  - AC3 regression cover is the pre-existing, unmodified frame/geometry suite, which asserts at both header shapes: `TestPanelChrome_RulesShareOneLane` / `TestPanelChrome_BorderStartsBelowTheRule` (`theme_panel_chrome_test.go:167-213`), `TestThemePanel_HeaderIsMeasuredAndCountless` (`theme_panel_test.go:226-245`, page-aligned shape at preferred width), and the compact-shape floor assertions at `theme_panel_geometry_test.go:799-804` plus `TestPanelGeometry_HeaderShapeFollowsTheHeight` (:888-909, page-aligned lane indices derived from `headerBand`/`renderHeaderBlock`).
- Notes:
  - Not over-tested. The equality test overlaps the structural guard, but the two catch different failure modes (a divergent-but-still-glyph-run reimplementation vs. a duplicated glyph run), and the 3×2 matrix is trivial. No redundant setup or mocking; the `panelRuleRow` helper (:14-17) drives the real `themePanelHeaderBlock` rather than asserting on internals.
  - No under-tested gap found: both ACs with observable behaviour (byte-identity, no-fallback) and the structural AC have direct tests, and the frame ACs ride existing shape coverage.

CODE QUALITY:
- Project conventions: Followed. Comments carry no spec-section citations, task ids or phase references (the discipline task 11-3 established); the new source guard reuses `sourceguardtest` rather than hand-rolling enumeration/traversal (CLAUDE.md's stated single-source rule for guard primitives); the guard is stdlib-only and untagged, so it runs in the unit lane. No `t.Parallel()`. No colour literals introduced (the `colour_literal_guard_test` surface is untouched).
- SOLID principles: Good. Clean split of responsibilities — `headerWidthOrFallback` resolves width (page policy), `ruleOfWidth` renders the glyph run (shared mechanism). The panel consumes the mechanism without inheriting the page's policy, which is exactly the distinction the old duplication blurred.
- Complexity: Low. `ruleOfWidth` is two statements; `headerSeparatorRule` is one.
- Modern idioms: Yes — builtin `max`, no redundant helpers left behind.
- Readability: Good. `ruleOfWidth`'s name reads at both call sites, and the shared-lane comment explains why a page-header file renders a panel row. The `theme_panel_render.go` comment is long but every clause records a decision that would otherwise be re-litigated; the "with the rule in the page's lane" phrasing is correctly conditional, so it does not falsely claim page alignment in the compact shape.
- Comment accuracy: Verified. `header.go:68-69` claims only that the panel's rule shares the page's lane and renders here too — true, and correctly narrower than a cardinality claim over all rules (`footerTopRule` and `panel.go`'s modal frames render other glyphs). `theme_panel_render.go:28-34` is accurate against the code: the rule does span the border column (`borderFrom() = ruleRow + 1`), the order is rule-then-label, and the width is the panel's own.
- Security: N/A — pure string rendering, no external input.
- Performance: N/A — one extra function call per header render.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `internal/tui/header_rule_test.go:60-61` — the failure message says "%d functions build the rule glyph run", but `renderers` accumulates one entry per matching *call site*, so a single function containing two matching calls would report `2` and list its name twice. Replace the message with: `t.Errorf("%d call sites build the rule glyph run (%s); want exactly 1 so the page and the panel cannot drift", len(renderers), strings.Join(renderers, ", "))`.
