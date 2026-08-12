TASK: theming-system-8-12 — Projects Gains The Transient-Flash Slot

ACCEPTANCE CRITERIA:
1. A flash set while Projects is active renders a `▌` band beneath the title separator, byte-identical to the Sessions band for the same message/width.
2. The Projects list height shrinks by the slot's measured height when the flash appears and is restored when it clears — asserted through `projectBandHeight`, no separate arithmetic.
3. A flash outranks the command-pending banner while shown; the banner returns when it clears.
4. The band never co-renders with the command-pending banner.
5. An actionable keypress on Projects clears the flash and still reaches its normal handler (`x` switches page, `e` opens the edit modal).
6. A non-key event (window size, focus, blur) does not clear a Projects flash.
7. The auto-clear tick clears a Projects flash through the existing generation guard; a superseded tick does not clear a newer flash.
8. No Sessions-only contender renders on Projects (no signpost, multi-select banner, unsupported banner).
9. Under `colourless` the Projects band drops hue and tint and keeps `▌` and `⚠`/`✓`.
10. The Sessions band is byte-unchanged, including the documented multi-select co-render exception.
11. The Projects filter header's precedence is unchanged (the Phase 9 change is absent).

STATUS: complete

SPEC CONTEXT:
§14A ("Projects gains a transient-flash slot"): the notice band was a Sessions-only arbiter, yet §9.6 binds `t` on Projects, §14.2 puts `t theme` in the Projects footer, and all six §14A theme flashes are reachable there. The spec is explicit that Projects gets **the flash contender alone**, not the full arbiter — "no other contender has a Projects analogue, and inventing them would be scope for nothing" — and that both alternatives (suppressing the flashes / refusing `t`) destroy a decided guarantee: suppression makes §9.10's proactive block a silent no-op and destroys §9.13's failed-commit report outright, because raising the flash *discharges* the outstanding state whether or not it rendered. §14A's other half — theme flashes outranking the filter line — is Phase 9's and correctly absent here. §13.3 requires a Projects-with-panel fixture (separate task 8-16).

