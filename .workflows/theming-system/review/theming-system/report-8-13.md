TASK: theming-system-8-13 — Entry Conditions, Blocked-t Flashes And Key-Exclusive Routing

ACCEPTANCE CRITERIA:
1. `t` opens the panel on Sessions and on Projects when nothing blocks it.
2. Under `NO_COLOR` (`colourless`), `t` on either page opens nothing and raises exactly `theme picker needs colour — NO_COLOR is set`.
3. Below the width floor `t` raises exactly `terminal too narrow for the theme picker`; below the height floor, `terminal too short for the theme picker`; when both fail, the width copy.
4. The entry gate and the resize path read the same floor predicate — a size that blocks entry also force-closes an open panel, asserted across one table.
5. With a usable directory, a terminal one row above the non-directory floor opens the panel — no speculative `⚠ dir unreadable` row reservation.
6. With an unusable directory at that same height the panel does not open: enumeration discarded, short flash raised, no panel state survives.
7. A panel that opens with an unusable directory renders at least one list row beneath the pinned `⚠ dir unreadable` row, asserted at the directory-inclusive floor exactly.
8. `t` on Preview and Loading does nothing and raises no flash.
9. `t` with a modal open does nothing; `t` during a pending burst is swallowed with no flash.
10. Blocked `t` inherits the auto-clear (tick + next actionable key, both pages).
11. While the panel is open `k`, `x`, `m`, `/`, `?`, `s`, `e`, `n`, `r`, `q` leave the model unchanged.
12. `d` and `l` never reach the page beneath (on Projects `d` opens no delete modal).
13. `Ctrl-C` while the panel is open quits.
14. `t` from multi-select leaves the marked set and its banner intact; closing returns to multi-select with the same set.
15. No descriptor is filtered by this task (both static keymaps byte-unchanged).

STATUS: complete

SPEC CONTEXT:
§9.6 (specification.md:1146-1155) pins the per-page table: `t` on Sessions and Projects, refused on Preview (out-of-theme scrollback body, already a full-screen overlay), refused **silently** on Loading (inert by design, no notice band to flash into), refused by modals (already key-exclusive), and a filter carve-out so `t` stays a literal filter character while `/` is focused. §9.7 (:1157-1168) pins the closed blocker set — modal, pending burst, `NO_COLOR`, below the render floor, unbound pages — plus the key-exclusive routing (`Ctrl-C` stays live; `k`/`x`/`m` must not reach the page), the multi-select nesting rule with `Esc` innermost-first, and the flash-versus-silent feedback rule. §9.8 (:1170-1181) makes below-the-floor an entry condition as well as a resize condition; §9.10 draws `NO_COLOR` (capability absence, proactive block) opposite to a narrow terminal (space shortage). §14A (:1809-1813) pins the three entry strings and, separately, the two differently-worded forced-close strings.

