TASK: theming-system-16-6 — Give The Flash's Tier Ordering One Home And Its Origin Field The Model

ACCEPTANCE CRITERIA:
- The theme-tier-then-ordinary-flash order appears once; neither arbiter restates it.
- `themeState` declares no flash field; `flashOrigin` sits with `flashText`/`flashGen`/`flashKind` on `Model`.
- `setFlash` no longer touches `m.themeState`.
- Band precedence on Sessions and on Projects is byte-identical to before on every rendered path.
- `go test ./internal/tui ./internal/capture` passes.

STATUS: complete

SPEC CONTEXT:
Spec §14A (specification.md:1816-1827) governs this surface. The notice band is a single-slot arbiter whose order is *filter line → burst progress → transient flash → multi-select banner → unsupported banner → no-tags signpost*; all six theme signals route through the transient-flash slot, and the theme flashes take precedence over the filter line because two decided guarantees fail silently otherwise — §9.13's failed-commit report is DESTROYED rather than deferred if it cannot claim the slot (raising the flash discharges the outstanding state), and §9.10's proactive `NO_COLOR` block would produce nothing at all. §14A:1825 adds the Projects flash slot ("Projects gets the flash contender alone, not the full arbiter"), which is why the ordering rule spans two arbiters — exactly the duplication this task removes. The task is a pure structural refactor: the spec-mandated ordering is unchanged, only its number of declaration sites.

IMPLEMENTATION:
- Status: Implemented (verified against commit 9fe6974d and the current tree)
- Location:
  - `internal/tui/notice_band.go:250-255` — new `func (m Model) flashSlotClaim() (role noticeBandRole, message string, ok bool)`, holding the theme tier first and the ordinary flash below it, declared beside `flashClaim` (:226) and `themeFlashClaim` (:235).
  - `internal/tui/notice_band.go:181` and `:197` — `activeNoticeBand` and `activeProjectNoticeBand` each open with one `m.flashSlotClaim()` call. Each keeps only its own page-specific arms below, in their prior order (Sessions: `byTagSignpost && !multiSelectMode && !unsupportedBannerActive`; Projects: `commandPending`).
  - `internal/tui/model.go:247-249` — `flashOrigin flashOrigin` now sits directly beneath `flashText`/`flashGen`/`flashKind` (:244-246), with its doc comment moved across.
  - `internal/tui/theme_state.go:34-81` — `themeState` declares no flash field; the struct is back to its documented charter (nomination, keys, seams, gate/appearance resolution, active palette, startup canvas hex, reply, commitFailed, capture seeds).
  - `internal/tui/model.go:1336` (`setFlash` resets `m.flashOrigin = flashOriginDefault`), `:1344` (`setThemeFlash` stamps `flashOriginTheme`), `notice_band.go:236` (band predicate reads `m.flashOrigin`). A repo-wide grep for `themeState.flashOrigin` returns nothing outside the historical diff.
  - `internal/tui/sessions_flash.go:21-26` — the `flashOrigin` type and its two constants stayed put, as instructed; only the comment's cross-reference was re-pointed to `flashSlotClaim`.
- Notes:
  - Behaviour-preservation is structural, not merely asserted: `flashSlotClaim`'s body is the two former arms in their exact prior order, so both arbiters' arm sequences are unchanged and no rendered path can differ. `setThemeFlash` remains the sole writer of the theme tier (grep confirms two writers total: the reset in `setFlash`, the stamp in `setThemeFlash`).
  - The field move is zero-risk on the value side: `flashOriginDefault` is the `iota` zero, so a `Model` zero value and `WithInitialFlash` (`model.go:648-655`, which sets text+kind directly at construction) both land on the default tier exactly as before.
  - Amended intent, not drift: Do-item 1 asked the new function to carry "the comment explaining where the filter line sits between them". The current comment (`notice_band.go:242-249`) instead says the tier is a forward guard that changes no rendered outcome today, because the model holds one `flashText` and the filter line claims the section-header row rather than contending for this slot. That wording is the deliberate product of the later task 17-15 (`7f95fc57`, "correct the theme-flash tier's claims and pin the conformance argument"), which removed the earlier overclaim. Per the review context's later-phase rule this is intent-as-amended and correct — the newer comment is the accurate one.
  - Line numbers in the task text (2314/2330, notice_band.go:392-400/461-468/526, theme_state.go:151) predate the phase-17 comment condensation; the code is at the locations listed above.

