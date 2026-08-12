TASK: theming-system-8-5 — The Panel Keymap Scope And Its Vertical Footer

ACCEPTANCE CRITERIA:
- `themePanelKeymap()` returns exactly six entries in the declared order, four of them `Core`.
- The rendered footer is exactly four rows reading `⏎ set theme`, `d set as dark`, `l set as light`, `esc close` — asserted against §14A verbatim.
- Arrows and paging present in the descriptor, absent from the rendered footer.
- Key glyphs in `accent.key`, labels in `text.muted`, asserted as distinct SGR runs.
- Labels share a left edge (fixed key column) and every row is exactly one line.
- A substituted entry slice renders that footer instead, with no renderer change.
- `themePanelFooterHeight` equals `lipgloss.Height` of the rendered block for the four- and two-entry cases.
- No `RightAligned` entry and no `?` entry in the scope.
- `sessionsKeymap()`, `projectsKeymap()`, both page footers and both help bodies byte-identical.
- Under `colourless` the footer emits no background SGR and no hue.

STATUS: complete

SPEC CONTEXT:
§9.12 ("The panel's keymap is descriptor-governed") requires the panel's keys to live in the keymap descriptor as a panel scope carrying **all six** keys (`↑`/`↓`, `Ctrl+↑`/`Ctrl+↓`, `Enter`, `d`, `l`, `Esc`) — complete, because the dispatch guard's contract is descriptor↔dispatch parity — with the vertical footer rendering the `Core` subset only, arrows and paging present as non-core exactly as §14.1 treats arrows on the main footer. `?` does nothing inside the panel and must not appear in the scope. §9.1 pins the vertical form (a horizontal keymap does not fit ~30 columns) and the token split (`accent.key` glyphs / `text.muted` labels). §14A line 1803 pins the copy verbatim: `⏎ set theme` / `d set as dark` / `l set as light` / `esc close`. §9.2/§1042 reserve a nested confirm scope (`y confirm` / `n cancel`) whose footer temporarily replaces the standing one — Phase 9's work, which this task only had to leave room for.

IMPLEMENTATION:
- Status: Implemented (with two deliberate later-phase amendments, neither drift)
- Location:
  - `internal/tui/keymap.go:62-71` — `themePanelKeymap()`; six entries, the two nav entries supplied by the shared `navKeymapEntries()` (`keymap.go:20-25`) and the four commits marked `Core`. Entry values are byte-for-byte the task's pinned literals (`Key`/`HelpKey`/`Action`/`HelpAction`), verified against the test's verbatim expectation table.
  - `internal/tui/theme_panel_footer.go:1-46` — `renderThemePanelFooter(entries, width, th, colourless)`, `themePanelFooterHeight(entries)`, `themePanelFooterRows`, `themePanelFooterRow`, `themePanelFooterKeyColumnWidth = 3`.
  - Entries are a parameter and the renderer never calls `themePanelKeymap()` — the substitution point is live in production: `themePanelFooterScope` (`theme_panel_message.go:74-79`) hands the confirm scope in while the confirm is raised, consumed at `theme_panel_render.go:23`.
  - Height is single-sourced: `themePanelChromeRows` (`theme_panel_geometry.go:117-122`) reserves `themePanelFooterHeight(footer)`, and both the entry floor (`themePanelFloorFor`, `:111`) and the live body budget (`themePanelListSize`, `:146-155`) read that one arithmetic — exactly the "8-6 layout and 8-11 floor read one source" requirement.
  - Rendering reuses the shared `keyColumnRow` (`modal_footer.go:32-41`) — the same two-column key-column primitive `helpModalRow` uses — with `helpKeyGlyph` for the `HelpKey` override, `headerStyle(th.AccentKey…)` / `headerStyle(th.TextMuted…)` for the split, and `headerPadRight` for the canvas pad.
- Notes:
  - Two later-plan amendments, both intentional and both improvements, not drift: task 16-7 replaced the task's inline nav literals with the shared `navKeymapEntries()` (identical values, one declaration for all three descriptors), and task 11-8 extracted `keyColumnRow` so the panel footer and the help modal share one key-column implementation.
  - The task text asked the doc comments to name "Phase 9" and the §-references; tasks 11-3 / 12-7 and the later comment audits deliberately stripped process-artifact citations from production comments repo-wide. The surviving comments still carry the substance — `keymap.go:62-63` states the scope is complete and must not be trimmed to the footer's `Core` keys; `theme_panel_footer.go:14` states the entries are a parameter *because the confirm scope substitutes a shorter footer*. Correct outcome under the amended convention; flagging the missing citations would be flagging the remediation.
  - Every comment in the new file holds against the code: over-wide rows really are returned unpadded (`padRightWithStyle`, `header.go:119-125`), the block really never wraps (pure `JoinHorizontal`), and the fixed column really does stop labels stepping sideways when the two-entry confirm substitutes in.
  - The deferred item landed as planned: `keymap_dispatch_guard_theme_test.go:152-157` now drives descriptor↔dispatch parity over both the panel and confirm scopes, so the "descriptor must already be complete" bet paid off.