IMPLEMENTATION:
- Status: Implemented (matches the plan's mechanism; later phases only trimmed comments and re-homed one descriptor test)
- Location:
  - `internal/tui/theme_panel.go:34-36` — the three pinned entry constants, sited beside the forced-close pair with a "worded differently on purpose — do not unify" note.
  - `internal/tui/theme_panel.go:51-56` — `themePanelEntryFlash(dim)`, the dimension→copy map, mirroring `themePanelForcedCloseFlash`.
  - `internal/tui/theme_panel.go:96-104` — `themePanelEntry()`: `colourless` first, then `themePanelFloor(contentWidth, contentHeight, false)`, else open. Single decision site; no other caller.
  - `internal/tui/theme_panel.go:106-119` — `handleThemePanelKey` (gate → open or block) and `blockThemePanel` (raise through `setThemeFlash`, return `flashTickCmd(m.flashGen)` after the generation bump).
  - `internal/tui/theme_panel.go:123-134` — `openThemePanel`: `Open` then the **same** `themePanelFloor` re-applied with the real `union.DirUnusable`; on failure it returns `blockThemePanel` before `armThemePanel`, so nothing is retained.
  - `internal/tui/model.go:1756-1757` (Projects) and `internal/tui/model.go:2424-2425` (Sessions) — the `t` arms, both below their page's `SettingFilter()` guard; the Sessions arm is deliberately outside the multi-select suppression block that gates `k`/`r`/`n`/`x`.
  - `internal/tui/model.go:1648-1655` — the panel's key intercept, ahead of the page dispatch and scoped to `tea.KeyPressMsg` only (resizes/refreshes still reach the page beneath).
  - `internal/tui/theme_panel.go:314-339` — `updateThemePanel`: `Ctrl-C` → `tea.Quit` first, then confirm, `Esc`, `Enter`, `d`, `l`, nav; `default: return m, nil` swallows everything else.
  - `internal/tui/theme_panel_geometry.go:127-135` — the one `themePanelFloor` predicate, width-first, consumed by entry, the post-read re-evaluation and `resizeThemePanel` (`theme_panel.go:265`).
- Notes:
  - Silence where `t` is unbound is structural rather than special-cased: `PageLoading` returns before any rune dispatch (`model.go:1658-1668`) and `pagePreview` forwards to the preview model, which binds no `t` (`pagepreview.go:324-340`). Nothing to remove, nothing to flash.
  - The burst swallow is likewise structural — the `burstPending` arm at `model.go:2333-2338` returns before the rune switch, exactly as the task asked (assert the lock, do not add a branch).
  - The three entry strings are byte-identical to specification.md:1809-1811, and the forced-close pair at `theme_panel.go:31-32` is byte-identical to :1812-1813 (the two pairs are correctly kept distinct).
  - Copy-level regression risk is contained: every renderer of these strings goes through the two `…Flash(dim)` helpers, so a dimension cannot silently borrow the other's copy.
  - The task's "state in-source" instructions are only partly present after the later comment-trim cycles (12-7 / e3fa1503). The mechanism is recorded (`theme_panel.go:94-95`, `:121-122`); the accepted `theme: enumerated` side effect of a post-read refusal is not. That is a comment-density judgment made downstream, not behavioural drift — see the `[do-now]` note.
  - Not drift: the plan's `TestPanelEntry_LeavesDescriptorsUnfiltered` was deleted by task 8-14 (commit 53a4fdad) and its invariant re-homed to `TestFooterRevision_StaticDescriptorsUnfiltered` (`footer_revision_test.go:289`), which is where the call-site filter it guards actually lives.

TESTS:
- Status: Adequate
- Coverage (`internal/tui/theme_panel_entry_test.go`, 747 lines, 15 tests — one per named plan test bar the re-homed descriptor one):
  - AC1 `TestPanelEntry_OpensOnSessionsAndProjects` (:170) — both pages, exactly one enumeration, no flash, active page unmoved.
  - AC2 `TestPanelEntry_NoColorBlocked` (:193) — both pages, verbatim copy, band actually rendered through `View()`, and `rec.opens == 0` proving the block is proactive (reads nothing).
  - AC3 `TestPanelEntry_FloorBlocked` (:214) — narrow / short / both-fail, with the both-fail row pinning width-first precedence; `rec.opens == 0` on all three.
  - AC4 `TestPanelEntry_SameFloorAsResize` (:323) — six regions, each driving entry and resize off the same `themePanelFloor` answer, asserting open-state parity plus the two differently-worded flashes.
  - AC5 `TestPanelEntry_UsableDirectoryOpensAtTheNonDirFloor` (:246) — opens at exactly the non-dir floor, with a fixture guard that fails loudly if the two floors ever stop differing by one row (otherwise the height would discriminate nothing).
  - AC6/AC7 `TestPanelEntry_UnusableDirectoryBlocksOnTheReEvaluation` (:273) — the refusal asserts `rec.opens == 1` (the read happened) **and** a zero-valued `themePanel` (width/rows/entries/badges), i.e. genuinely discarded; the sibling subtest opens one row higher and asserts the dir row sits directly under the header with a real list row beneath it, at the exact directory-inclusive floor.
  - AC8 `TestPanelEntry_SilentOnPreviewAndLoading` (:393) — no panel, no enumeration, empty `flashText`, and `cmd == nil`.
  - AC9 `TestPanelEntry_ModalKeepsTheKey` (:442, three modals incl. Projects delete) and `TestPanelEntry_SwallowedWhileBurstPending` (:427, also asserts the lock is not cleared).
  - AC10 `TestPanelEntry_BlockedFlashLifecycle` (:475) — tick-clears and next-actionable-key-clears, on both pages.
  - AC11 `TestPanelRouting_KeyExclusive` (:536) — a 10-key table where **each case first proves the effect reaches the page with the panel closed** (`t.Fatalf("precondition: …")`), so a swallow is never vacuously green. That precondition is the single strongest thing in this file.
  - AC12 `TestPanelRouting_PanelOwnedKeysNeverReachThePage` (:638) — `d` asserted as an absence of the page's effect (no delete modal), so it survives task 9-3 turning `d` into a write; `l` checked against modal, page, cmd, both cursors and both filter inputs.
  - AC13 `TestPanelRouting_CtrlCQuits` (:690); AC14 `TestPanelRouting_NestsOverMultiSelect` (:702) — mode still active, count and identity of the marked set intact, `N selected` band still in the rendered frame, and the same three re-checked after `Esc`.
  - AC15 — `TestFooterRevision_StaticDescriptorsUnfiltered` (`footer_revision_test.go:289`) asserts both static descriptors `reflect.DeepEqual`-unchanged under the blocked states.
  - End-to-end confirmation beyond the unit tests: `internal/capture/fixtures.go:445,464,550,575,635` drive the real `t` keypress (and `x`,`t` for the Projects fixture) through `Model.Update`, so every rendered theme-panel fixture transits this gate.
- Notes:
  - Verbatim `want` constants (:17-21) with an explicit "a test asserting a constant against itself pins nothing" note — this is the §14A pinning the task demanded.
  - Mild redundancy: `TestPanelEntry_PinnedCopy` (:512) re-asserts the same three constants the behavioural tests already compare verbatim against `flashText`. Cheap and self-documenting; not worth removing.
  - `requireFlashBandVisible` is applied to the NO_COLOR case on both pages and to the post-read short refusal, so the flash is proven to reach the band (not merely the field) on the Projects slot as well as the Sessions one.
  - No mocking beyond the existing `fakeThemeSource` seam; the `opens` counter is what discriminates the two refusal shapes, which is behaviour, not an implementation detail.

CODE QUALITY:
- Project conventions: Followed. Value-receiver `(tea.Model, tea.Cmd)` handlers with the `(&m).mutate()` idiom match the rest of `model.go`; the flash is raised through `setThemeFlash` so it carries the theme precedence tier rather than being classified by wording (`model.go:1340-1345`); no `t.Parallel()`; unit-lane only; no raw hex; no new log component.
- SOLID principles: Good. One decision function (`themePanelEntry`), one arithmetic authority (`themePanelFloor`), one copy map per lifecycle (`themePanelEntryFlash` / `themePanelForcedCloseFlash`), one arming path (`armThemePanel`, reachable only through `openThemePanel`).
- Complexity: Low. `themePanelEntry` is two guards; `updateThemePanel` is a flat switch with the quit case first; the double floor evaluation is two call sites of one pure function rather than two derivations.
- Modern idioms: Yes — `slices.IndexFunc`, `max`, range-over-int elsewhere in the file; nothing dated introduced here.
- Readability: Good. The comments that remain are why-comments (why the pre-read floor passes `false`, why the closed/entry copy pairs must not be unified, why the panel arm sits ahead of the page dispatch and is key-scoped, why `Enter` does not close).
- Comment accuracy: No stale or false claims found in the changed code, and no process artifacts (no task ids, phase numbers or `§` references) in `theme_panel.go`, `theme_panel_geometry.go` or the two `model.go` arms.
- Issues: none blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] internal/tui/theme_panel.go:121-122 — the post-read refusal silently emits a `theme: enumerated` line for a panel that never opens, and nothing in source records that this is accepted rather than a leak. Replace the doc comment above `openThemePanel` with: `// Re-read per open, so a drop-in edit shows without relaunching. The floor is` / `// re-checked with the real DirUnusable — the warning row raises it by one. A` / `// refusal here has already read the directory, so its enumerated line stands.`
- [idea] internal/tui/model.go:1756 — the Projects `t` arm carries no `m.commandPending` guard, unlike the `x`/`d`/`e` arms above it (:1735, :1745, :1750), so the panel opens from the command-pending mode whose footer advertises only `⏎ / n / esc`. Nothing breaks (Esc resolves innermost-first, the pending command survives) and §9.7's blocker list does not name command-pending, so this needs a decision: either add the guard for symmetry with the other suppressed page keys, or leave it live and record command-pending as deliberately non-blocking.