TESTS:
- Status: Adequate (one redundant subtest — see notes)
- Coverage:
  - `internal/tui/flash_slot_claim_test.go:5-65` — `TestFlashSlotClaim_TierOrder`: no flash claims nothing; a theme flash claims through the theme tier (asserted against `themeFlashClaim`'s own return, with a fixture-sanity `Fatal`); an ordinary flash claims through the flash tier (asserted against `flashClaim`); the theme tier is not displaced while its flash is live; `setFlash` resets the origin to the default tier.
  - `internal/tui/flash_slot_claim_test.go:67-97` — `TestFlashSlotClaim_BothArbitersOpenWithIt`: table over {theme flash, ordinary flash}, with `byTagSignpost` AND `commandPending` both true, asserting `activeNoticeBand` and `activeProjectNoticeBand` each return byte-identical `(role, message, ok)` to `flashSlotClaim`. This is the right shape for the task's central claim — it proves both arbiters delegate rather than restate, and that the shared prologue outranks each page's own arm.
  - Pre-existing suites re-pointed to `m.flashOrigin` and still green in intent: `theme_flash_precedence_test.go` (origin discriminator, the six-copy `setThemeFlash` table + AST source guard, outranks-applied/live-filter on both pages, single-slot holds, composes with multi-select, lifecycle, no new band role), `sessions_flash_state_test.go:118`, `theme_panel_close_report_test.go:68`. Those cover the *rendered* precedence paths the "byte-identical" criterion needs; the refactor introduces no new rendered path.
  - Capture: no fixture ever set `flashOrigin` (grep across `internal/capture` and `cmd/capturetool` finds no reference), so the flash / theme-signal / command-pending frames are untouched by the field move; `sessionsInlineFlashFixture` (`internal/capture/fixtures.go:317-330`) seeds only `initialFlash`, whose default-tier origin is unchanged.
- Notes:
  - Mild over-testing: `flash_slot_claim_test.go:55-64` ("setFlash resets the origin to the default tier") is a verbatim duplicate in substance of the pre-existing `theme_flash_precedence_test.go:59-68` ("setFlash resets the origin to default") — same fixture, same setup, same single assertion. The task's Tests list asked for it without noticing one already existed. Non-blocking; see notes below.
  - Not under-tested: the one behaviour that genuinely cannot be exercised today (an ordinary flash and a theme flash live simultaneously) is impossible by construction — the model holds one `flashText`, and `setFlash` clears the origin — so the tests correctly assert the order via delegation-equality against each tier's own claim rather than fabricating an unreachable state. `theme_flash_precedence_test.go:560-575` explicitly pins the "the theme tier changes no outcome under a filter" invariant, which is the honest statement of today's mechanism.

CODE QUALITY:
- Project conventions: Followed. Naming matches the neighbouring `flashClaim`/`themeFlashClaim` pair; named multi-returns are consistent across all three; no `t.Parallel()`; the new test file is unit-lane, hermetic, and spawns no tmux/daemon/binary.
- SOLID principles: Good. This is a straight single-responsibility improvement on both halves — the tier order gets one owner, and `themeState` is restored to its documented charter (the generic `setFlash` no longer reaches into the theme subsystem, and the general band arbiter no longer resolves NON-theme precedence by reading out of `m.themeState`).
- Complexity: Low. Two two-line arms extracted; both call sites shrink from four lines to two.
- Modern idioms: Yes. `iota` enum with a load-bearing zero value; the extracted function returns the inner claim verbatim rather than rebuilding it.
- Readability: Good. The four attributes of the one live flash are now declared adjacently on `Model`, and a reader chasing "why does a theme signal outrank X" lands on one function.
- Comment accuracy: Verified line by line against the code. `flashSlotClaim`'s comment (`:242-249`) accurately describes the tier as a forward guard granted at set time by `setThemeFlash`, and its claim that "the filter line claims the section-header row instead of contending here" matches `activeNoticeBand`'s arm list (no filter arm) and the `filterPromptPrefix` claimant in the section-header path. The `Model.flashOrigin` comment ("reset by setFlash / setSuccessFlash and stamped only by setThemeFlash") holds — `setSuccessFlash` (`model.go:1348`) delegates to `setFlash`. `themeState`'s remaining comments make no reference to a flash field. No task ids, phase numbers or spec-section references leaked into production comments.
- Issues: None.

BLOCKING ISSUES:
- None.

NON-BLOCKING NOTES:
- [quickfix] internal/tui/flash_slot_claim_test.go:55-64 — delete the `"setFlash resets the origin to the default tier"` subtest; `theme_flash_precedence_test.go:59-68` already asserts exactly this (same `noticeBandModel` fixture, same `setThemeFlash` → `setFlash` sequence, same single `flashOriginDefault` check), and the sibling `setSuccessFlash` case lives beside that one, so the surviving copy is the better-placed of the two.
