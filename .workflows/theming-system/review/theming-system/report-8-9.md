TASK: theming-system-8-9 — Arrowing Re-Themes The App And The Panel Through The Restyle Path

ACCEPTANCE CRITERIA:
1. `↑`/`↓` move the panel cursor one row and `Ctrl+↑`/`Ctrl+↓` page it; no other key moves it, and no vim alias, `PgUp`/`PgDn` or `Home`/`End` is bound.
2. Landing on a row applies that row's `Theme`: rendered main screen AND panel chrome both change colour, asserted as a diff of the composed frame across one arrow.
3. The cursor never rests on an unselectable row: down through three consecutive invalid rows lands on the next valid one; up past the top-most invalid row reverses rather than falling off.
4. The skip composes with paging exactly as the group-header skip does: `Ctrl+↓` onto a page whose first row is invalid lands on a selectable row on that page.
5. A preview swap performs zero file or directory reads (themes directory removed after open).
6. A preview swap does not call `rebuildSessionList` and triggers no `DirReader` pane read.
7. `startupCanvasHex` is byte-identical after one swap and after fifty.
8. `prefs.json` byte-identical after any number of arrow keypresses; an absent `prefs.json` stays absent.
9. Panel pagination dots, help styles and delegate all render in the previewed theme after a swap — on a union large enough to paginate.
10. A colourless model emits no hue and no background SGR after a preview swap.
11. Arrowing onto the already-active row is a no-op frame; repeated swaps idempotent per swap.

STATUS: complete

SPEC CONTEXT:
§9.2 pins the panel's interaction model: arrows move the cursor, "the app re-themes live behind the panel. Nothing is written", `Ctrl+↑`/`Ctrl+↓` page per MV §12.2, and the invariant "the cursor is always on a selectable row, and that row is always what is painted behind the panel". §9.5 requires arrows to skip invalid rows "reusing the mechanism that already skips group-header rows", composing with paging. §11.1 fixes the swap as the O(1) `applyCanvasMode` restyle and states flatly that it "does not call `rebuildSessionList`". §11.2 names the panel's `bubbles/list` "the worst case of this class" and requires its library-owned styles (pagination dots, help/title) be re-pointed by "the same restyle path as the main list, extended to cover the panel's instance" — not a rebuild — with the delegate re-derived per frame; it also demands a paginating fixture so the dots are actually drawn. §11.3 rules OSC 11 a deliberate no-op (Bubble Tea diffs the declarative background; the query is issued only from `Init`, so the echo guard needs no new handling). §5.8 requires previewing from the enumeration's retained parse — no file read per keystroke. §9.11 requires the panel's own chrome to re-theme, no exceptions. §9.2 also declares the mixed-mode flash the feature, with same-mode-first ordering explicitly rejected.

