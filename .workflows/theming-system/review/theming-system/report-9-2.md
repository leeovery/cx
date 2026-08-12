TASK: theming-system-9-2 — The Post-Commit Recompute — Rows, Order, Badges And The Identity-Anchored Cursor

ACCEPTANCE CRITERIA:
1. `Enter` on a setting whose slots named a nonexistent slug removes that `not found` row.
2. A slot commit that makes an unresolvable opposite slot live adds its row.
3. Rows re-sorted by task 8-2's comparator (inserted row lands alphabetically, not appended).
4. Badges re-derive — a virgin install collapses two slot badges to one bare `●` on `Enter`.
5. No directory read (asserted with the themes dir removed after open) and no prefs read.
6. Never calls `ApplyTheme` — previewed theme and frame colours unchanged across the commit.
7. Cursor re-anchored by identity — a row inserted above the cursor leaves the cursor on the same theme.
8. A vanished previewed identity clamps to the first selectable row with no panic.
9. A commit of the already-persisted slug yields identical rows, badges and cursor index.
10. A `Resolve` fatal during recompute keeps the existing badge map while rows still re-derive/re-sort/re-anchor.
11. A failed commit does not recompute.
12. The list instance is the same object after recompute (items replaced, model not rebuilt); pagination dots still render in the previewed theme.

STATUS: complete

SPEC CONTEXT: §9.2 (specification.md:1018-1027) states that a successful commit recomputes the panel's full row set, not just the badges — `Enter` clears both slots so a persisted-only `not found`/`bad name` row disappears, while `d`/`l` on a constant makes the opposite slot live so a row for an unresolvable slug appears (the open-time union never minted one). The recompute must use the construction-time enumeration plus this instance's own mutation, never the RMW's merged bytes (§8.4/§8.9 decline cross-instance sync; the residue is the accepted per-instance staleness). It must re-derive the union (§9.4), re-sort it (§9.5) and re-anchor the cursor to the previewed theme's **identity, never its index**, with no re-enumeration (§5.8 pins enumeration to panel open). §9.2 also fixes the surrounding invariant "the cursor is always on a selectable row, and that row is always what is painted behind the panel", and "a commit is a write, not a navigation" — nothing on screen changes.

IMPLEMENTATION:
- Status: Implemented (mechanism amended by later in-plan tasks — see notes)
- Location:
  - `internal/tui/theme_panel_commit.go:97-107` — `recomputeThemePanel`: captures identity → `Reassemble(p.enumeration, themeState.keys)` → `applyCommittedSetting` (badges) → `list.SetItems(rowItems())` → `anchorThemePanelCursor(previewed)`.
  - `internal/tui/theme_panel_commit.go:109-115` — `previewedThemeIdentity` (row `Identity()`, not index).
  - `internal/tui/theme_panel_commit.go:120-127` — `applyCommittedSetting`: `Resolve` against the **retained** enumeration; non-nil error returns early, leaving the existing badge map (and nomination) untouched; never `ApplyTheme`.
  - `internal/tui/theme_panel_commit.go:31-43` — the single commit protocol: write → `applyCommitResult` → on error return (no recompute) → mirror keys → `recomputeThemePanel`. Both `commitConstant` (:45) and `commitSlot` (:86) route through it, and the confirm path (`theme_panel_confirm.go:55-64`) reaches it via `commitSlot`, so all three successful commit paths recompute and nothing else calls it (single call site, verified by grep).
  - `internal/tui/theme_panel.go:220-236` — `anchorThemePanelCursor` / `themePanelRowIndex`: identity match filtered by `Selectable()`, clamping to the first selectable row (`max(IndexFunc(...), 0)`) — shared with the open path, so one anchor helper serves both.
  - `internal/theme/union.go:126-138` — `Reassemble` sorts inside the assembler (`sortRows`) and touches no filesystem; `internal/theme/dir_theme_source.go:18-29` confirms neither `Reassemble` nor `Resolve` consults `Dir`.
