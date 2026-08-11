## Attempt 1

ISSUES:
- `internal/tui/header_rule_test.go:51-93` — `TestRuleGlyphRun_HasASingleRenderer` re-implements both primitives that `internal/sourceguard` provides for exactly this guard shape: `os.ReadDir(".")` + `.go`/`_test.go` filtering duplicates `sourceguard.PackageGoFiles(".", false)`, and `ruleGlyphRunFuncs`'s decl loop + `ast.Inspect` duplicates `sourceguard.ForEachFuncCall`. The package doc (`internal/sourceguard/doc.go`) declares itself the shared scanning primitive for guards, and several `package tui` test files already import it (`theme_panel_confirm_test.go:12`, `theme_panel_commit_test.go:9`, `modal_placement_consolidation_test.go`, `colour_literal_guard_test.go`, `nomination_test.go`) — so there is no in-package/external-package obstacle. Adding a private copy in a task whose stated sources are "duplication, standards" reintroduces the class of drift the task removes; it also drops `PackageGoFiles`'s "enumerated nothing is an error" behaviour.
  FIX: Replace the enumeration with `paths, err := sourceguard.PackageGoFiles(".", false)` (`t.Fatalf` on err) and the walk with `sourceguard.ForEachFuncCall(file, func(funcName string, call *ast.CallExpr) bool { if repeatsRuleGlyph(call) { renderers = append(renderers, funcName) }; return true })`, deleting `ruleGlyphRunFuncs` and the `os`/`ast.Inspect` plumbing; keep `repeatsRuleGlyph` and the `len(renderers) != 1` assertion as-is. Follow `internal/tui/theme_panel_confirm_test.go` for the parse+walk shape. (Always return true from the visitor — false ends the whole walk, not just the current declaration. A function containing two matching calls would then be listed twice, which only makes the guard stricter.)
  CONFIDENCE: high

COMMENT_CORRECTIONS:
- `internal/tui/header.go:68-69` — "Every rule in the app renders here" is false and is a cardinality claim the comment discipline forbids: `footerTopRule` (`internal/tui/footer.go:135-137`) renders the footer's rule from `footerRuleGlyph` through the same `th.Border`, and `panel.go:65-75` renders the modal frame's rules. Only the page-header rule and the panel's route through here.
  OLD:
  ```
  // Every rule in the app renders here, so a change of glyph or token moves the
  // page's rule and the panel's together — they share one lane.
  ```
  NEW:
  ```
  // The panel's rule shares the page header's lane, so it renders here too — a
  // change of glyph or token must move both.
  ```

NOTES:
- The acceptance criterion "byte-identical rule rows for the same width" holds for width > 0 only; at width ≤ 0 the two deliberately diverge (page falls back to 80, panel renders its own width). The tests encode that split correctly — identity at 24/30/80, exactness at 0/1/7.
- The panel comment now carries four claims in one block. All are currently true, but the older "do not reorder or notch it" and the new "must not be flipped" say adjacent things; a future edit could usefully merge them. Not a change request.
- The reviewer independently mutation-checked the new tests with `go test -overlay` (no repo edits): swapping the panel back to `headerSeparatorRule` fails `TestPanelRule_TakesNoPageFallbackWidth` at width 0 (80 cells vs 0). It also re-ran the guard's AST predicate against both the real `header.go` and a re-inlined `theme_panel_render.go` in a scratch program — it matches both call shapes, so a re-inlined duplicate would count 2 and fail.
