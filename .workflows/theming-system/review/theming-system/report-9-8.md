TASK: theming-system-9-8 — Theme Flashes Outrank The Filter Line On Both Pages

ACCEPTANCE CRITERIA:
- `setThemeFlash` sets the theme origin; `setFlash` / `setSuccessFlash` reset it to default.
- All six theme flashes are raised through `setThemeFlash` — asserted by a source-level or call-site test that no theme copy reaches `setFlash` directly.
- With `FilterState == FilterApplied` on Sessions, a theme flash renders in the notice band and the locked-query header still renders on its own row.
- With `FilterState == Filtering` on Sessions, a theme flash renders in the notice band and the live filter input still owns the section-header row.
- Both filter states behave identically on Projects, through task 8-12's flash slot.
- A non-theme flash's rendering, precedence and lifecycle are byte-identical to today under both filter states.
- The band still holds at most one contender — no double band on any page.
- The theme flash still outranks the multi-select banner and still co-renders per the documented Sessions exception.
- The flash lifecycle is unchanged (auto-clear tick + generation guard, clear-on-next-actionable-key).
- No new notice-band role, contender or permanent entry; the six route through the single transient-flash slot.
- `t` during a pending burst is still swallowed, so the theme flash and `Opening n/N…` are never both due.

STATUS: complete

SPEC CONTEXT:
Specification §14A (lines 1807–1827) fixes the six theme flash copies in a table, states the band's contender order (*filter line → burst progress → transient flash → multi-select banner → unsupported banner → no-tags signpost*), and rules that the theme flashes take precedence over the filter line because it is the one contender above flash that can stay live across a whole panel open/use/close. It names the two guarantees that would otherwise fail silently: §9.13's failed-commit report (destroyed rather than deferred, because raising the flash discharges the outstanding state) and §9.10's proactive `NO_COLOR` block (would produce nothing at all). §14A also requires Projects to gain the flash contender alone (task 8-12), since `t` is bound there and all six flashes are reachable on that page.

The plan task flagged and resolved an ambiguity: Portal splits the spec's single contender list across two *physical* rows — the filter line is a section-header claimant (`applySectionHeader` / `applyProjectsSectionHeader`) and the flash is the separate notice-band row — so no filter-state suppression exists and the guarantee holds by construction. The task therefore mandates an origin discriminator plus an explicit, tested rule rather than inventing a suppression.

