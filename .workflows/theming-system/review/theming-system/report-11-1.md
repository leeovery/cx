TASK: theming-system-11-1 — Extract The Shared Per-List Canvas/Colourless Restyle Sequence (tick-70a3ce, severity high, source: duplication)

ACCEPTANCE CRITERIA:
1. `SetDelegate` + help/no-items/pagination/title-bar/title-strip restyle appears in exactly one function in `internal/tui`.
2. All three former functions delegate to it; none retains its own `if m.colourless` fork over these steps.
3. Panel, sessions and projects rendered output unchanged (committed capture references and swap fixtures match, no reference regeneration).
4. The panel's open-state guard remains at the panel call site, not inside the shared helper.

STATUS: complete

SPEC CONTEXT:
Spec §11.1 (line 1355) defines the restyle as the O(1) mid-session swap path: `applyCanvasMode` swaps the delegate and re-points the cached style structs `bubbles/list` holds — no rebuild, no I/O — with the panel's arrow-preview / open / close as its production callers. §11.2 (lines 1374–1382) names the panel's list as "the third `bubbles/list` instance, and the worst case of this class" (styles assigned once at open, re-themed on every arrow keypress) and requires its library-owned styles to be re-pointed by "the same restyle path as the main list, extended to cover the panel's instance". §11.2 also states explicitly that the two known fixes are "what was found, not the boundary of the class", with §13.4's swap-and-diff guard as the safety net. The extraction here is exactly the "same restyle path" the spec mandates, made structural rather than triplicated.