IMPLEMENTATION:
- Status: Implemented (comment prose later condensed by the plan's sanctioned comment-trim tasks — see Non-blocking notes; behaviour intact)
- Location:
  - `internal/tui/notice_band.go:196-204` — `activeProjectNoticeBand`: flash (via the shared `flashSlotClaim`) → `bandCommand` → `ok=false` placeholder. Arbitrates, never co-renders.
  - `internal/tui/notice_band.go:208-217` — `renderActiveProjectNoticeBand`: routes the flash through the SAME `renderNoticeBand` primitive and the SAME `noticeBandOnBandText` token the Sessions band uses; only `bandCommand` diverts to `renderCommandBand`.
  - `internal/tui/model.go:3139-3146` — `renderProjectBandSlot` re-pointed at the arbiter, keeping the existing blank breathing row.
  - `internal/tui/model.go:1055-1074` — `applyProjectListSize` / `projectBandHeight` unchanged: the reserve is still measured off the rendered slot, so the flash row is budgeted by construction (criterion 2 satisfied with no second arithmetic).
  - `internal/tui/model.go:1330-1369` — `setFlash` / `setSuccessFlash` (via `setFlash`) / `clearFlash` now call `resyncPageLayouts`, which sizes **both** lists and keeps the pre-`WindowSizeMsg` no-op guard (`termWidth<=0 || termHeight<=0`).
  - `internal/tui/model.go:1707-1713` — `updateProjectsPage`'s actionable-key clear, in the same relative position as `updateSessionList`'s (`model.go:2351`): ahead of the Ctrl-C and `SettingFilter` guards, with the same deliberate fall-through and the same "one key, one intent" comment.
  - `internal/tui/model.go:1607-1611` — the shared `flashTickMsg` handler with its generation guard is reused unchanged; no Projects-specific duration, kind or tick was added.
  - `internal/tui/theme_panel.go:117,254,274` — theme flash raisers return `flashTickCmd(m.flashGen)` exactly as the Sessions callers do.
- Notes:
  - The plan asked for `resyncSessionLayout` to be "widened (or given a Projects sibling)". The implementation went further: `resyncPageLayouts` sizes **both** pages, not the active one, because a page can be *entered by a message* rather than a keypress (the §10.2 cold-boot route raises the warnings flash on Sessions and then `evaluateDefaultPage` flips to Projects on an empty session list with the flash still live). That is a correct superset of the requirement, and it is pinned by two dedicated tests. Sizing an inactive page costs an invisible reserve, so there is no regression on the other side.
  - The theme-precedence tier (`flashSlotClaim` / `themeFlashClaim`, `notice_band.go:235-255`) is Phase 9 work layered on top; both arbiters open with the same shared claim, so the two pages cannot drift. `TestFlashSlotClaim_BothArbitersOpenWithIt` pins that.
  - Criterion 11 holds in the current tree: `applyProjectsSectionHeader` (`model.go:3151-3169`) is untouched, and Phase 9 ultimately resolved the filter question by *co-render* (the filter line claims the section-header row, a different physical row — `notice_band.go:178-179`), so the Projects filter header is unchanged either way.
  - `TestProjectsPage_FlashTextNotRendered` was deliberately reversed to `TestProjectsPage_FlashRendered` (`sessions_flash_render_test.go:158`) — an intentional, spec-backed inversion of a prior assertion, not a weakened test. `TestLoadingPage_FlashTextNotRendered` is retained, so the Loading page still refuses the flash.
  - No raw hex introduced; the band routes through theme tokens only, so `colour_literal_guard_test.go` is unaffected.

TESTS:
- Status: Adequate
- Coverage: `internal/tui/projects_flash_test.go` carries all ten plan-named tests, one per acceptance criterion, plus three earned extras:
  - `TestProjectsFlash_RendersInTheBandSlot` — asserts true byte-identity by comparing `m.renderActiveProjectNoticeBand()` against a Sessions peer's `renderActiveNoticeBand()` at the same width/message, then pins placement in the composed frame (band under the header rule, band → blank → section header = 2 rows) and the `⚠` glyph.
  - `TestProjectsFlash_RecomputesListHeight` — height delta asserted purely through `projectBandHeight`, exactly as the plan required.
  - `TestProjectsFlash_WinsTheSlotOverCommandPending` — baseline banner → flash wins → banner returns.
  - `TestProjectsFlash_SingleSlot` — bans `commandBandText` / `commandBandCaret` / the chip text from the rendered view AND counts `▌`-prefixed rows, so a co-render is caught structurally, not only by copy.
  - `TestProjectsFlash_ActionableKeyClearsAndFallsThrough` — both halves of the fall-through (`x` reaches the page switch, `e` reaches the edit modal), which is the part a naive early-`return` would break.
  - `TestProjectsFlash_SurvivesWindowSize` — table over window-size/focus/blur.
  - `TestProjectsFlash_TickClearsWithGenerationGuard` — matching tick clears, superseded tick does not.
  - `TestProjectsFlash_OnlyTheFlashContender` — arms every Sessions-only contender at once (signpost, multi-select + selection, unsupported identity, burst opening) and asserts the Projects slot stays empty and none of their copy leaks.
  - `TestProjectsFlash_Colourless` — warning and success kinds; asserts the band equals its own `ansi.Strip` (no SGR at all) and keeps `▌` + `⚠`/`✓`.
  - `TestProjectsFlash_SessionsBandUnchanged` — the multi-select co-render exception and the Sessions height reserve.
  - Extras: `TestProjectsFlash_FilterHeaderPrecedenceUnchanged` (criterion 11, byte-compares the section-header row before/after the flash and still requires the flash in the band); `TestProjectsFlash_PageEnteredByMessageIsSized` and `TestProjectsFlash_ColdBootWarningsLandOnASizedProjectsFrame` (the both-pages resync — the second drives the real cold-boot route end to end and asserts the composed frame does not exceed `contentHeight`).
  - Cross-page parity is additionally held by `flash_slot_claim_test.go:67-97` and `theme_flash_precedence_test.go:329-331,612` (table-driven over both arbiters), so the two pages cannot silently diverge.
- Notes: Not over-tested — the three extras cover distinct failure modes (off-page raise, real cold-boot route, filter-header precedence) rather than repeating the same assertion. Each test would fail if the feature broke: the arbiter tests read the arbiter directly, the render tests read the composed view, and the height tests read `projectList.Height()`. Nothing was verified by executing the suite (reading only, per the review contract).

CODE QUALITY:
- Project conventions: Followed. Value-receiver `Model` methods and pointer-receiver mutators match the package's existing split; the arbiter/render/slot triple mirrors the Sessions naming (`activeNoticeBand` / `renderActiveNoticeBand` / `renderSessionBandSlot` → `activeProjectNoticeBand` / `renderActiveProjectNoticeBand` / `renderProjectBandSlot`), so the two pages read as siblings. No new logging, no new state, no `t.Parallel()`, no test touching real tmux.
- SOLID principles: Good. One arbiter per page, one render entry point per page, one shared band primitive — the flash cannot render two ways. The height reserve stays measured off the same block the view composes, so the pagination invariant is held by construction rather than by a second calculation.
- Complexity: Low. `activeProjectNoticeBand` is two guarded returns; `renderActiveProjectNoticeBand` is one branch. No nesting added to `updateProjectsPage` beyond the one-line clear.
- Modern idioms: Yes.
- Readability: Good. Comment accuracy checked against the code: `projectBandHeight`'s "measured off renderProjectBandSlot" and `resyncPageLayouts`' "both pages, not just the active one" both hold; the "same position as updateSessionList's clear" claim in `updateProjectsPage` is true (both sit ahead of the Ctrl-C and `SettingFilter` guards). No comment names a task id, phase or spec section in production code (the trim tasks' standard).
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [do-now] `internal/tui/notice_band.go:193-195` — `activeProjectNoticeBand`'s doc comment no longer records *why* Projects carries only the flash contender, so the asymmetry with `activeNoticeBand` reads as an oversight a later contributor may "fix" by porting the signpost/multi-select/unsupported arms across. The sentence existed in the task's own commit and was removed by the plan's later comment-trim tasks (11–17), which is sanctioned in general but dropped a durable decision here. Add one line above the existing comment: `// Projects carries the flash contender alone: every other contender in activeNoticeBand is a Sessions element with no Projects analogue, so the asymmetry between the two arbiters is deliberate — do not port one across.`