- Notes:
  - Sorting is entirely delegated to `Reassemble`; there is no caller-side comparator (criterion 3), and `TestPanelRecompute_ResolveErrorKeepsBadges` pins that by handing back a deliberately non-alphabetical reassembly and asserting the panel renders it verbatim.
  - `applyCommittedSetting` also assigns `m.themeState.nomination` (line 126). That is **not** drift from 9-2: it was introduced by the later in-plan task 12-12 ("make the nomination contract structural at both ends", commit c502058b) and `theme_panel_confirm.go:66-71` names `applyCommittedSetting` as the nomination's single owner. It rides the same success-only path and does not weaken any 9-2 criterion.
  - The task's "Do" list asked for an in-source comment citing §9.2 on the index-anchor trap, and for the accepted cross-instance residue to be stated in-source. Later in-plan tasks 11-3 / 12-7 and the comment audits (e3fa1503, 915e7fcb) deliberately stripped spec-section citations and design-argument prose from production comments. The substance survives without the citations: `theme_panel.go:217-219` ("By identity, never index — the commit recompute can insert rows above the cursor") and `theme_state.go:39-41` ("A construction-time snapshot, never refreshed: re-reading prefs would let another instance's commit change what this panel shows"). Amended intent, not omission.
  - The clamp path (criterion 8) is unreachable in production by construction — `persistedRows` always attaches a `Rejection`, so every row a commit can delete is unselectable, and the previewed row is selectable by §9.2's invariant. It is drivable only through the `fakeThemeSource` seam, which is exactly how the test drives it; the "cursor is what is painted" invariant therefore cannot be broken by the clamp.
  - The recompute deliberately does not re-run `applyThemePanelListStyles`. Verified safe: `themePanelListSize` (`theme_panel_geometry.go:146-155`) is a function of panel width, header shape, `union.DirUnusable`, the message slot and the footer scope — none of which a recompute can change (`Reassemble` carries `DirUnusable` through from the retained enumeration), and row count does not enter the budget. `list.SetItems` re-runs `updatePagination` itself.
  - `_ = list.SetItems(...)` discards a `tea.Cmd`; the comment claims it is always nil. Verified against `charm.land/bubbles/v2@v2.1.0/list/list.go:380-392` — a command is returned only when `filterState != Unfiltered`, and `newThemePanelList` calls `SetFilteringEnabled(false)` with panel key dispatch never invoking the filter. Comment is accurate.