IMPLEMENTATION:
- Status: Implemented (as amended by later in-plan remediation tasks 16-6 and 17-15)
- Location:
  - `internal/tui/sessions_flash.go:18-26` — `flashOrigin` type with `flashOriginDefault` (zero value) / `flashOriginTheme`.
  - `internal/tui/model.go:247-249` — `flashOrigin` field on `Model`, beside `flashText` / `flashGen` / `flashKind`.
  - `internal/tui/model.go:1330-1351` — `setFlash` resets kind and origin; `setThemeFlash` delegates to `setFlash` (bump generation, set text, `flashWarning` kind, `resyncPageLayouts`) then stamps `flashOriginTheme`; `setSuccessFlash` likewise delegates, so it resets the origin.
  - `internal/tui/notice_band.go:226-255` — `flashClaim` → `themeFlashClaim` → `flashSlotClaim` tier prologue; both arbiters (`activeNoticeBand:180-191`, `activeProjectNoticeBand:196-204`) open with `flashSlotClaim`, so the tier order cannot fork per page.
  - `internal/tui/theme_panel.go:116-119, 130, 252, 273` — all six copies raised through `setThemeFlash`: the NO_COLOR block and both entry-floor blocks via `blockThemePanel`, both forced-close flashes via `resizeThemePanel`, and the failed-commit report via `reportOutstandingCommitFailure`.
  - `internal/tui/theme_panel.go:29-56` — the six copies match the §14A table byte-for-byte (`themeNotSavedFlash` deliberately carries no `⚠`; the band's warning role prepends it).
- Notes:
  - All six routes verified by reading: `themePanelEntry` → `blockThemePanel` (NO_COLOR, narrow, short), `openThemePanel`'s re-check → `blockThemePanel`, `resizeThemePanel` → `setThemeFlash(themePanelForcedCloseFlash)`, `closeThemePanel` → `reportOutstandingCommitFailure` → `setThemeFlash(themeNotSavedFlash)`. Every remaining `setFlash` call site in the package (`bootstrap_warnings.go:64`, `model.go:1593,2485`, `burst_progress.go:251,270`, `burst_partial_failure.go:23,33`) carries non-theme copy.
  - Burst swallow confirmed at `model.go:2333-2338` — the `burstPending` early-return precedes the flash-clear, the `SettingFilter` guard and the `t` case at `model.go:2424`, so `t` raises nothing while `Opening n/N…` owns the header row.
  - Lifecycle confirmed unchanged and identical on both pages: the clear-on-actionable-key sites are `model.go:2351` (Sessions) and `model.go:1711` (Projects), and `flashTickMsg` + `flashGen` are untouched by the new setter.
  - Deliberate, twice-reviewed design residue: `flashOrigin` has **no runtime effect today**. `flashSlotClaim`'s two arms return the same claim because the model holds one `flashText`, and the filter line never contends for the band slot. Task 17-15 (`7f95fc57`) replaced an earlier comment that overclaimed ("The filter line sits between the two arms") with an accurate one stating the tier is a forward guard granted at set time. This matches the plan's own ambiguity resolution, so it is intent-as-amended rather than drift — not raised as a finding.
  - No drift from the plan's "leave the lifecycle alone / no new band role / no seventh contender" constraints: `noticeBandRole` is unchanged (four roles), no new arbiter arm was added, and both arbiters share one prologue (extracted by task 16-6, `9fe6974d`).

TESTS:
- Status: Adequate (mild, contained redundancy — see notes)
- Coverage: `internal/tui/theme_flash_precedence_test.go` carries all ten planned test functions plus one added by 17-15:
  - `TestThemeFlash_OriginDiscriminator` (:15) — default origin on a fresh model, `setThemeFlash` stamps the theme origin while keeping `flashWarning`, bumps `flashGen`, and re-syncs layout (asserts the session list loses exactly the band-slot rows); `setFlash` / `setSuccessFlash` reset the origin; the tier claims only a theme-origin flash.
  - `TestThemeFlash_AllSixUseSetThemeFlash` (:106) — a six-case table driving each copy through its **real** production path (keypress-blocked entry ×3, resize-forced close ×2, panel close report) and asserting both the copy and `flashOriginTheme`; plus an AST source guard over the package's non-test files that fails any `setFlash`/`setSuccessFlash` call whose args reference the theme-copy vocabulary (consts/vars whose literal mentions "theme", and string-returning funcs declared in `theme*.go`). The guard fatals when the vocabulary is empty (:194-196), so it cannot pass by having stopped looking. Vocabulary construction covers all six copies, including the two dim-selecting helpers.
  - `TestThemeFlash_OutranksAppliedFilterOnSessions` (:470) / `TestThemeFlash_OutranksLiveFilterOnSessions` (:486) / `TestThemeFlash_OutranksAppliedFilterOnProjects` (:500, both filter states) — each asserts the arbitrated band, the composed frame containing the copy, exactly one bar-glyph row in the notice slot, the slot being band+blank, the section-header row byte-identical to the pre-flash baseline, the band sitting two rows above the filter row with a blank between, and the filter state left untouched.
  - `TestThemeFlash_FilterLineIsNotABandContender` (:525, added by 17-15) — pins the conformance argument: an applied filter arbitrates no band and renders no slot; flash and filter occupy separate rows; the tier changes no rendered outcome under a filter.
  - `TestThemeFlash_NonThemeFlashUnchanged` (:579) — both surfaces × both filter states, plus origin-stays-default, band render equal to the shared primitive's output, precedence over the signpost and command banner, and the tick/generation lifecycle.
  - `TestThemeFlash_SingleSlotHolds` (:632) — one-row band, one arbitrated band, zero rows after clear, and the burst-swallow case asserting `t` returns no command, raises nothing, and leaves `Opening` on the header row.
  - `TestThemeFlash_ComposesWithMultiSelect` (:676) — theme flash wins the slot while the marked set survives, banner renders below the band, exactly one band row.
  - `TestThemeFlash_LifecycleUntouched` (:711) — matching tick clears, superseded tick is dropped, next actionable key clears; run on both pages.
  - `TestThemeFlash_NoNewBandRole` (:750) — themed vs ordinary flash render byte-identically, arbitrate to the existing `bandWarning`, and leave nothing behind after clear.
  - `internal/tui/flash_slot_claim_test.go` (task 16-6) covers the extracted prologue directly; `internal/tui/theme_panel_close_report_test.go:347` (`TestCloseReport_OutranksFilterLine`) reuses this task's helpers to prove §9.13's report specifically reaches the band through the real commit-failure path under an applied filter — complementary, not redundant.
  Every test would fail if the feature broke: the source guard fails on a re-pointed call site, the discriminator tests fail if either reset is dropped, and the render assertions fail if the slot stops carrying the flash or the header row changes.
- Notes: the tests assert behaviour (arbitrated band, composed frame, row geometry, filter state) rather than internals, apart from the deliberate `flashOrigin` field assertions the acceptance criteria require. The `Filtering` case is raised via `setThemeFlash` directly rather than by keypress, correctly documented at :485-486 — `t` is a literal filter character while the input is focused, and no production path can raise a theme flash in that state, so a direct raise is the only honest way to pin the guarantee.

CODE QUALITY:
- Project conventions: Followed. No new log component or attr key; no new tmux/state touch; the change stays inside `internal/tui`. Tests carry no `t.Parallel()`, live in the unit lane, and reuse the package's existing helpers (`sourceguardtest.PackageGoFiles` via `parsePackageFiles`) rather than hand-rolling a source walk — consistent with the repo's ~20 source guards.
- SOLID principles: Good. `setThemeFlash` composes `setFlash` instead of duplicating it, so the generation bump, kind reset and layout re-sync cannot drift. The claim functions are a clean three-step chain (`flashClaim` → `themeFlashClaim` → `flashSlotClaim`) with a single home for the tier order consumed by both arbiters.
- Complexity: Low. Every added function is a guard clause plus a delegation; no new branching in the render path.
- Modern idioms: Yes. `iota` enum with a meaningful zero value (`flashOriginDefault`), so an un-stamped flash is a default-tier flash by construction; named multi-returns consistent with the neighbouring claim functions.
- Readability: Good. The precedence rule is documented where it is encoded (`flashSlotClaim`), the discriminator's rationale is on the type, and the "granted at set time, never inferred from text" argument appears at both the setter and the claim.
- Comment accuracy: Verified against the code. `flashSlotClaim`'s comment correctly describes the tier as a forward guard that changes no rendered outcome today (17-15 corrected an earlier version that falsely placed the filter line between the two arms); `activeNoticeBand`'s "the filter line has no arm here: it claims the section-header row" matches `section_header.go`'s `filterPromptPrefix` claimant; the `Model.flashOrigin` field comment ("reset by setFlash / setSuccessFlash and stamped only by setThemeFlash") holds, since `setSuccessFlash` delegates to `setFlash`. No process-artifact references (task ids, phases, spec sections) survive in the changed production comments.
- Security: N/A — no external input, no I/O.
- Performance: No concern. The tier adds one integer comparison per band arbitration.
- Issues: None blocking.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/flash_slot_claim_test.go:55-64 — the `"setFlash resets the origin to the default tier"` subtest duplicates `theme_flash_precedence_test.go:59-68` (`"setFlash resets the origin to default"`) in intent and assertion. Delete it from `flash_slot_claim_test.go`, which owns the tier-order contract; the setter's reset semantics belong to `TestThemeFlash_OriginDiscriminator` alone.