TESTS:
- Status: Adequate
- Coverage: All ten named tests exist with the specified names — `TestThemePanelKeymap_CarriesAllSixKeys`, `_CoreIsTheFourCommits`, `_DoesNotLeakIntoPageSurfaces` (`theme_panel_keymap_test.go`), and `TestThemePanelFooter_PinnedCopy`, `_NonCoreEntriesAreNotRendered`, `_KeyIsAccentKeyLabelIsTextMuted`, `_KeyColumnIsFixedWidth`, `_AcceptsASubstitutedScope`, `_HeightMatchesRender`, `_Colourless` (`theme_panel_footer_test.go`). Each acceptance criterion maps to a real assertion:
  - Copy is pinned verbatim in a *function* returning the four strings (`theme_panel_footer_test.go:20-27`, with an in-comment justification for not sharing a slice), and compared after collapsing padding — not re-derived from the descriptor, so a descriptor typo fails.
  - The token split is asserted as painted *runs* (`themeRowRunAfter`, `theme_row_test.go:54-70` returns the text a given SGR run painted), not mere sequence presence — the assertion cannot pass on a row that carries the sequence around the wrong text. Run for both built-in themes.
  - The fixed key column is measured in **cells**, not bytes (`themePanelFooterLabelColumn`, `:234-241`), which is the correct measurement for multi-byte glyphs, and is checked for the substituted scope too.
  - Non-core absence checks both the label and both glyph forms (`Key` and `helpKeyGlyph`).
  - Containment (`_DoesNotLeakIntoPageSurfaces`) asserts over both descriptors, both rendered footers and both help bodies, and — importantly — carries an anti-vacuity subtest ("the panel footer is the scope's only consumer") proving the needles actually appear somewhere, so the containment assertions cannot silently guard nothing. The byte-identity half of that criterion is separately pinned by Phase 9's `TestKeymapGuard_PageDescriptorsUnchanged` (`keymap_dispatch_guard_theme_test.go:393+`), a literal golden of every page entry's flags.
  - `_Colourless` asserts no background *and* no foreground SGR plus intact copy; `_HeightMatchesRender` cross-checks the constant-time height against the real render across two themes × two widths for both scopes.
  - One extra beyond the plan, `TestThemePanelFooter_WidestRowIsMeasured` (`:181-198`), pins the widest row (16 cells) under the 22-cell minimum inner width — justified, since the renderer pads and never truncates, so an over-wide row would silently break the panel's minimum-width contract rather than fail loudly.
- Notes: Minor redundancy only, both from Phase 9 landing broader versions of assertions this task authored — see the non-blocking notes. Nothing is over-mocked; the tests drive the real renderer with real built-in themes via `themetest`.

CODE QUALITY:
- Project conventions: Followed. Renderer signature matches the codebase's pervasive `(…, width int, th theme.Theme, colourless bool)` shape; no raw hex (the `colour_literal_guard_test` contract holds); the file is styling-layer only with no state; comments carry the *why* and no process artifacts, matching the post-11-3 standard.
- SOLID principles: Good. The renderer takes its entries as a parameter, so the confirm scope substitutes without a second renderer (open/closed, and the reason the Phase 9 footer swap is a one-line change at `theme_panel_render.go:23`). Descriptor (data) and renderer (presentation) stay separate.
- Complexity: Low. Four short functions, one loop, no branching beyond the `Core` filter.
- Modern idioms: Yes — `make([]string, 0, len(entries))` pre-sizing, variadic `JoinVertical`, no reflection or stringly-typed dispatch.
- Readability: Good. `themePanelFooterRows` / `themePanelFooterRow` split reads cleanly; the fixed-column constant carries its rationale.
- Issues: None material. The `keyColumnRow` call packs 8 arguments across 4 semantically grouped lines rather than one-per-line; this matches `helpModalRow`'s identical call shape (`help_modal.go:77-82`), so it is the established local convention and the grouping is at semantic boundaries.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/theme_panel_footer.go:23 — `themePanelFooterHeight` measures with `lipgloss.Height`, which returns 1 for the empty string, so a scope with zero `Core` entries would reserve one chrome row that `appendBlock` (theme_panel_render.go:124-129) never emits. Switch it to the file-local `blockHeight` helper (theme_panel_render.go:142-147), which `themePanelMessageHeight` already uses for exactly this reason; both live scopes are unaffected (4 and 2 rows), so the pinned heights stay.
- [quickfix] internal/tui/theme_panel_footer_test.go:50-52 — the `lipgloss.Height(lines[i]) != 1` check is vacuous: `lines` comes from `strings.Split(…, "\n")`, so each element is single-line by construction. The "every row is exactly one line" criterion is already carried by the `len(lines) != len(themePanelFooterPinnedRows())` fatal above it. Drop the inner check, or measure the unsplit row instead.
- [quickfix] internal/tui/theme_panel_keymap_test.go:30-39 — the "it pins no right anchor and no ? entry" subtest is now a strict subset of Phase 9's `TestThemePanelKeymap_NoRightAlignedEntry` and `TestThemePanelKeymap_NoHelpEntry` (keymap_dispatch_guard_theme_test.go:297, 329), which assert the same two properties over *both* theme scopes, add a page-descriptor control, and additionally prove `?` is swallowed by the live dispatch. Delete the subtest here and leave the guard file as the single owner.