IMPLEMENTATION:
- Status: Implemented (matches the plan's `Do` steps 1–5 and all four acceptance criteria)
- Location:
  - `internal/tui/model.go:917-933` — `applyListCanvasMode(l *list.Model, delegate list.ItemDelegate, th theme.Theme, colourless bool)`: the whole six-step sequence (`SetDelegate`, help styles, no-items, pagination dots + paginator re-feed, `Styles.TitleBar` background/unset, `Styles.Title = stripListTitleColours(...)`) with the `colourless` fork inside it, in the order the session/project arms previously used (title strip inside each branch).
  - `internal/tui/model.go:937-940` — `applyPageListCanvasMode`: the shared sequence plus the `listTitleBarStyle(...)` geometry, the one thing the two page lists carry and the slide-over does not. The task's step 2 ("make the title-bar geometry difference explicit") is satisfied by this named wrapper rather than a boolean or a parameter — the better of the two options offered, since the geometry now has a name and a documented reason.
  - `internal/tui/model.go:905-913` — `applyCanvasMode` reduced to filter input + preview palette + three delegating calls.
  - `internal/tui/model.go:966-969` — `applyProjectCanvasMode` reduced to a delegate expression + one call.
  - `internal/tui/theme_panel.go:304-309` — `applyThemePanelCanvasMode` reduced to the `open` guard + one call to the bare `applyListCanvasMode` (deliberately not the page wrapper).
- Byte-for-byte preservation check (AC 3), verified against the commit diff (`2520a16a`):
  - Page lists, colour path: was `listTitleBarStyle(TitleBar.Background(canvas))`; now `TitleBar.Background(canvas)` then `listTitleBarStyle(TitleBar)`. Same three fields set (Background, PaddingLeft, PaddingBottom) on a copied `lipgloss.Style` — identical result. Colourless path identical by the same argument with `UnsetBackground()`.
  - Panel: the old arm hoisted `stripListTitleColours` above the fork; the shared helper applies it inside each branch. `Styles.Title` and `Styles.TitleBar` are separate fields and the strip is unconditional on both paths, so ordering is not observable. The panel still does NOT receive `listTitleBarStyle` — the divergence the task explicitly forbade changing is preserved.
  - Projects delegate: was two literals (`Colourless: true` / field omitted); now `Colourless: m.colourless` — identical for both values.
  - The commit touched only `.go` files plus workflow/tick bookkeeping — no `testdata/vhs` reference or fixture PNG was regenerated, as AC 3 requires.
- AC 4: the `if !m.themePanel.open { return }` guard is at `theme_panel.go:305-307`, outside the shared helper. `armThemePanel` (`theme_panel.go:147-149`) sets `open = true` before `applyThemePanelListStyles`, so the guard does not skip the initial styling.
- AC 5 sweep (no other surface performs part of the sequence inline): a repo-wide grep for `SetDelegate` / `Styles.TitleBar` / `Styles.NoItems` / `Styles.HelpStyle` / `ActivePaginationDot` in non-test production code returns only the step helpers at `model.go:702-745` and the two new functions. The one other `SetDelegate` (`model.go:890`, `refreshSessionDelegate`) is a multi-select marked-set re-point, a different concern, correctly left alone.
- Notes: `model.go:1020-1027` (`applyListSize`) retains its own `if m.colourless` fork choosing the pagination row's base style — a partial echo of the sequence's canvas-vs-nothing decision. It pre-dates this feature (`a306115e`, three-column-keymap-footer) and is a resize concern rather than a restyle step, so it is out of this task's stated scope, but it is the nearest surviving relative of the duplication this task removed. Recorded as a non-blocking note.

TESTS:
- Status: Adequate
- Coverage:
  - `internal/tui/restyle_repoint_test.go:258-280` — `TestRestylePath_NoStaleColourSurvivesOnAnyList`: the test the task asked for. It opens the slide-over through the production `t` keypress over a 20-row union (asserting `TotalPages >= 2` so the dot row — the exemplar cache, since `bubbles/list` snapshots its dot strings at construction — is actually on the scanned frame), renders all three surfaces under the pre-swap palette, calls `ApplyTheme`, then scans each rendered surface for *every* token of the pre-swap palette (`staleRuns`, `restyle_repoint_test.go:282-294`). Scanning the whole palette rather than a hand-maintained list of named caches is the right call: it backstops exactly the "cache nobody thought to name" failure §11.2 warns is the real boundary of the class.
  - Vacuous-pass defences are present and load-bearing: a pre-swap `t.Fatalf` that each surface *does* carry the old canvas (line 263), a post-swap assertion that each carries the new canvas (line 275), a probe-setup assertion that the panel open did not itself repaint off a different palette (line 229), and `populateRestyleProbe` renders both pages *before* the swap so a once-assigned cache cannot pass by never having been populated.
  - Surfaces are rendered separately rather than read off one composed frame, so a survivor names the surface it survived on — a genuine diagnostic improvement, not ceremony.
  - The per-style probes (`TestRestylePath_RepointsListOwnedStyles`, lines 103-131) still pin the named caches at field granularity for the two page lists; `theme_panel_arrow_test.go:585-592` pins the panel's paginator strings through the arrow entry point.
  - Colourless fork of the extracted helper: covered for the panel by `theme_panel_open_test.go:430-443` (dot row carries no bg/fg SGR) and for the pages by `pagination_dots_test.go:177`.
- Notes: The whole-surface scan overlaps the per-style probes, but at a different granularity and with a different failure mode (unnamed cache vs named cache), and the overlap is explained in-source. Not over-tested. No new mocks or scaffolding were introduced — the panel probe composes existing helpers (`newArrowPanelDeps`, `pressThemeKey`), and `populateRestyleProbe` was factored out of the existing probe constructor rather than copied.

CODE QUALITY:
- Project conventions: Followed. Package-level functions taking `*list.Model` keep the sequence testable without a `Model`; no `t.Parallel()`; test helpers call `t.Helper()`; no new logging or state.
- SOLID principles: Good. Single responsibility is the point of the change — `applyListCanvasMode` restyles one list, `applyPageListCanvasMode` adds the page-only geometry, the panel's open-state precondition stays with the panel. The former incidental divergence is now an explicit, named difference.
- Complexity: Low. Each of the three former functions is now 1–3 statements; the fork exists once.
- Modern idioms: Yes (`for i := range n` in the new test helper, value-returning `lipgloss.Style` chaining).
- Readability: Good. The comments at `model.go:915-916`, `model.go:935-936` and `theme_panel.go:301-303` each state a non-obvious *reason* (why the geometry is excluded, why the geometry layers over the background, why the paginator must be re-fed, why the guard is where it is) rather than restating code. All hold true against the code as written. No process-artifact references remain in either file (zero `§` occurrences in `model.go`, `theme_panel.go` and `restyle_repoint_test.go` — later phases cleaned these, and this task's regions are clean at HEAD).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/model.go:1020-1027 — `applyListSize` keeps its own `if m.colourless` fork to choose the pagination row's base style (`lipgloss.NewStyle()` vs `.Background(canvas)`), duplicating the same canvas-or-nothing decision made at `model.go:737` and `model.go:745`. Extract `paginationRowBase(th theme.Theme, colourless bool) lipgloss.Style` and call it from all three sites, so the last surviving fork over this decision goes the way of the three this task collapsed.
- [idea] internal/tui/model.go:917 — AC 1 ("exactly one function") is currently held by convention only; a fourth list can re-inline the sequence without any test failing. Consider a unit-lane source guard (the repo already has ~20 driven by `sourceguardtest`) asserting that no production file outside `model.go` assigns `Styles.NoItems`, `Styles.ActivePaginationDot`, `Styles.HelpStyle` or `Styles.TitleBar` — decide whether the guard is worth its maintenance against the swap-and-diff net that already covers the rendered consequence.