IMPLEMENTATION:
- Status: Implemented (mechanism intact; later phases refactored the shared restyle helper and condensed comments, neither of which changes this task's outcome)
- Location:
  - `internal/tui/theme_panel.go:334-335` — the nav arm in `updateThemePanel`, positioned after `Ctrl-C`/confirm/`Esc`/`Enter`/`d`/`l` and ahead of the swallow-everything `default`.
  - `internal/tui/theme_panel.go:342-344` — `themePanelNavKey` matches against the live `list.KeyMap` (`CursorUp`/`CursorDown`/`PrevPage`/`NextPage`), so routing and the list's own dispatch share one binding set.
  - `internal/tui/theme_panel.go:349-355` — `moveThemePanelCursor`: `list.Update` → skip → preview, returning the list's cmd. Carries the deliberate OSC-11 / mixed-mode-flash no-op note.
  - `internal/tui/theme_panel.go:359-380` — `skipUnselectableThemeRow`: bounded `2*rows` loop, direction from `CursorUp`/`PrevPage`, reversal at either boundary.
  - `internal/tui/theme_panel.go:384-390` — `previewSelectedThemeRow`: reads `themeRowItem.Row.Theme` (the retained parse) and calls `Model.ApplyTheme` only when it differs from the active theme; guards on `Selectable()` so a zero `Theme` can never be painted.
  - `internal/tui/theme_panel.go:84-92` — `newThemePanelList` applies `pinArrowOnlyNav`, the same pin the two page lists take.
  - `internal/tui/theme_panel.go:304-309` — `applyThemePanelCanvasMode`, guarded on `themePanel.open` so `armThemePanel`'s mid-arm `ApplyTheme` cannot run against a zero `list.Model`.
  - `internal/tui/model.go:905-913` — `applyCanvasMode` fans out to the panel alongside both page lists; `internal/tui/model.go:917-933` — `applyListCanvasMode` is the shared body (delegate, help, no-items, pagination dots, TitleBar, Title) with an explicit colourless branch, consumed by all three instances.
  - `internal/tui/model.go:897-900` — `ApplyTheme` is restyle-only, with `startupCanvasHex` named as the deliberate exclusion.
  - `internal/tui/model.go:1651-1655` — the panel's key-exclusive arm sits ahead of page dispatch; only key input is intercepted.
- Notes:
  - Every "Do" bullet lands. The swap goes through the production `Model.ApplyTheme` (no test-only setter), reads the already-parsed `Row.Theme` (no file, no `Reassemble`, no directory access), and the third list is covered by *extending* `applyCanvasMode` rather than rebuilding.
  - The restyle path was later refactored (phases 11–17) from two inline branches into the shared `applyListCanvasMode` / `applyPageListCanvasMode` pair. This is a strict improvement on the state this task shipped and preserves the requirement exactly — the panel now cannot drift from the page lists, because they run the same function body.
  - `newThemePanelList` deliberately does not set `InfiniteScrolling`, so the library's boundary behaviour is inert and the reversal in `skipUnselectableThemeRow` is the sole boundary rule — the two cannot fight. The reversal check reads `l.Index()` *before* moving, so it is correct either way.
  - Degenerate inputs are safe: zero rows gives zero loop iterations; an all-invalid union exits the bound with the cursor unselectable, and `previewSelectedThemeRow`'s `Selectable()` guard then declines to paint a zero `Theme`. The comment states built-ins guarantee termination in practice.
  - OSC 11 is correctly left alone: `View` assigns `BackgroundColor` from the active theme (`model.go:2804` for the canvas fill; the panel composites at `model.go:2831-2837`), with no suppression or debounce anywhere on the arrow path, and the in-source comment records that as deliberate.
  - No same-mode-first ordering, transition or flash mitigation exists anywhere on the path — §9.2's rejection is honoured.

TESTS:
- Status: Adequate
- Coverage: `internal/tui/theme_panel_arrow_test.go` (677 lines) carries all thirteen named tests, and each maps to a criterion:
  - `TestPanelArrow_NavigationBindings:141` — step vs page distinguished (asserts `Ctrl+↓` moves more than one row AND that `Paginator.Page` advanced, so a page key that merely stepped fails); panel stays open. (crit 1)
  - `TestPanelArrow_ArrowOnlyNavigation:172` — asserts the six live `KeyMap` bindings' key strings, then drives `j k h l g G d ← → PgUp PgDn Home End Space` through the real `Update` and asserts the cursor is unmoved and the panel still open. Covers the `d`-pages-instead-of-committing collision §12.2 warns about. (crit 1)
  - `TestPanelArrow_PreviewsThroughApplyTheme:243` — diffs composed-frame cell backgrounds at both a main-screen column and a panel-interior column across one arrow, with fixture guards that the two rows paint differently and that the open painted the cursor row. This is the criterion-2 assertion in its strong form. (crit 2, and §9.11)
  - `TestPanelArrow_SkipsConsecutiveInvalidRows:281` — three adjacent invalid rows; asserts landing index, label, `Selectable()`, and that the landing previewed. (crit 3)
  - `TestPanelArrow_SkipReversesAtTheBoundary:303` — both directions, each with two fixture guards: `requireArrowUnskippedLandingAt` proves the raw list would land on an unselectable row (so there is a real skip to exercise), and `requireArrowMoves` proves the arm is live in the opposite direction (so a dead arm cannot masquerade as a reversal). Also asserts the preview did not move. (crit 3)
  - `TestPanelArrow_SkipComposesWithPaging:352` — page size pinned to 2 via terminal height, unskipped landing proven invalid first, then asserts the landing row, that the paginator stayed on the page `Ctrl+↓` moved to, and that the paged landing previewed. (crit 4)
  - `TestPanelArrow_NoFileReadPerKeystroke:375` — real drop-ins on disk, `os.RemoveAll` the directory after open, then arrow to the second theme and assert its canvas painted from the retained parse plus `enumerator.opens == 1`. (crit 5)
  - `TestPanelArrow_DoesNotRebuildSessionList:448` — By-Project model with a counting `DirReader`, dirs deliberately re-emptied after the pre-render so a warm cache cannot make the zero vacuous; twenty presses plus a render must record zero reads, with a `rebuildSessionList` positive control proving the counter counts. Behavioural proxy rather than a call counter, which is the stronger choice. (crit 6)
  - `TestPanelArrow_StartupCanvasHexUnmoved:478` — one arrow and fifty, with a fixture guard that the arrow actually changed the active theme. (crit 7)
  - `TestPanelArrow_WritesNothing:504` — counting mode persister across sixteen arrows with a `s`-keypress positive control, plus a themes-directory-contents assertion. (crit 8, partially — see note)
  - `TestPanelArrow_PanelListStylesRepointed:558` — 20-row union so the list genuinely paginates; diffs the *rendered* dot row (not just the style structs), then pins `Paginator.ActiveDot`/`InactiveDot`, `HelpStyle`, `TitleBar`, `NoItems`, the colourless `Title`, and the delegate's cursor-row foreground. Directly covers §11.2's "the dots are read out of the styles once" trap. (crit 9)
  - `TestPanelArrow_ColourlessStaysColourless:607` — asserts no `38;2;`/`48;2;` in the post-preview colourless frame with a coloured positive control. (crit 10)
  - `TestPanelArrow_SameRowIsANoOp:646` — blocked arrow, A→B→A round trip, and A→B→A→B idempotence, each with a fixture guard that the intermediate frame actually changed. (crit 11)
- Notes:
  - Not under-tested. Every negative the task demanded ("assert the negatives, not just the positive") is a named test with a positive control, which is what stops a zero from being vacuous. Tests drive the real `Model.Update` via `pressPanelKey`, so nothing bypasses production dispatch.
  - Not over-tested. There is no redundant pair; the closest thing is `TestPanelArrow_PanelListStylesRepointed` asserting `HelpStyle`/`TitleBar`/`Title`/`NoItems`, which the panel's list does not render (title, help and status bar are all disabled at construction). That is defensible rather than bloat: §11.2's risk is that the third instance is *partially* covered by the shared path, and pinning the whole member set is what detects a partial extension.
  - Gap, minor: criterion 8 says `prefs.json` is byte-identical and an absent one stays absent, but the arrow test counts only the *mode* persister. The theme persister — the other writer of `prefs.json` — is counted at the `ApplyTheme` level (`apply_theme_test.go:207` `TestApplyTheme_PerformsNoFileRead`, fifty swaps, seven seams, zero calls) but not on the arrow path itself. The arrow path adds no persister call by inspection, so this is a coverage-shape gap rather than a defect. Noted below.

CODE QUALITY:
- Project conventions: Followed. Small interfaces at every seam; no `t.Parallel()`; the theme lives on the model and is threaded, never package-level; no raw hex at any call site (the colour-literal guard's domain); `internal/themetest` supplies the fixtures, keeping test-only helpers out of production.
- SOLID principles: Good. `moveThemePanelCursor` is three named steps (drive, skip, preview), each independently testable. The skip and the preview are separate functions with separate reasons to change. `applyListCanvasMode` is the single re-point body all three list instances share, so the completeness risk §11.2 raises is closed structurally rather than by discipline.
- Complexity: Low. `skipUnselectableThemeRow` is the only branching function and is a bounded loop with one switch; `themePanelNavKey` and `previewSelectedThemeRow` are trivially readable.
- Modern idioms: Yes — `for range 2 * rows`, `slices.IndexFunc`, builtin `max`. `key.Matches` against the live `KeyMap` rather than restated key strings.
- Readability: Good. Comments carry the non-obvious *why* and hold true against the code: the `open` guard's reason (a zero `list.Model` mid-arm), the loop-and-reverse deviations, the bound's purpose, and the OSC-11 / flash no-ops. No comment restates code and none references a task id or phase.
- Security: N/A — no I/O, no external input on this path.
- Performance: The whole point of the task, and met: one arrow costs `list.Update` + a bounded skip + an O(1) style re-point. No file read, no directory access, no enumeration, no list rebuild, no pane read.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_panel_arrow_test.go:504 — `TestPanelArrow_WritesNothing` counts only `countingModePersister`; add a `countingThemePersister` (already declared at `internal/tui/apply_theme_test.go:178`) to the same model and assert `calls == 0` after the sixteen arrows, so the criterion's "`prefs.json` is byte-identical" covers both writers of that file on the arrow path rather than one.
- [do-now] internal/tui/theme_panel.go:357 — the comment states the two deviations without naming what they deviate *from*. Replace the first line with: `// model.go's skipHeaderRow applied to Row.Selectable, with two differences: it` / `// loops (broken drop-ins can be adjacent) and reverses at either boundary rather` / `// than falling off. The 2×rows bound keeps an all-invalid union from spinning.`