TESTS:
- Status: Adequate
- Coverage: `internal/tui/theme_panel_commit_recompute_test.go` carries all twelve named tests, one per criterion, plus `theme_panel_commit_protocol_test.go` (task 17-2) which pins the shared protocol across both commit shapes:
  - Row removal (:48) / row appearance with its `not found` reason and unselectability (:78) / alphabetical insertion (:99) — criteria 1-3.
  - `VirginInstallBadgeCollapse` (:131) asserts both the badge map and the **rendered** counts (1 bare `●`, 0 `● light`, 0 `● dark`) — criterion 4, checked at the pixel level rather than only in state.
  - `ReadsNothing` (:154): the "no directory read" subtest `os.RemoveAll`s the themes dir after open, then asserts the `sunset` row survives and `opens` stays at 1 — a re-read could not produce that row. The "no prefs read" subtest seeds `prefs.json` on disk naming `phantom` in a slot the in-memory keys hold as `ghost`, then asserts `phantom` never appears as a row — the load-bearing assertion for "not the merged bytes, not a fresh read" (criterion 5, §9.2's RMW rule).
  - `DoesNotApplyTheme` (:211) compares both `themeState.active` and the **set of SGR sequences** in the composed frame before/after, deliberately set-wise because rows legitimately move — criterion 6.
  - `CursorAnchoredByIdentity` (:237) pins the fixture at index 1, inserts a row above, and asserts index `before+1` **and** an unchanged active palette — an index anchor would pass a naive "cursor moved" check and fails this one (criterion 7).
  - `CursorClampsOnMissingIdentity` (:361) leads the reassembly with an unselectable row so the clamp must skip it (criterion 8).
  - `NoChangeCommitIsStable` (:262) asserts identical rows, `maps.Equal` badges, identical index **and** a byte-identical frame — idempotence proven, not assumed (criterion 9).
  - `ResolveErrorKeepsBadges` (:382) sets a **zero** `Resolution` alongside the fatal (commented as deliberate, so the assertion is not vacuous) and asserts the badge map is unchanged while rows still re-derive, hold the fake's non-alphabetical order and re-anchor (criterion 10).
  - `SkippedOnFailedCommit` (:410) asserts `reassembles == 0` on a failing persister **with a positive control** proving the counter moves on success (criterion 11); `TestCommitProtocol_FailedWriteMovesNothing` repeats it for both commit shapes, and `TestCommitProtocol_NilPersisterIsInert` covers the writer-less path.
  - `ItemsReplacedNotRebuilt` (:440) uses a `list.Title` sentinel production never touches, plus size/keymap survival, plus the rendered pagination-dot SGR against `AccentPrimary`/`TextFaint` — the exact failure a rebuilt `list.Model` produces (criterion 12).
- Notes:
  - Fakes are used where a real loader cannot produce the shape (`fakeThemeSource` for the fatal, the vanished identity, the split open/reassembly unions); everywhere else the tests run over `countingThemeSource`, which **embeds** the production `theme.DirThemeSource` rather than re-implementing it — so the "no directory read" and row-set assertions run against real assembly, real parsing and the real comparator.
  - No redundant tests found: `requireCommitDoesNoOtherIO` (used by 9-1/9-3's commit tests) overlaps only in theme with `ReadsNothing`, which asserts the strictly stronger removed-directory case.
  - Not over-mocked, not testing implementation details: assertions are on rendered output, row labels, cursor index and badge state rather than call sequences, with the two call-counters (`opens`, `reassembles`) used only where the criterion is literally "did not read"/"did not recompute".

CODE QUALITY:
- Project conventions: Followed. Small seam interface (`ThemeSource`, 4 methods) with production adapter in `internal/theme` and fakes in `_test.go`; no `t.Parallel()`; unit-lane only, no daemon/binary spawn; no raw hex; the panel's logging stays out of `internal/tui` (the `theme` component is emitted from the loader's injected `EventLogger`), matching CLAUDE.md's closed-component rule — this path emits nothing of its own.
- SOLID principles: Good. `recomputeThemePanel` is five ordered steps delegating each concern (assembly/sort to `theme.Assembler`, badge derivation to `theme.Badges`, anchoring to the shared `anchorThemePanelCursor`); the degrade policy sits with the resolution reader; the commit protocol (`commit`) is the single chokepoint so the three commit paths cannot drift.
- Complexity: Low. `recomputeThemePanel` is branch-free; `applyCommittedSetting` has one guard; `previewedThemeIdentity` one.
- Modern idioms: Yes — `slices.IndexFunc`, `max`, `cmp.Or` for `Row.Identity()`, `maps.Clone`/`maps.Equal` in tests.
- Readability: Good. Comments explain the non-obvious constraints (why not the merged bytes, why identity not index, why the error keeps the badges) without restating code.
- Comment accuracy: Verified. The `SetItems` "always nil" claim holds against bubbles v2.1.0; "the cursor re-anchors last" matches the body; "Never ApplyTheme" holds (no call in the recompute path, confirmed by grep and by the frame-colour test). No spec-section or task-id citations remain in the production comments.
- Security / performance: No concerns. The recompute re-parses the three embedded built-ins per commit keypress — deliberate (`union.go:140-142` explains the no-cache choice) and trivial at this scale; no I/O, no allocation growth.
- Issues: None found.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- None.
